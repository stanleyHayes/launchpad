package assignments

import (
	"context"
	"time"
)

// Repository persists journey assignments, step assignments, and approvals.
type Repository interface {
	EnsureIndexes(ctx context.Context) error
	CreateAssignment(ctx context.Context, assignment JourneyAssignment) error
	CreateStepAssignments(ctx context.Context, steps []StepAssignment) error
	CreateApproval(ctx context.Context, approval Approval) error
	FindActiveAssignment(ctx context.Context, organizationID, employeeID, templateID string) (JourneyAssignment, error)
	GetAssignment(ctx context.Context, organizationID, assignmentID string) (JourneyAssignment, error)
	ListAssignments(ctx context.Context, organizationID string) ([]JourneyAssignment, error)
	ListAssignmentsForEmployee(ctx context.Context, organizationID, employeeID string) ([]JourneyAssignment, error)
	UpdateAssignment(ctx context.Context, assignment JourneyAssignment) error
	ListSteps(ctx context.Context, organizationID, journeyAssignmentID string) ([]StepAssignment, error)
	GetStep(ctx context.Context, organizationID, stepAssignmentID string) (StepAssignment, error)
	UpdateStep(ctx context.Context, step StepAssignment) error
	// ListDueSoonSteps returns incomplete steps due in (from, to] that have
	// not yet received a due-soon notification. Cross-organization; used by
	// the scheduler sweep.
	ListDueSoonSteps(ctx context.Context, from, to time.Time) ([]StepAssignment, error)
	// ListOverdueSteps returns incomplete steps due before now that have not
	// yet received an overdue notification.
	ListOverdueSteps(ctx context.Context, now time.Time) ([]StepAssignment, error)
	GetApproval(ctx context.Context, organizationID, approvalID string) (Approval, error)
	ListApprovals(ctx context.Context, organizationID string) ([]Approval, error)
	GetApprovalByStep(ctx context.Context, organizationID, stepAssignmentID string) (Approval, error)
	UpdateApproval(ctx context.Context, approval Approval) error
	// DeleteForOrganization removes every assignment, step assignment,
	// approval, and assignment rule of the organization and returns the
	// number of documents deleted. Called only by the platform GDPR tenant
	// purge (PRD 7.4).
	DeleteForOrganization(ctx context.Context, organizationID string) (int64, error)
}
