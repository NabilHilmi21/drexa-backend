package payment

import (
	"errors"
	"time"
)

// ─── Entities ───────────────────────────────────────────────────────────────

type TransactionType string

const (
	TxDeposit    TransactionType = "deposit"
	TxWithdrawal TransactionType = "withdrawal"
)

type TransactionStatus string

const (
	TxPending   TransactionStatus = "pending"
	TxCompleted TransactionStatus = "completed"
	TxFailed    TransactionStatus = "failed"
)

// Wallet holds a user's fiat balance in the smallest currency unit (e.g. cents for USD).
type Wallet struct {
	WalletID  string    `gorm:"primaryKey;column:wallet_id"`
	UserID    string    `gorm:"column:user_id;uniqueIndex"`
	Balance   int64     `gorm:"column:balance;default:0"` // stored in cents
	Currency  string    `gorm:"column:currency;default:usd"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

// Transaction records every deposit or withdrawal attempt.
type Transaction struct {
	TxID                  string            `gorm:"primaryKey;column:tx_id"`
	UserID                string            `gorm:"column:user_id;index"`
	Type                  TransactionType   `gorm:"column:type"`
	Amount                int64             `gorm:"column:amount"` // in cents
	Currency              string            `gorm:"column:currency"`
	Status                TransactionStatus `gorm:"column:status;default:pending"`
	StripePaymentIntentID string            `gorm:"column:stripe_payment_intent_id;index"`
	CreatedAt             time.Time         `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt             time.Time         `gorm:"column:updated_at;autoUpdateTime"`
}

// ─── Domain Errors ───────────────────────────────────────────────────────────

var (
	ErrWalletNotFound      = errors.New("wallet not found")
	ErrInsufficientFunds   = errors.New("insufficient funds")
	ErrTransactionNotFound = errors.New("transaction not found")
	ErrInvalidAmount       = errors.New("amount must be greater than zero")
	ErrMinimumDeposit      = errors.New("minimum deposit is $10")
	ErrMinimumWithdrawal   = errors.New("minimum withdrawal is $10")
)

const MinimumAmountCents int64 = 1000 // $10.00
