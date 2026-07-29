package requests

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"launchpad/internal/employees"
)

// EmployeeReader resolves the employee behind a user account.
type EmployeeReader interface {
	GetByUserID(ctx context.Context, organizationID, userID string) (employees.Employee, error)
}

// Service implements equipment and access request use cases.
type Service struct {
	repo      Repository
	employees EmployeeReader
}

// NewService constructs a Service. The employee reader is optional so the
// service can be wired for journey-step auto-creation alone; self-service
// endpoints report ErrInvalidState without it.
func NewService(repo Repository, employeeReaders ...EmployeeReader) *Service {
	var employeeReader EmployeeReader
	if len(employeeReaders) > 0 {
		employeeReader = employeeReaders[0]
	}

	return &Service{repo: repo, employees: employeeReader}
}

// Create raises a request for an employee (also used by journey-step
// auto-creation).
func (s *Service) Create(ctx context.Context, in CreateInput) (Request, error) {
	kind := strings.TrimSpace(in.Kind)
	item := strings.TrimSpace(in.Item)
	details := strings.TrimSpace(in.Details)

	if in.OrganizationID == "" || in.EmployeeID == "" || !isValidKind(kind) || !isValidItem(item) {
		return Request{}, ErrInvalidInput
	}

	now := time.Now().UTC()

	request := Request{
		ID:                  uuid.NewString(),
		OrganizationID:      in.OrganizationID,
		Kind:                kind,
		Item:                item,
		Details:             details,
		Status:              statusPending,
		RequesterEmployeeID: in.EmployeeID,
		ApproverUserID:      "",
		DecisionNote:        "",
		DecidedAt:           nil,
		FulfilledAt:         nil,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	if err := s.repo.Create(ctx, request); err != nil {
		return Request{}, fmt.Errorf("create request: %w", err)
	}

	return request, nil
}

// CreateFromStep auto-creates a request when an equipment_request or
// access_request journey step completes (assignments.RequestCreator port).
// An empty or unknown item falls back to "other" so a step is never blocked
// by a partial submission.
func (s *Service) CreateFromStep(
	ctx context.Context,
	organizationID, employeeID, kind, item, details string,
) error {
	if !isValidItem(item) {
		item = itemOther
	}

	_, err := s.Create(ctx, CreateInput{
		OrganizationID: organizationID,
		EmployeeID:     employeeID,
		Kind:           kind,
		Item:           item,
		Details:        details,
	})

	return err
}

// CreateMine raises a request for the employee behind a user account.
func (s *Service) CreateMine(ctx context.Context, organizationID, userID string, in CreateInput) (Request, error) {
	employee, err := s.resolveEmployee(ctx, organizationID, userID)
	if err != nil {
		return Request{}, err
	}

	in.OrganizationID = organizationID
	in.EmployeeID = employee.ID

	return s.Create(ctx, in)
}

// ListMine returns the caller's own requests, newest first.
func (s *Service) ListMine(ctx context.Context, organizationID, userID string) ([]Request, error) {
	employee, err := s.resolveEmployee(ctx, organizationID, userID)
	if err != nil {
		return nil, err
	}

	items, err := s.repo.ListByRequester(ctx, organizationID, employee.ID)
	if err != nil {
		return nil, fmt.Errorf("list requests: %w", err)
	}

	return items, nil
}

// CancelMine cancels one of the caller's own pending requests.
func (s *Service) CancelMine(ctx context.Context, organizationID, userID, requestID string) (Request, error) {
	employee, err := s.resolveEmployee(ctx, organizationID, userID)
	if err != nil {
		return Request{}, err
	}

	request, err := s.repo.GetByIDForOrganization(ctx, organizationID, requestID)
	if err != nil {
		return Request{}, fmt.Errorf("get request: %w", err)
	}

	if request.RequesterEmployeeID != employee.ID {
		return Request{}, ErrForbidden
	}

	if request.Status != statusPending {
		return Request{}, ErrInvalidState
	}

	request.Status = statusCancelled

	request.UpdatedAt = time.Now().UTC()
	if err := s.repo.Update(ctx, request); err != nil {
		return Request{}, fmt.Errorf("cancel request: %w", err)
	}

	return request, nil
}

// List returns requests for a tenant, optionally filtered by status.
func (s *Service) List(ctx context.Context, organizationID, status string) ([]Request, error) {
	status = strings.TrimSpace(status)
	if status != "" && !isValidStatus(status) {
		return nil, ErrInvalidInput
	}

	items, err := s.repo.ListByOrganization(ctx, organizationID, status)
	if err != nil {
		return nil, fmt.Errorf("list requests: %w", err)
	}

	return items, nil
}

// Decide approves or rejects a pending request.
func (s *Service) Decide(ctx context.Context, in DecideInput) (Request, error) {
	if in.OrganizationID == "" || in.RequestID == "" || in.ApproverUserID == "" {
		return Request{}, ErrInvalidInput
	}

	request, err := s.repo.GetByIDForOrganization(ctx, in.OrganizationID, in.RequestID)
	if err != nil {
		return Request{}, fmt.Errorf("get request: %w", err)
	}

	if request.Status != statusPending {
		return Request{}, ErrInvalidState
	}

	now := time.Now().UTC()
	request.ApproverUserID = in.ApproverUserID
	request.DecisionNote = strings.TrimSpace(in.Note)
	request.DecidedAt = &now
	request.UpdatedAt = now

	if in.Approve {
		request.Status = statusApproved
	} else {
		request.Status = statusRejected
	}

	if err := s.repo.Update(ctx, request); err != nil {
		return Request{}, fmt.Errorf("decide request: %w", err)
	}

	return request, nil
}

// Fulfill marks an approved request as provisioned.
func (s *Service) Fulfill(ctx context.Context, organizationID, requestID string) (Request, error) {
	if organizationID == "" || requestID == "" {
		return Request{}, ErrInvalidInput
	}

	request, err := s.repo.GetByIDForOrganization(ctx, organizationID, requestID)
	if err != nil {
		return Request{}, fmt.Errorf("get request: %w", err)
	}

	if request.Status != statusApproved {
		return Request{}, ErrInvalidState
	}

	now := time.Now().UTC()
	request.Status = statusFulfilled
	request.FulfilledAt = &now

	request.UpdatedAt = now
	if err := s.repo.Update(ctx, request); err != nil {
		return Request{}, fmt.Errorf("fulfill request: %w", err)
	}

	return request, nil
}

func (s *Service) resolveEmployee(
	ctx context.Context,
	organizationID, userID string,
) (employees.Employee, error) {
	if s.employees == nil {
		return employees.Employee{}, ErrInvalidState
	}

	employee, err := s.employees.GetByUserID(ctx, organizationID, userID)
	if err != nil {
		if errors.Is(err, employees.ErrNotFound) {
			return employees.Employee{}, ErrForbidden
		}

		return employees.Employee{}, fmt.Errorf("resolve employee: %w", err)
	}

	return employee, nil
}
