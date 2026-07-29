package requests_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"launchpad/internal/audit"
	"launchpad/internal/employees"
	"launchpad/internal/requests"
	"launchpad/pkg/security"
)

const (
	testOrgID        = "org-1"
	testEmployeeID   = "emp-1"
	testEmployeeUser = "user-1"
	testManagerUser  = "user-manager"
)

// memoryRepo is an in-memory requests.Repository for tests.
type memoryRepo struct {
	items map[string]requests.Request
}

func newMemoryRepo() *memoryRepo {
	return &memoryRepo{items: map[string]requests.Request{}}
}

func (m *memoryRepo) EnsureIndexes(context.Context) error { return nil }

func (m *memoryRepo) Create(_ context.Context, request requests.Request) error {
	m.items[request.ID] = request

	return nil
}

func (m *memoryRepo) GetByIDForOrganization(
	_ context.Context,
	organizationID, id string,
) (requests.Request, error) {
	request, ok := m.items[id]
	if !ok || request.OrganizationID != organizationID {
		return requests.Request{}, requests.ErrNotFound
	}

	return request, nil
}

func (m *memoryRepo) Update(_ context.Context, request requests.Request) error {
	existing, ok := m.items[request.ID]
	if !ok || existing.OrganizationID != request.OrganizationID {
		return requests.ErrNotFound
	}

	m.items[request.ID] = request

	return nil
}

func (m *memoryRepo) ListByOrganization(
	_ context.Context,
	organizationID, status string,
) ([]requests.Request, error) {
	items := make([]requests.Request, 0)

	for _, request := range m.items {
		if request.OrganizationID != organizationID {
			continue
		}

		if status != "" && request.Status != status {
			continue
		}

		items = append(items, request)
	}

	return items, nil
}

func (m *memoryRepo) ListByRequester(
	_ context.Context,
	organizationID, employeeID string,
) ([]requests.Request, error) {
	items := make([]requests.Request, 0)

	for _, request := range m.items {
		if request.OrganizationID == organizationID && request.RequesterEmployeeID == employeeID {
			items = append(items, request)
		}
	}

	return items, nil
}

// stubEmployees resolves user ids to employees.
type stubEmployees struct {
	byUserID map[string]employees.Employee
}

func (s stubEmployees) GetByUserID(
	_ context.Context,
	_, userID string,
) (employees.Employee, error) {
	employee, ok := s.byUserID[userID]
	if !ok {
		return employees.Employee{}, employees.ErrNotFound
	}

	return employee, nil
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

func (r *recordingAuditRepo) CountByOrganization(context.Context, string) (int64, error) {
	return int64(len(r.events)), nil
}

func (r *recordingAuditRepo) DeleteForOrganization(context.Context, string) (int64, error) {
	deleted := int64(len(r.events))
	r.events = nil

	return deleted, nil
}

func newService(repo *memoryRepo) *requests.Service {
	return requests.NewService(repo, stubEmployees{
		byUserID: map[string]employees.Employee{
			testEmployeeUser: {ID: testEmployeeID, UserID: testEmployeeUser, OrganizationID: testOrgID},
		},
	})
}

func seedPending(t *testing.T, svc *requests.Service) requests.Request {
	t.Helper()

	request, err := svc.Create(context.Background(), requests.CreateInput{
		OrganizationID: testOrgID,
		EmployeeID:     testEmployeeID,
		Kind:           "equipment",
		Item:           "laptop",
		Details:        "Standard build",
	})
	if err != nil {
		t.Fatalf("seed request: %v", err)
	}

	return request
}

func TestRequestLifecycleCreateDecideFulfill(t *testing.T) {
	t.Parallel()

	svc := newService(newMemoryRepo())
	request := seedPending(t, svc)

	if request.Status != "pending" {
		t.Fatalf("status = %q, want pending", request.Status)
	}

	decided, err := svc.Decide(context.Background(), requests.DecideInput{
		OrganizationID: testOrgID,
		RequestID:      request.ID,
		ApproverUserID: testManagerUser,
		Approve:        true,
		Note:           "Go ahead",
	})
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}

	if decided.Status != "approved" || decided.ApproverUserID != testManagerUser ||
		decided.DecisionNote != "Go ahead" || decided.DecidedAt == nil {
		t.Fatalf("decided request = %+v, want approved with approver, note, decidedAt", decided)
	}

	fulfilled, err := svc.Fulfill(context.Background(), testOrgID, request.ID)
	if err != nil {
		t.Fatalf("Fulfill: %v", err)
	}

	if fulfilled.Status != "fulfilled" || fulfilled.FulfilledAt == nil {
		t.Fatalf("fulfilled request = %+v, want fulfilled with fulfilledAt", fulfilled)
	}
}

