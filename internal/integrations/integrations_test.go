package integrations_test

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"testing"

	"launchpad/internal/integrations"
)

const (
	orgA = "org-a"
	orgB = "org-b"
)

// fakeRepo is an in-memory Repository keyed by organization + provider.
type fakeRepo struct {
	connections map[string]integrations.Connection
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{connections: map[string]integrations.Connection{}}
}

func repoKey(organizationID, provider string) string {
	return organizationID + "|" + provider
}

func (f *fakeRepo) EnsureIndexes(context.Context) error { return nil }

func (f *fakeRepo) Upsert(_ context.Context, conn integrations.Connection) error {
	f.connections[repoKey(conn.OrganizationID, conn.Provider)] = conn

	return nil
}

func (f *fakeRepo) Get(_ context.Context, organizationID, provider string) (integrations.Connection, error) {
	conn, ok := f.connections[repoKey(organizationID, provider)]
	if !ok {
		return integrations.Connection{}, integrations.ErrNotFound
	}

	return conn, nil
}

func (f *fakeRepo) List(_ context.Context, organizationID string) ([]integrations.Connection, error) {
	connections := make([]integrations.Connection, 0, len(f.connections))

	for _, conn := range f.connections {
		if conn.OrganizationID == organizationID {
			connections = append(connections, conn)
		}
	}

	sort.Slice(connections, func(i, j int) bool { return connections[i].Provider < connections[j].Provider })

	return connections, nil
}

func (f *fakeRepo) Delete(_ context.Context, organizationID, provider string) error {
	key := repoKey(organizationID, provider)
	if _, ok := f.connections[key]; !ok {
		return integrations.ErrNotFound
	}

	delete(f.connections, key)

	return nil
}

// stubProvider is a scripted providerClient double.
type stubProvider struct {
	handle   string
	err      error
	calls    int
	gotInput integrations.ConnectInput
}

func (s *stubProvider) Validate(_ context.Context, in integrations.ConnectInput) (string, error) {
	s.calls++
	s.gotInput = in

	return s.handle, s.err
}

// recordingAudit captures recorded actions.
type recordingAudit struct {
	actions []string
}

func (f *recordingAudit) Record(
	_ context.Context,
	_ *string,
	_, action, _, _ string,
	_ map[string]any,
) error {
	f.actions = append(f.actions, action)

	return nil
}

func newService(repo *fakeRepo, github, jira *stubProvider) (*integrations.Service, *recordingAudit) {
	audit := &recordingAudit{}

	return integrations.NewService(repo, audit, github, jira), audit
}

func connectGitHub(t *testing.T, svc *integrations.Service) {
	t.Helper()

	_, err := svc.Connect(context.Background(), orgA, "user-1", "github", integrations.ConnectInput{
		Token: "secret-token",
	})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
}

func TestConnectPersistsOnlyAfterSuccessfulValidation(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	github := &stubProvider{handle: "octocat"}
	svc, audit := newService(repo, github, &stubProvider{})

	connectGitHub(t, svc)

	if github.calls != 1 {
		t.Fatalf("expected 1 validation call, got %d", github.calls)
	}

	stored, err := repo.Get(context.Background(), orgA, "github")
	if err != nil {
		t.Fatalf("expected connection to be persisted: %v", err)
	}

	if stored.Status != integrations.StatusConnected || stored.AccountHandle != "octocat" {
		t.Fatalf("unexpected stored connection: %+v", stored)
	}

	if stored.Token != "secret-token" || stored.CreatedBy != "user-1" || stored.LastSyncAt == nil {
		t.Fatalf("unexpected stored connection: %+v", stored)
	}

	if len(audit.actions) != 1 || audit.actions[0] != "integration.connected" {
		t.Fatalf("expected integration.connected audit, got %v", audit.actions)
	}
}

func TestConnectFailedValidationPersistsNothing(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	github := &stubProvider{err: integrations.ErrInvalidCredential}
	svc, audit := newService(repo, github, &stubProvider{})

	_, err := svc.Connect(context.Background(), orgA, "user-1", "github", integrations.ConnectInput{Token: "bad-token"})
	if !errors.Is(err, integrations.ErrInvalidCredential) {
		t.Fatalf("expected ErrInvalidCredential, got %v", err)
	}

	if _, getErr := repo.Get(context.Background(), orgA, "github"); !errors.Is(getErr, integrations.ErrNotFound) {
		t.Fatalf("nothing should be persisted after failed validation, got %v", getErr)
	}

	if len(audit.actions) != 0 {
		t.Fatalf("no audit event expected, got %v", audit.actions)
	}
}

