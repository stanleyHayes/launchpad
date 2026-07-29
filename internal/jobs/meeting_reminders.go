package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"launchpad/internal/meetings"
	"launchpad/internal/notifications"
)

const DefaultMeetingReminderHorizon = 24 * time.Hour

type MeetingReminderStore interface {
	ListUpcomingUnreminded(ctx context.Context, from, to time.Time) ([]meetings.Meeting, error)
	Update(ctx context.Context, meeting meetings.Meeting) error
}

// NewMeetingReminderSweep notifies the attendee and organizer once during the
// configured pre-meeting window. Rescheduling clears the marker so the new
// time receives a fresh reminder.
func NewMeetingReminderSweep(
	store MeetingReminderStore,
	employees EmployeeReader,
	notifier Notifier,
	horizon time.Duration,
) SweepFunc {
	return func(ctx context.Context) error {
		now := time.Now().UTC()
		items, err := store.ListUpcomingUnreminded(ctx, now, now.Add(horizon))
		if err != nil {
			return fmt.Errorf("list meetings requiring reminders: %w", err)
		}
		for _, meeting := range items {
			attendee, err := employees.Get(ctx, meeting.OrganizationID, meeting.AttendeeEmployeeID)
			if err != nil {
				slog.WarnContext(ctx, "meeting reminder: load attendee", "meetingId", meeting.ID, "error", err)
				continue
			}
			recipients := []string{attendee.UserID}
			if meeting.OrganizerUserID != "" && meeting.OrganizerUserID != attendee.UserID {
				recipients = append(recipients, meeting.OrganizerUserID)
			}
			delivered := false
			for _, userID := range recipients {
				if userID == "" {
					continue
				}
				_, notifyErr := notifier.Create(ctx, meeting.OrganizationID, notifications.CreateInput{
					UserID: userID, Type: notifications.TypeMeetingReminder,
					Title: "Meeting coming up",
					Body:  "\"" + meeting.Title + "\" starts " + meeting.StartsAt.UTC().Format(time.RFC1123) + ".",
					Link:  "/meetings",
				})
				if notifyErr != nil {
					slog.WarnContext(ctx, "meeting reminder: notify participant", "meetingId", meeting.ID, "error", notifyErr)
					continue
				}
				delivered = true
			}
			if !delivered {
				continue
			}
			meeting.ReminderNotifiedAt = &now
			meeting.UpdatedAt = now
			if err := store.Update(ctx, meeting); err != nil {
				slog.WarnContext(ctx, "meeting reminder: mark delivered", "meetingId", meeting.ID, "error", err)
			}
		}

		return nil
	}
}
