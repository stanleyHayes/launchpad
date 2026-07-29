package roles_test

import (
	"context"
	"errors"
	"testing"

	"launchpad/internal/roles"
)

// --- in-memory fakes ---------------------------------------------------------

// fakeRoleRepo mirrors the mongo store semantics that matter to the service:
// unique (organization, name) pairs and organization-scoped lookups.
type fakeRoleRepo struct {
	items map[string]roles.Role
}

func newFakeRoleRepo() *fakeRoleRepo {
	return &fakeRoleRepo{items: map[string]roles.Role{}}
}

func roleKey(organizationID, name string) string { return organizationID + "|" + name }

func (f *fakeRoleRepo) EnsureIndexes(context.Context) error { return nil }

func (f *fakeRoleRepo) Create(_ context.Context, role roles.Role) error {
	if _, taken := f.items[roleKey(role.OrganizationID, role.Name)]; taken {
		return roles.ErrNameTaken
	}

	f.items[roleKey(role.OrganizationID, role.Name)] = role

	return nil
}

func (f *fakeRoleRepo) GetByID(_ context.Context, organizationID, id string) (roles.Role, error) {
	for _, role := range f.items {
		if role.OrganizationID == organizationID && role.ID == id {
			return role, nil
		}
	}

	return roles.Role{}, roles.ErrNotFound
}

func (f *fakeRoleRepo) GetByName(_ context.Context, organizationID, name string) (roles.Role, error) {
	if role, ok := f.items[roleKey(organizationID, name)]; ok {
		return role, nil
	}

	return roles.Role{}, roles.ErrNotFound
}

func (f *fakeRoleRepo) List(_ context.Context, organizationID string) ([]roles.Role, error) {
	out := make([]roles.Role, 0)

	for _, role := range f.items {
		if role.OrganizationID == organizationID {
			out = append(out, role)
		}
	}

	return out, nil
}

func (f *fakeRoleRepo) Update(_ context.Context, role roles.Role) error {
	key := roleKey(role.OrganizationID, role.Name)
	if _, ok := f.items[key]; !ok {
		return roles.ErrNotFound
	}

	f.items[key] = role

	return nil
}

func (f *fakeRoleRepo) Delete(_ context.Context, organizationID, id string) error {
	for key, role := range f.items {
		if role.OrganizationID == organizationID && role.ID == id {
			delete(f.items, key)

			return nil
		}
	}

	return roles.ErrNotFound
}

type fakePlanReader struct {
	planCode string
	err      error
}

func (f fakePlanReader) PlanCode(context.Context, string) (string, error) {
	return f.planCode, f.err
}

func newService(planCode string) *roles.Service {
	return roles.NewService(newFakeRoleRepo(), fakePlanReader{planCode: planCode, err: nil})
}

func createRole(t *testing.T, svc *roles.Service) roles.Role {
	t.Helper()

	role, err := svc.Create(context.Background(), "org-1", roles.CreateInput{
		Name:        "team_lead",
		Permissions: []string{roles.PermissionEmployeesRead, roles.PermissionAssignmentsRead},
	})
	if err != nil {
		t.Fatalf("create role: %v", err)
	}

	return role
}

// --- built-in permission maps -------------------------------------------------

func TestBuiltinPermissionsOwnerHasEverything(t *testing.T) {
	t.Parallel()

	permissions, ok := roles.BuiltinPermissions(roles.RoleOrganizationOwner)
	if !ok {
		t.Fatal("organization_owner must be a built-in role")
	}

	if len(permissions) != len(roles.AllPermissions()) {
		t.Fatalf("owner permissions=%d, want full registry %d", len(permissions), len(roles.AllPermissions()))
	}
}

func TestBuiltinPermissionsHRAdminExcludesBillingManage(t *testing.T) {
	t.Parallel()

	permissions, ok := roles.BuiltinPermissions(roles.RoleHRAdmin)
	if !ok {
		t.Fatal("hr_admin must be a built-in role")
	}

	set := map[string]bool{}
	for _, permission := range permissions {
		set[permission] = true
	}

	if set[roles.PermissionBillingManage] {
		t.Error("hr_admin must not hold billing.manage")
	}

	if !set[roles.PermissionBillingRead] {
		t.Error("hr_admin must keep billing.read")
	}

	if !set[roles.PermissionMembersUpdate] {
		t.Error("hr_admin must hold members.update for member role management")
	}
}

