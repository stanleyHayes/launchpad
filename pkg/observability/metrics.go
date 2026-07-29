// Package observability provides hand-rolled Prometheus text exposition
// (format version 0.0.4) with no client dependency: a registry of HTTP
// request counters and duration summaries, process gauges, and dependency
// ping gauges, plus the middleware that records per-request metrics.
package observability

import (
	"context"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
)

// dependencyPingTimeout bounds dependency pings during a /metrics scrape so a
// hung Mongo or Redis cannot stall the scrape (and the scraper behind it).
const dependencyPingTimeout = 2 * time.Second

// unmatchedRoute is the path_template label for requests chi never matched
// (404s). A constant keeps label cardinality bounded instead of leaking raw
// URL paths — which would let any client mint new time series at will.
const unmatchedRoute = "unmatched"

// PingFunc checks whether a dependency is reachable. It matches the shape of
// the readiness ping deps wired in internal/app so the same functions can be
// reused for the dependency_up gauges.
type PingFunc func(ctx context.Context) error

type requestKey struct {
	method string
	route  string
	status string
}

type durationStat struct {
	count      uint64
	sumSeconds float64
}

// Registry accumulates request metrics and renders the Prometheus text
// exposition. It is safe for concurrent use.
type Registry struct {
	mu        sync.Mutex
	requests  map[requestKey]uint64
	durations map[string]durationStat
	started   time.Time
	mongoPing PingFunc
	redisPing PingFunc
}

// NewRegistry returns an empty registry; started anchors the uptime gauge.
func NewRegistry() *Registry {
	return &Registry{
		mu:        sync.Mutex{},
		requests:  map[requestKey]uint64{},
		durations: map[string]durationStat{},
		started:   time.Now(),
		mongoPing: nil,
		redisPing: nil,
	}
}

// WithPingChecks wires dependency pings for the dependency_up gauges. Pass
// the same functions used for readiness; a nil ping means the gauge for that
// dependency is omitted from the exposition.
func (reg *Registry) WithPingChecks(mongoPing, redisPing PingFunc) *Registry {
	reg.mongoPing = mongoPing
	reg.redisPing = redisPing

	return reg
}

// RecordRequest increments the request counter and duration summary for the
// (method, route template, status) tuple.
func (reg *Registry) RecordRequest(method, route string, status int, duration time.Duration) {
	reg.mu.Lock()
	defer reg.mu.Unlock()

	reg.requests[requestKey{method: method, route: route, status: strconv.Itoa(status)}]++

	stat := reg.durations[route]
	stat.count++
	stat.sumSeconds += duration.Seconds()
	reg.durations[route] = stat
}

// Middleware records per-request metrics. It must be mounted on the chi
// router (router.Use) so the route pattern — not the raw URL path — is
// available as the path_template label after routing; unmatched requests fall
// back to the constant unmatchedRoute label.
func (reg *Registry) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)

		route := chi.RouteContext(r.Context()).RoutePattern()
		if route == "" {
			route = unmatchedRoute
		}

		reg.RecordRequest(r.Method, route, recorder.status, time.Since(start))
	})
}

// Handler serves the Prometheus text exposition, format version 0.0.4. It is
// a public, unauthenticated scrape endpoint and exposes only aggregate
// counters/gauges — never per-tenant or per-user data.
func (reg *Registry) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		if _, err := io.WriteString(w, reg.exposition(r.Context())); err != nil {
			slog.ErrorContext(r.Context(), "write metrics response", "error", err)
		}
	}
}

func (reg *Registry) exposition(ctx context.Context) string {
	var sb strings.Builder

	requests, durations := reg.snapshot()
	writeCounters(&sb, requests)
	writeDurations(&sb, durations)
	reg.writeGauges(ctx, &sb)

	return sb.String()
}

func (reg *Registry) snapshot() (map[requestKey]uint64, map[string]durationStat) {
	reg.mu.Lock()
	defer reg.mu.Unlock()

	requests := make(map[requestKey]uint64, len(reg.requests))
	maps.Copy(requests, reg.requests)

	durations := make(map[string]durationStat, len(reg.durations))
	maps.Copy(durations, reg.durations)

	return requests, durations
}

