package observability

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
)

// ExternalExporter is a lightweight vendor-neutral JSON hook for error
// tracking and distributed tracing gateways. Both endpoints are optional.
type ExternalExporter struct {
	errorURL string
	traceURL string
	token    string
	client   *http.Client
}

func requestID(ctx context.Context) string { return chimw.GetReqID(ctx) }

func routePattern(r *http.Request) string {
	if pattern := chi.RouteContext(r.Context()).RoutePattern(); pattern != "" {
		return pattern
	}
	return "unmatched"
}

func NewExternalExporter(errorURL, traceURL, token string) *ExternalExporter {
	return &ExternalExporter{
		errorURL: strings.TrimSpace(errorURL), traceURL: strings.TrimSpace(traceURL),
		token: strings.TrimSpace(token), client: &http.Client{Timeout: 3 * time.Second},
	}
}

func (e *ExternalExporter) ErrorSink(ctx context.Context, err error) {
	if e.errorURL == "" || err == nil {
		return
	}
	_ = e.post(ctx, e.errorURL, map[string]any{
		"timestamp": time.Now().UTC(), "message": err.Error(), "requestId": requestID(ctx),
	})
}

// Middleware emits W3C traceparent and exports one server span per request.
func (e *ExternalExporter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traceID, parentSpanID := parseTraceparent(r.Header.Get("traceparent"))
		if traceID == "" {
			traceID = randomHex(16)
		}
		spanID := randomHex(8)
		started := time.Now().UTC()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		w.Header().Set("traceparent", "00-"+traceID+"-"+spanID+"-01")
		next.ServeHTTP(recorder, r)

		if recorder.status >= http.StatusInternalServerError {
			e.ErrorSink(r.Context(), fmt.Errorf("%s %s returned %d", r.Method, r.URL.Path, recorder.status))
		}
		if e.traceURL != "" {
			_ = e.post(r.Context(), e.traceURL, map[string]any{
				"traceId": traceID, "spanId": spanID, "parentSpanId": parentSpanID,
				"name": r.Method + " " + routePattern(r), "kind": "server",
				"startedAt": started, "durationMs": time.Since(started).Milliseconds(),
				"statusCode": recorder.status, "requestId": requestID(r.Context()),
			})
		}
	})
}

func (e *ExternalExporter) post(ctx context.Context, endpoint string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	exportCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(exportCtx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if e.token != "" {
		req.Header.Set("Authorization", "Bearer "+e.token)
	}
	resp, err := e.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("observability exporter returned %d", resp.StatusCode)
	}
	return nil
}

func parseTraceparent(value string) (string, string) {
	parts := strings.Split(value, "-")
	if len(parts) != 4 || len(parts[1]) != 32 || len(parts[2]) != 16 {
		return "", ""
	}
	if _, err := hex.DecodeString(parts[1] + parts[2]); err != nil {
		return "", ""
	}
	return parts[1], parts[2]
}

func randomHex(bytesCount int) string {
	buffer := make([]byte, bytesCount)
	if _, err := rand.Read(buffer); err != nil {
		return strings.Repeat("0", bytesCount*2)
	}
	return hex.EncodeToString(buffer)
}
