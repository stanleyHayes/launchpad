package employees

import "context"

// Repository persists employees.
type Repository interface {
	EnsureIndexes(ctx context.Context) error
	Create(ctx context.Context, employee Employee) error
	GetByID(ctx context.Context, organizationID, employeeID string) (Employee, error)
	GetByUserID(ctx context.Context, organizationID, userID string) (Employee, error)
	List(ctx context.Context, organizationID string, limit int64) ([]Employee, error)
	Update(ctx context.Context, employee Employee) error
	ProvisionAccess(ctx context.Context, organizationID, employeeID, userID string) error
}
