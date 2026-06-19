package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"drexa/internal/wallet"
)

// chainInfo maps a domain currency to its blockchain and a human-readable network label.
type chainInfo struct {
	chain   string
	mainnet string
	testnet string
}

func chainFor(c wallet.CurrencyCode, testnet bool) (chainInfo, bool) {
	table := map[wallet.CurrencyCode]chainInfo{
		wallet.CurrencyBTC:  {chain: "bitcoin", mainnet: "Bitcoin", testnet: "Bitcoin testnet"},
		wallet.CurrencyETH:  {chain: "ethereum", mainnet: "Ethereum", testnet: "Ethereum testnet (Sepolia)"},
		wallet.CurrencySOL:  {chain: "solana", mainnet: "Solana", testnet: "Solana devnet"},
		wallet.CurrencyUSDT: {chain: "ethereum", mainnet: "Ethereum (ERC-20)", testnet: "Ethereum testnet (ERC-20)"},
		wallet.CurrencyBNB:  {chain: "bsc", mainnet: "BNB Smart Chain", testnet: "BSC testnet"},
	}
	info, ok := table[c]
	return info, ok
}

// supportedCryptoCurrencies lists currencies the crypto provider can serve addresses for.
var supportedCryptoCurrencies = []wallet.CurrencyCode{
	wallet.CurrencyBTC, wallet.CurrencyETH, wallet.CurrencySOL, wallet.CurrencyUSDT, wallet.CurrencyBNB,
}

type cryptoWalletUsecase struct {
	addrRepo   wallet.CryptoAddressRepository
	walletRepo wallet.WalletRepository
	txRepo     wallet.TransactionRepository
	tx         wallet.TxManager
	provider   wallet.CryptoProvider
	testnet    bool
}

func NewCryptoWalletUsecase(
	addrRepo wallet.CryptoAddressRepository,
	walletRepo wallet.WalletRepository,
	txRepo wallet.TransactionRepository,
	txManager wallet.TxManager,
	provider wallet.CryptoProvider,
	testnet bool,
) wallet.CryptoWalletUsecase {
	return &cryptoWalletUsecase{
		addrRepo:   addrRepo,
		walletRepo: walletRepo,
		txRepo:     txRepo,
		tx:         txManager,
		provider:   provider,
		testnet:    testnet,
	}
}

// getOrCreateAddress returns the persisted deposit address for a user+currency,
// generating a new HD wallet and deriving index 0 on first use.
func (uc *cryptoWalletUsecase) getOrCreateAddress(ctx context.Context, userID string, currency wallet.CurrencyCode) (*wallet.CryptoAddress, chainInfo, error) {
	info, ok := chainFor(currency, uc.testnet)
	if !ok {
		return nil, info, wallet.ErrUnsupportedCurrency
	}

	existing, err := uc.addrRepo.FindByUserAndCurrency(ctx, userID, currency)
	if err == nil {
		return existing, info, nil
	}
	if !errors.Is(err, wallet.ErrCryptoAddressNotFound) {
		return nil, info, err
	}

	// Solana uses a single hot-wallet address for all users (no xpub HD derivation).
	// Deposits are attributed to users by memo/reference at the application layer.
	if info.chain == "solana" {
		solAddr, err := uc.provider.GetXpub("solana")
		if err != nil {
			return nil, info, fmt.Errorf("get SOL address: %w", err)
		}
		rec := &wallet.CryptoAddress{
			ID:              uuid.NewString(),
			UserID:          userID,
			Currency:        currency,
			Chain:           info.chain,
			Address:         solAddr,
			Xpub:            solAddr,
			DerivationIndex: 0,
		}
		if err := uc.addrRepo.Create(ctx, rec); err != nil {
			return nil, info, fmt.Errorf("save address: %w", err)
		}
		return rec, info, nil
	}

	// Look up the master xpub for the chain
	xpub, err := uc.provider.GetXpub(info.chain)
	if err != nil {
		return nil, info, fmt.Errorf("get master xpub: %w", err)
	}

	// Get the highest derivation index currently in use
	highestIndex, err := uc.addrRepo.GetHighestDerivationIndex(ctx, info.chain)
	if err != nil {
		return nil, info, fmt.Errorf("get highest derivation index: %w", err)
	}

	nextIndex := highestIndex + 1

	address, err := uc.provider.DeriveAddress(ctx, info.chain, xpub, nextIndex)
	if err != nil {
		return nil, info, fmt.Errorf("derive address: %w", err)
	}

	// Webhook subscription is best-effort — never block deposit-address creation on it.
	// Tatum subscriptions can fail (plan limits, chain-code differences); the balance
	// endpoint still reflects on-chain deposits even without the push webhook.
	_, _ = uc.provider.SubscribeAddressWebhook(ctx, info.chain, address)

	rec := &wallet.CryptoAddress{
		ID:              uuid.NewString(),
		UserID:          userID,
		Currency:        currency,
		Chain:           info.chain,
		Address:         address,
		Xpub:            xpub,
		DerivationIndex: nextIndex,
	}
	if err := uc.addrRepo.Create(ctx, rec); err != nil {
		return nil, info, fmt.Errorf("save address: %w", err)
	}
	return rec, info, nil
}

func (uc *cryptoWalletUsecase) networkLabel(info chainInfo) string {
	if uc.testnet {
		return info.testnet
	}
	return info.mainnet
}

