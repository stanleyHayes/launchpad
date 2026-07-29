package analytics

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"launchpad/internal/assignments"
	"launchpad/internal/assistant"
	"launchpad/internal/employees"
)

const percentScale = 100

// Service computes analytics summaries from domain sources.
type Service struct {
	assignments AssignmentSource
	employees   EmployeeSource
	directory   DirectorySource
	assistant   AssistantSource
}

// NewService constructs a Service.
func NewService(assignmentSource AssignmentSource, employeeSource EmployeeSource) *Service {
	return &Service{
		assignments: assignmentSource,
		employees:   employeeSource,
		directory:   nil,
		assistant:   nil,
	}
}

// WithSources attaches the directory and assistant sources used by the
// breakdown and assistant reports. It returns the service for chaining.
func (s *Service) WithSources(directory DirectorySource, assistantSource AssistantSource) *Service {
	s.directory = directory
	s.assistant = assistantSource

	return s
}

// OnboardingSummary returns an organization onboarding snapshot.
func (s *Service) OnboardingSummary(ctx context.Context, organizationID string) (OnboardingSummary, error) {
	if organizationID == "" {
		return OnboardingSummary{}, ErrInvalidInput
	}

	employeeCount, err := s.employees.Count(ctx, organizationID)
	if err != nil {
		return OnboardingSummary{}, fmt.Errorf("count employees for analytics: %w", err)
	}

	assignmentItems, err := s.assignments.List(ctx, organizationID)
	if err != nil {
		return OnboardingSummary{}, fmt.Errorf("list assignments for analytics: %w", err)
	}

	approvalItems, err := s.assignments.ListApprovals(ctx, organizationID)
	if err != nil {
		return OnboardingSummary{}, fmt.Errorf("list approvals for analytics: %w", err)
	}

	summary := summarizeAssignments(assignmentItems)
	summary.EmployeeCount = int(employeeCount)
	summary.PendingApprovalCount = countPendingApprovals(approvalItems)

	if err := s.addStepStats(ctx, organizationID, assignmentItems, &summary); err != nil {
		return OnboardingSummary{}, err
	}

	summary.GeneratedAt = time.Now().UTC()

	return summary, nil
}

// OnboardingBreakdown computes completion rates grouped by department or job role.
func (s *Service) OnboardingBreakdown(
	ctx context.Context,
	organizationID, by string,
) (OnboardingBreakdown, error) {
	if organizationID == "" || (by != BreakdownByDepartment && by != BreakdownByJobRole) {
		return OnboardingBreakdown{}, ErrInvalidInput
	}

	if s.directory == nil {
		return OnboardingBreakdown{}, fmt.Errorf("onboarding breakdown: %w", ErrSourceNotConfigured)
	}

	groupNames, err := s.groupNames(ctx, organizationID, by)
	if err != nil {
		return OnboardingBreakdown{}, err
	}

	employeeItems, err := s.listAllEmployees(ctx, organizationID)
	if err != nil {
		return OnboardingBreakdown{}, err
	}

	assignmentItems, err := s.assignments.List(ctx, organizationID)
	if err != nil {
		return OnboardingBreakdown{}, fmt.Errorf("list assignments for breakdown: %w", err)
	}

	rows := buildBreakdownRows(employeeItems, assignmentItems, groupNames, by)

	return OnboardingBreakdown{By: by, Rows: rows, GeneratedAt: time.Now().UTC()}, nil
}

// AssistantReport aggregates assistant interactions for an organization.
func (s *Service) AssistantReport(ctx context.Context, organizationID string) (AssistantReport, error) {
	if organizationID == "" {
		return AssistantReport{}, ErrInvalidInput
	}

	if s.assistant == nil {
		return AssistantReport{}, fmt.Errorf("assistant report: %w", ErrSourceNotConfigured)
	}

	interactions, err := s.assistant.ListInteractions(ctx, organizationID)
	if err != nil {
		return AssistantReport{}, fmt.Errorf("list assistant interactions for report: %w", err)
	}

	report := summarizeInteractions(interactions)
	report.GeneratedAt = time.Now().UTC()

	return report, nil
}

