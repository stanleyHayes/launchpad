package assignments_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"launchpad/internal/assignments"
	"launchpad/internal/employees"
	"launchpad/internal/journeys"
)

// quizJourneyConfig builds a quiz config with questionCount questions whose
// correct option is always index 0.
func quizJourneyConfig(questionCount int) map[string]any {
	questions := make([]any, 0, questionCount)

	for i := 1; i <= questionCount; i++ {
		questions = append(questions, map[string]any{
			"id":            fmt.Sprintf("q%d", i),
			"text":          fmt.Sprintf("Question %d?", i),
			"options":       []any{"Right", "Wrong"},
			"correctOption": 0,
		})
	}

	return map[string]any{"questions": questions}
}

// quizAnswersFor builds a submission answering each questionID with the given
// option index.
func quizAnswersFor(questionIDs ...string) map[string]any {
	answers := map[string]any{}

	for _, id := range questionIDs {
		answers[id] = 0
	}

	return map[string]any{"answers": answers}
}

func newQuizService(t *testing.T, questionCount int) (*assignments.Service, *memoryRepo) {
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
					ID:       "js-quiz",
					StepType: "quiz",
					Title:    "Security quiz",
					Position: 1,
					Config:   quizJourneyConfig(questionCount),
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

	return svc, repo
}

func assignQuizJourney(t *testing.T, svc *assignments.Service) assignments.AssignResult {
	t.Helper()

	result, err := svc.Assign(context.Background(), testOrgID, "actor-user", assignments.AssignInput{
		EmployeeID:        testEmployeeID,
		JourneyTemplateID: "journey-1",
	})
	if err != nil {
		t.Fatalf("Assign: %v", err)
	}

	if len(result.Steps) != 1 || result.Steps[0].StepType != "quiz" {
		t.Fatalf("expected one quiz step, got %+v", result.Steps)
	}

	return result
}

func TestAssignQuizStepCarriesQuestionsWithoutAnswerKey(t *testing.T) {
	t.Parallel()

	svc, _ := newQuizService(t, 2)
	result := assignQuizJourney(t, svc)

	questions := result.Steps[0].QuizQuestions
	if len(questions) != 2 {
		t.Fatalf("quiz questions = %+v, want 2", questions)
	}

	for _, question := range questions {
		if question.ID == "" || question.Text == "" || len(question.Options) != 2 {
			t.Fatalf("unexpected question view: %+v", question)
		}
	}
}

func TestCompleteQuizStepFullPassCompletesAssignment(t *testing.T) {
	t.Parallel()

	svc, repo := newQuizService(t, 2)
	result := assignQuizJourney(t, svc)

	step, err := svc.CompleteStep(
		context.Background(),
		testOrgID,
		testEmployeeUser,
		result.Steps[0].ID,
		assignments.CompleteStepInput{Submission: quizAnswersFor("q1", "q2")},
	)
	if err != nil {
		t.Fatalf("CompleteStep: %v", err)
	}

	if step.Status != "completed" || step.Score == nil || *step.Score != 100 {
		t.Fatalf("step = status %s score %v, want completed/100", step.Status, step.Score)
	}

	assignment := repo.assignments[result.Assignment.ID]
	if assignment.Status != "completed" || assignment.ProgressPercent != 100 {
		t.Fatalf("assignment = status %s progress %v, want completed/100",
			assignment.Status, assignment.ProgressPercent)
	}
}

func TestCompleteQuizStepPartialPass(t *testing.T) {
	t.Parallel()

	svc, _ := newQuizService(t, 4)
	result := assignQuizJourney(t, svc)

	// 3 of 4 correct = 75%, above the 70% pass mark.
	submission := map[string]any{"answers": map[string]any{
		"q1": 0, "q2": 0, "q3": 0, "q4": 1,
	}}

	step, err := svc.CompleteStep(
		context.Background(),
		testOrgID,
		testEmployeeUser,
		result.Steps[0].ID,
		assignments.CompleteStepInput{Submission: submission},
	)
	if err != nil {
		t.Fatalf("CompleteStep: %v", err)
	}

	if step.Status != "completed" || step.Score == nil || *step.Score != 75 {
		t.Fatalf("step = status %s score %v, want completed/75", step.Status, step.Score)
	}
}

