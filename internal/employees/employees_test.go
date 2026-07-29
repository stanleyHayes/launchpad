package employees_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"launchpad/internal/audit"
	"launchpad/internal/employees"
	"launchpad/internal/entitlements"
	"launchpad/pkg/security"
)

type noopReferences struct{}

type starterPlanReader struct{}

func (starterPlanReader) PlanCode(context.Context, string) (string, error) {
	return "starter", nil
}

func (noopReferences) EnsureDepartmentExists(context.Context, string, string) error {
	return nil
}

func (noopReferences) EnsureJobRoleExists(context.Context, string, string) error {
	return nil
}

func TestCreateRejectsInvalidEmail(t *testing.T) {
	t.Parallel()

	svc := employees.NewService(nil, noopReferences{})

	_, err := svc.Create(context.Background(), "org-1", employees.CreateInput{
		FirstName: "Ada",
		LastName:  "Lovelace",
		WorkEmail: "not-an-email",
		StartDate: time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC),
	})
	if err == nil {
		t.Fatal("expected validation error")
	}

	if !errors.Is(err, employees.ErrInvalidInput) {
		t.Fatalf("got %v want %v", err, employees.ErrInvalidInput)
	}
}

func TestCreateRejectsZeroStartDate(t *testing.T) {
	t.Parallel()

	svc := employees.NewService(nil, noopReferences{})

	_, err := svc.Create(context.Background(), "org-1", employees.CreateInput{
		FirstName: "Ada",
		LastName:  "Lovelace",
		WorkEmail: "ada@example.com",
	})
	if err == nil {
		t.Fatal("expected validation error")
	}

	if !errors.Is(err, employees.ErrInvalidInput) {
		t.Fatalf("got %v want %v", err, employees.ErrInvalidInput)
	}
}

func TestCreateRejectsEmployeeAbovePlanLimit(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	for index := 0; index < 25; index++ {
		id := fmt.Sprintf("employee-%d", index)
		repo.items[id] = employees.Employee{ID: id, OrganizationID: "org-1"}
	}
	svc := employees.NewService(repo, noopReferences{}).WithPlanLimits(starterPlanReader{})

	_, err := svc.Create(context.Background(), "org-1", employees.CreateInput{
		FirstName: "Ada",
		LastName:  "Lovelace",
		WorkEmail: "ada@example.com",
		StartDate: time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC),
	})
	if !errors.Is(err, entitlements.ErrLimitExceeded) {
		t.Fatalf("got %v, want ErrLimitExceeded", err)
	}
}

// memoryRepo is an in-memory employees.Repository.
type memoryRepo struct {
	items          map[string]employees.Employee
	provisionCalls int
	updateCalls    int
}

func newMemoryRepo() *memoryRepo {
	return &memoryRepo{items: map[string]employees.Employee{}}
}

func (m *memoryRepo) EnsureIndexes(context.Context) error { return nil }

func (m *memoryRepo) Create(_ context.Context, employee employees.Employee) error {
	m.items[employee.ID] = employee

	return nil
}

func (m *memoryRepo) GetByID(_ context.Context, organizationID, employeeID string) (employees.Employee, error) {
	employee, ok := m.items[employeeID]
	if !ok || employee.OrganizationID != organizationID {
		return employees.Employee{}, employees.ErrNotFound
	}

	return employee, nil
}

func (m *memoryRepo) GetByUserID(_ context.Context, organizationID, userID string) (employees.Employee, error) {
	for _, employee := range m.items {
		if employee.OrganizationID == organizationID && employee.UserID == userID {
			return employee, nil
		}
	}

	return employees.Employee{}, employees.ErrNotFound
}

func (m *memoryRepo) GetByWorkEmail(_ context.Context, organizationID, workEmail string) (employees.Employee, error) {
	for _, employee := range m.items {
		if employee.OrganizationID == organizationID && employee.WorkEmail == workEmail {
			return employee, nil
		}
	}

	return employees.Employee{}, employees.ErrNotFound
}

func (m *memoryRepo) List(_ context.Context, organizationID string, offset, limit int64) ([]employees.Employee, error) {
	items := make([]employees.Employee, 0)
	for _, employee := range m.items {
		if employee.OrganizationID == organizationID {
			items = append(items, employee)
		}
	}

	if offset > int64(len(items)) {
		return []employees.Employee{}, nil
	}

	items = items[offset:]
	if limit > 0 && limit < int64(len(items)) {
		items = items[:limit]
	}

	return items, nil
}

func (m *memoryRepo) Count(_ context.Context, organizationID string) (int64, error) {
	count := int64(0)

	for _, employee := range m.items {
		if employee.OrganizationID == organizationID {
			count++
		}
	}

	return count, nil
}