func TestConnectValidatesInputPerProvider(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	jira := &stubProvider{handle: "Ada Lovelace"}
	svc, _ := newService(repo, &stubProvider{}, jira)

	cases := []struct {
		name  string
		input integrations.ConnectInput
	}{
		{name: "missing base url", input: integrations.ConnectInput{Token: "tok", Email: "a@b.co"}},
		{name: "missing email", input: integrations.ConnectInput{Token: "tok", BaseURL: "https://x.atlassian.net"}},
		{
			name:  "non-https base url",
			input: integrations.ConnectInput{Token: "tok", BaseURL: "http://x.atlassian.net", Email: "a@b.co"},
		},
		{name: "missing token", input: integrations.ConnectInput{BaseURL: "https://x.atlassian.net", Email: "a@b.co"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := svc.Connect(context.Background(), orgA, "user-1", "jira", tc.input)
			if !errors.Is(err, integrations.ErrInvalidInput) {
				t.Fatalf("expected ErrInvalidInput, got %v", err)
			}
		})
	}

	if jira.calls != 0 {
		t.Fatalf("provider must not be called for invalid input, got %d calls", jira.calls)
	}
}

func TestListMasksToken(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	svc, _ := newService(repo, &stubProvider{handle: "octocat"}, &stubProvider{})

	connectGitHub(t, svc)

	connections, err := svc.List(context.Background(), orgA)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if len(connections) != 1 {
		t.Fatalf("expected 1 connection, got %d", len(connections))
	}

	raw, err := json.Marshal(connections)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if strings.Contains(string(raw), "secret-token") || strings.Contains(string(raw), "integrationToken") {
		t.Fatalf("token leaked into list response: %s", raw)
	}
}

func TestDisconnectIsOrgScoped(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	svc, _ := newService(repo, &stubProvider{handle: "octocat"}, &stubProvider{})

	connectGitHub(t, svc)

	err := svc.Disconnect(context.Background(), orgB, "user-2", "github")
	if !errors.Is(err, integrations.ErrNotFound) {
		t.Fatalf("tenant B must not delete tenant A's connection, got %v", err)
	}

	if _, err := repo.Get(context.Background(), orgA, "github"); err != nil {
		t.Fatalf("tenant A connection should still exist: %v", err)
	}

	if err := svc.Disconnect(context.Background(), orgA, "user-1", "github"); err != nil {
		t.Fatalf("tenant A disconnect: %v", err)
	}

	if _, err := repo.Get(context.Background(), orgA, "github"); !errors.Is(err, integrations.ErrNotFound) {
		t.Fatalf("expected connection to be deleted, got %v", err)
	}
}

func TestHealthTransitionsConnectedToError(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	github := &stubProvider{handle: "octocat"}
	svc, _ := newService(repo, github, &stubProvider{})

	connectGitHub(t, svc)

	if _, err := svc.Health(context.Background(), orgA, "github"); err != nil {
		t.Fatalf("health with valid credential: %v", err)
	}

	stored, _ := repo.Get(context.Background(), orgA, "github")
	if stored.Status != integrations.StatusConnected || stored.LastError != "" {
		t.Fatalf("expected healthy connection, got %+v", stored)
	}

	// The provider starts rejecting the stored credential.
	github.err = integrations.ErrInvalidCredential

	if _, err := svc.Health(context.Background(), orgA, "github"); !errors.Is(err, integrations.ErrInvalidCredential) {
		t.Fatalf("expected ErrInvalidCredential, got %v", err)
	}

	stored, _ = repo.Get(context.Background(), orgA, "github")
	if stored.Status != integrations.StatusError || stored.LastError == "" || stored.LastSyncAt == nil {
		t.Fatalf("expected errored connection with lastError and lastSyncAt, got %+v", stored)
	}

	if github.gotInput.Token != "secret-token" {
		t.Fatal("health must re-validate with the stored token")
	}
}

func TestUnknownProviderRejected(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	github := &stubProvider{handle: "octocat"}
	svc, _ := newService(repo, github, &stubProvider{})

	_, err := svc.Connect(context.Background(), orgA, "user-1", "gitlab", integrations.ConnectInput{Token: "tok"})
	if !errors.Is(err, integrations.ErrUnknownProvider) {
		t.Fatalf("connect: expected ErrUnknownProvider, got %v", err)
	}

	err = svc.Disconnect(context.Background(), orgA, "user-1", "gitlab")
	if !errors.Is(err, integrations.ErrUnknownProvider) {
		t.Fatalf("disconnect: expected ErrUnknownProvider, got %v", err)
	}

	if _, err := svc.Health(context.Background(), orgA, "gitlab"); !errors.Is(err, integrations.ErrUnknownProvider) {
		t.Fatalf("health: expected ErrUnknownProvider, got %v", err)
	}

	if github.calls != 0 {
		t.Fatalf("no provider call expected for unknown provider, got %d", github.calls)
	}
}

func (f *fakeRepo) DeleteForOrganization(context.Context, string) (int64, error) {
	return 0, nil
}
