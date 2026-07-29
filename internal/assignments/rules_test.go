package assignments_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"launchpad/internal/assignments"
	"launchpad/internal/employees"
	"launchpad/internal/journeys"
)

const (
	testRuleID   = "rule-1"
	testTemplate = "tpl-1"
	testDeptID   = "dept-1"
	testRoleID   = "role-1"
)

// memoryRuleRepo is an in-memory assignments.RuleRepository.
type memoryRuleRepo struct {
	rules map[string]assignments.Rule
}

func newMemoryRuleRepo() *memoryRuleRepo {
	return &memoryRuleRepo{rules: map[string]assignments.Rule{}}
}

func (m *memoryRuleRepo) CreateRule(_ context.Context, rule assignments.Rule) error {
	m.rules[rule.ID] = rule

	return nil
}

func (m *memoryRuleRepo) GetRule(_ context.Context, organizationID, ruleID string) (assignments.Rule, error) {
	rule, ok := m.rules[ruleID]
	if !ok || rule.OrganizationID != organizationID {
		return assignments.Rule{}, assignments.ErrNotFound
	}

	return rule, nil
}

func (m *memoryRuleRepo) ListRules(_ context.Context, organizationID string) ([]assignments.Rule, error) {
	items := make([]assignments.Rule, 0)

	for _, rule := range m.rules {
		if rule.OrganizationID == organizationID {
			items = append(items, rule)
		}
	}

	return items, nil
}

func (m *memoryRuleRepo) UpdateRule(ctx context.Context, rule assignments.Rule) error {
	if _, err := m.GetRule(ctx, rule.OrganizationID, rule.ID); err != nil {
		return err
	}

	m.rules[rule.ID] = rule

	return nil
}

func (m *memoryRuleRepo) DeleteRule(ctx context.Context, organizationID, ruleID string) error {
	if _, err := m.GetRule(ctx, organizationID, ruleID); err != nil {
		return err
	}

	delete(m.rules, ruleID)

	return nil
}

// activeTrackingRepo makes FindActiveAssignment actually search so the
// already-assigned skip path is exercised.
type activeTrackingRepo struct {
	*memoryRepo
}

func (m *activeTrackingRepo) FindActiveAssignment(
	_ context.Context,
	organizationID, employeeID, templateID string,
) (assignments.JourneyAssignment, error) {
	for _, assignment := range m.assignments {
		if assignment.OrganizationID == organizationID &&
			assignment.EmployeeID == employeeID &&
			assignment.JourneyTemplateID == templateID &&
			assignment.Status != "completed" {
			return assignment, nil
		}
	}

	return assignments.JourneyAssignment{}, assignments.ErrNotFound
}

// failingJourneys reports templates as not published.
type failingJourneys struct {
	stubJourneys
}

func (failingJourneys) RequirePublished(context.Context, string, string) (journeys.Template, error) {
	return journeys.Template{}, journeys.ErrNotPublished
}

// echoJourneys reports every requested template as published, echoing the
// requested ID so distinct rules produce distinct assignments.
type echoJourneys struct {
	steps []journeys.Step
}

func (e echoJourneys) RequirePublished(
	_ context.Context,
	organizationID, templateID string,
) (journeys.Template, error) {
	return journeys.Template{
		ID:             templateID,
		OrganizationID: organizationID,
		Status:         "published",
		CurrentVersion: 1,
	}, nil
}

func (e echoJourneys) ListStepsForVersion(context.Context, string, string, int) ([]journeys.Step, error) {
	return e.steps, nil
}

func newRuleFixture(employeeList ...employees.Employee) (
	*activeTrackingRepo,
	*memoryRuleRepo,
	*assignments.RuleService,
) {
	repo := &activeTrackingRepo{memoryRepo: newMemoryRepo()}

	byID := map[string]employees.Employee{}
	for _, employee := range employeeList {
		byID[employee.ID] = employee
	}

	reader := echoJourneys{steps: []journeys.Step{
		{ID: "js-1", StepType: "task", Title: "Read the handbook", Position: 1},
	}}
	svc := assignments.NewService(repo, reader, stubEmployees{byID: byID}, &stubNotify{})
	rules := newMemoryRuleRepo()

	return repo, rules, assignments.NewRuleService(rules, reader, stubEmployees{byID: byID}, svc)
}

