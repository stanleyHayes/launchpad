package middleware_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"launchpad/pkg/httpx"
	"launchpad/pkg/middleware"
)

// csrfProtectedHandler stacks Authenticate under CSRF exactly as the app
// wires them, over a handler that only reports the request got through.
func csrfProtectedHandler(secure bool) http.Handler {
	return middleware.Authenticate(testJWTSecret, activeSessions{}, testTokenCookieName)(
		middleware.CSRF(secure)(authenticatedHandler()),
	)
}

func cookieAuthenticatedRequest(t *testing.T, method, csrfCookie, csrfHeader string) *http.Request {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), method, "/api/v1/employees", nil)
	req.AddCookie(&http.Cookie{Name: testTokenCookieName, Value: issueToken(t)})

	if csrfCookie != "" {
		req.AddCookie(&http.Cookie{Name: middleware.CSRFCookieName, Value: csrfCookie})
	}

	if csrfHeader != "" {
		req.Header.Set(middleware.CSRFHeaderName, csrfHeader)
	}

	return req
}

func responseErrorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()

	var envelope httpx.Envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode error envelope: %v", err)
	}

	if envelope.Error == nil {
		t.Fatalf("expected error envelope, got %s", rec.Body.String())
	}

	return envelope.Error.Code
}

func TestCSRFAllowsMutatingRequestWhenHeaderMatchesCookie(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()

	csrfProtectedHandler(false).ServeHTTP(
		rec,
		cookieAuthenticatedRequest(t, http.MethodPost, "token-123", "token-123"),
	)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected matching CSRF token accepted, got status %d", rec.Code)
	}
}

func TestCSRFRejectsMutatingRequestWithoutHeader(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()

	csrfProtectedHandler(false).ServeHTTP(
		rec,
		cookieAuthenticatedRequest(t, http.MethodPost, "token-123", ""),
	)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 without CSRF header, got status %d", rec.Code)
	}

	if got := responseErrorCode(t, rec); got != "CSRF_TOKEN_INVALID" {
		t.Fatalf("expected CSRF_TOKEN_INVALID, got %q", got)
	}
}

func TestCSRFRejectsMutatingRequestWithMismatchedHeader(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()

	csrfProtectedHandler(false).ServeHTTP(
		rec,
		cookieAuthenticatedRequest(t, http.MethodPost, "token-123", "token-999"),
	)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on token mismatch, got status %d", rec.Code)
	}

	if got := responseErrorCode(t, rec); got != "CSRF_TOKEN_INVALID" {
		t.Fatalf("expected CSRF_TOKEN_INVALID, got %q", got)
	}
}

func TestCSRFRejectsMutatingRequestWithoutCookie(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()

	csrfProtectedHandler(false).ServeHTTP(
		rec,
		cookieAuthenticatedRequest(t, http.MethodDelete, "", "token-123"),
	)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 without CSRF cookie, got status %d", rec.Code)
	}

	// A fresh token is still issued so the client can recover.
	if rec.Header().Get("Set-Cookie") == "" {
		t.Fatal("expected a fresh CSRF cookie to be set")
	}
}

func TestCSRFExemptsBearerAuthenticatedRequests(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/employees", nil)
	req.Header.Set("Authorization", "Bearer "+issueToken(t))

	rec := httptest.NewRecorder()

	csrfProtectedHandler(false).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected bearer-authenticated mutation exempt from CSRF, got status %d", rec.Code)
	}
}

func TestCSRFAllowsSafeMethodsWithoutToken(t *testing.T) {
	t.Parallel()

	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodOptions} {
		rec := httptest.NewRecorder()

		csrfProtectedHandler(false).ServeHTTP(rec, cookieAuthenticatedRequest(t, method, "", ""))

		if rec.Code != http.StatusNoContent {
			t.Fatalf("expected %s without CSRF token allowed, got status %d", method, rec.Code)
		}
	}
}

func TestCSRFSetsSecureCookieOutsideLocal(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()

	csrfProtectedHandler(true).ServeHTTP(rec, cookieAuthenticatedRequest(t, http.MethodGet, "", ""))

	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == middleware.CSRFCookieName && !cookie.Secure {
			t.Fatal("expected Secure CSRF cookie outside local development")
		}
	}
}

func TestCSRFSetsReadableLaxCookieWhenAbsent(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()

	csrfProtectedHandler(false).ServeHTTP(rec, cookieAuthenticatedRequest(t, http.MethodGet, "", ""))

	cookies := rec.Result().Cookies()

	var csrfCookie *http.Cookie

	for _, cookie := range cookies {
		if cookie.Name == middleware.CSRFCookieName {
			csrfCookie = cookie
		}
	}

	if csrfCookie == nil {
		t.Fatalf("expected %s cookie to be set, got %v", middleware.CSRFCookieName, cookies)
	}

	if csrfCookie.Value == "" {
		t.Fatal("expected a non-empty CSRF token")
	}

	if csrfCookie.HttpOnly {
		t.Fatal("CSRF cookie must be readable by JavaScript (not HttpOnly)")
	}

	if csrfCookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("expected SameSite=Lax, got %v", csrfCookie.SameSite)
	}

	// The issued token unlocks a subsequent mutating request (double submit).
	next := httptest.NewRecorder()

	csrfProtectedHandler(false).ServeHTTP(
		next,
		cookieAuthenticatedRequest(t, http.MethodPost, csrfCookie.Value, csrfCookie.Value),
	)

	if next.Code != http.StatusNoContent {
		t.Fatalf("expected issued token to pass on mutation, got status %d", next.Code)
	}
}
