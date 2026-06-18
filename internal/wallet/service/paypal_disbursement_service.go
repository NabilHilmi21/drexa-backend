package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"drexa/internal/wallet"
)

// PayPalDisbursementService implements wallet.DisbursementService using the PayPal Payouts API.
// It authenticates with OAuth2 client credentials (token cached until shortly before expiry) and
// sends a single-item payout batch per withdrawal, returning the payout_batch_id as the provider ref.
//
// Docs: https://developer.paypal.com/docs/api/payments.payouts-batch/v1/
type PayPalDisbursementService struct {
	clientID string
	secret   string
	baseURL  string // e.g. https://api-m.sandbox.paypal.com
	http     *http.Client

	mu          sync.Mutex
	cachedToken string
	tokenExpiry time.Time
}

func NewPayPalDisbursementService(clientID, secret, baseURL string) wallet.DisbursementService {
	return &PayPalDisbursementService{
		clientID: clientID,
		secret:   secret,
		baseURL:  strings.TrimRight(baseURL, "/"),
		http:     &http.Client{Timeout: 30 * time.Second},
	}
}

// token returns a valid OAuth2 access token, reusing a cached one until it is about to expire.
func (s *PayPalDisbursementService) token(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cachedToken != "" && time.Now().Before(s.tokenExpiry) {
		return s.cachedToken, nil
	}

	form := url.Values{"grant_type": {"client_credentials"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		s.baseURL+"/v1/oauth2/token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("paypal: build token request: %w", err)
	}
	req.SetBasicAuth(s.clientID, s.secret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := s.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("paypal: token request: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("paypal: token failed (%d): %s", resp.StatusCode, string(body))
	}

	var tr struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"` // seconds
	}
	if err := json.Unmarshal(body, &tr); err != nil {
		return "", fmt.Errorf("paypal: decode token: %w", err)
	}
	if tr.AccessToken == "" {
		return "", fmt.Errorf("paypal: empty access token")
	}

	s.cachedToken = tr.AccessToken
	// Refresh 60s early to avoid using a token that expires mid-request.
	s.tokenExpiry = time.Now().Add(time.Duration(tr.ExpiresIn-60) * time.Second)
	return s.cachedToken, nil
}

// CreateDisbursement sends a one-item PayPal payout to the recipient email and returns the
// payout_batch_id. The amount is given in the currency's smallest unit (cents) and converted
// to PayPal's decimal-string value.
func (s *PayPalDisbursementService) CreateDisbursement(
	ctx context.Context,
	req *wallet.DisbursementRequest,
) (providerRef string, err error) {
	if req.RecipientEmail == "" {
		return "", wallet.ErrRecipientRequired
	}

	token, err := s.token(ctx)
	if err != nil {
		return "", err
	}

	payload := paypalPayoutRequest{}
	payload.SenderBatchHeader.SenderBatchID = req.WithdrawalID
	payload.SenderBatchHeader.EmailSubject = "You have a withdrawal payout from Drexa"
	payload.SenderBatchHeader.EmailMessage = "Your withdrawal has been processed."
	payload.Items = []paypalPayoutItem{{
		RecipientType: "EMAIL",
		Receiver:      req.RecipientEmail,
		SenderItemID:  req.WithdrawalID,
		Note:          "Drexa wallet withdrawal",
	}}
	payload.Items[0].Amount.Value = minorUnitsToDecimal(req.Amount)
	payload.Items[0].Amount.Currency = string(req.Currency)

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("paypal: marshal payout: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		s.baseURL+"/v1/payments/payouts", bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("paypal: build payout request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := s.http.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("paypal: payout request: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	// PayPal returns 201 Created on a successfully accepted batch.
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("paypal: payout failed (%d): %s", resp.StatusCode, string(respBody))
	}

	var pr paypalPayoutResponse
	if err := json.Unmarshal(respBody, &pr); err != nil {
		return "", fmt.Errorf("paypal: decode payout response: %w", err)
	}
	if pr.BatchHeader.PayoutBatchID == "" {
		return "", fmt.Errorf("paypal: payout accepted but no batch id returned: %s", string(respBody))
	}

	return pr.BatchHeader.PayoutBatchID, nil
}

// minorUnitsToDecimal converts an integer amount in the currency's smallest unit (cents) to a
// PayPal decimal string, e.g. 1050 -> "10.50".
func minorUnitsToDecimal(minor int64) string {
	neg := ""
	if minor < 0 {
		neg, minor = "-", -minor
	}
	return fmt.Sprintf("%s%d.%02d", neg, minor/100, minor%100)
}

// ─── PayPal Payouts wire types ───────────────────────────────────────────────

type paypalPayoutRequest struct {
	SenderBatchHeader struct {
		SenderBatchID string `json:"sender_batch_id"`
		EmailSubject  string `json:"email_subject"`
		EmailMessage  string `json:"email_message"`
	} `json:"sender_batch_header"`
	Items []paypalPayoutItem `json:"items"`
}

type paypalPayoutItem struct {
	RecipientType string `json:"recipient_type"`
	Amount        struct {
		Value    string `json:"value"`
		Currency string `json:"currency"`
	} `json:"amount"`
	Receiver     string `json:"receiver"`
	SenderItemID string `json:"sender_item_id"`
	Note         string `json:"note"`
}

type paypalPayoutResponse struct {
	BatchHeader struct {
		PayoutBatchID string `json:"payout_batch_id"`
		BatchStatus   string `json:"batch_status"`
	} `json:"batch_header"`
}
