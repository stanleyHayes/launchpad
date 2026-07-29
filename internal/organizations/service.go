package organizations

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Service implements organization use cases.
type Service struct {
	repo     Repository
	accounts AccountCreator
	users    MemberUserReader
	roles    RoleChecker
}

// AccountCreator provisions login accounts for invited members. It is the
// service-side port for the invite orchestration (see InviteMember).
type AccountCreator interface {
	// CreateUserAccount creates an account, returning ErrInviteEmailTaken when
	// the email is already registered.
	CreateUserAccount(ctx context.Context, email, displayName, password string) (userID string, err error)
	// FindUserByEmail resolves an existing account's user id by email.
	FindUserByEmail(ctx context.Context, email string) (userID string, err error)
}

// MemberUserReader loads display info for member accounts. It is the
// service-side port for member listings; the app layer wires the auth user
// store (this package cannot import auth without an import cycle).
type MemberUserReader interface {
	GetMemberUser(ctx context.Context, userID string) (MemberUser, error)
}

// RoleChecker reports whether a role code is assignable in an organization —
// a built-in role or the name of an existing custom role. It is the
// service-side port; the app layer wires the roles service.
type RoleChecker interface {
	RoleExists(ctx context.Context, organizationID, roleCode string) (bool, error)
}

// NewService constructs a Service. accounts may be nil when the invite flow
// is not wired (InviteMember then fails with ErrInvalidInput); users and
// roles may be nil, in which case member listings omit display info and only
// built-in role codes are assignable.
func NewService(
	repo Repository,
	accounts AccountCreator,
	users MemberUserReader,
	roles RoleChecker,
) *Service {
	return &Service{repo: repo, accounts: accounts, users: users, roles: roles}
}

