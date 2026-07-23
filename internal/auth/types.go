package auth

import (
	"errors"
	"time"
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
)

const (
	userStatusActive            = "active"
	roleOrganizationOwner       = "organization_owner"
	tokenTypeBearer             = "Bearer"
	refreshTokenPartsExpected   = 2
	sessionPayloadPartsExpected = 3
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

// UserPublic is the API-safe user representation.
type UserPublic struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
	Status      string `json:"status"`
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
