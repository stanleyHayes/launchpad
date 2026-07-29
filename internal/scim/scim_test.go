package scim_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"launchpad/internal/scim"
	"launchpad/pkg/security"
)

const (
	orgA = "org-A"
	orgB = "org-B"
)

// memStore is an in-memory, org-scoped SCIM record store.
type memStore struct {
	records []scim.Record
}

func (m *memStore) EnsureIndexes(context.Context) error { return nil }

func (m *memStore) Create(_ context.Context, record scim.Record) error {
	for _, existing := range m.records {
		if existing.OrganizationID == record.OrganizationID && existing.UserName == record.UserName {
			return scim.ErrConflict
		}
	}

	m.records = append(m.records, record)

	return nil
}

func (m *memStore) GetByID(_ context.Context, organizationID, id string) (scim.Record, error) {
	for _, existing := range m.records {
		if existing.OrganizationID == organizationID && existing.ID == id {
			return existing, nil
		}
	}

	return scim.Record{}, scim.ErrNotFound
}

func (m *memStore) GetByUserName(_ context.Context, organizationID, userName string) (scim.Record, error) {
	for _, existing := range m.records {
		if existing.OrganizationID == organizationID && existing.UserName == userName {
			return existing, nil
		}
	}

	return scim.Record{}, scim.ErrNotFound
}

func (m *memStore) List(
	_ context.Context,
	organizationID, filter string,
	offset, limit int,
) ([]scim.Record, int, error) {
	all := make([]scim.Record, 0)

	for _, existing := range m.records {
		if existing.OrganizationID == organizationID && (filter == "" || existing.UserName == filter) {
			all = append(all, existing)
		}
	}

	sort.Slice(all, func(i, j int) bool { return all[i].UserName < all[j].UserName })

	return paginate(all, offset, limit), len(all), nil
}

// paginate mirrors the store's offset/limit slicing for the in-memory fakes.
func paginate[T any](all []T, offset, limit int) []T {
	if limit <= 0 || offset >= len(all) {
		return []T{}
	}

	if offset < 0 {
		offset = 0
	}

	return all[offset:min(offset+limit, len(all))]
}

func (m *memStore) Update(_ context.Context, record scim.Record) error {
	for i, existing := range m.records {
		if existing.OrganizationID == record.OrganizationID && existing.ID == record.ID {
			m.records[i] = record

			return nil
		}
	}

	return scim.ErrNotFound
}

func (m *memStore) Delete(_ context.Context, organizationID, id string) error {
	for i, existing := range m.records {
		if existing.OrganizationID == organizationID && existing.ID == id {
			m.records = append(m.records[:i], m.records[i+1:]...)

			return nil
		}
	}

	return scim.ErrNotFound
}

// memTokenStore maps org<->hash in memory, with per-token expiry.
type memTokenStore struct {
	byOrg   map[string]string
	byHash  map[string]string
	expires map[string]time.Time
}

func newTokenStore() *memTokenStore {
	return &memTokenStore{byOrg: map[string]string{}, byHash: map[string]string{}, expires: map[string]time.Time{}}
}

func (t *memTokenStore) EnsureIndexes(context.Context) error { return nil }

func (t *memTokenStore) StoreToken(_ context.Context, organizationID, tokenHash string, _, expiresAt time.Time) error {
	if old, ok := t.byOrg[organizationID]; ok {
		delete(t.byHash, old)
		delete(t.expires, old)
	}

	t.byOrg[organizationID] = tokenHash
	t.byHash[tokenHash] = organizationID
	t.expires[tokenHash] = expiresAt

	return nil
}

func (t *memTokenStore) ResolveOrganization(_ context.Context, tokenHash string) (string, time.Time, error) {
	if organizationID, ok := t.byHash[tokenHash]; ok {
		return organizationID, t.expires[tokenHash], nil
	}

	return "", time.Time{}, scim.ErrUnauthorized
}

// fakeProvisioner models accounts (by email) and per-org memberships so the
// service's cross-tenant safety logic can be exercised.
type fakeProvisioner struct {
	accounts   map[string]string // email -> userID
	members    map[string]bool   // "org/userID" -> active
	lastActive *bool
}

func newProvisioner() *fakeProvisioner {
	return &fakeProvisioner{accounts: map[string]string{}, members: map[string]bool{}, lastActive: nil}
}

func memberKey(organizationID, userID string) string { return organizationID + "/" + userID }

func (p *fakeProvisioner) CreateAccount(_ context.Context, email, _ string) (string, error) {
	if _, exists := p.accounts[email]; exists {
		return "", scim.ErrAccountExists
	}

	id := "user-" + email
	p.accounts[email] = id

	return id, nil
}

