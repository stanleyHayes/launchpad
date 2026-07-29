package organizations_test

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"launchpad/internal/organizations"
)

func TestSlugify(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"Acme Corp":     "acme-corp",
		"  Hello!!  ":   "hello",
		"LaunchPad Inc": "launchpad-inc",
	}
	for in, want := range tests {
		if got := organizations.Slugify(in); got != want {
			t.Fatalf("Slugify(%q)=%q want %q", in, got, want)
		}
	}
}

// --- invite + provisioning fakes ---------------------------------------------

type inviteTestRepo struct {
	orgs                map[string]organizations.Organization
	memberships         []organizations.Membership
	createMembershipErr error
	deletedOrgs         []string
}

func newInviteTestRepo() *inviteTestRepo {
	return &inviteTestRepo{orgs: map[string]organizations.Organization{}}
}

func (f *inviteTestRepo) EnsureIndexes(context.Context) error { return nil }

func (f *inviteTestRepo) CreateOrganization(_ context.Context, org organizations.Organization) error {
	for _, existing := range f.orgs {
		if existing.Slug == org.Slug {
			return organizations.ErrSlugTaken
		}
	}

	f.orgs[org.ID] = org

	return nil
}

func (f *inviteTestRepo) DeleteOrganization(_ context.Context, id string) error {
	delete(f.orgs, id)
	f.deletedOrgs = append(f.deletedOrgs, id)

	return nil
}

func (f *inviteTestRepo) GetByID(_ context.Context, id string) (organizations.Organization, error) {
	if org, ok := f.orgs[id]; ok {
		return org, nil
	}

	return organizations.Organization{}, organizations.ErrNotFound
}

func (f *inviteTestRepo) GetBySlug(_ context.Context, slug string) (organizations.Organization, error) {
	for _, org := range f.orgs {
		if org.Slug == slug {
			return org, nil
		}
	}

	return organizations.Organization{}, organizations.ErrNotFound
}

func (f *inviteTestRepo) Update(_ context.Context, org organizations.Organization) error {
	if _, ok := f.orgs[org.ID]; !ok {
		return organizations.ErrNotFound
	}

	f.orgs[org.ID] = org

	return nil
}

func (f *inviteTestRepo) List(context.Context) ([]organizations.Organization, error) {
	out := make([]organizations.Organization, 0, len(f.orgs))
	for _, org := range f.orgs {
		out = append(out, org)
	}

	return out, nil
}

func (f *inviteTestRepo) CountByStatus(context.Context) (map[string]int64, error) {
	out := map[string]int64{}
	for _, org := range f.orgs {
		out[org.Status]++
	}

	return out, nil
}

func (f *inviteTestRepo) CreateMembership(_ context.Context, membership organizations.Membership) error {
	if f.createMembershipErr != nil {
		return f.createMembershipErr
	}

	for _, existing := range f.memberships {
		if existing.OrganizationID == membership.OrganizationID && existing.UserID == membership.UserID {
			return errors.New("duplicate membership")
		}
	}

	f.memberships = append(f.memberships, membership)

	return nil
}

func (f *inviteTestRepo) GetMembership(
	_ context.Context,
	organizationID, userID string,
) (organizations.Membership, error) {
	for _, membership := range f.memberships {
		if membership.OrganizationID == organizationID &&
			membership.UserID == userID &&
			membership.Status == "active" {
			return membership, nil
		}
	}

	return organizations.Membership{}, organizations.ErrNotFound
}

func (f *inviteTestRepo) ListMembershipsByUser(
	_ context.Context,
	userID string,
) ([]organizations.Membership, error) {
	out := make([]organizations.Membership, 0)

	for _, membership := range f.memberships {
		if membership.UserID == userID && membership.Status == "active" {
			out = append(out, membership)
		}
	}

	return out, nil
}

func (f *inviteTestRepo) UpdateMembershipStatus(_ context.Context, organizationID, userID, status string) error {
	for i, membership := range f.memberships {
		if membership.OrganizationID == organizationID && membership.UserID == userID {
			f.memberships[i].Status = status

			return nil
		}
	}

	return organizations.ErrNotFound
}

