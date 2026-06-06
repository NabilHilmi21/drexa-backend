package payment

import "context"

type PaymentUsecase interface {
	// CreateDepositIntent creates a Stripe PaymentIntent and a pending Transaction.
	// Returns the Stripe client_secret for the frontend to confirm the payment.
	CreateDepositIntent(ctx context.Context, userID string, amount int64) (clientSecret, txID string, err error)

	// HandleWebhook processes a Stripe webhook event.
	// On payment_intent.succeeded it credits the user's wallet.
	HandleWebhook(ctx context.Context, payload []byte, signature string) error

	// Withdraw debits the wallet immediately. Actual payout to the user's bank
	// requires Stripe Connect and is handled as a follow-up step.
	Withdraw(ctx context.Context, userID string, amount int64) error

	GetBalance(ctx context.Context, userID string) (*Wallet, error)
	GetTransactions(ctx context.Context, userID string, limit, offset int) ([]Transaction, error)
}