func (uc *cryptoWalletUsecase) GetDepositAddress(ctx context.Context, userID string, currency wallet.CurrencyCode) (*wallet.CryptoAsset, error) {
	rec, info, err := uc.getOrCreateAddress(ctx, userID, currency)
	if err != nil {
		return nil, err
	}

	// Best-effort live balance; never fail the address lookup over a balance hiccup.
	balance, err := uc.provider.GetBalance(ctx, info.chain, rec.Address)
	if err != nil {
		balance = "0"
	}

	return &wallet.CryptoAsset{
		Currency: currency,
		Chain:    info.chain,
		Network:  uc.networkLabel(info),
		Address:  rec.Address,
		Balance:  balance,
	}, nil
}

func (uc *cryptoWalletUsecase) GetAssets(ctx context.Context, userID string) ([]wallet.CryptoAsset, error) {
	assets := make([]wallet.CryptoAsset, 0, len(supportedCryptoCurrencies))
	for _, currency := range supportedCryptoCurrencies {
		asset, err := uc.GetDepositAddress(ctx, userID, currency)
		if err != nil {
			return nil, err
		}
		assets = append(assets, *asset)
	}
	return assets, nil
}

func (uc *cryptoWalletUsecase) HandleCryptoWebhook(ctx context.Context, payload wallet.WebhookPayload) error {
	addrRec, err := uc.addrRepo.FindByAddress(ctx, payload.Address)
	if err != nil {
		return fmt.Errorf("address not found: %w", err)
	}

	requiredConf := 1
	if !uc.testnet {
		switch addrRec.Currency {
		case wallet.CurrencyBTC:
			requiredConf = 3
		case wallet.CurrencyETH, wallet.CurrencyUSDT:
			requiredConf = 12
		case wallet.CurrencyBNB:
			requiredConf = 15
		case wallet.CurrencySOL:
			requiredConf = 32
		}
	} else {
		switch addrRec.Currency {
		case wallet.CurrencyETH, wallet.CurrencyUSDT:
			requiredConf = 2
		}
	}

	// Only enforce a confirmation threshold when Tatum actually reports one.
	// ADDRESS_TRANSACTION notifications often omit `confirmations` (fire on detection);
	// crediting is idempotent by txId below, so a missing count must not drop the deposit.
	if payload.Confirmations > 0 && payload.Confirmations < requiredConf {
		return fmt.Errorf("insufficient confirmations: %d < %d", payload.Confirmations, requiredConf)
	}

	// We use txManager to atomically update the wallet balance
	return uc.tx.Do(ctx, func(ctx context.Context) error {
		// First check if transaction already exists (idempotency by tx_hash)
		existingTx, err := uc.txRepo.FindByRefID(ctx, payload.TxId)
		if err == nil && existingTx != nil {
			// Already processed
			return nil
		}

		w, err := uc.walletRepo.FindByUserAndCurrency(ctx, addrRec.UserID, addrRec.Currency)
		if errors.Is(err, wallet.ErrWalletNotFound) {
			// First deposit for this user+currency — create the ledger wallet on the fly.
			w = &wallet.Wallet{
				WalletID: uuid.NewString(),
				UserID:   addrRec.UserID,
				Currency: addrRec.Currency,
				Status:   wallet.WalletStatusActive,
			}
			if cerr := uc.walletRepo.Create(ctx, w); cerr != nil {
				return cerr
			}
		} else if err != nil {
			return err
		}

		lockedW, err := uc.walletRepo.FindByIDForUpdate(ctx, w.WalletID)
		if err != nil {
			return err
		}

		// The amount in webhook is usually a string representing the coin's main unit (e.g. "0.005").
		// We parse it as a float64 and convert it to the smallest unit (satoshi / wei).
		var amountFloat float64
		_, err = fmt.Sscanf(payload.Amount, "%f", &amountFloat)
		if err != nil {
			return wallet.ErrInvalidAmount
		}

		var amountInt64 int64
		switch addrRec.Currency {
		case wallet.CurrencyBTC:
			amountInt64 = int64(amountFloat * 100_000_000) // satoshis (10^8)
		case wallet.CurrencyETH, wallet.CurrencyBNB:
			amountInt64 = int64(amountFloat * 1_000_000_000_000_000_000) // wei (10^18)
		case wallet.CurrencyUSDT:
			amountInt64 = int64(amountFloat * 1_000_000) // micro-USDT (10^6)
		case wallet.CurrencySOL:
			amountInt64 = int64(amountFloat * 1_000_000_000) // lamports (10^9)
		default:
			return wallet.ErrUnsupportedCurrency
		}

		balanceBefore := lockedW.Balance
		newBalance := lockedW.Balance + amountInt64

		if err := uc.walletRepo.UpdateBalance(ctx, lockedW.WalletID, newBalance); err != nil {
			return err
		}

		return uc.txRepo.Create(ctx, &wallet.Transaction{
			TxID:          uuid.New().String(),
			WalletID:      lockedW.WalletID,
			UserID:        lockedW.UserID,
			Type:          wallet.TxTypeDeposit,
			Status:        wallet.TxStatusCompleted,
			Amount:        amountInt64,
			BalanceBefore: balanceBefore,
			BalanceAfter:  newBalance,
			Currency:      lockedW.Currency,
			RefID:         payload.TxId, // store Tatum tx hash as ref ID
			Description:   "Crypto deposit",
		})
	})
}

