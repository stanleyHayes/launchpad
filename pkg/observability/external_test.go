package observability_test

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"launchpad/pkg/observability"
)

func TestExternalExporterSendsTraceAndServerError(t *testing.T) {
	t.Parallel()
	var traces, errors atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/traces" {
			traces.Add(1)
		} else {
			errors.Add(1)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	exporter := observability.NewExternalExporter(server.URL+"/errors", server.URL+"/traces", "token")
	handler := exporter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	request := httptest.NewRequest(http.MethodGet, "/broken", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if traces.Load() != 1 || errors.Load() != 1 {
		t.Fatalf("exports = traces %d errors %d", traces.Load(), errors.Load())
	}
	if response.Header().Get("traceparent") == "" {
		t.Fatal("traceparent response header is missing")
	}
}
