// Package sso implements per-organization OIDC single sign-on. An employee
// authenticates with their company's identity provider (Okta, Entra ID, …) via
// the OIDC Authorization Code flow; on the callback the verified email is mapped
// to an already-provisioned LaunchPad user and a normal session is issued. The
// user must already exist in the tenant (e.g. provisioned via SCIM) — SSO
// authenticates, it does not create accounts.
package sso

import (
	"errors"
	"time"
)

var (
	// ErrNotConfigured indicates the organization has no enabled SSO config.
	ErrNotConfigured = errors.New("sso not configured")
	// ErrInvalidInput indicates configuration or request validation failed.
	ErrInvalidInput = errors.New("invalid sso input")
	// ErrInvalidState indicates the callback state is missing, expired, or forged.
	ErrInvalidState = errors.New("invalid sso state")
	// ErrNotProvisioned indicates the authenticated user is not a member of the org.
	ErrNotProvisioned = errors.New("user is not provisioned for this organization")
	// ErrVerification indicates the IdP token exchange or id_token verification failed.
	ErrVerification = errors.New("sso verification failed")
)

// Config is a tenant's OIDC configuration.
type Config struct {
	OrganizationID        string    `bson:"_id"                   json:"organizationId"`
	Enabled               bool      `bson:"enabled"               json:"enabled"`
	Issuer                string    `bson:"issuer"                json:"issuer"`
	ClientID              string    `bson:"clientId"              json:"clientId"`
	ClientSecret          string    `bson:"clientSecret"          json:"-"`
	AuthorizationEndpoint string    `bson:"authorizationEndpoint" json:"authorizationEndpoint"`
	TokenEndpoint         string    `bson:"tokenEndpoint"         json:"tokenEndpoint"`
	JWKSURI               string    `bson:"jwksUri"               json:"jwksUri"`
	RedirectURI           string    `bson:"redirectUri"           json:"redirectUri"`
	UpdatedAt             time.Time `bson:"updatedAt"             json:"updatedAt"`
}

// SetConfigInput is a full replacement of a tenant's OIDC configuration.
type SetConfigInput struct {
	Enabled               bool
	Issuer                string
	ClientID              string
	ClientSecret          string
	AuthorizationEndpoint string
	TokenEndpoint         string
	JWKSURI               string
	RedirectURI           string
}

// Claims are the verified identity claims from an id_token.
type Claims struct {
	Subject string
	Email   string
}

// Session is the LaunchPad session issued for a federated user.
type Session struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	TokenType    string `json:"tokenType"`
	ExpiresIn    int64  `json:"expiresIn"`
}

// AuthState is the short-lived server-side state for an in-flight login,
// keyed by the opaque `state` value and consumed once at callback.
type AuthState struct {
	OrganizationID string `json:"organizationId"`
	Nonce          string `json:"nonce"`
	ActorUserID    string `json:"actorUserId,omitempty"`
	Provider       string `json:"provider,omitempty"`
}
