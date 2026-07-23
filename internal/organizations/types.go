package organizations

import (
	"errors"
	"regexp"
	"strings"
	"time"
)

var (
	// ErrNotFound indicates the organization does not exist.
	ErrNotFound = errors.New("organization not found")
	// ErrSlugTaken indicates the slug is already used.
	ErrSlugTaken = errors.New("organization slug already taken")
	// ErrInvalidInput indicates validation failed.
	ErrInvalidInput = errors.New("invalid organization input")
	// ErrInviteEmailTaken indicates the invited email is already registered.
	ErrInviteEmailTaken = errors.New("email already registered")
	// ErrInviteWeakPassword indicates the invited password does not meet policy.
	ErrInviteWeakPassword = errors.New("password does not meet requirements")
	// ErrInviteInvalidInput indicates invite validation failed.
	ErrInviteInvalidInput = errors.New("invalid invite input")
)

const (
	statusTrial            = "trial"
	statusActive           = "active"
	statusSuspended        = "suspended"
	planStarter            = "starter"
	defaultTimezone        = "UTC"
	roleOrganizationOwner  = "organization_owner"
	roleHRAdmin            = "hr_admin"
	roleEmployee           = "employee"
	membershipStatusActive = "active"
)

// Organization is a tenant.
type Organization struct {
	ID        string    `bson:"_id"       json:"id"`
	Name      string    `bson:"name"      json:"name"`
	Slug      string    `bson:"slug"      json:"slug"`
	Status    string    `bson:"status"    json:"status"`
	PlanCode  string    `bson:"planCode"  json:"planCode"`
	Timezone  string    `bson:"timezone"  json:"timezone"`
	CreatedAt time.Time `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time `bson:"updatedAt" json:"updatedAt"`
}

// Membership links a user to an organization.
type Membership struct {
	ID             string    `bson:"_id"            json:"id"`
	OrganizationID string    `bson:"organizationId" json:"organizationId"`
	UserID         string    `bson:"userId"         json:"userId"`
	RoleCode       string    `bson:"roleCode"       json:"roleCode"`
	Status         string    `bson:"status"         json:"status"`
	CreatedAt      time.Time `bson:"createdAt"      json:"createdAt"`
}

// CreateInput creates an organization with an owner.
type CreateInput struct {
	Name     string
	Slug     string
	Timezone string
	OwnerID  string
}

// UpdateInput updates mutable organization fields.
type UpdateInput struct {
	Name     *string
	Timezone *string
}

func slugPattern() *regexp.Regexp {
	return regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
}

func nonAlphanumericPattern() *regexp.Regexp {
	return regexp.MustCompile(`[^a-z0-9]+`)
}

// StatusTrial returns the trial organization status.
func StatusTrial() string {
	return statusTrial
}

// StatusActive returns the active organization status.
func StatusActive() string {
	return statusActive
}

// StatusSuspended returns the suspended organization status.
func StatusSuspended() string {
	return statusSuspended
}

// Slugify converts a name into a URL-safe slug.
func Slugify(name string) string {
	normalized := strings.ToLower(strings.TrimSpace(name))
	normalized = nonAlphanumericPattern().ReplaceAllString(normalized, "-")

	normalized = strings.Trim(normalized, "-")
	if normalized == "" {
		return "org"
	}

	return normalized
}
