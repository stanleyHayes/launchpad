package scim_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"launchpad/internal/scim"
)

// memGroupStore is an in-memory, org-scoped SCIM group store.
type memGroupStore struct {
	groups []scim.GroupRecord
}

func (m *memGroupStore) EnsureIndexes(context.Context) error { return nil }

func (m *memGroupStore) Create(_ context.Context, group scim.GroupRecord) error {
	for _, existing := range m.groups {
		if existing.OrganizationID == group.OrganizationID && existing.DisplayName == group.DisplayName {
			return scim.ErrConflict
		}
	}

	m.groups = append(m.groups, group)

	return nil
}

func (m *memGroupStore) GetByID(_ context.Context, organizationID, id string) (scim.GroupRecord, error) {
	for _, existing := range m.groups {
		if existing.OrganizationID == organizationID && existing.ID == id {
			return existing, nil
		}
	}

	return scim.GroupRecord{}, scim.ErrNotFound
}

func (m *memGroupStore) GetByDisplayName(_ context.Context, organizationID, name string) (scim.GroupRecord, error) {
	for _, existing := range m.groups {
		if existing.OrganizationID == organizationID && existing.DisplayName == name {
			return existing, nil
		}
	}

	return scim.GroupRecord{}, scim.ErrNotFound
}

func (m *memGroupStore) List(
	_ context.Context,
	organizationID, filter string,
	offset, limit int,
) ([]scim.GroupRecord, int, error) {
	all := make([]scim.GroupRecord, 0)

	for _, existing := range m.groups {
		if existing.OrganizationID == organizationID && (filter == "" || existing.DisplayName == filter) {
			all = append(all, existing)
		}
	}

	sort.Slice(all, func(i, j int) bool { return all[i].DisplayName < all[j].DisplayName })

	return paginate(all, offset, limit), len(all), nil
}

func (m *memGroupStore) Update(_ context.Context, group scim.GroupRecord) error {
	for _, existing := range m.groups {
		if existing.OrganizationID == group.OrganizationID &&
			existing.ID != group.ID && existing.DisplayName == group.DisplayName {
			return scim.ErrConflict
		}
	}

	for i, existing := range m.groups {
		if existing.OrganizationID == group.OrganizationID && existing.ID == group.ID {
			m.groups[i] = group

			return nil
		}
	}

	return scim.ErrNotFound
}

func (m *memGroupStore) Delete(_ context.Context, organizationID, id string) error {
	for i, existing := range m.groups {
		if existing.OrganizationID == organizationID && existing.ID == id {
			m.groups = append(m.groups[:i], m.groups[i+1:]...)

			return nil
		}
	}

	return scim.ErrNotFound
}

func newGroupSvc() *scim.Service {
	return scim.NewService(&memStore{}, newTokenStore(), newProvisioner(), &memGroupStore{}, noopAudit{})
}

// seedUser provisions a SCIM user and returns its resource id (used as a member value).
func seedUser(t *testing.T, svc *scim.Service, org, userName string) string {
	t.Helper()

	record, err := svc.CreateUser(context.Background(), org, createInput(userName, userName))
	if err != nil {
		t.Fatalf("seed user %s: %v", userName, err)
	}

	return record.ID
}

func TestCreateGroupKeepsValidMembersDropsUnknown(t *testing.T) {
	t.Parallel()

	svc := newGroupSvc()
	ctx := context.Background()
	id1 := seedUser(t, svc, orgA, "u1@co.com")
	id2 := seedUser(t, svc, orgA, "u2@co.com")

	group, err := svc.CreateGroup(ctx, orgA, scim.GroupInput{
		DisplayName: "Engineering",
		ExternalID:  "grp-1",
		MemberIDs:   []string{id1, id2, "bogus-id", id1},
	})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}

	if group.DisplayName != "Engineering" || len(group.Members) != 2 {
		t.Fatalf("unexpected group: %+v", group)
	}

	if !slices.Contains(group.Members, id1) || !slices.Contains(group.Members, id2) {
		t.Fatalf("expected members %s,%s got %v", id1, id2, group.Members)
	}
}

