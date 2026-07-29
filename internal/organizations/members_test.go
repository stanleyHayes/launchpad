package organizations_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"launchpad/internal/organizations"
)

// fakeMemberUserReader returns canned display info per user id.
type fakeMemberUserReader struct {
	users map[string]organizations.MemberUser
}

func (f fakeMemberUserReader) GetMemberUser(_ context.Context, userID string) (organizations.MemberUser, error) {
	if user, ok := f.users[userID]; ok {
		return user, nil
	}

	return organizations.MemberUser{}, errors.New("user not found")
}

// fakeRoleChecker marks a fixed set of custom role names as existing.
type fakeRoleChecker struct {
	existing map[string]bool
}

func (f fakeRoleChecker) RoleExists(_ context.Context, _, roleCode string) (bool, error) {
	return f.existing[roleCode], nil
}

func seedMembership(
	t *testing.T,
	repo *fakeOrgRepo,
	organizationID, userID, roleCode string,
) {
	t.Helper()

	membership := organizations.Membership{
		ID:             "m-" + userID,
		OrganizationID: organizationID,
		UserID:         userID,
		RoleCode:       roleCode,
		Status:         "active",
		CreatedAt:      time.Now().UTC(),
	}
	if err := repo.CreateMembership(context.Background(), membership); err != nil {
		t.Fatalf("seed membership: %v", err)
	}
}

func newMemberService(
	repo *fakeOrgRepo,
	users organizations.MemberUserReader,
	checker organizations.RoleChecker,
) *organizations.Service {
	return organizations.NewService(repo, nil, users, checker)
}

func TestListMembersIncludesDisplayInfo(t *testing.T) {
	t.Parallel()

	repo := newFakeOrgRepo()
	seedMembership(t, repo, "org-1", "user-1", "organization_owner")
	seedMembership(t, repo, "org-1", "user-2", "employee")
	seedMembership(t, repo, "org-2", "user-3", "employee")

	users := fakeMemberUserReader{users: map[string]organizations.MemberUser{
		"user-1": {ID: "user-1", Email: "owner@example.com", DisplayName: "Owner", Status: "active"},
	}}

	svc := newMemberService(repo, users, nil)

	members, err := svc.ListMembers(context.Background(), "org-1")
	if err != nil {
		t.Fatalf("list members: %v", err)
	}

	if len(members) != 2 {
		t.Fatalf("len=%d, want 2 (tenant-scoped)", len(members))
	}

	byUser := map[string]organizations.Member{}
	for _, member := range members {
		byUser[member.Membership.UserID] = member
	}

	if byUser["user-1"].Email != "owner@example.com" || byUser["user-1"].DisplayName != "Owner" {
		t.Errorf("user-1 display info missing: %+v", byUser["user-1"])
	}

	// user-2 has no account record: listed with empty display info, not an error.
	if byUser["user-2"].Email != "" {
		t.Errorf("user-2 should have empty display info: %+v", byUser["user-2"])
	}
}

func TestChangeMemberRole(t *testing.T) {
	t.Parallel()

	repo := newFakeOrgRepo()
	seedMembership(t, repo, "org-1", "owner-1", "organization_owner")
	seedMembership(t, repo, "org-1", "owner-2", "organization_owner")
	seedMembership(t, repo, "org-1", "user-1", "employee")

	svc := newMemberService(repo, nil, fakeRoleChecker{existing: map[string]bool{"team_lead": true}})

	membership, err := svc.ChangeMemberRole(context.Background(), "org-1", "owner-1", "user-1", "hr_admin")
	if err != nil {
		t.Fatalf("change role: %v", err)
	}

	if membership.RoleCode != "hr_admin" {
		t.Fatalf("roleCode=%q, want hr_admin", membership.RoleCode)
	}
}

func TestChangeMemberRoleToCustomRole(t *testing.T) {
	t.Parallel()

	repo := newFakeOrgRepo()
	seedMembership(t, repo, "org-1", "owner-1", "organization_owner")
	seedMembership(t, repo, "org-1", "user-1", "employee")

	svc := newMemberService(repo, nil, fakeRoleChecker{existing: map[string]bool{"team_lead": true}})

	if _, err := svc.ChangeMemberRole(context.Background(), "org-1", "owner-1", "user-1", "team_lead"); err != nil {
		t.Fatalf("change to custom role: %v", err)
	}
}

func TestChangeMemberRoleRejectsUnknownRole(t *testing.T) {
	t.Parallel()

	repo := newFakeOrgRepo()
	seedMembership(t, repo, "org-1", "owner-1", "organization_owner")
	seedMembership(t, repo, "org-1", "user-1", "employee")

	svc := newMemberService(repo, nil, fakeRoleChecker{existing: map[string]bool{}})

	_, err := svc.ChangeMemberRole(context.Background(), "org-1", "owner-1", "user-1", "wizard")
	if !errors.Is(err, organizations.ErrUnknownRole) {
		t.Fatalf("err=%v, want ErrUnknownRole", err)
	}
}

func TestChangeMemberRoleRejectsOwnRole(t *testing.T) {
	t.Parallel()

	repo := newFakeOrgRepo()
	seedMembership(t, repo, "org-1", "owner-1", "organization_owner")

	svc := newMemberService(repo, nil, nil)

	_, err := svc.ChangeMemberRole(context.Background(), "org-1", "owner-1", "owner-1", "employee")
	if !errors.Is(err, organizations.ErrCannotChangeOwnRole) {
		t.Fatalf("err=%v, want ErrCannotChangeOwnRole", err)
	}
}

func TestChangeMemberRoleProtectsLastOwner(t *testing.T) {
	t.Parallel()

	repo := newFakeOrgRepo()
	seedMembership(t, repo, "org-1", "owner-1", "organization_owner")
	seedMembership(t, repo, "org-1", "user-1", "employee")

	svc := newMemberService(repo, nil, nil)

	// The actor is another admin (hr_admin) demoting the only owner.
	_, err := svc.ChangeMemberRole(context.Background(), "org-1", "user-1", "owner-1", "employee")
	if !errors.Is(err, organizations.ErrLastOwner) {
		t.Fatalf("err=%v, want ErrLastOwner", err)
	}

	// Promoting a second owner first unlocks the demotion.
	seedMembership(t, repo, "org-1", "owner-2", "organization_owner")

	if _, err := svc.ChangeMemberRole(context.Background(), "org-1", "user-1", "owner-1", "employee"); err != nil {
		t.Fatalf("demote with second owner present: %v", err)
	}
}

func TestChangeMemberRoleUnknownMember(t *testing.T) {
	t.Parallel()

	repo := newFakeOrgRepo()
	seedMembership(t, repo, "org-1", "owner-1", "organization_owner")

	svc := newMemberService(repo, nil, nil)

	_, err := svc.ChangeMemberRole(context.Background(), "org-1", "owner-1", "ghost", "employee")
	if !errors.Is(err, organizations.ErrNotFound) {
		t.Fatalf("err=%v, want ErrNotFound", err)
	}
}