// SetRoleChecker wires the custom-role checker after construction, breaking
// the organizations <-> roles construction cycle in the app layer (the roles
// service reads plan codes from this service). Same late-binding pattern as
// the invite AccountCreator.
func (s *Service) SetRoleChecker(roles RoleChecker) {
	s.roles = roles
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
		// Compensate: remove the organization so its slug is not burned by an
		// org with no owner.
		if deleteErr := s.repo.DeleteOrganization(ctx, org.ID); deleteErr != nil {
			slog.ErrorContext(ctx, "compensating organization delete failed",
				"organizationId", org.ID, "error", deleteErr)
		}

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

// GetBySlug loads an organization by its URL slug.
func (s *Service) GetBySlug(ctx context.Context, slug string) (Organization, error) {
	org, err := s.repo.GetBySlug(ctx, slug)
	if err != nil {
		return Organization{}, fmt.Errorf("get organization by slug: %w", err)
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

	if in.Branding != nil {
		branding, err := sanitizeBranding(*in.Branding)
		if err != nil {
			return Organization{}, err
		}

		org.Branding = branding
	}
	if in.CustomDomain != nil {
		domain := strings.ToLower(strings.TrimSpace(*in.CustomDomain))
		if domain != "" && !domainPattern().MatchString(domain) {
			return Organization{}, ErrInvalidInput
		}
		org.CustomDomain = domain
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

// SetMembershipStatus activates or suspends a user's membership in an
// organization, used by SCIM provisioning to (de)provision access. It returns
// ErrNotFound when the user has no membership in the organization.
func (s *Service) SetMembershipStatus(ctx context.Context, organizationID, userID string, active bool) error {
	if organizationID == "" || userID == "" {
		return ErrInvalidInput
	}

	status := membershipStatusSuspended
	if active {
		status = membershipStatusActive
	}

	if err := s.repo.UpdateMembershipStatus(ctx, organizationID, userID, status); err != nil {
		return fmt.Errorf("set membership status: %w", err)
	}

	return nil
}

// HasMembership reports whether a user is a member of an organization (any
// status). SCIM provisioning uses it to distinguish a same-tenant re-provision
// from an attempt to graft a foreign-tenant account.
func (s *Service) HasMembership(ctx context.Context, organizationID, userID string) (bool, error) {
	exists, err := s.repo.MembershipExists(ctx, organizationID, userID)
	if err != nil {
		return false, fmt.Errorf("check membership exists: %w", err)
	}

	return exists, nil
}

// RoleEmployee is the membership role for onboarded employees.
func RoleEmployee() string {
	return roleEmployee
}

// RoleHRAdmin is the membership role for HR administrators.
func RoleHRAdmin() string {
	return roleHRAdmin
}

// ListMembers lists the organization's active memberships with each account's
// display info. Accounts that fail to load are listed with empty display
// fields rather than failing the whole listing.
func (s *Service) ListMembers(ctx context.Context, organizationID string) ([]Member, error) {
	if organizationID == "" {
		return nil, ErrInvalidInput
	}

	memberships, err := s.repo.ListMemberships(ctx, organizationID)
	if err != nil {
		return nil, fmt.Errorf("list memberships: %w", err)
	}

	members := make([]Member, 0, len(memberships))

	for _, membership := range memberships {
		members = append(members, s.buildMember(ctx, membership))
	}

	return members, nil
}

// ChangeMemberRole sets a member's role. The actor may never change their own
// role, the role code must be a built-in role or an existing custom role
// (via the RoleChecker port), and the change must not demote the
// organization's last organization_owner.
func (s *Service) ChangeMemberRole(
	ctx context.Context,
	organizationID, actorUserID, targetUserID, roleCode string,
) (Membership, error) {
	roleCode = strings.TrimSpace(roleCode)
	if organizationID == "" || targetUserID == "" || roleCode == "" {
		return Membership{}, ErrInvalidInput
	}

	if actorUserID == targetUserID {
		return Membership{}, ErrCannotChangeOwnRole
	}

	if err := s.checkRoleAssignable(ctx, organizationID, roleCode); err != nil {
		return Membership{}, err
	}

	membership, err := s.repo.GetMembership(ctx, organizationID, targetUserID)
	if err != nil {
		return Membership{}, fmt.Errorf("get membership: %w", err)
	}

	if err := s.guardLastOwner(ctx, organizationID, membership.RoleCode, roleCode); err != nil {
		return Membership{}, err
	}

	if err := s.repo.UpdateMembershipRole(ctx, organizationID, targetUserID, roleCode); err != nil {
		return Membership{}, fmt.Errorf("update membership role: %w", err)
	}

	membership.RoleCode = roleCode

	return membership, nil
}

func isBuiltinRole(roleCode string) bool {
	switch roleCode {
	case roleOrganizationOwner, roleHRAdmin, roleManager, roleEmployee:
		return true
	default:
		return false
	}
}

// sanitizeBranding validates and normalizes an organization's brand overrides.
// Empty fields are allowed and clear the override; malformed colors or logo
// URLs are rejected so a bad value can never reach the frontend.
func sanitizeBranding(in Branding) (Branding, error) {
	primary, err := validHexColor(in.PrimaryColor)
	if err != nil {
		return Branding{}, err
	}

	hover, err := validHexColor(in.PrimaryHoverColor)
	if err != nil {
		return Branding{}, err
	}

	accent, err := validHexColor(in.AccentColor)
	if err != nil {
		return Branding{}, err
	}

	logo, err := validLogoURL(in.LogoURL)
	if err != nil {
		return Branding{}, err
	}

	return Branding{
		PrimaryColor:      primary,
		PrimaryHoverColor: hover,
		AccentColor:       accent,
		LogoURL:           logo,
	}, nil
}

func validHexColor(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}

	if !hexColorPattern().MatchString(value) {
		return "", ErrInvalidInput
	}

	return value, nil
}

func validLogoURL(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}

	if len(value) > maxLogoURLLen {
		return "", ErrInvalidInput
	}

	if !strings.HasPrefix(value, "https://") && !strings.HasPrefix(value, "http://") {
		return "", ErrInvalidInput
	}

	return value, nil
}

// List returns all organizations.
func (s *Service) List(ctx context.Context) ([]Organization, error) {
	items, err := s.repo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list organizations: %w", err)
	}

	return items, nil
}

// CountByStatus returns organization counts grouped by status.
func (s *Service) CountByStatus(ctx context.Context) (map[string]int64, error) {
	counts, err := s.repo.CountByStatus(ctx)
	if err != nil {
		return nil, fmt.Errorf("count organizations by status: %w", err)
	}

	return counts, nil
}

// SetPlanCode updates an organization billing plan.
func (s *Service) SetPlanCode(ctx context.Context, id, planCode string) (Organization, error) {
	planCode = strings.TrimSpace(planCode)
	if id == "" || planCode == "" {
		return Organization{}, ErrInvalidInput
	}

	org, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return Organization{}, fmt.Errorf("get organization: %w", err)
	}

	org.PlanCode = planCode

	org.UpdatedAt = time.Now().UTC()
	if err := s.repo.Update(ctx, org); err != nil {
		return Organization{}, fmt.Errorf("update organization plan: %w", err)
	}

	return org, nil
}

// SetStatus updates an organization status.
func (s *Service) SetStatus(ctx context.Context, id, status string) (Organization, error) {
	status = strings.TrimSpace(status)
	if id == "" ||
		(status != statusActive && status != statusTrial && status != statusSuspended && status != statusClosed) {
		return Organization{}, ErrInvalidInput
	}

	org, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return Organization{}, fmt.Errorf("get organization: %w", err)
	}

	// Closed is terminal. Reopening requires the separate GDPR/data-recovery
	// process rather than the ordinary activate control.
	if org.Status == statusClosed && status != statusClosed {
		return Organization{}, ErrInvalidInput
	}

	org.Status = status

	org.UpdatedAt = time.Now().UTC()
	if err := s.repo.Update(ctx, org); err != nil {
		return Organization{}, fmt.Errorf("update organization status: %w", err)
	}

	return org, nil
}

