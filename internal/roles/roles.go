// Package roles implements the PRD 6.3 RBAC model: the `resource.action`
// permission registry, the built-in role definitions, tenant-scoped custom
// roles, and resolution of a membership's role code to its effective
// permission set.
package roles

import (
	"errors"
	"regexp"
	"slices"
	"strings"
	"time"
)

var (
	// ErrNotFound indicates the custom role does not exist.
	ErrNotFound = errors.New("role not found")
	// ErrInvalidInput indicates validation failed.
	ErrInvalidInput = errors.New("invalid role input")
	// ErrNameTaken indicates the role name is already used, either by another
	// custom role in the organization or by a reserved built-in role code.
	ErrNameTaken = errors.New("role name already taken")
	// ErrPlanNotEligible indicates the organization's plan does not include
	// custom roles (enterprise plan only).
	ErrPlanNotEligible = errors.New("custom roles require the enterprise plan")
	// ErrBuiltinRole indicates an attempt to modify a built-in role.
	ErrBuiltinRole = errors.New("built-in roles cannot be modified")
)

// Permission codes use the PRD 6.3 `resource.action` format.
const (
	PermissionEmployeesRead   = "employees.read"
	PermissionEmployeesCreate = "employees.create"
	PermissionEmployeesUpdate = "employees.update"

	PermissionJourneysCreate  = "journeys.create"
	PermissionJourneysPublish = "journeys.publish"
	PermissionJourneysAssign  = "journeys.assign"

	PermissionAssignmentsRead   = "assignments.read"
	PermissionAssignmentsManage = "assignments.manage"

	PermissionApprovalsDecide = "approvals.decide"

	PermissionDepartmentsManage = "departments.manage"

	PermissionBillingRead   = "billing.read"
	PermissionBillingManage = "billing.manage"

	PermissionAuditRead = "audit.read"

	PermissionAnalyticsRead = "analytics.read"

	// PermissionDataExport gates the GDPR organization data export (PRD 7.4).
	// Only organization_owner and hr_admin hold it (via the full registry);
	// manager and employee do not.
	PermissionDataExport = "data.export"

	PermissionKnowledgeManage = "knowledge.manage"

	PermissionAssessmentsManage = "assessments.manage"

	PermissionIntegrationsManage = "integrations.manage"

	PermissionMembersRead   = "members.read"
	PermissionMembersInvite = "members.invite"
	PermissionMembersUpdate = "members.update"

	PermissionNotificationsRead = "notifications.read"

	PermissionStepsComplete = "steps.complete"

	// PermissionMeetingsManage gates scheduling meetings for employees and
	// recording their outcomes (PRD §5.3.7).
	PermissionMeetingsManage = "meetings.manage"
)

// Built-in membership role codes. Platform roles (platform_owner,
// platform_admin) are deliberately outside this model: platform routes keep
// their existing RequirePlatform gate untouched.
const (
	RoleOrganizationOwner = "organization_owner"
	RoleHRAdmin           = "hr_admin"
	RoleManager           = "manager"
	RoleEmployee          = "employee"
)

const (
	maxRoleNameLen     = 64
	maxRolePermissions = 64
)

// Role is a named collection of permissions. Custom roles are persisted per
// organization; built-in roles are synthesized on read (Builtin true) and
// never stored. A membership's roleCode references either a built-in role
// code or a custom role's name.
type Role struct {
	ID             string    `bson:"_id"            json:"id"`
	OrganizationID string    `bson:"organizationId" json:"organizationId"`
	Name           string    `bson:"name"           json:"name"`
	Permissions    []string  `bson:"permissions"    json:"permissions"`
	Builtin        bool      `bson:"-"              json:"builtin"`
	CreatedAt      time.Time `bson:"createdAt"      json:"createdAt"`
	UpdatedAt      time.Time `bson:"updatedAt"      json:"updatedAt"`
}

// CreateInput creates a custom role.
type CreateInput struct {
	Name        string
	Permissions []string
}