func writeCounters(sb *strings.Builder, requests map[requestKey]uint64) {
	sb.WriteString("# HELP http_requests_total Total HTTP requests handled, by method, route template, and status.\n")
	sb.WriteString("# TYPE http_requests_total counter\n")

	keys := make([]requestKey, 0, len(requests))
	for key := range requests {
		keys = append(keys, key)
	}

	slices.SortFunc(keys, func(left, right requestKey) int {
		if cmp := strings.Compare(left.route, right.route); cmp != 0 {
			return cmp
		}

		if cmp := strings.Compare(left.method, right.method); cmp != 0 {
			return cmp
		}

		return strings.Compare(left.status, right.status)
	})

	for _, key := range keys {
		sb.WriteString("http_requests_total{method=")
		sb.WriteString(quoteLabel(key.method))
		sb.WriteString(",path_template=")
		sb.WriteString(quoteLabel(key.route))
		sb.WriteString(",status=")
		sb.WriteString(quoteLabel(key.status))
		sb.WriteString("} ")
		sb.WriteString(strconv.FormatUint(requests[key], 10))
		sb.WriteString("\n")
	}
}

func writeDurations(sb *strings.Builder, durations map[string]durationStat) {
	sb.WriteString("# HELP http_request_duration_seconds HTTP request duration summary, by route template.\n")
	sb.WriteString("# TYPE http_request_duration_seconds summary\n")

	routes := make([]string, 0, len(durations))
	for route := range durations {
		routes = append(routes, route)
	}

	slices.Sort(routes)

	for _, route := range routes {
		stat := durations[route]
		label := "{path_template=" + quoteLabel(route) + "} "

		sb.WriteString("http_request_duration_seconds_count")
		sb.WriteString(label)
		sb.WriteString(strconv.FormatUint(stat.count, 10))
		sb.WriteString("\n")

		sb.WriteString("http_request_duration_seconds_sum")
		sb.WriteString(label)
		sb.WriteString(strconv.FormatFloat(stat.sumSeconds, 'f', -1, 64))
		sb.WriteString("\n")
	}
}

func (reg *Registry) writeGauges(ctx context.Context, sb *strings.Builder) {
	sb.WriteString("# HELP process_goroutines Number of goroutines in the process.\n")
	sb.WriteString("# TYPE process_goroutines gauge\n")
	sb.WriteString("process_goroutines ")
	sb.WriteString(strconv.Itoa(runtime.NumGoroutine()))
	sb.WriteString("\n")

	sb.WriteString("# HELP process_uptime_seconds Seconds since the metrics registry was created.\n")
	sb.WriteString("# TYPE process_uptime_seconds gauge\n")
	sb.WriteString("process_uptime_seconds ")
	sb.WriteString(strconv.FormatFloat(time.Since(reg.started).Seconds(), 'f', -1, 64))
	sb.WriteString("\n")

	sb.WriteString("# HELP dependency_up Whether a dependency ping succeeds (1) or fails (0) at scrape time.\n")
	sb.WriteString("# TYPE dependency_up gauge\n")
	writeDependencyGauge(ctx, sb, "mongo", reg.mongoPing)
	writeDependencyGauge(ctx, sb, "redis", reg.redisPing)
}

func writeDependencyGauge(ctx context.Context, sb *strings.Builder, name string, ping PingFunc) {
	if ping == nil {
		return
	}

	pingCtx, cancel := context.WithTimeout(ctx, dependencyPingTimeout)
	defer cancel()

	up := "1"
	if err := ping(pingCtx); err != nil {
		up = "0"
	}

	sb.WriteString("dependency_up{dependency=")
	sb.WriteString(quoteLabel(name))
	sb.WriteString("} ")
	sb.WriteString(up)
	sb.WriteString("\n")
}

// quoteLabel renders a label value per the 0.0.4 format: backslash, double
// quote, and newline are escaped.
func quoteLabel(value string) string {
	var sb strings.Builder

	for _, ch := range value {
		switch ch {
		case '\\':
			sb.WriteString(`\\`)
		case '"':
			sb.WriteString(`\"`)
		case '\n':
			sb.WriteString(`\n`)
		default:
			sb.WriteRune(ch)
		}
	}

	return `"` + sb.String() + `"`
}

type statusRecorder struct {
	http.ResponseWriter

	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}
