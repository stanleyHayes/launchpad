package supportsessions_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"

	"launchpad/internal/audit"
	"launchpad/internal/notifications"
	"launchpad/internal/organizations"
	"launchpad/internal/roles"
	"launchpad/internal/supportsessions"
	"launchpad/pkg/security"
)

const testJWTSecret = "test-secret"

type fakeRepo struct {
	sessions map[string]supportsessions.Session
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{sessions: map[string]supportsessions.Session{}}
}

func (f *fakeRepo) EnsureIndexes(context.Context) error { return nil }

func (f *fakeRepo) Create(_ context.Context, session supportsessions.Session) error {
	f.sessions[session.ID] = session

	return nil
}

func (f *fakeRepo) GetByID(_ context.Context, id string) (supportsessions.Session, error) {
	session, ok := f.sessions[id]
	if !ok {
		return supportsessions.Session{}, supportsessions.ErrNotFound
	}

	return session, nil
}

func (f *fakeRepo) Update(_ context.Context, session supportsessions.Session) error {
	if _, ok := f.sessions[session.ID]; !ok {
		return supportsessions.ErrNotFound
	}

	f.sessions[session.ID] = session

	return nil
}

func (f *fakeRepo) ListByOrganization(
	_ context.Context,
	organizationID string,
) ([]supportsessions.Session, error) {
	out := make([]supportsessions.Session, 0)

	for _, session := range f.sessions {
		if session.OrganizationID == organizationID {
			out = append(out, session)
		}
	}

	return out, nil
}

type fakeOrgs struct{ err error }

func (f fakeOrgs) Get(context.Context, string) (organizations.Organization, error) {
	if f.err != nil {
		return organizations.Organization{}, f.err
	}

	return organizations.Organization{ID: "org-1", Name: "Acme"}, nil
}

type fakeMembers struct{ members []organizations.Member }

func (f fakeMembers) ListMembers(context.Context, string) ([]organizations.Member, error) {
	return f.members, nil
}

type fakeNotifier struct{ created []notifications.CreateInput }

func (f *fakeNotifier) Create(
	_ context.Context,
	_ string,
	in notifications.CreateInput,
) (notifications.Notification, error) {
	f.created = append(f.created, in)

	return notifications.Notification{ID: "n-1"}, nil
}

type testDeps struct {
	svc      *supportsessions.Service
	repo     *fakeRepo
	notifier *fakeNotifier
}

func newTestService(members []organizations.Member) testDeps {
	repo := newFakeRepo()
	notifier := &fakeNotifier{}
	svc := supportsessions.NewService(
		repo,
		fakeOrgs{},
		fakeMembers{members: members},
		notifier,
		supportsessions.Config{JWTSecret: testJWTSecret},
	)

	return testDeps{svc: svc, repo: repo, notifier: notifier}
}

func ownerMember(userID string) organizations.Member {
	return organizations.Member{
		Membership: organizations.Membership{
			UserID:    userID,
			RoleCode:  roles.RoleOrganizationOwner,
			Status:    "active",
			CreatedAt: time.Now().UTC(),
		},
	}
}

func createSession(
	t *testing.T,
	svc *supportsessions.Service,
	in supportsessions.CreateInput,
) (supportsessions.Session, string) {
	t.Helper()

	session, token, err := svc.Create(context.Background(), in)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	return session, token
}

func validInput() supportsessions.CreateInput {
	return supportsessions.CreateInput{
		OrganizationID: "org-1",
		AgentUserID:    "agent-1",
		AgentEmail:     "agent@launchpad.example",
		Reason:         "Investigating ticket 12345",
	}
}

func TestCreateRequiresReason(t *testing.T) {
	t.Parallel()

	deps := newTestService(nil)

	for _, reason := range []string{"", "   ", "too short"} {
		in := validInput()
		in.Reason = reason

		if _, _, err := deps.svc.Create(context.Background(), in); !errors.Is(err, supportsessions.ErrInvalidInput) {
			t.Errorf("reason %q: err=%v, want ErrInvalidInput", reason, err)
		}
	}
}

