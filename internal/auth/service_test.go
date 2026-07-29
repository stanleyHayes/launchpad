package auth_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"launchpad/internal/audit"
	"launchpad/internal/auth"
	"launchpad/internal/organizations"
	"launchpad/pkg/security"
)

// stubOrgDirectory is a working in-memory OrgDirectory: unlike fakeOrgs (which
// only supports the invitation flows) it actually persists organizations and
// memberships so Register/Login can be exercised end to end.
type stubOrgDirectory struct {
	orgs    map[string]organizations.Organization
	slugs   map[string]string
	members map[string]organizations.Membership
}

func newStubOrgDirectory() *stubOrgDirectory {
	return &stubOrgDirectory{
		orgs:    map[string]organizations.Organization{},
		slugs:   map[string]string{},
		members: map[string]organizations.Membership{},
	}
}

func (f *stubOrgDirectory) CreateWithOwner(
	_ context.Context,
	in organizations.CreateInput,
) (organizations.Organization, organizations.Membership, error) {
	if _, taken := f.slugs[in.Slug]; taken {
		return organizations.Organization{}, organizations.Membership{}, organizations.ErrSlugTaken
	}

	org := organizations.Organization{
		ID: "org-" + in.Slug, Name: in.Name, Slug: in.Slug, Status: "active", Timezone: in.Timezone,
	}
	membership := organizations.Membership{
		ID: "m-" + in.OwnerID, OrganizationID: org.ID, UserID: in.OwnerID,
		RoleCode: "organization_owner", Status: "active",
	}

	f.orgs[org.ID] = org
	f.slugs[org.Slug] = org.ID
	f.members[org.ID+"|"+in.OwnerID] = membership

	return org, membership, nil
}

func (f *stubOrgDirectory) Get(_ context.Context, id string) (organizations.Organization, error) {
	if org, ok := f.orgs[id]; ok {
		return org, nil
	}

	return organizations.Organization{}, organizations.ErrNotFound
}

func (f *stubOrgDirectory) Membership(
	_ context.Context,
	organizationID, userID string,
) (organizations.Membership, error) {
	if membership, ok := f.members[organizationID+"|"+userID]; ok && membership.Status == "active" {
		return membership, nil
	}

	return organizations.Membership{}, organizations.ErrNotFound
}

func (f *stubOrgDirectory) ListMembershipsForUser(
	_ context.Context,
	userID string,
) ([]organizations.Membership, error) {
	out := make([]organizations.Membership, 0)

	for _, membership := range f.members {
		if membership.UserID == userID && membership.Status == "active" {
			out = append(out, membership)
		}
	}

	return out, nil
}

func (f *stubOrgDirectory) AddMember(
	_ context.Context,
	organizationID, userID, roleCode string,
) (organizations.Membership, error) {
	membership := organizations.Membership{
		ID: "m-" + userID, OrganizationID: organizationID, UserID: userID, RoleCode: roleCode, Status: "active",
	}
	f.members[organizationID+"|"+userID] = membership

	return membership, nil
}

func newAuthService(t *testing.T) (*auth.Service, *fakeUsers, *stubOrgDirectory) {
	t.Helper()

	users := newFakeUsers()
	orgs := newStubOrgDirectory()

	cfg := auth.Config{
		JWTSecret: "test-secret-value", AccessTTL: time.Minute, RefreshTTL: time.Hour,
		InviteTTL: time.Hour, PasswordMinLen: 10,
	}
	svc := auth.NewService(users, orgs, audit.NewService(noopAuditRepo{}), newFakeSessions(), newFakeInvites(), cfg, nil)

	return svc, users, orgs
}

func registerOwner(t *testing.T, svc *auth.Service, email string) auth.Result {
	t.Helper()

	result, err := svc.Register(context.Background(), auth.RegisterInput{
		Email:            email,
		Password:         goodPass,
		DisplayName:      "Owner",
		OrganizationName: "Acme Corp",
	})
	if err != nil {
		t.Fatalf("register %s: %v", email, err)
	}

	return result
}

// --- Register ---------------------------------------------------------------

