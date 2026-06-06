package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// EmailSender delivers a single email. Implement for any provider.
type EmailSender interface {
	SendEmail(ctx context.Context, to, subject, body string) error
}

type sendgridEmailSender struct {
	apiKey    string
	fromEmail string
	fromName  string
	client    *http.Client
}

// NewSendGridEmailSender returns an EmailSender backed by the SendGrid v3 API.
func NewSendGridEmailSender(apiKey, fromEmail, fromName string) EmailSender {
	return &sendgridEmailSender{
		apiKey:    apiKey,
		fromEmail: fromEmail,
		fromName:  fromName,
		client:    &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *sendgridEmailSender) SendEmail(ctx context.Context, to, subject, body string) error {
	payload := map[string]any{
		"personalizations": []map[string]any{
			{"to": []map[string]string{{"email": to}}},
		},
		"from":    map[string]string{"email": s.fromEmail, "name": s.fromName},
		"subject": subject,
		"content": []map[string]string{{"type": "text/plain", "value": body}},
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("sendgrid: marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.sendgrid.com/v3/mail/send", bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("sendgrid: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("sendgrid: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("sendgrid: unexpected status %d", resp.StatusCode)
	}
	return nil
}
