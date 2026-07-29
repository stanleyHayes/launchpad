package platform_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"launchpad/internal/organizations"
	"launchpad/internal/platform"
)

// --- in-memory fakes --------------------------------------------------------

type fakeStaffRepo struct {
	staff  map[string]platform.Staff
	getErr error
}

func newFakeStaffRepo() *fakeStaffRepo {
	return &fakeStaffRepo{staff: map[string]platform.Staff{}}
}

func (f *fakeStaffRepo) EnsureIndexes(context.Context) error { return nil }

func (f *fakeStaffRepo) GetByUserID(_ context.Context, userID string) (platform.Staff, error) {
	if f.getErr != nil {
		return platform.Staff{}, f.getErr
	}

	if staff, ok := f.staff[userID]; ok {
		return staff, nil
	}

	return platform.Staff{}, platform.ErrNotFound
}

func (f *fakeStaffRepo) Create(_ context.Context, staff platform.Staff) error {
	f.staff[staff.UserID] = staff

	return nil
}

func (f *fakeStaffRepo) GetByID(_ context.Context, staffID string) (platform.Staff, error) {
	for _, staff := range f.staff {
		if staff.ID == staffID {
			return staff, nil
		}
	}

	return platform.Staff{}, platform.ErrNotFound
}

func (f *fakeStaffRepo) List(context.Context) ([]platform.Staff, error) {
	out := make([]platform.Staff, 0, len(f.staff))
	for _, staff := range f.staff {
		out = append(out, staff)
	}

	return out, nil
}

func (f *fakeStaffRepo) Update(_ context.Context, staff platform.Staff) error {
	if _, ok := f.staff[staff.UserID]; !ok {
		return platform.ErrNotFound
	}

	f.staff[staff.UserID] = staff

	return nil
}

type stubOrgDirectory struct {
	orgs map[string]organizations.Organization
}

func newStubOrgDirectory() *stubOrgDirectory {
	return &stubOrgDirectory{orgs: map[string]organizations.Organization{}}
}

func (f *stubOrgDirectory) List(context.Context) ([]organizations.Organization, error) {
	out := make([]organizations.Organization, 0, len(f.orgs))
	for _, org := range f.orgs {
		out = append(out, org)
	}

	return out, nil
}

func (f *stubOrgDirectory) Get(_ context.Context, id string) (organizations.Organization, error) {
	if org, ok := f.orgs[id]; ok {
		return org, nil
	}

	return organizations.Organization{}, organizations.ErrNotFound
}

func (f *stubOrgDirectory) SetStatus(
	_ context.Context,
	id, status string,
) (organizations.Organization, error) {
	org, ok := f.orgs[id]
	if !ok {
		return organizations.Organization{}, organizations.ErrNotFound
	}

	org.Status = status
	f.orgs[id] = org

	return org, nil
}

func (f *stubOrgDirectory) CountByStatus(context.Context) (map[string]int64, error) {
	counts := map[string]int64{}

	for _, org := range f.orgs {
		counts[org.Status]++
	}

	return counts, nil
}

type stubLeadCounter struct{ count int64 }

func (f stubLeadCounter) Count(context.Context) (int64, error) { return f.count, nil }

type stubTicketCounter struct{ open int64 }

func (f stubTicketCounter) CountOpen(context.Context) (int64, error) { return f.open, nil }

func newPlatformService(leads, openTickets int64) (*platform.Service, *fakeStaffRepo, *stubOrgDirectory) {
	repo := newFakeStaffRepo()
	orgs := newStubOrgDirectory()
	svc := platform.NewService(repo, orgs, stubLeadCounter{count: leads}, stubTicketCounter{open: openTickets})

	return svc, repo, orgs
}

// --- tests ------------------------------------------------------------------

func TestEnsureStaffCreatesActiveStaff(t *testing.T) {
	t.Parallel()

	svc, _, _ := newPlatformService(0, 0)
	ctx := context.Background()

	staff, err := svc.EnsureStaff(ctx, "user-1", platform.RoleOwner())
	if err != nil {
		t.Fatalf("ensure staff: %v", err)
	}

	if staff.ID == "" || staff.UserID != "user-1" || staff.RoleCode != "platform_owner" || staff.Status != "active" {
		t.Fatalf("unexpected staff record: %+v", staff)
	}

	roleCode, err := svc.StaffRoleByUserID(ctx, "user-1")
	if err != nil || roleCode != "platform_owner" {
		t.Fatalf("staff role lookup: %q, %v", roleCode, err)
	}
}

func TestEnsureStaffIsIdempotent(t *testing.T) {
	t.Parallel()

	svc, repo, _ := newPlatformService(0, 0)
	ctx := context.Background()

	first, err := svc.EnsureStaff(ctx, "user-1", platform.RoleOwner())
	if err != nil {
		t.Fatalf("ensure staff: %v", err)
	}

	second, err := svc.EnsureStaff(ctx, "user-1", platform.RoleOwner())
	if err != nil {
		t.Fatalf("re-ensure staff: %v", err)
	}

	if second.ID != first.ID {
		t.Fatalf("re-ensure must return the existing record, got new ID %q (was %q)", second.ID, first.ID)
	}

	if len(repo.staff) != 1 {
		t.Fatalf("re-ensure created a duplicate record, have %d", len(repo.staff))
	}
}

