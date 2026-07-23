package organizations

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Service implements organization use cases.
type Service struct {
	repo Repository
}

// NewService constructs a Service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// CreateWithOwner creates an organization and owner membership.
func (s *Service) CreateWithOwner(ctx context.Context, in CreateInput) (Organization, Membership, error) {
	name := strings.TrimSpace(in.Name)
	slug := strings.ToLower(strings.TrimSpace(in.Slug))

	if name == "" || !slugPattern().MatchString(slug) {
		return Organization{}, Membership{}, ErrInvalidInput
	}

	timezone := strings.TrimSpace(in.Timezone)
	if timezone == "" {
		timezone = defaultTimezone
	}

	now := time.Now().UTC()

	org := Organization{
		ID:        uuid.NewString(),
		Name:      name,
		Slug:      slug,
		Status:    statusTrial,
		PlanCode:  planStarter,
		Timezone:  timezone,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.repo.CreateOrganization(ctx, org); err != nil {
		return Organization{}, Membership{}, fmt.Errorf("create organization: %w", err)
	}

	membership := Membership{
		ID:             uuid.NewString(),
		OrganizationID: org.ID,
		UserID:         in.OwnerID,
		RoleCode:       roleOrganizationOwner,
		Status:         membershipStatusActive,
		CreatedAt:      now,
	}
	if err := s.repo.CreateMembership(ctx, membership); err != nil {
		return Organization{}, Membership{}, fmt.Errorf("create organization membership: %w", err)
	}

	return org, membership, nil
}

// Get returns an organization by id.
func (s *Service) Get(ctx context.Context, id string) (Organization, error) {
	org, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return Organization{}, fmt.Errorf("get organization: %w", err)
	}

	return org, nil
}

// Update updates mutable organization fields.
func (s *Service) Update(ctx context.Context, id string, in UpdateInput) (Organization, error) {
	org, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return Organization{}, fmt.Errorf("get organization: %w", err)
	}

	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" {
			return Organization{}, ErrInvalidInput
		}

		org.Name = name
	}

	if in.Timezone != nil {
		timezone := strings.TrimSpace(*in.Timezone)
		if timezone == "" {
			return Organization{}, ErrInvalidInput
		}

		org.Timezone = timezone
	}

	org.UpdatedAt = time.Now().UTC()
	if err := s.repo.Update(ctx, org); err != nil {
		return Organization{}, fmt.Errorf("update organization: %w", err)
	}

	return org, nil
}

// Membership returns an active membership.
func (s *Service) Membership(ctx context.Context, organizationID, userID string) (Membership, error) {
	membership, err := s.repo.GetMembership(ctx, organizationID, userID)
	if err != nil {
		return Membership{}, fmt.Errorf("get organization membership: %w", err)
	}

	return membership, nil
}

// ListMembershipsForUser lists active memberships for a user.
func (s *Service) ListMembershipsForUser(ctx context.Context, userID string) ([]Membership, error) {
	memberships, err := s.repo.ListMembershipsByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list organization memberships: %w", err)
	}

	return memberships, nil
}

// CanManageOrganization reports whether a role may update org settings.
func CanManageOrganization(roleCode string) bool {
	return roleCode == roleOrganizationOwner || roleCode == roleHRAdmin
}

// RoleEmployee is the membership role for onboarded employees.
func RoleEmployee() string {
	return roleEmployee
}

// AddMember adds an active membership for a user.
func (s *Service) AddMember(
	ctx context.Context,
	organizationID, userID, roleCode string,
) (Membership, error) {
	if organizationID == "" || userID == "" || roleCode == "" {
		return Membership{}, ErrInvalidInput
	}

	membership := Membership{
		ID:             uuid.NewString(),
		OrganizationID: organizationID,
		UserID:         userID,
		RoleCode:       roleCode,
		Status:         membershipStatusActive,
		CreatedAt:      time.Now().UTC(),
	}
	if err := s.repo.CreateMembership(ctx, membership); err != nil {
		return Membership{}, fmt.Errorf("create membership: %w", err)
	}

	return membership, nil
}
