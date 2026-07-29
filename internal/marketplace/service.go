package marketplace

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"launchpad/internal/billing"
)

type Service struct {
	repo        Repository
	installer   JourneyInstaller
	payments    billing.PaymentProvider
	callbackURL string
	buyerEmail  func(context.Context, string) (string, error)
}

func (s *Service) WithBuyerEmail(loader func(context.Context, string) (string, error)) *Service {
	s.buyerEmail = loader
	return s
}

func (s *Service) WithPayments(provider billing.PaymentProvider, callbackURL string) *Service {
	s.payments = provider
	s.callbackURL = strings.TrimSpace(callbackURL)
	return s
}

func NewService(repo Repository, installer JourneyInstaller) *Service {
	return &Service{repo: repo, installer: installer}
}

func (s *Service) Create(ctx context.Context, in CreateInput) (Template, error) {
	name, category := strings.TrimSpace(in.Name), strings.TrimSpace(in.Category)
	if name == "" || category == "" || len(in.Steps) == 0 || strings.TrimSpace(in.CreatedBy) == "" {
		return Template{}, ErrInvalidInput
	}
	currency := strings.ToUpper(strings.TrimSpace(in.Currency))
	if in.PriceCents < 0 || (in.PriceCents > 0 && currency == "") {
		return Template{}, ErrInvalidInput
	}
	if currency == "" {
		currency = "USD"
	}
	now := time.Now().UTC()
	status := StatusSubmitted
	if in.Official {
		status = StatusDraft
	}
	item := Template{
		ID: uuid.NewString(), Name: name, Slug: slugify(name), Description: strings.TrimSpace(in.Description),
		Category: category, Status: status, Official: in.Official, Version: 1,
		SubmittedByOrganizationID: strings.TrimSpace(in.SubmittedByOrganizationID),
		Steps:                     in.Steps, PriceCents: in.PriceCents, Currency: currency,
		CreatedBy: strings.TrimSpace(in.CreatedBy), CreatedAt: now, UpdatedAt: now,
	}
	if err := s.repo.Create(ctx, item); err != nil {
		return Template{}, fmt.Errorf("create marketplace template: %w", err)
	}
	return item, nil
}

func (s *Service) ListPublished(ctx context.Context) ([]Template, error) {
	return s.list(ctx, StatusPublished)
}

func (s *Service) ListAll(ctx context.Context) ([]Template, error) {
	return s.list(ctx, "")
}

func (s *Service) ListByOrganization(ctx context.Context, organizationID string) ([]Template, error) {
	items, err := s.list(ctx, "")
	if err != nil {
		return nil, err
	}
	out := make([]Template, 0)
	for _, item := range items {
		if item.SubmittedByOrganizationID == organizationID {
			out = append(out, item)
		}
	}
	return out, nil
}

func (s *Service) list(ctx context.Context, status string) ([]Template, error) {
	items, err := s.repo.List(ctx, status)
	if err != nil {
		return nil, fmt.Errorf("list marketplace templates: %w", err)
	}
	return items, nil
}

func (s *Service) SetStatus(ctx context.Context, id, status string) (Template, error) {
	if status != StatusPublished && status != StatusRemoved && status != StatusSubmitted {
		return Template{}, ErrInvalidInput
	}
	item, err := s.repo.Get(ctx, id)
	if err != nil {
		return Template{}, fmt.Errorf("get marketplace template: %w", err)
	}
	if item.Status == StatusRemoved && status != StatusRemoved {
		return Template{}, ErrInvalidState
	}
	item.Status = status
	item.UpdatedAt = time.Now().UTC()
	if err := s.repo.Update(ctx, item); err != nil {
		return Template{}, fmt.Errorf("update marketplace template: %w", err)
	}
	return item, nil
}

func (s *Service) SetFeatured(ctx context.Context, id string, featured bool) (Template, error) {
	item, err := s.repo.Get(ctx, id)
	if err != nil {
		return Template{}, fmt.Errorf("get marketplace template: %w", err)
	}
	if item.Status != StatusPublished {
		return Template{}, ErrInvalidState
	}
	item.Featured, item.UpdatedAt = featured, time.Now().UTC()
	if err := s.repo.Update(ctx, item); err != nil {
		return Template{}, fmt.Errorf("feature marketplace template: %w", err)
	}
	return item, nil
}

