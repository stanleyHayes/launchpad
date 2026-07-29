package journeys_test

import (
	"context"
	"testing"

	"launchpad/internal/journeys"
)

func TestAddStepAcceptsRequestStepTypes(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	repo.templates[testTemplateID] = journeys.Template{
		ID: testTemplateID, OrganizationID: testOrgID, Status: "draft", CurrentVersion: 1,
	}

	svc := journeys.NewService(repo)

	for _, stepType := range []string{"equipment_request", "access_request"} {
		step, err := svc.AddStep(context.Background(), testOrgID, testTemplateID, journeys.AddStepInput{
			StepType: stepType,
			Title:    "Request step",
		})
		if err != nil {
			t.Fatalf("AddStep %s: %v", stepType, err)
		}

		if step.StepType != stepType {
			t.Fatalf("stepType = %q, want %q", step.StepType, stepType)
		}
	}
}
