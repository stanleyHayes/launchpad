package assignments_test

import (
	"context"
	"errors"
	"testing"

	"launchpad/internal/assignments"
	"launchpad/internal/employees"
	"launchpad/internal/journeys"
)

// stubVerifier implements assignments.AssessmentVerifier with a fixed answer.
type stubVerifier struct {
	passed bool
}

func (s stubVerifier) LatestAttemptPassed(context.Context, string, string, string) (bool, error) {
	return s.passed, nil
}

func newAssessmentService(
	t *testing.T,
	verifier assignments.AssessmentVerifier,
) (*assignments.Service, *memoryRepo) {
	t.Helper()

	repo := newMemoryRepo()
	svc := assignments.NewService(
		repo,
		stubJourneys{
			template: journeys.Template{
				ID: "journey-1", Name: "Onboarding", Status: "published", CurrentVersion: 1,
			},
			steps: []journeys.Step{
				{
					ID:       "js-assessment",
					StepType: "assessment",
					Title:    "Security assessment",
					Position: 1,
					Config:   map[string]any{"assessmentId": "assessment-1"},
				},
			},
		},
		stubEmployees{
			byID: map[string]employees.Employee{
				testEmployeeID: {ID: testEmployeeID, UserID: testEmployeeUser},
			},
			byUserID: map[string]employees.Employee{
				testEmployeeUser: {ID: testEmployeeID, UserID: testEmployeeUser},
			},
		},
		nil,
	)
	svc.SetAssessmentVerifier(verifier)

	return svc, repo
}

func assignAssessmentJourney(t *testing.T, svc *assignments.Service) assignments.AssignResult {
	t.Helper()

	result, err := svc.Assign(context.Background(), testOrgID, "actor-user", assignments.AssignInput{
		EmployeeID:        testEmployeeID,
		JourneyTemplateID: "journey-1",
	})
	if err != nil {
		t.Fatalf("Assign: %v", err)
	}

	return result
}

func TestAssessmentStepSnapshotsAssessmentID(t *testing.T) {
	t.Parallel()

	svc, _ := newAssessmentService(t, stubVerifier{passed: true})
	result := assignAssessmentJourney(t, svc)

	if result.Steps[0].AssessmentID != "assessment-1" {
		t.Fatalf("expected snapshot assessmentId, got %+v", result.Steps[0])
	}
}

func TestAssessmentStepRequiresPassingAttempt(t *testing.T) {
	t.Parallel()

	svc, repo := newAssessmentService(t, stubVerifier{passed: false})
	result := assignAssessmentJourney(t, svc)

	_, err := svc.CompleteStep(
		context.Background(),
		testOrgID,
		testEmployeeUser,
		result.Steps[0].ID,
		assignments.CompleteStepInput{},
	)
	if !errors.Is(err, assignments.ErrInvalidState) {
		t.Fatalf("got %v want %v", err, assignments.ErrInvalidState)
	}

	if repo.steps[result.Steps[0].ID].Status != testStatusInProgress {
		t.Fatalf("expected step to stay in progress, got %s", repo.steps[result.Steps[0].ID].Status)
	}
}

func TestAssessmentStepCompletesOnPassingAttempt(t *testing.T) {
	t.Parallel()

	svc, _ := newAssessmentService(t, stubVerifier{passed: true})
	result := assignAssessmentJourney(t, svc)

	step, err := svc.CompleteStep(
		context.Background(),
		testOrgID,
		testEmployeeUser,
		result.Steps[0].ID,
		assignments.CompleteStepInput{},
	)
	if err != nil {
		t.Fatalf("CompleteStep: %v", err)
	}

	if step.Status != "completed" || step.CompletedAt == nil {
		t.Fatalf("expected completed step, got %+v", step)
	}
}

func TestAssessmentStepUnavailableWithoutVerifier(t *testing.T) {
	t.Parallel()

	svc, _ := newAssessmentService(t, nil)
	result := assignAssessmentJourney(t, svc)

	_, err := svc.CompleteStep(
		context.Background(),
		testOrgID,
		testEmployeeUser,
		result.Steps[0].ID,
		assignments.CompleteStepInput{},
	)
	if !errors.Is(err, assignments.ErrInvalidState) {
		t.Fatalf("got %v want %v", err, assignments.ErrInvalidState)
	}
}
