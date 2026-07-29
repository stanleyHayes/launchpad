// Package assessments manages scored assessments: question banks with
// server-side grading, attempt limits, manual review of short answers, and
// certificates issued on a pass (PRD §5.3.6).
package assessments

import (
	"errors"
	"time"
)

var (
	// ErrNotFound indicates an assessment, attempt, or certificate was not found.
	ErrNotFound = errors.New("assessment not found")
	// ErrAttemptNotFound indicates an assessment attempt was not found.
	ErrAttemptNotFound = errors.New("assessment attempt not found")
	// ErrInvalidInput indicates validation failed.
	ErrInvalidInput = errors.New("invalid assessment input")
	// ErrInvalidState indicates an illegal status transition.
	ErrInvalidState = errors.New("invalid assessment state")
	// ErrNotPublished indicates the assessment is not published.
	ErrNotPublished = errors.New("assessment is not published")
	// ErrAttemptsExhausted indicates the employee used every allowed attempt.
	ErrAttemptsExhausted = errors.New("assessment attempt limit reached")
)

// Question types (PRD §5.3.6 v1 scope).
const (
	QuestionTypeSingleChoice   = "single_choice"
	QuestionTypeMultipleChoice = "multiple_choice"
	QuestionTypeTrueFalse      = "true_false"
	QuestionTypeShortAnswer    = "short_answer"
)

// Assessment lifecycle statuses.
const (
	StatusDraft     = "draft"
	StatusPublished = "published"
	StatusArchived  = "archived"
)

// Attempt grading statuses.
const (
	// AttemptGraded means the attempt has a final score (auto-graded or
	// finalized by a manager review).
	AttemptGraded = "graded"
	// AttemptPendingReview means at least one short answer did not match an
	// accepted answer, so a manager must finalize the score before the
	// attempt counts.
	AttemptPendingReview = "pending_review"
)

const (
	// percentScale converts point ratios to percentages.
	percentScale = 100.0
	// defaultQuestionPoints is applied when a question carries no points.
	defaultQuestionPoints = 1
)

// trueFalseOptions are the fixed options for true_false questions.
var trueFalseOptions = []string{"True", "False"}

// Question is one scored question in an assessment. Choice answers are
// option indexes (CorrectOptions); short answers match AcceptedAnswers
// case-insensitively after normalization.
type Question struct {
	ID              string   `bson:"id"                        json:"id"`
	Type            string   `bson:"type"                      json:"type"`
	Text            string   `bson:"text"                      json:"text"`
	Options         []string `bson:"options,omitempty"         json:"options,omitempty"`
	CorrectOptions  []int    `bson:"correctOptions,omitempty"  json:"correctOptions,omitempty"`
	AcceptedAnswers []string `bson:"acceptedAnswers,omitempty" json:"acceptedAnswers,omitempty"`
	Points          int      `bson:"points"                    json:"points"`
}

// QuestionView is the answer-key-free view of a question, safe to show to
// employees taking the assessment.
type QuestionView struct {
	ID      string   `json:"id"`
	Type    string   `json:"type"`
	Text    string   `json:"text"`
	Options []string `json:"options,omitempty"`
	Points  int      `json:"points"`
}

// Assessment is a scored question set owned by an organization.
type Assessment struct {
	ID             string     `bson:"_id"            json:"id"`
	OrganizationID string     `bson:"organizationId" json:"organizationId"`
	Title          string     `bson:"title"          json:"title"`
	Description    string     `bson:"description"    json:"description"`
	Questions      []Question `bson:"questions"      json:"questions"`
	// PassingScore is the minimum percent score to pass.
	PassingScore float64 `bson:"passingScore" json:"passingScore"`
	// MaxAttempts caps submitted attempts per employee; 0 means unlimited.
	MaxAttempts int       `bson:"maxAttempts" json:"maxAttempts"`
	Randomize   bool      `bson:"randomize"   json:"randomize"`
	Status      string    `bson:"status"      json:"status"`
	CreatedBy   string    `bson:"createdBy"   json:"createdBy"`
	CreatedAt   time.Time `bson:"createdAt" json:"createdAt"`
	UpdatedAt   time.Time `bson:"updatedAt" json:"updatedAt"`
}

