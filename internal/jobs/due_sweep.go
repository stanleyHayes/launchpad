package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"launchpad/internal/assignments"
	"launchpad/internal/employees"
	"launchpad/internal/notifications"
)

// DefaultDueSoonHorizon is how far ahead of a step's due date the due-soon
// notification is sent.
const DefaultDueSoonHorizon = 24 * time.Hour

// StepAssignmentStore queries and updates step assignments for the due sweep.
type StepAssignmentStore interface {
	ListDueSoonSteps(ctx context.Context, from, to time.Time) ([]assignments.StepAssignment, error)
	ListOverdueSteps(ctx context.Context, now time.Time) ([]assignments.StepAssignment, error)
	UpdateStep(ctx context.Context, step assignments.StepAssignment) error
}

// EmployeeReader resolves step owners to their user accounts.
type EmployeeReader interface {
	Get(ctx context.Context, organizationID, employeeID string) (employees.Employee, error)
}

// Notifier creates in-app notifications.
type Notifier interface {
	Create(ctx context.Context, organizationID string, in notifications.CreateInput) (notifications.Notification, error)
}

// NewDueNotificationSweep builds a sweep that notifies step owners once when a
// step is due within the horizon and once when it passes its due date. Dedupe
// lives on the step assignment itself (DueSoonNotifiedAt/OverdueNotifiedAt),
// which is simpler than a separate dedupe collection and travels with the
// document the sweep already updates. Step statuses are deliberately left
// unchanged: overdue is conveyed by the notification, not a status transition.
func NewDueNotificationSweep(
	steps StepAssignmentStore,
	employeeReader EmployeeReader,
	notifier Notifier,
	horizon time.Duration,
) SweepFunc {
	return func(ctx context.Context) error {
		now := time.Now().UTC()

		dueSoon, err := steps.ListDueSoonSteps(ctx, now, now.Add(horizon))
		if err != nil {
			return fmt.Errorf("list due-soon steps: %w", err)
		}

		notifySteps(ctx, steps, employeeReader, notifier, dueSoon, notifications.TypeDueSoon, "Step due soon",
			func(step assignments.StepAssignment) string {
				return "\"" + step.Title + "\" is due " + formatDue(step.DueAt) + "."
			},
			func(step *assignments.StepAssignment, at time.Time) { step.DueSoonNotifiedAt = &at },
			now,
		)

		overdue, err := steps.ListOverdueSteps(ctx, now)
		if err != nil {
			return fmt.Errorf("list overdue steps: %w", err)
		}

		notifySteps(ctx, steps, employeeReader, notifier, overdue, notifications.TypeOverdue, "Step overdue",
			func(step assignments.StepAssignment) string {
				return "\"" + step.Title + "\" was due " + formatDue(step.DueAt) + " and is still open."
			},
			func(step *assignments.StepAssignment, at time.Time) { step.OverdueNotifiedAt = &at },
			now,
		)

		return nil
	}
}

// notifySteps notifies each step's owner and marks the step so later ticks
// skip it. Per-step failures are logged and skipped so one bad record cannot
// block the rest of the sweep; a step whose notification failed stays
// unmarked and is retried on the next tick.
func notifySteps(
	ctx context.Context,
	steps StepAssignmentStore,
	employeeReader EmployeeReader,
	notifier Notifier,
	due []assignments.StepAssignment,
	notificationType string,
	title string,
	body func(assignments.StepAssignment) string,
	mark func(*assignments.StepAssignment, time.Time),
	now time.Time,
) {
	for _, step := range due {
		employee, err := employeeReader.Get(ctx, step.OrganizationID, step.EmployeeID)
		if err != nil {
			slog.WarnContext(ctx, "due sweep: load step owner", "stepId", step.ID, "error", err)

			continue
		}

		if employee.UserID == "" {
			continue
		}

		if _, err := notifier.Create(ctx, step.OrganizationID, notifications.CreateInput{
			UserID: employee.UserID,
			Type:   notificationType,
			Title:  title,
			Body:   body(step),
			Link:   "/assignments/" + step.JourneyAssignmentID,
		}); err != nil {
			slog.WarnContext(ctx, "due sweep: notify step owner", "stepId", step.ID, "error", err)

			continue
		}

		mark(&step, now)

		if err := steps.UpdateStep(ctx, step); err != nil {
			slog.WarnContext(ctx, "due sweep: mark step notified", "stepId", step.ID, "error", err)
		}
	}
}

func formatDue(dueAt *time.Time) string {
	if dueAt == nil {
		return "unknown"
	}

	return dueAt.UTC().Format(time.DateOnly)
}