func TestCreateRequiresOrganizationAndAgent(t *testing.T) {
	t.Parallel()

	deps := newTestService(nil)

	in := validInput()
	in.OrganizationID = ""

	if _, _, err := deps.svc.Create(context.Background(), in); !errors.Is(err, supportsessions.ErrInvalidInput) {
		t.Errorf("empty organization: err=%v, want ErrInvalidInput", err)
	}

	in = validInput()
	in.AgentUserID = ""

	if _, _, err := deps.svc.Create(context.Background(), in); !errors.Is(err, supportsessions.ErrInvalidInput) {
		t.Errorf("empty agent: err=%v, want ErrInvalidInput", err)
	}
}

func TestCreateCapsDurationAtTwoHours(t *testing.T) {
	t.Parallel()

	deps := newTestService(nil)

	session, _ := createSession(t, deps.svc, validInput())
	if got := session.ExpiresAt.Sub(session.CreatedAt); got != 2*time.Hour {
		t.Errorf("default duration=%v, want 2h", got)
	}

	in := validInput()
	in.DurationMinutes = 121
	if _, _, err := deps.svc.Create(context.Background(), in); !errors.Is(err, supportsessions.ErrInvalidInput) {
		t.Errorf("121 minutes: err=%v, want ErrInvalidInput", err)
	}

	in.DurationMinutes = -5
	if _, _, err := deps.svc.Create(context.Background(), in); !errors.Is(err, supportsessions.ErrInvalidInput) {
		t.Errorf("negative duration: err=%v, want ErrInvalidInput", err)
	}

	in.DurationMinutes = 30
	session, _ = createSession(t, deps.svc, in)
	if got := session.ExpiresAt.Sub(session.CreatedAt); got != 30*time.Minute {
		t.Errorf("custom duration=%v, want 30m", got)
	}
}

func TestCreateFailsWhenOrganizationMissing(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	svc := supportsessions.NewService(
		repo,
		fakeOrgs{err: organizations.ErrNotFound},
		fakeMembers{},
		&fakeNotifier{},
		supportsessions.Config{JWTSecret: testJWTSecret},
	)

	if _, _, err := svc.Create(context.Background(), validInput()); !errors.Is(err, organizations.ErrNotFound) {
		t.Fatalf("err=%v, want organizations.ErrNotFound", err)
	}
}

func TestCreateIssuesMarkedImpersonationToken(t *testing.T) {
	t.Parallel()

	deps := newTestService(nil)
	session, token := createSession(t, deps.svc, validInput())

	principal, err := security.ParseAccessToken(testJWTSecret, token)
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}

	if !principal.Impersonator {
		t.Error("token is not marked as impersonator")
	}
	if principal.UserID != "agent-1" {
		t.Errorf("subject=%q, want the support agent", principal.UserID)
	}
	if principal.OrganizationID != "org-1" || principal.RoleCode != roles.RoleHRAdmin {
		t.Errorf("tenant context=(%q, %q), want (org-1, hr_admin)", principal.OrganizationID, principal.RoleCode)
	}
	if principal.SessionID != session.ID || principal.ImpersonationSessionID != session.ID {
		t.Errorf("session ids=(%q, %q), want the support session id", principal.SessionID, principal.ImpersonationSessionID)
	}

	claims := &security.Claims{}
	if _, _, err := jwt.NewParser().ParseUnverified(token, claims); err != nil {
		t.Fatalf("parse unverified: %v", err)
	}
	if got := claims.ExpiresAt.Sub(claims.IssuedAt.Time); got != supportsessions.TokenTTL {
		t.Errorf("token ttl=%v, want %v", got, supportsessions.TokenTTL)
	}
}

