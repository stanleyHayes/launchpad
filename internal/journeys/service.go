package journeys

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
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
	if title == "" || !isValidStepType(stepType) || in.DueOffsetDays < 0 {
		return Step{}, ErrInvalidInput
	}

	if stepType == stepTypeQuiz {
		if err := ValidateQuizConfig(in.Config); err != nil {
			return Step{}, err
		}
	}

	if stepType == stepTypeAssessment {
		if err := ValidateAssessmentConfig(in.Config); err != nil {
			return Step{}, err
		}
	}
	if err := validateWorkflowConfig(in.Config, templateID); err != nil {
		return Step{}, err
	}

	if len(in.PrerequisiteStepIDs) > 0 || configConditionStepID(in.Config) != "" {
		existing, listErr := s.repo.ListSteps(ctx, organizationID, templateID, template.CurrentVersion)
		if listErr != nil {
			return Step{}, listErr
		}
		valid := make(map[string]bool, len(existing))
		for _, candidate := range existing {
			valid[candidate.ID] = true
		}
		seen := make(map[string]bool, len(in.PrerequisiteStepIDs))
		for _, prerequisite := range in.PrerequisiteStepIDs {
			if !valid[prerequisite] || seen[prerequisite] {
				return Step{}, ErrInvalidInput
			}
			seen[prerequisite] = true
		}
		if conditionStepID := configConditionStepID(in.Config); conditionStepID != "" && !valid[conditionStepID] {
			return Step{}, ErrInvalidInput
		}
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
		ID:                  uuid.NewString(),
		OrganizationID:      organizationID,
		JourneyTemplateID:   templateID,
		Version:             template.CurrentVersion,
		StepType:            stepType,
		Title:               title,
		Instructions:        strings.TrimSpace(in.Instructions),
		Position:            int(count) + 1,
		DueOffsetDays:       in.DueOffsetDays,
		BusinessDays:        in.BusinessDays,
		Stage:               strings.TrimSpace(in.Stage),
		ParallelGroup:       strings.TrimSpace(in.ParallelGroup),
		PrerequisiteStepIDs: append([]string(nil), in.PrerequisiteStepIDs...),
		Locale:              strings.TrimSpace(in.Locale),
		Config:              config,
		CreatedAt:           time.Now().UTC(),
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

func validateWorkflowConfig(config map[string]any, templateID string) error {
	if config == nil {
		return nil
	}
	if raw, ok := config["maxAttempts"]; ok {
		value, valid := numericInt(raw)
		if !valid || value < 0 {
			return ErrInvalidInput
		}
	}
	if raw, ok := config["subflowTemplateId"]; ok {
		value, valid := raw.(string)
		if !valid || strings.TrimSpace(value) == "" || strings.TrimSpace(value) == templateID {
			return ErrInvalidInput
		}
	}
	if raw, ok := config["condition"]; ok {
		encoded, err := json.Marshal(raw)
		if err != nil {
			return ErrInvalidInput
		}
		var condition struct {
			StepID string `json:"stepId"`
			Field  string `json:"field"`
			Equals any    `json:"equals"`
		}
		if json.Unmarshal(encoded, &condition) != nil ||
			strings.TrimSpace(condition.StepID) == "" || strings.TrimSpace(condition.Field) == "" ||
			condition.Equals == nil {
			return ErrInvalidInput
		}
	}
	return nil
}

func configConditionStepID(config map[string]any) string {
	raw, ok := config["condition"]
	if !ok {
		return ""
	}
	encoded, _ := json.Marshal(raw)
	var condition struct {
		StepID string `json:"stepId"`
	}
	_ = json.Unmarshal(encoded, &condition)
	return strings.TrimSpace(condition.StepID)
}

func numericInt(raw any) (int, bool) {
	switch value := raw.(type) {
	case int:
		return value, true
	case int32:
		return int(value), true
	case int64:
		return int(value), true
	case float64:
		return int(value), value == float64(int(value))
	default:
		return 0, false
	}
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

// DeleteStep removes a step from the current draft version and renumbers the
// remaining steps so positions stay a contiguous 1..N sequence.
func (s *Service) DeleteStep(ctx context.Context, organizationID, templateID, stepID string) error {
	template, err := s.repo.GetTemplate(ctx, organizationID, templateID)
	if err != nil {
		return fmt.Errorf("get journey template: %w", err)
	}

	if template.Status != statusDraft {
		return ErrNotDraft
	}

	steps, err := s.repo.ListSteps(ctx, organizationID, templateID, template.CurrentVersion)
	if err != nil {
		return fmt.Errorf("list journey steps: %w", err)
	}

	remaining := make([]Step, 0, len(steps))
	found := false

	for _, step := range steps {
		if step.ID == stepID {
			found = true

			continue
		}

		remaining = append(remaining, step)
	}

	if !found {
		return ErrStepNotFound
	}

	if err := s.repo.DeleteStep(ctx, organizationID, templateID, template.CurrentVersion, stepID); err != nil {
		return fmt.Errorf("delete journey step: %w", err)
	}

	for i, step := range remaining {
		if step.Position == i+1 {
			continue
		}

		step.Position = i + 1
		if err := s.repo.UpdateStep(ctx, step); err != nil {
			return fmt.Errorf("renumber journey step: %w", err)
		}
	}

	template.UpdatedAt = time.Now().UTC()
	if err := s.repo.UpdateTemplate(ctx, template); err != nil {
		return fmt.Errorf("touch journey template: %w", err)
	}

	return nil
}

// CreateNewVersion starts a new draft version of a published template by
// copying the published steps into the next version. The published steps are
// left untouched, so assignments pinned to older versions keep working.
func (s *Service) CreateNewVersion(ctx context.Context, organizationID, templateID string) (Template, error) {
	template, err := s.repo.GetTemplate(ctx, organizationID, templateID)
	if err != nil {
		return Template{}, fmt.Errorf("get journey template: %w", err)
	}

	if template.Status != statusPublished {
		return Template{}, ErrNotPublished
	}

	sourceVersion := template.CurrentVersion
	if err := s.copySteps(ctx, organizationID, template.ID, sourceVersion, template.ID, sourceVersion+1); err != nil {
		return Template{}, err
	}

	template.CurrentVersion++
	template.Status = statusDraft
	template.UpdatedAt = time.Now().UTC()

	if err := s.repo.UpdateTemplate(ctx, template); err != nil {
		return Template{}, fmt.Errorf("start new journey version: %w", err)
	}

	return template, nil
}

// CloneTemplate creates a new draft template with the same steps as the
// source template's current version.
func (s *Service) CloneTemplate(ctx context.Context, organizationID, templateID, createdBy string) (Template, error) {
	source, err := s.repo.GetTemplate(ctx, organizationID, templateID)
	if err != nil {
		return Template{}, fmt.Errorf("get journey template: %w", err)
	}

	now := time.Now().UTC()

	clone := Template{
		ID:             uuid.NewString(),
		OrganizationID: organizationID,
		Name:           source.Name + " (copy)",
		Description:    source.Description,
		Status:         statusDraft,
		CurrentVersion: 1,
		CreatedBy:      createdBy,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := s.repo.CreateTemplate(ctx, clone); err != nil {
		return Template{}, fmt.Errorf("clone journey template: %w", err)
	}

	if err := s.copySteps(ctx, organizationID, source.ID, source.CurrentVersion, clone.ID, 1); err != nil {
		return Template{}, err
	}

	return clone, nil
}

// ImportTemplate creates an independent draft journey from a curated
// marketplace snapshot.
func (s *Service) ImportTemplate(
	ctx context.Context,
	organizationID, createdBy, name, description string,
	steps []ImportStep,
) (Template, error) {
	if len(steps) == 0 {
		return Template{}, ErrInvalidInput
	}
	template, err := s.CreateTemplate(ctx, organizationID, CreateTemplateInput{
		Name: name, Description: description, CreatedBy: createdBy,
	})
	if err != nil {
		return Template{}, err
	}
	for _, step := range steps {
		if _, err := s.AddStep(ctx, organizationID, template.ID, AddStepInput{
			StepType: step.StepType, Title: step.Title, Instructions: step.Instructions,
			DueOffsetDays: step.DueOffsetDays, BusinessDays: step.BusinessDays,
			Stage: step.Stage, ParallelGroup: step.ParallelGroup,
			PrerequisiteStepIDs: step.PrerequisiteStepIDs, Locale: step.Locale, Config: step.Config,
		}); err != nil {
			return Template{}, fmt.Errorf("import marketplace step: %w", err)
		}
	}
	return template, nil
}

// Rollback publishes an older version as current by copying its steps into a
// new version. All historical versions keep their steps, so assignments
// pinned to any previous version keep working. Rolling back to the current
// version is a no-op.
func (s *Service) Rollback(ctx context.Context, organizationID, templateID string, version int) (Template, error) {
	template, err := s.repo.GetTemplate(ctx, organizationID, templateID)
	if err != nil {
		return Template{}, fmt.Errorf("get journey template: %w", err)
	}

	if template.Status != statusPublished {
		return Template{}, ErrNotPublished
	}

	if version < 1 || version > template.CurrentVersion {
		return Template{}, ErrInvalidInput
	}

	if version == template.CurrentVersion {
		return template, nil
	}

	count, err := s.repo.CountSteps(ctx, organizationID, templateID, version)
	if err != nil {
		return Template{}, err
	}

	if count == 0 {
		return Template{}, ErrInvalidInput
	}

	if err := s.copySteps(ctx, organizationID, template.ID, version, template.ID, template.CurrentVersion+1); err != nil {
		return Template{}, err
	}

	template.CurrentVersion++
	template.UpdatedAt = time.Now().UTC()

	if err := s.repo.UpdateTemplate(ctx, template); err != nil {
		return Template{}, fmt.Errorf("roll back journey template: %w", err)
	}

	return template, nil
}

// ListVersions lists every version of a template with its status and step
// count. All versions before the current one are published; the current
// version carries the template's status.
func (s *Service) ListVersions(ctx context.Context, organizationID, templateID string) ([]VersionSummary, error) {
	template, err := s.repo.GetTemplate(ctx, organizationID, templateID)
	if err != nil {
		return nil, fmt.Errorf("get journey template: %w", err)
	}

	versions := make([]VersionSummary, 0, template.CurrentVersion)

	for version := 1; version <= template.CurrentVersion; version++ {
		count, err := s.repo.CountSteps(ctx, organizationID, templateID, version)
		if err != nil {
			return nil, err
		}

		status := statusPublished
		if version == template.CurrentVersion {
			status = template.Status
		}

		versions = append(versions, VersionSummary{Version: version, Status: status, StepCount: count})
	}

	return versions, nil
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

// copySteps copies every step of one template version into another template
// version, assigning fresh step IDs.
func (s *Service) copySteps(
	ctx context.Context,
	organizationID, sourceTemplateID string,
	sourceVersion int,
	targetTemplateID string,
	targetVersion int,
) error {
	steps, err := s.repo.ListSteps(ctx, organizationID, sourceTemplateID, sourceVersion)
	if err != nil {
		return fmt.Errorf("list journey steps: %w", err)
	}

	for _, step := range steps {
		config := make(map[string]any, len(step.Config))
		maps.Copy(config, step.Config)

		copied := Step{
			ID:                  uuid.NewString(),
			OrganizationID:      organizationID,
			JourneyTemplateID:   targetTemplateID,
			Version:             targetVersion,
			StepType:            step.StepType,
			Title:               step.Title,
			Instructions:        step.Instructions,
			Position:            step.Position,
			DueOffsetDays:       step.DueOffsetDays,
			BusinessDays:        step.BusinessDays,
			Stage:               step.Stage,
			ParallelGroup:       step.ParallelGroup,
			PrerequisiteStepIDs: append([]string(nil), step.PrerequisiteStepIDs...),
			Locale:              step.Locale,
			Config:              config,
			CreatedAt:           time.Now().UTC(),
		}
		if err := s.repo.CreateStep(ctx, copied); err != nil {
			return fmt.Errorf("copy journey step: %w", err)
		}
	}

	return nil
}