func (s *Service) FunnelReport(ctx context.Context, organizationID string) (FunnelReport, error) {
	if organizationID == "" {
		return FunnelReport{}, ErrInvalidInput
	}
	assignmentItems, err := s.assignments.List(ctx, organizationID)
	if err != nil {
		return FunnelReport{}, fmt.Errorf("list assignments for funnel: %w", err)
	}
	type accumulator struct {
		title                             string
		position, reached, total, dropped int
	}
	byPosition := map[int]*accumulator{}
	for _, assignment := range assignmentItems {
		steps, listErr := s.assignments.ListSteps(ctx, organizationID, assignment.ID)
		if listErr != nil {
			return FunnelReport{}, fmt.Errorf("list steps for funnel: %w", listErr)
		}
		sort.Slice(steps, func(i, j int) bool { return steps[i].Position < steps[j].Position })
		dropRecorded := false
		for _, step := range steps {
			row := byPosition[step.Position]
			if row == nil {
				row = &accumulator{title: step.Title, position: step.Position}
				byPosition[step.Position] = row
			}
			row.total++
			if step.Status == statusCompleted {
				row.reached++
			} else if !dropRecorded {
				row.dropped++
				dropRecorded = true
			}
		}
	}
	positions := make([]int, 0, len(byPosition))
	for position := range byPosition {
		positions = append(positions, position)
	}
	sort.Ints(positions)
	report := FunnelReport{
		Milestones:  make([]MilestoneStat, 0, len(positions)),
		DropOff:     make([]DropOffStat, 0, len(positions)),
		GeneratedAt: time.Now().UTC(),
	}
	for _, position := range positions {
		row := byPosition[position]
		rate := 0.0
		if row.total > 0 {
			rate = round2(float64(row.reached) / float64(row.total))
		}
		report.Milestones = append(report.Milestones, MilestoneStat{
			StepTitle: row.title, Position: row.position, Reached: row.reached, Total: row.total, Rate: rate,
		})
		if row.dropped > 0 {
			report.DropOff = append(report.DropOff, DropOffStat{
				StepTitle: row.title, Position: row.position, Count: row.dropped,
			})
		}
	}
	sort.Slice(report.DropOff, func(i, j int) bool { return report.DropOff[i].Count > report.DropOff[j].Count })
	return report, nil
}

// addStepStats computes incomplete/overdue step counts from step due dates.
func (s *Service) addStepStats(
	ctx context.Context,
	organizationID string,
	assignmentItems []assignments.JourneyAssignment,
	summary *OnboardingSummary,
) error {
	now := time.Now().UTC()

	for _, assignment := range assignmentItems {
		steps, err := s.assignments.ListSteps(ctx, organizationID, assignment.ID)
		if err != nil {
			return fmt.Errorf("list steps for analytics: %w", err)
		}

		for _, step := range steps {
			if step.Status == statusCompleted {
				continue
			}

			summary.IncompleteStepCount++

			if step.DueAt != nil && step.DueAt.Before(now) {
				summary.OverdueStepCount++
			}
		}
	}

	if summary.IncompleteStepCount > 0 {
		summary.OverdueRate = round2(float64(summary.OverdueStepCount) / float64(summary.IncompleteStepCount))
	}

	return nil
}

// groupNames maps department or job-role IDs to display names.
func (s *Service) groupNames(ctx context.Context, organizationID, by string) (map[string]string, error) {
	names := map[string]string{}

	if by == BreakdownByJobRole {
		roles, err := s.directory.ListJobRoles(ctx, organizationID)
		if err != nil {
			return nil, fmt.Errorf("list job roles for breakdown: %w", err)
		}

		for _, role := range roles {
			names[role.ID] = role.Name
		}

		return names, nil
	}

	departmentItems, err := s.directory.ListDepartments(ctx, organizationID)
	if err != nil {
		return nil, fmt.Errorf("list departments for breakdown: %w", err)
	}

	for _, department := range departmentItems {
		names[department.ID] = department.Name
	}

	return names, nil
}

// listAllEmployees pages through the full employee roster.
func (s *Service) listAllEmployees(ctx context.Context, organizationID string) ([]employees.Employee, error) {
	items := make([]employees.Employee, 0)

	for offset := int64(0); ; offset += employeePageSize {
		page, err := s.employees.List(ctx, organizationID, offset, employeePageSize)
		if err != nil {
			return nil, fmt.Errorf("list employees for breakdown: %w", err)
		}

		items = append(items, page...)

		if len(page) < employeePageSize {
			return items, nil
		}
	}
}

// employeeGroupID picks the grouping key for an employee.
func employeeGroupID(employee employees.Employee, by string) string {
	if by == BreakdownByJobRole {
		return employee.JobRoleID
	}

	return employee.DepartmentID
}

