// Package middleware provides shared HTTP middleware.
package middleware

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	chimw "github.com/go-chi/chi/v5/middleware"

	"launchpad/pkg/httpx"
	"launchpad/pkg/security"
)

// CORS restricts cross-origin requests to the configured origins.
func CORS(origins []string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(origins))
	for _, origin := range origins {
		allowed[origin] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if _, ok := allowed[origin]; ok {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
				w.Header().Set(
					"Access-Control-Allow-Headers",
					"Authorization, Content-Type, Idempotency-Key, X-Organization-Id",
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

// RequestLogger logs each request with slog.
func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

// Authenticate validates a Bearer JWT and injects the principal.
func Authenticate(jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if !strings.HasPrefix(header, "Bearer ") {
				if err := httpx.WriteError(
					w,
					http.StatusUnauthorized,
					"UNAUTHORIZED",
					"Authentication required",
				); err != nil {
					slog.ErrorContext(r.Context(), "write unauthorized response", "error", err)
				}

				return
			}

			principal, err := security.ParseAccessToken(jwtSecret, strings.TrimPrefix(header, "Bearer "))
			if err != nil {
				if writeErr := httpx.WriteError(
					w,
					http.StatusUnauthorized,
					"UNAUTHORIZED",
					"Invalid or expired token",
				); writeErr != nil {
					slog.ErrorContext(r.Context(), "write unauthorized response", "error", writeErr)
				}

				return
			}

			ctx := security.WithPrincipal(r.Context(), principal)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequirePlatform rejects requests from non-platform staff.
func RequirePlatform(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := security.PrincipalFromContext(r.Context())
		if !ok || !isPlatformRole(principal.RoleCode) {
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

func isPlatformRole(roleCode string) bool {
	return roleCode == "platform_owner" ||
		roleCode == "platform_admin" ||
		strings.HasPrefix(roleCode, "platform_")
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
