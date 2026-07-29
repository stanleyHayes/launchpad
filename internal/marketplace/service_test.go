package marketplace_test

import (
	"context"
	"errors"
	"testing"

	"launchpad/internal/billing"
	"launchpad/internal/marketplace"
)

type memoryRepo struct {
	templates     map[string]marketplace.Template
	installations []marketplace.Installation
	ratings       map[string]marketplace.Rating
	purchases     map[string]marketplace.Purchase
}

func newRepo() *memoryRepo {
	return &memoryRepo{
		templates: map[string]marketplace.Template{},
		ratings:   map[string]marketplace.Rating{},
		purchases: map[string]marketplace.Purchase{},
	}
}
func (r *memoryRepo) CreatePurchase(_ context.Context, item marketplace.Purchase) error {
	r.purchases[item.Reference] = item
	return nil
}
func (r *memoryRepo) GetPurchaseByReference(_ context.Context, organizationID, reference string) (marketplace.Purchase, error) {
	item, ok := r.purchases[reference]
	if !ok || item.OrganizationID != organizationID {
		return marketplace.Purchase{}, marketplace.ErrNotFound
	}
	return item, nil
}
func (r *memoryRepo) UpdatePurchase(_ context.Context, item marketplace.Purchase) error {
	r.purchases[item.Reference] = item
	return nil
}
func (r *memoryRepo) HasPaidPurchase(_ context.Context, organizationID, templateID string) (bool, error) {
	for _, item := range r.purchases {
		if item.OrganizationID == organizationID && item.TemplateID == templateID && item.Status == marketplace.PurchasePaid {
			return true, nil
		}
	}
	return false, nil
}
func (*memoryRepo) EnsureIndexes(context.Context) error { return nil }
func (r *memoryRepo) Create(_ context.Context, item marketplace.Template) error {
	r.templates[item.ID] = item
	return nil
}
func (r *memoryRepo) Get(_ context.Context, id string) (marketplace.Template, error) {
	item, ok := r.templates[id]
	if !ok {
		return marketplace.Template{}, marketplace.ErrNotFound
	}
	return item, nil
}
func (r *memoryRepo) List(_ context.Context, status string) ([]marketplace.Template, error) {
	out := []marketplace.Template{}
	for _, item := range r.templates {
		if status == "" || item.Status == status {
			out = append(out, item)
		}
	}
	return out, nil
}
func (r *memoryRepo) Update(_ context.Context, item marketplace.Template) error {
	r.templates[item.ID] = item
	return nil
}
func (r *memoryRepo) CreateInstallation(_ context.Context, item marketplace.Installation) error {
	r.installations = append(r.installations, item)
	return nil
}
func (r *memoryRepo) UpsertRating(_ context.Context, item marketplace.Rating) error {
	r.ratings[item.TemplateID+"|"+item.OrganizationID+"|"+item.UserID] = item
	return nil
}
func (r *memoryRepo) ListRatings(_ context.Context, id string) ([]marketplace.Rating, error) {
	out := []marketplace.Rating{}
	for _, item := range r.ratings {
		if item.TemplateID == id {
			out = append(out, item)
		}
	}
	return out, nil
}

type installer struct{ calls int }

func (i *installer) InstallMarketplaceTemplate(_ context.Context, _, _, _, _ string, steps []marketplace.Step) (string, error) {
	i.calls++
	if len(steps) == 0 {
		return "", marketplace.ErrInvalidInput
	}
	return "journey-installed", nil
}

type paymentProvider struct{}

func (paymentProvider) InitializeTransaction(
	_ context.Context,
	in billing.InitializeTransactionInput,
) (billing.CheckoutSession, error) {
	return billing.CheckoutSession{
		AuthorizationURL: "https://checkout.example/" + in.Reference,
		Reference:        in.Reference,
	}, nil
}

func (paymentProvider) VerifyTransaction(
	_ context.Context,
	reference string,
) (billing.VerifiedPayment, error) {
	return billing.VerifiedPayment{
		Reference: reference, AmountCents: 5000, Currency: "USD", Paid: true,
	}, nil
}