func (p *fakeProvisioner) FindOrgMember(_ context.Context, organizationID, email string) (string, error) {
	id, ok := p.accounts[email]
	if !ok {
		return "", scim.ErrNotOrgMember
	}

	if _, member := p.members[memberKey(organizationID, id)]; !member {
		return "", scim.ErrNotOrgMember
	}

	return id, nil
}

func (p *fakeProvisioner) AddMember(_ context.Context, organizationID, userID string) error {
	p.members[memberKey(organizationID, userID)] = true

	return nil
}

func (p *fakeProvisioner) SetActive(_ context.Context, organizationID, userID string, active bool) error {
	p.members[memberKey(organizationID, userID)] = active
	value := active
	p.lastActive = &value

	return nil
}

// noopAudit satisfies scim.AuditRecorder without recording anything.
type noopAudit struct{}

func (noopAudit) Record(context.Context, *string, string, string, string, string, map[string]any) error {
	return nil
}

func newSvc(store scim.Store, prov scim.Provisioner) *scim.Service {
	return scim.NewService(store, newTokenStore(), prov, &memGroupStore{}, noopAudit{})
}

func newService() (*scim.Service, *memStore, *fakeProvisioner) {
	store := &memStore{}
	prov := newProvisioner()

	return newSvc(store, prov), store, prov
}

func createInput(userName, email string) scim.CreateUserInput {
	return scim.CreateUserInput{
		UserName:    userName,
		Email:       email,
		DisplayName: userName,
		ExternalID:  "ext-1",
		GivenName:   "Given",
		FamilyName:  "Family",
		Active:      true,
	}
}

func TestTokenRoundTrip(t *testing.T) {
	t.Parallel()

	svc := newSvc(&memStore{}, newProvisioner())

	token, expiresAt, err := svc.GenerateToken(context.Background(), orgA, "actor")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	if !strings.HasPrefix(token, "scim_") {
		t.Fatalf("token missing prefix: %q", token)
	}

	if want := time.Now().UTC().Add(scim.TokenTTL); expiresAt.Before(want.Add(-time.Minute)) || expiresAt.After(want) {
		t.Fatalf("expiresAt = %v, want ~%v", expiresAt, want)
	}

	resolved, err := svc.ResolveOrganization(context.Background(), token)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	if resolved != orgA {
		t.Fatalf("resolved = %q, want %q", resolved, orgA)
	}
}

func TestResolveRejectsBadToken(t *testing.T) {
	t.Parallel()

	svc := newSvc(&memStore{}, newProvisioner())

	if _, err := svc.ResolveOrganization(context.Background(), ""); !errors.Is(err, scim.ErrUnauthorized) {
		t.Fatalf("empty token err = %v, want ErrUnauthorized", err)
	}

	if _, err := svc.ResolveOrganization(context.Background(), "scim_bogus"); !errors.Is(err, scim.ErrUnauthorized) {
		t.Fatalf("unknown token err = %v, want ErrUnauthorized", err)
	}
}

func TestResolveRejectsExpiredToken(t *testing.T) {
	t.Parallel()

	tokens := newTokenStore()
	svc := scim.NewService(&memStore{}, tokens, newProvisioner(), &memGroupStore{}, noopAudit{})
	ctx := context.Background()

	raw := "scim_expired"
	expiresAt := time.Now().UTC().Add(-time.Hour)

	if err := tokens.StoreToken(ctx, orgA, security.HashToken(raw), expiresAt.Add(-scim.TokenTTL), expiresAt); err != nil {
		t.Fatalf("store token: %v", err)
	}

	// Expired tokens get the same error as unknown ones (no expiry oracle).
	if _, err := svc.ResolveOrganization(ctx, raw); !errors.Is(err, scim.ErrUnauthorized) {
		t.Fatalf("expired token err = %v, want ErrUnauthorized", err)
	}
}

func TestResolveAllowsLegacyTokenWithoutExpiry(t *testing.T) {
	t.Parallel()

	tokens := newTokenStore()
	svc := scim.NewService(&memStore{}, tokens, newProvisioner(), &memGroupStore{}, noopAudit{})
	ctx := context.Background()

	raw := "scim_legacy"

	// Tokens issued before expiry existed carry a zero expiry and stay valid
	// until rotated.
	if err := tokens.StoreToken(ctx, orgA, security.HashToken(raw), time.Time{}, time.Time{}); err != nil {
		t.Fatalf("store token: %v", err)
	}

	resolved, err := svc.ResolveOrganization(ctx, raw)
	if err != nil {
		t.Fatalf("resolve legacy token: %v", err)
	}

	if resolved != orgA {
		t.Fatalf("resolved = %q, want %q", resolved, orgA)
	}
}

