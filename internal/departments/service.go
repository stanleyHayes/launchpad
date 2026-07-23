package departments

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Service implements department and job-role use cases.
type Service struct {
	store *Store
}

// NewService constructs a Service.
func NewService(store *Store) *Service {
	return &Service{store: store}
}

// CreateDepartment creates a department for an organization.
func (s *Service) CreateDepartment(
	ctx context.Context,
	organizationID string,
	in CreateDepartmentInput,
) (Department, error) {
	name := strings.TrimSpace(in.Name)
	if organizationID == "" || name == "" {
		return Department{}, ErrInvalidInput
	}

	now := time.Now().UTC()

	department := Department{
		ID:             uuid.NewString(),
		OrganizationID: organizationID,
		Name:           name,
		Description:    strings.TrimSpace(in.Description),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := s.store.CreateDepartment(ctx, department); err != nil {
		return Department{}, fmt.Errorf("create department: %w", err)
	}

	return department, nil
}

// ListDepartments lists departments for an organization.
func (s *Service) ListDepartments(ctx context.Context, organizationID string) ([]Department, error) {
	if organizationID == "" {
		return nil, ErrInvalidInput
	}

	items, err := s.store.ListDepartments(ctx, organizationID)
	if err != nil {
		return nil, fmt.Errorf("list departments: %w", err)
	}

	return items, nil
}

// CreateJobRole creates a job role for an organization.
func (s *Service) CreateJobRole(
	ctx context.Context,
	organizationID string,
	in CreateJobRoleInput,
) (JobRole, error) {
	name := strings.TrimSpace(in.Name)
	if organizationID == "" || name == "" {
		return JobRole{}, ErrInvalidInput
	}

	now := time.Now().UTC()

	role := JobRole{
		ID:             uuid.NewString(),
		OrganizationID: organizationID,
		Name:           name,
		Description:    strings.TrimSpace(in.Description),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := s.store.CreateJobRole(ctx, role); err != nil {
		return JobRole{}, fmt.Errorf("create job role: %w", err)
	}

	return role, nil
}

// ListJobRoles lists job roles for an organization.
func (s *Service) ListJobRoles(ctx context.Context, organizationID string) ([]JobRole, error) {
	if organizationID == "" {
		return nil, ErrInvalidInput
	}

	items, err := s.store.ListJobRoles(ctx, organizationID)
	if err != nil {
		return nil, fmt.Errorf("list job roles: %w", err)
	}

	return items, nil
}

// EnsureDepartmentExists verifies a department belongs to the organization.
func (s *Service) EnsureDepartmentExists(ctx context.Context, organizationID, departmentID string) error {
	if departmentID == "" {
		return nil
	}

	_, err := s.store.GetDepartment(ctx, organizationID, departmentID)

	return err
}

// EnsureJobRoleExists verifies a job role belongs to the organization.
func (s *Service) EnsureJobRoleExists(ctx context.Context, organizationID, roleID string) error {
	if roleID == "" {
		return nil
	}

	_, err := s.store.GetJobRole(ctx, organizationID, roleID)

	return err
}
