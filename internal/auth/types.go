package auth

import (
	"errors"
	"time"

	"launchpad/internal/organizations"
)

var (
	// ErrInvalidCredentials indicates authentication failed.
	ErrInvalidCredentials = errors.New("invalid credentials")
	// ErrEmailTaken indicates the email is already registered.
	ErrEmailTaken = errors.New("email already registered")
	// ErrWeakPassword indicates the password does not meet policy.
	ErrWeakPassword = errors.New("password does not meet requirements")
	// ErrInvalidInput indicates request validation failed.
	ErrInvalidInput = errors.New("invalid input")
	// ErrSessionInvalid indicates the session or refresh token is invalid.
	ErrSessionInvalid = errors.New("session invalid")
	// ErrAuditFailed indicates an audit write failed after a successful mutation.
	ErrAuditFailed = errors.New("audit write failed")
	// ErrPlatformStaffNotFound indicates the user is not platform staff.
	ErrPlatformStaffNotFound = errors.New("platform staff not found")
	// ErrNotProvisioned indicates a federated (SSO) user has no account or
	// membership in the target organization.
	ErrNotProvisioned = errors.New("user is not provisioned for this organization")
	// ErrInvitationInvalid indicates an invitation token is unknown, expired, or
	// already used.
	ErrInvitationInvalid = errors.New("invitation is invalid or expired")
	// ErrPasswordResetInvalid indicates a password-reset token is unknown,
	// expired, or already used.
	ErrPasswordResetInvalid = errors.New("password reset token is invalid or expired")
	// ErrMFACodeInvalid indicates the TOTP or backup code did not match.
	ErrMFACodeInvalid = errors.New("mfa code is invalid")
	// ErrMFANotEnrolled indicates the user has no MFA enrollment in this scope.
	ErrMFANotEnrolled = errors.New("mfa is not enrolled")
	// ErrMFAAlreadyEnabled indicates MFA is already enabled for the user.
	ErrMFAAlreadyEnabled = errors.New("mfa is already enabled")
	// ErrMFATicketInvalid indicates an MFA login ticket is unknown, expired, or
	// already used.
	ErrMFATicketInvalid = errors.New("mfa ticket is invalid or expired")
)

const (
	userStatusActive          = "active"
	userStatusInvited         = "invited"
	roleOrganizationOwner     = "organization_owner"
	roleHRAdmin               = "hr_admin"
	roleEmployee              = "employee"
	tokenTypeBearer           = "Bearer"
	invitationTokenPrefix     = "invite_"
	passwordResetTokenPrefix  = "reset_"
	refreshTokenPartsExpected = 2
)

// User is an authenticated platform user.
type User struct {
	ID           string    `bson:"_id"          json:"id"`
	Email        string    `bson:"email"        json:"email"`
	DisplayName  string    `bson:"displayName"  json:"displayName"`
	PasswordHash string    `bson:"passwordHash" json:"-"`
	Status       string    `bson:"status"       json:"status"`
	MFAEnabled   bool      `bson:"mfaEnabled"   json:"mfaEnabled"`
	CreatedAt    time.Time `bson:"createdAt"    json:"createdAt"`
	UpdatedAt    time.Time `bson:"updatedAt"    json:"updatedAt"`
}

// Invitation is a pending account-activation grant, stored with its token
// hashed. Consuming it (single use) lets the invitee set a password and log in.
type Invitation struct {
	ID             string    `bson:"_id" json:"id"`
	TokenHash      string    `bson:"tokenHash" json:"-"`
	UserID         string    `bson:"userId" json:"userId"`
	OrganizationID string    `bson:"organizationId" json:"organizationId"`
	RoleCode       string    `bson:"roleCode" json:"roleCode"`
	Email          string    `bson:"email" json:"email"`
	ExpiresAt      time.Time `bson:"expiresAt" json:"expiresAt"`
	CreatedAt      time.Time `bson:"createdAt" json:"createdAt"`
}

// PasswordReset is a single-use, expiring grant to set a new password without
// the old one, stored with its token hashed. Consuming it resets the password
// and revokes all of the user's sessions.
type PasswordReset struct {
	ID        string    `bson:"_id"`
	TokenHash string    `bson:"tokenHash"`
	UserID    string    `bson:"userId"`
	Email     string    `bson:"email"`
	ExpiresAt time.Time `bson:"expiresAt"`
	CreatedAt time.Time `bson:"createdAt"`
}

// UserPublic is the API-safe user representation.
type UserPublic struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
	Status      string `json:"status"`
}

// OrganizationPublic is the API-safe organization representation returned by
// /auth/me; it keeps the Mongo document (bson tags, internal fields) out of
// the API response while preserving the JSON field names.
type OrganizationPublic struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Slug      string         `json:"slug"`
	Status    string         `json:"status"`
	PlanCode  string         `json:"planCode"`
	Timezone  string         `json:"timezone"`
	Branding  BrandingPublic `json:"branding"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
}

// BrandingPublic is the API-safe branding representation.
type BrandingPublic struct {
	PrimaryColor      string `json:"primaryColor,omitempty"`
	PrimaryHoverColor string `json:"primaryHoverColor,omitempty"`
	AccentColor       string `json:"accentColor,omitempty"`
	LogoURL           string `json:"logoUrl,omitempty"`
}

// RegisterInput is the signup payload.
type RegisterInput struct {
	Email            string `json:"email"`
	Password         string `json:"password"`
	DisplayName      string `json:"displayName"`
	OrganizationName string `json:"organizationName"`
	OrganizationSlug string `json:"organizationSlug"`
	Timezone         string `json:"timezone"`
}

// LoginInput is the login payload.
type LoginInput struct {
	Email          string `json:"email"`
	Password       string `json:"password"`
	OrganizationID string `json:"organizationId"`
}

// OrganizationChoice is one active membership available for in-session
// switching.
type OrganizationChoice struct {
	Organization OrganizationPublic `json:"organization"`
	RoleCode     string             `json:"roleCode"`
}

// TokenPair contains issued credentials.
type TokenPair struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	TokenType    string `json:"tokenType"`
	ExpiresIn    int64  `json:"expiresIn"`
}

func toPublic(user User) UserPublic {
	return UserPublic{
		ID:          user.ID,
		Email:       user.Email,
		DisplayName: user.DisplayName,
		Status:      user.Status,
	}
}

func toOrganizationPublic(org organizations.Organization) OrganizationPublic {
	return OrganizationPublic{
		ID:       org.ID,
		Name:     org.Name,
		Slug:     org.Slug,
		Status:   org.Status,
		PlanCode: org.PlanCode,
		Timezone: org.Timezone,
		Branding: BrandingPublic{
			PrimaryColor:      org.Branding.PrimaryColor,
			PrimaryHoverColor: org.Branding.PrimaryHoverColor,
			AccentColor:       org.Branding.AccentColor,
			LogoURL:           org.Branding.LogoURL,
		},
		CreatedAt: org.CreatedAt,
		UpdatedAt: org.UpdatedAt,
	}
}

func toOrganizationPublicPtr(org organizations.Organization) *OrganizationPublic {
	public := toOrganizationPublic(org)

	return &public
}
