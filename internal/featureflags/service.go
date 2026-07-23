package featureflags

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
)

// OrganizationReader loads tenant organizations for flag resolution.
type OrganizationReader interface {
	PlanCode(ctx context.Context, organizationID string) (string, error)
}

// Service implements feature flag use cases.
type Service struct {
	repo Repository
	orgs OrganizationReader
}

// NewService constructs a Service.
func NewService(repo Repository, orgs OrganizationReader) *Service {
	return &Service{repo: repo, orgs: orgs}
}

// SeedDefaults upserts built-in feature flags.
func (s *Service) SeedDefaults(ctx context.Context) error {
	now := time.Now().UTC()

	defaults := []Flag{
		{
			Key:         flagKeyAIAssistant,
			Description: "AI assistant for HR workflows",
			Enabled:     false,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			Key:         flagKeySlack,
			Description: "Slack workspace integrations",
			Enabled:     false,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			Key:         flagKeySSO,
			Description: "Single sign-on authentication",
			Enabled:     false,
			PlanCodes:   []string{planCodeEnterprise},
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}

	for _, flag := range defaults {
		existing, err := s.repo.GetFlag(ctx, flag.Key)
		if err == nil {
			flag.CreatedAt = existing.CreatedAt
		}

		if err := s.repo.UpsertFlag(ctx, flag); err != nil {
			return fmt.Errorf("seed feature flag %q: %w", flag.Key, err)
		}
	}

	return nil
}

// ListFlags returns all global flags.
func (s *Service) ListFlags(ctx context.Context) ([]Flag, error) {
	items, err := s.repo.ListFlags(ctx)
	if err != nil {
		return nil, fmt.Errorf("list feature flags: %w", err)
	}

	return items, nil
}

// CreateFlag registers a new global flag.
func (s *Service) CreateFlag(ctx context.Context, in CreateFlagInput) (Flag, error) {
	key := strings.TrimSpace(in.Key)
	description := strings.TrimSpace(in.Description)

	if key == "" {
		return Flag{}, ErrInvalidInput
	}

	now := time.Now().UTC()

	flag := Flag{
		Key:         key,
		Description: description,
		Enabled:     in.Enabled,
		PlanCodes:   normalizePlanCodes(in.PlanCodes),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.repo.CreateFlag(ctx, flag); err != nil {
		return Flag{}, fmt.Errorf("create feature flag: %w", err)
	}

	return flag, nil
}

// UpdateFlag patches a global flag.
func (s *Service) UpdateFlag(ctx context.Context, key string, in UpdateFlagInput) (Flag, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return Flag{}, ErrInvalidInput
	}

	flag, err := s.repo.GetFlag(ctx, key)
	if err != nil {
		return Flag{}, fmt.Errorf("get feature flag: %w", err)
	}

	if in.Description != nil {
		flag.Description = strings.TrimSpace(*in.Description)
	}

	if in.Enabled != nil {
		flag.Enabled = *in.Enabled
	}

	if in.PlanCodes != nil {
		flag.PlanCodes = normalizePlanCodes(*in.PlanCodes)
	}

	flag.UpdatedAt = time.Now().UTC()
	if err := s.repo.UpdateFlag(ctx, flag); err != nil {
		return Flag{}, fmt.Errorf("update feature flag: %w", err)
	}

	return flag, nil
}

// SetOverride upserts a tenant-specific override.
func (s *Service) SetOverride(ctx context.Context, in SetOverrideInput) (Override, error) {
	key := strings.TrimSpace(in.Key)
	organizationID := strings.TrimSpace(in.OrganizationID)

	if key == "" || organizationID == "" || strings.TrimSpace(in.UpdatedBy) == "" {
		return Override{}, ErrInvalidInput
	}

	if _, err := s.repo.GetFlag(ctx, key); err != nil {
		return Override{}, fmt.Errorf("validate feature flag: %w", err)
	}

	override := Override{
		ID:             uuid.NewString(),
		OrganizationID: organizationID,
		Key:            key,
		Enabled:        in.Enabled,
		UpdatedAt:      time.Now().UTC(),
		UpdatedBy:      strings.TrimSpace(in.UpdatedBy),
	}
	if err := s.repo.UpsertOverride(ctx, override); err != nil {
		return Override{}, fmt.Errorf("set feature flag override: %w", err)
	}

	saved, err := s.repo.GetOverride(ctx, organizationID, key)
	if err != nil {
		return Override{}, fmt.Errorf("load feature flag override: %w", err)
	}

	return saved, nil
}

// Resolve returns effective flag values for a tenant and plan.
func (s *Service) Resolve(ctx context.Context, organizationID, planCode string) (map[string]bool, error) {
	flags, err := s.repo.ListFlags(ctx)
	if err != nil {
		return nil, fmt.Errorf("list feature flags: %w", err)
	}

	overrides, err := s.repo.ListOverridesByOrganization(ctx, organizationID)
	if err != nil {
		return nil, fmt.Errorf("list feature flag overrides: %w", err)
	}

	overrideByKey := make(map[string]Override, len(overrides))
	for _, item := range overrides {
		overrideByKey[item.Key] = item
	}

	planCode = strings.TrimSpace(planCode)
	if planCode == "" {
		resolvedPlan, planErr := s.orgs.PlanCode(ctx, organizationID)
		if planErr != nil {
			return nil, fmt.Errorf("resolve organization plan: %w", planErr)
		}

		planCode = resolvedPlan
	}

	out := make(map[string]bool, len(flags))
	for _, flag := range flags {
		out[flag.Key] = resolveFlag(flag, planCode, overrideByKey[flag.Key])
	}

	return out, nil
}

func resolveFlag(flag Flag, planCode string, override Override) bool {
	if override.OrganizationID != "" {
		return override.Enabled
	}

	enabled := flag.Enabled
	if len(flag.PlanCodes) > 0 {
		if !slices.Contains(flag.PlanCodes, planCode) {
			return false
		}
	}

	return enabled
}

func normalizePlanCodes(codes []string) []string {
	if len(codes) == 0 {
		return nil
	}

	out := make([]string, 0, len(codes))
	for _, code := range codes {
		code = strings.TrimSpace(code)
		if code != "" {
			out = append(out, code)
		}
	}

	if len(out) == 0 {
		return nil
	}

	return out
}
