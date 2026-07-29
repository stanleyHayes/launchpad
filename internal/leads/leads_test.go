package leads_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"launchpad/internal/leads"
)

type fakeLeadRepo struct {
	leads []leads.Lead
}

func (f *fakeLeadRepo) EnsureIndexes(context.Context) error { return nil }

func (f *fakeLeadRepo) Create(_ context.Context, lead leads.Lead) error {
	f.leads = append(f.leads, lead)

	return nil
}

func (f *fakeLeadRepo) List(_ context.Context, limit int64, before time.Time) ([]leads.Lead, error) {
	out := make([]leads.Lead, 0, len(f.leads))

	for _, lead := range f.leads {
		if !before.IsZero() && !lead.CreatedAt.Before(before) {
			continue
		}

		out = append(out, lead)
	}

	if limit > 0 && int64(len(out)) > limit {
		out = out[:limit]
	}

	return out, nil
}

func (f *fakeLeadRepo) Count(context.Context) (int64, error) { return int64(len(f.leads)), nil }

func newLeadService() (*leads.Service, *fakeLeadRepo) {
	repo := &fakeLeadRepo{}

	return leads.NewService(repo), repo
}

func TestCreateLeadValidatesInput(t *testing.T) {
	t.Parallel()

	svc, _ := newLeadService()
	ctx := context.Background()

	for _, in := range []leads.CreateInput{
		{Email: "a@acme.test"},               // missing name
		{Name: "Ada"},                        // missing email
		{Name: "Ada", Email: "not-an-email"}, // malformed email
		{Name: "  ", Email: "a@acme.test"},   // blank name
		{Name: "Ada", Email: "  "},           // blank email
	} {
		if _, err := svc.Create(ctx, in); !errors.Is(err, leads.ErrInvalidInput) {
			t.Fatalf("input %+v: got %v, want ErrInvalidInput", in, err)
		}
	}
}

func TestCreateLeadNormalizesAndDefaults(t *testing.T) {
	t.Parallel()

	svc, repo := newLeadService()
	ctx := context.Background()

	lead, err := svc.Create(ctx, leads.CreateInput{
		Name: "  Ada Lovelace ", Email: " ADA@Acme.TEST ", Company: "Acme",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if lead.ID == "" {
		t.Fatal("lead ID not assigned")
	}

	if lead.Name != "Ada Lovelace" || lead.Email != "ada@acme.test" {
		t.Fatalf("input not normalized: %+v", lead)
	}

	if lead.Source != "website" {
		t.Fatalf("empty source should default to website, got %q", lead.Source)
	}

	if lead.Status != "new" {
		t.Fatalf("new lead should start in status new, got %q", lead.Status)
	}

	// An explicit source is preserved.
	second, err := svc.Create(ctx, leads.CreateInput{
		Name: "Grace", Email: "grace@acme.test", Source: "referral",
	})
	if err != nil {
		t.Fatalf("create second: %v", err)
	}

	if second.Source != "referral" {
		t.Fatalf("explicit source not preserved, got %q", second.Source)
	}

	if len(repo.leads) != 2 {
		t.Fatalf("expected 2 stored leads, got %d", len(repo.leads))
	}
}

func TestListAndCountReflectCreatedLeads(t *testing.T) {
	t.Parallel()

	svc, _ := newLeadService()
	ctx := context.Background()

	if count, err := svc.Count(ctx); err != nil || count != 0 {
		t.Fatalf("empty count = %d, %v", count, err)
	}

	for _, name := range []string{"Ada", "Grace", "Alan"} {
		if _, err := svc.Create(ctx, leads.CreateInput{Name: name, Email: name + "@acme.test"}); err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
	}

	count, err := svc.Count(ctx)
	if err != nil || count != 3 {
		t.Fatalf("count = %d, %v; want 3", count, err)
	}

	items, err := svc.List(ctx, 0, time.Time{})
	if err != nil || len(items) != 3 {
		t.Fatalf("list = %+v, %v; want 3 leads", items, err)
	}
}
