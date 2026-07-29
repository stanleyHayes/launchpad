package privacy

import (
	"context"
	"fmt"
	"time"

	"launchpad/internal/organizations"
)

// OrganizationPurger deletes every document of one tenant collection.
// Every tenant-scoped Mongo store satisfies it.
type OrganizationPurger interface {
	DeleteForOrganization(ctx context.Context, organizationID string) (int64, error)
}

// PasswordResetPurger deletes password-reset tokens by user id. Reset tokens
// carry no organization scope, so the purge deletes them by the user ids
// collected from the organization's memberships and employee records.
// auth.PasswordResetStore satisfies it.
type PasswordResetPurger interface {
	DeleteForUsers(ctx context.Context, userIDs []string) (int64, error)
}

// TombstoneRecorder writes the platform-level audit event that remains after
// a purge wipes the tenant's own audit history. *audit.Service satisfies it.
type TombstoneRecorder interface {
	Record(
		ctx context.Context,
		organizationID *string,
		actorUserID, action, resourceType, resourceID string,
		metadata map[string]any,
	) error
}

// PurgeStores groups the per-domain purgers the tenant purge calls. Every
// field is required; the app wiring supplies the concrete Mongo stores.
type PurgeStores struct {
	Organizations          OrganizationPurger // organizations + organization_memberships
	Employees              OrganizationPurger
	Departments            OrganizationPurger // departments + job_roles
	Journeys               OrganizationPurger // journey_templates + journey_steps
	Assignments            OrganizationPurger // assignments, step assignments, approvals, rules
	Notifications          OrganizationPurger
	NotificationChannels   OrganizationPurger
	NotificationDeliveries OrganizationPurger
	Knowledge              OrganizationPurger
	AssistantChunks        OrganizationPurger
	AssistantInteractions  OrganizationPurger
	Audit                  OrganizationPurger
	Roles                  OrganizationPurger
	Integrations           OrganizationPurger
	HRISConfigs            OrganizationPurger
	HRISState              OrganizationPurger
	SSOConfigs             OrganizationPurger
	SAMLConfigs            OrganizationPurger
	SCIMUsers              OrganizationPurger
	SCIMTokens             OrganizationPurger
	SCIMGroups             OrganizationPurger
	BillingSubscriptions   OrganizationPurger
	Support                OrganizationPurger // support_tickets + support_blockers
	FeatureFlagOverrides   OrganizationPurger
	Invitations            OrganizationPurger
	PasswordResets         PasswordResetPurger
	MFAEnrollments         OrganizationPurger
	Requests               OrganizationPurger
}

// PurgeResult reports what the tenant purge deleted.
type PurgeResult struct {
	OrganizationID string           `json:"organizationId"`
	Slug           string           `json:"slug"`
	Deleted        map[string]int64 `json:"deleted"`
	PurgedAt       time.Time        `json:"purgedAt"`
}

// PurgeService irreversibly deletes all tenant data (PRD 7.4 right to
// erasure). It runs only from the platform staff route group and requires the
// caller to confirm the organization slug.
type PurgeService struct {
	orgs      OrganizationReader
	employees EmployeeLister
	stores    PurgeStores
	audit     TombstoneRecorder
}

// NewPurgeService constructs a PurgeService. The employee lister supplies the
// user ids whose password resets are deleted; memberships alone would miss
// suspended members.
func NewPurgeService(
	orgs OrganizationReader,
	employeeLister EmployeeLister,
	stores PurgeStores,
	audit TombstoneRecorder,
) *PurgeService {
	return &PurgeService{orgs: orgs, employees: employeeLister, stores: stores, audit: audit}
}

// Purge deletes every tenant document of the organization after verifying
// that confirm equals the organization slug. actorUserID is recorded on the
// platform-level tombstone audit event.
func (s *PurgeService) Purge(
	ctx context.Context,
	organizationID, confirm, actorUserID string,
) (PurgeResult, error) {
	org, err := s.orgs.GetByID(ctx, organizationID)
	if err != nil {
		return PurgeResult{}, fmt.Errorf("get organization: %w", err)
	}

	if confirm != org.Slug {
		return PurgeResult{}, ErrConfirmationMismatch
	}

	userIDs, err := s.memberUserIDs(ctx, organizationID)
	if err != nil {
		return PurgeResult{}, err
	}

	deleted, err := s.deleteTenantData(ctx, organizationID, userIDs)
	if err != nil {
		return PurgeResult{}, err
	}

	s.recordTombstone(ctx, org, actorUserID, deleted)

	return PurgeResult{
		OrganizationID: org.ID,
		Slug:           org.Slug,
		Deleted:        deleted,
		PurgedAt:       time.Now().UTC(),
	}, nil
}

