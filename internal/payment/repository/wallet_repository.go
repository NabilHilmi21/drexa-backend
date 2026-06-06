package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"drexa/internal/payment"
)

type walletRepository struct {
	db *gorm.DB
}

func NewWalletRepository(db *gorm.DB) payment.WalletRepository {
	return &walletRepository{db: db}
}

func (r *walletRepository) FindByUserID(ctx context.Context, userID string) (*payment.Wallet, error) {
	var w payment.Wallet
	result := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&w)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, payment.ErrWalletNotFound
		}
		return nil, result.Error
	}
	return &w, nil
}

func (r *walletRepository) Create(ctx context.Context, w *payment.Wallet) error {
	return r.db.WithContext(ctx).Create(w).Error
}

func (r *walletRepository) Credit(ctx context.Context, userID string, amount int64) error {
	result := r.db.WithContext(ctx).
		Model(&payment.Wallet{}).
		Where("user_id = ?", userID).
		Update("balance", gorm.Expr("balance + ?", amount))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return payment.ErrWalletNotFound
	}
	return nil
}

// Debit uses SELECT FOR UPDATE to prevent double-spend under concurrent requests.
func (r *walletRepository) Debit(ctx context.Context, userID string, amount int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var w payment.Wallet
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ?", userID).
			First(&w).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return payment.ErrWalletNotFound
			}
			return err
		}
		if w.Balance < amount {
			return payment.ErrInsufficientFunds
		}
		return tx.Model(&w).Update("balance", w.Balance-amount).Error
	})
}
