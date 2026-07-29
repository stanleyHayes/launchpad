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

const (
	testOrg   = "org-test"
	testActor = "actor-1"
	goodPass  = "strong-password-1"
)

// --- in-memory fakes -------------------------------------------------------

type fakeUsers struct {
	byID           map[string]auth.User
	byEmail        map[string]auth.User
	failUpdateOnce bool
}

func newFakeUsers() *fakeUsers {
	return &fakeUsers{byID: map[string]auth.User{}, byEmail: map[string]auth.User{}}
}

func (f *fakeUsers) EnsureIndexes(context.Context) error { return nil }

func (f *fakeUsers) Create(_ context.Context, user auth.User) error {
	if _, ok := f.byEmail[user.Email]; ok {
		return auth.ErrEmailTaken
	}

	f.byID[user.ID] = user
	f.byEmail[user.Email] = user

	return nil
}

func (f *fakeUsers) GetByEmail(_ context.Context, email string) (auth.User, error) {
	if user, ok := f.byEmail[strings.ToLower(email)]; ok {
		return user, nil
	}

	return auth.User{}, auth.ErrInvalidCredentials
}

func (f *fakeUsers) GetByID(_ context.Context, id string) (auth.User, error) {
	if user, ok := f.byID[id]; ok {
		return user, nil
	}

	return auth.User{}, auth.ErrInvalidCredentials
}

func (f *fakeUsers) Update(_ context.Context, user auth.User) error {
	if f.failUpdateOnce {
		f.failUpdateOnce = false

		return errors.New("transient update failure")
	}

	if _, ok := f.byID[user.ID]; !ok {
		return auth.ErrInvalidCredentials
	}

	f.byID[user.ID] = user
	f.byEmail[user.Email] = user

	return nil
}

type fakeInvites struct {
	byHash map[string]auth.Invitation
}

func newFakeInvites() *fakeInvites {
	return &fakeInvites{byHash: map[string]auth.Invitation{}}
}

func (f *fakeInvites) EnsureIndexes(context.Context) error { return nil }

func (f *fakeInvites) Save(_ context.Context, invitation auth.Invitation) error {
	f.byHash[invitation.TokenHash] = invitation

	return nil
}

func (f *fakeInvites) Consume(_ context.Context, tokenHash string) (auth.Invitation, error) {
	invitation, ok := f.byHash[tokenHash]
	if !ok || !invitation.ExpiresAt.After(time.Now().UTC()) {
		return auth.Invitation{}, auth.ErrInvitationInvalid
	}

	delete(f.byHash, tokenHash)

	return invitation, nil
}

type fakeSessions struct {
	saved map[string][3]string
}

func newFakeSessions() *fakeSessions {
	return &fakeSessions{saved: map[string][3]string{}}
}

func (f *fakeSessions) Save(_ context.Context, sessionID, userID, orgID, refreshHash string) error {
	f.saved[sessionID] = [3]string{userID, orgID, refreshHash}

	return nil
}

func (f *fakeSessions) Get(_ context.Context, sessionID string) (string, string, string, error) {
	value, ok := f.saved[sessionID]
	if !ok {
		return "", "", "", auth.ErrSessionInvalid
	}

	return value[0], value[1], value[2], nil
}

func (f *fakeSessions) Delete(_ context.Context, sessionID string) error {
	delete(f.saved, sessionID)

	return nil
}

func (f *fakeSessions) DeleteForUser(_ context.Context, userID string) error {
	for sessionID, value := range f.saved {
		if value[0] == userID {
			delete(f.saved, sessionID)
		}
	}

	return nil
}

type fakeOrgs struct {
	orgs    map[string]organizations.Organization
	members map[string]organizations.Membership
}

func newFakeOrgs() *fakeOrgs {
	return &fakeOrgs{orgs: map[string]organizations.Organization{}, members: map[string]organizations.Membership{}}
}

func (f *fakeOrgs) CreateWithOwner(
	context.Context,
	organizations.CreateInput,
) (organizations.Organization, organizations.Membership, error) {
	return organizations.Organization{}, organizations.Membership{}, nil
}

func (f *fakeOrgs) Get(_ context.Context, id string) (organizations.Organization, error) {
	if org, ok := f.orgs[id]; ok {
		return org, nil
	}

	return organizations.Organization{}, errors.New("organization not found")
}

func (f *fakeOrgs) Membership(_ context.Context, organizationID, userID string) (organizations.Membership, error) {
	if membership, ok := f.members[organizationID+"|"+userID]; ok {
		return membership, nil
	}

	return organizations.Membership{}, errors.New("membership not found")
}

func (f *fakeOrgs) ListMembershipsForUser(
	_ context.Context,
	userID string,
) ([]organizations.Membership, error) {
	out := make([]organizations.Membership, 0)

	for _, membership := range f.members {
		if membership.UserID == userID {
			out = append(out, membership)
		}
	}

	return out, nil
}

