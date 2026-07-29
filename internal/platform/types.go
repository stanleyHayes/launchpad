package platform

import (
	"errors"
	"slices"
	"strings"
	"time"

	"launchpad/internal/organizations"
)

// OrganizationListInput filters the platform tenant directory. Search is a
// case-insensitive substring match across name and slug.
type OrganizationListInput struct {
	Search   string
	Status   string
	PlanCode string
	Offset   int
	Limit    int
}

func (in OrganizationListInput) normalized() OrganizationListInput {
	return OrganizationListInput{
		Search:   strings.ToLower(strings.TrimSpace(in.Search)),
		Status:   strings.ToLower(strings.TrimSpace(in.Status)),
		PlanCode: strings.ToLower(strings.TrimSpace(in.PlanCode)),
		Offset:   max(in.Offset, 0),
		Limit:    in.Limit,
	}
}

type OrganizationPage struct {
	Items  []organizations.Organization
	Total  int
	Offset int
	Limit  int
}

var (
	// ErrNotFound indicates the platform staff record does not exist.
	ErrNotFound = errors.New("platform staff not found")
	// ErrInvalidInput indicates validation failed.
	ErrInvalidInput = errors.New("invalid platform input")
	// ErrProvisioningUnavailable indicates staff account provisioning (user
	// creation) is not wired into the service.
	ErrProvisioningUnavailable = errors.New("staff account provisioning is not configured")
)

const (
	rolePlatformOwner   = "platform_owner"
	rolePlatformAdmin   = "platform_admin"
	roleSupportAgent    = "support_agent"
	roleBillingAdmin    = "billing_admin"
	roleContentEditor   = "content_editor"
	roleSecurityAdmin   = "security_admin"
	roleAnalyst         = "analyst"
	roleReadOnlyAuditor = "read_only_auditor"

	staffStatusActive      = "active"
	staffStatusDeactivated = "deactivated"
)

// RoleCodes returns the recognized platform staff role codes (PRD §5.2.6).
func RoleCodes() []string {
	return []string{
		rolePlatformOwner,
		rolePlatformAdmin,
		roleSupportAgent,
		roleBillingAdmin,
		roleContentEditor,
		roleSecurityAdmin,
		roleAnalyst,
		roleReadOnlyAuditor,
	}
}

// IsValidRole reports whether roleCode is a recognized platform staff role.
func IsValidRole(roleCode string) bool {
	return slices.Contains(RoleCodes(), roleCode)
}

// Staff is a platform operator account.
type Staff struct {
	ID               string           `bson:"_id"         json:"id"`
	UserID           string           `bson:"userId"      json:"userId"`
	Email            string           `bson:"email"       json:"email"`
	DisplayName      string           `bson:"displayName" json:"displayName"`
	RoleCode         string           `bson:"roleCode"    json:"roleCode"`
	Status           string           `bson:"status"      json:"status"`
	CreatedAt        time.Time        `bson:"createdAt"   json:"createdAt"`
	BreakGlass       *BreakGlassGrant `bson:"breakGlass,omitempty" json:"breakGlass,omitempty"`
	AccessReviewedAt *time.Time       `bson:"accessReviewedAt,omitempty" json:"accessReviewedAt,omitempty"`
	AccessReviewedBy string           `bson:"accessReviewedBy,omitempty" json:"accessReviewedBy,omitempty"`
}

// BreakGlassGrant is a short-lived emergency privilege elevation approved by
// a platform owner. Expired/revoked grants never affect authorization.
type BreakGlassGrant struct {
	RoleCode   string     `bson:"roleCode" json:"roleCode"`
	Reason     string     `bson:"reason" json:"reason"`
	ApprovedBy string     `bson:"approvedBy" json:"approvedBy"`
	GrantedAt  time.Time  `bson:"grantedAt" json:"grantedAt"`
	ExpiresAt  time.Time  `bson:"expiresAt" json:"expiresAt"`
	RevokedAt  *time.Time `bson:"revokedAt,omitempty" json:"revokedAt,omitempty"`
	RevokedBy  string     `bson:"revokedBy,omitempty" json:"revokedBy,omitempty"`
}

type AccessReviewItem struct {
	Staff             Staff  `json:"staff"`
	ReviewDue         bool   `json:"reviewDue"`
	EffectiveRoleCode string `json:"effectiveRoleCode"`
}

// CreateStaffInput carries the details for a new platform staff account.
type CreateStaffInput struct {
	Email       string
	DisplayName string
	RoleCode    string
}

// CreateStaffResult reports a provisioned staff account. TempPassword is only
// populated when no mail sender is configured (it is shown once to the
// operator); Invited is true when the credentials were emailed instead.
type CreateStaffResult struct {
	Staff        Staff
	TempPassword string
	Invited      bool
}

// Overview summarizes platform-wide metrics.
type Overview struct {
	TotalOrgs           int64 `json:"totalOrgs"`
	TrialOrgs           int64 `json:"trialOrgs"`
	ActiveOrgs          int64 `json:"activeOrgs"`
	SuspendedOrgs       int64 `json:"suspendedOrgs"`
	TotalLeads          int64 `json:"totalLeads"`
	OpenTicketCount     int64 `json:"openTicketCount"`
	OverdueTicketCount  int   `json:"overdueTicketCount"`
	UrgentTicketCount   int   `json:"urgentTicketCount"`
	MRRTotalCents       int64 `json:"mrrTotalCents"`
	ARRTotalCents       int64 `json:"arrTotalCents"`
	ActiveSubscriptions int   `json:"activeSubscriptions"`
}

type RevenueMetrics struct {
	MRRTotalCents       int64
	ARRTotalCents       int64
	ActiveSubscriptions int
}

type SupportMetrics struct {
	Overdue int
	Urgent  int
}

type StorageOverview struct {
	Collections      int64 `json:"collections"`
	Objects          int64 `json:"objects"`
	DataSizeBytes    int64 `json:"dataSizeBytes"`
	StorageSizeBytes int64 `json:"storageSizeBytes"`
	IndexSizeBytes   int64 `json:"indexSizeBytes"`
}

// RoleOwner returns the platform owner role code.
func RoleOwner() string {
	return rolePlatformOwner
}

// RoleAdmin returns the platform admin role code.
func RoleAdmin() string {
	return rolePlatformAdmin
}