// buildBreakdownRows aggregates employees and assignments into group rows.
func buildBreakdownRows(
	employeeItems []employees.Employee,
	assignmentItems []assignments.JourneyAssignment,
	groupNames map[string]string,
	by string,
) []BreakdownRow {
	employeeGroup := make(map[string]string, len(employeeItems))
	totals := map[string]*BreakdownRow{}

	for _, employee := range employeeItems {
		groupID := employeeGroupID(employee, by)
		employeeGroup[employee.ID] = groupID

		breakdownRow(totals, groupNames, groupID).EmployeeCount++
	}

	for _, assignment := range assignmentItems {
		groupID, tracked := employeeGroup[assignment.EmployeeID]
		if !tracked {
			continue
		}

		row := breakdownRow(totals, groupNames, groupID)
		row.AssignmentCount++

		if assignment.Status == statusCompleted {
			row.CompletedAssignmentCount++
		}
	}

	rows := make([]BreakdownRow, 0, len(totals))
	for _, row := range totals {
		if row.AssignmentCount > 0 {
			row.CompletionRate = round2(float64(row.CompletedAssignmentCount) / float64(row.AssignmentCount))
		}

		rows = append(rows, *row)
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })

	return rows
}

// breakdownRow returns the accumulator for a group, creating it on first use.
func breakdownRow(totals map[string]*BreakdownRow, groupNames map[string]string, groupID string) *BreakdownRow {
	row, ok := totals[groupID]
	if ok {
		return row
	}

	name, known := groupNames[groupID]
	if !known {
		name = unassignedGroupName
	}

	row = &BreakdownRow{
		ID:                       groupID,
		Name:                     name,
		EmployeeCount:            0,
		AssignmentCount:          0,
		CompletedAssignmentCount: 0,
		CompletionRate:           0,
	}
	totals[groupID] = row

	return row
}

func summarizeInteractions(interactions []assistant.Interaction) AssistantReport {
	var report AssistantReport

	refusedFrequency := map[string]int{}

	for _, interaction := range interactions {
		report.TotalQuestions++

		if interaction.Refused {
			report.RefusalCount++

			countRefusedQuestion(refusedFrequency, interaction.Question)
		}

		if interaction.Helpful != nil {
			report.FeedbackCount++

			if *interaction.Helpful {
				report.HelpfulCount++
			}
		}
	}

	if report.TotalQuestions > 0 {
		report.RefusalRate = round2(float64(report.RefusalCount) / float64(report.TotalQuestions))
	}

	if report.FeedbackCount > 0 {
		report.HelpfulRate = round2(float64(report.HelpfulCount) / float64(report.FeedbackCount))
	}

	report.TopRefusedQuestions = topRefusedQuestions(refusedFrequency)

	return report
}

func countRefusedQuestion(frequency map[string]int, question string) {
	normalized := strings.TrimSpace(question)
	if normalized == "" {
		return
	}

	frequency[normalized]++
}

// topRefusedQuestions ranks refused questions by frequency, capped at the
// report limit, with ties broken alphabetically for determinism.
func topRefusedQuestions(frequency map[string]int) []AssistantQuestionStat {
	stats := make([]AssistantQuestionStat, 0, len(frequency))
	for question, count := range frequency {
		stats = append(stats, AssistantQuestionStat{Question: question, Count: count})
	}

	sort.Slice(stats, func(i, j int) bool {
		if stats[i].Count != stats[j].Count {
			return stats[i].Count > stats[j].Count
		}

		return stats[i].Question < stats[j].Question
	})

	if len(stats) > topQuestionLimit {
		stats = stats[:topQuestionLimit]
	}

	return stats
}

func summarizeAssignments(items []assignments.JourneyAssignment) OnboardingSummary {
	summary := OnboardingSummary{}
	completedDurationsDays := make([]float64, 0)

	for _, assignment := range items {
		switch assignment.Status {
		case statusScheduled:
			summary.ScheduledAssignmentCount++
		case statusInProgress:
			summary.ActiveAssignmentCount++
		case statusCompleted:
			summary.CompletedAssignmentCount++
			completedDurationsDays = appendCompletionDays(completedDurationsDays, assignment)
		}
	}

	totalTracked := summary.ActiveAssignmentCount +
		summary.CompletedAssignmentCount +
		summary.ScheduledAssignmentCount
	if totalTracked > 0 {
		summary.CompletionRate = round2(
			float64(summary.CompletedAssignmentCount) / float64(totalTracked),
		)
	}

	summary.AverageDaysToComplete = average(completedDurationsDays)

	return summary
}

func appendCompletionDays(
	days []float64,
	assignment assignments.JourneyAssignment,
) []float64 {
	if assignment.CompletedAt == nil || assignment.StartsAt.IsZero() {
		return days
	}

	durationDays := assignment.CompletedAt.Sub(assignment.StartsAt).Hours() / hoursPerDay
	if durationDays < 0 {
		return days
	}

	return append(days, durationDays)
}

func countPendingApprovals(items []assignments.Approval) int {
	count := 0

	for _, approval := range items {
		if approval.Status == approvalPending {
			count++
		}
	}

	return count
}

func average(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	var total float64
	for _, value := range values {
		total += value
	}

	return round2(total / float64(len(values)))
}

func round2(value float64) float64 {
	return math.Round(value*percentScale) / percentScale
}
