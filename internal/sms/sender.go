// Package sms provides a small provider-agnostic HTTP SMS adapter.
package sms

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Config struct {
	APIKey  string
	From    string
	BaseURL string
}

type Sender struct {
	config Config
	client *http.Client
}

func NewSender(config Config) *Sender {
	config.BaseURL = strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	return &Sender{config: config, client: &http.Client{Timeout: 15 * time.Second}}
}

func (s *Sender) Configured() bool {
	return s.config.APIKey != "" && s.config.BaseURL != "" && s.config.From != ""
}

// Send uses a deliberately small JSON contract supported by an adapter or
// gateway: {"to","from","message"}. Provider credentials stay server-side.
func (s *Sender) Send(ctx context.Context, to, message string) error {
	if !s.Configured() {
		return fmt.Errorf("SMS provider is not configured")
	}
	payload, err := json.Marshal(map[string]string{
		"to": strings.TrimSpace(to), "from": s.config.From, "message": message,
	})
	if err != nil {
		return fmt.Errorf("encode SMS: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.config.BaseURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create SMS request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.config.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("send SMS: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("SMS provider returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}
