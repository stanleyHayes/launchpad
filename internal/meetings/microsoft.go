package meetings

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"launchpad/pkg/safehttp"
)

const microsoftGraphBaseURL = "https://graph.microsoft.com/v1.0"

// MicrosoftCalendarClient implements delegated Microsoft Graph calendar
// access using Calendars.ReadWrite.
type MicrosoftCalendarClient struct {
	httpClient *http.Client
}

func NewMicrosoftCalendarClient(client *http.Client) *MicrosoftCalendarClient {
	if client == nil {
		client = safehttp.Client(calendarTimeout)
	}
	return &MicrosoftCalendarClient{httpClient: client}
}

func (c *MicrosoftCalendarClient) Validate(ctx context.Context, accessToken string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, microsoftGraphBaseURL+"/me/calendar", nil)
	if err != nil {
		return "", fmt.Errorf("build Microsoft calendar request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")
	var payload struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := doCalendarJSON(c.httpClient, req, &payload); err != nil {
		return "", err
	}
	if payload.Name != "" {
		return payload.Name, nil
	}
	if payload.ID == "" {
		return "", ErrInvalidCredential
	}
	return payload.ID, nil
}

func (c *MicrosoftCalendarClient) CreateEvent(ctx context.Context, accessToken string, event CalendarEvent) (string, error) {
	start := event.StartsAt.UTC()
	body := map[string]any{
		"subject":  event.Title,
		"location": map[string]string{"displayName": event.Location},
		"body":     map[string]string{"contentType": "text", "content": event.Description},
		"start":    map[string]string{"dateTime": start.Format("2006-01-02T15:04:05"), "timeZone": "UTC"},
		"end":      map[string]string{"dateTime": start.Add(time.Duration(event.DurationMin) * time.Minute).Format("2006-01-02T15:04:05"), "timeZone": "UTC"},
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("encode Microsoft event: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, microsoftGraphBaseURL+"/me/calendar/events", bytes.NewReader(raw))
	if err != nil {
		return "", fmt.Errorf("build Microsoft event request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	var payload struct {
		ID string `json:"id"`
	}
	if err := doCalendarJSON(c.httpClient, req, &payload); err != nil {
		return "", err
	}
	if payload.ID == "" {
		return "", ErrMissingEventID
	}
	return payload.ID, nil
}

func (c *MicrosoftCalendarClient) UpdateEvent(
	ctx context.Context,
	accessToken, eventID string,
	event CalendarEvent,
) error {
	start := event.StartsAt.UTC()
	body := map[string]any{
		"subject":  event.Title,
		"location": map[string]string{"displayName": event.Location},
		"start":    map[string]string{"dateTime": start.Format("2006-01-02T15:04:05"), "timeZone": "UTC"},
		"end": map[string]string{
			"dateTime": start.Add(time.Duration(event.DurationMin) * time.Minute).Format("2006-01-02T15:04:05"),
			"timeZone": "UTC",
		},
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("encode Microsoft event update: %w", err)
	}
	req, err := http.NewRequestWithContext(
		ctx, http.MethodPatch, microsoftGraphBaseURL+"/me/events/"+url.PathEscape(eventID), bytes.NewReader(raw),
	)
	if err != nil {
		return fmt.Errorf("build Microsoft event update: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	var payload map[string]any

	return doCalendarJSON(c.httpClient, req, &payload)
}
