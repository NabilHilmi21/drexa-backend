package service

import (
	"context"
	"fmt"
	"time"

	"drexa/internal/wallet"
)

// NullPaymentService is a no-op implementation of PaymentService for local development.
// It auto-generates fake provider references without calling any real payment API.
// Replace with StripePaymentService or MidtransPaymentService for production.
type NullPaymentService struct{}

func NewNullPaymentService() wallet.PaymentService {
	return &NullPaymentService{}
}

func (s *NullPaymentService) CreatePaymentSession(
	ctx context.Context,
	depositID string,
	amount int64,
	currency wallet.CurrencyCode,
	userEmail string,
) (sessionURL, providerRef string, err error) {
	fakeRef := fmt.Sprintf("null_dep_%s_%d", depositID[:8], time.Now().Unix())
	fakeURL := fmt.Sprintf("http://localhost:3000/wallet/deposit/mock?ref=%s", fakeRef)
	return fakeURL, fakeRef, nil
}

func (s *NullPaymentService) CreateDisbursement(
	ctx context.Context,
	req *wallet.DisbursementRequest,
) (providerRef string, err error) {
	fakeRef := fmt.Sprintf("null_dis_%s_%d", req.WithdrawalID[:8], time.Now().Unix())
	return fakeRef, nil
}
