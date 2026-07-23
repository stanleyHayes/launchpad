package billing

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// OrganizationReader loads tenant organizations for billing.
type OrganizationReader interface {
	Get(ctx context.Context, id string) (OrganizationSummary, error)
}

// OrgPlanUpdater updates an organization plan code.
type OrgPlanUpdater interface {
	SetPlanCode(ctx context.Context, id, planCode string) (OrganizationSummary, error)
}

// OrganizationSummary is the organization data billing needs.
type OrganizationSummary struct {
	ID       string
	PlanCode string
	Status   string
}

// Service implements billing use cases.
type Service struct {
	repo  Repository
	orgs  OrganizationReader
	plans OrgPlanUpdater
}

// NewService constructs a Service.
func NewService(repo Repository, orgs OrganizationReader, plans OrgPlanUpdater) *Service {
	return &Service{repo: repo, orgs: orgs, plans: plans}
}

// SeedDefaults upserts built-in billing plans.
func (s *Service) SeedDefaults(ctx context.Context) error {
	now := time.Now().UTC()

	defaults := []Plan{
		{
			Code:              planStarter,
			Name:              "Starter",
			Description:       "Free tier for small teams",
			PriceMonthlyCents: 0,
			Currency:          currencyUSD,
			Features:          []string{featureCoreOnboarding},
			Active:            true,
			CreatedAt:         now,
			UpdatedAt:         now,
		},
		{
			Code:              planGrowth,
			Name:              "Growth",
			Description:       "For growing HR teams",
			PriceMonthlyCents: growthPriceMonthlyCents,
			Currency:          currencyUSD,
			Features:          []string{featureCoreOnboarding, featureAnalytics},
			Active:            true,
			CreatedAt:         now,
			UpdatedAt:         now,
		},
		{
			Code:              planEnterprise,
			Name:              "Enterprise",
			Description:       "Custom pricing and enterprise features",
			PriceMonthlyCents: 0,
			Currency:          currencyUSD,
			Features: []string{
				featureCoreOnboarding,
				featureAnalytics,
				featureSSO,
				featureSupportSLA,
			},
			Active:    true,
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	for _, plan := range defaults {
		existing, err := s.repo.GetPlan(ctx, plan.Code)
		if err == nil {
			plan.CreatedAt = existing.CreatedAt
		}

		if err := s.repo.UpsertPlan(ctx, plan); err != nil {
			return fmt.Errorf("seed billing plan %q: %w", plan.Code, err)
		}
	}

	return nil
}

// ListPlans returns billing plans.
func (s *Service) ListPlans(ctx context.Context, activeOnly bool) ([]Plan, error) {
	items, err := s.repo.ListPlans(ctx, activeOnly)
	if err != nil {
		return nil, fmt.Errorf("list billing plans: %w", err)
	}

	return items, nil
}

// CreatePlan registers a billing plan.
func (s *Service) CreatePlan(ctx context.Context, in CreatePlanInput) (Plan, error) {
	code := strings.TrimSpace(in.Code)
	name := strings.TrimSpace(in.Name)
	currency := strings.TrimSpace(in.Currency)

	if code == "" || name == "" {
		return Plan{}, ErrInvalidInput
	}

	if currency == "" {
		currency = currencyUSD
	}

	now := time.Now().UTC()

	plan := Plan{
		Code:              code,
		Name:              name,
		Description:       strings.TrimSpace(in.Description),
		PriceMonthlyCents: in.PriceMonthlyCents,
		Currency:          currency,
		Features:          in.Features,
		Active:            in.Active,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := s.repo.CreatePlan(ctx, plan); err != nil {
		return Plan{}, fmt.Errorf("create billing plan: %w", err)
	}

	return plan, nil
}

// UpdatePlan patches a billing plan.
func (s *Service) UpdatePlan(ctx context.Context, code string, in UpdatePlanInput) (Plan, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return Plan{}, ErrInvalidInput
	}

	plan, err := s.repo.GetPlan(ctx, code)
	if err != nil {
		return Plan{}, fmt.Errorf("get billing plan: %w", err)
	}

	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" {
			return Plan{}, ErrInvalidInput
		}

		plan.Name = name
	}

	if in.Description != nil {
		plan.Description = strings.TrimSpace(*in.Description)
	}

	if in.PriceMonthlyCents != nil {
		plan.PriceMonthlyCents = *in.PriceMonthlyCents
	}

	if in.Currency != nil {
		currency := strings.TrimSpace(*in.Currency)
		if currency == "" {
			return Plan{}, ErrInvalidInput
		}

		plan.Currency = currency
	}

	if in.Features != nil {
		plan.Features = *in.Features
	}

	if in.Active != nil {
		plan.Active = *in.Active
	}

	plan.UpdatedAt = time.Now().UTC()
	if err := s.repo.UpdatePlan(ctx, plan); err != nil {
		return Plan{}, fmt.Errorf("update billing plan: %w", err)
	}

	return plan, nil
}

