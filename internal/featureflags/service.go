package featureflags

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
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

// SeedDefaults inserts built-in feature flags that are missing. Existing flags
// are left untouched so admin changes survive restarts.
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
		if _, err := s.repo.GetFlag(ctx, flag.Key); err == nil {
			continue
		} else if !errors.Is(err, ErrNotFound) {
			return fmt.Errorf("get feature flag %q: %w", flag.Key, err)
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
	if in.RolloutPercentage < 0 || in.RolloutPercentage > 100 {
		return Flag{}, ErrInvalidInput
	}

	now := time.Now().UTC()

	flag := Flag{
		Key:               key,
		Description:       description,
		Enabled:           in.Enabled,
		PlanCodes:         normalizePlanCodes(in.PlanCodes),
		RolloutPercentage: normalizeRolloutPercentage(in.RolloutPercentage),
		CohortUserIDs:     normalizeValues(in.CohortUserIDs),
		ExpiresAt:         normalizeExpiry(in.ExpiresAt),
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := s.repo.CreateFlag(ctx, flag); err != nil {
		return Flag{}, fmt.Errorf("create feature flag: %w", err)
	}
	if err := s.appendHistory(ctx, flag, "created", in.ActorUserID, ""); err != nil {
		return Flag{}, err
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
	if in.RolloutPercentage != nil {
		if *in.RolloutPercentage < 1 || *in.RolloutPercentage > 100 {
			return Flag{}, ErrInvalidInput
		}
		flag.RolloutPercentage = *in.RolloutPercentage
	}
	if in.CohortUserIDs != nil {
		flag.CohortUserIDs = normalizeValues(*in.CohortUserIDs)
	}
	if in.ExpiresAt != nil {
		flag.ExpiresAt = normalizeExpiry(*in.ExpiresAt)
	}

	flag.UpdatedAt = time.Now().UTC()
	if err := s.repo.UpdateFlag(ctx, flag); err != nil {
		return Flag{}, fmt.Errorf("update feature flag: %w", err)
	}
	if err := s.appendHistory(ctx, flag, "updated", in.UpdatedBy, ""); err != nil {
		return Flag{}, err
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
	flag, err := s.repo.GetFlag(ctx, key)
	if err != nil {
		return Override{}, fmt.Errorf("load feature flag for history: %w", err)
	}
	if err := s.appendHistory(ctx, flag, "override_set", in.UpdatedBy, organizationID); err != nil {
		return Override{}, err
	}

	return saved, nil
}

// Resolve returns effective flag values for a tenant and plan.
func (s *Service) Resolve(
	ctx context.Context,
	organizationID, planCode string,
	userID ...string,
) (map[string]bool, error) {
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
	resolvedUserID := ""
	if len(userID) > 0 {
		resolvedUserID = strings.TrimSpace(userID[0])
	}
	for _, flag := range flags {
		out[flag.Key] = resolveFlag(flag, organizationID, planCode, resolvedUserID, overrideByKey[flag.Key])
	}

	return out, nil
}

func resolveFlag(flag Flag, organizationID, planCode, userID string, override Override) bool {
	if override.OrganizationID != "" {
		return override.Enabled
	}

	if !flag.Enabled {
		return false
	}
	if flag.ExpiresAt != nil && !time.Now().UTC().Before(*flag.ExpiresAt) {
		return false
	}
	if len(flag.PlanCodes) > 0 {
		if !slices.Contains(flag.PlanCodes, planCode) {
			return false
		}
	}

	if userID != "" && slices.Contains(flag.CohortUserIDs, userID) {
		return true
	}

	percentage := normalizeRolloutPercentage(flag.RolloutPercentage)
	if percentage >= 100 {
		return true
	}

	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(flag.Key + ":" + organizationID))

	return int(hasher.Sum32()%100) < percentage
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

func normalizeRolloutPercentage(value int) int {
	if value == 0 {
		return 100
	}
	return value
}

func normalizeValues(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !slices.Contains(out, value) {
			out = append(out, value)
		}
	}
	return out
}

func normalizeExpiry(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	normalized := value.UTC()
	return &normalized
}

func (s *Service) appendHistory(
	ctx context.Context,
	flag Flag,
	action, actorUserID, organizationID string,
) error {
	history := History{
		ID:             uuid.NewString(),
		Key:            flag.Key,
		Action:         action,
		ActorUserID:    strings.TrimSpace(actorUserID),
		OrganizationID: organizationID,
		Snapshot:       flag,
		CreatedAt:      time.Now().UTC(),
	}
	if err := s.repo.AppendHistory(ctx, history); err != nil {
		return fmt.Errorf("append feature flag history: %w", err)
	}
	return nil
}

// ListHistory returns the newest rollout mutations for a flag.
func (s *Service) ListHistory(ctx context.Context, key string, limit int64) ([]History, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, ErrInvalidInput
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	items, err := s.repo.ListHistory(ctx, key, limit)
	if err != nil {
		return nil, fmt.Errorf("list feature flag history: %w", err)
	}
	return items, nil
}
