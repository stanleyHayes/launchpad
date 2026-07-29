// Package middleware provides shared HTTP middleware.
package middleware

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	chimw "github.com/go-chi/chi/v5/middleware"

	"launchpad/pkg/httpx"
	"launchpad/pkg/security"
)

// RateLimit returns middleware that allows at most `limit` requests per client
// IP within `window`, responding 429 once exceeded. It keys on the TCP peer
// address (RemoteAddr) so a client cannot inflate its budget by spoofing
// X-Forwarded-For; front it with a trusted-proxy RealIP layer when deployed
// behind a load balancer.
//
// The limiter is in-memory and per-process: with N replicas the effective
// limit is N×limit per window, and all counters reset on every deploy or
// restart. When correctness at scale matters, move the counters to the
// already-required Redis (e.g. a sliding-window INCR/EXPIRE per key).
func RateLimit(limit int, window time.Duration) func(http.Handler) http.Handler {
	limiter := &ipLimiter{
		mu:        sync.Mutex{},
		limit:     limit,
		window:    window,
		lastSweep: time.Time{},
		hits:      map[string]hitWindow{},
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !limiter.allow(clientIP(r)) {
				w.Header().Set("Retry-After", strconv.Itoa(int(window.Seconds())))

				if err := httpx.WriteError(w, http.StatusTooManyRequests, "RATE_LIMITED", "Too many requests"); err != nil {
					slog.ErrorContext(r.Context(), "write rate limit response", "error", err)
				}

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

type hitWindow struct {
	count int
	reset time.Time
}

type ipLimiter struct {
	mu        sync.Mutex
	limit     int
	window    time.Duration
	lastSweep time.Time
	hits      map[string]hitWindow
}

func (l *ipLimiter) allow(ip string) bool {
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	if now.Sub(l.lastSweep) > l.window {
		for key, hit := range l.hits {
			if now.After(hit.reset) {
				delete(l.hits, key)
			}
		}

		l.lastSweep = now
	}

	hit, ok := l.hits[ip]
	if !ok || now.After(hit.reset) {
		l.hits[ip] = hitWindow{count: 1, reset: now.Add(l.window)}

		return true
	}

	if hit.count >= l.limit {
		return false
	}

	hit.count++
	l.hits[ip] = hit

	return true
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return host
}

// bearerToken extracts the JWT from the Authorization header, or "" when the
// header is missing or not a Bearer credential.
func bearerToken(r *http.Request) string {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		return ""
	}

	return strings.TrimPrefix(header, "Bearer ")
}

// hstsValue pins HTTPS for two years, subdomains included.
const hstsValue = "max-age=63072000; includeSubDomains"

// SecurityHeaders sets the baseline response headers that stop browsers from
// MIME-sniffing, framing, or referrer-leaking API responses. It omits
// Strict-Transport-Security because it does not know whether the process
// serves HTTPS; wired applications should use SecurityHeadersWithConfig.
func SecurityHeaders(next http.Handler) http.Handler {
	return securityHeaders("")(next)
}

// SecurityHeadersWithConfig behaves like SecurityHeaders and additionally sets
// Strict-Transport-Security whenever appEnv is not "local": local development
// serves plain HTTP, where HSTS would pin the browser to an HTTPS origin that
// does not exist.
func SecurityHeadersWithConfig(appEnv string) func(http.Handler) http.Handler {
	return securityHeaders(appEnv)
}

func securityHeaders(appEnv string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := w.Header()
			header.Set("X-Content-Type-Options", "nosniff")
			header.Set("X-Frame-Options", "DENY")
			header.Set("Referrer-Policy", "strict-origin-when-cross-origin")
			header.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
			// This is a JSON API: it serves no scripts, styles, or frames, so
			// nothing may load and nothing may frame it.
			header.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")

			if appEnv != "" && appEnv != "local" {
				header.Set("Strict-Transport-Security", hstsValue)
			}

			next.ServeHTTP(w, r)
		})
	}
}

