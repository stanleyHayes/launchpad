package journeys_test

import (
	"context"
	"errors"
	"testing"

	"launchpad/internal/journeys"
)

func TestAddStepAssessmentRequiresAssessmentID(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	repo.templates[testTemplateID] = journeys.Template{
		ID: testTemplateID, OrganizationID: testOrgID, Status: "draft", CurrentVersion: 1,
	}

	svc := journeys.NewService(repo)

	_, err := svc.AddStep(context.Background(), testOrgID, testTemplateID, journeys.AddStepInput{
		StepType: "assessment",
		Title:    "Security assessment",
	})
	if !errors.Is(err, journeys.ErrInvalidInput) {
		t.Fatalf("got %v want %v", err, journeys.ErrInvalidInput)
	}
}

func TestAddStepAssessmentAcceptsValidConfig(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	repo.templates[testTemplateID] = journeys.Template{
		ID: testTemplateID, OrganizationID: testOrgID, Status: "draft", CurrentVersion: 1,
	}

	svc := journeys.NewService(repo)

	step, err := svc.AddStep(context.Background(), testOrgID, testTemplateID, journeys.AddStepInput{
		StepType: "assessment",
		Title:    "Security assessment",
		Config:   map[string]any{"assessmentId": "assessment-1"},
	})
	if err != nil {
		t.Fatalf("AddStep: %v", err)
	}

	if got := journeys.AssessmentIDFromConfig(step.Config); got != "assessment-1" {
		t.Fatalf("unexpected stored assessment config: %+v", step.Config)
	}
}

func TestAssessmentIDFromConfigTrimsAndDefaults(t *testing.T) {
	t.Parallel()

	if got := journeys.AssessmentIDFromConfig(nil); got != "" {
		t.Fatalf("expected empty id for nil config, got %q", got)
	}

	if got := journeys.AssessmentIDFromConfig(map[string]any{"assessmentId": 42}); got != "" {
		t.Fatalf("expected empty id for non-string value, got %q", got)
	}

	if got := journeys.AssessmentIDFromConfig(map[string]any{"assessmentId": "  a-1 "}); got != "a-1" {
		t.Fatalf("expected trimmed id, got %q", got)
	}
}