func TestDecideRejectAndTerminalStateRules(t *testing.T) {
	t.Parallel()

	svc := newService(newMemoryRepo())
	request := seedPending(t, svc)

	rejected, err := svc.Decide(context.Background(), requests.DecideInput{
		OrganizationID: testOrgID,
		RequestID:      request.ID,
		ApproverUserID: testManagerUser,
		Approve:        false,
	})
	if err != nil {
		t.Fatalf("Decide reject: %v", err)
	}

	if rejected.Status != "rejected" {
		t.Fatalf("status = %q, want rejected", rejected.Status)
	}

	// Rejected is terminal: no re-decision, no fulfillment, no cancel.
	if _, err := svc.Decide(context.Background(), requests.DecideInput{
		OrganizationID: testOrgID,
		RequestID:      request.ID,
		ApproverUserID: testManagerUser,
		Approve:        true,
	}); !errors.Is(err, requests.ErrInvalidState) {
		t.Fatalf("re-decide: got %v, want ErrInvalidState", err)
	}

	if _, err := svc.Fulfill(context.Background(), testOrgID, request.ID); !errors.Is(
		err,
		requests.ErrInvalidState,
	) {
		t.Fatalf("fulfill rejected: got %v, want ErrInvalidState", err)
	}

	// Fulfill requires approved, not pending.
	pending := seedPending(t, svc)
	if _, err := svc.Fulfill(context.Background(), testOrgID, pending.ID); !errors.Is(
		err,
		requests.ErrInvalidState,
	) {
		t.Fatalf("fulfill pending: got %v, want ErrInvalidState", err)
	}
}

func TestCancelRules(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	svc := newService(repo)
	request := seedPending(t, svc)

	// Another employee may not cancel it.
	otherSvc := requests.NewService(repo, stubEmployees{
		byUserID: map[string]employees.Employee{
			"user-2": {ID: "emp-2", UserID: "user-2", OrganizationID: testOrgID},
		},
	})

	if _, err := otherSvc.CancelMine(context.Background(), testOrgID, "user-2", request.ID); !errors.Is(
		err,
		requests.ErrForbidden,
	) {
		t.Fatalf("other employee cancel: got %v, want ErrForbidden", err)
	}

	cancelled, err := svc.CancelMine(context.Background(), testOrgID, testEmployeeUser, request.ID)
	if err != nil {
		t.Fatalf("CancelMine: %v", err)
	}

	if cancelled.Status != "cancelled" {
		t.Fatalf("status = %q, want cancelled", cancelled.Status)
	}

	// Only pending requests can be cancelled.
	if _, err := svc.CancelMine(context.Background(), testOrgID, testEmployeeUser, request.ID); !errors.Is(
		err,
		requests.ErrInvalidState,
	) {
		t.Fatalf("re-cancel: got %v, want ErrInvalidState", err)
	}
}