func TestCompleteQuizStepFailKeepsStepInProgress(t *testing.T) {
	t.Parallel()

	svc, repo := newQuizService(t, 2)
	result := assignQuizJourney(t, svc)

	// 1 of 2 correct = 50%, below the pass mark.
	submission := map[string]any{"answers": map[string]any{"q1": 0, "q2": 1}}

	step, err := svc.CompleteStep(
		context.Background(),
		testOrgID,
		testEmployeeUser,
		result.Steps[0].ID,
		assignments.CompleteStepInput{Submission: submission},
	)
	if err != nil {
		t.Fatalf("CompleteStep: %v", err)
	}

	if step.Status != testStatusInProgress || step.CompletedAt != nil {
		t.Fatalf("failing quiz must stay in progress, got status %s", step.Status)
	}

	if step.Score == nil || *step.Score != 50 {
		t.Fatalf("score = %v, want 50", step.Score)
	}

	stored := repo.steps[result.Steps[0].ID]
	if stored.Score == nil || *stored.Score != 50 || stored.Submission == nil {
		t.Fatalf("failed attempt not persisted: %+v", stored)
	}

	// The employee can retry and pass.
	retry, err := svc.CompleteStep(
		context.Background(),
		testOrgID,
		testEmployeeUser,
		result.Steps[0].ID,
		assignments.CompleteStepInput{Submission: quizAnswersFor("q1", "q2")},
	)
	if err != nil {
		t.Fatalf("CompleteStep retry: %v", err)
	}

	if retry.Status != "completed" || retry.Score == nil || *retry.Score != 100 {
		t.Fatalf("retry = status %s score %v, want completed/100", retry.Status, retry.Score)
	}
}

func TestCompleteQuizStepBlocksAtAttemptLimit(t *testing.T) {
	t.Parallel()

	svc, repo := newQuizService(t, 2)
	result := assignQuizJourney(t, svc)
	stored := repo.steps[result.Steps[0].ID]
	stored.MaxAttempts = 1
	repo.steps[stored.ID] = stored

	step, err := svc.CompleteStep(
		context.Background(),
		testOrgID,
		testEmployeeUser,
		stored.ID,
		assignments.CompleteStepInput{
			Submission: map[string]any{"answers": map[string]any{"q1": 0, "q2": 1}},
		},
	)
	if err != nil {
		t.Fatalf("CompleteStep: %v", err)
	}
	if step.Status != "blocked" || step.AttemptCount != 1 || step.EscalatedAt == nil {
		t.Fatalf("limited attempt = %+v", step)
	}
}

func TestCompleteQuizStepIgnoresUnknownQuestionIDs(t *testing.T) {
	t.Parallel()

	svc, _ := newQuizService(t, 1)
	result := assignQuizJourney(t, svc)

	submission := map[string]any{"answers": map[string]any{
		"q1":           0,
		"not-a-real-q": 1,
	}}

	step, err := svc.CompleteStep(
		context.Background(),
		testOrgID,
		testEmployeeUser,
		result.Steps[0].ID,
		assignments.CompleteStepInput{Submission: submission},
	)
	if err != nil {
		t.Fatalf("CompleteStep: %v", err)
	}

	if step.Status != "completed" || step.Score == nil || *step.Score != 100 {
		t.Fatalf("step = status %s score %v, want completed/100", step.Status, step.Score)
	}
}

func TestCompleteQuizStepRequiresAnswers(t *testing.T) {
	t.Parallel()

	svc, repo := newQuizService(t, 1)
	result := assignQuizJourney(t, svc)

	_, err := svc.CompleteStep(
		context.Background(),
		testOrgID,
		testEmployeeUser,
		result.Steps[0].ID,
		assignments.CompleteStepInput{Submission: map[string]any{"notes": "done"}},
	)
	if !errors.Is(err, assignments.ErrInvalidInput) {
		t.Fatalf("got %v want %v", err, assignments.ErrInvalidInput)
	}

	if repo.steps[result.Steps[0].ID].Status != testStatusInProgress {
		t.Fatalf("step must stay in progress, got %s", repo.steps[result.Steps[0].ID].Status)
	}
}

func TestCompleteQuizStepIgnoresClientSuppliedScore(t *testing.T) {
	t.Parallel()

	svc, _ := newQuizService(t, 2)
	result := assignQuizJourney(t, svc)

	clientScore := 100.0

	// All answers wrong, but the client claims 100 — the server regrades.
	submission := map[string]any{"answers": map[string]any{"q1": 1, "q2": 1}}

	step, err := svc.CompleteStep(
		context.Background(),
		testOrgID,
		testEmployeeUser,
		result.Steps[0].ID,
		assignments.CompleteStepInput{Submission: submission, Score: &clientScore},
	)
	if err != nil {
		t.Fatalf("CompleteStep: %v", err)
	}

	if step.Status == "completed" {
		t.Fatal("client-supplied score must not complete the step")
	}

	if step.Score == nil || *step.Score != 0 {
		t.Fatalf("score = %v, want server-graded 0", step.Score)
	}
}