func TestCreateUserProvisions(t *testing.T) {
	t.Parallel()

	svc, store, prov := newService()

	record, err := svc.CreateUser(context.Background(), orgA, createInput("jane@co.com", "jane@co.com"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if record.UserID != "user-jane@co.com" || !record.Active {
		t.Fatalf("unexpected record: %+v", record)
	}

	if !prov.members[memberKey(orgA, record.UserID)] {
		t.Fatalf("expected user provisioned as an active member")
	}

	if len(store.records) != 1 {
		t.Fatalf("expected one stored record, got %d", len(store.records))
	}
}

// TestCreateUserRejectsCrossTenantGraft is the regression test for the
// account-grafting vulnerability: org B must not be able to annex an existing
// account that belongs only to org A by provisioning its email.
func TestCreateUserRejectsCrossTenantGraft(t *testing.T) {
	t.Parallel()

	store := &memStore{}
	prov := newProvisioner()
	svc := newSvc(store, prov)
	ctx := context.Background()

	// Alice already exists as a member of org A only.
	if _, err := svc.CreateUser(ctx, orgA, createInput("alice@corp-a.com", "alice@corp-a.com")); err != nil {
		t.Fatalf("seed org A user: %v", err)
	}

	// Org B tries to provision the same email.
	_, err := svc.CreateUser(ctx, orgB, createInput("alice@corp-a.com", "alice@corp-a.com"))
	if !errors.Is(err, scim.ErrConflict) {
		t.Fatalf("cross-tenant provision err = %v, want ErrConflict", err)
	}

	if prov.members[memberKey(orgB, "user-alice@corp-a.com")] {
		t.Fatalf("org B must not have been granted membership to org A's account")
	}
}

// TestCreateUserReprovisionsSameOrgMember allows the standard IdP
// deprovision→reprovision cycle within the same organization.
func TestCreateUserReprovisionsSameOrgMember(t *testing.T) {
	t.Parallel()

	store := &memStore{}
	prov := newProvisioner()
	svc := newSvc(store, prov)
	ctx := context.Background()

	record, err := svc.CreateUser(ctx, orgA, createInput("carol@co.com", "carol@co.com"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Deprovision (DELETE removes the SCIM record and suspends membership).
	if err := svc.DeleteUser(ctx, orgA, record.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	// Re-provisioning the same email in the same org must succeed and reactivate.
	reprovisioned, err := svc.CreateUser(ctx, orgA, createInput("carol@co.com", "carol@co.com"))
	if err != nil {
		t.Fatalf("reprovision: %v", err)
	}

	if reprovisioned.UserID != record.UserID {
		t.Fatalf("reprovision reused a different user id: %q vs %q", reprovisioned.UserID, record.UserID)
	}

	if !prov.members[memberKey(orgA, record.UserID)] {
		t.Fatalf("expected membership reactivated")
	}
}

func TestCreateUserConflict(t *testing.T) {
	t.Parallel()

	svc, _, _ := newService()
	ctx := context.Background()

	if _, err := svc.CreateUser(ctx, orgA, createInput("dup@co.com", "dup@co.com")); err != nil {
		t.Fatalf("first create: %v", err)
	}

	_, err := svc.CreateUser(ctx, orgA, createInput("dup@co.com", "dup@co.com"))
	if !errors.Is(err, scim.ErrConflict) {
		t.Fatalf("second create err = %v, want ErrConflict", err)
	}
}

func TestCreateUserInvalid(t *testing.T) {
	t.Parallel()

	svc, _, _ := newService()

	if _, err := svc.CreateUser(context.Background(), orgA, createInput("", "")); !errors.Is(err, scim.ErrInvalid) {
		t.Fatalf("err = %v, want ErrInvalid", err)
	}
}

func TestSetActiveDeprovisions(t *testing.T) {
	t.Parallel()

	svc, _, prov := newService()
	ctx := context.Background()

	record, err := svc.CreateUser(ctx, orgA, createInput("bob@co.com", "bob@co.com"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	updated, err := svc.SetActive(ctx, orgA, record.ID, false)
	if err != nil {
		t.Fatalf("set active: %v", err)
	}

	if updated.Active {
		t.Fatalf("expected record inactive")
	}

	if prov.lastActive == nil || *prov.lastActive {
		t.Fatalf("expected provisioner deactivated the user")
	}
}

func TestDeleteDeprovisions(t *testing.T) {
	t.Parallel()

	svc, store, prov := newService()
	ctx := context.Background()

	record, err := svc.CreateUser(ctx, orgA, createInput("gone@co.com", "gone@co.com"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := svc.DeleteUser(ctx, orgA, record.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if len(store.records) != 0 {
		t.Fatalf("expected record removed")
	}

	if prov.lastActive == nil || *prov.lastActive {
		t.Fatalf("expected provisioner deactivated the user on delete")
	}
}

func newRouter(h *scim.Handler) http.Handler {
	router := chi.NewRouter()
	router.Group(func(scimRoutes chi.Router) {
		scimRoutes.Use(h.Authenticate)
		scimRoutes.Post("/scim/v2/Users", h.HandleCreateUser)
		scimRoutes.Get("/scim/v2/Users/{userID}", h.HandleGetUser)
	})

	return router
}

func TestHTTPRejectsMissingToken(t *testing.T) {
	t.Parallel()

	svc := newSvc(&memStore{}, newProvisioner())
	router := newRouter(scim.NewHandler(svc))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/scim/v2/Users/x", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestHTTPCreateAndTenantIsolation(t *testing.T) {
	t.Parallel()

	svc := newSvc(&memStore{}, newProvisioner())
	router := newRouter(scim.NewHandler(svc))
	ctx := context.Background()

	tokenA, _, err := svc.GenerateToken(ctx, orgA, "actor")
	if err != nil {
		t.Fatalf("token A: %v", err)
	}

	tokenB, _, err := svc.GenerateToken(ctx, orgB, "actor")
	if err != nil {
		t.Fatalf("token B: %v", err)
	}

	created := postUser(t, router, tokenA, `{"userName":"amy@co.com","emails":[{"value":"amy@co.com","primary":true}]}`)
	if created.UserName != "amy@co.com" || !created.Active {
		t.Fatalf("unexpected created user: %+v", created)
	}

	// org A can read its own user.
	if code := getUser(router, tokenA, created.ID); code != http.StatusOK {
		t.Fatalf("org A get own user status = %d, want 200", code)
	}

	// org B must NOT see org A's user.
	if code := getUser(router, tokenB, created.ID); code != http.StatusNotFound {
		t.Fatalf("cross-tenant get status = %d, want 404", code)
	}
}

func postUser(t *testing.T, router http.Handler, token, body string) scim.User {
	t.Helper()

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/scim/v2/Users", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201 (body: %s)", rec.Code, rec.Body.String())
	}

	var user scim.User
	if err := json.Unmarshal(rec.Body.Bytes(), &user); err != nil {
		t.Fatalf("decode created user: %v", err)
	}

	return user
}

func getUser(router http.Handler, token, id string) int {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/scim/v2/Users/"+id, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	return rec.Code
}

// flakyStore fails Create until told to succeed, to exercise compensation.
type flakyStore struct {
	scim.Store

	failCreate bool
}

func (f *flakyStore) Create(ctx context.Context, record scim.Record) error {
	if f.failCreate {
		return errors.New("record store unavailable")
	}

	if err := f.Store.Create(ctx, record); err != nil {
		return fmt.Errorf("create scim record: %w", err)
	}

	return nil
}

func TestCreateUserCompensatesWhenRecordStoreFails(t *testing.T) {
	t.Parallel()

	store := &flakyStore{Store: &memStore{}, failCreate: true}
	prov := newProvisioner()
	svc := newSvc(store, prov)
	ctx := context.Background()

	if _, err := svc.CreateUser(ctx, orgA, createInput("jane@co.com", "jane@co.com")); err == nil {
		t.Fatal("expected record store failure")
	}

	userID := prov.accounts["jane@co.com"]
	if userID == "" {
		t.Fatal("account should have been provisioned before the failure")
	}

	if prov.members[memberKey(orgA, userID)] {
		t.Fatal("failed create must not leave an active membership without a SCIM record")
	}

	// A retry reuses the account, reactivates the membership, and succeeds.
	store.failCreate = false

	record, err := svc.CreateUser(ctx, orgA, createInput("jane@co.com", "jane@co.com"))
	if err != nil {
		t.Fatalf("retry: %v", err)
	}

	if !record.Active || !prov.members[memberKey(orgA, userID)] {
		t.Fatalf("retry should reactivate the membership, got record %+v", record)
	}
}

func (m *memStore) DeleteForOrganization(context.Context, string) (int64, error) {
	return 0, nil
}

func (t *memTokenStore) DeleteForOrganization(context.Context, string) (int64, error) {
	return 0, nil
}
