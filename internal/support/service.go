package support

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Service implements support ticket use cases.
type Service struct {
	repo Repository
}

// NewService constructs a Service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// Create creates a support ticket for a tenant.
func (s *Service) Create(ctx context.Context, in CreateTicketInput) (Ticket, error) {
	subject := strings.TrimSpace(in.Subject)
	body := strings.TrimSpace(in.Body)
	priority := strings.TrimSpace(in.Priority)

	if in.OrganizationID == "" || in.CreatedByUserID == "" || subject == "" || body == "" {
		return Ticket{}, ErrInvalidInput
	}

	if priority == "" {
		priority = priorityNormal
	}

	if !isValidPriority(priority) {
		return Ticket{}, ErrInvalidInput
	}

	now := time.Now().UTC()

	ticket := Ticket{
		ID:              uuid.NewString(),
		OrganizationID:  in.OrganizationID,
		CreatedByUserID: in.CreatedByUserID,
		Subject:         subject,
		Body:            body,
		Priority:        priority,
		Status:          statusOpen,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := s.repo.Create(ctx, ticket); err != nil {
		return Ticket{}, fmt.Errorf("create support ticket: %w", err)
	}

	return ticket, nil
}

// GetForOrganization returns one ticket scoped to a tenant.
func (s *Service) GetForOrganization(ctx context.Context, organizationID, ticketID string) (Ticket, error) {
	ticket, err := s.repo.GetByIDForOrganization(ctx, organizationID, ticketID)
	if err != nil {
		return Ticket{}, fmt.Errorf("get support ticket: %w", err)
	}

	return ticket, nil
}

// ListForOrganization returns tickets for a tenant.
func (s *Service) ListForOrganization(ctx context.Context, organizationID string) ([]Ticket, error) {
	items, err := s.repo.ListByOrganization(ctx, organizationID)
	if err != nil {
		return nil, fmt.Errorf("list support tickets: %w", err)
	}

	return items, nil
}

// Get returns one ticket for platform review.
func (s *Service) Get(ctx context.Context, ticketID string) (Ticket, error) {
	ticket, err := s.repo.GetByID(ctx, ticketID)
	if err != nil {
		return Ticket{}, fmt.Errorf("get support ticket: %w", err)
	}

	return ticket, nil
}

// List returns all tickets for platform review.
func (s *Service) List(ctx context.Context) ([]Ticket, error) {
	items, err := s.repo.ListAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("list support tickets: %w", err)
	}

	return items, nil
}

// UpdateStatus updates ticket workflow state.
func (s *Service) UpdateStatus(ctx context.Context, in UpdateTicketStatusInput) (Ticket, error) {
	status := strings.TrimSpace(in.Status)
	if in.TicketID == "" || !isValidStatus(status) {
		return Ticket{}, ErrInvalidInput
	}

	ticket, err := s.repo.GetByID(ctx, in.TicketID)
	if err != nil {
		return Ticket{}, fmt.Errorf("get support ticket: %w", err)
	}

	ticket.Status = status
	if in.AssigneeUserID != nil {
		ticket.AssigneeUserID = strings.TrimSpace(*in.AssigneeUserID)
	}

	ticket.UpdatedAt = time.Now().UTC()
	if err := s.repo.Update(ctx, ticket); err != nil {
		return Ticket{}, fmt.Errorf("update support ticket: %w", err)
	}

	return ticket, nil
}

// CountOpen returns open support tickets for platform metrics.
func (s *Service) CountOpen(ctx context.Context) (int64, error) {
	count, err := s.repo.CountOpen(ctx)
	if err != nil {
		return 0, fmt.Errorf("count open support tickets: %w", err)
	}

	return count, nil
}

func isValidPriority(priority string) bool {
	return priority == priorityLow ||
		priority == priorityNormal ||
		priority == priorityHigh ||
		priority == priorityUrgent
}

func isValidStatus(status string) bool {
	return status == statusOpen ||
		status == statusInProgress ||
		status == statusWaiting ||
		status == statusResolved ||
		status == statusClosed
}
