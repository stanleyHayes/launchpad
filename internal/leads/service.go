package leads

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Service implements lead use cases.
type Service struct {
	repo Repository
}

// NewService constructs a Service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// Create captures a new lead from the public form.
func (s *Service) Create(ctx context.Context, in CreateInput) (Lead, error) {
	name := strings.TrimSpace(in.Name)
	email := strings.ToLower(strings.TrimSpace(in.Email))
	company := strings.TrimSpace(in.Company)
	message := strings.TrimSpace(in.Message)
	source := strings.TrimSpace(in.Source)

	if name == "" || email == "" || !strings.Contains(email, "@") {
		return Lead{}, ErrInvalidInput
	}

	if source == "" {
		source = defaultSource
	}

	lead := Lead{
		ID:           uuid.NewString(),
		Name:         name,
		Email:        email,
		Company:      company,
		Message:      message,
		Source:       source,
		UTMSource:    strings.TrimSpace(in.UTMSource),
		UTMMedium:    strings.TrimSpace(in.UTMMedium),
		UTMCampaign:  strings.TrimSpace(in.UTMCampaign),
		ScheduledFor: in.ScheduledFor,
		Status:       statusNew,
		CreatedAt:    time.Now().UTC(),
	}
	if err := s.repo.Create(ctx, lead); err != nil {
		return Lead{}, fmt.Errorf("create lead: %w", err)
	}

	return lead, nil
}

// List returns a page of leads for platform review, newest first. before is
// the keyset cursor: when non-zero, only leads created before it are returned.
func (s *Service) List(ctx context.Context, limit int64, before time.Time) ([]Lead, error) {
	items, err := s.repo.List(ctx, limit, before)
	if err != nil {
		return nil, fmt.Errorf("list leads: %w", err)
	}

	return items, nil
}

// Count returns the total number of leads.
func (s *Service) Count(ctx context.Context) (int64, error) {
	count, err := s.repo.Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("count leads: %w", err)
	}

	return count, nil
}
