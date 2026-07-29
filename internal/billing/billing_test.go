package billing_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"launchpad/internal/billing"
)

// --- in-memory fakes --------------------------------------------------------

type fakeBillingRepo struct {
	plans      map[string]billing.Plan
	subs       map[string]billing.Subscription
	invoices   map[string]billing.Invoice
	invoiceSeq map[string]int64
	getPlanErr error
	coupons    map[string]billing.Coupon
}

func newFakeBillingRepo() *fakeBillingRepo {
	return &fakeBillingRepo{
		plans:      map[string]billing.Plan{},
		subs:       map[string]billing.Subscription{},
		invoices:   map[string]billing.Invoice{},
		invoiceSeq: map[string]int64{},
		coupons:    map[string]billing.Coupon{},
	}
}

func (f *fakeBillingRepo) CreateCoupon(_ context.Context, coupon billing.Coupon) error {
	if _, exists := f.coupons[coupon.Code]; exists {
		return billing.ErrCodeTaken
	}
	f.coupons[coupon.Code] = coupon
	return nil
}
func (f *fakeBillingRepo) GetCoupon(_ context.Context, code string) (billing.Coupon, error) {
	coupon, ok := f.coupons[code]
	if !ok {
		return billing.Coupon{}, billing.ErrNotFound
	}
	return coupon, nil
}
func (f *fakeBillingRepo) ListCoupons(context.Context) ([]billing.Coupon, error) {
	out := make([]billing.Coupon, 0, len(f.coupons))
	for _, coupon := range f.coupons {
		out = append(out, coupon)
	}
	return out, nil
}
func (f *fakeBillingRepo) UpdateCoupon(_ context.Context, coupon billing.Coupon) error {
	f.coupons[coupon.Code] = coupon
	return nil
}

func (f *fakeBillingRepo) EnsureIndexes(context.Context) error { return nil }

func (f *fakeBillingRepo) UpsertPlan(_ context.Context, plan billing.Plan) error {
	f.plans[plan.Code] = plan

	return nil
}

func (f *fakeBillingRepo) GetPlan(_ context.Context, code string) (billing.Plan, error) {
	if f.getPlanErr != nil {
		return billing.Plan{}, f.getPlanErr
	}

	if plan, ok := f.plans[code]; ok {
		return plan, nil
	}

	return billing.Plan{}, billing.ErrNotFound
}

func (f *fakeBillingRepo) ListPlans(_ context.Context, activeOnly bool) ([]billing.Plan, error) {
	out := make([]billing.Plan, 0)

	for _, plan := range f.plans {
		if activeOnly && !plan.Active {
			continue
		}

		out = append(out, plan)
	}

	return out, nil
}

func (f *fakeBillingRepo) CreatePlan(_ context.Context, plan billing.Plan) error {
	if _, exists := f.plans[plan.Code]; exists {
		return billing.ErrCodeTaken
	}

	f.plans[plan.Code] = plan

	return nil
}

func (f *fakeBillingRepo) UpdatePlan(_ context.Context, plan billing.Plan) error {
	if _, exists := f.plans[plan.Code]; !exists {
		return billing.ErrNotFound
	}

	f.plans[plan.Code] = plan

	return nil
}

func (f *fakeBillingRepo) GetSubscriptionByOrganization(
	_ context.Context,
	organizationID string,
) (billing.Subscription, error) {
	if subscription, ok := f.subs[organizationID]; ok {
		return subscription, nil
	}

	return billing.Subscription{}, billing.ErrNotFound
}

func (f *fakeBillingRepo) CreateSubscription(_ context.Context, subscription billing.Subscription) error {
	f.subs[subscription.OrganizationID] = subscription

	return nil
}

func (f *fakeBillingRepo) UpdateSubscription(_ context.Context, subscription billing.Subscription) error {
	if _, exists := f.subs[subscription.OrganizationID]; !exists {
		return billing.ErrNotFound
	}

	f.subs[subscription.OrganizationID] = subscription

	return nil
}

func (f *fakeBillingRepo) ListSubscriptions(context.Context) ([]billing.Subscription, error) {
	out := make([]billing.Subscription, 0, len(f.subs))
	for _, subscription := range f.subs {
		out = append(out, subscription)
	}

	return out, nil
}

func (f *fakeBillingRepo) ListInvoices(context.Context) ([]billing.Invoice, error) {
	out := make([]billing.Invoice, 0, len(f.invoices))
	for _, invoice := range f.invoices {
		out = append(out, invoice)
	}
	return out, nil
}

// stubOrgDirectory backs both billing.OrganizationReader and
// billing.OrgPlanUpdater over one shared map.
type stubOrgDirectory struct {
	orgs map[string]billing.OrganizationSummary
}

