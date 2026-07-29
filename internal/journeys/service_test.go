package journeys_test

import (
	"context"
	"errors"
	"testing"

	"launchpad/internal/journeys"
)

const (
	testOrgID      = "org-1"
	testTemplateID = "journey-1"
)

// memoryRepo is an in-memory journeys.Repository.
type memoryRepo struct {
	templates map[string]journeys.Template
	steps     []journeys.Step
}

func newMemoryRepo() *memoryRepo {
	return &memoryRepo{templates: map[string]journeys.Template{}}
}

func (m *memoryRepo) EnsureIndexes(context.Context) error { return nil }

func (m *memoryRepo) CreateTemplate(_ context.Context, template journeys.Template) error {
	m.templates[template.ID] = template

	return nil
}

func (m *memoryRepo) GetTemplate(_ context.Context, organizationID, templateID string) (journeys.Template, error) {
	template, ok := m.templates[templateID]
	if !ok || template.OrganizationID != organizationID {
		return journeys.Template{}, journeys.ErrNotFound
	}

	return template, nil
}

func (m *memoryRepo) ListTemplates(context.Context, string) ([]journeys.Template, error) {
	return nil, nil
}

func (m *memoryRepo) UpdateTemplate(_ context.Context, template journeys.Template) error {
	m.templates[template.ID] = template

	return nil
}

func (m *memoryRepo) CreateStep(_ context.Context, step journeys.Step) error {
	for _, existing := range m.steps {
		if existing.OrganizationID == step.OrganizationID &&
			existing.JourneyTemplateID == step.JourneyTemplateID &&
			existing.Version == step.Version &&
			existing.Position == step.Position {
			return journeys.ErrStepPositionTaken
		}
	}

	m.steps = append(m.steps, step)

	return nil
}

func (m *memoryRepo) UpdateStep(_ context.Context, step journeys.Step) error {
	for i, existing := range m.steps {
		if existing.ID == step.ID && existing.OrganizationID == step.OrganizationID {
			m.steps[i] = step

			return nil
		}
	}

	return journeys.ErrStepNotFound
}

func (m *memoryRepo) DeleteStep(
	_ context.Context,
	organizationID, templateID string,
	version int,
	stepID string,
) error {
	for i, existing := range m.steps {
		if existing.ID == stepID &&
			existing.OrganizationID == organizationID &&
			existing.JourneyTemplateID == templateID &&
			existing.Version == version {
			m.steps = append(m.steps[:i], m.steps[i+1:]...)

			return nil
		}
	}

	return journeys.ErrStepNotFound
}

func (m *memoryRepo) ListSteps(context.Context, string, string, int) ([]journeys.Step, error) {
	return m.steps, nil
}

func (m *memoryRepo) CountSteps(_ context.Context, organizationID, templateID string, version int) (int64, error) {
	count := int64(0)

	for _, step := range m.steps {
		if step.OrganizationID == organizationID &&
			step.JourneyTemplateID == templateID &&
			step.Version == version {
			count++
		}
	}

	return count, nil
}

func TestAddStepRejectsNegativeDueOffset(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	repo.templates[testTemplateID] = journeys.Template{
		ID: testTemplateID, OrganizationID: testOrgID, Status: "draft", CurrentVersion: 1,
	}

	svc := journeys.NewService(repo)

	_, err := svc.AddStep(context.Background(), testOrgID, testTemplateID, journeys.AddStepInput{
		StepType:      "task",
		Title:         "Sign paperwork",
		DueOffsetDays: -1,
	})
	if !errors.Is(err, journeys.ErrInvalidInput) {
		t.Fatalf("got %v want %v", err, journeys.ErrInvalidInput)
	}
}

func TestAddStepAssignsSequentialPositions(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	repo.templates[testTemplateID] = journeys.Template{
		ID: testTemplateID, OrganizationID: testOrgID, Status: "draft", CurrentVersion: 1,
	}

	svc := journeys.NewService(repo)

	first, err := svc.AddStep(context.Background(), testOrgID, testTemplateID, journeys.AddStepInput{
		StepType: "document",
		Title:    "Handbook",
	})
	if err != nil {
		t.Fatalf("AddStep first: %v", err)
	}

	second, err := svc.AddStep(context.Background(), testOrgID, testTemplateID, journeys.AddStepInput{
		StepType:      "task",
		Title:         "Sign paperwork",
		DueOffsetDays: 3,
	})
	if err != nil {
		t.Fatalf("AddStep second: %v", err)
	}

	if first.Position != 1 || second.Position != 2 {
		t.Fatalf("positions = %d, %d; want 1, 2", first.Position, second.Position)
	}
}
