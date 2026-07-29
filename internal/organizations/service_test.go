package organizations_test

import (
	"context"
	"errors"
	"testing"

	"launchpad/internal/organizations"
)

// --- in-memory fake ---------------------------------------------------------

// fakeOrgRepo mirrors the mongo store semantics that matter to the service:
// unique slugs, GetMembership returning only active memberships, and
// UpdateMembershipStatus failing with ErrNotFound for unknown members.
type fakeOrgRepo struct {
	orgs    map[string]organizations.Organization
	slugs   map[string]string
	members map[string]organizations.Membership
}

func newFakeOrgRepo() *fakeOrgRepo {
	return &fakeOrgRepo{
		orgs:    map[string]organizations.Organization{},
		slugs:   map[string]string{},
		members: map[string]organizations.Membership{},
	}
}

func memberKey(organizationID, userID string) string { return organizationID + "|" + userID }

func (f *fakeOrgRepo) EnsureIndexes(context.Context) error { return nil }

func (f *fakeOrgRepo) CreateOrganization(_ context.Context, org organizations.Organization) error {
	if _, taken := f.slugs[org.Slug]; taken {
		return organizations.ErrSlugTaken
	}

	f.orgs[org.ID] = org
	f.slugs[org.Slug] = org.ID

	return nil
}

func (f *fakeOrgRepo) DeleteOrganization(_ context.Context, id string) error {
	org, ok := f.orgs[id]
	if !ok {
		return organizations.ErrNotFound
	}

	delete(f.orgs, id)
	delete(f.slugs, org.Slug)

	return nil
}

func (f *fakeOrgRepo) GetByID(_ context.Context, id string) (organizations.Organization, error) {
	if org, ok := f.orgs[id]; ok {
		return org, nil
	}

	return organizations.Organization{}, organizations.ErrNotFound
}

func (f *fakeOrgRepo) GetBySlug(_ context.Context, slug string) (organizations.Organization, error) {
	if id, ok := f.slugs[slug]; ok {
		return f.orgs[id], nil
	}

	return organizations.Organization{}, organizations.ErrNotFound
}

func (f *fakeOrgRepo) Update(_ context.Context, org organizations.Organization) error {
	if _, ok := f.orgs[org.ID]; !ok {
		return organizations.ErrNotFound
	}

	f.orgs[org.ID] = org

	return nil
}

func (f *fakeOrgRepo) List(context.Context) ([]organizations.Organization, error) {
	out := make([]organizations.Organization, 0, len(f.orgs))
	for _, org := range f.orgs {
		out = append(out, org)
	}

	return out, nil
}

func (f *fakeOrgRepo) CountByStatus(context.Context) (map[string]int64, error) {
	counts := map[string]int64{}

	for _, org := range f.orgs {
		counts[org.Status]++
	}

	return counts, nil
}

func (f *fakeOrgRepo) CreateMembership(_ context.Context, membership organizations.Membership) error {
	f.members[memberKey(membership.OrganizationID, membership.UserID)] = membership

	return nil
}

func (f *fakeOrgRepo) GetMembership(
	_ context.Context,
	organizationID, userID string,
) (organizations.Membership, error) {
	if membership, ok := f.members[memberKey(organizationID, userID)]; ok && membership.Status == "active" {
		return membership, nil
	}

	return organizations.Membership{}, organizations.ErrNotFound
}

