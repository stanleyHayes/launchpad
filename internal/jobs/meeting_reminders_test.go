package jobs_test

import (
	"context"
	"testing"
	"time"

	"launchpad/internal/employees"
	"launchpad/internal/jobs"
	"launchpad/internal/meetings"
)

type fakeMeetingReminderStore struct {
	items map[string]meetings.Meeting
}

func (f *fakeMeetingReminderStore) ListUpcomingUnreminded(
	_ context.Context,
	from, to time.Time,
) ([]meetings.Meeting, error) {
	items := make([]meetings.Meeting, 0)
	for _, meeting := range f.items {
		if meeting.ReminderNotifiedAt == nil && meeting.Status == "scheduled" &&
			!meeting.StartsAt.Before(from) && !meeting.StartsAt.After(to) {
			items = append(items, meeting)
		}
	}
	return items, nil
}

func (f *fakeMeetingReminderStore) Update(_ context.Context, meeting meetings.Meeting) error {
	f.items[meeting.ID] = meeting
	return nil
}

func TestMeetingReminderNotifiesAttendeeAndOrganizerOnce(t *testing.T) {
	t.Parallel()

	meeting := meetings.Meeting{
		ID: "meeting-1", OrganizationID: "org-1", AttendeeEmployeeID: "emp-1",
		OrganizerUserID: "manager-1", Title: "First-week review",
		StartsAt: time.Now().UTC().Add(2 * time.Hour), Status: "scheduled",
	}
	store := &fakeMeetingReminderStore{items: map[string]meetings.Meeting{meeting.ID: meeting}}
	notifier := &fakeNotifier{}
	reader := fakeEmployeeReader{byID: map[string]employees.Employee{
		"emp-1": {ID: "emp-1", UserID: "employee-1"},
	}}
	sweep := jobs.NewMeetingReminderSweep(
		store, reader, notifier, jobs.DefaultMeetingReminderHorizon,
	)
	if err := sweep(context.Background()); err != nil {
		t.Fatalf("first sweep: %v", err)
	}
	if err := sweep(context.Background()); err != nil {
		t.Fatalf("second sweep: %v", err)
	}
	if len(notifier.created) != 2 {
		t.Fatalf("notifications = %d, want attendee + organizer exactly once", len(notifier.created))
	}
	if store.items[meeting.ID].ReminderNotifiedAt == nil {
		t.Fatal("meeting reminder marker was not persisted")
	}
}