func (m *memoryRepo) Update(_ context.Context, employee employees.Employee) error {
	m.updateCalls++
	m.items[employee.ID] = employee

	return nil
}

func (m *memoryRepo) ProvisionAccess(_ context.Context, organizationID, employeeID, userID string) error {
	m.provisionCalls++

	employee, ok := m.items[employeeID]
	if !ok || employee.OrganizationID != organizationID {
		return employees.ErrNotFound
	}

	if employee.UserID != "" {
		return employees.ErrAlreadyProvisioned
	}

	employee.UserID = userID
	employee.Status = "active"
	m.items[employeeID] = employee

	return nil
}

func TestUpdateRejectsSelfAsManager(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	repo.items["emp-1"] = employees.Employee{ID: "emp-1", OrganizationID: "org-1"}

	svc := employees.NewService(repo, noopReferences{})
	managerID := "emp-1"

	_, err := svc.Update(context.Background(), "org-1", "emp-1", employees.UpdateInput{
		ManagerEmployeeID: &managerID,
	})
	if !errors.Is(err, employees.ErrInvalidReference) {
		t.Fatalf("got %v want %v", err, employees.ErrInvalidReference)
	}
}

func TestUpdateRejectsInvalidStatus(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	repo.items["emp-1"] = employees.Employee{ID: "emp-1", OrganizationID: "org-1", Status: "active"}

	svc := employees.NewService(repo, noopReferences{})
	status := "deactivated"

	_, err := svc.Update(context.Background(), "org-1", "emp-1", employees.UpdateInput{
		Status: &status,
	})
	if !errors.Is(err, employees.ErrInvalidInput) {
		t.Fatalf("got %v want %v", err, employees.ErrInvalidInput)
	}

	if repo.updateCalls != 0 {
		t.Fatalf("expected no persisted update, got %d", repo.updateCalls)
	}
}

func TestUpdateRejectsBlankFirstName(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	repo.items["emp-1"] = employees.Employee{ID: "emp-1", OrganizationID: "org-1", FirstName: "Ada"}

	svc := employees.NewService(repo, noopReferences{})
	name := "   "

	_, err := svc.Update(context.Background(), "org-1", "emp-1", employees.UpdateInput{
		FirstName: &name,
	})
	if !errors.Is(err, employees.ErrInvalidInput) {
		t.Fatalf("got %v want %v", err, employees.ErrInvalidInput)
	}
}

func TestUpdateRejectsSelfAsBuddy(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	repo.items["emp-1"] = employees.Employee{ID: "emp-1", OrganizationID: "org-1"}

	svc := employees.NewService(repo, noopReferences{})
	buddyID := "emp-1"

	_, err := svc.Update(context.Background(), "org-1", "emp-1", employees.UpdateInput{
		BuddyEmployeeID: &buddyID,
	})
	if !errors.Is(err, employees.ErrInvalidReference) {
		t.Fatalf("got %v want %v", err, employees.ErrInvalidReference)
	}
}

func TestUpdateRejectsMissingBuddy(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	repo.items["emp-1"] = employees.Employee{ID: "emp-1", OrganizationID: "org-1"}

	svc := employees.NewService(repo, noopReferences{})
	buddyID := "emp-missing"

	_, err := svc.Update(context.Background(), "org-1", "emp-1", employees.UpdateInput{
		BuddyEmployeeID: &buddyID,
	})
	if !errors.Is(err, employees.ErrInvalidReference) {
		t.Fatalf("got %v want %v", err, employees.ErrInvalidReference)
	}
}