func (f *fakeOrgRepo) ListMembershipsByUser(
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

func (f *fakeOrgRepo) UpdateMembershipStatus(
	_ context.Context,
	organizationID, userID, status string,
) error {
	key := memberKey(organizationID, userID)

	membership, ok := f.members[key]
	if !ok {
		return organizations.ErrNotFound
	}

	membership.Status = status
	f.members[key] = membership

	return nil
}

func (f *fakeOrgRepo) MembershipExists(_ context.Context, organizationID, userID string) (bool, error) {
	_, ok := f.members[memberKey(organizationID, userID)]

	return ok, nil
}

func (f *fakeOrgRepo) ListMemberships(_ context.Context, organizationID string) ([]organizations.Membership, error) {
	out := make([]organizations.Membership, 0)

	for _, membership := range f.members {
		if membership.OrganizationID == organizationID && membership.Status == "active" {
			out = append(out, membership)
		}
	}

	return out, nil
}

func (f *fakeOrgRepo) UpdateMembershipRole(_ context.Context, organizationID, userID, roleCode string) error {
	key := memberKey(organizationID, userID)

	membership, ok := f.members[key]
	if !ok {
		return organizations.ErrNotFound
	}

	membership.RoleCode = roleCode
	f.members[key] = membership

	return nil
}

func (f *fakeOrgRepo) CountMembershipsByRole(_ context.Context, organizationID, roleCode string) (int64, error) {
	var count int64

	for _, membership := range f.members {
		if membership.OrganizationID == organizationID &&
			membership.RoleCode == roleCode &&
			membership.Status == "active" {
			count++
		}
	}

	return count, nil
}

func newOrgService() (*organizations.Service, *fakeOrgRepo) {
	repo := newFakeOrgRepo()

	return organizations.NewService(repo, nil, nil, nil), repo
}

func createOrg(t *testing.T, svc *organizations.Service) organizations.Organization {
	t.Helper()

	org, _, err := svc.CreateWithOwner(context.Background(), organizations.CreateInput{
		Name: "Acme", Slug: "acme", OwnerID: "user-1",
	})
	if err != nil {
		t.Fatalf("create organization: %v", err)
	}

	return org
}

// --- tests ------------------------------------------------------------------

func TestCreateWithOwnerAppliesDefaultsAndOwnerMembership(t *testing.T) {
	t.Parallel()

	svc, repo := newOrgService()
	ctx := context.Background()

	org, membership, err := svc.CreateWithOwner(ctx, organizations.CreateInput{
		Name: "Acme Corp", Slug: "ACME-corp", OwnerID: "user-1",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if org.Slug != "acme-corp" {
		t.Fatalf("slug should be lowercased, got %q", org.Slug)
	}

	if org.Status != "trial" || org.PlanCode != "starter" || org.Timezone != "UTC" {
		t.Fatalf("defaults not applied: %+v", org)
	}

	if membership.OrganizationID != org.ID || membership.UserID != "user-1" ||
		membership.RoleCode != "organization_owner" || membership.Status != "active" {
		t.Fatalf("owner membership not created correctly: %+v", membership)
	}

	// The organization is retrievable by id and by slug.
	if _, err := svc.Get(ctx, org.ID); err != nil {
		t.Fatalf("get by id: %v", err)
	}

	bySlug, err := svc.GetBySlug(ctx, "acme-corp")
	if err != nil || bySlug.ID != org.ID {
		t.Fatalf("get by slug: %v (%+v)", err, bySlug)
	}

	if _, ok := repo.slugs["acme-corp"]; !ok {
		t.Fatal("slug not indexed by the repository")
	}
}

func TestCreateWithOwnerValidatesInput(t *testing.T) {
	t.Parallel()

	svc, _ := newOrgService()
	ctx := context.Background()

	for _, in := range []organizations.CreateInput{
		{Name: "", Slug: "acme", OwnerID: "user-1"},
		{Name: "Acme", Slug: "Bad Slug!", OwnerID: "user-1"},
		{Name: "Acme", Slug: "-leading-dash", OwnerID: "user-1"},
	} {
		if _, _, err := svc.CreateWithOwner(ctx, in); !errors.Is(err, organizations.ErrInvalidInput) {
			t.Fatalf("input %+v: got %v, want ErrInvalidInput", in, err)
		}
	}

	createOrg(t, svc)

	if _, _, err := svc.CreateWithOwner(ctx, organizations.CreateInput{
		Name: "Copycat", Slug: "acme", OwnerID: "user-2",
	}); !errors.Is(err, organizations.ErrSlugTaken) {
		t.Fatalf("duplicate slug got %v, want ErrSlugTaken", err)
	}
}

func TestAddMemberAndMembershipLookup(t *testing.T) {
	t.Parallel()

	svc, _ := newOrgService()
	ctx := context.Background()

	org := createOrg(t, svc)

	for _, tc := range [][3]string{
		{"", "user-2", "employee"},
		{org.ID, "", "employee"},
		{org.ID, "user-2", ""},
	} {
		if _, err := svc.AddMember(ctx, tc[0], tc[1], tc[2]); !errors.Is(err, organizations.ErrInvalidInput) {
			t.Fatalf("input %v: got %v, want ErrInvalidInput", tc, err)
		}
	}

	membership, err := svc.AddMember(ctx, org.ID, "user-2", "employee")
	if err != nil {
		t.Fatalf("add member: %v", err)
	}

	if membership.Status != "active" || membership.RoleCode != "employee" {
		t.Fatalf("unexpected membership: %+v", membership)
	}

	if _, err := svc.Membership(ctx, org.ID, "user-2"); err != nil {
		t.Fatalf("membership lookup: %v", err)
	}

	memberships, err := svc.ListMembershipsForUser(ctx, "user-2")
	if err != nil || len(memberships) != 1 {
		t.Fatalf("list memberships: %v (%+v)", err, memberships)
	}

	if _, err := svc.Membership(ctx, org.ID, "stranger"); !errors.Is(err, organizations.ErrNotFound) {
		t.Fatalf("non-member got %v, want ErrNotFound", err)
	}
}

func TestSetMembershipStatusSuspendsAndReactivates(t *testing.T) {
	t.Parallel()

	svc, _ := newOrgService()
	ctx := context.Background()

	org := createOrg(t, svc)

	if _, err := svc.AddMember(ctx, org.ID, "user-2", "employee"); err != nil {
		t.Fatalf("add member: %v", err)
	}

	if err := svc.SetMembershipStatus(ctx, org.ID, "user-2", false); err != nil {
		t.Fatalf("suspend: %v", err)
	}

	// A suspended member keeps the membership record but fails the
	// active-membership lookup used by login.
	if _, err := svc.Membership(ctx, org.ID, "user-2"); !errors.Is(err, organizations.ErrNotFound) {
		t.Fatalf("suspended membership got %v, want ErrNotFound", err)
	}

	exists, err := svc.HasMembership(ctx, org.ID, "user-2")
	if err != nil || !exists {
		t.Fatalf("suspended membership should still exist: %v (%v)", exists, err)
	}

	if err := svc.SetMembershipStatus(ctx, org.ID, "user-2", true); err != nil {
		t.Fatalf("reactivate: %v", err)
	}

	if _, err := svc.Membership(ctx, org.ID, "user-2"); err != nil {
		t.Fatalf("reactivated membership should load: %v", err)
	}

	// Validation and unknown members.
	if err := svc.SetMembershipStatus(ctx, "", "user-2", true); !errors.Is(err, organizations.ErrInvalidInput) {
		t.Fatalf("empty organization got %v, want ErrInvalidInput", err)
	}

	if err := svc.SetMembershipStatus(ctx, org.ID, "ghost", true); !errors.Is(err, organizations.ErrNotFound) {
		t.Fatalf("unknown member got %v, want ErrNotFound", err)
	}
}

func TestUpdateOrganizationValidatesAndPersists(t *testing.T) {
	t.Parallel()

	svc, _ := newOrgService()
	ctx := context.Background()

	org := createOrg(t, svc)

	blank := "  "
	if _, err := svc.Update(ctx, org.ID, organizations.UpdateInput{Name: &blank}); !errors.Is(
		err, organizations.ErrInvalidInput,
	) {
		t.Fatalf("blank name got %v, want ErrInvalidInput", err)
	}

	badColor := organizations.Branding{PrimaryColor: "red"}
	if _, err := svc.Update(ctx, org.ID, organizations.UpdateInput{Branding: &badColor}); !errors.Is(
		err, organizations.ErrInvalidInput,
	) {
		t.Fatalf("bad color got %v, want ErrInvalidInput", err)
	}

	badLogo := organizations.Branding{LogoURL: "ftp://example.test/logo.png"}
	if _, err := svc.Update(ctx, org.ID, organizations.UpdateInput{Branding: &badLogo}); !errors.Is(
		err, organizations.ErrInvalidInput,
	) {
		t.Fatalf("bad logo URL got %v, want ErrInvalidInput", err)
	}

	badDomain := "https://example.test/path"
	if _, err := svc.Update(ctx, org.ID, organizations.UpdateInput{CustomDomain: &badDomain}); !errors.Is(
		err, organizations.ErrInvalidInput,
	) {
		t.Fatalf("bad custom domain got %v, want ErrInvalidInput", err)
	}

	name := "Acme Incorporated"
	customDomain := "Onboarding.Example.com"
	branding := organizations.Branding{PrimaryColor: "#0A0B0D", LogoURL: "https://example.test/logo.png"}

	updated, err := svc.Update(ctx, org.ID, organizations.UpdateInput{
		Name: &name, Branding: &branding, CustomDomain: &customDomain,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	if updated.Name != name || updated.Branding.PrimaryColor != "#0A0B0D" ||
		updated.CustomDomain != "onboarding.example.com" {
		t.Fatalf("update not applied: %+v", updated)
	}

	if _, err := svc.Update(ctx, "ghost", organizations.UpdateInput{Name: &name}); !errors.Is(
		err, organizations.ErrNotFound,
	) {
		t.Fatalf("unknown organization got %v, want ErrNotFound", err)
	}
}

func TestSetStatusAndPlanCode(t *testing.T) {
	t.Parallel()

	svc, _ := newOrgService()
	ctx := context.Background()

	org := createOrg(t, svc)

	if _, err := svc.SetStatus(ctx, org.ID, "archived"); !errors.Is(err, organizations.ErrInvalidInput) {
		t.Fatalf("invalid status got %v, want ErrInvalidInput", err)
	}

	updated, err := svc.SetStatus(ctx, org.ID, "suspended")
	if err != nil || updated.Status != "suspended" {
		t.Fatalf("set status: %v (%+v)", err, updated)
	}

	updated, err = svc.SetStatus(ctx, org.ID, organizations.StatusClosed())
	if err != nil || updated.Status != organizations.StatusClosed() {
		t.Fatalf("close organization: %v (%+v)", err, updated)
	}
	if _, err := svc.SetStatus(ctx, org.ID, organizations.StatusActive()); !errors.Is(
		err, organizations.ErrInvalidInput,
	) {
		t.Fatalf("reactivate closed organization got %v, want ErrInvalidInput", err)
	}

	if _, err := svc.SetPlanCode(ctx, org.ID, "  "); !errors.Is(err, organizations.ErrInvalidInput) {
		t.Fatalf("blank plan code got %v, want ErrInvalidInput", err)
	}

	updated, err = svc.SetPlanCode(ctx, org.ID, "growth")
	if err != nil || updated.PlanCode != "growth" {
		t.Fatalf("set plan code: %v (%+v)", err, updated)
	}

	stored, err := svc.Get(ctx, org.ID)
	if err != nil || stored.PlanCode != "growth" || stored.Status != organizations.StatusClosed() {
		t.Fatalf("changes not persisted: %v (%+v)", err, stored)
	}
}

func TestUpdateSetupProgressIsDurableMonotonicAndCompletable(t *testing.T) {
	t.Parallel()

	svc, _ := newOrgService()
	ctx := context.Background()
	org := createOrg(t, svc)

	updated, err := svc.UpdateSetupProgress(ctx, org.ID, organizations.SetupProgressInput{Step: 4})
	if err != nil || updated.SetupStep != 4 {
		t.Fatalf("advance setup: %v (%+v)", err, updated)
	}
	updated, err = svc.UpdateSetupProgress(ctx, org.ID, organizations.SetupProgressInput{Step: 2})
	if err != nil || updated.SetupStep != 4 {
		t.Fatalf("setup moved backwards: %v (%+v)", err, updated)
	}
	if _, err := svc.UpdateSetupProgress(ctx, org.ID, organizations.SetupProgressInput{
		Step: 9, Completed: true,
	}); !errors.Is(err, organizations.ErrInvalidInput) {
		t.Fatalf("early completion got %v, want ErrInvalidInput", err)
	}
	updated, err = svc.UpdateSetupProgress(ctx, org.ID, organizations.SetupProgressInput{
		Step: 10, Completed: true,
	})
	if err != nil || updated.SetupStep != 10 || updated.SetupCompletedAt == nil {
		t.Fatalf("complete setup: %v (%+v)", err, updated)
	}
}

func TestCanManageOrganization(t *testing.T) {
	t.Parallel()

	for role, want := range map[string]bool{
		"organization_owner": true,
		"hr_admin":           true,
		"employee":           false,
		"platform_owner":     false,
	} {
		if got := organizations.CanManageOrganization(role); got != want {
			t.Fatalf("CanManageOrganization(%q)=%v, want %v", role, got, want)
		}
	}
}

func (f *fakeOrgRepo) DeleteForOrganization(context.Context, string) (int64, error) {
	return 0, nil
}