// ListSubscriptions returns all subscriptions for platform review.
func (s *Service) ListSubscriptions(ctx context.Context) ([]Subscription, error) {
	items, err := s.repo.ListSubscriptions(ctx)
	if err != nil {
		return nil, fmt.Errorf("list billing subscriptions: %w", err)
	}

	return items, nil
}

// GetOrCreateSubscription loads or creates a subscription from organization data.
func (s *Service) GetOrCreateSubscription(ctx context.Context, organizationID string) (Subscription, error) {
	subscription, err := s.repo.GetSubscriptionByOrganization(ctx, organizationID)
	if err == nil {
		return subscription, nil
	}

	if !errors.Is(err, ErrNotFound) {
		return Subscription{}, fmt.Errorf("get billing subscription: %w", err)
	}

	org, err := s.orgs.Get(ctx, organizationID)
	if err != nil {
		return Subscription{}, fmt.Errorf("get organization: %w", err)
	}

	status := subscriptionStatusForOrg(org.Status)

	subscription = newSubscription(organizationID, org.PlanCode, status)
	if err := s.repo.CreateSubscription(ctx, subscription); err != nil {
		return Subscription{}, fmt.Errorf("create billing subscription: %w", err)
	}

	return subscription, nil
}

// SetOrganizationPlan updates org plan code and subscription.
func (s *Service) SetOrganizationPlan(ctx context.Context, in SetOrganizationPlanInput) (Subscription, error) {
	organizationID := strings.TrimSpace(in.OrganizationID)
	planCode := strings.TrimSpace(in.PlanCode)

	if organizationID == "" || planCode == "" {
		return Subscription{}, ErrInvalidInput
	}

	plan, err := s.repo.GetPlan(ctx, planCode)
	if err != nil {
		return Subscription{}, fmt.Errorf("validate billing plan: %w", err)
	}

	if !plan.Active {
		return Subscription{}, ErrInvalidInput
	}

	if _, err := s.plans.SetPlanCode(ctx, organizationID, planCode); err != nil {
		return Subscription{}, fmt.Errorf("set organization plan code: %w", err)
	}

	status, err := s.resolveSubscriptionStatus(ctx, organizationID, in.Status)
	if err != nil {
		return Subscription{}, err
	}

	return s.upsertOrganizationSubscription(ctx, organizationID, planCode, status)
}

func (s *Service) createOrganizationSubscription(
	ctx context.Context,
	organizationID, planCode, status string,
) (Subscription, error) {
	subscription := newSubscription(organizationID, planCode, status)
	if err := s.repo.CreateSubscription(ctx, subscription); err != nil {
		return Subscription{}, fmt.Errorf("create billing subscription: %w", err)
	}

	return subscription, nil
}

func (s *Service) resolveSubscriptionStatus(
	ctx context.Context,
	organizationID string,
	requestedStatus string,
) (string, error) {
	status := strings.TrimSpace(requestedStatus)
	if status != "" {
		if !isValidSubscriptionStatus(status) {
			return "", ErrInvalidInput
		}

		return status, nil
	}

	org, err := s.orgs.Get(ctx, organizationID)
	if err != nil {
		return "", fmt.Errorf("get organization: %w", err)
	}

	return subscriptionStatusForOrg(org.Status), nil
}

func (s *Service) upsertOrganizationSubscription(
	ctx context.Context,
	organizationID, planCode, status string,
) (Subscription, error) {
	subscription, err := s.repo.GetSubscriptionByOrganization(ctx, organizationID)
	if errors.Is(err, ErrNotFound) {
		return s.createOrganizationSubscription(ctx, organizationID, planCode, status)
	}

	if err != nil {
		return Subscription{}, fmt.Errorf("get billing subscription: %w", err)
	}

	subscription.PlanCode = planCode
	subscription.Status = status

	subscription.UpdatedAt = time.Now().UTC()
	if err := s.repo.UpdateSubscription(ctx, subscription); err != nil {
		return Subscription{}, fmt.Errorf("update billing subscription: %w", err)
	}

	return subscription, nil
}

func newSubscription(organizationID, planCode, status string) Subscription {
	now := time.Now().UTC()
	periodEnd := now.AddDate(0, 1, 0)

	return Subscription{
		ID:               uuid.NewString(),
		OrganizationID:   organizationID,
		PlanCode:         planCode,
		Status:           status,
		CurrentPeriodEnd: &periodEnd,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

func subscriptionStatusForOrg(orgStatus string) string {
	if orgStatus == "trial" {
		return statusTrialing
	}

	return statusActive
}

func isValidSubscriptionStatus(status string) bool {
	return status == statusTrialing ||
		status == statusActive ||
		status == statusPastDue ||
		status == statusCanceled
}
