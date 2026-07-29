package middleware

import (
	"context"
	"crypto/subtle"
	"log/slog"
	"net/http"

	"launchpad/pkg/httpx"
	"launchpad/pkg/security"
)

const (
	// CSRFCookieName carries the double-submit CSRF token. It is deliberately
	// readable by JavaScript (not HttpOnly) so browser clients can echo it in
	// CSRFHeaderName; it holds no credential, so reading it grants nothing.
	CSRFCookieName = "lp_csrf"
	// CSRFHeaderName is the header cookie-authenticated clients must send on
	// mutating requests, set to the current CSRFCookieName value.
	CSRFHeaderName = "X-CSRF-Token"
	// csrfHeaderLookupKey is CSRFHeaderName in Go's canonical header-key form,
	// used for Header map lookups. HTTP header names are case-insensitive, so
	// both spellings address the same header on the wire.
	csrfHeaderLookupKey = "X-Csrf-Token"
)

// authSource records how Authenticate resolved the caller's credentials, so
// CSRF can tell browser (cookie) callers from bearer-token clients.
type authSource int

const (
	authSourceNone authSource = iota
	authSourceBearer
	authSourceCookie
)

type authSourceKey struct{}

func withAuthSource(ctx context.Context, source authSource) context.Context {
	return context.WithValue(ctx, authSourceKey{}, source)
}

func authSourceFrom(ctx context.Context) authSource {
	source, _ := ctx.Value(authSourceKey{}).(authSource)

	return source
}

// CSRF protects cookie-authenticated mutating requests with the
// double-submit-cookie pattern. It must run after Authenticate, which marks
// whether the credentials came from the session cookie.
//
// On every authenticated request it ensures a CSRFCookieName cookie exists,
// setting a fresh 32-byte random token when absent. On POST/PUT/PATCH/DELETE
// authenticated via the cookie it requires CSRFHeaderName to match the cookie
// value, rejecting with 403 CSRF_TOKEN_INVALID otherwise: a cross-site
// attacker can make the browser send cookies but cannot read the token to
// echo it back. Bearer-authenticated clients (SCIM, CLI) are exempt — a
// browser never attaches an Authorization header on the attacker's behalf.
//
// secure gates the cookie's Secure attribute and must be true outside local
// development (wire config.AppEnv != "local", same as the session cookies).
func CSRF(secure bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := csrfCookieValue(r)
			if token == "" {
				fresh, err := security.NewRefreshToken()
				if err != nil {
					slog.ErrorContext(r.Context(), "generate csrf token", "error", err)
					writeCSRFError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to issue CSRF token")

					return
				}

				token = fresh
				http.SetCookie(w, newCSRFCookie(token, secure))
			}

			if isMutatingMethod(r.Method) && authSourceFrom(r.Context()) == authSourceCookie {
				header := r.Header.Get(csrfHeaderLookupKey)
				if header == "" || subtle.ConstantTimeCompare([]byte(header), []byte(token)) != 1 {
					writeCSRFError(w, r, http.StatusForbidden, "CSRF_TOKEN_INVALID", "CSRF token missing or invalid")

					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

func csrfCookieValue(r *http.Request) string {
	cookie, err := r.Cookie(CSRFCookieName)
	if err != nil {
		return ""
	}

	return cookie.Value
}

func isMutatingMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

// newCSRFCookie builds the readable, SameSite=Lax CSRF cookie. secure gates
// the Secure attribute and is false only for local development over plain
// HTTP; the app wires config.AppEnv != "local" so it is true in every
// deployed environment. Same nolint rationale as the auth session cookies.
func newCSRFCookie(value string, secure bool) *http.Cookie {
	return &http.Cookie{ //nolint:gosec // G124: Secure is config-gated for local dev; always true outside it.
		Name:     CSRFCookieName,
		Value:    value,
		Path:     "/",
		HttpOnly: false,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	}
}

func writeCSRFError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	if err := httpx.WriteError(w, status, code, message); err != nil {
		slog.ErrorContext(r.Context(), "write csrf error response", "error", err)
	}
}
