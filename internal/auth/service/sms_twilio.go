package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// SMSSender delivers a single SMS. Implement for any provider.
type SMSSender interface {
	SendSMS(ctx context.Context, to, body string) error
}

type twilioSMSSender struct {
	accountSID string
	authToken  string
	fromPhone  string
	client     *http.Client
}

// NewTwilioSMSSender returns an SMSSender backed by the Twilio Messages API.
func NewTwilioSMSSender(accountSID, authToken, fromPhone string) SMSSender {
	return &twilioSMSSender{
		accountSID: accountSID,
		authToken:  authToken,
		fromPhone:  fromPhone,
		client:     &http.Client{Timeout: 10 * time.Second},
	}
}

func (t *twilioSMSSender) SendSMS(ctx context.Context, to, body string) error {
	endpoint := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", t.accountSID)

	form := url.Values{}
	form.Set("From", t.fromPhone)
	form.Set("To", to)
	form.Set("Body", body)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("twilio: build request: %w", err)
	}
	req.SetBasicAuth(t.accountSID, t.authToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("twilio: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("twilio: status %d: %s", resp.StatusCode, raw)
	}
	return nil
}
