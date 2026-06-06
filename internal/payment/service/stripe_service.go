package service

import (
	"context"
	"encoding/json"
	"fmt"

	stripe "github.com/stripe/stripe-go/v79"
	"github.com/stripe/stripe-go/v79/paymentintent"
	"github.com/stripe/stripe-go/v79/webhook"

	"drexa/internal/payment"
)

type stripeService struct {
	webhookSecret string
}

func NewStripeService(secretKey, webhookSecret string) payment.StripeService {
	stripe.Key = secretKey
	return &stripeService{webhookSecret: webhookSecret}
}

func (s *stripeService) CreatePaymentIntent(_ context.Context, amount int64, currency, userID, txID string) (string, string, error) {
	params := &stripe.PaymentIntentParams{
		Amount:   stripe.Int64(amount),
		Currency: stripe.String(currency),
		Metadata: map[string]string{
			"user_id": userID,
			"tx_id":   txID,
		},
		AutomaticPaymentMethods: &stripe.PaymentIntentAutomaticPaymentMethodsParams{
			Enabled: stripe.Bool(true),
		},
	}

	pi, err := paymentintent.New(params)
	if err != nil {
		return "", "", fmt.Errorf("stripe: create payment intent: %w", err)
	}

	return pi.ClientSecret, pi.ID, nil
}

func (s *stripeService) ConstructWebhookEvent(payload []byte, signature string) (string, string, error) {
	event, err := webhook.ConstructEvent(payload, signature, s.webhookSecret)
	if err != nil {
		return "", "", fmt.Errorf("stripe: webhook signature verification failed: %w", err)
	}

	if event.Type != "payment_intent.succeeded" && event.Type != "payment_intent.payment_failed" {
		return string(event.Type), "", nil
	}

	var pi stripe.PaymentIntent
	if err := json.Unmarshal(event.Data.Raw, &pi); err != nil {
		return "", "", fmt.Errorf("stripe: unmarshal payment intent: %w", err)
	}

	return string(event.Type), pi.ID, nil
}