// UpdateInput replaces a custom role's permission set.
type UpdateInput struct {
	Permissions []string
}

// AllPermissions returns the full permission registry.
func AllPermissions() []string {
	return []string{
		PermissionEmployeesRead,
		PermissionEmployeesCreate,
		PermissionEmployeesUpdate,
		PermissionJourneysCreate,
		PermissionJourneysPublish,
		PermissionJourneysAssign,
		PermissionAssignmentsRead,
		PermissionAssignmentsManage,
		PermissionApprovalsDecide,
		PermissionDepartmentsManage,
		PermissionBillingRead,
		PermissionBillingManage,
		PermissionAuditRead,
		PermissionAnalyticsRead,
		PermissionDataExport,
		PermissionKnowledgeManage,
		PermissionAssessmentsManage,
		PermissionIntegrationsManage,
		PermissionMembersRead,
		PermissionMembersInvite,
		PermissionMembersUpdate,
		PermissionNotificationsRead,
		PermissionStepsComplete,
		PermissionMeetingsManage,
	}
}

// IsPermission reports whether code is a registered permission.
func IsPermission(code string) bool {
	return slices.Contains(AllPermissions(), code)
}

// BuiltinPermissions returns the permission set of a built-in role code.
//
// organization_owner holds every permission. hr_admin holds everything except
// billing.manage, which stays owner-only so HR administrators can view the
// subscription (billing.read) but never change billing. manager is scoped to
// people operations: directory reads, journey assignment, assignment
// management, approval decisions, and read-only audit/analytics. employee is
// self-service only: read own assignments, complete steps, read notifications.
func BuiltinPermissions(roleCode string) ([]string, bool) {
	switch roleCode {
	case RoleOrganizationOwner:
		return AllPermissions(), true
	case RoleHRAdmin:
		return withoutPermission(AllPermissions(), PermissionBillingManage), true
	case RoleManager:
		return []string{
			PermissionEmployeesRead,
			PermissionJourneysAssign,
			PermissionAssignmentsRead,
			PermissionAssignmentsManage,
			PermissionApprovalsDecide,
			PermissionAssessmentsManage,
			PermissionAnalyticsRead,
			PermissionAuditRead,
			PermissionNotificationsRead,
			PermissionMeetingsManage,
		}, true
	case RoleEmployee:
		return []string{
			PermissionAssignmentsRead,
			PermissionStepsComplete,
			PermissionNotificationsRead,
		}, true
	default:
		return nil, false
	}
}

func withoutPermission(permissions []string, drop string) []string {
	out := make([]string, 0, len(permissions))

	for _, permission := range permissions {
		if permission != drop {
			out = append(out, permission)
		}
	}

	return out
}

func permissionSet(permissions []string) map[string]struct{} {
	set := make(map[string]struct{}, len(permissions))
	for _, permission := range permissions {
		set[permission] = struct{}{}
	}

	return set
}

// validRoleName reports whether name may identify a custom role. Role names
// double as membership role codes, so they use the same code-like shape.
func validRoleName(name string) bool {
	return len(name) <= maxRoleNameLen && roleNamePattern().MatchString(name)
}

func roleNamePattern() *regexp.Regexp {
	return regexp.MustCompile(`^[a-z0-9]+(?:[-_][a-z0-9]+)*$`)
}

// normalizePermissions validates and dedupes a requested permission set,
// preserving the input order.
func normalizePermissions(permissions []string) ([]string, error) {
	if len(permissions) == 0 || len(permissions) > maxRolePermissions {
		return nil, ErrInvalidInput
	}

	seen := make(map[string]struct{}, len(permissions))
	out := make([]string, 0, len(permissions))

	for _, raw := range permissions {
		permission := strings.TrimSpace(raw)
		if !IsPermission(permission) {
			return nil, ErrInvalidInput
		}

		if _, dup := seen[permission]; dup {
			continue
		}

		seen[permission] = struct{}{}

		out = append(out, permission)
	}

	return out, nil
}
