package audit

import (
	"context"
	"net"
	"net/http"

	chimw "github.com/go-chi/chi/v5/middleware"
)

// RequestContext carries per-request metadata captured into every audit
// event: the client IP (post-RealIP), the User-Agent header, and chi's
// request id.
type RequestContext struct {
	IP        string
	UserAgent string
	RequestID string
}

type requestContextKey struct{}

// WithRequestContext stores rc on ctx; Service.Record reads it back. Tests
// and non-HTTP callers (schedulers, webhook consumers) use it directly when
// no HTTP request is in flight.
func WithRequestContext(ctx context.Context, rc RequestContext) context.Context {
	return context.WithValue(ctx, requestContextKey{}, rc)
}

// requestContextFrom returns the captured request metadata, or a zero value
// when the request never passed through Middleware (background jobs, tests).
func requestContextFrom(ctx context.Context) RequestContext {
	rc, _ := ctx.Value(requestContextKey{}).(RequestContext)

	return rc
}

// Middleware populates the audit request context for the rest of the chain.
// Mount it after chimw.RequestID and middleware.RealIP so the request id
// exists and RemoteAddr already holds the real client IP.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := WithRequestContext(r.Context(), RequestContext{
			IP:        clientHost(r.RemoteAddr),
			UserAgent: r.UserAgent(),
			RequestID: chimw.GetReqID(r.Context()),
		})

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// clientHost strips the port from a host:port RemoteAddr, leaving bare IPs
// (already rewritten by RealIP) untouched.
func clientHost(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}

	return host
}