func (f *fakeOrgs) AddMember(
	_ context.Context,
	organizationID, userID, roleCode string,
) (organizations.Membership, error) {
	membership := organizations.Membership{
		ID: "m-" + userID, OrganizationID: organizationID, UserID: userID, RoleCode: roleCode, Status: "active",
	}
	f.members[organizationID+"|"+userID] = membership

	return membership, nil
}

type noopAuditRepo struct{}

func (noopAuditRepo) EnsureIndexes(context.Context) error      { return nil }
func (noopAuditRepo) Write(context.Context, audit.Event) error { return nil }

func (noopAuditRepo) ListByOrganization(context.Context, string, int64) ([]audit.Event, error) {
	return nil, nil
}

func (noopAuditRepo) ListAll(context.Context, int64) ([]audit.Event, error) {
	return nil, nil
}

func newInviteService(t *testing.T) (*auth.Service, *fakeUsers, *fakeInvites) {
	t.Helper()

	users := newFakeUsers()
	invites := newFakeInvites()
	orgs := newFakeOrgs()
	orgs.orgs[testOrg] = organizations.Organization{ID: testOrg, Name: "Acme", Slug: "acme", Status: "active"}

	cfg := auth.Config{
		JWTSecret: "test-secret-value", AccessTTL: time.Minute, RefreshTTL: time.Hour,
		InviteTTL: time.Hour, PasswordMinLen: 10,
	}
	svc := auth.NewService(users, orgs, audit.NewService(noopAuditRepo{}), newFakeSessions(), invites, cfg, nil)

	return svc, users, invites
}

// --- tests -----------------------------------------------------------------

func TestIssueInvitationCreatesPendingUserAndToken(t *testing.T) {
	t.Parallel()

	svc, users, invites := newInviteService(t)

	token, err := svc.IssueInvitation(context.Background(), testOrg, "new@acme.test", "New Hire", "employee", testActor)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	if !strings.HasPrefix(token, "invite_") {
		t.Fatalf("token missing prefix: %q", token)
	}

	user, ok := users.byEmail["new@acme.test"]
	if !ok {
		t.Fatal("pending user not created")
	}

	if user.Status != "invited" || user.PasswordHash != "" {
		t.Fatalf("pending user should be invited with no password, got status=%q", user.Status)
	}

	invitation, ok := invites.byHash[security.HashToken(token)]
	if !ok || invitation.UserID != user.ID || invitation.OrganizationID != testOrg || invitation.RoleCode != "employee" {
		t.Fatalf("invitation not stored correctly: %+v", invitation)
	}
}

func TestIssueInvitationRejectsExistingEmail(t *testing.T) {
	t.Parallel()

	svc, users, _ := newInviteService(t)
	users.byEmail["taken@acme.test"] = auth.User{ID: "u-1", Email: "taken@acme.test", Status: "active"}

	_, err := svc.IssueInvitation(context.Background(), testOrg, "taken@acme.test", "Dup", "employee", testActor)
	if !errors.Is(err, auth.ErrEmailTaken) {
		t.Fatalf("got %v, want ErrEmailTaken", err)
	}
}

func TestIssueInvitationRejectsPrivilegedOrUnknownRole(t *testing.T) {
	t.Parallel()

	svc, _, _ := newInviteService(t)
	ctx := context.Background()

	for _, role := range []string{"organization_owner", "superuser"} {
		_, err := svc.IssueInvitation(ctx, testOrg, "x@acme.test", "X", role, testActor)
		if !errors.Is(err, auth.ErrInvalidInput) {
			t.Fatalf("role %q: got %v, want ErrInvalidInput", role, err)
		}
	}
}

func TestAcceptInvitationActivatesLogsInAndIsSingleUse(t *testing.T) {
	t.Parallel()

	svc, users, _ := newInviteService(t)
	ctx := context.Background()

	token, err := svc.IssueInvitation(ctx, testOrg, "hire@acme.test", "Hire", "employee", testActor)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	result, err := svc.AcceptInvitation(ctx, token, goodPass)
	if err != nil {
		t.Fatalf("accept: %v", err)
	}

	if result.Tokens.AccessToken == "" || result.Organization == nil || result.Organization.ID != testOrg {
		t.Fatalf("accept did not return a session for the org: %+v", result)
	}

	user := users.byEmail["hire@acme.test"]
	if user.Status != "active" || !security.CheckPassword(user.PasswordHash, goodPass) {
		t.Fatalf("user not activated with the chosen password: status=%q", user.Status)
	}

	// The invited user can now log in with the password they set.
	login := auth.LoginInput{Email: "hire@acme.test", Password: goodPass, OrganizationID: testOrg}
	if _, err := svc.Login(ctx, login); err != nil {
		t.Fatalf("login after accept failed: %v", err)
	}

	// The token is single-use.
	if _, err := svc.AcceptInvitation(ctx, token, goodPass); !errors.Is(err, auth.ErrInvitationInvalid) {
		t.Fatalf("second accept got %v, want ErrInvitationInvalid", err)
	}
}

