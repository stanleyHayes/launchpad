package roles

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// planEnterprise is the only plan whose organizations may create custom roles.
const planEnterprise = "enterprise"

// Service implements role use cases.
type Service struct {
	repo  Repository
	plans PlanReader
}

// PlanReader reads an organization's billing plan code so the service can
// gate custom-role creation on the enterprise plan. It is the service-side
// port; the app layer wires the organizations service.
type PlanReader interface {
	PlanCode(ctx context.Context, organizationID string) (string, error)
}

// NewService constructs a Service. plans may be nil, in which case custom-role
// creation always fails with ErrPlanNotEligible (fail closed).
func NewService(repo Repository, plans PlanReader) *Service {
	return &Service{repo: repo, plans: plans}
}

// Create stores a custom role for an organization on the enterprise plan.
func (s *Service) Create(ctx context.Context, organizationID string, in CreateInput) (Role, error) {
	name := strings.TrimSpace(in.Name)
	if organizationID == "" || !validRoleName(name) {
		return Role{}, ErrInvalidInput
	}

	if _, builtin := BuiltinPermissions(name); builtin {
		return Role{}, ErrNameTaken
	}

	permissions, err := normalizePermissions(in.Permissions)
	if err != nil {
		return Role{}, err
	}

	if err := s.requireEnterprise(ctx, organizationID); err != nil {
		return Role{}, err
	}

	now := time.Now().UTC()

	role := Role{
		ID:             uuid.NewString(),
		OrganizationID: organizationID,
		Name:           name,
		Permissions:    permissions,
		Builtin:        false,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := s.repo.Create(ctx, role); err != nil {
		return Role{}, fmt.Errorf("create role: %w", err)
	}

	return role, nil
}

// List returns the organization's built-in roles followed by its custom roles.
func (s *Service) List(ctx context.Context, organizationID string) ([]Role, error) {
	if organizationID == "" {
		return nil, ErrInvalidInput
	}

	custom, err := s.repo.List(ctx, organizationID)
	if err != nil {
		return nil, fmt.Errorf("list custom roles: %w", err)
	}

	items := builtinRoles(organizationID)

	return append(items, custom...), nil
}

// Update replaces a custom role's permission set. Built-in roles are
// immutable and rejected with ErrBuiltinRole.
func (s *Service) Update(ctx context.Context, organizationID, roleID string, in UpdateInput) (Role, error) {
	if organizationID == "" || roleID == "" {
		return Role{}, ErrInvalidInput
	}

	if _, builtin := BuiltinPermissions(roleID); builtin {
		return Role{}, ErrBuiltinRole
	}

	permissions, err := normalizePermissions(in.Permissions)
	if err != nil {
		return Role{}, err
	}

	role, err := s.repo.GetByID(ctx, organizationID, roleID)
	if err != nil {
		return Role{}, fmt.Errorf("get role: %w", err)
	}

	role.Permissions = permissions
	role.UpdatedAt = time.Now().UTC()

	if err := s.repo.Update(ctx, role); err != nil {
		return Role{}, fmt.Errorf("update role: %w", err)
	}

	return role, nil
}

// Delete removes a custom role. Deletion is deliberately allowed while
// memberships still reference the role: those memberships resolve to zero
// permissions beyond the built-ins (see ResolvePermissions), which fails
// closed until an administrator reassigns them.
func (s *Service) Delete(ctx context.Context, organizationID, roleID string) error {
	if organizationID == "" || roleID == "" {
		return ErrInvalidInput
	}

	if _, builtin := BuiltinPermissions(roleID); builtin {
		return ErrBuiltinRole
	}

	if err := s.repo.Delete(ctx, organizationID, roleID); err != nil {
		return fmt.Errorf("delete role: %w", err)
	}

	return nil
}

// ResolvePermissions maps a membership's role code to its effective
// permission set: the built-in registry first, then the organization's custom
// roles by name. A role code that matches neither resolves to an empty set
// (fail closed). Resolution happens once per request in the authorization
// middleware; there is intentionally no cross-request cache.
func (s *Service) ResolvePermissions(
	ctx context.Context,
	organizationID, roleCode string,
) (map[string]struct{}, error) {
	if permissions, builtin := BuiltinPermissions(roleCode); builtin {
		return permissionSet(permissions), nil
	}

	role, err := s.repo.GetByName(ctx, organizationID, roleCode)
	if errors.Is(err, ErrNotFound) {
		return map[string]struct{}{}, nil
	}

	if err != nil {
		return nil, fmt.Errorf("resolve custom role: %w", err)
	}

	return permissionSet(role.Permissions), nil
}

// RoleExists reports whether a role code is assignable to members of the
// organization: a built-in code or the name of an existing custom role. It
// satisfies organizations.RoleChecker without coupling the two packages.
func (s *Service) RoleExists(ctx context.Context, organizationID, roleCode string) (bool, error) {
	if _, builtin := BuiltinPermissions(roleCode); builtin {
		return true, nil
	}

	_, err := s.repo.GetByName(ctx, organizationID, roleCode)
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("check custom role: %w", err)
	}

	return true, nil
}

// requireEnterprise gates custom-role creation on the organization's plan.
func (s *Service) requireEnterprise(ctx context.Context, organizationID string) error {
	if s.plans == nil {
		return ErrPlanNotEligible
	}

	planCode, err := s.plans.PlanCode(ctx, organizationID)
	if err != nil {
		return fmt.Errorf("read organization plan: %w", err)
	}

	if planCode != planEnterprise {
		return ErrPlanNotEligible
	}

	return nil
}

// builtinRoles synthesizes the built-in roles for list responses. Their IDs
// equal their codes so update/delete attempts on them hit the ErrBuiltinRole
// guard.
func builtinRoles(organizationID string) []Role {
	codes := []string{RoleOrganizationOwner, RoleHRAdmin, RoleManager, RoleEmployee}
	items := make([]Role, 0, len(codes))

	for _, code := range codes {
		permissions, _ := BuiltinPermissions(code)
		items = append(items, Role{
			ID:             code,
			OrganizationID: organizationID,
			Name:           code,
			Permissions:    permissions,
			Builtin:        true,
			CreatedAt:      time.Time{},
			UpdatedAt:      time.Time{},
		})
	}

	return items
}
