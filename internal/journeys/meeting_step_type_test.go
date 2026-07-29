package journeys_test

import (
	"context"
	"testing"

	"launchpad/internal/journeys"
)

func TestAddStepAcceptsMeetingStepType(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	repo.templates[testTemplateID] = journeys.Template{
		ID: testTemplateID, OrganizationID: testOrgID, Status: "draft", CurrentVersion: 1,
	}

	svc := journeys.NewService(repo)

	step, err := svc.AddStep(context.Background(), testOrgID, testTemplateID, journeys.AddStepInput{
		StepType: "meeting",
		Title:    "Meet your manager",
	})
	if err != nil {
		t.Fatalf("AddStep meeting: %v", err)
	}

	if step.StepType != "meeting" {
		t.Fatalf("stepType = %q, want meeting", step.StepType)
	}
}
