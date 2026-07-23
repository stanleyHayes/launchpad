package platform

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"launchpad/internal/organizations"
)

// OrganizationReader loads tenant organizations for platform operations.
type OrganizationReader interface {
	List(ctx context.Context) ([]organizations.Organization, error)
	Get(ctx context.Context, id string) (organizations.Organization, error)
	SetStatus(ctx context.Context, id, status string) (organizations.Organization, error)
	CountByStatus(ctx context.Context) (map[string]int64, error)
}

// LeadCounter counts captured leads.
type LeadCounter interface {
	Count(ctx context.Context) (int64, error)
}

// OpenTicketCounter counts open support tickets.
type OpenTicketCounter interface {
	CountOpen(ctx context.Context) (int64, error)
}

// Service implements platform staff use cases.
type Service struct {
	repo    Repository
	orgs    OrganizationReader
	leads   LeadCounter
	support OpenTicketCounter
}

// NewService constructs a Service.
func NewService(repo Repository, orgs OrganizationReader, leadsSvc LeadCounter, supportSvc OpenTicketCounter) *Service {
	return &Service{repo: repo, orgs: orgs, leads: leadsSvc, support: supportSvc}
}

// GetByUserID returns an active staff record.
func (s *Service) GetByUserID(ctx context.Context, userID string) (Staff, error) {
	staff, err := s.repo.GetByUserID(ctx, userID)
	if err != nil {
		return Staff{}, fmt.Errorf("get platform staff: %w", err)
	}

	return staff, nil
}

// StaffRoleByUserID implements auth.PlatformStaffReader.
func (s *Service) StaffRoleByUserID(ctx context.Context, userID string) (string, error) {
	staff, err := s.GetByUserID(ctx, userID)
	if err != nil {
		return "", err
	}

	return staff.RoleCode, nil
}

// EnsureStaff creates a staff record when one does not already exist.
func (s *Service) EnsureStaff(ctx context.Context, userID, roleCode string) (Staff, error) {
	userID = strings.TrimSpace(userID)
	roleCode = strings.TrimSpace(roleCode)

	if userID == "" || (roleCode != rolePlatformOwner && roleCode != rolePlatformAdmin) {
		return Staff{}, ErrInvalidInput
	}

	existing, err := s.repo.GetByUserID(ctx, userID)
	if err == nil {
		return existing, nil
	}

	if !errors.Is(err, ErrNotFound) {
		return Staff{}, fmt.Errorf("lookup platform staff: %w", err)
	}

	staff := Staff{
		ID:        uuid.NewString(),
		UserID:    userID,
		RoleCode:  roleCode,
		Status:    staffStatusActive,
		CreatedAt: time.Now().UTC(),
	}
	if err := s.repo.Create(ctx, staff); err != nil {
		return Staff{}, fmt.Errorf("create platform staff: %w", err)
	}

	return staff, nil
}

// ListOrganizations returns all tenant organizations.
func (s *Service) ListOrganizations(ctx context.Context) ([]organizations.Organization, error) {
	items, err := s.orgs.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list organizations: %w", err)
	}

	return items, nil
}

// GetOrganization returns one tenant organization.
func (s *Service) GetOrganization(ctx context.Context, organizationID string) (organizations.Organization, error) {
	org, err := s.orgs.Get(ctx, organizationID)
	if err != nil {
		return organizations.Organization{}, fmt.Errorf("get organization: %w", err)
	}

	return org, nil
}

// SetOrganizationStatus updates a tenant organization status.
func (s *Service) SetOrganizationStatus(
	ctx context.Context,
	organizationID, status string,
) (organizations.Organization, error) {
	org, err := s.orgs.SetStatus(ctx, organizationID, status)
	if err != nil {
		return organizations.Organization{}, fmt.Errorf("set organization status: %w", err)
	}

	return org, nil
}

// Overview returns platform-wide metrics.
func (s *Service) Overview(ctx context.Context) (Overview, error) {
	counts, err := s.orgs.CountByStatus(ctx)
	if err != nil {
		return Overview{}, fmt.Errorf("count organizations: %w", err)
	}

	totalLeads, err := s.leads.Count(ctx)
	if err != nil {
		return Overview{}, fmt.Errorf("count leads: %w", err)
	}

	openTickets, err := s.support.CountOpen(ctx)
	if err != nil {
		return Overview{}, fmt.Errorf("count open support tickets: %w", err)
	}

	var total int64
	for _, count := range counts {
		total += count
	}

	return Overview{
		TotalOrgs:       total,
		TrialOrgs:       counts[organizations.StatusTrial()],
		ActiveOrgs:      counts[organizations.StatusActive()],
		SuspendedOrgs:   counts[organizations.StatusSuspended()],
		TotalLeads:      totalLeads,
		OpenTicketCount: openTickets,
	}, nil
}
