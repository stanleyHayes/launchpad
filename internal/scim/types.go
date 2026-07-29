// Package scim implements SCIM 2.0 (RFC 7643/7644) user provisioning so a
// customer's identity provider (Okta, Entra ID, …) can create, update, and
// deactivate users in their LaunchPad tenant. Every request is authenticated by
// a per-organization bearer token and scoped to that organization, so an IdP
// can only provision into its own tenant.
package scim

import (
	"encoding/json"
	"errors"
	"time"
)

// SCIM schema URNs.
const (
	SchemaUser                  = "urn:ietf:params:scim:schemas:core:2.0:User"
	SchemaGroup                 = "urn:ietf:params:scim:schemas:core:2.0:Group"
	SchemaListResponse          = "urn:ietf:params:scim:api:messages:2.0:ListResponse"
	SchemaError                 = "urn:ietf:params:scim:api:messages:2.0:Error"
	SchemaPatchOp               = "urn:ietf:params:scim:api:messages:2.0:PatchOp"
	SchemaServiceProviderConfig = "urn:ietf:params:scim:schemas:core:2.0:ServiceProviderConfig"

	// ContentType is the SCIM media type.
	ContentType = "application/scim+json"

	resourceTypeUser  = "User"
	resourceTypeGroup = "Group"
)

var (
	// ErrNotFound indicates the SCIM resource does not exist in the tenant.
	ErrNotFound = errors.New("scim resource not found")
	// ErrConflict indicates a resource with the same userName already exists.
	ErrConflict = errors.New("scim resource already exists")
	// ErrInvalid indicates a malformed SCIM request.
	ErrInvalid = errors.New("invalid scim request")
	// ErrUnauthorized indicates a missing or invalid provisioning token.
	ErrUnauthorized = errors.New("scim unauthorized")
	// ErrAccountExists indicates the email already belongs to an account (a
	// Provisioner signal, not an HTTP-facing error).
	ErrAccountExists = errors.New("account already exists")
	// ErrNotOrgMember indicates the email's account is not a member of the
	// organization (a Provisioner signal).
	ErrNotOrgMember = errors.New("account is not an organization member")
)

// Record is the stored mapping between a SCIM user resource and a LaunchPad user.
type Record struct {
	ID             string    `bson:"_id"`
	OrganizationID string    `bson:"organizationId"`
	UserID         string    `bson:"userId"`
	ExternalID     string    `bson:"externalId,omitempty"`
	UserName       string    `bson:"userName"`
	GivenName      string    `bson:"givenName,omitempty"`
	FamilyName     string    `bson:"familyName,omitempty"`
	Active         bool      `bson:"active"`
	CreatedAt      time.Time `bson:"createdAt"`
	UpdatedAt      time.Time `bson:"updatedAt"`
}

// GroupRecord is a stored SCIM group within a tenant. Members holds SCIM User
// resource ids (Record.ID) that belong to the same organization.
type GroupRecord struct {
	ID             string    `bson:"_id"`
	OrganizationID string    `bson:"organizationId"`
	ExternalID     string    `bson:"externalId,omitempty"`
	DisplayName    string    `bson:"displayName"`
	Members        []string  `bson:"members"`
	CreatedAt      time.Time `bson:"createdAt"`
	UpdatedAt      time.Time `bson:"updatedAt"`
}

// GroupInput is the normalized input for creating or replacing a group.
type GroupInput struct {
	DisplayName string
	ExternalID  string
	MemberIDs   []string
}

// GroupMemberOp is how a SCIM PATCH operation changes a group's membership.
type GroupMemberOp int

// Group membership patch modes. GroupMembersRemove removes the listed ids (an
// empty list is a no-op); GroupMembersClear removes the entire membership (a
// valueless `remove` on path "members", per RFC 7644 §3.5.2.2).
const (
	GroupMembersUnchanged GroupMemberOp = iota
	GroupMembersAdd
	GroupMembersRemove
	GroupMembersReplace
	GroupMembersClear
)

// GroupPatchChange is a normalized SCIM PATCH operation on a group: an optional
// rename and an optional membership change. The handler parses raw SCIM PATCH
// operations into these; the service applies them.
type GroupPatchChange struct {
	DisplayName *string
	MemberOp    GroupMemberOp
	MemberIDs   []string
}

// CreateUserInput is the normalized input for provisioning a user.
type CreateUserInput struct {
	UserName    string
	ExternalID  string
	Email       string
	DisplayName string
	GivenName   string
	FamilyName  string
	Active      bool
}

