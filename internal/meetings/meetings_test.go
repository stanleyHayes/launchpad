package meetings_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"launchpad/internal/audit"
	"launchpad/internal/employees"
	"launchpad/internal/meetings"
	"launchpad/pkg/security"
)

const (
	testOrgID        = "org-1"
	otherOrgID       = "org-2"
	testEmployeeID   = "emp-1"
	testEmployeeUser = "user-1"
	testManagerUser  = "user-manager"
	testManagerEmpID = "emp-manager"
	testStartsAt     = "2030-01-15T10:00:00Z"
)

// memoryRepo is an in-memory meetings.Repository for tests.
type memoryRepo struct {
	items map[string]meetings.Meeting
}

func newMemoryRepo() *memoryRepo {
	return &memoryRepo{items: map[string]meetings.Meeting{}}
}

func (m *memoryRepo) EnsureIndexes(context.Context) error { return nil }

func (m *memoryRepo) Create(_ context.Context, meeting meetings.Meeting) error {
	m.items[meeting.ID] = meeting

	return nil
}

func (m *memoryRepo) GetByIDForOrganization(
	_ context.Context,
	organizationID, id string,
) (meetings.Meeting, error) {
	meeting, ok := m.items[id]
	if !ok || meeting.OrganizationID != organizationID {
		return meetings.Meeting{}, meetings.ErrNotFound
	}

	return meeting, nil
}

func (m *memoryRepo) Update(_ context.Context, meeting meetings.Meeting) error {
	existing, ok := m.items[meeting.ID]
	if !ok || existing.OrganizationID != meeting.OrganizationID {
		return meetings.ErrNotFound
	}

	m.items[meeting.ID] = meeting

	return nil
}

func (m *memoryRepo) ListByOrganization(
	_ context.Context,
	organizationID, status string,
) ([]meetings.Meeting, error) {
	items := make([]meetings.Meeting, 0)

	for _, meeting := range m.items {
		if meeting.OrganizationID != organizationID {
			continue
		}

		if status != "" && meeting.Status != status {
			continue
		}

		items = append(items, meeting)
	}

	return items, nil
}

func (m *memoryRepo) ListByAttendee(
	_ context.Context,
	organizationID, employeeID string,
) ([]meetings.Meeting, error) {
	items := make([]meetings.Meeting, 0)

	for _, meeting := range m.items {
		if meeting.OrganizationID == organizationID && meeting.AttendeeEmployeeID == employeeID {
			items = append(items, meeting)
		}
	}

	return items, nil
}

func (m *memoryRepo) ListUpcomingUnreminded(
	_ context.Context,
	from, to time.Time,
) ([]meetings.Meeting, error) {
	items := make([]meetings.Meeting, 0)
	for _, meeting := range m.items {
		if meeting.Status == "scheduled" && meeting.ReminderNotifiedAt == nil &&
			!meeting.StartsAt.Before(from) && !meeting.StartsAt.After(to) {
			items = append(items, meeting)
		}
	}
	return items, nil
}

func (m *memoryRepo) DeleteForOrganization(_ context.Context, organizationID string) (int64, error) {
	var deleted int64

	for id, meeting := range m.items {
		if meeting.OrganizationID == organizationID {
			delete(m.items, id)
			deleted++
		}
	}

	return deleted, nil
}

// memoryConnRepo is an in-memory meetings.ConnectionRepository for tests.
type memoryConnRepo struct {
	items map[string]meetings.CalendarConnection
}

func newMemoryConnRepo() *memoryConnRepo {
	return &memoryConnRepo{items: map[string]meetings.CalendarConnection{}}
}

func (m *memoryConnRepo) EnsureIndexes(context.Context) error { return nil }

func (m *memoryConnRepo) Upsert(_ context.Context, conn meetings.CalendarConnection) error {
	m.items[conn.OrganizationID+"/"+conn.Provider] = conn

	return nil
}

