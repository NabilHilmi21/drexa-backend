package wallet

import (
	"errors"
	"time"
)

// ─── Enums ───────────────────────────────────────────────────────────────────

type TransactionType string

const (
	TxDeposit    TransactionType = "deposit"
	TxWithdrawal TransactionType = "withdrawal"
	TxTradeBuy   TransactionType = "trade_buy"
	TxTradeSell  TransactionType = "trade_sell"
	TxFee        TransactionType = "fee"
	TxP2PEscrow  TransactionType = "p2p_escrow"
	TxP2PRelease TransactionType = "p2p_release"
)

type TransactionStatus string

const (
	TxPending   TransactionStatus = "pending"
	TxConfirmed TransactionStatus = "confirmed"
	TxFailed    TransactionStatus = "failed"
)

type EntryType string

const (
	EntryDebit  EntryType = "debit"
	EntryCredit EntryType = "credit"
)

// ─── Entities ────────────────────────────────────────────────────────────────

// Wallet holds a user's balance for a single currency.
// Balances are derived from ledger entries — never modified directly.
type Wallet struct {
	WalletID         string  `gorm:"primaryKey;column:wallet_id"`
	UserID           string  `gorm:"column:user_id;index"`
	WalletAddress    string  `gorm:"column:wallet_address;default:''"`
	Currency         string  `gorm:"column:currency"`                     // e.g. "IDR", "BTC", "ETH"
	AvailableBalance float64 `gorm:"column:available_balance;type:numeric(36,18);default:0"`
	LockedBalance    float64 `gorm:"column:locked_balance;type:numeric(36,18);default:0"`
	CreatedAt        time.Time `gorm:"column:created_at;autoCreateTime"`
}

// LedgerEntry is one side of a double-entry bookkeeping record.
// Every mutation produces exactly two entries: one debit and one credit.
type LedgerEntry struct {
	EntryID     string    `gorm:"primaryKey;column:entry_id"`
	WalletID    string    `gorm:"column:wallet_id;index"`
	Type        EntryType `gorm:"column:type"`
	Amount      float64   `gorm:"column:amount;type:numeric(36,18)"`
	Currency    string    `gorm:"column:currency"`
	RefType     string    `gorm:"column:ref_type"` // e.g. "trade", "deposit", "withdrawal"
	RefID       string    `gorm:"column:ref_id"`
	Description string    `gorm:"column:description;default:''"`
	CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime"`
}

// Transaction is the high-level record of a user-visible financial event.
type Transaction struct {
	TransactionID string            `gorm:"primaryKey;column:transaction_id"`
	UserID        string            `gorm:"column:user_id;index"`
	Type          TransactionType   `gorm:"column:type"`
	Amount        float64           `gorm:"column:amount;type:numeric(36,18)"`
	Currency      string            `gorm:"column:currency"`
	Status        TransactionStatus `gorm:"column:status;default:pending"`
	TxHash        *string           `gorm:"column:tx_hash"` // on-chain hash for crypto transactions
	Fee           float64           `gorm:"column:fee;type:numeric(36,18);default:0"`
	CreatedAt     time.Time         `gorm:"column:created_at;autoCreateTime"`
}

// DepositAddress is a per-user per-currency crypto deposit address derived from an HD wallet.
type DepositAddress struct {
	ID              string    `gorm:"primaryKey;column:id"`
	UserID          string    `gorm:"column:user_id;index"`
	Currency        string    `gorm:"column:currency"`
	Network         string    `gorm:"column:network"` // e.g. "ERC20", "TRC20"
	Address         string    `gorm:"column:address;uniqueIndex"`
	DerivationIndex int       `gorm:"column:derivation_index"`
	CreatedAt       time.Time `gorm:"column:created_at;autoCreateTime"`
}

// ─── Domain Errors ───────────────────────────────────────────────────────────

var (
	ErrWalletNotFound      = errors.New("wallet not found")
	ErrInsufficientBalance = errors.New("insufficient available balance")
	ErrTransactionNotFound = errors.New("transaction not found")
)
