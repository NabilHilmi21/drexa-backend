package service

import (
	"context"
	"fmt"
	"time"

	"drexa/internal/wallet"
)

// NullDisbursementService is a no-op DisbursementService for local development when PayPal
// credentials are not configured. It returns a fake provider reference without paying anyone.
// Replace by configuring PAYPAL_CLIENT_ID/PAYPAL_SECRET to use PayPalDisbursementService.
type NullDisbursementService struct{}

func NewNullDisbursementService() wallet.DisbursementService {
	return &NullDisbursementService{}
}

func (s *NullDisbursementService) CreateDisbursement(
	ctx context.Context,
	req *wallet.DisbursementRequest,
) (providerRef string, err error) {
	return fmt.Sprintf("null_payout_%s_%d", req.WithdrawalID[:8], time.Now().Unix()), nil
}