func (f *inviteTestRepo) MembershipExists(_ context.Context, organizationID, userID string) (bool, error) {
	for _, membership := range f.memberships {
		if membership.OrganizationID == organizationID && membership.UserID == userID {
			return true, nil
		}
	}

	return false, nil
}

func (f *inviteTestRepo) ListMemberships(_ context.Context, organizationID string) ([]organizations.Membership, error) {
	out := make([]organizations.Membership, 0)

	for _, membership := range f.memberships {
		if membership.OrganizationID == organizationID && membership.Status == "active" {
			out = append(out, membership)
		}
	}

	return out, nil
}

func (f *inviteTestRepo) UpdateMembershipRole(_ context.Context, organizationID, userID, roleCode string) error {
	for i, membership := range f.memberships {
		if membership.OrganizationID == organizationID && membership.UserID == userID {
			f.memberships[i].RoleCode = roleCode

			return nil
		}
	}

	return organizations.ErrNotFound
}

func (f *inviteTestRepo) CountMembershipsByRole(_ context.Context, organizationID, roleCode string) (int64, error) {
	var count int64

	for _, membership := range f.memberships {
		if membership.OrganizationID == organizationID &&
			membership.RoleCode == roleCode &&
			membership.Status == "active" {
			count++
		}
	}

	return count, nil
}

type fakeAccountCreator struct {
	nextID  int
	byEmail map[string]string
}

func newFakeAccountCreator() *fakeAccountCreator {
	return &fakeAccountCreator{byEmail: map[string]string{}}
}

func (f *fakeAccountCreator) CreateUserAccount(_ context.Context, email, _, _ string) (string, error) {
	if _, ok := f.byEmail[email]; ok {
		return "", organizations.ErrInviteEmailTaken
	}

	f.nextID++
	id := "user-" + strconv.Itoa(f.nextID)
	f.byEmail[email] = id

	return id, nil
}

func (f *fakeAccountCreator) FindUserByEmail(_ context.Context, email string) (string, error) {
	if id, ok := f.byEmail[email]; ok {
		return id, nil
	}

	return "", errors.New("account not found")
}

func newInviteService() (*organizations.Service, *inviteTestRepo, *fakeAccountCreator) {
	repo := newInviteTestRepo()
	accounts := newFakeAccountCreator()

	return organizations.NewService(repo, accounts, nil, nil), repo, accounts
}

func inviteInput(email string) organizations.InviteMemberInput {
	return organizations.InviteMemberInput{
		Email:       email,
		DisplayName: "Invited Admin",
		Password:    "sufficiently-long-password",
		RoleCode:    organizations.RoleHRAdmin(),
	}
}

func TestInviteMemberCreatesAccountAndMembership(t *testing.T) {
	t.Parallel()

	svc, repo, accounts := newInviteService()

	membership, err := svc.InviteMember(context.Background(), "org-1", inviteInput("ada@acme.test"))
	if err != nil {
		t.Fatalf("invite: %v", err)
	}

	if membership.RoleCode != organizations.RoleHRAdmin() || membership.Status != "active" {
		t.Fatalf("unexpected membership: %+v", membership)
	}

	if accounts.byEmail["ada@acme.test"] != membership.UserID {
		t.Fatalf("membership not linked to the created account: %+v", membership)
	}

	if len(repo.memberships) != 1 {
		t.Fatalf("expected 1 membership, got %d", len(repo.memberships))
	}
}

func TestInviteMemberRejectsNonHRAdmin(t *testing.T) {
	t.Parallel()

	svc, _, accounts := newInviteService()

	in := inviteInput("ada@acme.test")
	in.RoleCode = organizations.RoleEmployee()

	_, err := svc.InviteMember(context.Background(), "org-1", in)
	if !errors.Is(err, organizations.ErrInviteInvalidInput) {
		t.Fatalf("got %v, want ErrInviteInvalidInput", err)
	}

	if len(accounts.byEmail) != 0 {
		t.Fatal("no account should be created for a rejected role")
	}
}

