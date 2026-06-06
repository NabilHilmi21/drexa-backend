package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"drexa/internal/auth"
)

type kycUsecase struct {
	userRepo auth.UserRepository
	kycRepo  auth.KycProfileRepository
}

func NewKycUsecase(userRepo auth.UserRepository, kycRepo auth.KycProfileRepository) auth.KycUsecase {
	return &kycUsecase{userRepo: userRepo, kycRepo: kycRepo}
}

func (uc *kycUsecase) Submit(ctx context.Context, userID string, kyc *auth.KycProfile) error {
	if _, err := uc.userRepo.FindByID(ctx, userID); err != nil {
		return auth.ErrUserNotFound
	}

	kyc.KycID = uuid.NewString()
	kyc.UserID = userID
	kyc.Status = auth.KycStatusPending
	kyc.SubmittedAt = time.Now()

	if err := uc.kycRepo.Create(ctx, kyc); err != nil {
		return errors.New("failed to submit KYC")
	}
	return nil
}

func (uc *kycUsecase) GetByUserID(ctx context.Context, userID string) (*auth.KycProfile, error) {
	kyc, err := uc.kycRepo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, auth.ErrKycNotFound
	}
	return kyc, nil
}

func (uc *kycUsecase) IsVerified(ctx context.Context, userID string) (bool, error) {
	kyc, err := uc.kycRepo.FindByUserID(ctx, userID)
	if err != nil {
		return false, auth.ErrKycNotFound
	}
	return kyc.Status == auth.KycStatusApproved, nil
}

func (uc *kycUsecase) IsExpired(ctx context.Context, userID string) (bool, error) {
	kyc, err := uc.kycRepo.FindByUserID(ctx, userID)
	if err != nil {
		return false, auth.ErrKycNotFound
	}
	if kyc.ExpiresAt == nil {
		return false, nil
	}
	return time.Now().After(*kyc.ExpiresAt), nil
}