func (m *memoryConnRepo) Get(
	_ context.Context,
	organizationID, provider string,
) (meetings.CalendarConnection, error) {
	conn, ok := m.items[organizationID+"/"+provider]
	if !ok {
		return meetings.CalendarConnection{}, meetings.ErrNotFound
	}

	return conn, nil
}

func (m *memoryConnRepo) Delete(_ context.Context, organizationID, provider string) error {
	key := organizationID + "/" + provider
	if _, ok := m.items[key]; !ok {
		return meetings.ErrNotFound
	}

	delete(m.items, key)

	return nil
}

func (m *memoryConnRepo) DeleteForOrganization(_ context.Context, organizationID string) (int64, error) {
	var deleted int64

	for key, conn := range m.items {
		if conn.OrganizationID == organizationID {
			delete(m.items, key)
			deleted++
		}
	}

	return deleted, nil
}

// stubEmployees is an in-memory meetings.EmployeeDirectory.
type stubEmployees struct {
	byUserID map[string]employees.Employee
	byID     map[string]employees.Employee
}

func newStubEmployees() *stubEmployees {
	return &stubEmployees{
		byUserID: map[string]employees.Employee{},
		byID:     map[string]employees.Employee{},
	}
}

func (s *stubEmployees) add(employee employees.Employee) {
	s.byID[employee.ID] = employee
	if employee.UserID != "" {
		s.byUserID[employee.UserID] = employee
	}
}

func (s *stubEmployees) GetByUserID(
	_ context.Context,
	_, userID string,
) (employees.Employee, error) {
	employee, ok := s.byUserID[userID]
	if !ok {
		return employees.Employee{}, employees.ErrNotFound
	}

	return employee, nil
}

func (s *stubEmployees) Get(_ context.Context, _, employeeID string) (employees.Employee, error) {
	employee, ok := s.byID[employeeID]
	if !ok {
		return employees.Employee{}, employees.ErrNotFound
	}

	return employee, nil
}

func (s *stubEmployees) List(
	_ context.Context,
	organizationID string,
	_, _ int64,
) ([]employees.Employee, error) {
	items := make([]employees.Employee, 0, len(s.byID))

	for _, employee := range s.byID {
		if employee.OrganizationID == organizationID {
			items = append(items, employee)
		}
	}

	return items, nil
}

// stubCalendar is a stubbed meetings.CalendarClient.
type stubCalendar struct {
	validateHandle string
	validateErr    error
	eventRef       string
	eventErr       error
	events         []meetings.CalendarEvent
	updatedEventID string
	updatedEvent   meetings.CalendarEvent
}

func (s *stubCalendar) UpdateEvent(
	_ context.Context,
	_ string,
	eventID string,
	event meetings.CalendarEvent,
) error {
	s.updatedEventID, s.updatedEvent = eventID, event
	return s.eventErr
}

func (s *stubCalendar) Validate(context.Context, string) (string, error) {
	return s.validateHandle, s.validateErr
}

