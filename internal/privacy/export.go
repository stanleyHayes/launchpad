package privacy

import (
	"context"
	"fmt"
	"time"

	"launchpad/internal/assignments"
	"launchpad/internal/audit"
	"launchpad/internal/departments"
	"launchpad/internal/employees"
	"launchpad/internal/journeys"
	"launchpad/internal/organizations"
)

const (
	// employeePageSize matches the employee repository's maximum page size so
	// the export walks every employee without tripping the limit clamp.
	employeePageSize = 100
	// auditRecentLimit bounds the recent audit events embedded in the export;
	// the total covers the full history.
	auditRecentLimit = 50
)

// OrganizationReader loads an organization and its memberships.
// organizations.Repository satisfies it.
type OrganizationReader interface {
	GetByID(ctx context.Context, id string) (organizations.Organization, error)
	ListMemberships(ctx context.Context, organizationID string) ([]organizations.Membership, error)
}

// EmployeeLister pages an organization's employees. employees.Repository
// satisfies it.
type EmployeeLister interface {
	List(ctx context.Context, organizationID string, offset, limit int64) ([]employees.Employee, error)
}

// DepartmentLister lists an organization's departments and job roles.
// departments.Repository satisfies it.
type DepartmentLister interface {
	ListDepartments(ctx context.Context, organizationID string) ([]departments.Department, error)
	ListJobRoles(ctx context.Context, organizationID string) ([]departments.JobRole, error)
}

// JourneyLister lists an organization's journey templates.
// journeys.Repository satisfies it.
type JourneyLister interface {
	ListTemplates(ctx context.Context, organizationID string) ([]journeys.Template, error)
}

// AssignmentLister lists an organization's journey assignments and approvals.
// assignments.Repository satisfies it.
type AssignmentLister interface {
	ListAssignments(ctx context.Context, organizationID string) ([]assignments.JourneyAssignment, error)
	ListApprovals(ctx context.Context, organizationID string) ([]assignments.Approval, error)
}

// AuditReader summarizes an organization's audit events. audit.Repository
// satisfies it.
type AuditReader interface {
	CountByOrganization(ctx context.Context, organizationID string) (int64, error)
	ListByOrganization(ctx context.Context, organizationID string, limit int64) ([]audit.Event, error)
}

// AuditExport carries the full audit-event count plus the most recent events.
type AuditExport struct {
	Total  int64         `json:"total"`
	Recent []audit.Event `json:"recent"`
}

// Export is the GDPR data-export document for one organization (PRD 7.4).
type Export struct {
	GeneratedAt  time.Time                       `json:"generatedAt"`
	Organization organizations.Organization      `json:"organization"`
	Memberships  []organizations.Membership      `json:"memberships"`
	Employees    []employees.Employee            `json:"employees"`
	Departments  []departments.Department        `json:"departments"`
	JobRoles     []departments.JobRole           `json:"jobRoles"`
	Journeys     []journeys.Template             `json:"journeys"`
	Assignments  []assignments.JourneyAssignment `json:"assignments"`
	Approvals    []assignments.Approval          `json:"approvals"`
	AuditEvents  AuditExport                     `json:"auditEvents"`
}

// ExportService assembles organization data exports from read-only ports.
type ExportService struct {
	orgs        OrganizationReader
	employees   EmployeeLister
	departments DepartmentLister
	journeys    JourneyLister
	assignments AssignmentLister
	audit       AuditReader
}

// NewExportService constructs an ExportService.
func NewExportService(
	orgs OrganizationReader,
	employeeLister EmployeeLister,
	departmentLister DepartmentLister,
	journeyLister JourneyLister,
	assignmentLister AssignmentLister,
	auditReader AuditReader,
) *ExportService {
	return &ExportService{
		orgs:        orgs,
		employees:   employeeLister,
		departments: departmentLister,
		journeys:    journeyLister,
		assignments: assignmentLister,
		audit:       auditReader,
	}
}

// Export returns all exportable data of an organization.
func (s *ExportService) Export(ctx context.Context, organizationID string) (Export, error) {
	org, err := s.orgs.GetByID(ctx, organizationID)
	if err != nil {
		return Export{}, fmt.Errorf("get organization: %w", err)
	}

	memberships, err := s.orgs.ListMemberships(ctx, organizationID)
	if err != nil {
		return Export{}, fmt.Errorf("list memberships: %w", err)
	}

	employeeList, err := s.listAllEmployees(ctx, organizationID)
	if err != nil {
		return Export{}, err
	}

	departmentList, err := s.departments.ListDepartments(ctx, organizationID)
	if err != nil {
		return Export{}, fmt.Errorf("list departments: %w", err)
	}

	jobRoleList, err := s.departments.ListJobRoles(ctx, organizationID)
	if err != nil {
		return Export{}, fmt.Errorf("list job roles: %w", err)
	}

	journeyList, err := s.journeys.ListTemplates(ctx, organizationID)
	if err != nil {
		return Export{}, fmt.Errorf("list journeys: %w", err)
	}

	assignmentList, err := s.assignments.ListAssignments(ctx, organizationID)
	if err != nil {
		return Export{}, fmt.Errorf("list assignments: %w", err)
	}

	approvalList, err := s.assignments.ListApprovals(ctx, organizationID)
	if err != nil {
		return Export{}, fmt.Errorf("list approvals: %w", err)
	}

	auditExport, err := s.auditSummary(ctx, organizationID)
	if err != nil {
		return Export{}, err
	}

	return Export{
		GeneratedAt:  time.Now().UTC(),
		Organization: org,
		Memberships:  memberships,
		Employees:    employeeList,
		Departments:  departmentList,
		JobRoles:     jobRoleList,
		Journeys:     journeyList,
		Assignments:  assignmentList,
		Approvals:    approvalList,
		AuditEvents:  auditExport,
	}, nil
}

// listAllEmployees pages through the employee repository until a short page
// ends the walk.
func (s *ExportService) listAllEmployees(ctx context.Context, organizationID string) ([]employees.Employee, error) {
	out := make([]employees.Employee, 0)

	for offset := int64(0); ; offset += employeePageSize {
		page, err := s.employees.List(ctx, organizationID, offset, employeePageSize)
		if err != nil {
			return nil, fmt.Errorf("list employees: %w", err)
		}

		out = append(out, page...)

		if int64(len(page)) < employeePageSize {
			return out, nil
		}
	}
}

func (s *ExportService) auditSummary(ctx context.Context, organizationID string) (AuditExport, error) {
	total, err := s.audit.CountByOrganization(ctx, organizationID)
	if err != nil {
		return AuditExport{}, fmt.Errorf("count audit events: %w", err)
	}

	recent, err := s.audit.ListByOrganization(ctx, organizationID, auditRecentLimit)
	if err != nil {
		return AuditExport{}, fmt.Errorf("list audit events: %w", err)
	}

	return AuditExport{Total: total, Recent: recent}, nil
}