func TestBuiltinPermissionsManagerScope(t *testing.T) {
	t.Parallel()

	permissions, ok := roles.BuiltinPermissions(roles.RoleManager)
	if !ok {
		t.Fatal("manager must be a built-in role")
	}

	set := map[string]bool{}
	for _, permission := range permissions {
		set[permission] = true
	}

	want := []string{
		roles.PermissionEmployeesRead,
		roles.PermissionJourneysAssign,
		roles.PermissionAssignmentsRead,
		roles.PermissionAssignmentsManage,
		roles.PermissionApprovalsDecide,
		roles.PermissionAnalyticsRead,
		roles.PermissionAuditRead,
		roles.PermissionNotificationsRead,
	}
	for _, permission := range want {
		if !set[permission] {
			t.Errorf("manager missing %s", permission)
		}
	}

	for _, denied := range []string{roles.PermissionBillingManage, roles.PermissionJourneysCreate} {
		if set[denied] {
			t.Errorf("manager must not hold %s", denied)
		}
	}
}

func TestBuiltinPermissionsEmployeeSelfService(t *testing.T) {
	t.Parallel()

	permissions, ok := roles.BuiltinPermissions(roles.RoleEmployee)
	if !ok {
		t.Fatal("employee must be a built-in role")
	}

	set := map[string]bool{}
	for _, permission := range permissions {
		set[permission] = true
	}

	if !set[roles.PermissionStepsComplete] || !set[roles.PermissionNotificationsRead] {
		t.Error("employee must hold steps.complete and notifications.read")
	}

	if set[roles.PermissionEmployeesCreate] || set[roles.PermissionMembersRead] {
		t.Error("employee must not hold management permissions")
	}
}

func TestBuiltinPermissionsUnknownRole(t *testing.T) {
	t.Parallel()

	if _, ok := roles.BuiltinPermissions("wizard"); ok {
		t.Error("unknown role code must not resolve to a built-in")
	}
}

// --- custom role CRUD + plan gate ----------------------------------------------

func TestCreateRoleRequiresEnterprisePlan(t *testing.T) {
	t.Parallel()

	for _, planCode := range []string{"starter", "growth"} {
		svc := newService(planCode)

		_, err := svc.Create(context.Background(), "org-1", roles.CreateInput{
			Name:        "team_lead",
			Permissions: []string{roles.PermissionEmployeesRead},
		})
		if !errors.Is(err, roles.ErrPlanNotEligible) {
			t.Fatalf("plan %q: err=%v, want ErrPlanNotEligible", planCode, err)
		}
	}
}

