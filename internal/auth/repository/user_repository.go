package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"drexa/internal/auth"
)

type userRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) auth.UserRepository {
	return &userRepository{db: db}
}

// ─── Write ───────────────────────────────────────────────────────────────────

func (r *userRepository) Create(ctx context.Context, user *auth.User) error {
	result := r.db.WithContext(ctx).Create(user)
	if result.Error != nil {
		return result.Error
	}
	return nil
}

func (r *userRepository) Update(ctx context.Context, user *auth.User) error {
	// Save updates all fields including zero values — intentional here
	result := r.db.WithContext(ctx).Save(user)
	return result.Error
}

func (r *userRepository) Delete(ctx context.Context, userID string) error {
	// gorm.DeletedAt on the struct means this soft deletes automatically
	result := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Delete(&auth.User{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return auth.ErrUserNotFound
	}
	return nil
}

// ─── Read ────────────────────────────────────────────────────────────────────

func (r *userRepository) FindByID(ctx context.Context, userID string) (*auth.User, error) {
	var user auth.User
	result := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		First(&user)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, auth.ErrUserNotFound
		}
		return nil, result.Error
	}
	return &user, nil
}

func (r *userRepository) FindByFirebaseUID(ctx context.Context, firebaseUID string) (*auth.User, error) {
	var user auth.User
	result := r.db.WithContext(ctx).
		Where("firebase_uid = ?", firebaseUID).
		First(&user)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, auth.ErrUserNotFound
		}
		return nil, result.Error
	}
	return &user, nil
}

func (r *userRepository) FindByEmail(ctx context.Context, email string) (*auth.User, error) {
	var user auth.User
	result := r.db.WithContext(ctx).
		Where("email = ?", email).
		First(&user)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, auth.ErrUserNotFound
		}
		return nil, result.Error
	}
	return &user, nil
}

// ─── Targeted Updates ────────────────────────────────────────────────────────
// Use column-specific Update() instead of Save() to avoid GORM zero-value pitfalls.
// Save() would skip false booleans and empty strings — targeted Update() does not.

func (r *userRepository) UpdateEmailVerified(ctx context.Context, userID string, verified bool) error {
	result := r.db.WithContext(ctx).
		Model(&auth.User{}).
		Where("user_id = ?", userID).
		Update("is_email_verified", verified)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return auth.ErrUserNotFound
	}
	return nil
}

func (r *userRepository) UpdatePhoneVerified(ctx context.Context, userID string, verified bool) error {
	result := r.db.WithContext(ctx).
		Model(&auth.User{}).
		Where("user_id = ?", userID).
		Update("is_phone_verified", verified)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return auth.ErrUserNotFound
	}
	return nil
}

func (r *userRepository) UpdateLastLoginAt(ctx context.Context, userID string) error {
	result := r.db.WithContext(ctx).
		Model(&auth.User{}).
		Where("user_id = ?", userID).
		Update("last_login_at", time.Now())
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return auth.ErrUserNotFound
	}
	return nil
}

func (r *userRepository) UpdateTradingPinHash(ctx context.Context, userID, pinHash string) error {
	result := r.db.WithContext(ctx).
		Model(&auth.User{}).
		Where("user_id = ?", userID).
		Update("trading_pin_hash", pinHash)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return auth.ErrUserNotFound
	}
	return nil
}
