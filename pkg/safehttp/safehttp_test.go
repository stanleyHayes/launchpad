package safehttp_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"launchpad/pkg/safehttp"
)

func TestClientBlocksLoopbackAddress(t *testing.T) {
	t.Parallel()

	// httptest servers listen on 127.0.0.1, which the guard must refuse.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}

	resp, err := safehttp.Client(2 * time.Second).Do(req)
	if resp != nil {
		_ = resp.Body.Close()
	}

	if !errors.Is(err, safehttp.ErrBlockedAddress) {
		t.Fatalf("expected ErrBlockedAddress dialing loopback, got %v", err)
	}
}

func TestClientDoesNotFollowRedirects(t *testing.T) {
	t.Parallel()

	// A public host cannot be reached from the test env, but we can assert the
	// no-redirect policy directly via CheckRedirect.
	client := safehttp.Client(2 * time.Second)
	if client.CheckRedirect == nil {
		t.Fatal("expected a CheckRedirect policy that refuses redirects")
	}

	if err := client.CheckRedirect(nil, nil); !errors.Is(err, http.ErrUseLastResponse) {
		t.Fatalf("expected redirects to be refused, got %v", err)
	}
}
