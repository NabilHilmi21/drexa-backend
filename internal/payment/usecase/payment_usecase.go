package usecase

import (
	"context"
	"fmt"
	"log"

	"github.com/google/uuid"

	"drexa/internal/payment"
)

type paymentUsecase struct {
	walletRepo  payment.WalletRepository
	txRepo      payment.TransactionRepository
	stripeSvc   payment.StripeService
}

func NewPaymentUsecase(
	walletRepo payment.WalletRepository,
	txRepo payment.TransactionRepository,
	stripeSvc payment.StripeService,
) payment.PaymentUsecase {
	return &paymentUsecase{
		walletRepo: walletRepo,
		txRepo:     txRepo,
		stripeSvc:  stripeSvc,
	}
}

func (uc *paymentUsecase) CreateDepositIntent(ctx context.Context, userID string, amount int64) (string, string, error) {
	if amount <= 0 {
		return "", "", payment.ErrInvalidAmount
	}
	if amount < payment.MinimumAmountCents {
		return "", "", payment.ErrMinimumDeposit
	}

	// Ensure the user has a wallet, creating one on first deposit.
	if _, err := uc.walletRepo.FindByUserID(ctx, userID); err != nil {
		if err != payment.ErrWalletNotFound {
			return "", "", err
		}
		w := &payment.Wallet{
			WalletID: uuid.NewString(),
			UserID:   userID,
			Currency: "usd",
		}
		if err := uc.walletRepo.Create(ctx, w); err != nil {
			return "", "", fmt.Errorf("create wallet: %w", err)
		}
	}

	txID := uuid.NewString()
	clientSecret, piID, err := uc.stripeSvc.CreatePaymentIntent(ctx, amount, "usd", userID, txID)
	if err != nil {
		return "", "", err
	}

	tx := &payment.Transaction{
		TxID:                  txID,
		UserID:                userID,
		Type:                  payment.TxDeposit,
		Amount:                amount,
		Currency:              "usd",
		Status:                payment.TxPending,
		StripePaymentIntentID: piID,
	}
	if err := uc.txRepo.Create(ctx, tx); err != nil {
		return "", "", fmt.Errorf("record transaction: %w", err)
	}

	return clientSecret, txID, nil
}

func (uc *paymentUsecase) HandleWebhook(ctx context.Context, payload []byte, signature string) error {
	eventType, piID, err := uc.stripeSvc.ConstructWebhookEvent(payload, signature)
	if err != nil {
		return err
	}

	switch eventType {
	case "payment_intent.succeeded":
		return uc.handlePaymentSucceeded(ctx, piID)
	case "payment_intent.payment_failed":
		return uc.handlePaymentFailed(ctx, piID)
	default:
		// Unhandled event types are not an error — Stripe sends many event types.
		log.Printf("payment webhook: unhandled event type %q", eventType)
	}

	return nil
}

func (uc *paymentUsecase) handlePaymentSucceeded(ctx context.Context, piID string) error {
	tx, err := uc.txRepo.FindByStripePaymentIntentID(ctx, piID)
	if err != nil {
		return fmt.Errorf("webhook: find transaction for pi %s: %w", piID, err)
	}
	if tx.Status == payment.TxCompleted {
		return nil // idempotent — already processed
	}

	if err := uc.walletRepo.Credit(ctx, tx.UserID, tx.Amount); err != nil {
		return fmt.Errorf("webhook: credit wallet for user %s: %w", tx.UserID, err)
	}

	return uc.txRepo.UpdateStatus(ctx, tx.TxID, payment.TxCompleted)
}

func (uc *paymentUsecase) handlePaymentFailed(ctx context.Context, piID string) error {
	tx, err := uc.txRepo.FindByStripePaymentIntentID(ctx, piID)
	if err != nil {
		return fmt.Errorf("webhook: find transaction for pi %s: %w", piID, err)
	}
	return uc.txRepo.UpdateStatus(ctx, tx.TxID, payment.TxFailed)
}

func (uc *paymentUsecase) Withdraw(ctx context.Context, userID string, amount int64) error {
	if amount <= 0 {
		return payment.ErrInvalidAmount
	}
	if amount < payment.MinimumAmountCents {
		return payment.ErrMinimumWithdrawal
	}

	if err := uc.walletRepo.Debit(ctx, userID, amount); err != nil {
		return err
	}

	tx := &payment.Transaction{
		TxID:     uuid.NewString(),
		UserID:   userID,
		Type:     payment.TxWithdrawal,
		Amount:   amount,
		Currency: "usd",
		Status:   payment.TxCompleted,
	}
	return uc.txRepo.Create(ctx, tx)
}

func (uc *paymentUsecase) GetBalance(ctx context.Context, userID string) (*payment.Wallet, error) {
	w, err := uc.walletRepo.FindByUserID(ctx, userID)
	if err != nil {
		if err == payment.ErrWalletNotFound {
			// Return a zero-balance wallet for users who have never deposited.
			return &payment.Wallet{UserID: userID, Currency: "usd"}, nil
		}
		return nil, err
	}
	return w, nil
}

func (uc *paymentUsecase) GetTransactions(ctx context.Context, userID string, limit, offset int) ([]payment.Transaction, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	return uc.txRepo.ListByUserID(ctx, userID, limit, offset)
}
