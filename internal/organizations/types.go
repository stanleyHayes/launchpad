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
	// ErrCannotChangeOwnRole indicates a member tried to change their own role.
	ErrCannotChangeOwnRole = errors.New("members cannot change their own role")
	// ErrLastOwner indicates an attempt to demote the organization's last
	// organization_owner, which would leave the tenant without an owner.
	ErrLastOwner = errors.New("cannot demote the last organization owner")
	// ErrUnknownRole indicates the role code is neither a built-in role nor an
	// existing custom role in the organization.
	ErrUnknownRole = errors.New("unknown role code")
)

const (
	statusTrial               = "trial"
	statusActive              = "active"
	statusSuspended           = "suspended"
	statusClosed              = "closed"
	planStarter               = "starter"
	defaultTimezone           = "UTC"
	roleOrganizationOwner     = "organization_owner"
	roleHRAdmin               = "hr_admin"
	roleManager               = "manager"
	roleEmployee              = "employee"
	membershipStatusActive    = "active"
	membershipStatusSuspended = "suspended"
)

// Organization is a tenant.
type Organization struct {
	ID               string     `bson:"_id"                json:"id"`
	Name             string     `bson:"name"               json:"name"`
	Slug             string     `bson:"slug"               json:"slug"`
	Status           string     `bson:"status"             json:"status"`
	PlanCode         string     `bson:"planCode"           json:"planCode"`
	Timezone         string     `bson:"timezone"           json:"timezone"`
	CustomDomain     string     `bson:"customDomain,omitempty" json:"customDomain,omitempty"`
	Branding         Branding   `bson:"branding,omitempty" json:"branding"`
	SetupStep        int        `bson:"setupStep,omitempty" json:"setupStep"`
	SetupCompletedAt *time.Time `bson:"setupCompletedAt,omitempty" json:"setupCompletedAt,omitempty"`
	CreatedAt        time.Time  `bson:"createdAt"          json:"createdAt"`
	UpdatedAt        time.Time  `bson:"updatedAt"          json:"updatedAt"`
}

// Branding holds an organization's customizable brand colors and logo. Empty
// fields fall back to the LaunchPad default palette. Only the brand accent
// colors are tenant-overridable; neutral and semantic colors stay fixed.
type Branding struct {
	PrimaryColor      string `bson:"primaryColor,omitempty"      json:"primaryColor,omitempty"`
	PrimaryHoverColor string `bson:"primaryHoverColor,omitempty" json:"primaryHoverColor,omitempty"`
	AccentColor       string `bson:"accentColor,omitempty"       json:"accentColor,omitempty"`
	LogoURL           string `bson:"logoUrl,omitempty"           json:"logoUrl,omitempty"`
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

// MemberUser is the API-safe display info of a member's user account. It is
// loaded through the MemberUserReader port so this package never imports auth.
type MemberUser struct {
	ID          string
	Email       string
	DisplayName string
	Status      string
}

// Member pairs an active membership with the account's display info. Display
// fields are empty when the account could not be loaded (for example a
// deleted account); the membership is still listed.
type Member struct {
	Membership  Membership
	Email       string
	DisplayName string
	UserStatus  string
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
	Name         *string
	Timezone     *string
	Branding     *Branding
	CustomDomain *string
}

// SetupProgressInput advances the durable organization setup wizard.
type SetupProgressInput struct {
	Step      int
	Completed bool
}

// InviteMemberInput invites a new member to an organization.
type InviteMemberInput struct {
	Email       string
	DisplayName string
	Password    string
	RoleCode    string
}

const maxLogoURLLen = 2048

func slugPattern() *regexp.Regexp {
	return regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
}

func domainPattern() *regexp.Regexp {
	return regexp.MustCompile(`^(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,63}$`)
}

func hexColorPattern() *regexp.Regexp {
	return regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
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

// StatusClosed returns the terminal organization status. Closing preserves
// tenant data for retention/export purposes while blocking reactivation
// through ordinary lifecycle controls.
func StatusClosed() string {
	return statusClosed
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