func seedRule(t *testing.T, rules *memoryRuleRepo, rule assignments.Rule) {
	t.Helper()

	if rule.ID == "" {
		rule.ID = testRuleID
	}

	if rule.OrganizationID == "" {
		rule.OrganizationID = testOrgID
	}

	if rule.JourneyTemplateID == "" {
		rule.JourneyTemplateID = testTemplate
	}

	rule.Active = true
	rule.CreatedAt = time.Now().UTC()
	rules.rules[rule.ID] = rule
}

func TestApplyAssignmentRulesMatchesDepartmentAndRole(t *testing.T) {
	t.Parallel()

	employee := employees.Employee{
		ID:             testEmployeeID,
		OrganizationID: testOrgID,
		DepartmentID:   testDeptID,
		JobRoleID:      testRoleID,
		Status:         "invited",
		StartDate:      time.Now().UTC(),
	}
	repo, rules, ruleSvc := newRuleFixture(employee)

	seedRule(t, rules, assignments.Rule{ID: "r-dept", JourneyTemplateID: "tpl-dept", DepartmentID: testDeptID})
	seedRule(t, rules, assignments.Rule{ID: "r-role", JourneyTemplateID: "tpl-role", JobRoleID: testRoleID})
	seedRule(t, rules, assignments.Rule{ID: "r-all", JourneyTemplateID: "tpl-all"})
	seedRule(t, rules, assignments.Rule{ID: "r-other-dept", DepartmentID: "dept-2"})
	seedRule(t, rules, assignments.Rule{ID: "r-other-role", JobRoleID: "role-2"})
	inactive := assignments.Rule{ID: "r-inactive", DepartmentID: testDeptID}
	seedRule(t, rules, inactive)
	inactive.Active = false
	rules.rules[inactive.ID] = inactive

	if err := ruleSvc.ApplyAssignmentRules(context.Background(), employee); err != nil {
		t.Fatalf("ApplyAssignmentRules: %v", err)
	}

	if len(repo.assignments) != 3 {
		t.Fatalf("assignments = %d, want 3 (dept, role, all match)", len(repo.assignments))
	}
}

func TestApplyAssignmentRulesSkipsAlreadyAssigned(t *testing.T) {
	t.Parallel()

	employee := employees.Employee{
		ID:             testEmployeeID,
		OrganizationID: testOrgID,
		DepartmentID:   testDeptID,
		Status:         "invited",
		StartDate:      time.Now().UTC(),
	}
	repo, rules, ruleSvc := newRuleFixture(employee)
	seedRule(t, rules, assignments.Rule{DepartmentID: testDeptID})

	for i := range 2 {
		if err := ruleSvc.ApplyAssignmentRules(context.Background(), employee); err != nil {
			t.Fatalf("ApplyAssignmentRules run %d: %v", i, err)
		}
	}

	if len(repo.assignments) != 1 {
		t.Fatalf("assignments = %d, want 1 (re-run must skip already assigned)", len(repo.assignments))
	}
}

func TestApplyAssignmentRulesTenantIsolation(t *testing.T) {
	t.Parallel()

	employee := employees.Employee{
		ID:             testEmployeeID,
		OrganizationID: "org-2",
		DepartmentID:   testDeptID,
		Status:         "invited",
		StartDate:      time.Now().UTC(),
	}
	repo, rules, ruleSvc := newRuleFixture(employee)
	// Rule belongs to org-1; the org-2 employee must never match it.
	seedRule(t, rules, assignments.Rule{OrganizationID: testOrgID, DepartmentID: testDeptID})

	if err := ruleSvc.ApplyAssignmentRules(context.Background(), employee); err != nil {
		t.Fatalf("ApplyAssignmentRules: %v", err)
	}

	if len(repo.assignments) != 0 {
		t.Fatalf("assignments = %d, want 0 (cross-tenant rule must not apply)", len(repo.assignments))
	}
}

