// Package departments manages organization departments and job roles.
package departments

import (
	"errors"
	"time"
)

var (
	// ErrNotFound indicates a department was not found.
	ErrNotFound = errors.New("department not found")
	// ErrNameTaken indicates a department name already exists in the organization.
	ErrNameTaken = errors.New("department name already taken")
	// ErrInvalidInput indicates validation failed.
	ErrInvalidInput = errors.New("invalid department input")
	// ErrRoleNotFound indicates a job role was not found.
	ErrRoleNotFound = errors.New("job role not found")
	// ErrRoleNameTaken indicates a job role name already exists in the organization.
	ErrRoleNameTaken = errors.New("job role name already taken")
)

// Department is an organization department.
type Department struct {
	ID             string    `bson:"_id"            json:"id"`
	OrganizationID string    `bson:"organizationId" json:"organizationId"`
	Name           string    `bson:"name"           json:"name"`
	Description    string    `bson:"description"    json:"description"`
	CreatedAt      time.Time `bson:"createdAt"      json:"createdAt"`
	UpdatedAt      time.Time `bson:"updatedAt"      json:"updatedAt"`
}

// JobRole is a role used for journey assignment rules.
type JobRole struct {
	ID             string    `bson:"_id"            json:"id"`
	OrganizationID string    `bson:"organizationId" json:"organizationId"`
	Name           string    `bson:"name"           json:"name"`
	Description    string    `bson:"description"    json:"description"`
	CreatedAt      time.Time `bson:"createdAt"      json:"createdAt"`
	UpdatedAt      time.Time `bson:"updatedAt"      json:"updatedAt"`
}

// CreateDepartmentInput creates a department.
type CreateDepartmentInput struct {
	Name        string
	Description string
}

// CreateJobRoleInput creates a job role.
type CreateJobRoleInput struct {
	Name        string
	Description string
}