func TestInviteMemberRetryReusesOrphanAccount(t *testing.T) {
	t.Parallel()

	svc, repo, accounts := newInviteService()
	ctx := context.Background()

	// First attempt: account created, membership write fails.
	repo.createMembershipErr = errors.New("mongo unreachable")

	if _, err := svc.InviteMember(ctx, "org-1", inviteInput("ada@acme.test")); err == nil {
		t.Fatal("expected membership failure")
	}

	// Retry: the orphaned account is reused instead of failing EMAIL_TAKEN.
	repo.createMembershipErr = nil

	membership, err := svc.InviteMember(ctx, "org-1", inviteInput("ada@acme.test"))
	if err != nil {
		t.Fatalf("retry: %v", err)
	}

	if len(accounts.byEmail) != 1 || membership.UserID != accounts.byEmail["ada@acme.test"] {
		t.Fatalf("retry should reuse the orphaned account: %+v", membership)
	}
}

func TestInviteMemberRetryAfterLateFailureIsIdempotent(t *testing.T) {
	t.Parallel()

	svc, repo, _ := newInviteService()
	ctx := context.Background()

	first, err := svc.InviteMember(ctx, "org-1", inviteInput("ada@acme.test"))
	if err != nil {
		t.Fatalf("invite: %v", err)
	}

	// Simulates a retry after the first attempt succeeded but the caller saw a
	// failure (e.g. audit write failed).
	second, err := svc.InviteMember(ctx, "org-1", inviteInput("ada@acme.test"))
	if err != nil {
		t.Fatalf("retry: %v", err)
	}

	if second.UserID != first.UserID {
		t.Fatalf("retry returned a different user: %q vs %q", second.UserID, first.UserID)
	}

	if len(repo.memberships) != 1 {
		t.Fatalf("retry must not duplicate the membership, got %d", len(repo.memberships))
	}
}

func TestInviteMemberRejectsForeignAccount(t *testing.T) {
	t.Parallel()

	svc, repo, accounts := newInviteService()

	foreignID, err := accounts.CreateUserAccount(context.Background(), "bob@acme.test", "Bob", "x")
	if err != nil {
		t.Fatalf("seed account: %v", err)
	}

	repo.memberships = append(repo.memberships, organizations.Membership{
		ID:             "m-1",
		OrganizationID: "org-2",
		UserID:         foreignID,
		RoleCode:       organizations.RoleHRAdmin(),
		Status:         "active",
	})

	_, err = svc.InviteMember(context.Background(), "org-1", inviteInput("bob@acme.test"))
	if !errors.Is(err, organizations.ErrInviteEmailTaken) {
		t.Fatalf("got %v, want ErrInviteEmailTaken", err)
	}
}

func TestCreateWithOwnerDeletesOrgWhenMembershipFails(t *testing.T) {
	t.Parallel()

	svc, repo, _ := newInviteService()
	ctx := context.Background()

	repo.createMembershipErr = errors.New("mongo unreachable")

	if _, _, err := svc.CreateWithOwner(ctx, organizations.CreateInput{
		Name: "Acme", Slug: "acme", OwnerID: "owner-1",
	}); err == nil {
		t.Fatal("expected membership failure")
	}

	if len(repo.orgs) != 0 || len(repo.deletedOrgs) != 1 {
		t.Fatalf("org should be compensated away: orgs=%v deleted=%v", repo.orgs, repo.deletedOrgs)
	}

	// The slug is not burned: the same slug can be registered after retry.
	repo.createMembershipErr = nil

	org, membership, err := svc.CreateWithOwner(ctx, organizations.CreateInput{
		Name: "Acme", Slug: "acme", OwnerID: "owner-1",
	})
	if err != nil {
		t.Fatalf("retry: %v", err)
	}

	if org.ID == "" || membership.UserID != "owner-1" {
		t.Fatalf("unexpected retry result: %+v %+v", org, membership)
	}
}

func (f *inviteTestRepo) DeleteForOrganization(context.Context, string) (int64, error) {
	return 0, nil
}
