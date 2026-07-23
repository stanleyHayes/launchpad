package assignments

import "context"

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
	GetApproval(ctx context.Context, organizationID, approvalID string) (Approval, error)
	ListApprovals(ctx context.Context, organizationID string) ([]Approval, error)
	GetApprovalByStep(ctx context.Context, organizationID, stepAssignmentID string) (Approval, error)
	UpdateApproval(ctx context.Context, approval Approval) error
}
