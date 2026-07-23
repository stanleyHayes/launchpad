package employees

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"launchpad/internal/departments"
)

// ReferenceChecker validates related department/role/manager references.
type ReferenceChecker interface {
	EnsureDepartmentExists(ctx context.Context, organizationID, departmentID string) error
	EnsureJobRoleExists(ctx context.Context, organizationID, roleID string) error
}

// Service implements employee use cases.
type Service struct {
	store      *Store
	references ReferenceChecker
}

// NewService constructs a Service.
func NewService(store *Store, references ReferenceChecker) *Service {
	return &Service{store: store, references: references}
}

// Create creates an invited employee.
func (s *Service) Create(ctx context.Context, organizationID string, in CreateInput) (Employee, error) {
	firstName := strings.TrimSpace(in.FirstName)
	lastName := strings.TrimSpace(in.LastName)
	workEmail := strings.ToLower(strings.TrimSpace(in.WorkEmail))

	if organizationID == "" || firstName == "" || lastName == "" || workEmail == "" || !strings.Contains(workEmail, "@") {
		return Employee{}, ErrInvalidInput
	}

	if in.StartDate.IsZero() {
		return Employee{}, ErrInvalidInput
	}

	if err := s.validateReferences(ctx, organizationID, in.DepartmentID, in.JobRoleID, in.ManagerEmployeeID); err != nil {
		return Employee{}, err
	}

	now := time.Now().UTC()

	employee := Employee{
		ID:                uuid.NewString(),
		OrganizationID:    organizationID,
		EmployeeNumber:    strings.TrimSpace(in.EmployeeNumber),
		FirstName:         firstName,
		LastName:          lastName,
		WorkEmail:         workEmail,
		JobRoleID:         strings.TrimSpace(in.JobRoleID),
		DepartmentID:      strings.TrimSpace(in.DepartmentID),
		ManagerEmployeeID: strings.TrimSpace(in.ManagerEmployeeID),
		StartDate:         in.StartDate.UTC(),
		Status:            statusInvited,
		Metadata:          map[string]any{},
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := s.store.Create(ctx, employee); err != nil {
		return Employee{}, fmt.Errorf("create employee: %w", err)
	}

	return employee, nil
}

// List lists employees for an organization.
func (s *Service) List(ctx context.Context, organizationID string, limit int64) ([]Employee, error) {
	if organizationID == "" {
		return nil, ErrInvalidInput
	}

	items, err := s.store.List(ctx, organizationID, limit)
	if err != nil {
		return nil, fmt.Errorf("list employees: %w", err)
	}

	return items, nil
}

// Get returns one employee.
func (s *Service) Get(ctx context.Context, organizationID, employeeID string) (Employee, error) {
	if organizationID == "" || employeeID == "" {
		return Employee{}, ErrInvalidInput
	}

	employee, err := s.store.GetByID(ctx, organizationID, employeeID)
	if err != nil {
		return Employee{}, fmt.Errorf("get employee: %w", err)
	}

	return employee, nil
}

// GetByUserID returns the employee linked to a user in an organization.
func (s *Service) GetByUserID(ctx context.Context, organizationID, userID string) (Employee, error) {
	if organizationID == "" || userID == "" {
		return Employee{}, ErrInvalidInput
	}

	employee, err := s.store.GetByUserID(ctx, organizationID, userID)
	if err != nil {
		return Employee{}, fmt.Errorf("get employee by user: %w", err)
	}

	return employee, nil
}

// ProvisionAccess links an employee record to an externally created user.
func (s *Service) ProvisionAccess(ctx context.Context, organizationID, employeeID, userID string) error {
	if organizationID == "" || employeeID == "" || userID == "" {
		return ErrInvalidInput
	}

	if err := s.store.ProvisionAccess(ctx, organizationID, employeeID, userID); err != nil {
		return fmt.Errorf("provision employee access: %w", err)
	}

	return nil
}

// Update updates mutable employee fields.
func (s *Service) Update(
	ctx context.Context,
	organizationID, employeeID string,
	in UpdateInput,
) (Employee, error) {
	employee, err := s.store.GetByID(ctx, organizationID, employeeID)
	if err != nil {
		return Employee{}, fmt.Errorf("get employee for update: %w", err)
	}

	if err := applyEmployeeUpdate(&employee, in); err != nil {
		return Employee{}, err
	}

	if err := s.validateReferences(
		ctx,
		organizationID,
		employee.DepartmentID,
		employee.JobRoleID,
		employee.ManagerEmployeeID,
	); err != nil {
		return Employee{}, err
	}

	employee.UpdatedAt = time.Now().UTC()
	if err := s.store.Update(ctx, employee); err != nil {
		return Employee{}, fmt.Errorf("update employee: %w", err)
	}

	return employee, nil
}

func applyEmployeeUpdate(employee *Employee, in UpdateInput) error {
	if in.FirstName != nil {
		name := strings.TrimSpace(*in.FirstName)
		if name == "" {
			return ErrInvalidInput
		}

		employee.FirstName = name
	}

	if in.LastName != nil {
		name := strings.TrimSpace(*in.LastName)
		if name == "" {
			return ErrInvalidInput
		}

		employee.LastName = name
	}

	if in.EmployeeNumber != nil {
		employee.EmployeeNumber = strings.TrimSpace(*in.EmployeeNumber)
	}

	if in.DepartmentID != nil {
		employee.DepartmentID = strings.TrimSpace(*in.DepartmentID)
	}

	if in.JobRoleID != nil {
		employee.JobRoleID = strings.TrimSpace(*in.JobRoleID)
	}

	if in.ManagerEmployeeID != nil {
		employee.ManagerEmployeeID = strings.TrimSpace(*in.ManagerEmployeeID)
	}

	if in.Status != nil {
		status := strings.TrimSpace(*in.Status)
		if status == "" {
			return ErrInvalidInput
		}

		employee.Status = status
	}

	return nil
}

// LinkUser attaches a user account to an invited employee.
func (s *Service) LinkUser(ctx context.Context, organizationID, employeeID, userID string) (Employee, error) {
	employee, err := s.store.GetByID(ctx, organizationID, employeeID)
	if err != nil {
		return Employee{}, fmt.Errorf("get employee for link: %w", err)
	}

	if employee.UserID != "" {
		return Employee{}, ErrAlreadyProvisioned
	}

	employee.UserID = userID
	employee.Status = statusActive

	employee.UpdatedAt = time.Now().UTC()
	if err := s.store.Update(ctx, employee); err != nil {
		return Employee{}, fmt.Errorf("link employee user: %w", err)
	}

	return employee, nil
}

func (s *Service) validateReferences(
	ctx context.Context,
	organizationID, departmentID, jobRoleID, managerEmployeeID string,
) error {
	if err := s.references.EnsureDepartmentExists(ctx, organizationID, departmentID); err != nil {
		if errors.Is(err, departments.ErrNotFound) {
			return ErrInvalidReference
		}

		return fmt.Errorf("validate department: %w", err)
	}

	if err := s.references.EnsureJobRoleExists(ctx, organizationID, jobRoleID); err != nil {
		if errors.Is(err, departments.ErrRoleNotFound) {
			return ErrInvalidReference
		}

		return fmt.Errorf("validate job role: %w", err)
	}

	if managerEmployeeID == "" {
		return nil
	}

	_, err := s.store.GetByID(ctx, organizationID, managerEmployeeID)
	if errors.Is(err, ErrNotFound) {
		return ErrInvalidReference
	}

	if err != nil {
		return fmt.Errorf("validate manager: %w", err)
	}

	return nil
}