func TestCreateNotifiesOrganizationOwners(t *testing.T) {
	t.Parallel()

	members := []organizations.Member{
		ownerMember("owner-1"),
		ownerMember("owner-2"),
		{Membership: organizations.Membership{UserID: "member-1", RoleCode: roles.RoleEmployee, Status: "active"}},
	}
	deps := newTestService(members)

	createSession(t, deps.svc, validInput())

	if len(deps.notifier.created) != 2 {
		t.Fatalf("notifications=%d, want 2 (owners only)", len(deps.notifier.created))
	}

	for _, in := range deps.notifier.created {
		if in.UserID != "owner-1" && in.UserID != "owner-2" {
			t.Errorf("notified non-owner %q", in.UserID)
		}
		if !strings.Contains(in.Body, "Investigating ticket 12345") {
			t.Errorf("notification body %q does not carry the reason", in.Body)
		}
	}
}

func TestEndInvalidatesSession(t *testing.T) {
	t.Parallel()

	deps := newTestService(nil)
	session, _ := createSession(t, deps.svc, validInput())

	active, err := deps.svc.Active(context.Background(), session.ID)
	if err != nil || !active {
		t.Fatalf("active=(%v, %v), want (true, nil)", active, err)
	}

	ended, err := deps.svc.End(context.Background(), session.ID, "")
	if err != nil {
		t.Fatalf("end: %v", err)
	}
	if ended.EndedAt == nil || ended.EndReason != supportsessions.EndReasonEndedByAgent {
		t.Errorf("ended session=%+v, want EndedAt set with default reason", ended)
	}

	active, err = deps.svc.Active(context.Background(), session.ID)
	if err != nil || active {
		t.Errorf("after end active=(%v, %v), want (false, nil)", active, err)
	}

	if _, err := deps.svc.End(context.Background(), session.ID, ""); !errors.Is(err, supportsessions.ErrSessionEnded) {
		t.Errorf("second end err=%v, want ErrSessionEnded", err)
	}
}

func TestActiveRejectsExpiredAndUnknownSessions(t *testing.T) {
	t.Parallel()

	deps := newTestService(nil)
	deps.repo.sessions["expired-1"] = supportsessions.Session{
		ID:        "expired-1",
		CreatedAt: time.Now().UTC().Add(-3 * time.Hour),
		ExpiresAt: time.Now().UTC().Add(-time.Hour),
	}

	active, err := deps.svc.Active(context.Background(), "expired-1")
	if err != nil || active {
		t.Errorf("expired active=(%v, %v), want (false, nil)", active, err)
	}

	active, err = deps.svc.Active(context.Background(), "missing")
	if err != nil || active {
		t.Errorf("unknown active=(%v, %v), want (false, nil)", active, err)
	}
}

func TestListRequiresOrganization(t *testing.T) {
	t.Parallel()

	deps := newTestService(nil)
	if _, err := deps.svc.List(context.Background(), ""); !errors.Is(err, supportsessions.ErrInvalidInput) {
		t.Fatalf("err=%v, want ErrInvalidInput", err)
	}
}

type stubChecker struct {
	exists bool
	err    error
}

func (s stubChecker) SessionExists(context.Context, string) (bool, error) {
	return s.exists, s.err
}

func TestChainedSessionChecker(t *testing.T) {
	t.Parallel()

	errStore := errors.New("store down")

	cases := []struct {
		name     string
		checkers []supportsessions.SessionChecker
		exists   bool
		wantErr  bool
	}{
		{"first matches", []supportsessions.SessionChecker{stubChecker{exists: true}, stubChecker{}}, true, false},
		{"second matches", []supportsessions.SessionChecker{stubChecker{}, stubChecker{exists: true}}, true, false},
		{"none match", []supportsessions.SessionChecker{stubChecker{}, stubChecker{}}, false, false},
		{"error then match", []supportsessions.SessionChecker{stubChecker{err: errStore}, stubChecker{exists: true}}, true, false},
		{"error no match", []supportsessions.SessionChecker{stubChecker{err: errStore}, stubChecker{}}, false, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			exists, err := supportsessions.ChainedSessionChecker(tc.checkers...).SessionExists(context.Background(), "s-1")
			if exists != tc.exists || (err != nil) != tc.wantErr {
				t.Errorf("got (%v, %v), want (%v, err=%v)", exists, err, tc.exists, tc.wantErr)
			}
		})
	}
}

