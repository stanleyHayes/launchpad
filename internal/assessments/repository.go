package assessments

import (
	"context"

	"launchpad/internal/employees"
)

// Repository persists assessments, attempts, and certificates. Every method
// is tenant-scoped by organizationId so no query can cross tenant boundaries.
type Repository interface {
	EnsureIndexes(ctx context.Context) error

	CreateAssessment(ctx context.Context, assessment Assessment) error
	GetAssessment(ctx context.Context, organizationID, assessmentID string) (Assessment, error)
	ListAssessments(ctx context.Context, organizationID string) ([]Assessment, error)
	UpdateAssessment(ctx context.Context, assessment Assessment) error

	CreateAttempt(ctx context.Context, attempt Attempt) error
	GetAttempt(ctx context.Context, organizationID, assessmentID, attemptID string) (Attempt, error)
	// CountAttempts returns the number of submitted attempts an employee has
	// on an assessment (used for attempt numbering and limits).
	CountAttempts(ctx context.Context, organizationID, assessmentID, employeeID string) (int64, error)
	ListAttempts(ctx context.Context, organizationID, assessmentID string) ([]Attempt, error)
	// LatestAttempt returns the employee's most recent submitted attempt.
	LatestAttempt(ctx context.Context, organizationID, assessmentID, employeeID string) (Attempt, error)
	UpdateAttempt(ctx context.Context, attempt Attempt) error

	CreateCertificate(ctx context.Context, certificate Certificate) error
	// FindCertificate returns the certificate an employee holds for an
	// assessment, or ErrNotFound when none was issued yet.
	FindCertificate(ctx context.Context, organizationID, assessmentID, employeeID string) (Certificate, error)
	ListCertificatesForEmployee(ctx context.Context, organizationID, employeeID string) ([]Certificate, error)
}

// EmployeeReader resolves employees from user accounts or ids. Implemented
// by internal/employees's service.
type EmployeeReader interface {
	Get(ctx context.Context, organizationID, employeeID string) (employees.Employee, error)
	GetByUserID(ctx context.Context, organizationID, userID string) (employees.Employee, error)
}
