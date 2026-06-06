package payment

import "context"

// StripeService wraps Stripe API calls so they can be mocked in tests.
type StripeService interface {
	CreatePaymentIntent(ctx context.Context, amount int64, currency, userID, txID string) (clientSecret, paymentIntentID string, err error)
	ConstructWebhookEvent(payload []byte, signature string) (eventType, paymentIntentID string, err error)
}
