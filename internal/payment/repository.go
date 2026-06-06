package payment

import "context"

type WalletRepository interface {
	FindByUserID(ctx context.Context, userID string) (*Wallet, error)
	Create(ctx context.Context, wallet *Wallet) error
	// Credit and Debit use row-level locking to prevent race conditions.
	Credit(ctx context.Context, userID string, amount int64) error
	Debit(ctx context.Context, userID string, amount int64) error
}

type TransactionRepository interface {
	Create(ctx context.Context, tx *Transaction) error
	FindByID(ctx context.Context, txID string) (*Transaction, error)
	FindByStripePaymentIntentID(ctx context.Context, piID string) (*Transaction, error)
	UpdateStatus(ctx context.Context, txID string, status TransactionStatus) error
	ListByUserID(ctx context.Context, userID string, limit, offset int) ([]Transaction, error)
}