func TestCreateGroupConflict(t *testing.T) {
	t.Parallel()

	svc := newGroupSvc()
	ctx := context.Background()

	if _, err := svc.CreateGroup(ctx, orgA, scim.GroupInput{DisplayName: "Eng"}); err != nil {
		t.Fatalf("first create: %v", err)
	}

	if _, err := svc.CreateGroup(ctx, orgA, scim.GroupInput{DisplayName: "Eng"}); !errors.Is(err, scim.ErrConflict) {
		t.Fatalf("duplicate create err = %v, want ErrConflict", err)
	}
}

func TestCreateGroupRequiresDisplayName(t *testing.T) {
	t.Parallel()

	svc := newGroupSvc()

	_, err := svc.CreateGroup(context.Background(), orgA, scim.GroupInput{DisplayName: "  "})
	if !errors.Is(err, scim.ErrInvalid) {
		t.Fatalf("blank displayName err = %v, want ErrInvalid", err)
	}
}

func TestPatchGroupAddRemoveAndRename(t *testing.T) {
	t.Parallel()

	svc := newGroupSvc()
	ctx := context.Background()
	id1 := seedUser(t, svc, orgA, "u1@co.com")
	id2 := seedUser(t, svc, orgA, "u2@co.com")

	group, err := svc.CreateGroup(ctx, orgA, scim.GroupInput{DisplayName: "Eng", MemberIDs: []string{id1}})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	added, err := svc.PatchGroup(ctx, orgA, group.ID, []scim.GroupPatchChange{
		{DisplayName: nil, MemberOp: scim.GroupMembersAdd, MemberIDs: []string{id2}},
	})
	if err != nil {
		t.Fatalf("patch add: %v", err)
	}

	if len(added.Members) != 2 {
		t.Fatalf("after add want 2 members, got %v", added.Members)
	}

	name := "Engineering"

	patched, err := svc.PatchGroup(ctx, orgA, group.ID, []scim.GroupPatchChange{
		{DisplayName: &name, MemberOp: scim.GroupMembersRemove, MemberIDs: []string{id1}},
	})
	if err != nil {
		t.Fatalf("patch remove+rename: %v", err)
	}

	if patched.DisplayName != "Engineering" || len(patched.Members) != 1 || patched.Members[0] != id2 {
		t.Fatalf("unexpected after remove+rename: %+v", patched)
	}
}

func TestReplaceGroup(t *testing.T) {
	t.Parallel()

	svc := newGroupSvc()
	ctx := context.Background()
	id1 := seedUser(t, svc, orgA, "u1@co.com")
	id2 := seedUser(t, svc, orgA, "u2@co.com")

	group, err := svc.CreateGroup(ctx, orgA, scim.GroupInput{DisplayName: "Eng", MemberIDs: []string{id1}})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	input := scim.GroupInput{DisplayName: "Platform", MemberIDs: []string{id2}}

	replaced, err := svc.ReplaceGroup(ctx, orgA, group.ID, input)
	if err != nil {
		t.Fatalf("replace: %v", err)
	}

	if replaced.DisplayName != "Platform" || len(replaced.Members) != 1 || replaced.Members[0] != id2 {
		t.Fatalf("unexpected replace: %+v", replaced)
	}
}

