package assignments

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"launchpad/internal/employees"
	"launchpad/internal/notifications"
	"launchpad/internal/support"
)

// employeeListPageSize is the page size used when scanning the directory for
// a manager's direct reports.
const employeeListPageSize = 100

// TeamRollup returns per-report assignment summaries for the caller's direct
// reports (PRD §5.3.9). The caller is resolved to their employee record;
// members without one (or without reports) get an empty rollup.
func (s *Service) TeamRollup(
	ctx context.Context,
	organizationID, managerUserID string,
) ([]TeamReportSummary, error) {
	manager, reports, err := s.resolveManagerReports(ctx, organizationID, managerUserID)
	if err != nil {
		return nil, err
	}

	pendingByEmployee, err := s.countPendingApprovalsByEmployee(ctx, organizationID, manager.UserID)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	summaries := make([]TeamReportSummary, 0, len(reports))

	for _, report := range reports {
		summary := TeamReportSummary{
			EmployeeID:           report.ID,
			Name:                 employeeDisplayName(report),
			ActiveAssignments:    0,
			CompletedAssignments: 0,
			OverdueSteps:         0,
			PendingApprovals:     pendingByEmployee[report.ID],
		}

		items, err := s.repo.ListAssignmentsForEmployee(ctx, organizationID, report.ID)
		if err != nil {
			return nil, fmt.Errorf("list report assignments: %w", err)
		}

		for _, assignment := range items {
			switch assignment.Status {
			case statusScheduled, statusInProgress:
				summary.ActiveAssignments++
			case statusCompleted:
				summary.CompletedAssignments++
			}

			overdue, err := s.countOverdueSteps(ctx, organizationID, assignment.ID, now)
			if err != nil {
				return nil, err
			}

			summary.OverdueSteps += overdue
		}

		summaries = append(summaries, summary)
	}

	return summaries, nil
}

// ReportBlocker records a blocker for the caller's employee record (PRD
// §5.4.4), backed by a support ticket. When a step assignment is referenced
// it must belong to the caller. The employee's manager is notified; a
// notification failure is logged but does not fail the committed report.
func (s *Service) ReportBlocker(
	ctx context.Context,
	organizationID, userID string,
	in ReportBlockerInput,
) (support.Blocker, error) {
	if s.blockers == nil {
		return support.Blocker{}, fmt.Errorf("%w: blocker store not configured", ErrInvalidState)
	}

	employee, err := s.employees.GetByUserID(ctx, organizationID, userID)
	if err != nil {
		return support.Blocker{}, fmt.Errorf("resolve employee: %w", err)
	}

	var stepTitle, stepLink string

	if in.StepAssignmentID != "" {
		step, err := s.repo.GetStep(ctx, organizationID, in.StepAssignmentID)
		if err != nil {
			return support.Blocker{}, fmt.Errorf("load step: %w", err)
		}

		if step.EmployeeID != employee.ID {
			return support.Blocker{}, ErrForbidden
		}

		stepTitle = step.Title
		stepLink = "/assignments/" + step.JourneyAssignmentID
	}

	blocker, err := s.blockers.ReportBlocker(ctx, support.ReportBlockerInput{
		OrganizationID:   organizationID,
		EmployeeID:       employee.ID,
		ReportedByUserID: userID,
		EmployeeName:     employeeDisplayName(employee),
		StepAssignmentID: in.StepAssignmentID,
		StepTitle:        stepTitle,
		Category:         in.Category,
		Message:          in.Message,
	})
	if err != nil {
		return support.Blocker{}, fmt.Errorf("report blocker: %w", err)
	}

	if err := s.notifyBlocker(ctx, organizationID, employee, stepLink); err != nil {
		slog.ErrorContext(ctx, "blocker notification failed", "blockerId", blocker.ID, "error", err)
	}

	return blocker, nil
}

// ListTeamBlockers returns blockers raised by the caller's direct reports,
// newest first.
func (s *Service) ListTeamBlockers(
	ctx context.Context,
	organizationID, managerUserID string,
) ([]support.Blocker, error) {
	if s.blockers == nil {
		return nil, fmt.Errorf("%w: blocker store not configured", ErrInvalidState)
	}

	_, reports, err := s.resolveManagerReports(ctx, organizationID, managerUserID)
	if err != nil {
		return nil, err
	}

	reportIDs := make(map[string]struct{}, len(reports))
	for _, report := range reports {
		reportIDs[report.ID] = struct{}{}
	}

	all, err := s.blockers.ListBlockers(ctx, organizationID)
	if err != nil {
		return nil, fmt.Errorf("list blockers: %w", err)
	}

	items := make([]support.Blocker, 0, len(all))

	for _, blocker := range all {
		if _, ok := reportIDs[blocker.EmployeeID]; ok {
			items = append(items, blocker)
		}
	}

	return items, nil
}

