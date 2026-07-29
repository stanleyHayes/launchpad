package assignments

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"launchpad/internal/employees"
)

// autoAssignActor is recorded as the assigning actor (and fallback approver)
// when a rule assigns a journey without a human actor, e.g. on employee
// create or HRIS sync.
const autoAssignActor = "system"

// Rule auto-assigns a published journey to employees whose department and job
// role match. An empty DepartmentID or JobRoleID matches every employee.
type Rule struct {
	ID                string    `bson:"_id"                    json:"id"`
	OrganizationID    string    `bson:"organizationId"         json:"organizationId"`
	Name              string    `bson:"name"                   json:"name"`
	JourneyTemplateID string    `bson:"journeyTemplateId"      json:"journeyTemplateId"`
	DepartmentID      string    `bson:"departmentId,omitempty" json:"departmentId,omitempty"`
	JobRoleID         string    `bson:"jobRoleId,omitempty"    json:"jobRoleId,omitempty"`
	Active            bool      `bson:"active"                 json:"active"`
	CreatedBy         string    `bson:"createdBy"              json:"createdBy"`
	CreatedAt         time.Time `bson:"createdAt"              json:"createdAt"`
}

// CreateRuleInput creates an assignment rule. Rules start active.
type CreateRuleInput struct {
	Name              string
	JourneyTemplateID string
	DepartmentID      string
	JobRoleID         string
}

// UpdateRuleInput replaces a rule's mutable fields.
type UpdateRuleInput struct {
	Name              string
	JourneyTemplateID string
	DepartmentID      string
	JobRoleID         string
	Active            bool
}

// RuleRepository persists assignment rules.
type RuleRepository interface {
	CreateRule(ctx context.Context, rule Rule) error
	GetRule(ctx context.Context, organizationID, ruleID string) (Rule, error)
	ListRules(ctx context.Context, organizationID string) ([]Rule, error)
	UpdateRule(ctx context.Context, rule Rule) error
	DeleteRule(ctx context.Context, organizationID, ruleID string) error
}

// RuleAssigner performs the actual assignment for a rule. Implemented by
// *Service; kept as an interface so the rule service stays testable on its
// own.
type RuleAssigner interface {
	Assign(ctx context.Context, organizationID, actorUserID string, in AssignInput) (AssignResult, error)
}

// RuleService implements assignment rule use cases.
type RuleService struct {
	rules     RuleRepository
	journeys  JourneyReader
	employees EmployeeReader
	assigner  RuleAssigner
}

// NewRuleService constructs a RuleService.
func NewRuleService(
	rules RuleRepository,
	journeyReader JourneyReader,
	employeeReader EmployeeReader,
	assigner RuleAssigner,
) *RuleService {
	return &RuleService{
		rules:     rules,
		journeys:  journeyReader,
		employees: employeeReader,
		assigner:  assigner,
	}
}

// ListRules lists an organization's assignment rules.
func (s *RuleService) ListRules(ctx context.Context, organizationID string) ([]Rule, error) {
	if organizationID == "" {
		return nil, ErrInvalidInput
	}

	items, err := s.rules.ListRules(ctx, organizationID)
	if err != nil {
		return nil, fmt.Errorf("list assignment rules: %w", err)
	}

	return items, nil
}

// CreateRule creates an active rule for a published journey template.
func (s *RuleService) CreateRule(
	ctx context.Context,
	organizationID, actorUserID string,
	in CreateRuleInput,
) (Rule, error) {
	name := strings.TrimSpace(in.Name)
	if organizationID == "" || name == "" || in.JourneyTemplateID == "" {
		return Rule{}, ErrInvalidInput
	}

	if _, err := s.journeys.RequirePublished(ctx, organizationID, in.JourneyTemplateID); err != nil {
		return Rule{}, fmt.Errorf("load journey: %w", err)
	}

	rule := Rule{
		ID:                uuid.NewString(),
		OrganizationID:    organizationID,
		Name:              name,
		JourneyTemplateID: in.JourneyTemplateID,
		DepartmentID:      strings.TrimSpace(in.DepartmentID),
		JobRoleID:         strings.TrimSpace(in.JobRoleID),
		Active:            true,
		CreatedBy:         actorUserID,
		CreatedAt:         time.Now().UTC(),
	}

	if err := s.rules.CreateRule(ctx, rule); err != nil {
		return Rule{}, fmt.Errorf("create assignment rule: %w", err)
	}

	return rule, nil
}

// UpdateRule replaces a rule's mutable fields.
func (s *RuleService) UpdateRule(
	ctx context.Context,
	organizationID, ruleID string,
	in UpdateRuleInput,
) (Rule, error) {
	name := strings.TrimSpace(in.Name)
	if organizationID == "" || ruleID == "" || name == "" || in.JourneyTemplateID == "" {
		return Rule{}, ErrInvalidInput
	}

	rule, err := s.rules.GetRule(ctx, organizationID, ruleID)
	if err != nil {
		return Rule{}, fmt.Errorf("get assignment rule: %w", err)
	}

	if _, err := s.journeys.RequirePublished(ctx, organizationID, in.JourneyTemplateID); err != nil {
		return Rule{}, fmt.Errorf("load journey: %w", err)
	}

	rule.Name = name
	rule.JourneyTemplateID = in.JourneyTemplateID
	rule.DepartmentID = strings.TrimSpace(in.DepartmentID)
	rule.JobRoleID = strings.TrimSpace(in.JobRoleID)
	rule.Active = in.Active

	if err := s.rules.UpdateRule(ctx, rule); err != nil {
		return Rule{}, fmt.Errorf("update assignment rule: %w", err)
	}

	return rule, nil
}