func TestDeleteGroup(t *testing.T) {
	t.Parallel()

	svc := newGroupSvc()
	ctx := context.Background()

	group, err := svc.CreateGroup(ctx, orgA, scim.GroupInput{DisplayName: "Eng"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := svc.DeleteGroup(ctx, orgA, group.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if _, err := svc.GetGroup(ctx, orgA, group.ID); !errors.Is(err, scim.ErrNotFound) {
		t.Fatalf("get after delete err = %v, want ErrNotFound", err)
	}
}

func TestGroupTenantIsolation(t *testing.T) {
	t.Parallel()

	svc := newGroupSvc()
	ctx := context.Background()

	// A user provisioned into org B must not be addable to an org A group.
	orgBUser := seedUser(t, svc, orgB, "b-user@co.com")

	group, err := svc.CreateGroup(ctx, orgA, scim.GroupInput{DisplayName: "Eng", MemberIDs: []string{orgBUser}})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if len(group.Members) != 0 {
		t.Fatalf("cross-tenant member must be dropped, got %v", group.Members)
	}

	// Org B cannot read org A's group.
	if _, err := svc.GetGroup(ctx, orgB, group.ID); !errors.Is(err, scim.ErrNotFound) {
		t.Fatalf("cross-tenant get err = %v, want ErrNotFound", err)
	}
}

func TestHTTPGroupCreateAndPatchMembers(t *testing.T) {
	t.Parallel()

	svc := newGroupSvc()
	handler := scim.NewHandler(svc)
	router := newGroupRouter(handler)
	ctx := context.Background()

	token, _, err := svc.GenerateToken(ctx, orgA, "actor")
	if err != nil {
		t.Fatalf("token: %v", err)
	}

	id1 := seedUser(t, svc, orgA, "u1@co.com")
	id2 := seedUser(t, svc, orgA, "u2@co.com")

	group := postGroup(t, router, token, `{"displayName":"Engineering","members":[{"value":"`+id1+`"}]}`)
	if group.DisplayName != "Engineering" || len(group.Members) != 1 {
		t.Fatalf("unexpected created group: %+v", group)
	}

	// PATCH add id2 via the SCIM members path.
	patched := patchGroup(t, router, token, group.ID,
		`{"Operations":[{"op":"add","path":"members","value":[{"value":"`+id2+`"}]}]}`)
	if len(patched.Members) != 2 {
		t.Fatalf("after add want 2 members, got %v", patched.Members)
	}

	// PATCH remove id1 via the filtered members path.
	removed := patchGroup(t, router, token, group.ID,
		`{"Operations":[{"op":"remove","path":"members[value eq \"`+id1+`\"]"}]}`)
	if len(removed.Members) != 1 || removed.Members[0].Value != id2 {
		t.Fatalf("after remove want only %s, got %+v", id2, removed.Members)
	}
}

func TestListGroupsPagination(t *testing.T) {
	t.Parallel()

	svc := newGroupSvc()
	ctx := context.Background()

	for _, name := range []string{"Alpha", "Bravo", "Charlie", "Delta", "Echo"} {
		if _, err := svc.CreateGroup(ctx, orgA, scim.GroupInput{DisplayName: name}); err != nil {
			t.Fatalf("seed %s: %v", name, err)
		}
	}

	page, total, err := svc.ListGroups(ctx, orgA, "", 2, 2)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if total != 5 {
		t.Fatalf("total = %d, want 5", total)
	}

	if len(page) != 2 || page[0].DisplayName != "Charlie" || page[1].DisplayName != "Delta" {
		t.Fatalf("page 3 (offset 2) = %v, want [Charlie Delta]", names(page))
	}

	// Last partial page.
	last, _, err := svc.ListGroups(ctx, orgA, "", 4, 2)
	if err != nil {
		t.Fatalf("last page: %v", err)
	}

	if len(last) != 1 || last[0].DisplayName != "Echo" {
		t.Fatalf("last page = %v, want [Echo]", names(last))
	}

	// count == 0 returns no resources but the true total.
	none, total0, err := svc.ListGroups(ctx, orgA, "", 0, 0)
	if err != nil {
		t.Fatalf("count-only: %v", err)
	}

	if len(none) != 0 || total0 != 5 {
		t.Fatalf("count-only = %d resources / total %d, want 0 / 5", len(none), total0)
	}
}

func names(groups []scim.GroupRecord) []string {
	out := make([]string, len(groups))
	for i, group := range groups {
		out[i] = group.DisplayName
	}

	return out
}

func TestHTTPListGroupsPaginationEnvelope(t *testing.T) {
	t.Parallel()

	svc := newGroupSvc()
	router := newGroupRouter(scim.NewHandler(svc))
	ctx := context.Background()

	token, _, err := svc.GenerateToken(ctx, orgA, "actor")
	if err != nil {
		t.Fatalf("token: %v", err)
	}

	for _, name := range []string{"Alpha", "Bravo", "Charlie", "Delta", "Echo"} {
		if _, err := svc.CreateGroup(ctx, orgA, scim.GroupInput{DisplayName: name}); err != nil {
			t.Fatalf("seed %s: %v", name, err)
		}
	}

	req := httptest.NewRequestWithContext(
		context.Background(), http.MethodGet, "/scim/v2/Groups?startIndex=3&count=2", nil,
	)
	req.Header.Set("Authorization", "Bearer "+token)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200", rec.Code)
	}

	var body scim.GroupListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if body.TotalResults != 5 || body.StartIndex != 3 || body.ItemsPerPage != 2 || len(body.Resources) != 2 {
		t.Fatalf("envelope = total %d start %d perPage %d n %d, want 5/3/2/2",
			body.TotalResults, body.StartIndex, body.ItemsPerPage, len(body.Resources))
	}
}

func TestPatchGroupRenameToExistingConflicts(t *testing.T) {
	t.Parallel()

	svc := newGroupSvc()
	ctx := context.Background()

	if _, err := svc.CreateGroup(ctx, orgA, scim.GroupInput{DisplayName: "Engineering"}); err != nil {
		t.Fatalf("create eng: %v", err)
	}

	sales, err := svc.CreateGroup(ctx, orgA, scim.GroupInput{DisplayName: "Sales"})
	if err != nil {
		t.Fatalf("create sales: %v", err)
	}

	name := "Engineering"

	_, err = svc.PatchGroup(ctx, orgA, sales.ID, []scim.GroupPatchChange{
		{DisplayName: &name, MemberOp: scim.GroupMembersUnchanged, MemberIDs: nil},
	})
	if !errors.Is(err, scim.ErrConflict) {
		t.Fatalf("rename to taken name err = %v, want ErrConflict", err)
	}
}

func TestHTTPPatchEmptyRemoveIsNoOpButValuelessRemoveClears(t *testing.T) {
	t.Parallel()

	svc := newGroupSvc()
	router := newGroupRouter(scim.NewHandler(svc))
	ctx := context.Background()

	token, _, err := svc.GenerateToken(ctx, orgA, "actor")
	if err != nil {
		t.Fatalf("token: %v", err)
	}

	id1 := seedUser(t, svc, orgA, "u1@co.com")
	id2 := seedUser(t, svc, orgA, "u2@co.com")

	group := postGroup(t, router, token,
		`{"displayName":"Engineering","members":[{"value":"`+id1+`"},{"value":"`+id2+`"}]}`)
	if len(group.Members) != 2 {
		t.Fatalf("setup: want 2 members, got %v", group.Members)
	}

	// `remove` on members WITH an empty value array must be a no-op.
	noop := patchGroup(t, router, token, group.ID,
		`{"Operations":[{"op":"remove","path":"members","value":[]}]}`)
	if len(noop.Members) != 2 {
		t.Fatalf("empty-value remove must NOT clear membership, got %v", noop.Members)
	}

	// `remove` on members with NO value clears the whole attribute.
	cleared := patchGroup(t, router, token, group.ID,
		`{"Operations":[{"op":"remove","path":"members"}]}`)
	if len(cleared.Members) != 0 {
		t.Fatalf("valueless remove must clear membership, got %v", cleared.Members)
	}
}

func newGroupRouter(handler *scim.Handler) http.Handler {
	router := chi.NewRouter()
	router.Group(func(scimRoutes chi.Router) {
		scimRoutes.Use(handler.Authenticate)
		scimRoutes.Get("/scim/v2/Groups", handler.HandleListGroups)
		scimRoutes.Post("/scim/v2/Groups", handler.HandleCreateGroup)
		scimRoutes.Get("/scim/v2/Groups/{groupID}", handler.HandleGetGroup)
		scimRoutes.Patch("/scim/v2/Groups/{groupID}", handler.HandlePatchGroup)
	})

	return router
}

func postGroup(t *testing.T, router http.Handler, token, body string) scim.Group {
	t.Helper()

	req := httptest.NewRequestWithContext(
		context.Background(), http.MethodPost, "/scim/v2/Groups", strings.NewReader(body),
	)
	req.Header.Set("Authorization", "Bearer "+token)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("create group status = %d, want 201 (body: %s)", rec.Code, rec.Body.String())
	}

	return decodeGroupResponse(t, rec)
}

func patchGroup(t *testing.T, router http.Handler, token, id, body string) scim.Group {
	t.Helper()

	req := httptest.NewRequestWithContext(
		context.Background(), http.MethodPatch, "/scim/v2/Groups/"+id, strings.NewReader(body),
	)
	req.Header.Set("Authorization", "Bearer "+token)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("patch group status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}

	return decodeGroupResponse(t, rec)
}

func decodeGroupResponse(t *testing.T, rec *httptest.ResponseRecorder) scim.Group {
	t.Helper()

	var group scim.Group
	if err := json.Unmarshal(rec.Body.Bytes(), &group); err != nil {
		t.Fatalf("decode group: %v", err)
	}

	return group
}

func (m *memGroupStore) DeleteForOrganization(context.Context, string) (int64, error) {
	return 0, nil
}