func TestHandleCreateRejectsShortReason(t *testing.T) {
	t.Parallel()

	handler := newTestHandler(t)

	req := httptest.NewRequestWithContext(
		principalContext(),
		http.MethodPost,
		"/platform/support-sessions",
		strings.NewReader(`{"organizationId":"org-1","reason":"short"}`),
	)
	recorder := httptest.NewRecorder()
	handler.HandleCreate(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestHandleCreateReturnsTokenAndEndInvalidates(t *testing.T) {
	t.Parallel()

	handler := newTestHandler(t)
	router := chi.NewRouter()
	router.Post("/platform/support-sessions", handler.HandleCreate)
	router.Post("/platform/support-sessions/{sessionID}/end", handler.HandleEnd)

	req := httptest.NewRequestWithContext(
		principalContext(),
		http.MethodPost,
		"/platform/support-sessions",
		strings.NewReader(`{"organizationId":"org-1","reason":"Investigating ticket 12345"}`),
	)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("create status=%d, want %d (body: %s)", recorder.Code, http.StatusCreated, recorder.Body.String())
	}

	body := recorder.Body.String()
	if !strings.Contains(body, `"token"`) || !strings.Contains(body, `"tokenExpiresAt"`) {
		t.Errorf("create response missing token fields: %s", body)
	}

	sessionID := strings.Split(strings.Split(body, `"id":"`)[1], `"`)[0]

	endReq := httptest.NewRequestWithContext(
		principalContext(),
		http.MethodPost,
		"/platform/support-sessions/"+sessionID+"/end",
		nil,
	)
	endRecorder := httptest.NewRecorder()
	router.ServeHTTP(endRecorder, endReq)

	if endRecorder.Code != http.StatusOK {
		t.Fatalf("end status=%d, want %d (body: %s)", endRecorder.Code, http.StatusOK, endRecorder.Body.String())
	}
	if !strings.Contains(endRecorder.Body.String(), `"endReason":"ended_by_agent"`) {
		t.Errorf("end response missing default end reason: %s", endRecorder.Body.String())
	}
}

func TestHandleEndUnknownSession(t *testing.T) {
	t.Parallel()

	handler := newTestHandler(t)
	router := chi.NewRouter()
	router.Post("/platform/support-sessions/{sessionID}/end", handler.HandleEnd)

	req := httptest.NewRequestWithContext(
		principalContext(),
		http.MethodPost,
		"/platform/support-sessions/nope/end",
		nil,
	)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status=%d, want %d", recorder.Code, http.StatusNotFound)
	}
}

type fakeAuditRepo struct{ events []audit.Event }

func (f *fakeAuditRepo) EnsureIndexes(context.Context) error { return nil }

func (f *fakeAuditRepo) Write(_ context.Context, event audit.Event) error {
	f.events = append(f.events, event)

	return nil
}

func (f *fakeAuditRepo) ListByOrganization(context.Context, string, int64) ([]audit.Event, error) {
	return nil, nil
}

func (f *fakeAuditRepo) ListAll(context.Context, int64) ([]audit.Event, error) { return nil, nil }

func (f *fakeAuditRepo) CountByOrganization(context.Context, string) (int64, error) { return 0, nil }

func (f *fakeAuditRepo) DeleteForOrganization(context.Context, string) (int64, error) { return 0, nil }

func principalContext() context.Context {
	return security.WithPrincipal(context.Background(), security.Principal{
		UserID:    "agent-1",
		Email:     "agent@launchpad.example",
		RoleCode:  "platform_admin",
		SessionID: "sess-1",
	})
}

func newTestHandler(t *testing.T) *supportsessions.Handler {
	t.Helper()

	deps := newTestService(nil)

	return supportsessions.NewHandler(deps.svc, audit.NewService(&fakeAuditRepo{}))
}