// UpdateSetupProgress advances (or completes) the ten-step setup wizard.
// Progress cannot move backwards, which keeps retries idempotent.
func (s *Service) UpdateSetupProgress(
	ctx context.Context,
	id string,
	input SetupProgressInput,
) (Organization, error) {
	if id == "" || input.Step < 1 || input.Step > 10 {
		return Organization{}, ErrInvalidInput
	}
	org, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return Organization{}, fmt.Errorf("get organization: %w", err)
	}
	if input.Step > org.SetupStep {
		org.SetupStep = input.Step
	}
	now := time.Now().UTC()
	if input.Completed {
		if input.Step != 10 {
			return Organization{}, ErrInvalidInput
		}
		org.SetupCompletedAt = &now
	}
	org.UpdatedAt = now
	if err := s.repo.Update(ctx, org); err != nil {
		return Organization{}, fmt.Errorf("update organization setup progress: %w", err)
	}
	return org, nil
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

// InviteMember creates a login account and an hr_admin membership for it in
// the organization. The flow is safe to retry: when a previous attempt created
// the account but failed partway, the existing account is reused instead of
// failing the retry with ErrInviteEmailTaken.
func (s *Service) InviteMember(ctx context.Context, organizationID string, in InviteMemberInput) (Membership, error) {
	email := strings.ToLower(strings.TrimSpace(in.Email))
	displayName := strings.TrimSpace(in.DisplayName)

	if organizationID == "" || email == "" || displayName == "" || s.accounts == nil {
		return Membership{}, ErrInviteInvalidInput
	}

	if in.RoleCode != roleHRAdmin {
		return Membership{}, fmt.Errorf("%w: only hr_admin members can be invited", ErrInviteInvalidInput)
	}

	userID, existing, err := s.createOrReuseInviteAccount(ctx, organizationID, email, displayName, in.Password)
	if err != nil {
		return Membership{}, err
	}

	if existing != nil {
		// The account is already a member: a previous attempt completed the
		// invite but the caller saw a failure (e.g. audit write failed).
		return *existing, nil
	}

	return s.AddMember(ctx, organizationID, userID, roleHRAdmin)
}

// createOrReuseInviteAccount creates the invite account. When the email is
// already registered it instead resolves how a retried invite may proceed:
// the existing membership when the account already belongs to the organization
// (a previous attempt failed after creating it), or the orphaned account's
// user id when it has no memberships anywhere (a previous attempt failed right
// after account creation). Accounts belonging to other organizations are
// rejected with ErrInviteEmailTaken.
func (s *Service) createOrReuseInviteAccount(
	ctx context.Context,
	organizationID, email, displayName, password string,
) (string, *Membership, error) {
	userID, err := s.accounts.CreateUserAccount(ctx, email, displayName, password)
	if err == nil {
		return userID, nil, nil
	}

	if !errors.Is(err, ErrInviteEmailTaken) {
		return "", nil, fmt.Errorf("create invite account: %w", err)
	}

	userID, err = s.accounts.FindUserByEmail(ctx, email)
	if err != nil {
		return "", nil, fmt.Errorf("load invited account: %w", err)
	}

	exists, err := s.repo.MembershipExists(ctx, organizationID, userID)
	if err != nil {
		return "", nil, fmt.Errorf("check invite membership: %w", err)
	}

	if exists {
		membership, getErr := s.repo.GetMembership(ctx, organizationID, userID)
		if getErr != nil {
			return "", nil, fmt.Errorf("get invite membership: %w", getErr)
		}

		return "", &membership, nil
	}

	memberships, err := s.repo.ListMembershipsByUser(ctx, userID)
	if err != nil {
		return "", nil, fmt.Errorf("list invite memberships: %w", err)
	}

	if len(memberships) > 0 {
		return "", nil, ErrInviteEmailTaken
	}

	return userID, nil, nil
}

func (s *Service) buildMember(ctx context.Context, membership Membership) Member {
	member := Member{Membership: membership, Email: "", DisplayName: "", UserStatus: ""}

	if s.users == nil {
		return member
	}

	user, err := s.users.GetMemberUser(ctx, membership.UserID)
	if err != nil {
		slog.WarnContext(ctx, "load member user failed", "userId", membership.UserID, "error", err)

		return member
	}

	member.Email = user.Email
	member.DisplayName = user.DisplayName
	member.UserStatus = user.Status

	return member
}

// checkRoleAssignable accepts built-in role codes and, when a RoleChecker is
// wired, names of existing custom roles.
func (s *Service) checkRoleAssignable(ctx context.Context, organizationID, roleCode string) error {
	if isBuiltinRole(roleCode) {
		return nil
	}

	if s.roles == nil {
		return ErrUnknownRole
	}

	exists, err := s.roles.RoleExists(ctx, organizationID, roleCode)
	if err != nil {
		return fmt.Errorf("check role: %w", err)
	}

	if !exists {
		return ErrUnknownRole
	}

	return nil
}

// guardLastOwner blocks demoting the organization's last active
// organization_owner so a tenant can never be left ownerless.
func (s *Service) guardLastOwner(ctx context.Context, organizationID, currentRole, newRole string) error {
	if currentRole != roleOrganizationOwner || newRole == roleOrganizationOwner {
		return nil
	}

	owners, err := s.repo.CountMembershipsByRole(ctx, organizationID, roleOrganizationOwner)
	if err != nil {
		return fmt.Errorf("count organization owners: %w", err)
	}

	if owners <= 1 {
		return ErrLastOwner
	}

	return nil
}