// resolveManagerReports resolves the caller's employee record and returns it
// together with the employees who list it as their manager.
func (s *Service) resolveManagerReports(
	ctx context.Context,
	organizationID, managerUserID string,
) (employees.Employee, []employees.Employee, error) {
	manager, err := s.employees.GetByUserID(ctx, organizationID, managerUserID)
	if err != nil {
		if errors.Is(err, employees.ErrNotFound) {
			return employees.Employee{}, []employees.Employee{}, nil
		}
		return employees.Employee{}, nil, fmt.Errorf("resolve employee: %w", err)
	}

	reports := make([]employees.Employee, 0)

	for offset := int64(0); ; offset += employeeListPageSize {
		page, err := s.employees.List(ctx, organizationID, offset, employeeListPageSize)
		if err != nil {
			return employees.Employee{}, nil, fmt.Errorf("list employees: %w", err)
		}

		for _, employee := range page {
			if employee.ManagerEmployeeID == manager.ID {
				reports = append(reports, employee)
			}
		}

		if len(page) < employeeListPageSize {
			break
		}
	}

	return manager, reports, nil
}

// countPendingApprovalsByEmployee counts pending approvals awaiting the
// manager's decision, grouped by the employee who owns the step.
func (s *Service) countPendingApprovalsByEmployee(
	ctx context.Context,
	organizationID, managerUserID string,
) (map[string]int, error) {
	counts := map[string]int{}

	if managerUserID == "" {
		return counts, nil
	}

	approvals, err := s.repo.ListApprovals(ctx, organizationID)
	if err != nil {
		return nil, fmt.Errorf("list approvals: %w", err)
	}

	for _, approval := range approvals {
		if approval.Status != approvalPending || approval.ApproverUserID != managerUserID {
			continue
		}

		step, err := s.repo.GetStep(ctx, organizationID, approval.StepAssignmentID)
		if err != nil {
			// The step is gone; the orphaned approval cannot be attributed.
			continue
		}

		counts[step.EmployeeID]++
	}

	return counts, nil
}

// countOverdueSteps counts a journey's steps whose due date passed without
// the step being completed.
func (s *Service) countOverdueSteps(
	ctx context.Context,
	organizationID, assignmentID string,
	now time.Time,
) (int, error) {
	steps, err := s.repo.ListSteps(ctx, organizationID, assignmentID)
	if err != nil {
		return 0, fmt.Errorf("list assignment steps: %w", err)
	}

	overdue := 0

	for _, step := range steps {
		if step.DueAt != nil && step.DueAt.Before(now) && step.Status != stepCompleted {
			overdue++
		}
	}

	return overdue, nil
}

// notifyBlocker tells the employee's manager about a new blocker. Employees
// without a linked manager (or a manager without portal access) skip the
// notification; the blocker still exists as a support ticket. link deep-links
// to the blocked step's assignment when the blocker references one.
func (s *Service) notifyBlocker(
	ctx context.Context,
	organizationID string,
	employee employees.Employee,
	link string,
) error {
	if s.notify == nil || employee.ManagerEmployeeID == "" {
		return nil
	}

	manager, err := s.employees.Get(ctx, organizationID, employee.ManagerEmployeeID)
	if err != nil || manager.UserID == "" {
		return nil //nolint:nilerr // no reachable manager; nothing to notify
	}

	if _, notifyErr := s.notify.Create(ctx, organizationID, notifications.CreateInput{
		UserID: manager.UserID,
		Type:   notifications.TypeBlocker,
		Title:  "Blocker reported",
		Body:   employeeDisplayName(employee) + " reported a blocker on their onboarding.",
		Link:   link,
	}); notifyErr != nil {
		return fmt.Errorf("notify manager: %w", notifyErr)
	}

	return nil
}

func employeeDisplayName(employee employees.Employee) string {
	return strings.TrimSpace(employee.FirstName + " " + employee.LastName)
}
