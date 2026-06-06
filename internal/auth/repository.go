package auth

import "context"

// UserRepository handles persistence for User entities
type UserRepository interface {
	// Write
	Create(ctx context.Context, user *User) error
	Update(ctx context.Context, user *User) error
	Delete(ctx context.Context, userID string) error // soft deletes via gorm.DeletedAt

	// Read
	FindByID(ctx context.Context, userID string) (*User, error)
	FindByFirebaseUID(ctx context.Context, firebaseUID string) (*User, error)
	FindByEmail(ctx context.Context, email string) (*User, error)

	// Targeted updates — use column-specific updates to avoid GORM zero-value pitfalls
	UpdateEmailVerified(ctx context.Context, userID string, verified bool) error
	UpdatePhoneVerified(ctx context.Context, userID string, verified bool) error
	UpdateLastLoginAt(ctx context.Context, userID string) error
	UpdateTradingPinHash(ctx context.Context, userID, pinHash string) error
}

// RefreshTokenRepository handles persistence for refresh token sessions
type RefreshTokenRepository interface {
	// Write
	Create(ctx context.Context, token *RefreshToken) error
	Revoke(ctx context.Context, tokenID string) error           // single session logout
	RevokeAllByUserID(ctx context.Context, userID string) error // logout from all devices

	// Read
	FindByTokenHash(ctx context.Context, tokenHash string) (*RefreshToken, error)
	FindActiveByUserID(ctx context.Context, userID string) ([]RefreshToken, error) // for "active sessions" screen

	// Cleanup — intended to be called by a background cron job, not inline in requests
	DeleteExpired(ctx context.Context) error
}

// KycProfileRepository handles persistence for KYC profiles
type KycProfileRepository interface {
	// Write
	Create(ctx context.Context, kyc *KycProfile) error
	Update(ctx context.Context, kyc *KycProfile) error
	UpdateStatus(ctx context.Context, kycID string, status KycStatus, reason, reviewedBy string) error

	// Read
	FindByID(ctx context.Context, kycID string) (*KycProfile, error)
	FindByUserID(ctx context.Context, userID string) (*KycProfile, error)
	FindByStatus(ctx context.Context, status KycStatus) ([]KycProfile, error) // used by admin review queue
}
