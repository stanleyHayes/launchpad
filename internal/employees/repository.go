package employees

import "context"

// Repository persists employees.
type Repository interface {
	EnsureIndexes(ctx context.Context) error
	Create(ctx context.Context, employee Employee) error
	GetByID(ctx context.Context, organizationID, employeeID string) (Employee, error)
	GetByUserID(ctx context.Context, organizationID, userID string) (Employee, error)
	// GetByWorkEmail returns the employee with a work email in an organization,
	// or ErrNotFound. Work email is unique per organization.
	GetByWorkEmail(ctx context.Context, organizationID, workEmail string) (Employee, error)
	List(ctx context.Context, organizationID string, offset, limit int64) ([]Employee, error)
	Count(ctx context.Context, organizationID string) (int64, error)
	Update(ctx context.Context, employee Employee) error
	ProvisionAccess(ctx context.Context, organizationID, employeeID, userID string) error
	// DeleteForOrganization removes every employee document of the
	// organization and returns the number deleted. Called only by the
	// platform GDPR tenant purge (PRD 7.4).
	DeleteForOrganization(ctx context.Context, organizationID string) (int64, error)
}
