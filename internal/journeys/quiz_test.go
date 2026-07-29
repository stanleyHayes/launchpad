package journeys_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"launchpad/internal/journeys"
)

func validQuizConfig() map[string]any {
	return map[string]any{
		"questions": []any{
			map[string]any{
				"id":            "q1",
				"text":          "What color is the sky?",
				"options":       []any{"Blue", "Green"},
				"correctOption": 0,
			},
		},
	}
}

func TestAddStepQuizAcceptsValidConfig(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	repo.templates[testTemplateID] = journeys.Template{
		ID: testTemplateID, OrganizationID: testOrgID, Status: "draft", CurrentVersion: 1,
	}

	svc := journeys.NewService(repo)

	step, err := svc.AddStep(context.Background(), testOrgID, testTemplateID, journeys.AddStepInput{
		StepType: "quiz",
		Title:    "Security quiz",
		Config:   validQuizConfig(),
	})
	if err != nil {
		t.Fatalf("AddStep: %v", err)
	}

	quiz, err := journeys.ParseQuizConfig(step.Config)
	if err != nil {
		t.Fatalf("ParseQuizConfig: %v", err)
	}

	if len(quiz.Questions) != 1 || quiz.Questions[0].CorrectOption != 0 {
		t.Fatalf("unexpected stored quiz config: %+v", quiz)
	}
}

func TestAddStepQuizValidation(t *testing.T) {
	t.Parallel()

	cases := map[string]map[string]any{
		"no config": nil,
		"no questions": {
			"questions": []any{},
		},
		"question without id": {
			"questions": []any{
				map[string]any{"text": "Q?", "options": []any{"a", "b"}, "correctOption": 0},
			},
		},
		"question without text": {
			"questions": []any{
				map[string]any{"id": "q1", "options": []any{"a", "b"}, "correctOption": 0},
			},
		},
		"duplicate question ids": {
			"questions": []any{
				map[string]any{"id": "q1", "text": "One?", "options": []any{"a", "b"}, "correctOption": 0},
				map[string]any{"id": "q1", "text": "Two?", "options": []any{"a", "b"}, "correctOption": 1},
			},
		},
		"fewer than two options": {
			"questions": []any{
				map[string]any{"id": "q1", "text": "Q?", "options": []any{"a"}, "correctOption": 0},
			},
		},
		"correctOption out of range": {
			"questions": []any{
				map[string]any{"id": "q1", "text": "Q?", "options": []any{"a", "b"}, "correctOption": 2},
			},
		},
		"correctOption negative": {
			"questions": []any{
				map[string]any{"id": "q1", "text": "Q?", "options": []any{"a", "b"}, "correctOption": -1},
			},
		},
	}

	for name, config := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			repo := newMemoryRepo()
			repo.templates[testTemplateID] = journeys.Template{
				ID: testTemplateID, OrganizationID: testOrgID, Status: "draft", CurrentVersion: 1,
			}

			svc := journeys.NewService(repo)

			_, err := svc.AddStep(context.Background(), testOrgID, testTemplateID, journeys.AddStepInput{
				StepType: "quiz",
				Title:    "Security quiz",
				Config:   config,
			})
			if !errors.Is(err, journeys.ErrInvalidInput) {
				t.Fatalf("got %v want %v", err, journeys.ErrInvalidInput)
			}
		})
	}
}

func TestPublicQuizQuestionsStripsAnswerKey(t *testing.T) {
	t.Parallel()

	views := journeys.PublicQuizQuestions(validQuizConfig())
	if len(views) != 1 {
		t.Fatalf("views = %+v, want one question", views)
	}

	if views[0].ID != "q1" || views[0].Text == "" || len(views[0].Options) != 2 {
		t.Fatalf("unexpected view: %+v", views[0])
	}

	raw, err := json.Marshal(views[0])
	if err != nil {
		t.Fatalf("marshal view: %v", err)
	}

	if strings.Contains(string(raw), "correctOption") {
		t.Fatalf("view leaks the answer key: %s", raw)
	}

	if views := journeys.PublicQuizQuestions(map[string]any{}); views != nil {
		t.Fatalf("invalid config: views = %+v, want nil", views)
	}
}

func (m *memoryRepo) DeleteForOrganization(context.Context, string) (int64, error) {
	return 0, nil
}
