// Package jobs runs periodic background sweeps such as due-date notifications.
package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

const (
	// DefaultInterval is the fallback tick interval for NewScheduler.
	DefaultInterval = 15 * time.Minute
	// DefaultSweepTimeout bounds a single sweep run so a stuck sweep cannot
	// block the scheduler loop forever.
	DefaultSweepTimeout = 5 * time.Minute
)

// SweepFunc is one unit of periodic work. Returning an error never stops the
// scheduler; the failure is logged and the next sweep still runs.
type SweepFunc func(ctx context.Context) error

type namedSweep struct {
	name string
	run  SweepFunc
}

// Scheduler runs registered sweeps on a fixed interval.
type Scheduler struct {
	interval     time.Duration
	sweepTimeout time.Duration
	sweeps       []namedSweep
	mu           sync.RWMutex
	statuses     map[string]SweepStatus
}

// SweepStatus is the operator-facing execution state for a registered sweep.
type SweepStatus struct {
	Name            string     `json:"name"`
	Running         bool       `json:"running"`
	LastStartedAt   *time.Time `json:"lastStartedAt,omitempty"`
	LastCompletedAt *time.Time `json:"lastCompletedAt,omitempty"`
	LastSucceededAt *time.Time `json:"lastSucceededAt,omitempty"`
	LastError       string     `json:"lastError,omitempty"`
	RunCount        int64      `json:"runCount"`
	FailureCount    int64      `json:"failureCount"`
}

// NewScheduler constructs a Scheduler. Non-positive values fall back to the
// package defaults.
func NewScheduler(interval, sweepTimeout time.Duration) *Scheduler {
	if interval <= 0 {
		interval = DefaultInterval
	}

	if sweepTimeout <= 0 {
		sweepTimeout = DefaultSweepTimeout
	}

	return &Scheduler{
		interval: interval, sweepTimeout: sweepTimeout, sweeps: nil,
		statuses: make(map[string]SweepStatus),
	}
}

// Register adds a named sweep. It is not safe to call once Start is running.
func (s *Scheduler) Register(name string, sweep SweepFunc) {
	s.sweeps = append(s.sweeps, namedSweep{name: name, run: sweep})
	s.statuses[name] = SweepStatus{Name: name}
}

// Statuses returns a point-in-time copy of all registered sweep states.
func (s *Scheduler) Statuses() []SweepStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]SweepStatus, 0, len(s.sweeps))
	for _, sweep := range s.sweeps {
		items = append(items, s.statuses[sweep.name])
	}
	return items
}

// RunNow executes one registered sweep immediately. Concurrent runs of the
// same sweep are rejected so operators cannot accidentally duplicate work.
func (s *Scheduler) RunNow(ctx context.Context, name string) error {
	for _, sweep := range s.sweeps {
		if sweep.name == name {
			return s.runSweep(ctx, sweep)
		}
	}
	return fmt.Errorf("unknown sweep %q", name)
}

// Start runs one sweep pass immediately and then one pass per interval until
// ctx is cancelled. It blocks, so callers run it in a goroutine; the app stops
// the scheduler by cancelling the process context during shutdown.
func (s *Scheduler) Start(ctx context.Context) {
	slog.InfoContext(ctx, "scheduler started", "interval", s.interval.String(), "sweeps", len(s.sweeps))

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	s.runSweeps(ctx)

	for {
		select {
		case <-ctx.Done():
			slog.InfoContext(ctx, "scheduler stopped")

			return
		case <-ticker.C:
			s.runSweeps(ctx)
		}
	}
}

func (s *Scheduler) runSweeps(ctx context.Context) {
	for _, sweep := range s.sweeps {
		if err := s.runSweep(ctx, sweep); err != nil {
			slog.ErrorContext(ctx, "scheduler sweep failed", "sweep", sweep.name, "error", err)
		}
	}
}

// runSweep executes one sweep under its own timeout. A panic is recovered and
// logged so a single bad sweep can never kill the scheduler loop.
func (s *Scheduler) runSweep(ctx context.Context, sweep namedSweep) (runErr error) {
	now := time.Now().UTC()
	s.mu.Lock()
	status := s.statuses[sweep.name]
	if status.Running {
		s.mu.Unlock()
		return fmt.Errorf("sweep %q is already running", sweep.name)
	}
	status.Running = true
	status.LastStartedAt = &now
	status.RunCount++
	s.statuses[sweep.name] = status
	s.mu.Unlock()

	defer func() {
		if recovered := recover(); recovered != nil {
			runErr = fmt.Errorf("panic: %v", recovered)
			slog.ErrorContext(ctx, "scheduler sweep panicked", "sweep", sweep.name, "panic", recovered)
		}
		completed := time.Now().UTC()
		s.mu.Lock()
		status := s.statuses[sweep.name]
		status.Running = false
		status.LastCompletedAt = &completed
		if runErr != nil {
			status.LastError = runErr.Error()
			status.FailureCount++
		} else {
			status.LastError = ""
			status.LastSucceededAt = &completed
		}
		s.statuses[sweep.name] = status
		s.mu.Unlock()
	}()

	sweepCtx, cancel := context.WithTimeout(ctx, s.sweepTimeout)
	defer cancel()

	if err := sweep.run(sweepCtx); err != nil {
		runErr = fmt.Errorf("run sweep %q: %w", sweep.name, err)
		return runErr
	}

	return nil
}
