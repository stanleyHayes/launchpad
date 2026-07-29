package middleware_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"launchpad/pkg/middleware"
	"launchpad/pkg/security"
)

type fakePermissionResolver struct {
	permissions map[string]struct{}
	err         error

	gotOrganizationID string
	gotRoleCode       string
}

func (f *fakePermissionResolver) ResolvePermissions(
	_ context.Context,
	organizationID, roleCode string,
) (map[string]struct{}, error) {
	f.gotOrganizationID = organizationID
	f.gotRoleCode = roleCode

	return f.permissions, f.err
}

func requestWithPrincipal(principal *security.Principal) *http.Request {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil)
	if principal != nil {
		req = req.WithContext(security.WithPrincipal(req.Context(), *principal))
	}

	return req
}

func servePermissionRequest(
	t *testing.T,
	resolver middleware.PermissionResolver,
	principal *security.Principal,
) *httptest.ResponseRecorder {
	t.Helper()

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	handler := middleware.RequirePermission(resolver, "journeys.create")(next)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, requestWithPrincipal(principal))

	return recorder
}

func TestRequirePermissionAllow(t *testing.T) {
	t.Parallel()

	resolver := &fakePermissionResolver{
		permissions: map[string]struct{}{"journeys.create": {}},
		err:         nil,
	}
	principal := &security.Principal{
		UserID:         "user-1",
		Email:          "a@b.c",
		OrganizationID: "org-1",
		RoleCode:       "hr_admin",
		SessionID:      "sess-1",
	}

	recorder := servePermissionRequest(t, resolver, principal)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status=%d, want %d", recorder.Code, http.StatusNoContent)
	}

	if resolver.gotOrganizationID != "org-1" || resolver.gotRoleCode != "hr_admin" {
		t.Errorf(
			"resolver called with (%q, %q), want (org-1, hr_admin)",
			resolver.gotOrganizationID,
			resolver.gotRoleCode,
		)
	}
}

func TestRequirePermissionDenyMissingPermission(t *testing.T) {
	t.Parallel()

	resolver := &fakePermissionResolver{
		permissions: map[string]struct{}{"employees.read": {}},
		err:         nil,
	}
	principal := &security.Principal{
		UserID:         "user-1",
		Email:          "a@b.c",
		OrganizationID: "org-1",
		RoleCode:       "employee",
		SessionID:      "sess-1",
	}

	recorder := servePermissionRequest(t, resolver, principal)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status=%d, want %d", recorder.Code, http.StatusForbidden)
	}
}

func TestRequirePermissionDenyUnknownRole(t *testing.T) {
	t.Parallel()

	// A membership whose role resolves to the empty set (deleted custom role)
	// must be denied.
	resolver := &fakePermissionResolver{permissions: map[string]struct{}{}, err: nil}
	principal := &security.Principal{
		UserID:         "user-1",
		Email:          "a@b.c",
		OrganizationID: "org-1",
		RoleCode:       "deleted_role",
		SessionID:      "sess-1",
	}

	recorder := servePermissionRequest(t, resolver, principal)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status=%d, want %d", recorder.Code, http.StatusForbidden)
	}
}

func TestRequirePermissionNoPrincipal(t *testing.T) {
	t.Parallel()

	resolver := &fakePermissionResolver{permissions: nil, err: nil}

	recorder := servePermissionRequest(t, resolver, nil)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}

func TestRequirePermissionResolverErrorFailsClosed(t *testing.T) {
	t.Parallel()

	resolver := &fakePermissionResolver{
		permissions: nil,
		err:         errors.New("store down"),
	}
	principal := &security.Principal{
		UserID:         "user-1",
		Email:          "a@b.c",
		OrganizationID: "org-1",
		RoleCode:       "hr_admin",
		SessionID:      "sess-1",
	}

	recorder := servePermissionRequest(t, resolver, principal)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}

func serveImpersonatorRequest(t *testing.T, permission string) *httptest.ResponseRecorder {
	t.Helper()

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	principal := &security.Principal{
		UserID:                 "agent-1",
		Email:                  "agent@launchpad.example",
		OrganizationID:         "org-1",
		RoleCode:               "hr_admin",
		SessionID:              "support-session-1",
		Impersonator:           true,
		ImpersonationSessionID: "support-session-1",
	}

	// The resolver would grant hr_admin's full write-capable set; an
	// impersonator must never see it. A resolver error also proves the
	// resolver is bypassed for impersonators.
	resolver := &fakePermissionResolver{permissions: nil, err: errors.New("must not be called")}

	handler := middleware.RequirePermission(resolver, permission)(next)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, requestWithPrincipal(principal))

	return recorder
}

func TestRequirePermissionImpersonatorAllowedReadOnlyPermission(t *testing.T) {
	t.Parallel()

	for _, permission := range []string{
		"employees.read",
		"assignments.read",
		"analytics.read",
		"audit.read",
		"notifications.read",
	} {
		recorder := serveImpersonatorRequest(t, permission)
		if recorder.Code != http.StatusNoContent {
			t.Errorf("permission %q: status=%d, want %d", permission, recorder.Code, http.StatusNoContent)
		}
	}
}

func TestRequirePermissionImpersonatorDeniedWriteAndBillingPermissions(t *testing.T) {
	t.Parallel()

	for _, permission := range []string{
		"employees.create",
		"employees.update",
		"journeys.create",
		"journeys.assign",
		"billing.read",
		"billing.manage",
		"members.invite",
		"integrations.manage",
		"data.export",
	} {
		recorder := serveImpersonatorRequest(t, permission)
		if recorder.Code != http.StatusForbidden {
			t.Errorf("permission %q: status=%d, want %d", permission, recorder.Code, http.StatusForbidden)
		}
	}
}
