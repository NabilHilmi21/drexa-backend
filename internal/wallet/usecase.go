package wallet

import "context"

// WalletUsecase handles user-facing wallet operations
type WalletUsecase interface {
	// Wallet management
	GetOrCreate(ctx context.Context, userID string, currency CurrencyCode) (*Wallet, error)
	GetBalance(ctx context.Context, userID string, currency CurrencyCode) (*Wallet, error)
	GetAllBalances(ctx context.Context, userID string) ([]Wallet, error)

	// Deposit flow
	InitiateDeposit(ctx context.Context, userID string, req *InitiateDepositRequest) (*DepositRequest, error)
	// CreateDepositIntent creates a Stripe-style PaymentIntent and returns its client secret for the
	// frontend's embedded payment form (POST /payments/deposit/intent).
	CreateDepositIntent(ctx context.Context, userID string, req *InitiateDepositRequest) (*DepositIntent, error)
	ConfirmDeposit(ctx context.Context, providerRef string) error // called by webhook
	VerifyDeposit(ctx context.Context, providerRef string) error  // explicit check by frontend

	// Withdrawal flow (requires KYC approved + trading PIN verified upstream)
	InitiateWithdrawal(ctx context.Context, userID string, req *InitiateWithdrawalRequest) (*WithdrawalRequest, error)

	// Transaction history
	GetTransactions(ctx context.Context, userID string, page, pageSize int) ([]Transaction, error)

	// Transfer and Crypto
	Transfer(ctx context.Context, req *InternalTransferRequest) (*Transaction, error)
	InitiateCryptoWithdrawal(ctx context.Context, userID string, req *InitiateCryptoWithdrawalRequest) (*Transaction, error)
}

// AdminWalletUsecase handles admin-facing wallet operations
type AdminWalletUsecase interface {
	// Manual balance adjustments — for OJK reconciliation or support cases
	Credit(ctx context.Context, walletID string, amount int64, description, adminID string) error
	Debit(ctx context.Context, walletID string, amount int64, description, adminID string) error

	// Withdrawal review queue
	ListPendingWithdrawals(ctx context.Context) ([]WithdrawalRequest, error)
	ApproveWithdrawal(ctx context.Context, withdrawalID, adminID string) error
	RejectWithdrawal(ctx context.Context, withdrawalID, adminID, reason string) error
}

// CryptoWalletUsecase handles on-chain (crypto) deposit addresses and balances.
type CryptoWalletUsecase interface {
	// GetDepositAddress returns (creating on first call) the user's on-chain deposit
	// address for a currency, along with its current on-chain balance.
	GetDepositAddress(ctx context.Context, userID string, currency CurrencyCode) (*CryptoAsset, error)

	// GetAssets returns every supported on-chain asset for the user with live balances.
	GetAssets(ctx context.Context, userID string) ([]CryptoAsset, error)

	// HandleCryptoWebhook processes Tatum's incoming deposit webhook
	HandleCryptoWebhook(ctx context.Context, payload WebhookPayload) error
}

// CryptoProvider abstracts the external crypto infrastructure (Tatum).
type CryptoProvider interface {
	// GenerateWallet creates a new HD wallet for a chain and returns its extended public key.
	GenerateWallet(ctx context.Context, chain string) (xpub string, err error)
	// GetXpub returns the configured master extended public key for the chain.
	GetXpub(chain string) (string, error)
	// DeriveAddress derives the receiving address for an xpub at a derivation index.
	DeriveAddress(ctx context.Context, chain, xpub string, index int) (address string, err error)
	// GetBalance returns the address's confirmed balance as a decimal string (in the coin's main unit).
	GetBalance(ctx context.Context, chain, address string) (balance string, err error)
	// SendTransaction sends a crypto transaction and returns the transaction hash.
	SendTransaction(ctx context.Context, chain string, amount string, toAddress string) (txHash string, err error)
	// SubscribeAddressWebhook subscribes an address to receive webhooks for deposits.
	SubscribeAddressWebhook(ctx context.Context, chain, address string) (subscriptionID string, err error)
}

// CryptoAsset is the user-facing view of an on-chain asset.
type CryptoAsset struct {
	Currency CurrencyCode `json:"currency"`
	Chain    string       `json:"chain"`
	Network  string       `json:"network"` // human label, e.g. "Bitcoin testnet"
	Address  string       `json:"address"`
	Balance  string       `json:"balance"` // decimal string in main unit (e.g. "0.0123")
}

// PaymentService abstracts the deposit (pay-in) provider integration — Stripe in this build.
// Implement this per-provider in internal/wallet/service/.
type PaymentService interface {
	// CreatePaymentSession creates a hosted payment page and returns a URL + provider reference ID
	CreatePaymentSession(ctx context.Context, depositID string, amount int64, currency CurrencyCode, userEmail string) (sessionURL, providerRef string, err error)

	// CreatePaymentIntent creates a PaymentIntent and returns its client secret (consumed by the
	// frontend Stripe Elements form) plus the provider reference ID used to reconcile the webhook.
	CreatePaymentIntent(ctx context.Context, depositID string, amount int64, currency CurrencyCode, userEmail string) (clientSecret, providerRef string, err error)

	// VerifyPayment verifies the payment status with the provider synchronously
	VerifyPayment(ctx context.Context, providerRef string) (bool, error)
}

// DisbursementService abstracts the withdrawal (pay-out) provider integration — PayPal Payouts in
// this build. Kept separate from PaymentService so deposits (Stripe) and withdrawals (PayPal) can
// use different providers. Implement per-provider in internal/wallet/service/.
type DisbursementService interface {
	// CreateDisbursement sends money to the recipient and returns the provider's disbursement ID.
	CreateDisbursement(ctx context.Context, req *DisbursementRequest) (providerRef string, err error)
}

// ─── Request DTOs (domain layer — not HTTP layer) ────────────────────────────

type InitiateDepositRequest struct {
	Amount    int64        // in smallest unit (e.g. cents for IDR = 1 IDR)
	Currency  CurrencyCode
	UserEmail string // needed for Stripe checkout session
}

// DepositIntent is the result of CreateDepositIntent — the client secret the frontend hands to
// Stripe Elements, plus the deposit record id used as the transaction reference.
type DepositIntent struct {
	DepositID    string
	ClientSecret string
}

type InitiateWithdrawalRequest struct {
	Amount      int64
	Currency    CurrencyCode
	PayPalEmail string // recipient PayPal account that receives the payout
}

type DisbursementRequest struct {
	WithdrawalID   string
	Amount         int64
	Currency       CurrencyCode
	RecipientEmail string // PayPal account to pay out to
}