func (s *Service) NewVersion(ctx context.Context, id string, steps []Step) (Template, error) {
	if len(steps) == 0 {
		return Template{}, ErrInvalidInput
	}
	item, err := s.repo.Get(ctx, id)
	if err != nil {
		return Template{}, fmt.Errorf("get marketplace template: %w", err)
	}
	if item.Status == StatusRemoved {
		return Template{}, ErrInvalidState
	}
	item.Version++
	item.Steps = steps
	item.Status = StatusDraft
	if !item.Official {
		item.Status = StatusSubmitted
	}
	item.Featured = false
	item.UpdatedAt = time.Now().UTC()
	if err := s.repo.Update(ctx, item); err != nil {
		return Template{}, fmt.Errorf("version marketplace template: %w", err)
	}
	return item, nil
}

func (s *Service) Install(ctx context.Context, id, organizationID, userID string) (Installation, error) {
	item, err := s.repo.Get(ctx, id)
	if err != nil {
		return Installation{}, fmt.Errorf("get marketplace template: %w", err)
	}
	if item.Status != StatusPublished || s.installer == nil {
		return Installation{}, ErrInvalidState
	}
	if item.PriceCents > 0 {
		paid, purchaseErr := s.repo.HasPaidPurchase(ctx, organizationID, item.ID)
		if purchaseErr != nil {
			return Installation{}, fmt.Errorf("check marketplace purchase: %w", purchaseErr)
		}
		if !paid {
			return Installation{}, ErrPaymentRequired
		}
	}
	return s.install(ctx, item, organizationID, userID)
}

func (s *Service) install(ctx context.Context, item Template, organizationID, userID string) (Installation, error) {
	journeyID, err := s.installer.InstallMarketplaceTemplate(ctx, organizationID, userID, item.Name, item.Description, item.Steps)
	if err != nil {
		return Installation{}, fmt.Errorf("install journey template: %w", err)
	}
	installation := Installation{
		ID: uuid.NewString(), TemplateID: item.ID, TemplateVersion: item.Version,
		OrganizationID: organizationID, JourneyTemplateID: journeyID, InstalledBy: userID, InstalledAt: time.Now().UTC(),
	}
	if err := s.repo.CreateInstallation(ctx, installation); err != nil {
		return Installation{}, fmt.Errorf("record installation: %w", err)
	}
	item.InstallationCount++
	item.UpdatedAt = time.Now().UTC()
	if err := s.repo.Update(ctx, item); err != nil {
		return Installation{}, fmt.Errorf("update installation count: %w", err)
	}
	return installation, nil
}

