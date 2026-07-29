// Package webhook delivers notifications to Slack and Microsoft Teams incoming
// webhooks over stdlib net/http. It implements notifications.Dispatcher; the
// destination URLs are validated (https + host allowlist) by the notifications
// service before they reach here.
package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"launchpad/internal/notifications"
)

const (
	defaultTimeout   = 5 * time.Second
	maxResponseBytes = 1 << 16
)

var (
	_ notifications.Dispatcher = (*Dispatcher)(nil)

	errWebhookStatus = errors.New("webhook returned an error status")
)

// Dispatcher posts notifications to configured chat webhooks.
type Dispatcher struct {
	httpClient *http.Client
}

// NewDispatcher constructs a Dispatcher, applying a default HTTP client. The
// default client does not follow redirects, so an allowlisted webhook host
// cannot redirect delivery to an internal address (an SSRF bypass); a redirect
// is surfaced as a non-2xx status and treated as a failed delivery.
func NewDispatcher(client *http.Client) *Dispatcher {
	if client == nil {
		client = &http.Client{
			Timeout: defaultTimeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	}

	return &Dispatcher{httpClient: client}
}

// Dispatch delivers to every configured channel, joining any per-channel errors.
func (d *Dispatcher) Dispatch(
	ctx context.Context,
	config notifications.ChannelConfig,
	notification notifications.Notification,
) error {
	var errs []error

	if config.SlackWebhookURL != "" {
		if err := d.post(ctx, config.SlackWebhookURL, slackPayload(notification)); err != nil {
			errs = append(errs, fmt.Errorf("slack: %w", err))
		}
	}

	if config.TeamsWebhookURL != "" {
		if err := d.post(ctx, config.TeamsWebhookURL, teamsPayload(notification)); err != nil {
			errs = append(errs, fmt.Errorf("teams: %w", err))
		}
	}

	return errors.Join(errs...)
}

func (d *Dispatcher) post(ctx context.Context, endpoint string, payload map[string]any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", redactURL(err))
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("post webhook: %w", redactURL(err))
	}
	defer func() { _ = resp.Body.Close() }()

	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxResponseBytes))

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("%w: %d", errWebhookStatus, resp.StatusCode)
	}

	return nil
}

// redactURL strips the URL from a *url.Error so a secret webhook URL never
// reaches logs; the operation and underlying cause are preserved.
func redactURL(err error) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return fmt.Errorf("%s: %w", urlErr.Op, urlErr.Err)
	}

	return err
}

func slackPayload(notification notifications.Notification) map[string]any {
	return map[string]any{
		"text": "*" + notification.Title + "*\n" + notification.Body,
	}
}

func teamsPayload(notification notifications.Notification) map[string]any {
	return map[string]any{
		"@type":    "MessageCard",
		"@context": "https://schema.org/extensions",
		"summary":  notification.Title,
		"title":    notification.Title,
		"text":     notification.Body,
	}
}
