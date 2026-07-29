//nolint:testpackage // internal test: exercises the unexported hrisDirectoryApplier adapter.
package app

import (
	"context"
	"fmt"
	"testing"
	"time"

	"launchpad/internal/departments"
	"launchpad/internal/employees"
	"launchpad/internal/hris"
)

// memDepartments is an in-memory departments.Repository for exercising the
// HRIS directory applier's department name -> id mapping.
type memDepartments struct {
	depts []departments.Department
}

func (m *memDepartments) EnsureIndexes(context.Context) error { return nil }

func (m *memDepartments) CreateDepartment(_ context.Context, dept departments.Department) error {
	m.depts = append(m.depts, dept)

	return nil
}

func (m *memDepartments) ListDepartments(_ context.Context, org string) ([]departments.Department, error) {
	out := make([]departments.Department, 0)

	for _, dept := range m.depts {
		if dept.OrganizationID == org {
			out = append(out, dept)
		}
	}

	return out, nil
}

func (m *memDepartments) GetDepartment(_ context.Context, org, id string) (departments.Department, error) {
	for _, dept := range m.depts {
		if dept.OrganizationID == org && dept.ID == id {
			return dept, nil
		}
	}

	return departments.Department{}, departments.ErrNotFound
}

func (m *memDepartments) CreateJobRole(context.Context, departments.JobRole) error { return nil }

func (m *memDepartments) ListJobRoles(context.Context, string) ([]departments.JobRole, error) {
	return nil, nil
}

func (m *memDepartments) GetJobRole(context.Context, string, string) (departments.JobRole, error) {
	return departments.JobRole{}, departments.ErrRoleNotFound
}

// memEmployees is an in-memory employees.Repository. It enforces the (org,
// workEmail) uniqueness the real store guarantees; failCreate, when set, forces
// Create to return that error (used to simulate a concurrent-insert race).
type memEmployees struct {
	items      []employees.Employee
	failCreate error
}

func (m *memEmployees) EnsureIndexes(context.Context) error { return nil }

func (m *memEmployees) Create(_ context.Context, emp employees.Employee) error {
	if m.failCreate != nil {
		return m.failCreate
	}

	for _, existing := range m.items {
		if existing.OrganizationID == emp.OrganizationID && existing.WorkEmail == emp.WorkEmail {
			return employees.ErrEmailTaken
		}
	}

	m.items = append(m.items, emp)

	return nil
}

func (m *memEmployees) GetByID(_ context.Context, org, id string) (employees.Employee, error) {
	for _, emp := range m.items {
		if emp.OrganizationID == org && emp.ID == id {
			return emp, nil
		}
	}

	return employees.Employee{}, employees.ErrNotFound
}

func (m *memEmployees) GetByUserID(_ context.Context, org, userID string) (employees.Employee, error) {
	for _, emp := range m.items {
		if emp.OrganizationID == org && emp.UserID == userID {
			return emp, nil
		}
	}

	return employees.Employee{}, employees.ErrNotFound
}

func (m *memEmployees) GetByWorkEmail(_ context.Context, org, email string) (employees.Employee, error) {
	for _, emp := range m.items {
		if emp.OrganizationID == org && emp.WorkEmail == email {
			return emp, nil
		}
	}

	return employees.Employee{}, employees.ErrNotFound
}

func (m *memEmployees) List(_ context.Context, org string, offset, limit int64) ([]employees.Employee, error) {
	out := make([]employees.Employee, 0)

	for _, emp := range m.items {
		if emp.OrganizationID == org {
			out = append(out, emp)
		}
	}

	if offset > int64(len(out)) {
		return []employees.Employee{}, nil
	}

	out = out[offset:]
	if limit > 0 && limit < int64(len(out)) {
		out = out[:limit]
	}

	return out, nil
}

func (m *memEmployees) Count(_ context.Context, org string) (int64, error) {
	count := int64(0)

	for _, emp := range m.items {
		if emp.OrganizationID == org {
			count++
		}
	}

	return count, nil
}

func (m *memEmployees) Update(context.Context, employees.Employee) error { return nil }

func (m *memEmployees) ProvisionAccess(context.Context, string, string, string) error { return nil }