func TestListScopingAndStatusFilter(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	svc := newService(repo)
	seedPending(t, svc)

	if _, err := svc.Create(context.Background(), requests.CreateInput{
		OrganizationID: testOrgID,
		EmployeeID:     "emp-2",
		Kind:           "access",
		Item:           "vpn",
	}); err != nil {
		t.Fatalf("seed second request: %v", err)
	}

	mine, err := svc.ListMine(context.Background(), testOrgID, testEmployeeUser)
	if err != nil {
		t.Fatalf("ListMine: %v", err)
	}

	if len(mine) != 1 || mine[0].RequesterEmployeeID != testEmployeeID {
		t.Fatalf("mine = %+v, want only the caller's request", mine)
	}

	all, err := svc.List(context.Background(), testOrgID, "")
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(all) != 2 {
		t.Fatalf("all = %d requests, want 2", len(all))
	}

	if _, err := svc.List(context.Background(), testOrgID, "bogus"); !errors.Is(
		err,
		requests.ErrInvalidInput,
	) {
		t.Fatalf("status filter: got %v, want ErrInvalidInput", err)
	}
}

func TestCreateValidation(t *testing.T) {
	t.Parallel()

	svc := newService(newMemoryRepo())

	for _, in := range []requests.CreateInput{
		{OrganizationID: testOrgID, EmployeeID: testEmployeeID, Kind: "furniture", Item: "laptop"},
		{OrganizationID: testOrgID, EmployeeID: testEmployeeID, Kind: "equipment", Item: "desk"},
		{OrganizationID: testOrgID, EmployeeID: "", Kind: "equipment", Item: "laptop"},
	} {
		if _, err := svc.Create(context.Background(), in); !errors.Is(err, requests.ErrInvalidInput) {
			t.Fatalf("Create(%+v): got %v, want ErrInvalidInput", in, err)
		}
	}
}

func TestCreateFromStepDefaultsUnknownItem(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	svc := newService(repo)

	if err := svc.CreateFromStep(context.Background(), testOrgID, testEmployeeID, "access", "", "VPN step"); err != nil {
		t.Fatalf("CreateFromStep: %v", err)
	}

	items, err := repo.ListByRequester(context.Background(), testOrgID, testEmployeeID)
	if err != nil {
		t.Fatalf("ListByRequester: %v", err)
	}

	if len(items) != 1 || items[0].Item != "other" || items[0].Kind != "access" ||
		items[0].Status != "pending" {
		t.Fatalf("request = %+v, want pending access/other", items)
	}
}

func TestTenantIsolation(t *testing.T) {
	t.Parallel()

	svc := newService(newMemoryRepo())
	request := seedPending(t, svc)

	if _, err := svc.Decide(context.Background(), requests.DecideInput{
		OrganizationID: "org-2",
		RequestID:      request.ID,
		ApproverUserID: testManagerUser,
		Approve:        true,
	}); !errors.Is(err, requests.ErrNotFound) {
		t.Fatalf("cross-tenant decide: got %v, want ErrNotFound", err)
	}

	if _, err := svc.Fulfill(context.Background(), "org-2", request.ID); !errors.Is(
		err,
		requests.ErrNotFound,
	) {
		t.Fatalf("cross-tenant fulfill: got %v, want ErrNotFound", err)
	}

	all, err := svc.List(context.Background(), "org-2", "")
	if err != nil {
		t.Fatalf("List org-2: %v", err)
	}

	if len(all) != 0 {
		t.Fatalf("org-2 sees %d requests, want 0", len(all))
	}
}

