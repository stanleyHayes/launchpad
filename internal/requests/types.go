// Package requests manages employee equipment and access requests (PRD
// §5.3.8): employees raise requests, managers approve or reject them, and
// approved requests are marked fulfilled once provisioned.
package requests

import (
	"errors"
	"time"
)

var (
	// ErrNotFound indicates the request does not exist.
	ErrNotFound = errors.New("request not found")
	// ErrInvalidInput indicates validation failed.
	ErrInvalidInput = errors.New("invalid request input")
	// ErrInvalidState indicates an illegal status transition.
	ErrInvalidState = errors.New("invalid request state")
	// ErrForbidden indicates the actor may not access the request.
	ErrForbidden = errors.New("request access denied")
)

const (
	kindEquipment = "equipment"
	kindAccess    = "access"
)

// Request items (PRD §5.3.8): equipment kinds cover hardware, access kinds
// cover accounts and permissions.
const (
	itemLaptop        = "laptop"
	itemMobile        = "mobile"
	itemBadge         = "badge"
	itemDeskEquipment = "desk_equipment"
	itemVPN           = "vpn"
	itemEmail         = "email"
	itemSoftware      = "software"
	itemGitHubRepo    = "github_repo"
	itemJiraProject   = "jira_project"
	itemOther         = "other"
)

const (
	statusPending   = "pending"
	statusApproved  = "approved"
	statusFulfilled = "fulfilled"
	statusRejected  = "rejected"
	statusCancelled = "cancelled"
)

// Request is one org-scoped equipment or access request raised by an
// employee.
type Request struct {
	ID                  string     `bson:"_id"                      json:"id"`
	OrganizationID      string     `bson:"organizationId"           json:"organizationId"`
	Kind                string     `bson:"kind"                     json:"kind"`
	Item                string     `bson:"item"                     json:"item"`
	Details             string     `bson:"details,omitempty"        json:"details,omitempty"`
	Status              string     `bson:"status"                   json:"status"`
	RequesterEmployeeID string     `bson:"requesterEmployeeId"      json:"requesterEmployeeId"`
	ApproverUserID      string     `bson:"approverUserId,omitempty" json:"approverUserId,omitempty"`
	DecisionNote        string     `bson:"decisionNote,omitempty"   json:"decisionNote,omitempty"`
	DecidedAt           *time.Time `bson:"decidedAt,omitempty"      json:"decidedAt,omitempty"`
	FulfilledAt         *time.Time `bson:"fulfilledAt,omitempty"    json:"fulfilledAt,omitempty"`
	CreatedAt           time.Time  `bson:"createdAt"                json:"createdAt"`
	UpdatedAt           time.Time  `bson:"updatedAt"                json:"updatedAt"`
}

// CreateInput raises a request for an employee.
type CreateInput struct {
	OrganizationID string
	EmployeeID     string
	Kind           string
	Item           string
	Details        string
}

// DecideInput records an approve/reject decision.
type DecideInput struct {
	OrganizationID string
	RequestID      string
	ApproverUserID string
	Approve        bool
	Note           string
}

func isValidKind(kind string) bool {
	return kind == kindEquipment || kind == kindAccess
}

func isValidItem(item string) bool {
	switch item {
	case itemLaptop, itemMobile, itemBadge, itemDeskEquipment, itemVPN,
		itemEmail, itemSoftware, itemGitHubRepo, itemJiraProject, itemOther:
		return true
	default:
		return false
	}
}

func isValidStatus(status string) bool {
	switch status {
	case statusPending, statusApproved, statusFulfilled, statusRejected, statusCancelled:
		return true
	default:
		return false
	}
}
