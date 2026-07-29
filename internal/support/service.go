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
		Category:        strings.TrimSpace(in.Category),
		Status:          statusOpen,
		SLADueAt:        now.Add(slaDuration(priority)),
		Tags:            []string{},
		Messages:        []TicketMessage{},
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := s.repo.Create(ctx, ticket); err != nil {
		return Ticket{}, fmt.Errorf("create support ticket: %w", err)
	}

	return ticket, nil
}

func slaDuration(priority string) time.Duration {
	switch priority {
	case priorityUrgent:
		return 4 * time.Hour
	case priorityHigh:
		return 8 * time.Hour
	case priorityNormal:
		return 24 * time.Hour
	default:
		return 72 * time.Hour
	}
}

// ReportBlocker records an employee blocker and opens a high-priority support
// ticket so the blocker also appears in the organization's support queue.
func (s *Service) ReportBlocker(ctx context.Context, in ReportBlockerInput) (Blocker, error) {
	category := strings.TrimSpace(in.Category)
	message := strings.TrimSpace(in.Message)

	if in.OrganizationID == "" || in.EmployeeID == "" || in.ReportedByUserID == "" ||
		message == "" || !isValidBlockerCategory(category) {
		return Blocker{}, ErrInvalidInput
	}

	subject := "Blocker (" + category + ")"
	if in.EmployeeName != "" {
		subject += ": " + in.EmployeeName
	}

	body := message
	if in.StepTitle != "" {
		body = "Step: " + in.StepTitle + "\n\n" + message
	}

	ticket, err := s.Create(ctx, CreateTicketInput{
		OrganizationID:  in.OrganizationID,
		CreatedByUserID: in.ReportedByUserID,
		Subject:         subject,
		Body:            body,
		Priority:        priorityHigh,
		Category:        category,
	})
	if err != nil {
		return Blocker{}, fmt.Errorf("create blocker ticket: %w", err)
	}

	blocker := Blocker{
		ID:               uuid.NewString(),
		OrganizationID:   in.OrganizationID,
		EmployeeID:       in.EmployeeID,
		ReportedByUserID: in.ReportedByUserID,
		StepAssignmentID: strings.TrimSpace(in.StepAssignmentID),
		Category:         category,
		Message:          message,
		TicketID:         ticket.ID,
		CreatedAt:        time.Now().UTC(),
	}
	if err := s.repo.CreateBlocker(ctx, blocker); err != nil {
		return Blocker{}, fmt.Errorf("create blocker: %w", err)
	}

	return blocker, nil
}

// ListBlockers returns blockers for a tenant, newest first.
func (s *Service) ListBlockers(ctx context.Context, organizationID string) ([]Blocker, error) {
	items, err := s.repo.ListBlockers(ctx, organizationID)
	if err != nil {
		return nil, fmt.Errorf("list blockers: %w", err)
	}

	return items, nil
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
	if (status == statusResolved || status == statusClosed) && ticket.ResolvedAt == nil {
		now := ticket.UpdatedAt
		ticket.ResolvedAt = &now
	}
	if err := s.repo.Update(ctx, ticket); err != nil {
		return Ticket{}, fmt.Errorf("update support ticket: %w", err)
	}

	return ticket, nil
}

func (s *Service) AddMessage(ctx context.Context, ticketID, authorUserID, body string, internal bool) (Ticket, error) {
	body = strings.TrimSpace(body)
	if ticketID == "" || authorUserID == "" || body == "" {
		return Ticket{}, ErrInvalidInput
	}
	ticket, err := s.repo.GetByID(ctx, ticketID)
	if err != nil {
		return Ticket{}, fmt.Errorf("get support ticket: %w", err)
	}
	now := time.Now().UTC()
	ticket.Messages = append(ticket.Messages, TicketMessage{
		ID: uuid.NewString(), AuthorUserID: authorUserID, Body: body, Internal: internal, CreatedAt: now,
	})
	if ticket.FirstResponseAt == nil && authorUserID != ticket.CreatedByUserID {
		ticket.FirstResponseAt = &now
	}
	ticket.UpdatedAt = now
	if err := s.repo.Update(ctx, ticket); err != nil {
		return Ticket{}, fmt.Errorf("add support message: %w", err)
	}
	return ticket, nil
}

func (s *Service) Escalate(ctx context.Context, ticketID, assigneeUserID string) (Ticket, error) {
	ticket, err := s.repo.GetByID(ctx, ticketID)
	if err != nil {
		return Ticket{}, fmt.Errorf("get support ticket: %w", err)
	}
	if ticket.Status == statusClosed || ticket.Status == statusResolved {
		return Ticket{}, ErrInvalidInput
	}
	ticket.Priority = priorityUrgent
	ticket.EscalationCount++
	ticket.AssigneeUserID = strings.TrimSpace(assigneeUserID)
	ticket.SLADueAt = time.Now().UTC().Add(slaDuration(priorityUrgent))
	ticket.UpdatedAt = time.Now().UTC()
	if err := s.repo.Update(ctx, ticket); err != nil {
		return Ticket{}, fmt.Errorf("escalate support ticket: %w", err)
	}
	return ticket, nil
}

func (s *Service) Summary(ctx context.Context) (SupportSummary, error) {
	items, err := s.List(ctx)
	if err != nil {
		return SupportSummary{}, err
	}
	now := time.Now().UTC()
	var summary SupportSummary
	var responseMinutes float64
	var responses int
	for _, ticket := range items {
		if ticket.Status == statusOpen || ticket.Status == statusInProgress || ticket.Status == statusWaiting {
			summary.Open++
			if !ticket.SLADueAt.IsZero() && ticket.SLADueAt.Before(now) {
				summary.Overdue++
			}
			if ticket.AssigneeUserID == "" {
				summary.Unassigned++
			}
		}
		if ticket.Priority == priorityUrgent {
			summary.Urgent++
		}
		if ticket.FirstResponseAt != nil {
			responseMinutes += ticket.FirstResponseAt.Sub(ticket.CreatedAt).Minutes()
			responses++
		}
	}
	if responses > 0 {
		summary.AverageFirstResponseMinutes = responseMinutes / float64(responses)
	}
	return summary, nil
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

func isValidBlockerCategory(category string) bool {
	return category == categoryHR ||
		category == categoryIT ||
		category == categoryManager ||
		category == categoryOther
}
