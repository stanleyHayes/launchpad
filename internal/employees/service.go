package employees

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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

// RuleApplier applies assignment rules to a freshly created employee.
// Implemented by internal/assignments; attached after construction so the
// employees -> assignments -> employees construction cycle never forms.
type RuleApplier interface {
	ApplyAssignmentRules(ctx context.Context, employee Employee) error
}

// Service implements employee use cases.
type Service struct {
	repo        Repository
	references  ReferenceChecker
	ruleApplier RuleApplier
}

// NewService constructs a Service.
func NewService(repo Repository, references ReferenceChecker) *Service {
	return &Service{repo: repo, references: references, ruleApplier: nil}
}

// SetRuleApplier attaches the assignment rule applier invoked after every
// successful Create. Nil-safe: without an applier, Create is unchanged.
func (s *Service) SetRuleApplier(applier RuleApplier) {
	s.ruleApplier = applier
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
	mobilePhone := strings.TrimSpace(in.MobilePhone)
	if mobilePhone != "" && (!strings.HasPrefix(mobilePhone, "+") || len(mobilePhone) < 8) {
		return Employee{}, ErrInvalidInput
	}

	if err := s.validateReferences(
		ctx,
		organizationID,
		in.DepartmentID,
		in.JobRoleID,
		in.ManagerEmployeeID,
		in.BuddyEmployeeID,
	); err != nil {
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
		MobilePhone:       mobilePhone,
		JobRoleID:         strings.TrimSpace(in.JobRoleID),
		DepartmentID:      strings.TrimSpace(in.DepartmentID),
		ManagerEmployeeID: strings.TrimSpace(in.ManagerEmployeeID),
		BuddyEmployeeID:   strings.TrimSpace(in.BuddyEmployeeID),
		Team:              strings.TrimSpace(in.Team),
		Location:          strings.TrimSpace(in.Location),
		StartDate:         in.StartDate.UTC(),
		Status:            statusInvited,
		Metadata:          map[string]any{},
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := s.repo.Create(ctx, employee); err != nil {
		return Employee{}, fmt.Errorf("create employee: %w", err)
	}

	// The employee is already persisted; a rule application failure must not
	// turn the committed create into a reported failure. Log and continue.
	if s.ruleApplier != nil {
		if err := s.ruleApplier.ApplyAssignmentRules(ctx, employee); err != nil {
			slog.ErrorContext(ctx, "apply assignment rules failed", "employeeId", employee.ID, "error", err)
		}
	}

	return employee, nil
}

// List lists employees for an organization.
func (s *Service) List(ctx context.Context, organizationID string, offset, limit int64) ([]Employee, error) {
	if organizationID == "" {
		return nil, ErrInvalidInput
	}

	items, err := s.repo.List(ctx, organizationID, offset, limit)
	if err != nil {
		return nil, fmt.Errorf("list employees: %w", err)
	}

	return items, nil
}

// Count returns the number of employees in an organization.
func (s *Service) Count(ctx context.Context, organizationID string) (int64, error) {
	if organizationID == "" {
		return 0, ErrInvalidInput
	}

	count, err := s.repo.Count(ctx, organizationID)
	if err != nil {
		return 0, fmt.Errorf("count employees: %w", err)
	}

	return count, nil
}

func (s *Service) ContactsForUser(
	ctx context.Context, organizationID, userID string,
) ([]Contact, error) {
	employee, err := s.GetByUserID(ctx, organizationID, userID)
	if err != nil {
		return nil, err
	}
	refs := []struct{ kind, id string }{
		{"manager", employee.ManagerEmployeeID}, {"buddy", employee.BuddyEmployeeID},
	}
	contacts := make([]Contact, 0, 2)
	for _, ref := range refs {
		if ref.id == "" {
			continue
		}
		person, getErr := s.Get(ctx, organizationID, ref.id)
		if getErr != nil {
			continue
		}
		contacts = append(contacts, Contact{
			ID: person.ID, Kind: ref.kind, Name: person.FirstName + " " + person.LastName,
			WorkEmail: person.WorkEmail, Team: person.Team, Location: person.Location,
		})
	}
	return contacts, nil
}

// Get returns one employee.
func (s *Service) Get(ctx context.Context, organizationID, employeeID string) (Employee, error) {
	if organizationID == "" || employeeID == "" {
		return Employee{}, ErrInvalidInput
	}

	employee, err := s.repo.GetByID(ctx, organizationID, employeeID)
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

	employee, err := s.repo.GetByUserID(ctx, organizationID, userID)
	if err != nil {
		return Employee{}, fmt.Errorf("get employee by user: %w", err)
	}

	return employee, nil
}

// GetByWorkEmail returns the employee with a work email in an organization.
// The email is normalized (trimmed, lowercased) to match stored records.
func (s *Service) GetByWorkEmail(ctx context.Context, organizationID, workEmail string) (Employee, error) {
	email := strings.ToLower(strings.TrimSpace(workEmail))
	if organizationID == "" || email == "" {
		return Employee{}, ErrInvalidInput
	}

	employee, err := s.repo.GetByWorkEmail(ctx, organizationID, email)
	if err != nil {
		return Employee{}, fmt.Errorf("get employee by work email: %w", err)
	}

	return employee, nil
}

// ProvisionAccess links an employee record to an externally created user.
func (s *Service) ProvisionAccess(ctx context.Context, organizationID, employeeID, userID string) error {
	if organizationID == "" || employeeID == "" || userID == "" {
		return ErrInvalidInput
	}

	if err := s.repo.ProvisionAccess(ctx, organizationID, employeeID, userID); err != nil {
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
	employee, err := s.repo.GetByID(ctx, organizationID, employeeID)
	if err != nil {
		return Employee{}, fmt.Errorf("get employee for update: %w", err)
	}

	if err := applyEmployeeUpdate(&employee, in); err != nil {
		return Employee{}, err
	}

	if employee.ManagerEmployeeID != "" && employee.ManagerEmployeeID == employeeID {
		return Employee{}, ErrInvalidReference
	}

	if employee.BuddyEmployeeID != "" && employee.BuddyEmployeeID == employeeID {
		return Employee{}, ErrInvalidReference
	}

	if err := s.validateReferences(
		ctx,
		organizationID,
		employee.DepartmentID,
		employee.JobRoleID,
		employee.ManagerEmployeeID,
		employee.BuddyEmployeeID,
	); err != nil {
		return Employee{}, err
	}

	employee.UpdatedAt = time.Now().UTC()
	if err := s.repo.Update(ctx, employee); err != nil {
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
	if in.MobilePhone != nil {
		phone := strings.TrimSpace(*in.MobilePhone)
		if phone != "" && (!strings.HasPrefix(phone, "+") || len(phone) < 8) {
			return ErrInvalidInput
		}
		employee.MobilePhone = phone
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

	if in.BuddyEmployeeID != nil {
		employee.BuddyEmployeeID = strings.TrimSpace(*in.BuddyEmployeeID)
	}

	if in.Team != nil {
		employee.Team = strings.TrimSpace(*in.Team)
	}
	if in.Location != nil {
		employee.Location = strings.TrimSpace(*in.Location)
	}

	if in.Status != nil {
		if err := applyEmployeeStatus(employee, *in.Status); err != nil {
			return err
		}
	}

	return nil
}

func applyEmployeeStatus(employee *Employee, raw string) error {
	status := strings.TrimSpace(raw)
	switch status {
	case statusInvited, statusActive, statusOffboarded:
		employee.Status = status
	default:
		return ErrInvalidInput
	}

	return nil
}

// Offboard marks an employee as offboarded. Offboarded employees are
// excluded from department bulk-assign (assignments requires status active).
func (s *Service) Offboard(ctx context.Context, organizationID, employeeID string) (Employee, error) {
	status := statusOffboarded

	return s.Update(ctx, organizationID, employeeID, UpdateInput{
		FirstName:         nil,
		LastName:          nil,
		EmployeeNumber:    nil,
		JobRoleID:         nil,
		DepartmentID:      nil,
		ManagerEmployeeID: nil,
		BuddyEmployeeID:   nil,
		Status:            &status,
	})
}

// LinkUser attaches a user account to an invited employee. The link is a
// single conditional update, so a concurrent provision cannot overwrite an
// existing link.
func (s *Service) LinkUser(ctx context.Context, organizationID, employeeID, userID string) (Employee, error) {
	if organizationID == "" || employeeID == "" || userID == "" {
		return Employee{}, ErrInvalidInput
	}

	if err := s.repo.ProvisionAccess(ctx, organizationID, employeeID, userID); err != nil {
		return Employee{}, fmt.Errorf("link employee user: %w", err)
	}

	employee, err := s.repo.GetByID(ctx, organizationID, employeeID)
	if err != nil {
		return Employee{}, fmt.Errorf("get employee after link: %w", err)
	}

	return employee, nil
}

func (s *Service) validateReferences(
	ctx context.Context,
	organizationID, departmentID, jobRoleID, managerEmployeeID, buddyEmployeeID string,
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

	for _, referenceID := range []string{managerEmployeeID, buddyEmployeeID} {
		if referenceID == "" {
			continue
		}

		_, err := s.repo.GetByID(ctx, organizationID, referenceID)
		if errors.Is(err, ErrNotFound) {
			return ErrInvalidReference
		}

		if err != nil {
			return fmt.Errorf("validate employee reference: %w", err)
		}
	}

	return nil
}