func TestRunRuleAssignsMatchingActiveEmployees(t *testing.T) {
	t.Parallel()

	matching := employees.Employee{
		ID:             "emp-a",
		OrganizationID: testOrgID,
		DepartmentID:   testDeptID,
		Status:         "active",
	}
	wrongDept := employees.Employee{
		ID:             "emp-b",
		OrganizationID: testOrgID,
		DepartmentID:   "dept-2",
		Status:         "active",
	}
	offboarded := employees.Employee{
		ID:             "emp-c",
		OrganizationID: testOrgID,
		DepartmentID:   testDeptID,
		Status:         "offboarded",
	}
	repo, rules, ruleSvc := newRuleFixture(matching, wrongDept, offboarded)
	seedRule(t, rules, assignments.Rule{DepartmentID: testDeptID})

	result, err := ruleSvc.RunRule(context.Background(), testOrgID, "admin-1", testRuleID)
	if err != nil {
		t.Fatalf("RunRule: %v", err)
	}

	if result.Employees != 1 || result.Assigned != 1 || result.Skipped != 0 {
		t.Fatalf("result = %+v, want {1 1 0}", result)
	}

	// Re-running skips the already-assigned employee.
	result, err = ruleSvc.RunRule(context.Background(), testOrgID, "admin-1", testRuleID)
	if err != nil {
		t.Fatalf("RunRule re-run: %v", err)
	}

	if result.Employees != 1 || result.Assigned != 0 || result.Skipped != 1 {
		t.Fatalf("re-run result = %+v, want {1 0 1}", result)
	}

	if len(repo.assignments) != 1 {
		t.Fatalf("assignments = %d, want 1", len(repo.assignments))
	}
}

func TestRunRuleRejectsInactiveRule(t *testing.T) {
	t.Parallel()

	_, rules, ruleSvc := newRuleFixture()
	seedRule(t, rules, assignments.Rule{})
	rule := rules.rules[testRuleID]
	rule.Active = false
	rules.rules[testRuleID] = rule

	_, err := ruleSvc.RunRule(context.Background(), testOrgID, "admin-1", testRuleID)
	if !errors.Is(err, assignments.ErrInvalidState) {
		t.Fatalf("err = %v, want ErrInvalidState", err)
	}
}

func TestCreateRuleRequiresPublishedJourney(t *testing.T) {
	t.Parallel()

	rules := newMemoryRuleRepo()
	ruleSvc := assignments.NewRuleService(rules, failingJourneys{}, stubEmployees{}, nil)

	_, err := ruleSvc.CreateRule(context.Background(), testOrgID, "admin-1", assignments.CreateRuleInput{
		Name:              "Engineers",
		JourneyTemplateID: testTemplate,
	})
	if !errors.Is(err, journeys.ErrNotPublished) {
		t.Fatalf("err = %v, want ErrNotPublished", err)
	}
}

func TestRuleCRUDLifecycle(t *testing.T) {
	t.Parallel()

	rules := newMemoryRuleRepo()
	ruleSvc := assignments.NewRuleService(rules, echoJourneys{}, stubEmployees{}, nil)

	created, err := ruleSvc.CreateRule(context.Background(), testOrgID, "admin-1", assignments.CreateRuleInput{
		Name:              "All new hires",
		JourneyTemplateID: testTemplate,
	})
	if err != nil {
		t.Fatalf("CreateRule: %v", err)
	}

	if !created.Active || created.CreatedBy != "admin-1" {
		t.Fatalf("created = %+v, want active rule created by admin-1", created)
	}

	updated, err := ruleSvc.UpdateRule(context.Background(), testOrgID, created.ID, assignments.UpdateRuleInput{
		Name:              "Dept hires",
		JourneyTemplateID: testTemplate,
		DepartmentID:      testDeptID,
		Active:            false,
	})
	if err != nil {
		t.Fatalf("UpdateRule: %v", err)
	}

	if updated.Name != "Dept hires" || updated.DepartmentID != testDeptID || updated.Active {
		t.Fatalf("updated = %+v", updated)
	}

	// Updates are tenant-scoped: the same rule id under another org fails.
	if _, err := ruleSvc.UpdateRule(context.Background(), "org-2", created.ID, assignments.UpdateRuleInput{
		Name:              "Hijack",
		JourneyTemplateID: testTemplate,
	}); !errors.Is(err, assignments.ErrNotFound) {
		t.Fatalf("cross-tenant update err = %v, want ErrNotFound", err)
	}

	if err := ruleSvc.DeleteRule(context.Background(), testOrgID, created.ID); err != nil {
		t.Fatalf("DeleteRule: %v", err)
	}

	if _, err := rules.GetRule(context.Background(), testOrgID, created.ID); !errors.Is(err, assignments.ErrNotFound) {
		t.Fatalf("get after delete err = %v, want ErrNotFound", err)
	}
}