func newStubOrgDirectory() *stubOrgDirectory {
	return &stubOrgDirectory{orgs: map[string]billing.OrganizationSummary{}}
}

func (f *stubOrgDirectory) Get(_ context.Context, id string) (billing.OrganizationSummary, error) {
	if org, ok := f.orgs[id]; ok {
		return org, nil
	}

	return billing.OrganizationSummary{}, billing.ErrNotFound
}

func (f *stubOrgDirectory) SetPlanCode(
	_ context.Context,
	id, planCode string,
) (billing.OrganizationSummary, error) {
	org, ok := f.orgs[id]
	if !ok {
		return billing.OrganizationSummary{}, billing.ErrNotFound
	}

	org.PlanCode = planCode
	f.orgs[id] = org

	return org, nil
}

func newBillingService() (*billing.Service, *fakeBillingRepo, *stubOrgDirectory) {
	repo := newFakeBillingRepo()
	orgs := newStubOrgDirectory()

	return billing.NewService(repo, orgs, orgs), repo, orgs
}

func seedPlan(t *testing.T, svc *billing.Service, code string, active bool) {
	t.Helper()

	_, err := svc.CreatePlan(context.Background(), billing.CreatePlanInput{
		Code: code, Name: code, PriceMonthlyCents: 100, Active: active,
	})
	if err != nil {
		t.Fatalf("seed plan %q: %v", code, err)
	}
}

// --- plans ------------------------------------------------------------------

func TestCreatePlanRequiresCodeAndName(t *testing.T) {
	t.Parallel()

	svc, _, _ := newBillingService()
	ctx := context.Background()

	for _, in := range []billing.CreatePlanInput{
		{Name: "No code"},
		{Code: "no-name"},
		{Code: "  ", Name: "  "},
	} {
		if _, err := svc.CreatePlan(ctx, in); !errors.Is(err, billing.ErrInvalidInput) {
			t.Fatalf("input %+v: got %v, want ErrInvalidInput", in, err)
		}
	}
}

