package meetings

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"launchpad/pkg/safehttp"
)

const (
	// calendarTimeout bounds a single provider call.
	calendarTimeout = 10 * time.Second

	googleCalendarBaseURL = "https://www.googleapis.com/calendar/v3"

	maxCalendarBodyBytes = 1 << 20 // 1 MiB; event payloads are small
)

// CalendarEvent is the provider-agnostic event payload derived from a meeting.
type CalendarEvent struct {
	Title       string
	Location    string
	Description string
	StartsAt    time.Time
	DurationMin int
}

// CalendarClient creates events on a tenant's calendar provider using an
// access token from the tenant's stored calendar connection. The token is
// passed per call (never stored on the client) so OAuth token refresh can
// slot in later without changing this interface.
type CalendarClient interface {
	// Validate checks an access token and returns the provider-side account
	// handle (e.g. the primary calendar id, usually the user's email).
	Validate(ctx context.Context, accessToken string) (string, error)
	// CreateEvent creates an event on the primary calendar and returns the
	// provider's event id, stored as the meeting's calendarEventRef.
	CreateEvent(ctx context.Context, accessToken string, event CalendarEvent) (string, error)
}

type CalendarEventUpdater interface {
	UpdateEvent(ctx context.Context, accessToken, eventID string, event CalendarEvent) error
}

// GoogleCalendarClient implements CalendarClient against the Google Calendar
// v3 API with a manually supplied OAuth2 access token.
type GoogleCalendarClient struct {
	httpClient *http.Client
}

// NewGoogleCalendarClient constructs a GoogleCalendarClient. When client is
// nil it applies an SSRF-hardened default; tests inject a stubbed transport
// instead.
func NewGoogleCalendarClient(client *http.Client) *GoogleCalendarClient {
	if client == nil {
		client = safehttp.Client(calendarTimeout)
	}

	return &GoogleCalendarClient{httpClient: client}
}

// Validate calls GET /users/me/calendarList with the token and returns the
// primary calendar id as the account handle. A 401/403 response means the
// credential was rejected: ErrInvalidCredential.
func (c *GoogleCalendarClient) Validate(ctx context.Context, accessToken string) (string, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		googleCalendarBaseURL+"/users/me/calendarList?maxResults=50",
		nil,
	)
	if err != nil {
		return "", fmt.Errorf("build calendar list request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	var payload struct {
		Items []struct {
			ID      string `json:"id"`
			Primary bool   `json:"primary"`
		} `json:"items"`
	}
	if err := doCalendarJSON(c.httpClient, req, &payload); err != nil {
		return "", err
	}

	handle := ""
	for _, item := range payload.Items {
		if item.Primary {
			handle = item.ID

			break
		}
	}

	if handle == "" && len(payload.Items) > 0 {
		handle = payload.Items[0].ID
	}

	if handle == "" {
		return "", ErrInvalidCredential
	}

	return handle, nil
}

// CreateEvent inserts an event on the primary calendar and returns its id.
func (c *GoogleCalendarClient) CreateEvent(
	ctx context.Context,
	accessToken string,
	event CalendarEvent,
) (string, error) {
	startsAt := event.StartsAt.UTC()

	body := map[string]any{
		"summary":  event.Title,
		"location": event.Location,
		"start": map[string]string{
			"dateTime": startsAt.Format(time.RFC3339),
			"timeZone": "UTC",
		},
		"end": map[string]string{
			"dateTime": startsAt.Add(time.Duration(event.DurationMin) * time.Minute).Format(time.RFC3339),
			"timeZone": "UTC",
		},
	}
	if event.Description != "" {
		body["description"] = event.Description
	}

	raw, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("encode calendar event: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		googleCalendarBaseURL+"/calendars/primary/events",
		bytes.NewReader(raw),
	)
	if err != nil {
		return "", fmt.Errorf("build calendar event request: %w", err)
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

func (c *GoogleCalendarClient) UpdateEvent(
	ctx context.Context,
	accessToken, eventID string,
	event CalendarEvent,
) error {
	startsAt := event.StartsAt.UTC()
	body := map[string]any{
		"summary": event.Title, "location": event.Location,
		"start": map[string]string{"dateTime": startsAt.Format(time.RFC3339), "timeZone": "UTC"},
		"end": map[string]string{
			"dateTime": startsAt.Add(time.Duration(event.DurationMin) * time.Minute).Format(time.RFC3339),
			"timeZone": "UTC",
		},
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("encode calendar event update: %w", err)
	}
	req, err := http.NewRequestWithContext(
		ctx, http.MethodPatch,
		googleCalendarBaseURL+"/calendars/primary/events/"+url.PathEscape(eventID),
		bytes.NewReader(raw),
	)
	if err != nil {
		return fmt.Errorf("build calendar event update: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	var payload map[string]any

	return doCalendarJSON(c.httpClient, req, &payload)
}

// doCalendarJSON executes req and decodes a 2xx JSON body into dst. 401/403
// map to ErrInvalidCredential; the response body is never included in errors
// so provider messages cannot leak tokens or payload details into logs.
func doCalendarJSON(client *http.Client, req *http.Request, dst any) error {
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("send calendar request: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxCalendarBodyBytes))
	if err != nil {
		return fmt.Errorf("read calendar response: %w", err)
	}

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return ErrInvalidCredential
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("%w: status %d", ErrCalendarProvider, resp.StatusCode)
	}

	if err := json.Unmarshal(body, dst); err != nil {
		return fmt.Errorf("decode calendar response: %w", err)
	}

	return nil
}
