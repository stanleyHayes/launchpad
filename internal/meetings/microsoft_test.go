package meetings_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"launchpad/internal/meetings"
)

func TestMicrosoftCalendarValidateAndCreate(t *testing.T) {
	t.Parallel()
	client := meetings.NewMicrosoftCalendarClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Header.Get("Authorization") != "Bearer token" {
			t.Fatalf("missing bearer token")
		}
		if req.Method == http.MethodGet {
			return jsonResponse(http.StatusOK, map[string]any{"id": "calendar-1", "name": "Work calendar"}), nil
		}
		if req.Method != http.MethodPost || !strings.HasSuffix(req.URL.Path, "/me/calendar/events") {
			t.Fatalf("unexpected request %s %s", req.Method, req.URL.Path)
		}
		raw, _ := io.ReadAll(req.Body)
		if !strings.Contains(string(raw), `"subject":"Manager intro"`) {
			t.Fatalf("body = %s", raw)
		}
		return jsonResponse(http.StatusCreated, map[string]any{"id": "event-1"}), nil
	})})
	handle, err := client.Validate(context.Background(), "token")
	if err != nil || handle != "Work calendar" {
		t.Fatalf("validate: %q %v", handle, err)
	}
	id, err := client.CreateEvent(context.Background(), "token", meetings.CalendarEvent{
		Title: "Manager intro", StartsAt: time.Now(), DurationMin: 30,
	})
	if err != nil || id != "event-1" {
		t.Fatalf("create: %q %v", id, err)
	}
}
