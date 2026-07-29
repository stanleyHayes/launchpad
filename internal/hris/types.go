// Package hris syncs a tenant's HR information system (BambooHR, Workday, …)
// into LaunchPad. A sync fetches the external employee directory via a provider
// adapter and persists it as a per-organization snapshot plus a sync-run record.
// Applying the snapshot to the employees domain (with department mapping and
// account provisioning) is a separate, org-specific step.
package hris

import (
	"errors"
	"time"
)

var (
	// ErrNotConfigured indicates the organization has no enabled HRIS config.
	ErrNotConfigured = errors.New("hris not configured")
	// ErrInvalidInput indicates configuration validation failed.
	ErrInvalidInput = errors.New("invalid hris input")
	// ErrUnsupportedProvider indicates the configured provider is unknown.
	ErrUnsupportedProvider = errors.New("unsupported hris provider")
	// ErrSyncFailed indicates a sync run failed (see the run's Error).
	ErrSyncFailed = errors.New("hris sync failed")
	// ErrNoDirectory indicates there is no synced directory snapshot to apply.
	ErrNoDirectory = errors.New("no synced hris directory")
)

// Sync run statuses.
const (
	StatusSuccess = "success"
	StatusFailed  = "failed"
)

// Config is a tenant's HRIS provider configuration.
type Config struct {
	OrganizationID string    `bson:"_id"               json:"organizationId"`
	Provider       string    `bson:"provider"          json:"provider"`
	Subdomain      string    `bson:"subdomain"         json:"subdomain"`
	APIToken       string    `bson:"apiToken"          json:"-"`
	BaseURL        string    `bson:"baseUrl,omitempty" json:"baseUrl,omitempty"`
	Enabled        bool      `bson:"enabled"           json:"enabled"`
	UpdatedAt      time.Time `bson:"updatedAt"         json:"updatedAt"`
}

// SetConfigInput is a full replacement of a tenant's HRIS configuration.
type SetConfigInput struct {
	Provider  string
	Subdomain string
	APIToken  string
	BaseURL   string
	Enabled   bool
}

// DirectoryEntry is a normalized employee record from an HRIS provider.
type DirectoryEntry struct {
	ExternalID string `bson:"externalId"           json:"externalId"`
	FirstName  string `bson:"firstName"            json:"firstName"`
	LastName   string `bson:"lastName"             json:"lastName"`
	Email      string `bson:"email"                json:"email"`
	JobTitle   string `bson:"jobTitle,omitempty"   json:"jobTitle,omitempty"`
	Department string `bson:"department,omitempty" json:"department,omitempty"`
	Active     bool   `bson:"active"               json:"active"`
}

// SyncRun records the outcome of a single sync.
type SyncRun struct {
	Provider   string    `bson:"provider"        json:"provider"`
	Status     string    `bson:"status"          json:"status"`
	EntryCount int       `bson:"entryCount"      json:"entryCount"`
	Error      string    `bson:"error,omitempty" json:"error,omitempty"`
	StartedAt  time.Time `bson:"startedAt"       json:"startedAt"`
	FinishedAt time.Time `bson:"finishedAt"      json:"finishedAt"`
}

// State is a tenant's latest synced directory and last run.
type State struct {
	Entries []DirectoryEntry `bson:"entries" json:"entries"`
	LastRun SyncRun          `bson:"lastRun" json:"lastRun"`
}

// ApplyResult summarizes an apply of the synced directory into the employees
// domain: how many entries were seen, created, skipped (already present), and
// failed, with a bounded list of per-entry error messages for the failures.
type ApplyResult struct {
	Total   int      `json:"total"`
	Created int      `json:"created"`
	Skipped int      `json:"skipped"`
	Failed  int      `json:"failed"`
	Errors  []string `json:"errors,omitempty"`
}