// CORS restricts cross-origin requests to configured exact origins or narrow
// wildcard patterns. Patterns are intended for provider-generated preview
// hosts, for example https://launchpad-marketing-*.vercel.app.
func CORS(origins, originPatterns []string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(origins))
	for _, origin := range origins {
		allowed[origin] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if originAllowed(origin, allowed, originPatterns) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
				w.Header().Set(
					"Access-Control-Allow-Headers",
					"Authorization, Content-Type, Idempotency-Key, X-Organization-Id, X-CSRF-Token",
				)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func originAllowed(origin string, exact map[string]struct{}, patterns []string) bool {
	if origin == "" {
		return false
	}

	if _, ok := exact[origin]; ok {
		return true
	}

	parsed, err := url.Parse(origin)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") ||
		parsed.Host == "" || parsed.User != nil || parsed.Path != "" ||
		parsed.RawQuery != "" || parsed.Fragment != "" {
		return false
	}

	for _, pattern := range patterns {
		if strings.Count(pattern, "*") != 1 {
			continue
		}

		parts := strings.SplitN(pattern, "*", 2)
		if parts[0] == "" || parts[1] == "" {
			continue
		}

		if strings.HasPrefix(origin, parts[0]) && strings.HasSuffix(origin, parts[1]) {
			return true
		}
	}

	return false
}

// RequestLogger logs each request with slog. Health probes (/healthz,
// /readyz) are skipped so load-balancer checks do not flood the request log.
func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
			next.ServeHTTP(w, r)

			return
		}

		start := time.Now().UTC()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)

		slog.InfoContext(
			r.Context(),
			"http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", recorder.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"request_id", chimw.GetReqID(r.Context()),
		)
	})
}

