package privacy_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"launchpad/internal/assignments"
	"launchpad/internal/audit"
	"launchpad/internal/departments"
	"launchpad/internal/employees"
	"launchpad/internal/journeys"
	"launchpad/internal/organizations"
	"launchpad/internal/privacy"
)

const (
	testOrgID   = "org-1"
	testOrgSlug = "acme"
)

// fakeOrgReader serves one organization and its memberships.
type fakeOrgReader struct {
	org         organizations.Organization
	orgErr      error
	memberships []organizations.Membership
}

func (f *fakeOrgReader) GetByID(context.Context, string) (organizations.Organization, error) {
	return f.org, f.orgErr
}

func (f *fakeOrgReader) ListMemberships(context.Context, string) ([]organizations.Membership, error) {
	return f.memberships, nil
}

// fakeEmployeeLister pages a fixed employee list like the Mongo repository.
type fakeEmployeeLister struct {
	items []employees.Employee
}

func (f *fakeEmployeeLister) List(
	_ context.Context,
	_ string,
	offset, limit int64,
) ([]employees.Employee, error) {
	if offset >= int64(len(f.items)) {
		return []employees.Employee{}, nil
	}

	end := min(offset+limit, int64(len(f.items)))

	return f.items[offset:end], nil
}

type fakeDepartmentLister struct {
	departments []departments.Department
	jobRoles    []departments.JobRole
}

func (f *fakeDepartmentLister) ListDepartments(context.Context, string) ([]departments.Department, error) {
	return f.departments, nil
}

func (f *fakeDepartmentLister) ListJobRoles(context.Context, string) ([]departments.JobRole, error) {
	return f.jobRoles, nil
}

type fakeJourneyLister struct {
	templates []journeys.Template
}

func (f *fakeJourneyLister) ListTemplates(context.Context, string) ([]journeys.Template, error) {
	return f.templates, nil
}

type fakeAssignmentLister struct {
	assignments []assignments.JourneyAssignment
	approvals   []assignments.Approval
}

func (f *fakeAssignmentLister) ListAssignments(context.Context, string) ([]assignments.JourneyAssignment, error) {
	return f.assignments, nil
}

func (f *fakeAssignmentLister) ListApprovals(context.Context, string) ([]assignments.Approval, error) {
	return f.approvals, nil
}

type fakeAuditReader struct {
	total  int64
	recent []audit.Event
}

func (f *fakeAuditReader) CountByOrganization(context.Context, string) (int64, error) {
	return f.total, nil
}

func (f *fakeAuditReader) ListByOrganization(context.Context, string, int64) ([]audit.Event, error) {
	return f.recent, nil
}

func newExportService(employeeCount int) *privacy.ExportService {
	orgs := &fakeOrgReader{
		org: organizations.Organization{ID: testOrgID, Slug: testOrgSlug, Name: "Acme"},
		memberships: []organizations.Membership{
			{ID: "m-1", OrganizationID: testOrgID, UserID: "user-1", RoleCode: "organization_owner"},
		},
	}

	employeeList := make([]employees.Employee, 0, employeeCount)
	for range employeeCount {
		employeeList = append(employeeList, employees.Employee{OrganizationID: testOrgID})
	}

	return privacy.NewExportService(
		orgs,
		&fakeEmployeeLister{items: employeeList},
		&fakeDepartmentLister{
			departments: []departments.Department{{ID: "dep-1", OrganizationID: testOrgID}},
			jobRoles:    []departments.JobRole{{ID: "role-1", OrganizationID: testOrgID}},
		},
		&fakeJourneyLister{templates: []journeys.Template{{ID: "j-1", OrganizationID: testOrgID}}},
		&fakeAssignmentLister{
			assignments: []assignments.JourneyAssignment{{ID: "a-1", OrganizationID: testOrgID}},
			approvals:   []assignments.Approval{{ID: "ap-1", OrganizationID: testOrgID}},
		},
		&fakeAuditReader{total: 137, recent: []audit.Event{{ID: "ev-1"}}},
	)
}

func TestExportAssemblesOrganizationData(t *testing.T) {
	t.Parallel()

	svc := newExportService(1)

	export, err := svc.Export(context.Background(), testOrgID)
	if err != nil {
		t.Fatalf("export: %v", err)
	}

	if export.Organization.ID != testOrgID || export.Organization.Slug != testOrgSlug {
		t.Fatalf("organization = %+v", export.Organization)
	}

	if len(export.Memberships) != 1 || export.Memberships[0].UserID != "user-1" {
		t.Fatalf("memberships = %+v", export.Memberships)
	}

	if len(export.Employees) != 1 {
		t.Fatalf("employees = %d, want 1", len(export.Employees))
	}

	if len(export.Departments) != 1 || len(export.JobRoles) != 1 {
		t.Fatalf("departments = %+v jobRoles = %+v", export.Departments, export.JobRoles)
	}

	if len(export.Journeys) != 1 || len(export.Assignments) != 1 || len(export.Approvals) != 1 {
		t.Fatalf("journeys = %+v assignments = %+v approvals = %+v",
			export.Journeys, export.Assignments, export.Approvals)
	}

	if export.AuditEvents.Total != 137 || len(export.AuditEvents.Recent) != 1 {
		t.Fatalf("auditEvents = %+v", export.AuditEvents)
	}

	if export.GeneratedAt.IsZero() {
		t.Fatal("generatedAt not set")
	}
}