func TestHRISDirectoryApplierMapsSkipsAndFails(t *testing.T) {
	t.Parallel()

	const org = "org-apply"

	ctx := context.Background()
	deptRepo := &memDepartments{depts: []departments.Department{
		{ID: "dept-eng", OrganizationID: org, Name: "Engineering"},
	}}
	empRepo := &memEmployees{}
	deptSvc := departments.NewService(deptRepo)
	empSvc := employees.NewService(empRepo, deptSvc)

	// Seed an existing employee so the applier's idempotent-skip path is hit.
	if _, err := empSvc.Create(ctx, org, employees.CreateInput{
		FirstName: "Existing", LastName: "Person", WorkEmail: "existing@acme.test",
		DepartmentID: "dept-eng", StartDate: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("seed existing employee: %v", err)
	}

	applier := hrisDirectoryApplier{employees: empSvc, departments: deptSvc}

	entries := []hris.DirectoryEntry{
		// Mixed-case email + department name -> created, mapped to dept-eng.
		{
			ExternalID: "E1", FirstName: "Priya", LastName: "Rao",
			Email: "Priya@Acme.Test", Department: "engineering", Active: true,
		},
		// Unknown department name -> created with no department.
		{ExternalID: "E2", FirstName: "Sam", LastName: "Lee", Email: "sam@acme.test", Department: "Sales", Active: true},
		// Already present -> skipped.
		{ExternalID: "E3", FirstName: "Existing", LastName: "Person", Email: "existing@acme.test", Active: true},
		// Missing name -> failed (employees.Create rejects it).
		{ExternalID: "E4", FirstName: "", LastName: "", Email: "noname@acme.test", Active: true},
		// Invalid email -> failed before any lookup.
		{ExternalID: "E5", FirstName: "Bad", LastName: "Email", Email: "not-an-email", Active: true},
	}

	result, err := applier.Apply(ctx, org, entries)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}

	if result.Total != 5 || result.Created != 2 || result.Skipped != 1 || result.Failed != 2 {
		t.Fatalf("unexpected counts: %+v", result)
	}

	if len(result.Errors) != 2 {
		t.Fatalf("expected 2 error messages, got %v", result.Errors)
	}

	priya, err := empSvc.GetByWorkEmail(ctx, org, "priya@acme.test")
	if err != nil {
		t.Fatalf("priya should have been created: %v", err)
	}

	if priya.DepartmentID != "dept-eng" {
		t.Fatalf("expected priya mapped to dept-eng, got %q", priya.DepartmentID)
	}

	sam, err := empSvc.GetByWorkEmail(ctx, org, "sam@acme.test")
	if err != nil {
		t.Fatalf("sam should have been created: %v", err)
	}

	if sam.DepartmentID != "" {
		t.Fatalf("expected sam with no department, got %q", sam.DepartmentID)
	}

	// Re-applying the same snapshot must be idempotent: nothing new created.
	second, err := applier.Apply(ctx, org, entries)
	if err != nil {
		t.Fatalf("second apply: %v", err)
	}

	if second.Created != 0 || second.Skipped != 3 || second.Failed != 2 {
		t.Fatalf("re-apply not idempotent: %+v", second)
	}
}

func TestHRISDirectoryApplierSkipsConcurrentInsert(t *testing.T) {
	t.Parallel()

	const org = "org-race"

	ctx := context.Background()
	// Lookup misses but Create loses the race with a concurrent insert: the goal
	// state (employee exists) holds, so the applier must count it as skipped.
	empRepo := &memEmployees{failCreate: employees.ErrEmailTaken}
	deptSvc := departments.NewService(&memDepartments{})
	empSvc := employees.NewService(empRepo, deptSvc)
	applier := hrisDirectoryApplier{employees: empSvc, departments: deptSvc}

	result, err := applier.Apply(ctx, org, []hris.DirectoryEntry{
		{ExternalID: "R1", FirstName: "Race", LastName: "Winner", Email: "race@acme.test", Active: true},
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}

	if result.Created != 0 || result.Skipped != 1 || result.Failed != 0 {
		t.Fatalf("expected concurrent insert counted as skipped, got %+v", result)
	}
}

func TestHRISDirectoryApplierBoundsErrorMessages(t *testing.T) {
	t.Parallel()

	const org = "org-bound"

	ctx := context.Background()
	deptSvc := departments.NewService(&memDepartments{})
	empSvc := employees.NewService(&memEmployees{}, deptSvc)
	applier := hrisDirectoryApplier{employees: empSvc, departments: deptSvc}

	entries := make([]hris.DirectoryEntry, 0, maxHRISApplyErrors+10)
	for i := range maxHRISApplyErrors + 10 {
		entries = append(entries, hris.DirectoryEntry{ExternalID: fmt.Sprintf("B%d", i), Email: "bad-email", Active: true})
	}

	result, err := applier.Apply(ctx, org, entries)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}

	if result.Failed != maxHRISApplyErrors+10 {
		t.Fatalf("expected %d failed, got %d", maxHRISApplyErrors+10, result.Failed)
	}

	if len(result.Errors) != maxHRISApplyErrors {
		t.Fatalf("expected errors capped at %d, got %d", maxHRISApplyErrors, len(result.Errors))
	}
}

func (m *memDepartments) DeleteForOrganization(context.Context, string) (int64, error) {
	return 0, nil
}

func (m *memEmployees) DeleteForOrganization(context.Context, string) (int64, error) {
	return 0, nil
}