// User is a SCIM core User resource (subset used for provisioning).
type User struct {
	Schemas     []string `json:"schemas"`
	ID          string   `json:"id,omitempty"`
	ExternalID  string   `json:"externalId,omitempty"`
	UserName    string   `json:"userName"`
	Name        *Name    `json:"name,omitempty"`
	DisplayName string   `json:"displayName,omitempty"`
	Emails      []Email  `json:"emails,omitempty"`
	Active      bool     `json:"active"`
	Meta        *Meta    `json:"meta,omitempty"`
}

// Name is a SCIM user's structured name.
type Name struct {
	GivenName  string `json:"givenName,omitempty"`
	FamilyName string `json:"familyName,omitempty"`
	Formatted  string `json:"formatted,omitempty"`
}

// Email is a SCIM multi-valued email.
type Email struct {
	Value   string `json:"value"`
	Primary bool   `json:"primary,omitempty"`
	Type    string `json:"type,omitempty"`
}

// Meta is SCIM resource metadata.
type Meta struct {
	ResourceType string `json:"resourceType"`
	Created      string `json:"created,omitempty"`
	LastModified string `json:"lastModified,omitempty"`
	Location     string `json:"location,omitempty"`
}

// ListResponse is a SCIM list response envelope.
type ListResponse struct {
	Schemas      []string `json:"schemas"`
	TotalResults int      `json:"totalResults"`
	StartIndex   int      `json:"startIndex"`
	ItemsPerPage int      `json:"itemsPerPage"`
	Resources    []User   `json:"Resources"` //nolint:tagliatelle // SCIM RFC 7644 field name
}

// Group is a SCIM core Group resource.
type Group struct {
	Schemas     []string      `json:"schemas"`
	ID          string        `json:"id,omitempty"`
	ExternalID  string        `json:"externalId,omitempty"`
	DisplayName string        `json:"displayName"`
	Members     []GroupMember `json:"members"`
	Meta        *Meta         `json:"meta,omitempty"`
}

// GroupMember is a SCIM group member reference (value = SCIM User resource id).
type GroupMember struct {
	Value   string `json:"value"`
	Display string `json:"display,omitempty"`
}

// GroupListResponse is a SCIM list response envelope of groups.
type GroupListResponse struct {
	Schemas      []string `json:"schemas"`
	TotalResults int      `json:"totalResults"`
	StartIndex   int      `json:"startIndex"`
	ItemsPerPage int      `json:"itemsPerPage"`
	Resources    []Group  `json:"Resources"` //nolint:tagliatelle // SCIM RFC 7644 field name
}

// PatchRequest is a SCIM PATCH request.
type PatchRequest struct {
	Schemas    []string  `json:"schemas"`
	Operations []PatchOp `json:"Operations"` //nolint:tagliatelle // SCIM RFC 7644 field name
}

// PatchOp is a single SCIM patch operation.
type PatchOp struct {
	Op    string          `json:"op"`
	Path  string          `json:"path,omitempty"`
	Value json.RawMessage `json:"value,omitempty"`
}

// ErrorResponse is a SCIM error payload.
type ErrorResponse struct {
	Schemas []string `json:"schemas"`
	Detail  string   `json:"detail"`
	Status  string   `json:"status"`
}

// ServiceProviderConfig advertises supported SCIM features.
type ServiceProviderConfig struct {
	Schemas               []string             `json:"schemas"`
	Patch                 Supported            `json:"patch"`
	Bulk                  BulkConfig           `json:"bulk"`
	Filter                FilterConfig         `json:"filter"`
	ChangePassword        Supported            `json:"changePassword"`
	Sort                  Supported            `json:"sort"`
	ETag                  Supported            `json:"etag"`
	AuthenticationSchemes []AuthenticationType `json:"authenticationSchemes"`
}

// Supported is a simple feature flag.
type Supported struct {
	Supported bool `json:"supported"`
}

// BulkConfig describes bulk-operation support.
type BulkConfig struct {
	Supported      bool `json:"supported"`
	MaxOperations  int  `json:"maxOperations"`
	MaxPayloadSize int  `json:"maxPayloadSize"`
}

// FilterConfig describes filter support.
type FilterConfig struct {
	Supported  bool `json:"supported"`
	MaxResults int  `json:"maxResults"`
}

// AuthenticationType describes an accepted authentication scheme.
type AuthenticationType struct {
	Type        string `json:"type"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Primary     bool   `json:"primary"`
}