// memberUserIDs collects the distinct user ids linked to the organization via
// memberships or employee records.
func (s *PurgeService) memberUserIDs(ctx context.Context, organizationID string) ([]string, error) {
	memberships, err := s.orgs.ListMemberships(ctx, organizationID)
	if err != nil {
		return nil, fmt.Errorf("list memberships: %w", err)
	}

	seen := make(map[string]struct{}, len(memberships))
	userIDs := make([]string, 0, len(memberships))

	for _, membership := range memberships {
		if _, dup := seen[membership.UserID]; !dup {
			seen[membership.UserID] = struct{}{}
			userIDs = append(userIDs, membership.UserID)
		}
	}

	for offset := int64(0); ; offset += employeePageSize {
		page, err := s.employees.List(ctx, organizationID, offset, employeePageSize)
		if err != nil {
			return nil, fmt.Errorf("list employees: %w", err)
		}

		for _, employee := range page {
			if employee.UserID == "" {
				continue
			}

			if _, dup := seen[employee.UserID]; !dup {
				seen[employee.UserID] = struct{}{}
				userIDs = append(userIDs, employee.UserID)
			}
		}

		if int64(len(page)) < employeePageSize {
			return userIDs, nil
		}
	}
}

// deleteTenantData deletes every tenant collection in dependency-safe order:
// domain data first, the tenant's audit history second to last, and the
// organization document with its memberships last.
func (s *PurgeService) deleteTenantData(
	ctx context.Context,
	organizationID string,
	userIDs []string,
) (map[string]int64, error) {
	purgers := []struct {
		label  string
		purger OrganizationPurger
	}{
		{"employees", s.stores.Employees},
		{"departments", s.stores.Departments},
		{"journeys", s.stores.Journeys},
		{"assignments", s.stores.Assignments},
		{"notifications", s.stores.Notifications},
		{"notificationChannels", s.stores.NotificationChannels},
		{"notificationDeliveries", s.stores.NotificationDeliveries},
		{"knowledge", s.stores.Knowledge},
		{"assistantChunks", s.stores.AssistantChunks},
		{"assistantInteractions", s.stores.AssistantInteractions},
		{"roles", s.stores.Roles},
		{"integrations", s.stores.Integrations},
		{"hrisConfigs", s.stores.HRISConfigs},
		{"hrisState", s.stores.HRISState},
		{"ssoConfigs", s.stores.SSOConfigs},
		{"samlConfigs", s.stores.SAMLConfigs},
		{"scimUsers", s.stores.SCIMUsers},
		{"scimTokens", s.stores.SCIMTokens},
		{"scimGroups", s.stores.SCIMGroups},
		{"billingSubscriptions", s.stores.BillingSubscriptions},
		{"support", s.stores.Support},
		{"featureFlagOverrides", s.stores.FeatureFlagOverrides},
		{"invitations", s.stores.Invitations},
		{"mfaEnrollments", s.stores.MFAEnrollments},
		{"requests", s.stores.Requests},
		{"auditEvents", s.stores.Audit},
		{"organizations", s.stores.Organizations},
	}

	// +1 for password resets, which are deleted by user id below.
	deleted := make(map[string]int64, len(purgers)+1)

	for _, entry := range purgers {
		if entry.purger == nil {
			continue
		}

		count, err := entry.purger.DeleteForOrganization(ctx, organizationID)
		if err != nil {
			return nil, fmt.Errorf("purge %s: %w", entry.label, err)
		}

		deleted[entry.label] = count
	}

	count, err := s.stores.PasswordResets.DeleteForUsers(ctx, userIDs)
	if err != nil {
		return nil, fmt.Errorf("purge passwordResets: %w", err)
	}

	deleted["passwordResets"] = count

	return deleted, nil
}

// recordTombstone writes the platform-level audit event that outlives the
// purge, best-effort: the data is already gone, so a broken audit store must
// not change the outcome.
func (s *PurgeService) recordTombstone(
	ctx context.Context,
	org organizations.Organization,
	actorUserID string,
	deleted map[string]int64,
) {
	_ = s.audit.Record(
		ctx,
		nil,
		actorUserID,
		"organization.purged",
		"organization",
		org.ID,
		map[string]any{"slug": org.Slug, "deleted": deleted},
	)
}
