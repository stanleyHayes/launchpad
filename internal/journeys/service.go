package journeys

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Service implements journey use cases.
type Service struct {
	repo Repository
}

// NewService constructs a Service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// CreateTemplate creates a draft journey template.
func (s *Service) CreateTemplate(
	ctx context.Context,
	organizationID string,
	in CreateTemplateInput,
) (Template, error) {
	name := strings.TrimSpace(in.Name)
	if organizationID == "" || name == "" || strings.TrimSpace(in.CreatedBy) == "" {
		return Template{}, ErrInvalidInput
	}

	now := time.Now().UTC()

	template := Template{
		ID:             uuid.NewString(),
		OrganizationID: organizationID,
		Name:           name,
		Description:    strings.TrimSpace(in.Description),
		Status:         statusDraft,
		CurrentVersion: 1,
		CreatedBy:      in.CreatedBy,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := s.repo.CreateTemplate(ctx, template); err != nil {
		return Template{}, fmt.Errorf("create journey template: %w", err)
	}

	return template, nil
}

// ListTemplates lists journey templates.
func (s *Service) ListTemplates(ctx context.Context, organizationID string) ([]Template, error) {
	if organizationID == "" {
		return nil, ErrInvalidInput
	}

	items, err := s.repo.ListTemplates(ctx, organizationID)
	if err != nil {
		return nil, fmt.Errorf("list journey templates: %w", err)
	}

	return items, nil
}

// GetTemplate returns one template.
func (s *Service) GetTemplate(ctx context.Context, organizationID, templateID string) (Template, error) {
	template, err := s.repo.GetTemplate(ctx, organizationID, templateID)
	if err != nil {
		return Template{}, fmt.Errorf("get journey template: %w", err)
	}

	return template, nil
}

// AddStep adds a step to the current draft version.
func (s *Service) AddStep(
	ctx context.Context,
	organizationID, templateID string,
	in AddStepInput,
) (Step, error) {
	template, err := s.repo.GetTemplate(ctx, organizationID, templateID)
	if err != nil {
		return Step{}, fmt.Errorf("get journey template: %w", err)
	}

	if template.Status != statusDraft {
		return Step{}, ErrNotDraft
	}

	title := strings.TrimSpace(in.Title)

	stepType := strings.TrimSpace(in.StepType)
	if title == "" || !isValidStepType(stepType) {
		return Step{}, ErrInvalidInput
	}

	count, err := s.repo.CountSteps(ctx, organizationID, templateID, template.CurrentVersion)
	if err != nil {
		return Step{}, err
	}

	config := in.Config
	if config == nil {
		config = map[string]any{}
	}

	step := Step{
		ID:                uuid.NewString(),
		OrganizationID:    organizationID,
		JourneyTemplateID: templateID,
		Version:           template.CurrentVersion,
		StepType:          stepType,
		Title:             title,
		Instructions:      strings.TrimSpace(in.Instructions),
		Position:          int(count) + 1,
		DueOffsetDays:     in.DueOffsetDays,
		Config:            config,
		CreatedAt:         time.Now().UTC(),
	}
	if err := s.repo.CreateStep(ctx, step); err != nil {
		return Step{}, fmt.Errorf("create journey step: %w", err)
	}

	template.UpdatedAt = time.Now().UTC()
	if err := s.repo.UpdateTemplate(ctx, template); err != nil {
		return Step{}, fmt.Errorf("touch journey template: %w", err)
	}

	return step, nil
}

// ListSteps lists steps for a template's current version.
func (s *Service) ListSteps(ctx context.Context, organizationID, templateID string) ([]Step, error) {
	template, err := s.repo.GetTemplate(ctx, organizationID, templateID)
	if err != nil {
		return nil, fmt.Errorf("get journey template: %w", err)
	}

	items, err := s.repo.ListSteps(ctx, organizationID, templateID, template.CurrentVersion)
	if err != nil {
		return nil, fmt.Errorf("list journey steps: %w", err)
	}

	return items, nil
}

// ListStepsForVersion lists steps for a specific published version.
func (s *Service) ListStepsForVersion(
	ctx context.Context,
	organizationID, templateID string,
	version int,
) ([]Step, error) {
	items, err := s.repo.ListSteps(ctx, organizationID, templateID, version)
	if err != nil {
		return nil, fmt.Errorf("list journey steps for version: %w", err)
	}

	return items, nil
}

// Publish marks a draft journey as published.
func (s *Service) Publish(ctx context.Context, organizationID, templateID string) (Template, error) {
	template, err := s.repo.GetTemplate(ctx, organizationID, templateID)
	if err != nil {
		return Template{}, fmt.Errorf("get journey template: %w", err)
	}

	if template.Status != statusDraft {
		return Template{}, ErrNotDraft
	}

	count, err := s.repo.CountSteps(ctx, organizationID, templateID, template.CurrentVersion)
	if err != nil {
		return Template{}, err
	}

	if count == 0 {
		return Template{}, ErrNoSteps
	}

	template.Status = statusPublished

	template.UpdatedAt = time.Now().UTC()
	if err := s.repo.UpdateTemplate(ctx, template); err != nil {
		return Template{}, fmt.Errorf("publish journey template: %w", err)
	}

	return template, nil
}

// RequirePublished returns a published template.
func (s *Service) RequirePublished(ctx context.Context, organizationID, templateID string) (Template, error) {
	template, err := s.repo.GetTemplate(ctx, organizationID, templateID)
	if err != nil {
		return Template{}, fmt.Errorf("get journey template: %w", err)
	}

	if template.Status != statusPublished {
		return Template{}, ErrNotPublished
	}

	return template, nil
}