func TestModerationVersionInstallAndRatings(t *testing.T) {
	t.Parallel()
	repo, install := newRepo(), &installer{}
	svc := marketplace.NewService(repo, install)
	ctx := context.Background()
	item, err := svc.Create(ctx, marketplace.CreateInput{
		Name: "Engineer onboarding", Category: "engineering", Official: true, CreatedBy: "staff-1",
		Steps: []marketplace.Step{{StepType: "task", Title: "Ship a change"}},
	})
	if err != nil || item.Status != marketplace.StatusDraft || item.Version != 1 {
		t.Fatalf("create: %v (%+v)", err, item)
	}
	item, err = svc.SetStatus(ctx, item.ID, marketplace.StatusPublished)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	item, err = svc.SetFeatured(ctx, item.ID, true)
	if err != nil || !item.Featured {
		t.Fatalf("feature: %v (%+v)", err, item)
	}
	installation, err := svc.Install(ctx, item.ID, "org-1", "user-1")
	if err != nil || installation.JourneyTemplateID != "journey-installed" || install.calls != 1 {
		t.Fatalf("install: %v (%+v)", err, installation)
	}
	item, err = svc.Rate(ctx, item.ID, "org-1", "user-1", 5)
	if err != nil || item.RatingAverage != 5 || item.RatingCount != 1 {
		t.Fatalf("rate: %v (%+v)", err, item)
	}
	item, err = svc.NewVersion(ctx, item.ID, []marketplace.Step{{StepType: "document", Title: "Read handbook"}})
	if err != nil || item.Version != 2 || item.Status != marketplace.StatusDraft || item.Featured {
		t.Fatalf("version: %v (%+v)", err, item)
	}
}

func TestPaidTemplateRequiresVerifiedPurchaseBeforeInstall(t *testing.T) {
	t.Parallel()
	repo, install := newRepo(), &installer{}
	svc := marketplace.NewService(repo, install).
		WithPayments(paymentProvider{}, "https://app.example/marketplace").
		WithBuyerEmail(func(context.Context, string) (string, error) {
			return "buyer@example.com", nil
		})
	item, err := svc.Create(context.Background(), marketplace.CreateInput{
		Name: "Manager onboarding", Category: "leadership", CreatedBy: "seller-user",
		SubmittedByOrganizationID: "seller-org", PriceCents: 5000, Currency: "USD",
		Steps: []marketplace.Step{{StepType: "task", Title: "Meet the team"}},
	})
	if err != nil {
		t.Fatalf("create paid template: %v", err)
	}
	item, err = svc.SetStatus(context.Background(), item.ID, marketplace.StatusPublished)
	if err != nil {
		t.Fatalf("publish paid template: %v", err)
	}
	if _, err = svc.Install(context.Background(), item.ID, "buyer-org", "buyer-user"); !errors.Is(err, marketplace.ErrPaymentRequired) {
		t.Fatalf("install without purchase = %v, want payment required", err)
	}
	checkout, err := svc.BeginPurchase(
		context.Background(), item.ID, "buyer-org", "buyer-user",
	)
	if err != nil || checkout.AuthorizationURL == "" {
		t.Fatalf("begin purchase: %v (%+v)", err, checkout)
	}
	installation, err := svc.CompletePurchase(
		context.Background(), checkout.Reference, "buyer-org", "buyer-user",
	)
	if err != nil || installation.TemplateID != item.ID || install.calls != 1 {
		t.Fatalf("complete purchase: %v (%+v), calls=%d", err, installation, install.calls)
	}
	purchase := repo.purchases[checkout.Reference]
	if purchase.Status != marketplace.PurchasePaid || purchase.PlatformFeeCents != 750 ||
		purchase.SellerEarningsCents != 4250 {
		t.Fatalf("purchase ledger = %+v", purchase)
	}
}

func TestCustomerSubmissionRequiresReview(t *testing.T) {
	t.Parallel()
	svc := marketplace.NewService(newRepo(), &installer{})
	item, err := svc.Create(context.Background(), marketplace.CreateInput{
		Name: "Sales ramp", Category: "sales", SubmittedByOrganizationID: "org-1", CreatedBy: "user-1",
		Steps: []marketplace.Step{{StepType: "task", Title: "Practice pitch"}},
	})
	if err != nil || item.Status != marketplace.StatusSubmitted || item.Official {
		t.Fatalf("submission: %v (%+v)", err, item)
	}
}
