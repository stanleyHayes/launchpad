// Package knowledge implements per-tenant knowledge document management.
//
// Documents move through a lifecycle of draft -> approved -> indexed, with an
// explicit human approval gate before any content is handed to the AI indexer
// (PRD §5.3.5, §16.1). Archiving removes a document from the AI index.
package knowledge

import (
	"errors"
	"time"
)

var (
	// ErrNotFound indicates the document does not exist for the tenant.
	ErrNotFound = errors.New("knowledge document not found")
	// ErrInvalidInput indicates validation failed.
	ErrInvalidInput = errors.New("invalid knowledge document input")
	// ErrInvalidState indicates the document is not in a state that permits the action.
	ErrInvalidState = errors.New("knowledge document is not in a valid state for this action")
)

const (
	// StatusDraft is a newly created, unapproved document.
	StatusDraft = "draft"
	// StatusApproved is a document a manager has approved for AI indexing.
	StatusApproved = "approved"
	// StatusIndexed is a document whose content has been handed to the AI indexer.
	StatusIndexed = "indexed"
	// StatusArchived is a document removed from the AI index.
	StatusArchived = "archived"
)

const (
	// ScopeOrganization is visible to every member of the organization.
	ScopeOrganization = "organization"
	// ScopeRestricted is visible only to organization managers.
	ScopeRestricted = "restricted"
)

const (
	sourceManual     = "manual"
	sourceUpload     = "upload"
	sourceURL        = "url"
	sourceNotion     = "notion"
	sourceConfluence = "confluence"
	sourceGoogleDoc  = "google_drive"
	sourceGitHub     = "github"
	sourceSharePoint = "sharepoint"
	sourceWiki       = "wiki"
)

// Document is a tenant knowledge source available to the AI onboarding assistant.
type Document struct {
	ID               string            `bson:"_id"                        json:"id"`
	OrganizationID   string            `bson:"organizationId"             json:"organizationId"`
	Title            string            `bson:"title"                      json:"title"`
	Source           string            `bson:"source"                     json:"source"`
	URI              string            `bson:"uri,omitempty"              json:"uri,omitempty"`
	Body             string            `bson:"body,omitempty"             json:"body,omitempty"`
	Tags             []string          `bson:"tags,omitempty"             json:"tags,omitempty"`
	AccessScope      string            `bson:"accessScope"                json:"accessScope"`
	Status           string            `bson:"status"                     json:"status"`
	Version          int               `bson:"version"                    json:"version"`
	OwnerUserID      string            `bson:"ownerUserId"                json:"ownerUserId"`
	ReviewDate       *time.Time        `bson:"reviewDate,omitempty"       json:"reviewDate,omitempty"`
	RetentionDays    int               `bson:"retentionDays,omitempty"    json:"retentionDays,omitempty"`
	LastSyncedAt     *time.Time        `bson:"lastSyncedAt,omitempty"     json:"lastSyncedAt,omitempty"`
	StaleNotifiedAt  *time.Time        `bson:"staleNotifiedAt,omitempty"  json:"staleNotifiedAt,omitempty"`
	SyncError        string            `bson:"syncError,omitempty"        json:"syncError,omitempty"`
	History          []VersionSnapshot `bson:"history,omitempty"   json:"-"`
	ApprovedByUserID string            `bson:"approvedByUserId,omitempty" json:"approvedByUserId,omitempty"`
	ApprovedAt       *time.Time        `bson:"approvedAt,omitempty"       json:"approvedAt,omitempty"`
	IndexedAt        *time.Time        `bson:"indexedAt,omitempty"        json:"indexedAt,omitempty"`
	CreatedByUserID  string            `bson:"createdByUserId"            json:"createdByUserId"`
	CreatedAt        time.Time         `bson:"createdAt"                  json:"createdAt"`
	UpdatedAt        time.Time         `bson:"updatedAt"                  json:"updatedAt"`
}

type VersionSnapshot struct {
	Version     int       `bson:"version" json:"version"`
	Title       string    `bson:"title" json:"title"`
	Body        string    `bson:"body" json:"body"`
	URI         string    `bson:"uri" json:"uri"`
	Tags        []string  `bson:"tags" json:"tags"`
	AccessScope string    `bson:"accessScope" json:"accessScope"`
	SavedAt     time.Time `bson:"savedAt" json:"savedAt"`
}

// CreateInput is the payload to register a knowledge document.
type CreateInput struct {
	Title         string
	Source        string
	URI           string
	Body          string
	Tags          []string
	AccessScope   string
	OwnerUserID   string
	ReviewDate    *time.Time
	RetentionDays int
}

// UpdateInput mutates editable fields of a draft document. Nil fields are unchanged.
type UpdateInput struct {
	Title         *string
	URI           *string
	Body          *string
	Tags          *[]string
	AccessScope   *string
	OwnerUserID   *string
	ReviewDate    *time.Time
	RetentionDays *int
}

func isValidSource(source string) bool {
	switch source {
	case sourceManual, sourceUpload, sourceURL, sourceNotion, sourceConfluence,
		sourceGoogleDoc, sourceGitHub, sourceSharePoint, sourceWiki:
		return true
	default:
		return false
	}
}

func isValidScope(scope string) bool {
	return scope == ScopeOrganization || scope == ScopeRestricted
}
