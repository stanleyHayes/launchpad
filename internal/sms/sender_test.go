package sms_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"launchpad/internal/sms"
)

func TestSenderPostsProviderContract(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("authorization = %q", r.Header.Get("Authorization"))
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode body: %v", err)
		}
		if body["to"] != "+233200000000" || body["from"] != "LaunchPad" || body["message"] != "Welcome" {
			t.Errorf("body = %#v", body)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	sender := sms.NewSender(sms.Config{APIKey: "secret", From: "LaunchPad", BaseURL: server.URL})
	if err := sender.Send(context.Background(), "+233200000000", "Welcome"); err != nil {
		t.Fatalf("Send: %v", err)
	}
}
