package sso

import (
	"context"
	"time"
)

// ConfigStore persists per-organization OIDC configuration.
type ConfigStore interface {
	EnsureIndexes(ctx context.Context) error
	// GetByOrganization returns the tenant's config, or ErrNotConfigured if none.
	GetByOrganization(ctx context.Context, organizationID string) (Config, error)
	// SetConfig upserts the tenant's config.
	SetConfig(ctx context.Context, config Config) error
	// DeleteForOrganization removes the organization's OIDC config and
	// returns the number of documents deleted. Called only by the platform
	// GDPR tenant purge (PRD 7.4).
	DeleteForOrganization(ctx context.Context, organizationID string) (int64, error)
}

// SAMLConfigStore persists tenant SAML service-provider configuration.
type SAMLConfigStore interface {
	GetSAMLByOrganization(ctx context.Context, organizationID string) (SAMLConfig, error)
	SetSAMLConfig(ctx context.Context, config SAMLConfig) error
}

// StateStore holds short-lived login state keyed by the opaque `state` value.
type StateStore interface {
	Save(ctx context.Context, state string, data AuthState, ttl time.Duration) error
	// Consume atomically returns and deletes the state, so it can be used once.
	Consume(ctx context.Context, state string) (AuthState, error)
}

// Verifier performs the OIDC token exchange and id_token verification for a
// callback code, returning the verified identity claims. Implementations own
// all IdP network calls and cryptographic verification.
type Verifier interface {
	Verify(ctx context.Context, config Config, code, expectedNonce string) (Claims, error)
}

// SessionIssuer issues a LaunchPad session for a federated, already-provisioned
// user. Implemented in the composition root over the auth service.
type SessionIssuer interface {
	IssueFederatedSession(ctx context.Context, email, organizationID string) (Session, error)
}

// OrgResolver maps a public organization slug to its id.
type OrgResolver interface {
	OrganizationIDBySlug(ctx context.Context, slug string) (string, error)
}

// OrgSlugResolver maps an authenticated tenant id to its public login slug.
type OrgSlugResolver interface {
	OrganizationSlugByID(ctx context.Context, organizationID string) (string, error)
}