func TestRegisterValidatesInput(t *testing.T) {
	t.Parallel()

	svc, _, _ := newAuthService(t)
	ctx := context.Background()

	cases := map[string]struct {
		in  auth.RegisterInput
		err error
	}{
		"missing email": {
			in:  auth.RegisterInput{Password: goodPass, DisplayName: "A", OrganizationName: "Acme"},
			err: auth.ErrInvalidInput,
		},
		"malformed email": {
			in:  auth.RegisterInput{Email: "not-an-email", Password: goodPass, DisplayName: "A", OrganizationName: "Acme"},
			err: auth.ErrInvalidInput,
		},
		"missing display name": {
			in:  auth.RegisterInput{Email: "a@acme.test", Password: goodPass, OrganizationName: "Acme"},
			err: auth.ErrInvalidInput,
		},
		"missing organization name": {
			in:  auth.RegisterInput{Email: "a@acme.test", Password: goodPass, DisplayName: "A"},
			err: auth.ErrInvalidInput,
		},
		"weak password": {
			in:  auth.RegisterInput{Email: "a@acme.test", Password: "short", DisplayName: "A", OrganizationName: "Acme"},
			err: auth.ErrWeakPassword,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if _, err := svc.Register(ctx, tc.in); !errors.Is(err, tc.err) {
				t.Fatalf("got %v, want %v", err, tc.err)
			}
		})
	}
}

func TestRegisterCreatesUserOrgAndSession(t *testing.T) {
	t.Parallel()

	svc, users, _ := newAuthService(t)

	result := registerOwner(t, svc, "Owner@Acme.test")

	if result.User.Email != "owner@acme.test" || result.User.Status != "active" {
		t.Fatalf("unexpected public user: %+v", result.User)
	}

	if result.Organization == nil || result.Organization.Slug != "acme-corp" {
		t.Fatalf("organization missing or slug not derived from the name: %+v", result.Organization)
	}

	if result.Tokens.AccessToken == "" || result.Tokens.RefreshToken == "" {
		t.Fatal("register must issue a token pair")
	}

	user, ok := users.byEmail["owner@acme.test"]
	if !ok || user.Status != "active" {
		t.Fatalf("active user not persisted: %+v", user)
	}
}

func TestListAndSwitchOrganizationsRotatesTenantSession(t *testing.T) {
	t.Parallel()

	svc, _, orgs := newAuthService(t)
	result := registerOwner(t, svc, "owner@acme.test")
	principal, err := security.ParseAccessToken("test-secret-value", result.Tokens.AccessToken)
	if err != nil {
		t.Fatalf("parse access token: %v", err)
	}
	orgs.orgs["org-second"] = organizations.Organization{
		ID: "org-second", Name: "Second Workspace", Slug: "second", Status: organizations.StatusActive(),
	}
	orgs.members["org-second|"+result.User.ID] = organizations.Membership{
		ID: "m-second", OrganizationID: "org-second", UserID: result.User.ID,
		RoleCode: "hr_admin", Status: "active",
	}

	choices, err := svc.ListOrganizations(context.Background(), result.User.ID)
	if err != nil || len(choices) != 2 {
		t.Fatalf("list organizations: %v (%+v)", err, choices)
	}
	switched, err := svc.SwitchOrganization(context.Background(), principal, "org-second")
	if err != nil {
		t.Fatalf("switch organization: %v", err)
	}
	if switched.Organization == nil || switched.Organization.ID != "org-second" {
		t.Fatalf("switched result = %+v", switched)
	}
	newPrincipal, err := security.ParseAccessToken("test-secret-value", switched.Tokens.AccessToken)
	if err != nil {
		t.Fatalf("parse switched token: %v", err)
	}
	if newPrincipal.OrganizationID != "org-second" || newPrincipal.RoleCode != "hr_admin" ||
		newPrincipal.SessionID == principal.SessionID {
		t.Fatalf("switched principal = %+v", newPrincipal)
	}
}