func TestHandlersScopingAndAudit(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	svc := newService(repo)
	auditRepo := &recordingAuditRepo{}
	handler := requests.NewHandler(svc, audit.NewService(auditRepo))

	// Employee creates a request.
	createReq := newAuthedRequest(
		http.MethodPost,
		"/me/requests",
		`{"kind":"equipment","item":"laptop","details":"Standard build"}`,
		security.Principal{UserID: testEmployeeUser, OrganizationID: testOrgID, RoleCode: "employee"},
		nil,
	)
	createRec := httptest.NewRecorder()
	handler.HandleCreateMine(createRec, createReq)

	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201 (body: %s)", createRec.Code, createRec.Body.String())
	}

	var created requests.Request
	for _, item := range repo.items {
		created = item
	}

	// Employee list only contains their own request.
	listRec := httptest.NewRecorder()
	handler.HandleListMine(listRec, newAuthedRequest(
		http.MethodGet,
		"/me/requests",
		"",
		security.Principal{UserID: testEmployeeUser, OrganizationID: testOrgID, RoleCode: "employee"},
		nil,
	))

	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200", listRec.Code)
	}

	// Manager decides; audit event recorded.
	decideRec := httptest.NewRecorder()
	handler.HandleDecide(decideRec, newAuthedRequest(
		http.MethodPost,
		"/requests/"+created.ID+"/decide",
		`{"approve":true,"note":"ok"}`,
		security.Principal{UserID: testManagerUser, OrganizationID: testOrgID, RoleCode: "manager"},
		map[string]string{"requestID": created.ID},
	))

	if decideRec.Code != http.StatusOK {
		t.Fatalf("decide status = %d, want 200 (body: %s)", decideRec.Code, decideRec.Body.String())
	}

	var sawCreated, sawDecided bool

	for _, event := range auditRepo.events {
		if event.Action == "request.created" {
			sawCreated = true
		}

		if event.Action == "request.decided" && event.ActorUserID == testManagerUser {
			sawDecided = true
		}
	}

	if !sawCreated || !sawDecided {
		t.Fatalf("audit events = %+v, want request.created and request.decided", auditRepo.events)
	}

	// Cross-tenant decide returns 404.
	crossRec := httptest.NewRecorder()
	handler.HandleDecide(crossRec, newAuthedRequest(
		http.MethodPost,
		"/requests/"+created.ID+"/decide",
		`{"approve":true}`,
		security.Principal{UserID: "user-9", OrganizationID: "org-2", RoleCode: "manager"},
		map[string]string{"requestID": created.ID},
	))

	if crossRec.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant decide status = %d, want 404", crossRec.Code)
	}

	// A non-employee account gets 403 on self-service endpoints.
	noEmployeeRec := httptest.NewRecorder()
	handler.HandleListMine(noEmployeeRec, newAuthedRequest(
		http.MethodGet,
		"/me/requests",
		"",
		security.Principal{UserID: "user-unknown", OrganizationID: testOrgID, RoleCode: "manager"},
		nil,
	))

	if noEmployeeRec.Code != http.StatusForbidden {
		t.Fatalf("non-employee list status = %d, want 403", noEmployeeRec.Code)
	}
}

func TestHandleCancelMineForbiddenForOtherEmployee(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	svc := requests.NewService(repo, stubEmployees{
		byUserID: map[string]employees.Employee{
			testEmployeeUser: {ID: testEmployeeID, UserID: testEmployeeUser, OrganizationID: testOrgID},
			"user-2":         {ID: "emp-2", UserID: "user-2", OrganizationID: testOrgID},
		},
	})
	handler := requests.NewHandler(svc, audit.NewService(&recordingAuditRepo{}))

	request := seedPending(t, svc)

	rec := httptest.NewRecorder()
	handler.HandleCancelMine(rec, newAuthedRequest(
		http.MethodPost,
		"/me/requests/"+request.ID+"/cancel",
		"",
		security.Principal{UserID: "user-2", OrganizationID: testOrgID, RoleCode: "employee"},
		map[string]string{"requestID": request.ID},
	))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("cancel status = %d, want 403 (body: %s)", rec.Code, rec.Body.String())
	}
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

func (m *memoryRepo) DeleteForOrganization(_ context.Context, organizationID string) (int64, error) {
	var count int64

	for id, item := range m.items {
		if item.OrganizationID == organizationID {
			delete(m.items, id)

			count++
		}
	}

	return count, nil
}
