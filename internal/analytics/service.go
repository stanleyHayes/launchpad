package analytics

import (
	"context"
	"fmt"
	"math"
	"time"

	"launchpad/internal/assignments"
)

const percentScale = 100

// Service computes analytics summaries from domain sources.
type Service struct {
	assignments AssignmentSource
	employees   EmployeeSource
}

// NewService constructs a Service.
func NewService(assignmentSource AssignmentSource, employeeSource EmployeeSource) *Service {
	return &Service{
		assignments: assignmentSource,
		employees:   employeeSource,
	}
}

// OnboardingSummary returns an organization onboarding snapshot.
func (s *Service) OnboardingSummary(ctx context.Context, organizationID string) (OnboardingSummary, error) {
	if organizationID == "" {
		return OnboardingSummary{}, ErrInvalidInput
	}

	employeeItems, err := s.employees.List(ctx, organizationID, analyticsListCap)
	if err != nil {
		return OnboardingSummary{}, fmt.Errorf("list employees for analytics: %w", err)
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
	summary.EmployeeCount = len(employeeItems)
	summary.PendingApprovalCount = countPendingApprovals(approvalItems)
	summary.GeneratedAt = time.Now().UTC()

	return summary, nil
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