func TestIssueInvitationReusesPendingInviteOnRetry(t *testing.T) {
	t.Parallel()

	svc, users, _ := newInviteService(t)
	ctx := context.Background()

	first, err := svc.IssueInvitation(ctx, testOrg, "hire@acme.test", "Hire", "employee", testActor)
	if err != nil {
		t.Fatalf("first issue: %v", err)
	}

	// Re-inviting the same pending email (e.g. after a prior partial failure)
	// must succeed with a fresh token, not 409, and must not create a second user.
	second, err := svc.IssueInvitation(ctx, testOrg, "hire@acme.test", "Hire", "employee", testActor)
	if err != nil {
		t.Fatalf("re-issue should reuse the pending invite, got %v", err)
	}

	if second == first || second == "" {
		t.Fatalf("expected a fresh token on re-issue, got %q (first %q)", second, first)
	}

	if len(users.byID) != 1 {
		t.Fatalf("re-issue must not create a second user, have %d", len(users.byID))
	}

	// The freshly issued token still activates the account.
	if _, err := svc.AcceptInvitation(ctx, second, goodPass); err != nil {
		t.Fatalf("accept re-issued token: %v", err)
	}
}

func TestAcceptInvitationConsumesTokenBeforeActivation(t *testing.T) {
	t.Parallel()

	svc, users, _ := newInviteService(t)
	ctx := context.Background()

	token, err := svc.IssueInvitation(ctx, testOrg, "hire@acme.test", "Hire", "employee", testActor)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	// The token is consumed atomically before activation, so even a transient
	// failure while activating burns it — strict single-use under concurrency.
	users.failUpdateOnce = true

	if _, err := svc.AcceptInvitation(ctx, token, goodPass); err == nil {
		t.Fatal("expected the transient update failure to surface")
	}

	if _, err := svc.AcceptInvitation(ctx, token, goodPass); !errors.Is(err, auth.ErrInvitationInvalid) {
		t.Fatalf("burned token should be rejected, got %v", err)
	}

	// The account is healed by issuing a fresh invitation (re-uses the
	// pending user) and accepting the new token.
	fresh, err := svc.IssueInvitation(ctx, testOrg, "hire@acme.test", "Hire", "employee", testActor)
	if err != nil {
		t.Fatalf("re-issue after burned token: %v", err)
	}

	if _, err := svc.AcceptInvitation(ctx, fresh, goodPass); err != nil {
		t.Fatalf("accept re-issued token: %v", err)
	}
}

func TestAcceptInvitationRejectsWeakPasswordWithoutConsuming(t *testing.T) {
	t.Parallel()

	svc, _, _ := newInviteService(t)
	ctx := context.Background()

	token, err := svc.IssueInvitation(ctx, testOrg, "hire@acme.test", "Hire", "employee", testActor)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	if _, err := svc.AcceptInvitation(ctx, token, "short"); !errors.Is(err, auth.ErrWeakPassword) {
		t.Fatalf("weak password got %v, want ErrWeakPassword", err)
	}

	// A weak-password attempt must not have consumed the token.
	if _, err := svc.AcceptInvitation(ctx, token, goodPass); err != nil {
		t.Fatalf("token should still be valid after a weak-password attempt, got %v", err)
	}
}

func TestAcceptInvitationRejectsExpired(t *testing.T) {
	t.Parallel()

	svc, _, invites := newInviteService(t)
	ctx := context.Background()

	token, err := svc.IssueInvitation(ctx, testOrg, "hire@acme.test", "Hire", "employee", testActor)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	// Force the stored invitation to be expired.
	hash := security.HashToken(token)
	invitation := invites.byHash[hash]
	invitation.ExpiresAt = time.Now().UTC().Add(-time.Minute)
	invites.byHash[hash] = invitation

	if _, err := svc.AcceptInvitation(ctx, token, goodPass); !errors.Is(err, auth.ErrInvitationInvalid) {
		t.Fatalf("expired accept got %v, want ErrInvitationInvalid", err)
	}
}

func (f *fakeInvites) DeleteForOrganization(context.Context, string) (int64, error) {
	return 0, nil
}

func (noopAuditRepo) CountByOrganization(context.Context, string) (int64, error) {
	return 0, nil
}

func (noopAuditRepo) DeleteForOrganization(context.Context, string) (int64, error) {
	return 0, nil
}
