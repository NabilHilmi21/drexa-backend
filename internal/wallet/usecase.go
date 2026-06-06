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
	ConfirmDeposit(ctx context.Context, providerRef string) error // called by webhook

	// Withdrawal flow (requires KYC approved + trading PIN verified upstream)
	InitiateWithdrawal(ctx context.Context, userID string, req *InitiateWithdrawalRequest) (*WithdrawalRequest, error)

	// Transaction history
	GetTransactions(ctx context.Context, userID string, page, pageSize int) ([]Transaction, error)
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

// PaymentService abstracts payment provider integration (Stripe, Midtrans, etc.)
// Implement this per-provider in internal/wallet/service/
type PaymentService interface {
	// CreatePaymentSession creates a hosted payment page and returns a URL + provider reference ID
	CreatePaymentSession(ctx context.Context, depositID string, amount int64, currency CurrencyCode, userEmail string) (sessionURL, providerRef string, err error)

	// CreateDisbursement sends money to a bank account and returns the provider's disbursement ID
	CreateDisbursement(ctx context.Context, req *DisbursementRequest) (providerRef string, err error)
}

// ─── Request DTOs (domain layer — not HTTP layer) ────────────────────────────

type InitiateDepositRequest struct {
	Amount    int64        // in smallest unit (e.g. cents for IDR = 1 IDR)
	Currency  CurrencyCode
	UserEmail string // needed for Stripe checkout session
}

type InitiateWithdrawalRequest struct {
	Amount        int64
	Currency      CurrencyCode
	BankCode      string // e.g. "BCA", "BNI"
	AccountNumber string
	AccountName   string
}

type DisbursementRequest struct {
	WithdrawalID  string
	Amount        int64
	Currency      CurrencyCode
	BankCode      string
	AccountNumber string
	AccountName   string
}
