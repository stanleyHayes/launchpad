package audit_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	chimw "github.com/go-chi/chi/v5/middleware"

	"launchpad/internal/audit"
)

func TestMiddlewarePopulatesContextAndRecordCapturesIt(t *testing.T) {
	t.Parallel()

	repo := &fakeAuditRepo{}
	svc := audit.NewService(repo)

	handler := audit.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := svc.Record(r.Context(), nil, "user-1", "auth.login", "user", "user-1", nil); err != nil {
			t.Errorf("record: %v", err)
		}

		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/employees", nil)
	req.RemoteAddr = "203.0.113.9:55123"
	req.Header.Set("User-Agent", "launchpad-test/1.0")

	ctx := context.WithValue(req.Context(), chimw.RequestIDKey, "req-abc-123")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected handler to run, got status %d", rec.Code)
	}

	if len(repo.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(repo.events))
	}

	event := repo.events[0]
	if event.IP != "203.0.113.9" {
		t.Fatalf("expected client IP without port, got %q", event.IP)
	}

	if event.UserAgent != "launchpad-test/1.0" {
		t.Fatalf("expected user agent captured, got %q", event.UserAgent)
	}

	if event.RequestID != "req-abc-123" {
		t.Fatalf("expected chi request id captured, got %q", event.RequestID)
	}

	if event.Result != audit.ResultSuccess {
		t.Fatalf("expected result %q, got %q", audit.ResultSuccess, event.Result)
	}

	if event.FailureReason != "" {
		t.Fatalf("expected no failure reason on success, got %q", event.FailureReason)
	}
}

func TestRecordCapturesBareIPWithoutPort(t *testing.T) {
	t.Parallel()

	repo := &fakeAuditRepo{}
	svc := audit.NewService(repo)

	handler := audit.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := svc.Record(r.Context(), nil, "user-1", "auth.login", "user", "user-1", nil); err != nil {
			t.Errorf("record: %v", err)
		}

		w.WriteHeader(http.StatusNoContent)
	}))

	// Post-RealIP RemoteAddr is a bare IP with no port.
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/employees", nil)
	req.RemoteAddr = "198.51.100.4"

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if repo.events[0].IP != "198.51.100.4" {
		t.Fatalf("expected bare IP preserved, got %q", repo.events[0].IP)
	}
}

func TestRecordWithoutRequestContextLeavesFieldsEmpty(t *testing.T) {
	t.Parallel()

	repo := &fakeAuditRepo{}
	svc := audit.NewService(repo)

	if err := svc.Record(context.Background(), nil, "user-1", "journey.published", "journey", "jrn-1", nil); err != nil {
		t.Fatalf("record: %v", err)
	}

	event := repo.events[0]
	if event.IP != "" || event.UserAgent != "" || event.RequestID != "" {
		t.Fatalf("expected empty request metadata without Middleware, got %+v", event)
	}

	if event.Result != audit.ResultSuccess {
		t.Fatalf("expected result default %q, got %q", audit.ResultSuccess, event.Result)
	}
}

func TestRecordResultCapturesFailure(t *testing.T) {
	t.Parallel()

	repo := &fakeAuditRepo{}
	svc := audit.NewService(repo)

	ctx := audit.WithRequestContext(context.Background(), audit.RequestContext{
		IP:        "192.0.2.10",
		UserAgent: "cli/2.0",
		RequestID: "req-fail-1",
	})

	orgID := "org-1"

	err := svc.RecordResult(
		ctx, &orgID, "user-1", "organization.suspended", "organization", "org-1",
		audit.ResultFailure, "action_rejected", nil,
	)
	if err != nil {
		t.Fatalf("record result: %v", err)
	}

	event := repo.events[0]
	if event.Result != audit.ResultFailure || event.FailureReason != "action_rejected" {
		t.Fatalf("expected failure result with reason, got %+v", event)
	}

	if event.IP != "192.0.2.10" || event.UserAgent != "cli/2.0" || event.RequestID != "req-fail-1" {
		t.Fatalf("expected request metadata captured, got %+v", event)
	}
}

func TestRecordResultDefaultsEmptyResultToSuccess(t *testing.T) {
	t.Parallel()

	repo := &fakeAuditRepo{}
	svc := audit.NewService(repo)

	err := svc.RecordResult(
		context.Background(), nil, "user-1", "auth.login", "user", "user-1", "", "", nil,
	)
	if err != nil {
		t.Fatalf("record result: %v", err)
	}

	if repo.events[0].Result != audit.ResultSuccess {
		t.Fatalf("expected empty result to default to %q, got %q", audit.ResultSuccess, repo.events[0].Result)
	}
}
