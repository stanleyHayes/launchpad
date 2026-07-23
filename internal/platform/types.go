package platform

import (
	"errors"
	"time"
)

var (
	// ErrNotFound indicates the platform staff record does not exist.
	ErrNotFound = errors.New("platform staff not found")
	// ErrInvalidInput indicates validation failed.
	ErrInvalidInput = errors.New("invalid platform input")
)

const (
	rolePlatformOwner = "platform_owner"
	rolePlatformAdmin = "platform_admin"
	staffStatusActive = "active"
	fieldUserID       = "userId"
)

// Staff is a platform operator account.
type Staff struct {
	ID        string    `bson:"_id"       json:"id"`
	UserID    string    `bson:"userId"    json:"userId"`
	RoleCode  string    `bson:"roleCode"  json:"roleCode"`
	Status    string    `bson:"status"    json:"status"`
	CreatedAt time.Time `bson:"createdAt" json:"createdAt"`
}

// Overview summarizes platform-wide metrics.
type Overview struct {
	TotalOrgs       int64 `json:"totalOrgs"`
	TrialOrgs       int64 `json:"trialOrgs"`
	ActiveOrgs      int64 `json:"activeOrgs"`
	SuspendedOrgs   int64 `json:"suspendedOrgs"`
	TotalLeads      int64 `json:"totalLeads"`
	OpenTicketCount int64 `json:"openTicketCount"`
}

// RoleOwner returns the platform owner role code.
func RoleOwner() string {
	return rolePlatformOwner
}