func TestRegisterRejectsDuplicateEmailAndSlug(t *testing.T) {
	t.Parallel()

	svc, _, _ := newAuthService(t)
	ctx := context.Background()

	registerOwner(t, svc, "owner@acme.test")

	duplicate := auth.RegisterInput{
		Email: "owner@acme.test", Password: goodPass, DisplayName: "B", OrganizationName: "Other",
	}
	if _, err := svc.Register(ctx, duplicate); !errors.Is(err, auth.ErrEmailTaken) {
		t.Fatalf("duplicate email got %v, want ErrEmailTaken", err)
	}

	sameSlug := auth.RegisterInput{
		Email: "second@acme.test", Password: goodPass, DisplayName: "C", OrganizationName: "Acme Corp",
	}
	if _, err := svc.Register(ctx, sameSlug); !errors.Is(err, organizations.ErrSlugTaken) {
		t.Fatalf("duplicate slug got %v, want ErrSlugTaken", err)
	}
}

// --- Login ------------------------------------------------------------------

func TestLoginSucceedsWithAndWithoutExplicitOrganization(t *testing.T) {
	t.Parallel()

	svc, _, _ := newAuthService(t)
	ctx := context.Background()

	registered := registerOwner(t, svc, "owner@acme.test")

	explicit, err := svc.Login(ctx, auth.LoginInput{
		Email: "OWNER@acme.test", Password: goodPass, OrganizationID: registered.Organization.ID,
	})
	if err != nil {
		t.Fatalf("login with explicit organization: %v", err)
	}

	if explicit.Organization == nil || explicit.Organization.ID != registered.Organization.ID {
		t.Fatalf("login returned the wrong organization: %+v", explicit.Organization)
	}

	if explicit.Tokens.AccessToken == "" || explicit.Tokens.RefreshToken == "" {
		t.Fatal("login must issue a token pair")
	}

	// Without an organizationId the single membership is picked up implicitly.
	implicit, err := svc.Login(ctx, auth.LoginInput{Email: "owner@acme.test", Password: goodPass})
	if err != nil {
		t.Fatalf("login without organization: %v", err)
	}

	if implicit.Organization == nil || implicit.Organization.ID != registered.Organization.ID {
		t.Fatalf("implicit login returned the wrong organization: %+v", implicit.Organization)
	}
}

func TestLoginRejectsWrongPasswordAndUnknownEmailUniformly(t *testing.T) {
	t.Parallel()

	svc, _, _ := newAuthService(t)
	ctx := context.Background()

	registered := registerOwner(t, svc, "owner@acme.test")

	_, wrongPasswordErr := svc.Login(ctx, auth.LoginInput{
		Email: "owner@acme.test", Password: "wrong-password-1", OrganizationID: registered.Organization.ID,
	})
	if !errors.Is(wrongPasswordErr, auth.ErrInvalidCredentials) {
		t.Fatalf("wrong password got %v, want ErrInvalidCredentials", wrongPasswordErr)
	}

	_, unknownEmailErr := svc.Login(ctx, auth.LoginInput{
		Email: "ghost@acme.test", Password: goodPass, OrganizationID: registered.Organization.ID,
	})
	if !errors.Is(unknownEmailErr, auth.ErrInvalidCredentials) {
		t.Fatalf("unknown email got %v, want ErrInvalidCredentials", unknownEmailErr)
	}
}

func TestLoginDoesNotLeakMembershipExistence(t *testing.T) {
	t.Parallel()

	svc, _, orgs := newAuthService(t)
	ctx := context.Background()

	// A real account with the correct password but no membership anywhere must
	// fail with the same sentinel as an unknown email (no oracle).
	if _, err := svc.CreateUserAccount(ctx, "lonely@acme.test", "Lonely", goodPass); err != nil {
		t.Fatalf("create account: %v", err)
	}

	if _, err := svc.Login(ctx, auth.LoginInput{Email: "lonely@acme.test", Password: goodPass}); !errors.Is(
		err, auth.ErrInvalidCredentials,
	) {
		t.Fatalf("memberless account got %v, want ErrInvalidCredentials", err)
	}

	// A valid account in org A asking for org B (which exists) must fail the
	// same way, not with a distinguishable membership error.
	registered := registerOwner(t, svc, "owner@acme.test")

	other, _, err := orgs.CreateWithOwner(ctx, organizations.CreateInput{
		Name: "Other Corp", Slug: "other-corp", OwnerID: "someone-else",
	})
	if err != nil {
		t.Fatalf("seed other org: %v", err)
	}

	_, loginErr := svc.Login(ctx, auth.LoginInput{
		Email: registered.User.Email, Password: goodPass, OrganizationID: other.ID,
	})
	if !errors.Is(loginErr, auth.ErrInvalidCredentials) {
		t.Fatalf("foreign membership got %v, want ErrInvalidCredentials", loginErr)
	}
}

