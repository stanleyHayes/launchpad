package observability_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"launchpad/pkg/observability"
)

// expositionLine matches a single 0.0.4 sample: metric name, optional brace
// labels, a numeric value.
var expositionLine = regexp.MustCompile(`^[a-z_:][a-zA-Z0-9_:]*(\{[a-zA-Z0-9_="\\/{}.,:\- ]*\})? -?[0-9]+(\.[0-9]+)?$`)

func newRouterWithMetrics(reg *observability.Registry) *chi.Mux {
	router := chi.NewRouter()
	router.Use(reg.Middleware)
	router.Get("/api/v1/items/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	return router
}

func scrape(t *testing.T, reg *observability.Registry) (string, string) {
	t.Helper()

	recorder := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/metrics", nil)
	reg.Handler().ServeHTTP(recorder, req)

	return recorder.Body.String(), recorder.Header().Get("Content-Type")
}

func TestMiddlewareRecordsByRouteTemplate(t *testing.T) {
	t.Parallel()

	reg := observability.NewRegistry()
	router := newRouterWithMetrics(reg)

	for range 3 {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/items/abc-123", nil)
		router.ServeHTTP(recorder, req)
	}

	body, _ := scrape(t, reg)

	want := `http_requests_total{method="GET",path_template="/api/v1/items/{id}",status="201"} 3`
	if !strings.Contains(body, want) {
		t.Errorf("exposition missing %q\n%s", want, body)
	}

	if !strings.Contains(body, `http_request_duration_seconds_count{path_template="/api/v1/items/{id}"} 3`) {
		t.Errorf("exposition missing duration count\n%s", body)
	}

	// The raw request path must never become a label value: that would be
	// unbounded cardinality driven by client input.
	if strings.Contains(body, "abc-123") {
		t.Errorf("exposition leaks raw request path\n%s", body)
	}
}

func TestMiddlewareUnmatchedRouteUsesConstant(t *testing.T) {
	t.Parallel()

	reg := observability.NewRegistry()
	router := newRouterWithMetrics(reg)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/no/such/route/12345", nil)
	router.ServeHTTP(recorder, req)

	body, _ := scrape(t, reg)

	if !strings.Contains(body, `path_template="unmatched"`) {
		t.Errorf("exposition missing unmatched route label\n%s", body)
	}

	if strings.Contains(body, "12345") {
		t.Errorf("exposition leaks unmatched raw path\n%s", body)
	}
}

func TestExpositionFormatValid(t *testing.T) {
	t.Parallel()

	reg := observability.NewRegistry()
	router := newRouterWithMetrics(reg)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/items/1", nil)
	router.ServeHTTP(recorder, req)

	body, contentType := scrape(t, reg)

	if contentType != "text/plain; version=0.0.4; charset=utf-8" {
		t.Errorf("content type = %q, want text/plain version 0.0.4", contentType)
	}

	for line := range strings.SplitSeq(strings.TrimRight(body, "\n"), "\n") {
		if strings.HasPrefix(line, "# HELP ") || strings.HasPrefix(line, "# TYPE ") {
			continue
		}

		if !expositionLine.MatchString(line) {
			t.Errorf("line does not match 0.0.4 sample format: %q", line)
		}
	}

	for _, want := range []string{
		"# TYPE http_requests_total counter",
		"# TYPE http_request_duration_seconds summary",
		"# TYPE process_goroutines gauge",
		"# TYPE process_uptime_seconds gauge",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("exposition missing %q\n%s", want, body)
		}
	}
}

func TestDependencyGauges(t *testing.T) {
	t.Parallel()

	okPing := func(_ context.Context) error { return nil }
	failPing := func(_ context.Context) error { return errors.New("down") }

	reg := observability.NewRegistry().WithPingChecks(okPing, failPing)
	body, _ := scrape(t, reg)

	if !strings.Contains(body, `dependency_up{dependency="mongo"} 1`) {
		t.Errorf("exposition missing mongo up gauge\n%s", body)
	}

	if !strings.Contains(body, `dependency_up{dependency="redis"} 0`) {
		t.Errorf("exposition missing redis down gauge\n%s", body)
	}
}

func TestDependencyGaugesOmittedWhenUnwired(t *testing.T) {
	t.Parallel()

	reg := observability.NewRegistry()
	body, _ := scrape(t, reg)

	if strings.Contains(body, "dependency_up{") {
		t.Errorf("unwired registry must not emit dependency gauges\n%s", body)
	}
}

func TestCaptureError(t *testing.T) {
	t.Parallel()

	// Restore the no-op default afterwards; the sink is process-wide.
	t.Cleanup(func() { observability.SetErrorSink(nil) })

	// No sink installed: must not panic.
	observability.CaptureError(t.Context(), errors.New("boom"))
	observability.CaptureError(t.Context(), nil)

	var captured error

	observability.SetErrorSink(func(_ context.Context, err error) { captured = err })

	want := errors.New("reported")
	observability.CaptureError(t.Context(), want)

	if !errors.Is(captured, want) {
		t.Errorf("captured = %v, want %v", captured, want)
	}

	observability.CaptureError(t.Context(), nil)

	if !errors.Is(captured, want) {
		t.Errorf("nil error must not be captured, captured = %v", captured)
	}
}
