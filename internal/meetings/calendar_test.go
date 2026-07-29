package meetings_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"launchpad/internal/meetings"
)

// roundTripFunc adapts a function to http.RoundTripper for stubbed
// transports.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func jsonResponse(status int, payload any) *http.Response {
	raw, _ := json.Marshal(payload)

	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(string(raw))),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
}

func TestGoogleValidateReturnsPrimaryCalendarHandle(t *testing.T) {
	t.Parallel()

	var gotAuth string

	client := meetings.NewGoogleCalendarClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			gotAuth = req.Header.Get("Authorization")

			if req.Method != http.MethodGet || !strings.Contains(req.URL.Path, "/users/me/calendarList") {
				t.Fatalf("unexpected request %s %s", req.Method, req.URL)
			}

			return jsonResponse(http.StatusOK, map[string]any{
				"items": []map[string]any{
					{"id": "secondary@example.com"},
					{"id": "primary@example.com", "primary": true},
				},
			}), nil
		}),
	})

	handle, err := client.Validate(context.Background(), "token-123")
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}

	if handle != "primary@example.com" {
		t.Fatalf("handle = %q, want the primary calendar id", handle)
	}

	if gotAuth != "Bearer token-123" {
		t.Fatalf("authorization header = %q", gotAuth)
	}
}

func TestGoogleValidateRejectsBadCredential(t *testing.T) {
	t.Parallel()

	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		client := meetings.NewGoogleCalendarClient(&http.Client{
			Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return jsonResponse(status, map[string]any{"error": "invalid_token"}), nil
			}),
		})

		if _, err := client.Validate(context.Background(), "bad-token"); !errors.Is(err, meetings.ErrInvalidCredential) {
			t.Fatalf("Validate(%d) error = %v, want ErrInvalidCredential", status, err)
		}
	}
}

func TestGoogleCreateEventReturnsEventID(t *testing.T) {
	t.Parallel()

	var gotBody map[string]any

	client := meetings.NewGoogleCalendarClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method != http.MethodPost || !strings.HasSuffix(req.URL.Path, "/calendars/primary/events") {
				t.Fatalf("unexpected request %s %s", req.Method, req.URL)
			}

			if err := json.NewDecoder(req.Body).Decode(&gotBody); err != nil {
				t.Fatalf("decode request body: %v", err)
			}

			return jsonResponse(http.StatusOK, map[string]any{"id": "event-42"}), nil
		}),
	})

	startsAt := time.Date(2030, 1, 15, 10, 0, 0, 0, time.UTC)

	ref, err := client.CreateEvent(context.Background(), "token-123", meetings.CalendarEvent{
		Title:       "Manager intro",
		Location:    "Room 4",
		Description: "",
		StartsAt:    startsAt,
		DurationMin: 45,
	})
	if err != nil {
		t.Fatalf("CreateEvent: %v", err)
	}

	if ref != "event-42" {
		t.Fatalf("ref = %q, want event-42", ref)
	}

	if gotBody["summary"] != "Manager intro" || gotBody["location"] != "Room 4" {
		t.Fatalf("event body = %+v", gotBody)
	}

	end, ok := gotBody["end"].(map[string]any)
	if !ok || end["dateTime"] != "2030-01-15T10:45:00Z" {
		t.Fatalf("event end = %+v, want start + 45 minutes", gotBody["end"])
	}
}

func TestGoogleCreateEventMapsErrorStatuses(t *testing.T) {
	t.Parallel()

	unauthorized := meetings.NewGoogleCalendarClient(&http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusUnauthorized, map[string]any{}), nil
		}),
	})

	_, err := unauthorized.CreateEvent(context.Background(), "bad-token", meetings.CalendarEvent{
		Title:       "Sync",
		StartsAt:    time.Now(),
		DurationMin: 30,
	})
	if !errors.Is(err, meetings.ErrInvalidCredential) {
		t.Fatalf("CreateEvent(401) error = %v, want ErrInvalidCredential", err)
	}

	serverError := meetings.NewGoogleCalendarClient(&http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusInternalServerError, map[string]any{}), nil
		}),
	})

	if _, err := serverError.CreateEvent(context.Background(), "token", meetings.CalendarEvent{
		Title:       "Sync",
		StartsAt:    time.Now(),
		DurationMin: 30,
	}); err == nil || errors.Is(err, meetings.ErrInvalidCredential) {
		t.Fatalf("CreateEvent(500) error = %v, want a non-credential error", err)
	}
}
