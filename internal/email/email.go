// Package email provides outbound transactional email delivery through a
// Resend-compatible HTTP API. When no provider key is configured it degrades
// to a log-only sender, so local development and tests need no external
// account.
package email

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"launchpad/pkg/safehttp"
)

const (
	// DefaultBaseURL is the Resend-compatible API endpoint used when no
	// EMAIL_BASE_URL override is configured.
	DefaultBaseURL = "https://api.resend.com"
	sendTimeout    = 10 * time.Second
)

// ErrDeliveryFailed indicates the provider rejected the message.
var ErrDeliveryFailed = errors.New("email delivery failed")

// Sender sends transactional email. The html body may contain secrets
// (invitation/reset links); implementations must never log it.
type Sender interface {
	Send(ctx context.Context, to, subject, html string) error
}

// Config holds email provider settings.
type Config struct {
	// APIKey is the provider bearer key (EMAIL_API_KEY). Empty disables
	// delivery.
	APIKey string
	// From is the sender address (EMAIL_FROM). Required for delivery.
	From string
	// BaseURL is the provider API base (EMAIL_BASE_URL); DefaultBaseURL when
	// empty.
	BaseURL string
}

// NewSender returns an HTTP API sender when an API key and sender address are
// configured, and a log-only sender otherwise.
//
//nolint:ireturn // factory picks the configured delivery implementation.
func NewSender(cfg Config) Sender {
	if cfg.APIKey == "" || cfg.From == "" {
		return LogSender{}
	}

	return NewAPISender(cfg, safehttp.Client(sendTimeout))
}

// APISender delivers email through a Resend-compatible HTTP API
// (POST {baseURL}/emails with a bearer key).
type APISender struct {
	cfg    Config
	client *http.Client
}

// NewAPISender constructs an APISender with the given HTTP client. Production
// wiring passes safehttp.Client(10s); tests may inject a plain client.
func NewAPISender(cfg Config, client *http.Client) *APISender {
	return &APISender{cfg: cfg, client: client}
}

type sendRequest struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	HTML    string   `json:"html"`
}

// Send posts the message to the provider and treats any non-2xx status as a
// delivery failure. The response body is never logged (providers may echo
// request content).
func (s *APISender) Send(ctx context.Context, to, subject, html string) error {
	baseURL := strings.TrimRight(s.cfg.BaseURL, "/")
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}

	payload, err := json.Marshal(sendRequest{From: s.cfg.From, To: []string{to}, Subject: subject, HTML: html})
	if err != nil {
		return fmt.Errorf("encode email payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/emails", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build email request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+s.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("send email: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("%w: provider status %d", ErrDeliveryFailed, resp.StatusCode)
	}

	return nil
}

// LogSender drops the message and only logs the delivery metadata. It is used
// when no provider is configured; it never logs the body, which may contain
// secrets such as reset links.
type LogSender struct{}

// Send logs the recipient and subject and reports success.
func (LogSender) Send(ctx context.Context, to, subject, _ string) error {
	slog.InfoContext(ctx, "email not delivered (no provider configured)", "to", to, "subject", subject)

	return nil
}
