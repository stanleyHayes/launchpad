package platform

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"launchpad/internal/entitlements"
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

// AccountCreator provisions a login account for a new staff member and
// returns the new user id.
type AccountCreator interface {
	CreateAccount(ctx context.Context, email, displayName, password string) (string, error)
}

// MailSender sends transactional email. Satisfied by email.Sender.
type MailSender interface {
	Send(ctx context.Context, to, subject, html string) error
}

// Service implements platform staff use cases.
type Service struct {
	repo           Repository
	orgs           OrganizationReader
	leads          LeadCounter
	support        OpenTicketCounter
	readiness      *ReadinessDeps
	accounts       AccountCreator
	mailer         MailSender
	revenueMetrics func(context.Context) (RevenueMetrics, error)
	supportMetrics func(context.Context) (SupportMetrics, error)
	storageMetrics func(context.Context) (StorageOverview, error)
	usageLoader    func(context.Context, organizations.Organization) (entitlements.Usage, error)
}

func (s *Service) WithOrganizationUsage(
	loader func(context.Context, organizations.Organization) (entitlements.Usage, error),
) *Service {
	s.usageLoader = loader
	return s
}

func (s *Service) OrganizationUsage(ctx context.Context, organizationID string) (entitlements.Usage, error) {
	org, err := s.GetOrganization(ctx, organizationID)
	if err != nil {
		return entitlements.Usage{}, err
	}
	if s.usageLoader == nil {
		return entitlements.Usage{}, ErrInvalidInput
	}
	usage, err := s.usageLoader(ctx, org)
	if err != nil {
		return entitlements.Usage{}, fmt.Errorf("load organization usage: %w", err)
	}
	return usage, nil
}

func (s *Service) WithStorageMetrics(loader func(context.Context) (StorageOverview, error)) *Service {
	s.storageMetrics = loader
	return s
}

func (s *Service) StorageOverview(ctx context.Context) (StorageOverview, error) {
	if s.storageMetrics == nil {
		return StorageOverview{}, ErrInvalidInput
	}
	overview, err := s.storageMetrics(ctx)
	if err != nil {
		return StorageOverview{}, fmt.Errorf("load storage metrics: %w", err)
	}
	return overview, nil
}

func (s *Service) WithBusinessMetrics(
	revenue func(context.Context) (RevenueMetrics, error),
	support func(context.Context) (SupportMetrics, error),
) *Service {
	s.revenueMetrics, s.supportMetrics = revenue, support
	return s
}

// NewService constructs a Service.
func NewService(repo Repository, orgs OrganizationReader, leadsSvc LeadCounter, supportSvc OpenTicketCounter) *Service {
	return &Service{
		repo: repo, orgs: orgs, leads: leadsSvc, support: supportSvc,
		readiness: nil, accounts: nil, mailer: nil,
	}
}

// WithAccounts wires staff account provisioning: account creation plus the
// optional invite sender (nil sender returns the temp password to the
// operator instead of emailing it).
func (s *Service) WithAccounts(accounts AccountCreator, mailer MailSender) *Service {
	s.accounts = accounts
	s.mailer = mailer

	return s
}

// GetByUserID returns an active staff record; deactivated records resolve to
// ErrNotFound so the platform login path rejects them (defense in depth on
// top of the store's active-only query).
func (s *Service) GetByUserID(ctx context.Context, userID string) (Staff, error) {
	staff, err := s.repo.GetByUserID(ctx, userID)
	if err != nil {
		return Staff{}, fmt.Errorf("get platform staff: %w", err)
	}

	if staff.Status != staffStatusActive {
		return Staff{}, ErrNotFound
	}
	if grant := staff.BreakGlass; grant != nil && grant.RevokedAt == nil && grant.ExpiresAt.After(time.Now().UTC()) {
		staff.RoleCode = grant.RoleCode
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

// ListOrganizations returns tenant organizations matching the optional
// platform-directory filters.
func (s *Service) ListOrganizations(
	ctx context.Context,
	input ...OrganizationListInput,
) ([]organizations.Organization, error) {
	items, err := s.orgs.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list organizations: %w", err)
	}

	if len(input) == 0 {
		return items, nil
	}

	filter := input[0].normalized()
	out := make([]organizations.Organization, 0, len(items))
	for _, item := range items {
		if filter.Status != "" && strings.ToLower(item.Status) != filter.Status {
			continue
		}
		if filter.PlanCode != "" && strings.ToLower(item.PlanCode) != filter.PlanCode {
			continue
		}
		if filter.Search != "" &&
			!strings.Contains(strings.ToLower(item.Name), filter.Search) &&
			!strings.Contains(strings.ToLower(item.Slug), filter.Search) {
			continue
		}
		out = append(out, item)
	}

	return out, nil
}

func (s *Service) ListOrganizationsPage(
	ctx context.Context,
	input OrganizationListInput,
) (OrganizationPage, error) {
	filter := input.normalized()
	items, err := s.ListOrganizations(ctx, filter)
	if err != nil {
		return OrganizationPage{}, err
	}
	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset := min(filter.Offset, len(items))
	end := min(offset+limit, len(items))
	return OrganizationPage{
		Items: items[offset:end], Total: len(items), Offset: offset, Limit: limit,
	}, nil
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
	var revenue RevenueMetrics
	if s.revenueMetrics != nil {
		revenue, err = s.revenueMetrics(ctx)
		if err != nil {
			return Overview{}, fmt.Errorf("load revenue metrics: %w", err)
		}
	}
	var support SupportMetrics
	if s.supportMetrics != nil {
		support, err = s.supportMetrics(ctx)
		if err != nil {
			return Overview{}, fmt.Errorf("load support metrics: %w", err)
		}
	}

	var total int64
	for _, count := range counts {
		total += count
	}

	return Overview{
		TotalOrgs:           total,
		TrialOrgs:           counts[organizations.StatusTrial()],
		ActiveOrgs:          counts[organizations.StatusActive()],
		SuspendedOrgs:       counts[organizations.StatusSuspended()],
		TotalLeads:          totalLeads,
		OpenTicketCount:     openTickets,
		OverdueTicketCount:  support.Overdue,
		UrgentTicketCount:   support.Urgent,
		MRRTotalCents:       revenue.MRRTotalCents,
		ARRTotalCents:       revenue.ARRTotalCents,
		ActiveSubscriptions: revenue.ActiveSubscriptions,
	}, nil
}
