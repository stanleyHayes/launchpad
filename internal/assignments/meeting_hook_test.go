package assignments_test

import (
	"context"
	"errors"
	"testing"

	"launchpad/internal/assignments"
)

type meetingCall struct {
	organizationID string
	employeeID     string
	meetingType    string
	title          string
	startsAt       string
	durationMin    int
	location       string
}

// stubMeetingScheduler captures CreateFromStep calls.
type stubMeetingScheduler struct {
	calls []meetingCall
	err   error
}

func (s *stubMeetingScheduler) CreateFromStep(
	_ context.Context,
	organizationID, employeeID, meetingType, title, startsAt string,
	durationMin int,
	location string,
) error {
	if s.err != nil {
		return s.err
	}

	s.calls = append(s.calls, meetingCall{
		organizationID: organizationID,
		employeeID:     employeeID,
		meetingType:    meetingType,
		title:          title,
		startsAt:       startsAt,
		durationMin:    durationMin,
		location:       location,
	})

	return nil
}

func TestCompleteMeetingStepSchedulesMeeting(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	seedStep(repo, testStatusInProgress, "meeting")

	scheduler := &stubMeetingScheduler{}
	svc := newStepSvc(repo, nil)
	svc.SetMeetingScheduler(scheduler)

	step, err := svc.CompleteStep(
		context.Background(),
		testOrgID,
		testEmployeeUser,
		testStepID,
		assignments.CompleteStepInput{Submission: map[string]any{
			"meetingType": "manager_intro",
			"startsAt":    "2030-01-15T10:00:00Z",
			"durationMin": float64(45),
			"location":    "https://meet.example.com/abc",
		}},
	)
	if err != nil {
		t.Fatalf("CompleteStep: %v", err)
	}

	if step.Status != "completed" {
		t.Fatalf("status = %q, want completed", step.Status)
	}

	if len(scheduler.calls) != 1 {
		t.Fatalf("calls = %+v, want one meeting", scheduler.calls)
	}

	call := scheduler.calls[0]
	if call.organizationID != testOrgID || call.employeeID != testEmployeeID {
		t.Errorf("scope = %q/%q, want %q/%q", call.organizationID, call.employeeID, testOrgID, testEmployeeID)
	}

	if call.title != "Read the handbook" {
		t.Errorf("title = %q, want the step title", call.title)
	}

	if call.startsAt != "2030-01-15T10:00:00Z" || call.durationMin != 45 {
		t.Errorf("schedule = %q/%d", call.startsAt, call.durationMin)
	}

	if call.location != "https://meet.example.com/abc" || call.meetingType != "manager_intro" {
		t.Errorf("details = %q/%q", call.location, call.meetingType)
	}
}

func TestCompleteMeetingStepFailsWhenSchedulingFails(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	seedStep(repo, testStatusInProgress, "meeting")

	scheduler := &stubMeetingScheduler{err: errors.New("invalid meeting input")}
	svc := newStepSvc(repo, nil)
	svc.SetMeetingScheduler(scheduler)

	_, err := svc.CompleteStep(
		context.Background(),
		testOrgID,
		testEmployeeUser,
		testStepID,
		assignments.CompleteStepInput{Submission: map[string]any{}},
	)
	if err == nil {
		t.Fatal("expected CompleteStep to fail when the meeting cannot be scheduled")
	}

	if repo.steps[testStepID].Status != testStatusInProgress {
		t.Fatalf("step status persisted as %q, want still in_progress", repo.steps[testStepID].Status)
	}
}

func TestCompleteMeetingStepWithoutSchedulerStaysInProgress(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	seedStep(repo, testStatusInProgress, "meeting")
	svc := newStepSvc(repo, nil)

	_, err := svc.CompleteStep(
		context.Background(),
		testOrgID,
		testEmployeeUser,
		testStepID,
		assignments.CompleteStepInput{},
	)
	if err == nil {
		t.Fatal("expected CompleteStep to fail without a meeting scheduler")
	}

	if repo.steps[testStepID].Status != testStatusInProgress {
		t.Fatalf("step status persisted as %q, want still in_progress", repo.steps[testStepID].Status)
	}
}

func TestCompleteTaskStepDoesNotScheduleMeeting(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	seedStep(repo, testStatusInProgress, "task")

	scheduler := &stubMeetingScheduler{}
	svc := newStepSvc(repo, nil)
	svc.SetMeetingScheduler(scheduler)

	if _, err := svc.CompleteStep(
		context.Background(),
		testOrgID,
		testEmployeeUser,
		testStepID,
		assignments.CompleteStepInput{},
	); err != nil {
		t.Fatalf("CompleteStep: %v", err)
	}

	if len(scheduler.calls) != 0 {
		t.Fatalf("calls = %+v, want none for a task step", scheduler.calls)
	}
}