func (s *Service) BeginPurchase(
	ctx context.Context,
	id, organizationID, userID string,
) (Checkout, error) {
	item, err := s.repo.Get(ctx, id)
	if err != nil {
		return Checkout{}, fmt.Errorf("get marketplace template: %w", err)
	}
	if item.Status != StatusPublished || item.PriceCents <= 0 || s.payments == nil {
		return Checkout{}, ErrInvalidState
	}
	if s.buyerEmail == nil {
		return Checkout{}, ErrInvalidState
	}
	email, err := s.buyerEmail(ctx, userID)
	if err != nil {
		return Checkout{}, fmt.Errorf("load marketplace buyer: %w", err)
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if !strings.Contains(email, "@") {
		return Checkout{}, ErrInvalidInput
	}
	if item.SubmittedByOrganizationID == organizationID {
		return Checkout{}, ErrInvalidState
	}
	now := time.Now().UTC()
	reference := "marketplace-" + uuid.NewString()
	fee := item.PriceCents * 15 / 100
	purchase := Purchase{
		ID: uuid.NewString(), TemplateID: item.ID, OrganizationID: organizationID,
		BuyerUserID: userID, SellerOrganizationID: item.SubmittedByOrganizationID,
		AmountCents: item.PriceCents, Currency: item.Currency,
		PlatformFeeCents: fee, SellerEarningsCents: item.PriceCents - fee,
		Reference: reference, Status: PurchasePending, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.repo.CreatePurchase(ctx, purchase); err != nil {
		return Checkout{}, fmt.Errorf("create marketplace purchase: %w", err)
	}
	session, err := s.payments.InitializeTransaction(ctx, billing.InitializeTransactionInput{
		Email: email, AmountCents: purchase.AmountCents, Currency: purchase.Currency,
		Reference: reference, CallbackURL: s.callbackURL,
		Metadata: map[string]string{"kind": "marketplace_template", "templateId": item.ID, "organizationId": organizationID},
	})
	if err != nil {
		return Checkout{}, fmt.Errorf("initialize marketplace payment: %w", err)
	}
	return Checkout{AuthorizationURL: session.AuthorizationURL, Reference: reference, Purchase: purchase}, nil
}

func (s *Service) CompletePurchase(
	ctx context.Context,
	reference, organizationID, userID string,
) (Installation, error) {
	if s.payments == nil {
		return Installation{}, ErrInvalidState
	}
	purchase, err := s.repo.GetPurchaseByReference(ctx, organizationID, strings.TrimSpace(reference))
	if err != nil {
		return Installation{}, fmt.Errorf("get marketplace purchase: %w", err)
	}
	item, err := s.repo.Get(ctx, purchase.TemplateID)
	if err != nil {
		return Installation{}, fmt.Errorf("get marketplace template: %w", err)
	}
	if purchase.Status == PurchasePaid && purchase.InstallationID != "" {
		return Installation{
			ID: purchase.InstallationID, TemplateID: item.ID, TemplateVersion: item.Version,
			OrganizationID: organizationID, JourneyTemplateID: purchase.JourneyTemplateID,
			InstalledBy: userID, InstalledAt: purchase.UpdatedAt,
		}, nil
	}
	if purchase.Status != PurchasePaid {
		verified, verifyErr := s.payments.VerifyTransaction(ctx, purchase.Reference)
		if verifyErr != nil {
			return Installation{}, fmt.Errorf("verify marketplace payment: %w", verifyErr)
		}
		if !verified.Paid || verified.AmountCents != purchase.AmountCents ||
			!strings.EqualFold(verified.Currency, purchase.Currency) {
			return Installation{}, ErrPaymentRequired
		}
		now := time.Now().UTC()
		purchase.Status, purchase.PaidAt, purchase.UpdatedAt = PurchasePaid, &now, now
		if err := s.repo.UpdatePurchase(ctx, purchase); err != nil {
			return Installation{}, fmt.Errorf("record marketplace payment: %w", err)
		}
	}
	installation, err := s.install(ctx, item, organizationID, userID)
	if err != nil {
		return Installation{}, err
	}
	purchase.InstallationID = installation.ID
	purchase.JourneyTemplateID = installation.JourneyTemplateID
	purchase.UpdatedAt = time.Now().UTC()
	if err := s.repo.UpdatePurchase(ctx, purchase); err != nil {
		return Installation{}, fmt.Errorf("link marketplace installation: %w", err)
	}
	return installation, nil
}

func (s *Service) Rate(ctx context.Context, id, organizationID, userID string, score int) (Template, error) {
	if score < 1 || score > 5 {
		return Template{}, ErrInvalidInput
	}
	item, err := s.repo.Get(ctx, id)
	if err != nil {
		return Template{}, fmt.Errorf("get marketplace template: %w", err)
	}
	if item.Status != StatusPublished {
		return Template{}, ErrInvalidState
	}
	now := time.Now().UTC()
	if err := s.repo.UpsertRating(ctx, Rating{ID: uuid.NewString(), TemplateID: id, OrganizationID: organizationID, UserID: userID, Score: score, CreatedAt: now, UpdatedAt: now}); err != nil {
		return Template{}, fmt.Errorf("rate marketplace template: %w", err)
	}
	ratings, err := s.repo.ListRatings(ctx, id)
	if err != nil {
		return Template{}, fmt.Errorf("list ratings: %w", err)
	}
	var total int
	for _, rating := range ratings {
		total += rating.Score
	}
	item.RatingCount = int64(len(ratings))
	item.RatingAverage = float64(total) / float64(len(ratings))
	item.UpdatedAt = now
	if err := s.repo.Update(ctx, item); err != nil {
		return Template{}, fmt.Errorf("update rating summary: %w", err)
	}
	return item, nil
}

func slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(value, "-")
	return strings.Trim(value, "-") + "-" + uuid.NewString()[:8]
}
