package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"drexa/internal/payment"
)

type transactionRepository struct {
	db *gorm.DB
}

func NewTransactionRepository(db *gorm.DB) payment.TransactionRepository {
	return &transactionRepository{db: db}
}

func (r *transactionRepository) Create(ctx context.Context, tx *payment.Transaction) error {
	return r.db.WithContext(ctx).Create(tx).Error
}

func (r *transactionRepository) FindByID(ctx context.Context, txID string) (*payment.Transaction, error) {
	var tx payment.Transaction
	result := r.db.WithContext(ctx).Where("tx_id = ?", txID).First(&tx)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, payment.ErrTransactionNotFound
		}
		return nil, result.Error
	}
	return &tx, nil
}

func (r *transactionRepository) FindByStripePaymentIntentID(ctx context.Context, piID string) (*payment.Transaction, error) {
	var tx payment.Transaction
	result := r.db.WithContext(ctx).Where("stripe_payment_intent_id = ?", piID).First(&tx)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, payment.ErrTransactionNotFound
		}
		return nil, result.Error
	}
	return &tx, nil
}

func (r *transactionRepository) UpdateStatus(ctx context.Context, txID string, status payment.TransactionStatus) error {
	result := r.db.WithContext(ctx).
		Model(&payment.Transaction{}).
		Where("tx_id = ?", txID).
		Update("status", status)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return payment.ErrTransactionNotFound
	}
	return nil
}

func (r *transactionRepository) ListByUserID(ctx context.Context, userID string, limit, offset int) ([]payment.Transaction, error) {
	var txs []payment.Transaction
	result := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&txs)
	return txs, result.Error
}
