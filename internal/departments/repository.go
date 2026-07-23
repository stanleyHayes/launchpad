package departments

import "context"

// Repository persists departments and job roles.
type Repository interface {
	EnsureIndexes(ctx context.Context) error
	CreateDepartment(ctx context.Context, department Department) error
	ListDepartments(ctx context.Context, organizationID string) ([]Department, error)
	GetDepartment(ctx context.Context, organizationID, departmentID string) (Department, error)
	CreateJobRole(ctx context.Context, role JobRole) error
	ListJobRoles(ctx context.Context, organizationID string) ([]JobRole, error)
	GetJobRole(ctx context.Context, organizationID, roleID string) (JobRole, error)
}
