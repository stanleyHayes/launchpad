package auth

import (
	"net/http"
	"time"
)

const (
	// AccessTokenCookieName carries the short-lived access JWT. The
	// authentication middleware accepts it when no Authorization header is
	// present.
	AccessTokenCookieName = "lp_access_token"
	// RefreshTokenCookieName carries the refresh token; it is scoped to the
	// refresh endpoint so it is not sent on every API request.
	RefreshTokenCookieName = "lp_refresh_token"

	refreshCookiePath = "/api/v1/auth/refresh"
)

// SetSessionCookies writes the token pair as HttpOnly cookies so browser
// clients never handle tokens in JavaScript. secure must be true outside
// local development. The token pair remains in the JSON body for backward
// compatibility with non-browser clients.
func SetSessionCookies(w http.ResponseWriter, tokens TokenPair, refreshTTL time.Duration, secure bool) {
	accessMaxAge := int(tokens.ExpiresIn)
	refreshMaxAge := int(refreshTTL.Seconds())

	http.SetCookie(w, newSessionCookie(AccessTokenCookieName, tokens.AccessToken, "/", accessMaxAge, secure))
	http.SetCookie(w, newSessionCookie(
		RefreshTokenCookieName, tokens.RefreshToken, refreshCookiePath, refreshMaxAge, secure,
	))
}

// ClearSessionCookies expires both session cookies (logout).
func ClearSessionCookies(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, newSessionCookie(AccessTokenCookieName, "", "/", -1, secure))
	http.SetCookie(w, newSessionCookie(RefreshTokenCookieName, "", refreshCookiePath, -1, secure))
}

// newSessionCookie builds an HttpOnly, SameSite=Lax session cookie. secure
// gates the Secure attribute and is false only for local development over
// plain HTTP; the app wires config.AppEnv != "local" so it is true in every
// deployed environment. Gosec cannot prove that from a variable, hence the
// suppression — there is no constant-only formulation of a config-gated
// Secure attribute.
func newSessionCookie(name, value, path string, maxAge int, secure bool) *http.Cookie {
	return &http.Cookie{ //nolint:gosec // G124: Secure is config-gated for local dev; always true outside it.
		Name:     name,
		Value:    value,
		Path:     path,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	}
}
