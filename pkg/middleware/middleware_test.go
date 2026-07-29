package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"launchpad/pkg/middleware"
	"launchpad/pkg/security"
)

const (
	testJWTSecret       = "test-secret"
	testTokenCookieName = "lp_access_token"
)

type activeSessions struct{}

func (activeSessions) SessionExists(context.Context, string) (bool, error) { return true, nil }

func issueToken(t *testing.T) string {
	t.Helper()

	token, err := security.IssueAccessToken(testJWTSecret, time.Minute, security.Principal{
		UserID:         "user-1",
		Email:          "owner@example.com",
		OrganizationID: "org-1",
		RoleCode:       "organization_owner",
		SessionID:      "session-1",
	})
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	return token
}

// authenticatedHandler reports whether the request reached the protected
// handler with a principal in context.
func authenticatedHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := security.PrincipalFromContext(r.Context()); !ok {
			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	})
}

func TestAuthenticateAcceptsBearerHeader(t *testing.T) {
	t.Parallel()

	handler := middleware.Authenticate(testJWTSecret, activeSessions{}, testTokenCookieName)(authenticatedHandler())

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+issueToken(t))

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected bearer token accepted, got status %d", rec.Code)
	}
}

func TestAuthenticateAcceptsAccessTokenCookieWhenHeaderAbsent(t *testing.T) {
	t.Parallel()

	handler := middleware.Authenticate(testJWTSecret, activeSessions{}, testTokenCookieName)(authenticatedHandler())

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: testTokenCookieName, Value: issueToken(t)})

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected cookie token accepted, got status %d", rec.Code)
	}
}

func TestAuthenticateRejectsMissingCredentials(t *testing.T) {
	t.Parallel()

	handler := middleware.Authenticate(testJWTSecret, activeSessions{}, testTokenCookieName)(authenticatedHandler())

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/auth/me", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without credentials, got status %d", rec.Code)
	}
}

func TestSecurityHeadersSetsNosniff(t *testing.T) {
	t.Parallel()

	handler := middleware.SecurityHeaders(authenticatedHandler())

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/employees", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("expected X-Content-Type-Options nosniff, got %q", got)
	}
}

func TestSecurityHeadersSetsFullBaselineWithoutHSTS(t *testing.T) {
	t.Parallel()

	handler := middleware.SecurityHeaders(authenticatedHandler())

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/employees", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	want := map[string]string{
		"X-Content-Type-Options":  "nosniff",
		"X-Frame-Options":         "DENY",
		"Referrer-Policy":         "strict-origin-when-cross-origin",
		"Permissions-Policy":      "camera=(), microphone=(), geolocation=()",
		"Content-Security-Policy": "default-src 'none'; frame-ancestors 'none'",
	}

	for name, value := range want {
		if got := rec.Header().Get(name); got != value {
			t.Fatalf("expected %s %q, got %q", name, value, got)
		}
	}

	if got := rec.Header().Get("Strict-Transport-Security"); got != "" {
		t.Fatalf("SecurityHeaders must not set HSTS without the app env, got %q", got)
	}
}

func TestSecurityHeadersWithConfigSetsHSTSOutsideLocal(t *testing.T) {
	t.Parallel()

	const hsts = "max-age=63072000; includeSubDomains"

	for _, tc := range []struct {
		env      string
		wantHSTS string
	}{
		{"local", ""},
		{"production", hsts},
		{"staging", hsts},
	} {
		handler := middleware.SecurityHeadersWithConfig(tc.env)(authenticatedHandler())

		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/employees", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if got := rec.Header().Get("Strict-Transport-Security"); got != tc.wantHSTS {
			t.Fatalf("env %q: expected HSTS %q, got %q", tc.env, tc.wantHSTS, got)
		}

		// The baseline headers are always present regardless of env.
		if got := rec.Header().Get("Content-Security-Policy"); got != "default-src 'none'; frame-ancestors 'none'" {
			t.Fatalf("env %q: expected CSP baseline, got %q", tc.env, got)
		}
	}
}

// servePlatformRoleRequest runs a platform gating middleware against a
// principal and reports the status the protected handler produced.
func servePlatformRoleRequest(
	principal *security.Principal,
	wrap func(http.Handler) http.Handler,
) *httptest.ResponseRecorder {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	wrap(next).ServeHTTP(recorder, requestWithPrincipal(principal))

	return recorder
}

func platformPrincipal(roleCode string) *security.Principal {
	return &security.Principal{UserID: "user-1", RoleCode: roleCode, SessionID: "session-1"}
}

func TestRequirePlatformAdmitsFullStaffRoleSet(t *testing.T) {
	t.Parallel()

	for _, roleCode := range []string{
		"platform_owner",
		"platform_admin",
		"support_agent",
		"billing_admin",
		"content_editor",
		"security_admin",
		"analyst",
		"read_only_auditor",
	} {
		recorder := servePlatformRoleRequest(platformPrincipal(roleCode), middleware.RequirePlatform)
		if recorder.Code != http.StatusNoContent {
			t.Fatalf("role %q: got %d, want 204", roleCode, recorder.Code)
		}
	}
}

func TestRequirePlatformRejectsNonStaff(t *testing.T) {
	t.Parallel()

	recorder := servePlatformRoleRequest(platformPrincipal("organization_owner"), middleware.RequirePlatform)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("tenant role: got %d, want 403", recorder.Code)
	}

	if recorder := servePlatformRoleRequest(nil, middleware.RequirePlatform); recorder.Code != http.StatusForbidden {
		t.Fatalf("no principal: got %d, want 403", recorder.Code)
	}
}

func TestRequirePlatformRoleScopesStaffRoles(t *testing.T) {
	t.Parallel()

	wrap := middleware.RequirePlatformRole("support_agent")

	for _, tc := range []struct {
		roleCode string
		want     int
	}{
		{"platform_owner", http.StatusNoContent},
		{"platform_admin", http.StatusNoContent},
		{"support_agent", http.StatusNoContent},
		{"billing_admin", http.StatusForbidden},
		{"analyst", http.StatusForbidden},
		{"read_only_auditor", http.StatusForbidden},
		{"organization_owner", http.StatusForbidden},
	} {
		recorder := servePlatformRoleRequest(platformPrincipal(tc.roleCode), wrap)
		if recorder.Code != tc.want {
			t.Fatalf("role %q: got %d, want %d", tc.roleCode, recorder.Code, tc.want)
		}
	}

	if recorder := servePlatformRoleRequest(nil, wrap); recorder.Code != http.StatusForbidden {
		t.Fatalf("no principal: got %d, want 403", recorder.Code)
	}
}

func TestRequirePlatformRoleEmptyRestrictsToOwnerAndAdmin(t *testing.T) {
	t.Parallel()

	wrap := middleware.RequirePlatformRole()

	for _, tc := range []struct {
		roleCode string
		want     int
	}{
		{"platform_owner", http.StatusNoContent},
		{"platform_admin", http.StatusNoContent},
		{"support_agent", http.StatusForbidden},
		{"security_admin", http.StatusForbidden},
	} {
		recorder := servePlatformRoleRequest(platformPrincipal(tc.roleCode), wrap)
		if recorder.Code != tc.want {
			t.Fatalf("role %q: got %d, want %d", tc.roleCode, recorder.Code, tc.want)
		}
	}
}