func TestCreatePlanDefaultsCurrencyAndRejectsDuplicateCode(t *testing.T) {
	t.Parallel()

	svc, _, _ := newBillingService()
	ctx := context.Background()

	plan, err := svc.CreatePlan(ctx, billing.CreatePlanInput{
		Code: "growth", Name: " Growth ", PriceMonthlyCents: 9900, Active: true,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if plan.Currency != "USD" {
		t.Fatalf("empty currency should default to USD, got %q", plan.Currency)
	}

	if plan.Name != "Growth" {
		t.Fatalf("name should be trimmed, got %q", plan.Name)
	}

	if _, err := svc.CreatePlan(ctx, billing.CreatePlanInput{Code: "growth", Name: "Again"}); !errors.Is(
		err, billing.ErrCodeTaken,
	) {
		t.Fatalf("duplicate code got %v, want ErrCodeTaken", err)
	}
}

func TestUpdatePlanPatchesFieldsAndValidates(t *testing.T) {
	t.Parallel()

	svc, repo, _ := newBillingService()
	ctx := context.Background()

	seedPlan(t, svc, "growth", true)

	if _, err := svc.UpdatePlan(ctx, "missing", billing.UpdatePlanInput{}); !errors.Is(err, billing.ErrNotFound) {
		t.Fatalf("unknown plan got %v, want ErrNotFound", err)
	}

	blank := "   "
	if _, err := svc.UpdatePlan(ctx, "growth", billing.UpdatePlanInput{Name: &blank}); !errors.Is(
		err, billing.ErrInvalidInput,
	) {
		t.Fatalf("blank name got %v, want ErrInvalidInput", err)
	}

	price := 12900
	active := false

	updated, err := svc.UpdatePlan(ctx, "growth", billing.UpdatePlanInput{
		PriceMonthlyCents: &price, Active: &active,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	stored := repo.plans["growth"]
	if stored.PriceMonthlyCents != 12900 || stored.Active || stored.Code != updated.Code {
		t.Fatalf("patch not persisted: %+v", stored)
	}

	// Inactive plans are excluded from the active-only listing.
	activePlans, err := svc.ListPlans(ctx, true)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if len(activePlans) != 0 {
		t.Fatalf("inactive plan should be hidden from the active listing, got %+v", activePlans)
	}
}

func TestSeedDefaultsSeedsBuiltinPlans(t *testing.T) {
	t.Parallel()

	svc, repo, _ := newBillingService()
	ctx := context.Background()

	if err := svc.SeedDefaults(ctx); err != nil {
		t.Fatalf("seed: %v", err)
	}

	for _, code := range []string{"starter", "growth", "enterprise"} {
		plan, ok := repo.plans[code]
		if !ok || !plan.Active {
			t.Fatalf("built-in plan %q missing or inactive: %+v", code, plan)
		}
	}

	if repo.plans["growth"].PriceMonthlyCents != 9900 {
		t.Fatalf("growth price = %d, want 9900", repo.plans["growth"].PriceMonthlyCents)
	}

	// Re-seeding must preserve the original CreatedAt of existing plans.
	createdAt := repo.plans["growth"].CreatedAt

	if err := svc.SeedDefaults(ctx); err != nil {
		t.Fatalf("re-seed: %v", err)
	}

	if !repo.plans["growth"].CreatedAt.Equal(createdAt) {
		t.Fatalf("re-seed changed CreatedAt: %v -> %v", createdAt, repo.plans["growth"].CreatedAt)
	}
}

func TestSeedDefaultsDoesNotOverwriteAdminChanges(t *testing.T) {
	t.Parallel()

	svc, repo, _ := newBillingService()
	ctx := context.Background()

	if err := svc.SeedDefaults(ctx); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// An admin reprices and disables the growth plan.
	customized := repo.plans["growth"]
	customized.PriceMonthlyCents = 4200
	customized.Active = false
	repo.plans["growth"] = customized

	if err := svc.SeedDefaults(ctx); err != nil {
		t.Fatalf("re-seed: %v", err)
	}

	got := repo.plans["growth"]
	if got.PriceMonthlyCents != 4200 || got.Active {
		t.Fatalf("re-seed overwrote admin changes: %+v", got)
	}
}

func TestSeedDefaultsSurfacesLookupErrors(t *testing.T) {
	t.Parallel()

	svc, repo, _ := newBillingService()
	repo.getPlanErr = errors.New("mongo unreachable")

	err := svc.SeedDefaults(context.Background())
	if err == nil || !strings.Contains(err.Error(), "mongo unreachable") {
		t.Fatalf("got %v, want wrapped lookup error", err)
	}
}

// --- subscriptions ----------------------------------------------------------

func TestSetOrganizationPlanAssignsSubscription(t *testing.T) {
	t.Parallel()

	svc, _, orgs := newBillingService()
	ctx := context.Background()

	seedPlan(t, svc, "growth", true)

	orgs.orgs["org-1"] = billing.OrganizationSummary{ID: "org-1", PlanCode: "starter", Status: "trial"}

	subscription, err := svc.SetOrganizationPlan(ctx, billing.SetOrganizationPlanInput{
		OrganizationID: "org-1", PlanCode: "growth",
	})
	if err != nil {
		t.Fatalf("assign: %v", err)
	}

	if subscription.ID == "" || subscription.PlanCode != "growth" {
		t.Fatalf("unexpected subscription: %+v", subscription)
	}

	// No explicit status: derived from the trial organization.
	if subscription.Status != "trialing" {
		t.Fatalf("trial org should map to trialing, got %q", subscription.Status)
	}

	if subscription.CurrentPeriodEnd == nil || !subscription.CurrentPeriodEnd.After(time.Now()) {
		t.Fatalf("current period end should be in the future: %+v", subscription.CurrentPeriodEnd)
	}

	if orgs.orgs["org-1"].PlanCode != "growth" {
		t.Fatalf("organization plan code not updated: %+v", orgs.orgs["org-1"])
	}
}

func TestSetOrganizationPlanChangesExistingSubscription(t *testing.T) {
	t.Parallel()

	svc, _, orgs := newBillingService()
	ctx := context.Background()

	seedPlan(t, svc, "growth", true)
	seedPlan(t, svc, "enterprise", true)

	orgs.orgs["org-1"] = billing.OrganizationSummary{ID: "org-1", PlanCode: "starter", Status: "active"}

	first, err := svc.SetOrganizationPlan(ctx, billing.SetOrganizationPlanInput{
		OrganizationID: "org-1", PlanCode: "growth",
	})
	if err != nil {
		t.Fatalf("assign: %v", err)
	}

	changed, err := svc.SetOrganizationPlan(ctx, billing.SetOrganizationPlanInput{
		OrganizationID: "org-1", PlanCode: "enterprise", Status: "past_due",
	})
	if err != nil {
		t.Fatalf("change: %v", err)
	}

	if changed.ID != first.ID {
		t.Fatalf("plan change must update the existing subscription, got new ID %q (was %q)", changed.ID, first.ID)
	}

	if changed.PlanCode != "enterprise" || changed.Status != "past_due" {
		t.Fatalf("plan change not applied: %+v", changed)
	}
}

func TestSetOrganizationPlanErrorPaths(t *testing.T) {
	t.Parallel()

	svc, _, orgs := newBillingService()
	ctx := context.Background()

	seedPlan(t, svc, "growth", true)
	seedPlan(t, svc, "retired", false)

	orgs.orgs["org-1"] = billing.OrganizationSummary{ID: "org-1", PlanCode: "starter", Status: "active"}

	cases := map[string]struct {
		in  billing.SetOrganizationPlanInput
		err error
	}{
		"missing organization": {in: billing.SetOrganizationPlanInput{PlanCode: "growth"}, err: billing.ErrInvalidInput},
		"missing plan":         {in: billing.SetOrganizationPlanInput{OrganizationID: "org-1"}, err: billing.ErrInvalidInput},
		"unknown plan": {
			in:  billing.SetOrganizationPlanInput{OrganizationID: "org-1", PlanCode: "nope"},
			err: billing.ErrNotFound,
		},
		"inactive plan": {
			in:  billing.SetOrganizationPlanInput{OrganizationID: "org-1", PlanCode: "retired"},
			err: billing.ErrInvalidInput,
		},
		"invalid status": {
			in:  billing.SetOrganizationPlanInput{OrganizationID: "org-1", PlanCode: "growth", Status: "weird"},
			err: billing.ErrInvalidInput,
		},
		"unknown organization": {
			in:  billing.SetOrganizationPlanInput{OrganizationID: "ghost", PlanCode: "growth"},
			err: billing.ErrNotFound,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if _, err := svc.SetOrganizationPlan(ctx, tc.in); !errors.Is(err, tc.err) {
				t.Fatalf("got %v, want %v", err, tc.err)
			}
		})
	}
}

func TestGetOrCreateSubscriptionCreatesFromOrganization(t *testing.T) {
	t.Parallel()

	svc, _, orgs := newBillingService()
	ctx := context.Background()

	orgs.orgs["org-1"] = billing.OrganizationSummary{ID: "org-1", PlanCode: "starter", Status: "active"}

	created, err := svc.GetOrCreateSubscription(ctx, "org-1")
	if err != nil {
		t.Fatalf("get or create: %v", err)
	}

	if created.PlanCode != "starter" || created.Status != "active" {
		t.Fatalf("subscription should mirror the organization: %+v", created)
	}

	// A second call returns the persisted subscription, not a new one.
	again, err := svc.GetOrCreateSubscription(ctx, "org-1")
	if err != nil {
		t.Fatalf("second get or create: %v", err)
	}

	if again.ID != created.ID {
		t.Fatalf("second call created a new subscription: %q != %q", again.ID, created.ID)
	}

	if _, err := svc.GetOrCreateSubscription(ctx, "ghost"); !errors.Is(err, billing.ErrNotFound) {
		t.Fatalf("unknown organization got %v, want ErrNotFound", err)
	}
}

func (f *fakeBillingRepo) DeleteForOrganization(context.Context, string) (int64, error) {
	return 0, nil
}

func (f *fakeBillingRepo) NextInvoiceSequence(_ context.Context, organizationID string) (int64, error) {
	f.invoiceSeq[organizationID]++

	return f.invoiceSeq[organizationID], nil
}

func (f *fakeBillingRepo) CreateInvoice(_ context.Context, invoice billing.Invoice) error {
	f.invoices[invoice.ID] = invoice

	return nil
}

func (f *fakeBillingRepo) GetInvoice(
	_ context.Context,
	organizationID, invoiceID string,
) (billing.Invoice, error) {
	invoice, ok := f.invoices[invoiceID]
	if !ok || invoice.OrganizationID != organizationID {
		return billing.Invoice{}, billing.ErrNotFound
	}

	return invoice, nil
}

func (f *fakeBillingRepo) GetInvoiceByPaymentRef(
	_ context.Context,
	reference string,
) (billing.Invoice, error) {
	for _, invoice := range f.invoices {
		if invoice.PaymentRef == reference {
			return invoice, nil
		}
	}

	return billing.Invoice{}, billing.ErrNotFound
}

func (f *fakeBillingRepo) ListInvoicesByOrganization(
	_ context.Context,
	organizationID string,
) ([]billing.Invoice, error) {
	out := make([]billing.Invoice, 0)

	for _, invoice := range f.invoices {
		if invoice.OrganizationID == organizationID {
			out = append(out, invoice)
		}
	}

	return out, nil
}

func (f *fakeBillingRepo) UpdateInvoice(_ context.Context, invoice billing.Invoice) error {
	if _, exists := f.invoices[invoice.ID]; !exists {
		return billing.ErrNotFound
	}

	f.invoices[invoice.ID] = invoice

	return nil
}
