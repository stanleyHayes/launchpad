// Package analytics computes onboarding and platform operational summaries.
package analytics

import (
	"context"
	"errors"
	"time"

	"launchpad/internal/assignments"
	"launchpad/internal/employees"
)

var (
	// ErrInvalidInput indicates validation failed.
	ErrInvalidInput = errors.New("invalid analytics input")
)

// AssignmentSource loads assignment data for analytics.
type AssignmentSource interface {
	List(ctx context.Context, organizationID string) ([]assignments.JourneyAssignment, error)
	ListApprovals(ctx context.Context, organizationID string) ([]assignments.Approval, error)
}

// EmployeeSource loads employees for analytics.
type EmployeeSource interface {
	List(ctx context.Context, organizationID string, limit int64) ([]employees.Employee, error)
}

const (
	statusScheduled  = "scheduled"
	statusInProgress = "in_progress"
	statusCompleted  = "completed"
	approvalPending  = "pending"
	hoursPerDay      = 24.0
	analyticsListCap = int64(100)
)

// OnboardingSummary is an organization onboarding snapshot.
type OnboardingSummary struct {
	EmployeeCount            int       `json:"employeeCount"`
	ActiveAssignmentCount    int       `json:"activeAssignmentCount"`
	CompletedAssignmentCount int       `json:"completedAssignmentCount"`
	ScheduledAssignmentCount int       `json:"scheduledAssignmentCount"`
	PendingApprovalCount     int       `json:"pendingApprovalCount"`
	CompletionRate           float64   `json:"completionRate"`
	AverageDaysToComplete    float64   `json:"averageDaysToComplete"`
	GeneratedAt              time.Time `json:"generatedAt"`
}