func (s *stubCalendar) CreateEvent(_ context.Context, _ string, event meetings.CalendarEvent) (string, error) {
	if s.eventErr != nil {
		return "", s.eventErr
	}

	s.events = append(s.events, event)

	return s.eventRef, nil
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

func testEmployee() employees.Employee {
	return employees.Employee{
		ID:                testEmployeeID,
		OrganizationID:    testOrgID,
		UserID:            testEmployeeUser,
		ManagerEmployeeID: testManagerEmpID,
	}
}

func startsAt(t *testing.T) time.Time {
	t.Helper()

	parsed, err := time.Parse(time.RFC3339, testStartsAt)
	if err != nil {
		t.Fatalf("parse test start: %v", err)
	}

	return parsed
}

func createTestMeeting(t *testing.T, svc *meetings.Service, orgID string) meetings.Meeting {
	t.Helper()

	meeting, err := svc.Create(context.Background(), meetings.CreateInput{
		OrganizationID:     orgID,
		Title:              "Manager intro",
		Type:               meetings.TypeManagerIntro,
		OrganizerUserID:    testManagerUser,
		AttendeeEmployeeID: testEmployeeID,
		StartsAt:           startsAt(t),
		DurationMin:        30,
		Location:           "https://meet.example.com/abc",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	return meeting
}

func TestCreateValidatesInput(t *testing.T) {
	t.Parallel()

	svc := meetings.NewService(newMemoryRepo(), nil, nil, nil)
	valid := startsAt(t)

	cases := []struct {
		name string
		in   meetings.CreateInput
	}{
		{name: "missing title", in: meetings.CreateInput{
			OrganizationID: testOrgID, Type: meetings.TypeManagerIntro,
			AttendeeEmployeeID: testEmployeeID, StartsAt: valid,
		}},
		{name: "unknown type", in: meetings.CreateInput{
			OrganizationID: testOrgID, Title: "Sync", Type: "standup",
			AttendeeEmployeeID: testEmployeeID, StartsAt: valid,
		}},
		{name: "missing attendee", in: meetings.CreateInput{
			OrganizationID: testOrgID, Title: "Sync", Type: meetings.TypeTeamIntro, StartsAt: valid,
		}},
		{name: "missing start", in: meetings.CreateInput{
			OrganizationID: testOrgID, Title: "Sync", Type: meetings.TypeTeamIntro,
			AttendeeEmployeeID: testEmployeeID,
		}},
		{name: "duration too short", in: meetings.CreateInput{
			OrganizationID: testOrgID, Title: "Sync", Type: meetings.TypeTeamIntro,
			AttendeeEmployeeID: testEmployeeID, StartsAt: valid, DurationMin: 2,
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if _, err := svc.Create(context.Background(), tc.in); !errors.Is(err, meetings.ErrInvalidInput) {
				t.Fatalf("Create error = %v, want ErrInvalidInput", err)
			}
		})
	}
}

func TestCreateAcceptsEveryMeetingType(t *testing.T) {
	t.Parallel()

	svc := meetings.NewService(newMemoryRepo(), nil, nil, nil)

	for _, meetingType := range []string{
		meetings.TypeManagerIntro,
		meetings.TypeHROrientation,
		meetings.TypeTeamIntro,
		meetings.TypeBuddyCheckin,
		meetings.TypeArchitectureWalkthrough,
		meetings.TypeRoleCoaching,
		meetings.TypeFirstWeekReview,
	} {
		meeting, err := svc.Create(context.Background(), meetings.CreateInput{
			OrganizationID:     testOrgID,
			Title:              "Onboarding touchpoint",
			Type:               meetingType,
			AttendeeEmployeeID: testEmployeeID,
			StartsAt:           startsAt(t),
		})
		if err != nil {
			t.Fatalf("Create %s: %v", meetingType, err)
		}

		if meeting.Status != "scheduled" {
			t.Fatalf("status = %q, want scheduled", meeting.Status)
		}

		if meeting.DurationMin != 30 {
			t.Fatalf("duration = %d, want the 30-minute default", meeting.DurationMin)
		}
	}
}

func TestMeetingLifecycleCompleteAndCancel(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	svc := meetings.NewService(repo, nil, nil, nil)

	meeting := createTestMeeting(t, svc, testOrgID)

	completed, err := svc.Complete(context.Background(), meetings.CompleteInput{
		OrganizationID: testOrgID,
		MeetingID:      meeting.ID,
		NotesLink:      "https://notes.example.com/1",
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	if completed.Status != "completed" || completed.CompletedAt == nil {
		t.Fatalf("completed = %+v, want completed with timestamp", completed)
	}

	if completed.NotesLink != "https://notes.example.com/1" {
		t.Fatalf("notes link = %q", completed.NotesLink)
	}

	// Terminal meetings reject further transitions.
	if _, err := svc.Complete(context.Background(), meetings.CompleteInput{
		OrganizationID: testOrgID,
		MeetingID:      meeting.ID,
	}); !errors.Is(err, meetings.ErrInvalidState) {
		t.Fatalf("second Complete error = %v, want ErrInvalidState", err)
	}

	if _, err := svc.Cancel(context.Background(), testOrgID, meeting.ID); !errors.Is(err, meetings.ErrInvalidState) {
		t.Fatalf("Cancel after Complete error = %v, want ErrInvalidState", err)
	}

	other := createTestMeeting(t, svc, testOrgID)

	cancelled, err := svc.Cancel(context.Background(), testOrgID, other.ID)
	if err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	if cancelled.Status != "cancelled" {
		t.Fatalf("status = %q, want cancelled", cancelled.Status)
	}

	third := createTestMeeting(t, svc, testOrgID)

	noShow, err := svc.Complete(context.Background(), meetings.CompleteInput{
		OrganizationID: testOrgID,
		MeetingID:      third.ID,
		NoShow:         true,
	})
	if err != nil {
		t.Fatalf("Complete no-show: %v", err)
	}

	if noShow.Status != "no_show" {
		t.Fatalf("status = %q, want no_show", noShow.Status)
	}
}

func TestTenantIsolation(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	svc := meetings.NewService(repo, nil, nil, nil)
	meeting := createTestMeeting(t, svc, testOrgID)

	// Cross-tenant reads and transitions never find the meeting.
	if _, err := svc.Cancel(context.Background(), otherOrgID, meeting.ID); !errors.Is(err, meetings.ErrNotFound) {
		t.Fatalf("cross-tenant Cancel error = %v, want ErrNotFound", err)
	}

	if _, err := svc.Complete(context.Background(), meetings.CompleteInput{
		OrganizationID: otherOrgID,
		MeetingID:      meeting.ID,
	}); !errors.Is(err, meetings.ErrNotFound) {
		t.Fatalf("cross-tenant Complete error = %v, want ErrNotFound", err)
	}

	items, err := svc.ListForUser(context.Background(), otherOrgID, testManagerUser, "")
	if err != nil {
		t.Fatalf("ListForUser: %v", err)
	}

	if len(items) != 0 {
		t.Fatalf("cross-tenant list returned %d meetings, want 0", len(items))
	}

	if repo.items[meeting.ID].Status != "scheduled" {
		t.Fatalf("cross-tenant attempts mutated the meeting: %+v", repo.items[meeting.ID])
	}
}

func TestListMineAndCancelMine(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	directory := newStubEmployees()
	directory.add(testEmployee())
	directory.add(employees.Employee{ID: "emp-2", OrganizationID: testOrgID, UserID: "user-2"})

	svc := meetings.NewService(repo, nil, directory, nil)
	meeting := createTestMeeting(t, svc, testOrgID)

	items, err := svc.ListMine(context.Background(), testOrgID, testEmployeeUser)
	if err != nil {
		t.Fatalf("ListMine: %v", err)
	}

	if len(items) != 1 || items[0].ID != meeting.ID {
		t.Fatalf("ListMine = %+v, want the one meeting", items)
	}

	// Another employee cannot cancel it.
	if _, err := svc.CancelMine(context.Background(), testOrgID, "user-2", meeting.ID); !errors.Is(err, meetings.ErrForbidden) {
		t.Fatalf("CancelMine error = %v, want ErrForbidden", err)
	}

	cancelled, err := svc.CancelMine(context.Background(), testOrgID, testEmployeeUser, meeting.ID)
	if err != nil {
		t.Fatalf("CancelMine: %v", err)
	}

	if cancelled.Status != "cancelled" {
		t.Fatalf("status = %q, want cancelled", cancelled.Status)
	}
}

func TestListForUserScopesToManagerTeam(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	directory := newStubEmployees()
	directory.add(testEmployee())
	directory.add(employees.Employee{
		ID: testManagerEmpID, OrganizationID: testOrgID, UserID: testManagerUser,
	})
	// An employee reporting to a different manager.
	directory.add(employees.Employee{
		ID: "emp-3", OrganizationID: testOrgID, UserID: "user-3", ManagerEmployeeID: "emp-other-manager",
	})

	svc := meetings.NewService(repo, nil, directory, nil)

	createTestMeeting(t, svc, testOrgID) // attendee: testEmployeeID (in the manager's team)

	if _, err := svc.Create(context.Background(), meetings.CreateInput{
		OrganizationID:     testOrgID,
		Title:              "Other team sync",
		Type:               meetings.TypeTeamIntro,
		AttendeeEmployeeID: "emp-3",
		StartsAt:           startsAt(t),
	}); err != nil {
		t.Fatalf("Create other-team meeting: %v", err)
	}

	teamItems, err := svc.ListForUser(context.Background(), testOrgID, testManagerUser, "")
	if err != nil {
		t.Fatalf("ListForUser: %v", err)
	}

	if len(teamItems) != 1 || teamItems[0].AttendeeEmployeeID != testEmployeeID {
		t.Fatalf("team list = %+v, want only the direct report's meeting", teamItems)
	}

	// A caller with no employee record (HR admin) sees the whole org.
	allItems, err := svc.ListForUser(context.Background(), testOrgID, "user-hr", "")
	if err != nil {
		t.Fatalf("ListForUser hr: %v", err)
	}

	if len(allItems) != 2 {
		t.Fatalf("org-wide list = %d meetings, want 2", len(allItems))
	}

	// Status filter applies.
	scheduled, err := svc.ListForUser(context.Background(), testOrgID, "user-hr", "scheduled")
	if err != nil {
		t.Fatalf("ListForUser filtered: %v", err)
	}

	if len(scheduled) != 2 {
		t.Fatalf("filtered list = %d, want 2", len(scheduled))
	}

	if _, err := svc.ListForUser(context.Background(), testOrgID, "user-hr", "bogus"); !errors.Is(err, meetings.ErrInvalidInput) {
		t.Fatalf("status filter error = %v, want ErrInvalidInput", err)
	}
}

func TestCreateFromStepSchedulesMeeting(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	directory := newStubEmployees()
	directory.add(testEmployee())
	directory.add(employees.Employee{
		ID: testManagerEmpID, OrganizationID: testOrgID, UserID: testManagerUser,
	})

	svc := meetings.NewService(repo, nil, directory, nil)

	err := svc.CreateFromStep(
		context.Background(),
		testOrgID,
		testEmployeeID,
		"", // empty type defaults to manager_intro
		"Meet your manager",
		testStartsAt,
		45,
		"Room 4",
	)
	if err != nil {
		t.Fatalf("CreateFromStep: %v", err)
	}

	items, err := repo.ListByAttendee(context.Background(), testOrgID, testEmployeeID)
	if err != nil {
		t.Fatalf("ListByAttendee: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("meetings = %+v, want one", items)
	}

	meeting := items[0]
	if meeting.Type != meetings.TypeManagerIntro {
		t.Errorf("type = %q, want the manager_intro default", meeting.Type)
	}

	if meeting.OrganizerUserID != testManagerUser {
		t.Errorf("organizer = %q, want the employee's manager user", meeting.OrganizerUserID)
	}

	if meeting.DurationMin != 45 || meeting.Location != "Room 4" || meeting.Status != "scheduled" {
		t.Errorf("meeting = %+v", meeting)
	}
}

func TestCreateFromStepRequiresScheduledTime(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	svc := meetings.NewService(repo, nil, newStubEmployees(), nil)

	for _, startsAt := range []string{"", "next Tuesday"} {
		err := svc.CreateFromStep(
			context.Background(),
			testOrgID,
			testEmployeeID,
			meetings.TypeBuddyCheckin,
			"Buddy check-in",
			startsAt,
			30,
			"",
		)
		if !errors.Is(err, meetings.ErrInvalidInput) {
			t.Fatalf("CreateFromStep(%q) error = %v, want ErrInvalidInput", startsAt, err)
		}
	}

	if len(repo.items) != 0 {
		t.Fatalf("invalid submissions created meetings: %+v", repo.items)
	}
}

func TestCreateSyncsCalendarEventBestEffort(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	connections := newMemoryConnRepo()
	calendar := &stubCalendar{eventRef: "gcal-event-1"}
	svc := meetings.NewService(repo, connections, nil, calendar)

	// Without a connection no event is created and the meeting still works.
	meeting := createTestMeeting(t, svc, testOrgID)
	if meeting.CalendarEventRef != "" {
		t.Fatalf("event ref = %q, want empty without a connection", meeting.CalendarEventRef)
	}

	if len(calendar.events) != 0 {
		t.Fatalf("events = %+v, want none without a connection", calendar.events)
	}

	// With a connection the event reference is stored on the meeting.
	if err := connections.Upsert(context.Background(), meetings.CalendarConnection{
		ID:             "conn-1",
		OrganizationID: testOrgID,
		Provider:       meetings.ProviderGoogle,
		Token:          "token",
	}); err != nil {
		t.Fatalf("Upsert connection: %v", err)
	}

	synced := createTestMeeting(t, svc, testOrgID)
	if synced.CalendarEventRef != "gcal-event-1" {
		t.Fatalf("event ref = %q, want gcal-event-1", synced.CalendarEventRef)
	}

	// A provider failure never blocks the meeting.
	calendar.eventErr = errors.New("google api down")

	failed := createTestMeeting(t, svc, testOrgID)
	if failed.CalendarEventRef != "" {
		t.Fatalf("event ref = %q, want empty after provider failure", failed.CalendarEventRef)
	}
}

func TestCalendarConnectionLifecycle(t *testing.T) {
	t.Parallel()

	connections := newMemoryConnRepo()
	calendar := &stubCalendar{validateHandle: "admin@example.com"}
	svc := meetings.NewService(newMemoryRepo(), connections, nil, calendar)

	if _, err := svc.GetCalendarConnection(context.Background(), testOrgID); !errors.Is(err, meetings.ErrNotFound) {
		t.Fatalf("GetCalendarConnection error = %v, want ErrNotFound", err)
	}

	conn, err := svc.ConnectCalendar(context.Background(), testOrgID, testManagerUser, "token-1")
	if err != nil {
		t.Fatalf("ConnectCalendar: %v", err)
	}

	if conn.AccountHandle != "admin@example.com" || !conn.Connected {
		t.Fatalf("connection = %+v", conn)
	}

	// The stored record keeps the raw token for provider calls; the response
	// DTO never carries it (the struct has no token field to assert on).
	stored, err := connections.Get(context.Background(), testOrgID, meetings.ProviderGoogle)
	if err != nil {
		t.Fatalf("Get stored connection: %v", err)
	}

	if stored.Token != "token-1" {
		t.Fatalf("stored token = %q", stored.Token)
	}

	// A rejected credential is never stored.
	calendar.validateErr = meetings.ErrInvalidCredential

	if _, err := svc.ConnectCalendar(context.Background(), otherOrgID, testManagerUser, "bad-token"); !errors.Is(err, meetings.ErrInvalidCredential) {
		t.Fatalf("ConnectCalendar error = %v, want ErrInvalidCredential", err)
	}

	if _, err := connections.Get(context.Background(), otherOrgID, meetings.ProviderGoogle); !errors.Is(err, meetings.ErrNotFound) {
		t.Fatalf("rejected credential was stored: %v", err)
	}

	if err := svc.DisconnectCalendar(context.Background(), testOrgID); err != nil {
		t.Fatalf("DisconnectCalendar: %v", err)
	}

	if _, err := svc.GetCalendarConnection(context.Background(), testOrgID); !errors.Is(err, meetings.ErrNotFound) {
		t.Fatalf("GetCalendarConnection after disconnect error = %v, want ErrNotFound", err)
	}
}

// withPrincipal stores a principal on the request context and routes the
// request through the chi URL param machinery.
func withPrincipal(req *http.Request, orgID, userID string) *http.Request {
	ctx := security.WithPrincipal(req.Context(), security.Principal{
		UserID:         userID,
		OrganizationID: orgID,
	})

	return req.WithContext(ctx)
}

func newTestHandler() (*meetings.Handler, *recordingAuditRepo, *memoryRepo) {
	repo := newMemoryRepo()
	auditRepo := &recordingAuditRepo{}
	directory := newStubEmployees()
	directory.add(testEmployee())
	svc := meetings.NewService(repo, newMemoryConnRepo(), directory, nil)

	return meetings.NewHandler(svc, audit.NewService(auditRepo)), auditRepo, repo
}

func TestHandlerRequiresPrincipal(t *testing.T) {
	t.Parallel()

	handler, _, _ := newTestHandler()

	recorder := httptest.NewRecorder()
	handler.HandleListMine(recorder, httptest.NewRequest(http.MethodGet, "/me/meetings", nil))

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", recorder.Code)
	}
}

func TestHandlerCreateListAndCompleteFlow(t *testing.T) {
	t.Parallel()

	handler, auditRepo, _ := newTestHandler()

	router := chi.NewRouter()
	router.Post("/meetings", handler.HandleCreate)
	router.Get("/me/meetings", handler.HandleListMine)
	router.Post("/meetings/{meetingID}/complete", handler.HandleComplete)

	body := strings.NewReader(`{
		"title": "First-week review",
		"type": "first_week_review",
		"attendeeEmployeeId": "` + testEmployeeID + `",
		"startsAt": "` + testStartsAt + `",
		"durationMin": 30,
		"location": "https://meet.example.com/review"
	}`)

	createReq := withPrincipal(httptest.NewRequest(http.MethodPost, "/meetings", body), testOrgID, testManagerUser)
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	router.ServeHTTP(createRec, createReq)

	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201: %s", createRec.Code, createRec.Body.String())
	}

	listRec := httptest.NewRecorder()
	router.ServeHTTP(listRec, withPrincipal(
		httptest.NewRequest(http.MethodGet, "/me/meetings", nil), testOrgID, testEmployeeUser,
	))

	if listRec.Code != http.StatusOK || !strings.Contains(listRec.Body.String(), "first_week_review") {
		t.Fatalf("list = %d %s", listRec.Code, listRec.Body.String())
	}

	completeRec := httptest.NewRecorder()
	router.ServeHTTP(completeRec, withPrincipal(
		httptest.NewRequest(http.MethodPost, "/meetings/anything/complete", strings.NewReader(`{}`)),
		testOrgID,
		testManagerUser,
	))

	// The id does not exist, but the route and error mapping must hold.
	if completeRec.Code != http.StatusNotFound {
		t.Fatalf("complete status = %d, want 404", completeRec.Code)
	}

	if len(auditRepo.events) != 1 || auditRepo.events[0].Action != "meeting.created" {
		t.Fatalf("audit events = %+v, want one meeting.created", auditRepo.events)
	}
}

func TestRescheduleUpdatesMeetingAndCalendarEvent(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	connections := newMemoryConnRepo()
	calendar := &stubCalendar{validateHandle: "calendar@example.com", eventRef: "event-1"}
	svc := meetings.NewService(repo, connections, nil, calendar)
	if _, err := svc.ConnectCalendar(context.Background(), testOrgID, testManagerUser, "token"); err != nil {
		t.Fatalf("ConnectCalendar: %v", err)
	}
	created, err := svc.Create(context.Background(), meetings.CreateInput{
		OrganizationID: testOrgID, Title: "Orientation", Type: meetings.TypeHROrientation,
		AttendeeEmployeeID: testEmployeeID, StartsAt: time.Now().UTC().Add(48 * time.Hour), DurationMin: 30,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	newStart := created.StartsAt.Add(24 * time.Hour)
	updated, err := svc.Reschedule(context.Background(), meetings.RescheduleInput{
		OrganizationID: testOrgID, MeetingID: created.ID, StartsAt: newStart,
		DurationMin: 45, Location: "Room 4",
	})
	if err != nil {
		t.Fatalf("Reschedule: %v", err)
	}
	if !updated.StartsAt.Equal(newStart) || updated.DurationMin != 45 || calendar.updatedEventID != "event-1" {
		t.Fatalf("reschedule result=%+v calendarEventID=%q", updated, calendar.updatedEventID)
	}
}
