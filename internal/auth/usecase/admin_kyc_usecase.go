package usecase

import (
	"context"
	"log"

	"drexa/internal/auth"
)

type adminKycUsecase struct {
	kycRepo      auth.KycProfileRepository
	notifService auth.NotificationService
	userRepo     auth.UserRepository
}

func NewAdminKycUsecase(
	kycRepo auth.KycProfileRepository,
	notifService auth.NotificationService,
	userRepo auth.UserRepository,
) auth.AdminKycUsecase {
	return &adminKycUsecase{
		kycRepo:      kycRepo,
		notifService: notifService,
		userRepo:     userRepo,
	}
}

func (uc *adminKycUsecase) ListByStatus(ctx context.Context, status auth.KycStatus) ([]auth.KycProfile, error) {
	return uc.kycRepo.FindByStatus(ctx, status)
}

func (uc *adminKycUsecase) GetByID(ctx context.Context, kycID string) (*auth.KycProfile, error) {
	return uc.kycRepo.FindByID(ctx, kycID)
}

func (uc *adminKycUsecase) GetDecryptedNIK(ctx context.Context, kycID string) (string, error) {
	if _, err := uc.kycRepo.FindByID(ctx, kycID); err != nil {
		return "", auth.ErrKycNotFound
	}
	// NIK decryption (AES-256-GCM) is not yet implemented — callers must not treat the
	// return value as plaintext until this TODO is resolved.
	return "", auth.ErrKycNotFound
}

func (uc *adminKycUsecase) Approve(ctx context.Context, kycID, reviewedBy string) error {
	kyc, err := uc.kycRepo.FindByID(ctx, kycID)
	if err != nil {
		return auth.ErrKycNotFound
	}

	if err := uc.kycRepo.UpdateStatus(ctx, kycID, auth.KycStatusApproved, "", reviewedBy); err != nil {
		return err
	}

	user, err := uc.userRepo.FindByID(ctx, kyc.UserID)
	if err != nil {
		log.Printf("kyc approve: failed to find user %s for notification: %v", kyc.UserID, err)
		return nil
	}
	if err := uc.notifService.SendKycApproved(ctx, user.UserID, user.Email); err != nil {
		log.Printf("kyc approve: failed to send notification to %s: %v", user.Email, err)
	}
	return nil
}

func (uc *adminKycUsecase) Reject(ctx context.Context, kycID, reviewedBy, reason string) error {
	kyc, err := uc.kycRepo.FindByID(ctx, kycID)
	if err != nil {
		return auth.ErrKycNotFound
	}

	if err := uc.kycRepo.UpdateStatus(ctx, kycID, auth.KycStatusRejected, reason, reviewedBy); err != nil {
		return err
	}

	user, err := uc.userRepo.FindByID(ctx, kyc.UserID)
	if err != nil {
		log.Printf("kyc reject: failed to find user %s for notification: %v", kyc.UserID, err)
		return nil
	}
	if err := uc.notifService.SendKycRejected(ctx, user.UserID, user.Email, reason); err != nil {
		log.Printf("kyc reject: failed to send notification to %s: %v", user.Email, err)
	}
	return nil
}

func (uc *adminKycUsecase) UpdateStatus(ctx context.Context, kycID string, status auth.KycStatus) error {
	return uc.kycRepo.UpdateStatus(ctx, kycID, status, "", "")
}
