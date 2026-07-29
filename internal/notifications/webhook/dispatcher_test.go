package webhook_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"launchpad/internal/notifications"
	"launchpad/internal/notifications/webhook"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type capturedRequest struct {
	url  string
	body map[string]any
}

func recordingClient(status int, captured *[]capturedRequest) *http.Client {
	return &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		raw, _ := io.ReadAll(r.Body)

		var body map[string]any

		_ = json.Unmarshal(raw, &body)

		*captured = append(*captured, capturedRequest{url: r.URL.String(), body: body})

		return &http.Response{
			StatusCode: status,
			Body:       io.NopCloser(strings.NewReader("ok")),
			Header:     make(http.Header),
		}, nil
	})}
}

func notification() notifications.Notification {
	return notifications.Notification{Title: "Task assigned", Body: "Complete your laptop setup."}
}

func TestDispatchPostsSlackAndTeams(t *testing.T) {
	t.Parallel()

	var captured []capturedRequest

	dispatcher := webhook.NewDispatcher(recordingClient(http.StatusOK, &captured))

	err := dispatcher.Dispatch(context.Background(), notifications.ChannelConfig{
		SlackWebhookURL: "https://hooks.slack.com/services/T/B/X",
		TeamsWebhookURL: "https://tenant.webhook.office.com/webhookb2/abc",
	}, notification())
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	if len(captured) != 2 {
		t.Fatalf("expected 2 webhook posts, got %d", len(captured))
	}

	bySlack := captured[0]
	if !strings.Contains(bySlack.url, "hooks.slack.com") {
		t.Fatalf("first post should target slack, got %s", bySlack.url)
	}

	if _, ok := bySlack.body["text"]; !ok {
		t.Fatalf("slack payload missing text field: %v", bySlack.body)
	}

	teams := captured[1]
	if teams.body["@type"] != "MessageCard" {
		t.Fatalf("teams payload should be a MessageCard, got %v", teams.body)
	}
}

func TestDispatchReturnsErrorOnNon2xx(t *testing.T) {
	t.Parallel()

	var captured []capturedRequest

	dispatcher := webhook.NewDispatcher(recordingClient(http.StatusInternalServerError, &captured))

	err := dispatcher.Dispatch(context.Background(), notifications.ChannelConfig{
		SlackWebhookURL: "https://hooks.slack.com/services/T/B/X",
	}, notification())
	if err == nil {
		t.Fatalf("expected error on non-2xx webhook response")
	}
}

func TestDefaultClientDoesNotFollowRedirects(t *testing.T) {
	t.Parallel()

	reachedInternal := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/internal" {
			reachedInternal = true

			w.WriteHeader(http.StatusOK)

			return
		}

		http.Redirect(w, r, "/internal", http.StatusFound)
	}))
	defer server.Close()

	dispatcher := webhook.NewDispatcher(nil)

	err := dispatcher.Dispatch(
		context.Background(),
		notifications.ChannelConfig{SlackWebhookURL: server.URL + "/hook"},
		notification(),
	)
	if err == nil {
		t.Fatalf("expected a redirect to surface as a failed delivery")
	}

	if reachedInternal {
		t.Fatalf("dispatcher followed a redirect (SSRF bypass)")
	}
}

func TestDispatchNoChannelsNoRequests(t *testing.T) {
	t.Parallel()

	var captured []capturedRequest

	dispatcher := webhook.NewDispatcher(recordingClient(http.StatusOK, &captured))

	if err := dispatcher.Dispatch(context.Background(), notifications.ChannelConfig{}, notification()); err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	if len(captured) != 0 {
		t.Fatalf("expected no posts when no channels configured, got %d", len(captured))
	}
}
