package assessments

import (
	"fmt"
	"slices"
	"strings"
)

// minChoiceOptions is the minimum number of options a choice question needs.
const minChoiceOptions = 2

// normalizeQuestions validates every question, applying defaults: true_false
// questions get their fixed options, questions without an id get a positional
// one, and questions without points get one point.
func normalizeQuestions(questions []Question) ([]Question, error) {
	if len(questions) == 0 {
		return nil, fmt.Errorf("%w: assessments require at least one question", ErrInvalidInput)
	}

	normalized := make([]Question, 0, len(questions))
	seen := make(map[string]bool, len(questions))

	for index, question := range questions {
		question, err := normalizeQuestion(question, index)
		if err != nil {
			return nil, err
		}

		if seen[question.ID] {
			return nil, fmt.Errorf("%w: question ids must be unique", ErrInvalidInput)
		}

		seen[question.ID] = true

		normalized = append(normalized, question)
	}

	return normalized, nil
}

func normalizeQuestion(question Question, index int) (Question, error) {
	if strings.TrimSpace(question.ID) == "" {
		question.ID = fmt.Sprintf("q%d", index+1)
	}

	if strings.TrimSpace(question.Text) == "" {
		return Question{}, fmt.Errorf("%w: questions require text", ErrInvalidInput)
	}

	if question.Points <= 0 {
		question.Points = defaultQuestionPoints
	}

	switch question.Type {
	case QuestionTypeSingleChoice:
		return normalizeChoiceQuestion(question, true)
	case QuestionTypeMultipleChoice:
		return normalizeChoiceQuestion(question, false)
	case QuestionTypeTrueFalse:
		return normalizeTrueFalseQuestion(question)
	case QuestionTypeShortAnswer:
		return normalizeShortAnswerQuestion(question)
	default:
		return Question{}, fmt.Errorf("%w: unknown question type %q", ErrInvalidInput, question.Type)
	}
}

func normalizeChoiceQuestion(question Question, single bool) (Question, error) {
	if len(question.Options) < minChoiceOptions {
		return Question{}, fmt.Errorf("%w: choice questions require at least two options", ErrInvalidInput)
	}

	if len(question.CorrectOptions) == 0 || (single && len(question.CorrectOptions) != 1) {
		return Question{}, fmt.Errorf("%w: choice questions require a correct option", ErrInvalidInput)
	}

	for _, correct := range question.CorrectOptions {
		if correct < 0 || correct >= len(question.Options) {
			return Question{}, fmt.Errorf("%w: correctOptions must index an option", ErrInvalidInput)
		}
	}

	question.CorrectOptions = dedupeSorted(question.CorrectOptions)

	return question, nil
}

func normalizeTrueFalseQuestion(question Question) (Question, error) {
	if len(question.Options) == 0 {
		question.Options = slices.Clone(trueFalseOptions)
	}

	if len(question.Options) != minChoiceOptions {
		return Question{}, fmt.Errorf("%w: true/false questions have exactly two options", ErrInvalidInput)
	}

	if len(question.CorrectOptions) != 1 ||
		question.CorrectOptions[0] < 0 ||
		question.CorrectOptions[0] >= len(question.Options) {
		return Question{}, fmt.Errorf("%w: true/false questions require one correct option", ErrInvalidInput)
	}

	return question, nil
}

func normalizeShortAnswerQuestion(question Question) (Question, error) {
	accepted := make([]string, 0, len(question.AcceptedAnswers))

	for _, answer := range question.AcceptedAnswers {
		if normalized := normalizeShortAnswer(answer); normalized != "" {
			accepted = append(accepted, normalized)
		}
	}

	if len(accepted) == 0 {
		return Question{}, fmt.Errorf("%w: short answer questions require accepted answers", ErrInvalidInput)
	}

	question.AcceptedAnswers = accepted
	question.Options = nil
	question.CorrectOptions = nil

	return question, nil
}

// normalizeShortAnswer lowercases and collapses whitespace so short answers
// grade as case-insensitive exact matches.
func normalizeShortAnswer(answer string) string {
	return strings.Join(strings.Fields(strings.ToLower(answer)), " ")
}

func dedupeSorted(values []int) []int {
	out := slices.Clone(values)
	slices.Sort(out)

	return slices.Compact(out)
}

// publicQuestions returns the answer-key-free view of the questions.
func publicQuestions(questions []Question) []QuestionView {
	views := make([]QuestionView, 0, len(questions))

	for _, question := range questions {
		views = append(views, QuestionView{
			ID:      question.ID,
			Type:    question.Type,
			Text:    question.Text,
			Options: question.Options,
			Points:  question.Points,
		})
	}

	return views
}

// gradeAttempt scores the submitted answers server-side against the answer
// keys. It returns the percent score and whether any short answer failed to
// match (making the attempt pending review). Choice questions grade on an
// exact set match of option indexes; short answers grade on a normalized
// exact match against the accepted answers.
func gradeAttempt(questions []Question, answers []Answer) (score float64, pendingReview bool) {
	byQuestion := make(map[string]Answer, len(answers))
	for _, answer := range answers {
		byQuestion[answer.QuestionID] = answer
	}

	earned := 0
	total := 0

	for _, question := range questions {
		total += question.Points
		answer, answered := byQuestion[question.ID]

		switch question.Type {
		case QuestionTypeShortAnswer:
			if answered && shortAnswerMatches(question, answer.Text) {
				earned += question.Points
			} else {
				pendingReview = true
			}
		default:
			if answered && slices.Equal(dedupeSorted(answer.Options), question.CorrectOptions) {
				earned += question.Points
			}
		}
	}

	if total == 0 {
		return 0, pendingReview
	}

	return float64(earned) / float64(total) * percentScale, pendingReview
}

func shortAnswerMatches(question Question, text string) bool {
	normalized := normalizeShortAnswer(text)
	if normalized == "" {
		return false
	}

	return slices.Contains(question.AcceptedAnswers, normalized)
}