// Answer is an employee's answer to one question: Options for choice
// questions (selected option indexes), Text for short answers.
type Answer struct {
	QuestionID string `bson:"questionId"        json:"questionId"`
	Options    []int  `bson:"options,omitempty" json:"options,omitempty"`
	Text       string `bson:"text,omitempty"    json:"text,omitempty"`
}

// Attempt is one employee submission, graded server-side.
type Attempt struct {
	ID             string    `bson:"_id"            json:"id"`
	OrganizationID string    `bson:"organizationId" json:"organizationId"`
	AssessmentID   string    `bson:"assessmentId"   json:"assessmentId"`
	EmployeeID     string    `bson:"employeeId"     json:"employeeId"`
	Answers        []Answer  `bson:"answers"        json:"answers"`
	Score          float64   `bson:"score"          json:"score"`
	Passed         bool      `bson:"passed"         json:"passed"`
	Status         string    `bson:"status"         json:"status"`
	AttemptNumber  int       `bson:"attemptNumber"  json:"attemptNumber"`
	ReviewNote     string    `bson:"reviewNote,omitempty" json:"reviewNote,omitempty"`
	ReviewedBy     string    `bson:"reviewedBy,omitempty" json:"reviewedBy,omitempty"`
	StartedAt      time.Time `bson:"startedAt"      json:"startedAt"`
	SubmittedAt    time.Time `bson:"submittedAt"    json:"submittedAt"`
}

// Certificate is issued when an employee passes an assessment. It is a
// record only (no PDF in v1).
type Certificate struct {
	ID              string    `bson:"_id"             json:"id"`
	OrganizationID  string    `bson:"organizationId"  json:"organizationId"`
	EmployeeID      string    `bson:"employeeId"      json:"employeeId"`
	EmployeeName    string    `bson:"employeeName"    json:"employeeName"`
	AssessmentID    string    `bson:"assessmentId"    json:"assessmentId"`
	AssessmentTitle string    `bson:"assessmentTitle" json:"assessmentTitle"`
	Score           float64   `bson:"score"           json:"score"`
	Serial          string    `bson:"serial"          json:"serial"`
	IssuedAt        time.Time `bson:"issuedAt"        json:"issuedAt"`
}

// CreateAssessmentInput creates a draft assessment.
type CreateAssessmentInput struct {
	Title        string
	Description  string
	Questions    []Question
	PassingScore float64
	MaxAttempts  int
	Randomize    bool
}

// UpdateAssessmentInput replaces the editable fields of a draft assessment.
type UpdateAssessmentInput struct {
	Title        *string
	Description  *string
	Questions    *[]Question
	PassingScore *float64
	MaxAttempts  *int
	Randomize    *bool
}

// SubmitAttemptInput carries the employee's answers.
type SubmitAttemptInput struct {
	Answers []Answer
}

// ReviewAttemptInput finalizes a pending-review attempt with a manager score.
type ReviewAttemptInput struct {
	Score float64
	Note  string
}

// TakeView is what an employee needs to take an assessment: questions
// without answer keys (shuffled when the assessment randomizes) plus the
// attempt budget. AttemptsRemaining is -1 when attempts are unlimited.
type TakeView struct {
	AssessmentID      string         `json:"assessmentId"`
	Title             string         `json:"title"`
	Description       string         `json:"description"`
	PassingScore      float64        `json:"passingScore"`
	Questions         []QuestionView `json:"questions"`
	AttemptsUsed      int            `json:"attemptsUsed"`
	AttemptsRemaining int            `json:"attemptsRemaining"`
}