// DeleteRule removes a rule. Existing assignments are left untouched.
func (s *RuleService) DeleteRule(ctx context.Context, organizationID, ruleID string) error {
	if organizationID == "" || ruleID == "" {
		return ErrInvalidInput
	}

	if err := s.rules.DeleteRule(ctx, organizationID, ruleID); err != nil {
		return fmt.Errorf("delete assignment rule: %w", err)
	}

	return nil
}

// ApplyAssignmentRules assigns every active matching rule's journey to one
// employee. It is the assignments-side implementation of the employees
// package's RuleApplier port, invoked after a successful employee create
// (manual or HRIS). Employees who already have an active assignment for a
// rule's template are skipped, so re-runs are safe. Assignment starts at the
// employee's start date, so future hires get a scheduled assignment.
func (s *RuleService) ApplyAssignmentRules(ctx context.Context, employee employees.Employee) error {
	rules, err := s.rules.ListRules(ctx, employee.OrganizationID)
	if err != nil {
		return fmt.Errorf("list assignment rules: %w", err)
	}

	for _, rule := range rules {
		if !rule.Active || !ruleMatches(rule, employee) {
			continue
		}

		if _, err := s.assigner.Assign(ctx, employee.OrganizationID, autoAssignActor, AssignInput{
			EmployeeID:        employee.ID,
			JourneyTemplateID: rule.JourneyTemplateID,
			StartsAt:          employee.StartDate,
		}); err != nil {
			if errors.Is(err, ErrAlreadyAssigned) {
				continue
			}

			return fmt.Errorf("assign rule %q to employee: %w", rule.ID, err)
		}
	}

	return nil
}

// RunRule applies one rule to every matching active employee, skipping those
// already assigned. Counts mirror AssignDepartmentResult.
func (s *RuleService) RunRule(
	ctx context.Context,
	organizationID, actorUserID, ruleID string,
) (AssignDepartmentResult, error) {
	if organizationID == "" || ruleID == "" {
		return AssignDepartmentResult{}, ErrInvalidInput
	}

	rule, err := s.rules.GetRule(ctx, organizationID, ruleID)
	if err != nil {
		return AssignDepartmentResult{}, fmt.Errorf("get assignment rule: %w", err)
	}

	if !rule.Active {
		return AssignDepartmentResult{}, ErrInvalidState
	}

	total := AssignDepartmentResult{Employees: 0, Assigned: 0, Skipped: 0}

	const pageSize = 100

	for offset := int64(0); ; offset += pageSize {
		page, done, err := s.runRulePage(ctx, organizationID, actorUserID, rule, offset, pageSize)
		if err != nil {
			return AssignDepartmentResult{}, err
		}

		total.Employees += page.Employees
		total.Assigned += page.Assigned
		total.Skipped += page.Skipped

		if done {
			break
		}
	}

	return total, nil
}

// runRulePage applies the rule to one page of employees, returning per-page
// counts and whether this was the last page.
func (s *RuleService) runRulePage(
	ctx context.Context,
	organizationID, actorUserID string,
	rule Rule,
	offset, limit int64,
) (AssignDepartmentResult, bool, error) {
	page, err := s.employees.List(ctx, organizationID, offset, limit)
	if err != nil {
		return AssignDepartmentResult{}, false, fmt.Errorf("list employees: %w", err)
	}

	result := AssignDepartmentResult{Employees: 0, Assigned: 0, Skipped: 0}

	for _, employee := range page {
		if employee.Status != statusActive || !ruleMatches(rule, employee) {
			continue
		}

		result.Employees++

		if _, err := s.assigner.Assign(ctx, organizationID, actorUserID, AssignInput{
			EmployeeID:        employee.ID,
			JourneyTemplateID: rule.JourneyTemplateID,
			StartsAt:          time.Time{},
		}); err != nil {
			if errors.Is(err, ErrAlreadyAssigned) {
				result.Skipped++

				continue
			}

			return AssignDepartmentResult{}, false, fmt.Errorf("assign rule %q to employee: %w", rule.ID, err)
		}

		result.Assigned++
	}

	return result, len(page) < int(limit), nil
}

// ruleMatches reports whether the rule's department/job role criteria match
// the employee. Empty criteria match everyone.
func ruleMatches(rule Rule, employee employees.Employee) bool {
	if rule.DepartmentID != "" && rule.DepartmentID != employee.DepartmentID {
		return false
	}

	if rule.JobRoleID != "" && rule.JobRoleID != employee.JobRoleID {
		return false
	}

	return true
}
