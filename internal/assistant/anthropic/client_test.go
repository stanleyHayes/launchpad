package anthropic_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"launchpad/internal/assistant"
	"launchpad/internal/assistant/anthropic"
)

// roundTripFunc adapts a function to http.RoundTripper.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

func textBody(text string) string {
	return `{"content":[{"type":"text","text":"` + text + `"}],"stop_reason":"end_turn"}`
}

func clientWith(fn roundTripFunc) *anthropic.Client {
	return anthropic.NewClient(anthropic.Config{
		APIKey:     "test-key",
		Model:      "claude-opus-4-8",
		BaseURL:    "https://api.anthropic.test",
		HTTPClient: &http.Client{Transport: fn},
	})
}

func askInput() assistant.GenerateInput {
	return assistant.GenerateInput{
		Question: "How do I get VPN access?",
		Passages: []assistant.Passage{{Index: 1, DocumentTitle: "VPN Policy", Text: "Use the IT portal."}},
	}
}

func TestGenerateParsesTextAndSendsRequest(t *testing.T) {
	t.Parallel()

	var captured *http.Request

	client := clientWith(func(r *http.Request) (*http.Response, error) {
		captured = r

		return jsonResponse(http.StatusOK, textBody("Use the IT portal [1].")), nil
	})

	answer, err := client.Generate(context.Background(), askInput())
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	if answer.Refused || answer.Text != "Use the IT portal [1]." {
		t.Fatalf("unexpected answer: %+v", answer)
	}

	if captured.Method != http.MethodPost || captured.URL.Path != "/v1/messages" {
		t.Fatalf("unexpected request: %s %s", captured.Method, captured.URL.Path)
	}

	if captured.Header.Get("X-Api-Key") != "test-key" {
		t.Fatalf("missing api key header")
	}

	if captured.Header.Get("Anthropic-Version") != "2023-06-01" {
		t.Fatalf("missing anthropic-version header")
	}
}

func TestGenerateTreatsInsufficientMarkerAsRefusal(t *testing.T) {
	t.Parallel()

	client := clientWith(func(*http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, textBody("INSUFFICIENT_CONTEXT")), nil
	})

	answer, err := client.Generate(context.Background(), askInput())
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	if !answer.Refused || answer.Text != "" {
		t.Fatalf("expected refusal, got %+v", answer)
	}
}

func TestGenerateTreatsRefusalStopReasonAsRefusal(t *testing.T) {
	t.Parallel()

	client := clientWith(func(*http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"content":[],"stop_reason":"refusal"}`), nil
	})

	answer, err := client.Generate(context.Background(), askInput())
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	if !answer.Refused {
		t.Fatalf("expected refusal on refusal stop reason, got %+v", answer)
	}
}

func TestGenerateReturnsErrorOnNon2xx(t *testing.T) {
	t.Parallel()

	client := clientWith(func(*http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusBadRequest, `{"error":"bad"}`), nil
	})

	if _, err := client.Generate(context.Background(), askInput()); err == nil {
		t.Fatalf("expected error on non-2xx status")
	}
}
