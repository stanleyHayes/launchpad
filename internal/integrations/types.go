// Package integrations implements the GitHub and Jira provider-connection
// domain: per-organization credentials are validated against the provider
// before they are persisted, stored encrypted at rest, and re-validated on
// demand by health checks.
package integrations

import (
	"errors"
	"time"
)

// Connection lifecycle statuses.
const (
	StatusConnected = "connected"
	StatusError     = "error"
)

// Supported provider keys.
const (
	ProviderGitHub = "github"
	ProviderJira   = "jira"
)

// Sentinel errors mapped to HTTP responses at the handler.
var (
	ErrInvalidInput      = errors.New("invalid integration input")
	ErrNotFound          = errors.New("integration connection not found")
	ErrUnknownProvider   = errors.New("unknown integration provider")
	ErrInvalidCredential = errors.New("provider rejected the credential")
	// ErrAlreadyConnected is reserved for flows that must reject a second
	// connection; Connect is intentionally idempotent (upsert) and does not
	// return it.
	ErrAlreadyConnected = errors.New("provider already connected")
)

// Connection is a tenant's link to one external provider. Token (and Email,
// when the provider needs basic-auth) only travel between service and store:
// the Mongo adapter encrypts the token at rest, and API responses use
// ConnectionResponse, which has no credential fields.
type Connection struct {
	ID             string     `bson:"_id"                  json:"id"`
	OrganizationID string     `bson:"organizationId"       json:"organizationId"`
	Provider       string     `bson:"provider"             json:"provider"`
	Status         string     `bson:"status"               json:"status"`
	BaseURL        string     `bson:"baseUrl,omitempty"    json:"baseUrl,omitempty"`
	Email          string     `bson:"email,omitempty"      json:"-"`
	AccountHandle  string     `bson:"accountHandle"        json:"accountHandle"`
	Token          string     `bson:"integrationToken"     json:"-"`
	LastSyncAt     *time.Time `bson:"lastSyncAt,omitempty" json:"lastSyncAt,omitempty"`
	LastError      string     `bson:"lastError,omitempty"  json:"lastError,omitempty"`
	CreatedBy      string     `bson:"createdBy"            json:"createdBy"`
	CreatedAt      time.Time  `bson:"createdAt"            json:"createdAt"`
	UpdatedAt      time.Time  `bson:"updatedAt"            json:"updatedAt"`
}

// ProviderInfo describes one supported provider in the registry.
type ProviderInfo struct {
	Key             string `json:"key"`
	Name            string `json:"name"`
	RequiresBaseURL bool   `json:"requiresBaseUrl"`
	RequiresEmail   bool   `json:"requiresEmail"`
}

// Providers returns the supported-provider registry.
func Providers() []ProviderInfo {
	return []ProviderInfo{
		{Key: ProviderGitHub, Name: "GitHub", RequiresBaseURL: false, RequiresEmail: false},
		{Key: ProviderJira, Name: "Jira", RequiresBaseURL: true, RequiresEmail: true},
	}
}

// ConnectInput carries the credential material for a connect or validation.
type ConnectInput struct {
	Token   string
	BaseURL string
	Email   string
}
