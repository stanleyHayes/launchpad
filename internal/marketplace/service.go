package marketplace

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Service struct {
	repo      Repository
	installer JourneyInstaller
}

func NewService(repo Repository, installer JourneyInstaller) *Service {
	return &Service{repo: repo, installer: installer}
}

func (s *Service) Create(ctx context.Context, in CreateInput) (Template, error) {
	name, category := strings.TrimSpace(in.Name), strings.TrimSpace(in.Category)
	if name == "" || category == "" || len(in.Steps) == 0 || strings.TrimSpace(in.CreatedBy) == "" {
		return Template{}, ErrInvalidInput
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
		Steps:                     in.Steps, CreatedBy: strings.TrimSpace(in.CreatedBy), CreatedAt: now, UpdatedAt: now,
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
