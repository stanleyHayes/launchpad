package journeys

import (
	"encoding/json"
	"fmt"
	"strings"
)

// minQuizOptions is the minimum number of options a quiz question needs.
const minQuizOptions = 2

// QuizQuestion is a single-choice question stored in a quiz step's Config.
// CorrectOption is the index into Options of the correct answer.
type QuizQuestion struct {
	ID            string   `json:"id"`
	Text          string   `json:"text"`
	Options       []string `json:"options"`
	CorrectOption int      `json:"correctOption"`
}

// QuizConfig is the typed shape of a quiz step's Config.
type QuizConfig struct {
	Questions []QuizQuestion `json:"questions"`
}

// QuizQuestionView is a quiz question without its answer key, safe to show
// to employees taking the quiz.
type QuizQuestionView struct {
	ID      string   `json:"id"`
	Text    string   `json:"text"`
	Options []string `json:"options"`
}

// ParseQuizConfig decodes a quiz step's free-form Config into its typed shape.
func ParseQuizConfig(config map[string]any) (QuizConfig, error) {
	raw, err := json.Marshal(config)
	if err != nil {
		return QuizConfig{}, fmt.Errorf("%w: quiz config is not serializable", ErrInvalidInput)
	}

	var quiz QuizConfig
	if err := json.Unmarshal(raw, &quiz); err != nil {
		return QuizConfig{}, fmt.Errorf("%w: quiz config has an invalid shape", ErrInvalidInput)
	}

	return quiz, nil
}

// ValidateQuizConfig enforces the quiz invariants: at least one question,
// each with a unique non-empty id, non-empty text, at least two options, and
// a correctOption pointing at a real option.
func ValidateQuizConfig(config map[string]any) error {
	quiz, err := ParseQuizConfig(config)
	if err != nil {
		return err
	}

	if len(quiz.Questions) == 0 {
		return fmt.Errorf("%w: quiz steps require at least one question", ErrInvalidInput)
	}

	seen := make(map[string]bool, len(quiz.Questions))

	for _, question := range quiz.Questions {
		id := strings.TrimSpace(question.ID)
		if id == "" || strings.TrimSpace(question.Text) == "" {
			return fmt.Errorf("%w: quiz questions require an id and text", ErrInvalidInput)
		}

		if seen[id] {
			return fmt.Errorf("%w: quiz question ids must be unique", ErrInvalidInput)
		}

		seen[id] = true

		if len(question.Options) < minQuizOptions {
			return fmt.Errorf("%w: quiz questions require at least two options", ErrInvalidInput)
		}

		if question.CorrectOption < 0 || question.CorrectOption >= len(question.Options) {
			return fmt.Errorf("%w: quiz correctOption must index an option", ErrInvalidInput)
		}
	}

	return nil
}

// PublicQuizQuestions returns the answer-key-free view of a quiz step's
// questions. It returns nil when the config is not a valid quiz config.
func PublicQuizQuestions(config map[string]any) []QuizQuestionView {
	quiz, err := ParseQuizConfig(config)
	if err != nil {
		return nil
	}

	if len(quiz.Questions) == 0 {
		return nil
	}

	views := make([]QuizQuestionView, 0, len(quiz.Questions))

	for _, question := range quiz.Questions {
		views = append(views, QuizQuestionView{
			ID:      question.ID,
			Text:    question.Text,
			Options: question.Options,
		})
	}

	return views
}
