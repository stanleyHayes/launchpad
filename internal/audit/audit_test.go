package audit_test

import (
	"context"
	"testing"

	"launchpad/internal/audit"
	"launchpad/pkg/security"
)

// fakeAuditRepo applies the store's list semantics: only events belonging to
// the requested organization are returned, capped by the limit.
type fakeAuditRepo struct {
	events []audit.Event
}

func (f *fakeAuditRepo) EnsureIndexes(context.Context) error { return nil }

func (f *fakeAuditRepo) Write(_ context.Context, event audit.Event) error {
	f.events = append(f.events, event)

	return nil
}

func (f *fakeAuditRepo) ListByOrganization(
	_ context.Context,
	organizationID string,
	limit int64,
) ([]audit.Event, error) {
	out := make([]audit.Event, 0)

	for _, event := range f.events {
		if event.OrganizationID != nil && *event.OrganizationID == organizationID {
			out = append(out, event)
		}
	}

	if limit > 0 && int64(len(out)) > limit {
		out = out[:limit]
	}

	return out, nil
}

func (f *fakeAuditRepo) ListAll(_ context.Context, limit int64) ([]audit.Event, error) {
	out := f.events

	if limit > 0 && int64(len(out)) > limit {
		out = out[:limit]
	}

	return out, nil
}

func TestRecordPersistsEventFields(t *testing.T) {
	t.Parallel()

	repo := &fakeAuditRepo{}
	svc := audit.NewService(repo)
	ctx := context.Background()

	orgID := "org-1"
	metadata := map[string]any{"email": "owner@acme.test"}

	err := svc.Record(ctx, &orgID, "user-1", "auth.register", "organization", "org-1", metadata)
	if err != nil {
		t.Fatalf("record: %v", err)
	}

	if len(repo.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(repo.events))
	}

	event := repo.events[0]
	if event.OrganizationID == nil || *event.OrganizationID != orgID ||
		event.ActorUserID != "user-1" || event.Action != "auth.register" ||
		event.ResourceType != "organization" || event.ResourceID != "org-1" ||
		event.Metadata["email"] != "owner@acme.test" {
		t.Fatalf("event fields not passed through: %+v", event)
	}

	// Platform-level events carry no organization.
	if err := svc.Record(ctx, nil, "user-1", "auth.login", "user", "user-1", nil); err != nil {
		t.Fatalf("record without org: %v", err)
	}

	if repo.events[1].OrganizationID != nil {
		t.Fatalf("platform event should have no organization: %+v", repo.events[1])
	}
}

func TestListFiltersByOrganizationAndLimit(t *testing.T) {
	t.Parallel()

	repo := &fakeAuditRepo{}
	svc := audit.NewService(repo)
	ctx := context.Background()

	orgA, orgB := "org-a", "org-b"

	for _, tc := range []struct {
		org    *string
		action string
	}{
		{&orgA, "employee.created"},
		{&orgA, "employee.updated"},
		{&orgB, "employee.created"},
		{nil, "auth.login"},
	} {
		if err := svc.Record(ctx, tc.org, "user-1", tc.action, "employee", "emp-1", nil); err != nil {
			t.Fatalf("record: %v", err)
		}
	}

	events, err := svc.List(ctx, orgA, 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if len(events) != 2 {
		t.Fatalf("expected only org-a's 2 events, got %d: %+v", len(events), events)
	}

	for _, event := range events {
		if *event.OrganizationID != orgA {
			t.Fatalf("foreign event leaked into the listing: %+v", event)
		}
	}

	limited, err := svc.List(ctx, orgA, 1)
	if err != nil {
		t.Fatalf("list with limit: %v", err)
	}

	if len(limited) != 1 {
		t.Fatalf("limit not applied, got %d events", len(limited))
	}

	// Events without an organization never show up in a tenant listing.
	other, err := svc.List(ctx, orgB, 10)
	if err != nil {
		t.Fatalf("list org-b: %v", err)
	}

	if len(other) != 1 {
		t.Fatalf("expected org-b's single event, got %d", len(other))
	}
}

func TestListAllSpansOrganizationsAndHonorsLimit(t *testing.T) {
	t.Parallel()

	repo := &fakeAuditRepo{}
	svc := audit.NewService(repo)
	ctx := context.Background()

	orgA, orgB := "org-a", "org-b"

	for _, org := range []*string{&orgA, &orgB, nil} {
		if err := svc.Record(ctx, org, "user-1", "employee.created", "employee", "emp-1", nil); err != nil {
			t.Fatalf("record: %v", err)
		}
	}

	events, err := svc.ListAll(ctx, 10)
	if err != nil {
		t.Fatalf("list all: %v", err)
	}

	// All three events come back, including the platform-level one with no org.
	if len(events) != 3 {
		t.Fatalf("expected every event across tenants, got %d: %+v", len(events), events)
	}

	limited, err := svc.ListAll(ctx, 2)
	if err != nil {
		t.Fatalf("list all with limit: %v", err)
	}

	if len(limited) != 2 {
		t.Fatalf("limit not applied, got %d events", len(limited))
	}
}

func (f *fakeAuditRepo) CountByOrganization(context.Context, string) (int64, error) {
	return 0, nil
}

func (f *fakeAuditRepo) DeleteForOrganization(context.Context, string) (int64, error) {
	return 0, nil
}

func TestRecordAddsImpersonationContext(t *testing.T) {
	t.Parallel()

	repo := &fakeAuditRepo{}
	svc := audit.NewService(repo)

	ctx := security.WithPrincipal(context.Background(), security.Principal{
		UserID:                 "agent-1",
		OrganizationID:         "org-1",
		RoleCode:               "hr_admin",
		SessionID:              "support-session-1",
		Impersonator:           true,
		ImpersonationSessionID: "support-session-1",
	})

	orgID := "org-1"
	if err := svc.Record(ctx, &orgID, "agent-1", "employees.read", "employee", "emp-1", nil); err != nil {
		t.Fatalf("record: %v", err)
	}

	if len(repo.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(repo.events))
	}

	event := repo.events[0]
	if event.ActorUserID != "agent-1" {
		t.Errorf("ActorUserID=%q, want the support agent", event.ActorUserID)
	}

	if event.ImpersonatorUserID != "agent-1" {
		t.Errorf("ImpersonatorUserID=%q, want agent-1", event.ImpersonatorUserID)
	}

	if event.ImpersonationSessionID != "support-session-1" {
		t.Errorf("ImpersonationSessionID=%q, want support-session-1", event.ImpersonationSessionID)
	}
}

func TestRecordWithoutImpersonationLeavesContextEmpty(t *testing.T) {
	t.Parallel()

	repo := &fakeAuditRepo{}
	svc := audit.NewService(repo)

	ctx := security.WithPrincipal(context.Background(), security.Principal{
		UserID:         "user-1",
		OrganizationID: "org-1",
		RoleCode:       "hr_admin",
		SessionID:      "sess-1",
	})

	orgID := "org-1"
	if err := svc.Record(ctx, &orgID, "user-1", "employees.read", "employee", "emp-1", nil); err != nil {
		t.Fatalf("record: %v", err)
	}

	event := repo.events[0]
	if event.ImpersonatorUserID != "" || event.ImpersonationSessionID != "" {
		t.Errorf("expected no impersonation context, got %+v", event)
	}
}
