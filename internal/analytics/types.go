// Package analytics computes onboarding and platform operational summaries.
package analytics

import (
	"context"
	"errors"
	"time"

	"launchpad/internal/assignments"
	"launchpad/internal/assistant"
	"launchpad/internal/departments"
	"launchpad/internal/employees"
)

var (
	// ErrInvalidInput indicates validation failed.
	ErrInvalidInput = errors.New("invalid analytics input")
	// ErrSourceNotConfigured indicates a reporting source was not wired.
	ErrSourceNotConfigured = errors.New("analytics source not configured")
)

// AssignmentSource loads assignment data for analytics.
type AssignmentSource interface {
	List(ctx context.Context, organizationID string) ([]assignments.JourneyAssignment, error)
	ListApprovals(ctx context.Context, organizationID string) ([]assignments.Approval, error)
	ListSteps(ctx context.Context, organizationID, journeyAssignmentID string) ([]assignments.StepAssignment, error)
}

// EmployeeSource counts and pages employees for analytics.
type EmployeeSource interface {
	Count(ctx context.Context, organizationID string) (int64, error)
	List(ctx context.Context, organizationID string, offset, limit int64) ([]employees.Employee, error)
}

// DirectorySource names departments and job roles for breakdowns.
type DirectorySource interface {
	ListDepartments(ctx context.Context, organizationID string) ([]departments.Department, error)
	ListJobRoles(ctx context.Context, organizationID string) ([]departments.JobRole, error)
}

// AssistantSource loads assistant interactions for reporting.
type AssistantSource interface {
	ListInteractions(ctx context.Context, organizationID string) ([]assistant.Interaction, error)
}

const (
	statusScheduled  = "scheduled"
	statusInProgress = "in_progress"
	statusCompleted  = "completed"
	approvalPending  = "pending"
	hoursPerDay      = 24.0

	// BreakdownByDepartment groups onboarding completion by department.
	BreakdownByDepartment = "department"
	// BreakdownByJobRole groups onboarding completion by job role.
	BreakdownByJobRole = "jobRole"

	employeePageSize    = 500
	topQuestionLimit    = 10
	unassignedGroupName = "Unassigned"
)

// OnboardingSummary is an organization onboarding snapshot.
type OnboardingSummary struct {
	EmployeeCount            int       `json:"employeeCount"`
	ActiveAssignmentCount    int       `json:"activeAssignmentCount"`
	CompletedAssignmentCount int       `json:"completedAssignmentCount"`
	ScheduledAssignmentCount int       `json:"scheduledAssignmentCount"`
	PendingApprovalCount     int       `json:"pendingApprovalCount"`
	IncompleteStepCount      int       `json:"incompleteStepCount"`
	OverdueStepCount         int       `json:"overdueStepCount"`
	CompletionRate           float64   `json:"completionRate"`
	OverdueRate              float64   `json:"overdueRate"`
	AverageDaysToComplete    float64   `json:"averageDaysToComplete"`
	GeneratedAt              time.Time `json:"generatedAt"`
}

// BreakdownRow is one department or job-role completion rollup.
type BreakdownRow struct {
	ID                       string  `json:"id"`
	Name                     string  `json:"name"`
	EmployeeCount            int     `json:"employeeCount"`
	AssignmentCount          int     `json:"assignmentCount"`
	CompletedAssignmentCount int     `json:"completedAssignmentCount"`
	CompletionRate           float64 `json:"completionRate"`
}

// OnboardingBreakdown groups onboarding completion by department or job role.
type OnboardingBreakdown struct {
	By          string         `json:"by"`
	Rows        []BreakdownRow `json:"rows"`
	GeneratedAt time.Time      `json:"generatedAt"`
}

// AssistantQuestionStat is the frequency of one refused question.
type AssistantQuestionStat struct {
	Question string `json:"question"`
	Count    int    `json:"count"`
}

// AssistantReport summarizes assistant usage and answer quality.
type AssistantReport struct {
	TotalQuestions      int                     `json:"totalQuestions"`
	RefusalCount        int                     `json:"refusalCount"`
	RefusalRate         float64                 `json:"refusalRate"`
	FeedbackCount       int                     `json:"feedbackCount"`
	HelpfulCount        int                     `json:"helpfulCount"`
	HelpfulRate         float64                 `json:"helpfulRate"`
	TopRefusedQuestions []AssistantQuestionStat `json:"topRefusedQuestions"`
	GeneratedAt         time.Time               `json:"generatedAt"`
}

type MilestoneStat struct {
	StepTitle string  `json:"stepTitle"`
	Position  int     `json:"position"`
	Reached   int     `json:"reached"`
	Total     int     `json:"total"`
	Rate      float64 `json:"rate"`
}

type DropOffStat struct {
	StepTitle string `json:"stepTitle"`
	Position  int    `json:"position"`
	Count     int    `json:"count"`
}

type FunnelReport struct {
	Milestones  []MilestoneStat `json:"milestones"`
	DropOff     []DropOffStat   `json:"dropOff"`
	GeneratedAt time.Time       `json:"generatedAt"`
}
