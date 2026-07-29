package jobs_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"launchpad/internal/jobs"
)

// A failing or panicking sweep must neither block the other sweeps in the
// same tick nor stop the scheduler from ticking again.
func TestSchedulerContinuesAfterSweepFailure(t *testing.T) {
	t.Parallel()

	ran := make(chan string, 16)
	scheduler := jobs.NewScheduler(5*time.Millisecond, time.Second)
	scheduler.Register("failing", func(context.Context) error {
		ran <- "failing"

		return errors.New("boom")
	})
	scheduler.Register("panicking", func(context.Context) error {
		ran <- "panicking"

		panic("boom")
	})
	scheduler.Register("succeeding", func(context.Context) error {
		ran <- "succeeding"

		return nil
	})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	go scheduler.Start(ctx)

	seen := map[string]int{}
	deadline := time.After(2 * time.Second)

	for seen["failing"] < 2 || seen["panicking"] < 2 || seen["succeeding"] < 2 {
		select {
		case name := <-ran:
			seen[name]++
		case <-deadline:
			t.Fatalf("scheduler stalled; sweep runs: %v", seen)
		}
	}
}

func TestNewSchedulerFallsBackToDefaults(t *testing.T) {
	t.Parallel()

	scheduler := jobs.NewScheduler(0, 0)
	if scheduler == nil {
		t.Fatal("expected scheduler")
	}
}

func TestSchedulerReportsRunsAndFailures(t *testing.T) {
	t.Parallel()

	scheduler := jobs.NewScheduler(time.Hour, time.Second)
	scheduler.Register("healthy", func(context.Context) error { return nil })
	scheduler.Register("broken", func(context.Context) error { return errors.New("delivery unavailable") })

	if err := scheduler.RunNow(t.Context(), "healthy"); err != nil {
		t.Fatalf("run healthy sweep: %v", err)
	}
	if err := scheduler.RunNow(t.Context(), "broken"); err == nil {
		t.Fatal("expected broken sweep error")
	}

	statuses := scheduler.Statuses()
	if len(statuses) != 2 {
		t.Fatalf("got %d statuses, want 2", len(statuses))
	}
	if statuses[0].RunCount != 1 || statuses[0].LastSucceededAt == nil || statuses[0].LastError != "" {
		t.Fatalf("unexpected healthy status: %+v", statuses[0])
	}
	if statuses[1].RunCount != 1 || statuses[1].FailureCount != 1 || statuses[1].LastError == "" {
		t.Fatalf("unexpected broken status: %+v", statuses[1])
	}
}

func TestSchedulerRejectsUnknownSweep(t *testing.T) {
	t.Parallel()
	scheduler := jobs.NewScheduler(time.Hour, time.Second)
	if err := scheduler.RunNow(t.Context(), "missing"); err == nil {
		t.Fatal("expected unknown sweep error")
	}
}
