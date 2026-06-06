package auth

import "context"

// AuthUsecase handles user-facing authentication flows
type AuthUsecase interface {
	// Sign in — creates user on first call, finds existing user on subsequent calls
	SignInWithFirebase(ctx context.Context, claims *FirebaseClaims) (*AuthToken, error)

	// Phone verification — Firebase handles email; backend handles phone for trading compliance
	SendPhoneVerificationOTP(ctx context.Context, userID string) error
	VerifyPhone(ctx context.Context, userID, otp string) (bool, error)

	// Session management
	RefreshToken(ctx context.Context, refreshToken string) (*AuthToken, error)
	Logout(ctx context.Context, refreshToken string) error
	LogoutAll(ctx context.Context, userID string) error

	// Trading PIN — required before executing trades or withdrawals
	SetTradingPin(ctx context.Context, userID, pin string) error
	VerifyTradingPin(ctx context.Context, userID, pin string) (bool, error)
}

// KycUsecase handles user-facing KYC submission and status checks
type KycUsecase interface {
	Submit(ctx context.Context, userID string, kyc *KycProfile) error
	GetByUserID(ctx context.Context, userID string) (*KycProfile, error) // user checks their own status
	IsVerified(ctx context.Context, userID string) (bool, error)
	IsExpired(ctx context.Context, userID string) (bool, error)
}

// AdminKycUsecase handles admin-facing KYC review operations
type AdminKycUsecase interface {
	ListByStatus(ctx context.Context, status KycStatus) ([]KycProfile, error) // admin review queue
	GetByID(ctx context.Context, kycID string) (*KycProfile, error)
	GetDecryptedNIK(ctx context.Context, kycID string) (string, error) // decrypts NIK for admin review
	Approve(ctx context.Context, kycID, reviewedBy string) error
	Reject(ctx context.Context, kycID, reviewedBy, reason string) error
	UpdateStatus(ctx context.Context, kycID string, status KycStatus) error
}