func TestExportPagesThroughAllEmployees(t *testing.T) {
	t.Parallel()

	// 150 employees forces two pages of the repository's 100-per-page maximum.
	svc := newExportService(150)

	export, err := svc.Export(context.Background(), testOrgID)
	if err != nil {
		t.Fatalf("export: %v", err)
	}

	if len(export.Employees) != 150 {
		t.Fatalf("employees = %d, want 150", len(export.Employees))
	}
}

func TestExportPropagatesOrganizationErrors(t *testing.T) {
	t.Parallel()

	svc := privacy.NewExportService(
		&fakeOrgReader{orgErr: organizations.ErrNotFound},
		&fakeEmployeeLister{},
		&fakeDepartmentLister{},
		&fakeJourneyLister{},
		&fakeAssignmentLister{},
		&fakeAuditReader{},
	)

	if _, err := svc.Export(context.Background(), "ghost"); !errors.Is(err, organizations.ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
}

// recordingPurger records the organization ids it was asked to delete.
type recordingPurger struct {
	orgIDs []string
	count  int64
	err    error
}

func (p *recordingPurger) DeleteForOrganization(_ context.Context, organizationID string) (int64, error) {
	p.orgIDs = append(p.orgIDs, organizationID)

	return p.count, p.err
}

type fakeResetPurger struct {
	userIDs []string
	count   int64
}

func (p *fakeResetPurger) DeleteForUsers(_ context.Context, userIDs []string) (int64, error) {
	p.userIDs = append(p.userIDs, userIDs...)

	return p.count, nil
}

type tombstoneEvent struct {
	organizationID *string
	actorUserID    string
	action         string
	resourceID     string
	metadata       map[string]any
}

type fakeTombstone struct {
	events []tombstoneEvent
}

func (f *fakeTombstone) Record(
	_ context.Context,
	organizationID *string,
	actorUserID, action, _, resourceID string,
	metadata map[string]any,
) error {
	f.events = append(f.events, tombstoneEvent{
		organizationID: organizationID,
		actorUserID:    actorUserID,
		action:         action,
		resourceID:     resourceID,
		metadata:       metadata,
	})

	return nil
}

// purgeFixture wires one recording purger per store so tests can verify that
// every collection is purged and always with the target tenant id.
type purgeFixture struct {
	purgers map[string]*recordingPurger
	resets  *fakeResetPurger
	tomb    *fakeTombstone
	svc     *privacy.PurgeService
}

func newPurgeFixture() *purgeFixture {
	labels := []string{
		"organizations", "employees", "departments", "journeys", "assignments",
		"notifications", "notificationChannels", "notificationDeliveries", "knowledge", "assistantChunks",
		"assistantInteractions", "auditEvents", "roles", "integrations",
		"hrisConfigs", "hrisState", "ssoConfigs", "samlConfigs", "scimUsers", "scimTokens",
		"scimGroups", "billingSubscriptions", "support", "featureFlagOverrides",
		"invitations",
	}

	purgers := make(map[string]*recordingPurger, len(labels))
	for _, label := range labels {
		purgers[label] = &recordingPurger{count: 2}
	}

	resets := &fakeResetPurger{count: 1}
	tomb := &fakeTombstone{}

	stores := privacy.PurgeStores{
		Organizations:          purgers["organizations"],
		Employees:              purgers["employees"],
		Departments:            purgers["departments"],
		Journeys:               purgers["journeys"],
		Assignments:            purgers["assignments"],
		Notifications:          purgers["notifications"],
		NotificationChannels:   purgers["notificationChannels"],
		NotificationDeliveries: purgers["notificationDeliveries"],
		Knowledge:              purgers["knowledge"],
		AssistantChunks:        purgers["assistantChunks"],
		AssistantInteractions:  purgers["assistantInteractions"],
		Audit:                  purgers["auditEvents"],
		Roles:                  purgers["roles"],
		Integrations:           purgers["integrations"],
		HRISConfigs:            purgers["hrisConfigs"],
		HRISState:              purgers["hrisState"],
		SSOConfigs:             purgers["ssoConfigs"],
		SAMLConfigs:            purgers["samlConfigs"],
		SCIMUsers:              purgers["scimUsers"],
		SCIMTokens:             purgers["scimTokens"],
		SCIMGroups:             purgers["scimGroups"],
		BillingSubscriptions:   purgers["billingSubscriptions"],
		Support:                purgers["support"],
		FeatureFlagOverrides:   purgers["featureFlagOverrides"],
		Invitations:            purgers["invitations"],
		PasswordResets:         resets,
	}

	orgs := &fakeOrgReader{
		org: organizations.Organization{ID: testOrgID, Slug: testOrgSlug},
		memberships: []organizations.Membership{
			{OrganizationID: testOrgID, UserID: "user-1"},
			{OrganizationID: testOrgID, UserID: "user-2"},
		},
	}

	employeeLister := &fakeEmployeeLister{items: []employees.Employee{
		{ID: "emp-1", OrganizationID: testOrgID, UserID: "user-2"},
		{ID: "emp-2", OrganizationID: testOrgID, UserID: "user-3"},
	}}

	return &purgeFixture{
		purgers: purgers,
		resets:  resets,
		tomb:    tomb,
		svc:     privacy.NewPurgeService(orgs, employeeLister, stores, tomb),
	}
}

func TestPurgeDeletesEveryCollectionForTheTenant(t *testing.T) {
	t.Parallel()

	fixture := newPurgeFixture()

	result, err := fixture.svc.Purge(context.Background(), testOrgID, testOrgSlug, "staff-1")
	if err != nil {
		t.Fatalf("purge: %v", err)
	}

	for label, purger := range fixture.purgers {
		if !slices.Equal(purger.orgIDs, []string{testOrgID}) {
			t.Fatalf("%s purged org ids = %v, want [%s]", label, purger.orgIDs, testOrgID)
		}

		if result.Deleted[label] != 2 {
			t.Fatalf("deleted[%s] = %d, want 2", label, result.Deleted[label])
		}
	}

	// Password resets go out per member user id: the membership users plus
	// the employee-linked user, deduped.
	if !slices.Equal(fixture.resets.userIDs, []string{"user-1", "user-2", "user-3"}) {
		t.Fatalf("password reset user ids = %v", fixture.resets.userIDs)
	}

	if result.Deleted["passwordResets"] != 1 {
		t.Fatalf("deleted[passwordResets] = %d, want 1", result.Deleted["passwordResets"])
	}

	if result.OrganizationID != testOrgID || result.Slug != testOrgSlug || result.PurgedAt.IsZero() {
		t.Fatalf("result = %+v", result)
	}
}

func TestPurgeWritesPlatformLevelTombstone(t *testing.T) {
	t.Parallel()

	fixture := newPurgeFixture()

	if _, err := fixture.svc.Purge(context.Background(), testOrgID, testOrgSlug, "staff-1"); err != nil {
		t.Fatalf("purge: %v", err)
	}

	if len(fixture.tomb.events) != 1 {
		t.Fatalf("tombstone events = %d, want 1", len(fixture.tomb.events))
	}

	event := fixture.tomb.events[0]

	if event.organizationID != nil {
		t.Fatalf("tombstone organization id = %v, want nil (platform level)", *event.organizationID)
	}

	if event.actorUserID != "staff-1" || event.action != "organization.purged" || event.resourceID != testOrgID {
		t.Fatalf("tombstone = %+v", event)
	}

	if event.metadata["slug"] != testOrgSlug {
		t.Fatalf("tombstone metadata = %v", event.metadata)
	}
}

func TestPurgeRejectsWrongConfirmation(t *testing.T) {
	t.Parallel()

	fixture := newPurgeFixture()

	_, err := fixture.svc.Purge(context.Background(), testOrgID, "not-the-slug", "staff-1")
	if !errors.Is(err, privacy.ErrConfirmationMismatch) {
		t.Fatalf("got %v, want ErrConfirmationMismatch", err)
	}

	for label, purger := range fixture.purgers {
		if len(purger.orgIDs) != 0 {
			t.Fatalf("%s purger called despite confirmation mismatch", label)
		}
	}

	if len(fixture.resets.userIDs) != 0 || len(fixture.tomb.events) != 0 {
		t.Fatal("deletes or tombstone happened despite confirmation mismatch")
	}
}

func TestPurgeRejectsUnknownOrganization(t *testing.T) {
	t.Parallel()

	fixture := newPurgeFixture()
	fixture.svc = privacy.NewPurgeService(
		&fakeOrgReader{orgErr: organizations.ErrNotFound},
		&fakeEmployeeLister{},
		privacy.PurgeStores{PasswordResets: fixture.resets},
		fixture.tomb,
	)

	_, purgeErr := fixture.svc.Purge(context.Background(), "ghost", "acme", "staff-1")
	if !errors.Is(purgeErr, organizations.ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound", purgeErr)
	}
}

func TestPurgePropagatesStoreFailures(t *testing.T) {
	t.Parallel()

	fixture := newPurgeFixture()
	fixture.purgers["employees"].err = errors.New("mongo down")

	if _, err := fixture.svc.Purge(context.Background(), testOrgID, testOrgSlug, "staff-1"); err == nil {
		t.Fatal("expected purge to fail when a store fails")
	}

	// Employees is the first delete: nothing else may have been purged.
	for label, purger := range fixture.purgers {
		if label == "employees" {
			continue
		}

		if len(purger.orgIDs) != 0 {
			t.Fatalf("%s purger ran after the employees failure", label)
		}
	}

	if len(fixture.tomb.events) != 0 {
		t.Fatal("tombstone written despite failed purge")
	}
}
