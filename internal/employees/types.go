// Package employees manages tenant-scoped employee records.
package employees

import (
	"errors"
	"time"
)

var (
	// ErrNotFound indicates an employee was not found.
	ErrNotFound = errors.New("employee not found")
	// ErrEmailTaken indicates work email already exists in the organization.
	ErrEmailTaken = errors.New("employee work email already taken")
	// ErrInvalidInput indicates validation failed.
	ErrInvalidInput = errors.New("invalid employee input")
	// ErrInvalidReference indicates department, role, or manager reference is invalid.
	ErrInvalidReference = errors.New("invalid employee reference")
	// ErrAlreadyProvisioned indicates the employee already has portal access.
	ErrAlreadyProvisioned = errors.New("employee already provisioned")
)

const (
	statusInvited = "invited"
	statusActive  = "active"
)

// Employee is a tenant-scoped employee record.
type Employee struct {
	ID                string         `bson:"_id"                         json:"id"`
	OrganizationID    string         `bson:"organizationId"              json:"organizationId"`
	UserID            string         `bson:"userId,omitempty"            json:"userId,omitempty"`
	EmployeeNumber    string         `bson:"employeeNumber"              json:"employeeNumber"`
	FirstName         string         `bson:"firstName"                   json:"firstName"`
	LastName          string         `bson:"lastName"                    json:"lastName"`
	WorkEmail         string         `bson:"workEmail"                   json:"workEmail"`
	JobRoleID         string         `bson:"jobRoleId,omitempty"         json:"jobRoleId,omitempty"`
	DepartmentID      string         `bson:"departmentId,omitempty"      json:"departmentId,omitempty"`
	ManagerEmployeeID string         `bson:"managerEmployeeId,omitempty" json:"managerEmployeeId,omitempty"`
	StartDate         time.Time      `bson:"startDate"                   json:"startDate"`
	Status            string         `bson:"status"                      json:"status"`
	Metadata          map[string]any `bson:"metadata"                    json:"metadata"`
	CreatedAt         time.Time      `bson:"createdAt"                   json:"createdAt"`
	UpdatedAt         time.Time      `bson:"updatedAt"                   json:"updatedAt"`
}

// CreateInput creates an employee.
type CreateInput struct {
	EmployeeNumber    string
	FirstName         string
	LastName          string
	WorkEmail         string
	JobRoleID         string
	DepartmentID      string
	ManagerEmployeeID string
	StartDate         time.Time
}

// UpdateInput updates mutable employee fields.
type UpdateInput struct {
	FirstName         *string
	LastName          *string
	EmployeeNumber    *string
	JobRoleID         *string
	DepartmentID      *string
	ManagerEmployeeID *string
	Status            *string
}