func TestUpdateAssignsBuddy(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	repo.items["emp-1"] = employees.Employee{ID: "emp-1", OrganizationID: "org-1", Status: "invited"}
	repo.items["emp-2"] = employees.Employee{ID: "emp-2", OrganizationID: "org-1", Status: "active"}

	svc := employees.NewService(repo, noopReferences{})
	buddyID := "emp-2"

	updated, err := svc.Update(context.Background(), "org-1", "emp-1", employees.UpdateInput{
		BuddyEmployeeID: &buddyID,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	if updated.BuddyEmployeeID != "emp-2" {
		t.Fatalf("buddy = %q, want emp-2", updated.BuddyEmployeeID)
	}
}

func TestCreateRejectsMissingBuddy(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	svc := employees.NewService(repo, noopReferences{})

	_, err := svc.Create(context.Background(), "org-1", employees.CreateInput{
		FirstName:       "Ada",
		LastName:        "Lovelace",
		WorkEmail:       "ada@example.com",
		BuddyEmployeeID: "emp-missing",
		StartDate:       time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC),
	})
	if !errors.Is(err, employees.ErrInvalidReference) {
		t.Fatalf("got %v want %v", err, employees.ErrInvalidReference)
	}
}

func TestOffboardSetsStatus(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	repo.items["emp-1"] = employees.Employee{ID: "emp-1", OrganizationID: "org-1", Status: "active"}

	svc := employees.NewService(repo, noopReferences{})

	updated, err := svc.Offboard(context.Background(), "org-1", "emp-1")
	if err != nil {
		t.Fatalf("Offboard: %v", err)
	}

	if updated.Status != "offboarded" {
		t.Fatalf("status = %q, want offboarded", updated.Status)
	}

	if repo.items["emp-1"].Status != "offboarded" {
		t.Fatalf("persisted status = %q, want offboarded", repo.items["emp-1"].Status)
	}
}

// recordingAuditRepo captures written audit events.
type recordingAuditRepo struct {
	events []audit.Event
}

func (r *recordingAuditRepo) EnsureIndexes(context.Context) error { return nil }

func (r *recordingAuditRepo) Write(_ context.Context, event audit.Event) error {
	r.events = append(r.events, event)

	return nil
}

func (r *recordingAuditRepo) ListByOrganization(
	context.Context,
	string,
	int64,
) ([]audit.Event, error) {
	return r.events, nil
}

func (r *recordingAuditRepo) ListAll(context.Context, int64) ([]audit.Event, error) {
	return r.events, nil
}

func newAuthedRequest(
	method, target, body string,
	principal security.Principal,
	params map[string]string,
) *http.Request {
	rctx := chi.NewRouteContext()
	for key, value := range params {
		rctx.URLParams.Add(key, value)
	}

	ctx := context.WithValue(context.Background(), chi.RouteCtxKey, rctx)
	ctx = security.WithPrincipal(ctx, principal)

	return httptest.NewRequestWithContext(ctx, method, target, strings.NewReader(body))
}

func TestHandleUpdateOffboardRecordsAudit(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	repo.items["emp-1"] = employees.Employee{
		ID: "emp-1", OrganizationID: "org-1",
		FirstName: "Ada", LastName: "Lovelace",
		WorkEmail: "ada@example.com", Status: "active",
	}

	svc := employees.NewService(repo, noopReferences{})
	auditRepo := &recordingAuditRepo{}
	handler := employees.NewHandler(svc, audit.NewService(auditRepo), failingAccountCreator{}, &stubMemberAdder{})

	principal := security.Principal{UserID: "admin-1", OrganizationID: "org-1"}
	req := newAuthedRequest(
		http.MethodPatch,
		"/api/v1/employees/emp-1",
		`{"status":"offboarded"}`,
		principal,
		map[string]string{"employeeID": "emp-1"},
	)
	rec := httptest.NewRecorder()

	handler.HandleUpdate(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}

	var envelope struct {
		Data employees.Employee `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	updated := envelope.Data
	if updated.Status != "offboarded" {
		t.Fatalf("status = %q, want offboarded", updated.Status)
	}

	if len(auditRepo.events) != 1 || auditRepo.events[0].Action != "employee.offboarded" {
		t.Fatalf("expected one employee.offboarded event, got %+v", auditRepo.events)
	}

	if auditRepo.events[0].ActorUserID != "admin-1" {
		t.Errorf("actor = %q, want admin-1", auditRepo.events[0].ActorUserID)
	}
}

func TestHandleUpdateRejectsInvalidStatus(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	repo.items["emp-1"] = employees.Employee{
		ID: "emp-1", OrganizationID: "org-1",
		FirstName: "Ada", LastName: "Lovelace",
		WorkEmail: "ada@example.com", Status: "active",
	}

	svc := employees.NewService(repo, noopReferences{})
	auditRepo := &recordingAuditRepo{}
	handler := employees.NewHandler(svc, audit.NewService(auditRepo), failingAccountCreator{}, &stubMemberAdder{})

	principal := security.Principal{UserID: "admin-1", OrganizationID: "org-1"}
	req := newAuthedRequest(
		http.MethodPatch,
		"/api/v1/employees/emp-1",
		`{"status":"deactivated"}`,
		principal,
		map[string]string{"employeeID": "emp-1"},
	)
	rec := httptest.NewRecorder()

	handler.HandleUpdate(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
	}

	if len(auditRepo.events) != 0 {
		t.Fatalf("expected no audit events, got %+v", auditRepo.events)
	}
}

func TestHandleUpdateRejectsSelfBuddy(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	repo.items["emp-1"] = employees.Employee{
		ID: "emp-1", OrganizationID: "org-1",
		FirstName: "Ada", LastName: "Lovelace",
		WorkEmail: "ada@example.com", Status: "active",
	}

	svc := employees.NewService(repo, noopReferences{})
	auditRepo := &recordingAuditRepo{}
	handler := employees.NewHandler(svc, audit.NewService(auditRepo), failingAccountCreator{}, &stubMemberAdder{})

	principal := security.Principal{UserID: "admin-1", OrganizationID: "org-1"}
	req := newAuthedRequest(
		http.MethodPatch,
		"/api/v1/employees/emp-1",
		`{"buddyEmployeeId":"emp-1"}`,
		principal,
		map[string]string{"employeeID": "emp-1"},
	)
	rec := httptest.NewRecorder()

	handler.HandleUpdate(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestLinkUserProvisionsAccessAtomically(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	repo.items["emp-1"] = employees.Employee{ID: "emp-1", OrganizationID: "org-1", Status: "invited"}

	svc := employees.NewService(repo, noopReferences{})

	employee, err := svc.LinkUser(context.Background(), "org-1", "emp-1", "user-1")
	if err != nil {
		t.Fatalf("LinkUser: %v", err)
	}

	if employee.UserID != "user-1" || employee.Status != "active" {
		t.Fatalf("employee not linked/activated: %+v", employee)
	}

	if repo.provisionCalls != 1 || repo.updateCalls != 0 {
		t.Fatalf("expected atomic ProvisionAccess only, got provision=%d update=%d",
			repo.provisionCalls, repo.updateCalls)
	}

	_, err = svc.LinkUser(context.Background(), "org-1", "emp-1", "user-2")
	if !errors.Is(err, employees.ErrAlreadyProvisioned) {
		t.Fatalf("second link: got %v want %v", err, employees.ErrAlreadyProvisioned)
	}
}

type failingAccountCreator struct {
	existingUserID string
}

func (f failingAccountCreator) CreateUserAccount(context.Context, string, string, string) (string, error) {
	return "", errors.New("insert user: email taken")
}

func (f failingAccountCreator) FindUserIDByEmail(context.Context, string) (string, error) {
	return f.existingUserID, nil
}

type stubMemberAdder struct {
	calls int
}

func (s *stubMemberAdder) AddEmployeeMember(context.Context, string, string) error {
	s.calls++

	return nil
}

func TestProvisionerReusesAccountFromFailedAttempt(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	repo.items["emp-1"] = employees.Employee{
		ID: "emp-1", OrganizationID: "org-1",
		FirstName: "Ada", LastName: "Lovelace",
		WorkEmail: "ada@example.com", Status: "invited",
	}

	svc := employees.NewService(repo, noopReferences{})
	members := &stubMemberAdder{}
	provisioner := employees.NewProvisioner(svc, failingAccountCreator{existingUserID: "user-9"}, members)

	updated, userID, err := provisioner.Provision(context.Background(), "org-1", "emp-1", "", "s3cret-password")
	if err != nil {
		t.Fatalf("Provision: %v", err)
	}

	if userID != "user-9" || updated.UserID != "user-9" {
		t.Fatalf("expected reused account user-9, got userID=%q employee=%+v", userID, updated)
	}

	if members.calls != 1 {
		t.Fatalf("expected 1 membership add, got %d", members.calls)
	}
}

func TestProvisionerRejectsAlreadyProvisioned(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	repo.items["emp-1"] = employees.Employee{
		ID: "emp-1", OrganizationID: "org-1",
		FirstName: "Ada", LastName: "Lovelace",
		WorkEmail: "ada@example.com", Status: "active", UserID: "user-1",
	}

	svc := employees.NewService(repo, noopReferences{})
	members := &stubMemberAdder{}
	provisioner := employees.NewProvisioner(svc, failingAccountCreator{}, members)

	_, _, err := provisioner.Provision(context.Background(), "org-1", "emp-1", "", "s3cret-password")
	if !errors.Is(err, employees.ErrAlreadyProvisioned) {
		t.Fatalf("got %v want %v", err, employees.ErrAlreadyProvisioned)
	}

	if members.calls != 0 {
		t.Fatalf("expected no membership add, got %d", members.calls)
	}
}

func (m *memoryRepo) DeleteForOrganization(context.Context, string) (int64, error) {
	return 0, nil
}

func (r *recordingAuditRepo) CountByOrganization(context.Context, string) (int64, error) {
	return 0, nil
}

func (r *recordingAuditRepo) DeleteForOrganization(context.Context, string) (int64, error) {
	return 0, nil
}