func TestEnsureStaffValidatesInput(t *testing.T) {
	t.Parallel()

	svc, _, _ := newPlatformService(0, 0)
	ctx := context.Background()

	for _, tc := range [][2]string{
		{"", "platform_owner"},
		{"user-1", ""},
		{"user-1", "organization_owner"},
		{"user-1", "superuser"},
	} {
		if _, err := svc.EnsureStaff(ctx, tc[0], tc[1]); !errors.Is(err, platform.ErrInvalidInput) {
			t.Fatalf("input %v: got %v, want ErrInvalidInput", tc, err)
		}
	}
}

func TestEnsureStaffPropagatesLookupFailures(t *testing.T) {
	t.Parallel()

	svc, repo, _ := newPlatformService(0, 0)

	repo.getErr = errors.New("store unavailable")

	_, err := svc.EnsureStaff(context.Background(), "user-1", platform.RoleOwner())
	if !errors.Is(err, repo.getErr) {
		t.Fatalf("infrastructure error should surface, got %v", err)
	}
}

func TestGetByUserIDNotFound(t *testing.T) {
	t.Parallel()

	svc, _, _ := newPlatformService(0, 0)

	if _, err := svc.GetByUserID(context.Background(), "ghost"); !errors.Is(err, platform.ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
}

func TestOverviewAggregatesMetrics(t *testing.T) {
	t.Parallel()

	svc, _, orgs := newPlatformService(7, 3)

	orgs.orgs["org-1"] = organizations.Organization{ID: "org-1", Status: "trial"}
	orgs.orgs["org-2"] = organizations.Organization{ID: "org-2", Status: "trial"}
	orgs.orgs["org-3"] = organizations.Organization{ID: "org-3", Status: "active"}
	orgs.orgs["org-4"] = organizations.Organization{ID: "org-4", Status: "suspended"}

	overview, err := svc.Overview(context.Background())
	if err != nil {
		t.Fatalf("overview: %v", err)
	}

	want := platform.Overview{
		TotalOrgs: 4, TrialOrgs: 2, ActiveOrgs: 1, SuspendedOrgs: 1, TotalLeads: 7, OpenTicketCount: 3,
	}
	if overview != want {
		t.Fatalf("overview = %+v, want %+v", overview, want)
	}
}

func TestListOrganizationsFiltersBySearchStatusAndPlan(t *testing.T) {
	t.Parallel()

	svc, _, orgs := newPlatformService(0, 0)
	orgs.orgs["one"] = organizations.Organization{
		ID: "one", Name: "Acme Ghana", Slug: "acme-gh", Status: "active", PlanCode: "growth",
	}
	orgs.orgs["two"] = organizations.Organization{
		ID: "two", Name: "Orbit Labs", Slug: "orbit", Status: "trial", PlanCode: "starter",
	}

	items, err := svc.ListOrganizations(context.Background(), platform.OrganizationListInput{
		Search: "ACME", Status: "ACTIVE", PlanCode: "growth",
	})
	if err != nil {
		t.Fatalf("list organizations: %v", err)
	}
	if len(items) != 1 || items[0].ID != "one" {
		t.Fatalf("filtered organizations = %+v, want Acme", items)
	}

	items, err = svc.ListOrganizations(context.Background(), platform.OrganizationListInput{Search: "bit"})
	if err != nil {
		t.Fatalf("list organizations by slug: %v", err)
	}
	if len(items) != 1 || items[0].ID != "two" {
		t.Fatalf("slug-filtered organizations = %+v, want Orbit", items)
	}
}

func TestListOrganizationsPageReturnsStableMetadata(t *testing.T) {
	t.Parallel()

	svc, _, orgs := newPlatformService(0, 0)
	for index := 0; index < 5; index++ {
		id := fmt.Sprintf("org-%d", index)
		orgs.orgs[id] = organizations.Organization{
			ID: id, Name: id, Slug: id, Status: "active", PlanCode: "starter",
		}
	}

	page, err := svc.ListOrganizationsPage(context.Background(), platform.OrganizationListInput{
		Offset: 2,
		Limit:  2,
	})
	if err != nil {
		t.Fatalf("list organization page: %v", err)
	}
	if page.Total != 5 || page.Offset != 2 || page.Limit != 2 || len(page.Items) != 2 {
		t.Fatalf("page = %+v, want total=5 offset=2 limit=2 items=2", page)
	}
}

func TestClosedOrganizationCannotBeReactivated(t *testing.T) {
	t.Parallel()

	svc, _, orgs := newPlatformService(0, 0)
	orgs.orgs["org-1"] = organizations.Organization{ID: "org-1", Status: organizations.StatusActive()}

	closed, err := svc.SetOrganizationStatus(context.Background(), "org-1", organizations.StatusClosed())
	if err != nil {
		t.Fatalf("close organization: %v", err)
	}
	if closed.Status != organizations.StatusClosed() {
		t.Fatalf("status = %q, want closed", closed.Status)
	}
}

func TestSetOrganizationStatusDelegates(t *testing.T) {
	t.Parallel()

	svc, _, orgs := newPlatformService(0, 0)
	ctx := context.Background()

	orgs.orgs["org-1"] = organizations.Organization{ID: "org-1", Status: "trial"}

	updated, err := svc.SetOrganizationStatus(ctx, "org-1", "suspended")
	if err != nil {
		t.Fatalf("set status: %v", err)
	}

	if updated.Status != "suspended" || orgs.orgs["org-1"].Status != "suspended" {
		t.Fatalf("status change not applied: %+v", updated)
	}

	if _, err := svc.SetOrganizationStatus(ctx, "ghost", "suspended"); !errors.Is(err, organizations.ErrNotFound) {
		t.Fatalf("unknown organization got %v, want ErrNotFound", err)
	}
}