type statusRecorder struct {
	http.ResponseWriter

	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

// SessionChecker verifies that an auth session is still active. The Redis
// session store adapter is wired in internal/app.
type SessionChecker interface {
	// SessionExists reports whether the session still exists in the session
	// store. A false result with a nil error means the session was revoked
	// (logout, deprovisioning); a non-nil error means the check itself failed.
	SessionExists(ctx context.Context, sessionID string) (bool, error)
}

// Authenticate validates a Bearer JWT and injects the principal. Beyond
// signature/expiry it verifies the token's sessionId still exists in the
// session store, so logout and deprovisioning revoke access tokens
// immediately instead of at JWT expiry. The check fails closed: a missing
// session is rejected with 401, and a session-store error is rejected with
// 503 (logged) so a Redis outage never silently extends revoked sessions.
//
// Browser clients hold the access token in an HttpOnly cookie instead of the
// Authorization header; when the header is absent the token is read from the
// cookie named tokenCookieName (wired from auth.AccessTokenCookieName so this
// package does not import internal/auth). The resolved credential source is
// recorded on the context so downstream middleware (CSRF) can apply
// browser-only checks to cookie-authenticated requests.
func Authenticate(jwtSecret string, sessions SessionChecker, tokenCookieName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, source, ok := authenticateRequest(w, r, jwtSecret, sessions, tokenCookieName)
			if !ok {
				return
			}

			ctx := security.WithPrincipal(r.Context(), principal)
			ctx = withAuthSource(ctx, source)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// authenticateRequest resolves the caller's token — Authorization header
// first, the tokenCookieName cookie when the header is absent — and validates
// it together with its server-side session, also reporting which credential
// source succeeded. It writes the failure response itself; ok is false when
// the request must not proceed.
func authenticateRequest(
	w http.ResponseWriter,
	r *http.Request,
	jwtSecret string,
	sessions SessionChecker,
	tokenCookieName string,
) (security.Principal, authSource, bool) {
	source := authSourceBearer

	token := bearerToken(r)
	if token == "" && tokenCookieName != "" {
		if cookie, err := r.Cookie(tokenCookieName); err == nil {
			token = cookie.Value
			source = authSourceCookie
		}
	}

	if token == "" {
		writeAuthFailure(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")

		return security.Principal{}, authSourceNone, false
	}

	principal, err := security.ParseAccessToken(jwtSecret, token)
	if err != nil {
		writeAuthFailure(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid or expired token")

		return security.Principal{}, authSourceNone, false
	}

	if !sessionActive(w, r, sessions, principal.SessionID) {
		return security.Principal{}, authSourceNone, false
	}

	return principal, source, true
}

// sessionActive fails closed: a missing session is rejected and a
// session-store error is rejected with 503 (logged) so a Redis outage never
// silently extends revoked sessions.
func sessionActive(w http.ResponseWriter, r *http.Request, sessions SessionChecker, sessionID string) bool {
	exists, err := sessions.SessionExists(r.Context(), sessionID)
	if err != nil {
		slog.ErrorContext(r.Context(), "session existence check failed", "error", err)
		writeAuthFailure(
			w,
			r,
			http.StatusServiceUnavailable,
			"SESSION_STORE_UNAVAILABLE",
			"Session store unavailable, retry shortly",
		)

		return false
	}

	if !exists {
		writeAuthFailure(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Session is no longer active")

		return false
	}

	return true
}

func writeAuthFailure(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	if err := httpx.WriteError(w, status, code, message); err != nil {
		slog.ErrorContext(r.Context(), "write unauthorized response", "error", err)
	}
}

// RequirePlatform rejects requests from non-platform staff. Support
// impersonation principals are rejected explicitly as well: an impersonation
// token carries a tenant role so the role check already fails closed, but
// naming the guard keeps platform settings, subscription changes, and tenant
// purges unreachable even if an impersonation token is ever issued with a
// platform-looking role code.
func RequirePlatform(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := security.PrincipalFromContext(r.Context())
		if !ok || principal.Impersonator || !isPlatformRole(principal.RoleCode) {
			if err := httpx.WriteError(
				w,
				http.StatusForbidden,
				"FORBIDDEN",
				"Platform access required",
			); err != nil {
				slog.ErrorContext(r.Context(), "write forbidden response", "error", err)
			}

			return
		}

		next.ServeHTTP(w, r)
	})
}

// isPlatformRole reports whether roleCode belongs to the platform staff role
// set (PRD §5.2.6).
func isPlatformRole(roleCode string) bool {
	switch roleCode {
	case "platform_owner",
		"platform_admin",
		"support_agent",
		"billing_admin",
		"content_editor",
		"security_admin",
		"analyst",
		"read_only_auditor":
		return true
	}

	return false
}

// RequirePlatformRole rejects platform staff whose role is not in the allowed
// set. platform_owner and platform_admin always pass (full access), so an
// empty allowed list restricts the route to owners and admins only. Apply it
// per route or route group after RequirePlatform (PRD §5.2.6).
func RequirePlatformRole(allowed ...string) func(http.Handler) http.Handler {
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, role := range allowed {
		allowedSet[role] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, ok := security.PrincipalFromContext(r.Context())
			_, roleAllowed := allowedSet[principal.RoleCode]

			if !ok || !isPlatformRole(principal.RoleCode) || !platformRoleAllowed(principal.RoleCode, roleAllowed) {
				if err := httpx.WriteError(
					w,
					http.StatusForbidden,
					"FORBIDDEN",
					"Platform access required",
				); err != nil {
					slog.ErrorContext(r.Context(), "write forbidden response", "error", err)
				}

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// platformRoleAllowed reports whether a platform staff role may proceed:
// owners and admins always may, other roles only when explicitly allowed.
func platformRoleAllowed(roleCode string, allowed bool) bool {
	return roleCode == "platform_owner" || roleCode == "platform_admin" || allowed
}

// RequireOrganization rejects requests missing organization context.
func RequireOrganization(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := security.PrincipalFromContext(r.Context())
		if !ok || principal.OrganizationID == "" {
			if err := httpx.WriteError(
				w,
				http.StatusForbidden,
				"ORGANIZATION_CONTEXT_REQUIRED",
				"An organization context is required",
			); err != nil {
				slog.ErrorContext(r.Context(), "write forbidden response", "error", err)
			}

			return
		}

		next.ServeHTTP(w, r)
	})
}