func TestCreateRoleOnEnterprisePlan(t *testing.T) {
	t.Parallel()

	svc := newService("enterprise")

	role, err := svc.Create(context.Background(), "org-1", roles.CreateInput{
		Name:        "team_lead",
		Permissions: []string{roles.PermissionEmployeesRead, roles.PermissionEmployeesRead},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if role.Name != "team_lead" || role.Builtin {
		t.Fatalf("unexpected role: %+v", role)
	}

	// Duplicate permissions in the input are deduped.
	if len(role.Permissions) != 1 {
		t.Fatalf("permissions=%v, want deduped single entry", role.Permissions)
	}
}

func TestCreateRoleRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	svc := newService("enterprise")

	cases := map[string]roles.CreateInput{
		"bad name":              {Name: "Team Lead!", Permissions: []string{roles.PermissionEmployeesRead}},
		"empty permissions":     {Name: "team_lead", Permissions: nil},
		"unknown permission":    {Name: "team_lead", Permissions: []string{"employees.delete"}},
		"builtin name reserved": {Name: "manager", Permissions: []string{roles.PermissionEmployeesRead}},
	}

	for name, input := range cases {
		_, err := svc.Create(context.Background(), "org-1", input)

		switch name {
		case "builtin name reserved":
			if !errors.Is(err, roles.ErrNameTaken) {
				t.Errorf("%s: err=%v, want ErrNameTaken", name, err)
			}
		default:
			if !errors.Is(err, roles.ErrInvalidInput) {
				t.Errorf("%s: err=%v, want ErrInvalidInput", name, err)
			}
		}
	}
}

func TestCreateRoleDuplicateName(t *testing.T) {
	t.Parallel()

	svc := newService("enterprise")
	createRole(t, svc)

	_, err := svc.Create(context.Background(), "org-1", roles.CreateInput{
		Name:        "team_lead",
		Permissions: []string{roles.PermissionEmployeesRead},
	})
	if !errors.Is(err, roles.ErrNameTaken) {
		t.Fatalf("err=%v, want ErrNameTaken", err)
	}
}

func TestListIncludesBuiltinsAndCustom(t *testing.T) {
	t.Parallel()

	svc := newService("enterprise")
	createRole(t, svc)

	items, err := svc.List(context.Background(), "org-1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if len(items) != 5 { // 4 built-ins + 1 custom
		t.Fatalf("len=%d, want 5", len(items))
	}

	var custom *roles.Role

	builtins := 0

	for i := range items {
		if items[i].Builtin {
			builtins++
		} else {
			custom = &items[i]
		}
	}

	if builtins != 4 {
		t.Errorf("builtins=%d, want 4", builtins)
	}

	if custom == nil || custom.Name != "team_lead" {
		t.Errorf("custom role missing from list: %+v", items)
	}
}

func TestUpdateRole(t *testing.T) {
	t.Parallel()

	svc := newService("enterprise")
	role := createRole(t, svc)

	updated, err := svc.Update(context.Background(), "org-1", role.ID, roles.UpdateInput{
		Permissions: []string{roles.PermissionApprovalsDecide},
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	if len(updated.Permissions) != 1 || updated.Permissions[0] != roles.PermissionApprovalsDecide {
		t.Fatalf("permissions=%v, want [approvals.decide]", updated.Permissions)
	}
}

func TestUpdateBuiltinRoleRejected(t *testing.T) {
	t.Parallel()

	svc := newService("enterprise")

	_, err := svc.Update(context.Background(), "org-1", roles.RoleManager, roles.UpdateInput{
		Permissions: []string{roles.PermissionEmployeesRead},
	})
	if !errors.Is(err, roles.ErrBuiltinRole) {
		t.Fatalf("err=%v, want ErrBuiltinRole", err)
	}
}

func TestDeleteRole(t *testing.T) {
	t.Parallel()

	svc := newService("enterprise")
	role := createRole(t, svc)

	if err := svc.Delete(context.Background(), "org-1", role.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if err := svc.Delete(context.Background(), "org-1", role.ID); !errors.Is(err, roles.ErrNotFound) {
		t.Fatalf("second delete err=%v, want ErrNotFound", err)
	}

	if err := svc.Delete(context.Background(), "org-1", roles.RoleEmployee); !errors.Is(err, roles.ErrBuiltinRole) {
		t.Fatalf("builtin delete err=%v, want ErrBuiltinRole", err)
	}
}

// --- permission resolution ------------------------------------------------------

func TestResolvePermissionsBuiltin(t *testing.T) {
	t.Parallel()

	svc := newService("starter")

	set, err := svc.ResolvePermissions(context.Background(), "org-1", roles.RoleManager)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	if _, ok := set[roles.PermissionAssignmentsManage]; !ok {
		t.Error("manager set must include assignments.manage")
	}
}

func TestResolvePermissionsCustomRole(t *testing.T) {
	t.Parallel()

	svc := newService("enterprise")
	createRole(t, svc)

	set, err := svc.ResolvePermissions(context.Background(), "org-1", "team_lead")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	if _, ok := set[roles.PermissionEmployeesRead]; !ok {
		t.Error("custom role set must include employees.read")
	}

	if _, ok := set[roles.PermissionBillingManage]; ok {
		t.Error("custom role set must not include unassigned permissions")
	}
}

func TestResolvePermissionsCustomRoleIsTenantScoped(t *testing.T) {
	t.Parallel()

	svc := newService("enterprise")
	createRole(t, svc)

	set, err := svc.ResolvePermissions(context.Background(), "org-2", "team_lead")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	if len(set) != 0 {
		t.Errorf("cross-tenant resolution must be empty, got %v", set)
	}
}

func TestResolvePermissionsDeletedRoleFallsBackToEmpty(t *testing.T) {
	t.Parallel()

	svc := newService("enterprise")
	role := createRole(t, svc)

	if err := svc.Delete(context.Background(), "org-1", role.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	set, err := svc.ResolvePermissions(context.Background(), "org-1", "team_lead")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	if len(set) != 0 {
		t.Errorf("memberships on a deleted role must resolve to zero permissions, got %v", set)
	}
}

func TestRoleExists(t *testing.T) {
	t.Parallel()

	svc := newService("enterprise")
	createRole(t, svc)

	cases := []struct {
		roleCode string
		want     bool
	}{
		{roleCode: roles.RoleHRAdmin, want: true},
		{roleCode: "team_lead", want: true},
		{roleCode: "missing", want: false},
	}

	for _, tc := range cases {
		got, err := svc.RoleExists(context.Background(), "org-1", tc.roleCode)
		if err != nil {
			t.Fatalf("RoleExists(%q): %v", tc.roleCode, err)
		}

		if got != tc.want {
			t.Errorf("RoleExists(%q)=%v, want %v", tc.roleCode, got, tc.want)
		}
	}
}

func (f *fakeRoleRepo) DeleteForOrganization(context.Context, string) (int64, error) {
	return 0, nil
}
