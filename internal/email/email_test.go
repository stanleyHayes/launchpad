package email_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"launchpad/internal/email"
)

func TestNewSenderFallsBackToLogSenderWithoutKey(t *testing.T) {
	t.Parallel()

	for _, cfg := range []email.Config{
		{},
		{APIKey: "key-only"},
		{From: "from-only@example.com"},
	} {
		if _, ok := email.NewSender(cfg).(email.LogSender); !ok {
			t.Fatalf("cfg %+v: got %T, want LogSender", cfg, email.NewSender(cfg))
		}
	}
}

func TestLogSenderIsNoOp(t *testing.T) {
	t.Parallel()

	if err := (email.LogSender{}).Send(context.Background(), "to@example.com", "subject", "secret body"); err != nil {
		t.Fatalf("log sender returned %v, want nil", err)
	}
}

func TestAPISenderPostsResendPayload(t *testing.T) {
	t.Parallel()

	var (
		authHeader string
		body       map[string]any
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/emails" || r.Method != http.MethodPost {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}

		authHeader = r.Header.Get("Authorization")

		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}

		if err := json.Unmarshal(raw, &body); err != nil {
			t.Errorf("decode body: %v", err)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sender := email.NewAPISender(email.Config{
		APIKey: "test-key", From: "noreply@example.com", BaseURL: server.URL,
	}, server.Client())

	err := sender.Send(context.Background(), "user@example.com", "Hello", "<p>Hi</p>")
	if err != nil {
		t.Fatalf("send: %v", err)
	}

	if authHeader != "Bearer test-key" {
		t.Fatalf("authorization header = %q", authHeader)
	}

	if body["from"] != "noreply@example.com" || body["subject"] != "Hello" || body["html"] != "<p>Hi</p>" {
		t.Fatalf("payload mismatch: %v", body)
	}

	to, ok := body["to"].([]any)
	if !ok || len(to) != 1 || to[0] != "user@example.com" {
		t.Fatalf("to mismatch: %v", body["to"])
	}
}

func TestAPISenderTreatsNon2xxAsFailure(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	sender := email.NewAPISender(email.Config{
		APIKey: "bad-key", From: "noreply@example.com", BaseURL: server.URL,
	}, server.Client())

	err := sender.Send(context.Background(), "user@example.com", "Hello", "<p>Hi</p>")
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("got %v, want a 401 delivery error", err)
	}
}
