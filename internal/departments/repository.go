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
	// DeleteForOrganization removes every department and job role of the
	// organization and returns the number of documents deleted. Called only
	// by the platform GDPR tenant purge (PRD 7.4).
	DeleteForOrganization(ctx context.Context, organizationID string) (int64, error)
}
