package expiry

import (
	"context"
	"log"
	"time"
)

// Scheduler drives periodic sweeps of expired direct grants. It is the first
// backend-side background worker; future periodic jobs (access-request
// expiry, token-rotation reminders) should follow the same shape.
//
// Single-instance assumption: if the backend is ever deployed with N > 1
// replicas, only one replica should run the scheduler. Operators can gate
// this via EXPIRY_SCHEDULER_ENABLED=false on extra replicas. A future
// enhancement could add PG advisory-lock leader election.
type Scheduler struct {
	interval  time.Duration
	batchSize int
	done      chan struct{}
}

// NewScheduler constructs a Scheduler with a clamped batch size. The caller
// is expected to pass a non-zero interval; zero-or-negative intervals are
// replaced with a 5-minute default as a safety net.
func NewScheduler(interval time.Duration, batchSize int) *Scheduler {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	if batchSize < 1 {
		batchSize = 1
	}
	if batchSize > 10000 {
		batchSize = 10000
	}
	return &Scheduler{
		interval:  interval,
		batchSize: batchSize,
		done:      make(chan struct{}),
	}
}

// Done returns a channel that closes after Start has observed ctx
// cancellation and fully returned. Main's shutdown sequence waits on this
// (bounded by the shutdown timeout) before closing shared DB/Redis clients,
// so an in-flight sweep cannot race with connection teardown.
func (s *Scheduler) Done() <-chan struct{} {
	return s.done
}

// Start runs the scheduler loop until ctx is cancelled. It performs one
// immediate sweep on boot (so restarts don't leave expired grants lingering
// for up to one full interval) and then ticks at interval. Callers launch
// this as a goroutine; it returns (and closes the Done channel) when
// ctx.Done fires and the current sweep, if any, has finished.
func (s *Scheduler) Start(ctx context.Context) {
	log.Printf("[SCHEDULER] Starting grant-expiry scheduler: interval=%s batch=%d",
		s.interval, s.batchSize)
	defer close(s.done)

	s.runOnce(ctx)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("[SCHEDULER] Stopping on context cancellation")
			return
		case <-ticker.C:
			s.runOnce(ctx)
		}
	}
}

// runOnce executes a single sweep with panic recovery. A panic in sweep must
// not kill the goroutine: the scheduler is long-running and an isolated bug
// should surface in logs, not silently stall the whole enforcement surface.
func (s *Scheduler) runOnce(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[SCHEDULER] Panic recovered during sweep: %v", r)
		}
	}()

	start := timeNow()
	sweep(ctx, s.batchSize)
	log.Printf("[SCHEDULER] Sweep complete duration=%s", timeNow().Sub(start))
}