// --- Refresh ----------------------------------------------------------------

func splitRefreshToken(t *testing.T, tokens auth.TokenPair) (string, string) {
	t.Helper()

	refresh, sessionID, err := auth.ParseRefreshToken(tokens.RefreshToken)
	if err != nil {
		t.Fatalf("parse refresh token: %v", err)
	}

	return refresh, sessionID
}

func TestRefreshRotatesTokensAndRejectsReuse(t *testing.T) {
	t.Parallel()

	svc, _, _ := newAuthService(t)
	ctx := context.Background()

	registered := registerOwner(t, svc, "owner@acme.test")
	refresh, sessionID := splitRefreshToken(t, registered.Tokens)

	rotated, err := svc.Refresh(ctx, sessionID, refresh)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}

	if rotated.AccessToken == "" || rotated.RefreshToken == registered.Tokens.RefreshToken {
		t.Fatal("refresh must rotate to a new token pair")
	}

	// The rotated-out refresh token must be rejected (the session is gone).
	if _, err := svc.Refresh(ctx, sessionID, refresh); !errors.Is(err, auth.ErrSessionInvalid) {
		t.Fatalf("reused refresh token got %v, want ErrSessionInvalid", err)
	}
}

func TestRefreshRejectsUnknownSession(t *testing.T) {
	t.Parallel()

	svc, _, _ := newAuthService(t)

	if _, err := svc.Refresh(context.Background(), "missing-session", "token"); !errors.Is(
		err, auth.ErrSessionInvalid,
	) {
		t.Fatalf("got %v, want ErrSessionInvalid", err)
	}
}

func TestRefreshTokenMismatchRevokesSession(t *testing.T) {
	t.Parallel()

	svc, _, _ := newAuthService(t)
	ctx := context.Background()

	registered := registerOwner(t, svc, "owner@acme.test")
	refresh, sessionID := splitRefreshToken(t, registered.Tokens)

	// A mismatched token for a real session ID kills the session (theft
	// detection): even the legitimate token is rejected afterwards.
	if _, err := svc.Refresh(ctx, sessionID, "forged-token"); !errors.Is(err, auth.ErrSessionInvalid) {
		t.Fatalf("forged token got %v, want ErrSessionInvalid", err)
	}

	if _, err := svc.Refresh(ctx, sessionID, refresh); !errors.Is(err, auth.ErrSessionInvalid) {
		t.Fatalf("session should be revoked after a mismatch, got %v", err)
	}
}

// --- Logout -----------------------------------------------------------------

func TestLogoutRevokesSession(t *testing.T) {
	t.Parallel()

	svc, _, _ := newAuthService(t)
	ctx := context.Background()

	registered := registerOwner(t, svc, "owner@acme.test")
	refresh, sessionID := splitRefreshToken(t, registered.Tokens)

	if err := svc.Logout(ctx, sessionID); err != nil {
		t.Fatalf("logout: %v", err)
	}

	if _, err := svc.Refresh(ctx, sessionID, refresh); !errors.Is(err, auth.ErrSessionInvalid) {
		t.Fatalf("refresh after logout got %v, want ErrSessionInvalid", err)
	}
}

// Ensure the bearer scheme stays stable for clients.
func TestIssuedTokensUseBearerScheme(t *testing.T) {
	t.Parallel()

	svc, _, _ := newAuthService(t)

	registered := registerOwner(t, svc, "owner@acme.test")

	if registered.Tokens.TokenType != "Bearer" || registered.Tokens.ExpiresIn <= 0 {
		t.Fatalf("unexpected token metadata: %+v", registered.Tokens)
	}

	if !strings.Contains(registered.Tokens.RefreshToken, ".") {
		t.Fatalf("refresh token should embed the session ID, got %q", registered.Tokens.RefreshToken)
	}
}
