// Package periodic is the single background-worker shape shared by the
// backend schedulers (grant expiry, drift reconciliation): one immediate run
// on boot (so restarts don't leave work lingering for up to a full interval),
// then a ticker loop, panic recovery per run, and a Done channel that closes
// only after any in-flight run finishes — main's shutdown sequence joins on
// Done before closing shared DB/Redis clients, so a run cannot race teardown.
//
// Single-instance assumption: if the backend is ever deployed with N > 1
// replicas, only one replica should run each scheduler (operators gate via
// the *_SCHEDULER_ENABLED env vars). A future enhancement could add PG
// advisory-lock leader election.
package periodic

import (
	"context"
	"log"
	"time"
)

// Runner drives a run function on a fixed interval.
type Runner struct {
	name     string // log tag, e.g. "SCHEDULER", "DRIFT"
	interval time.Duration
	run      func(context.Context) error
	done     chan struct{}
}

// New constructs a Runner. Non-positive intervals fall back to fallback as a
// safety net — env parsing in main already validates, this guards direct
// callers. A run error is logged under the name tag; it never stops the loop.
func New(name string, interval, fallback time.Duration, run func(context.Context) error) *Runner {
	if interval <= 0 {
		interval = fallback
	}
	return &Runner{name: name, interval: interval, run: run, done: make(chan struct{})}
}

// Done returns a channel that closes after Start has observed ctx
// cancellation and fully returned, including any in-flight run.
func (r *Runner) Done() <-chan struct{} { return r.done }

// Start runs the loop until ctx is cancelled: one immediate run, then one per
// tick. Callers launch this as a goroutine; it returns (and closes Done) when
// ctx.Done fires and the current run, if any, has finished.
func (r *Runner) Start(ctx context.Context) {
	log.Printf("[%s] Starting: interval=%s", r.name, r.interval)
	defer close(r.done)

	r.runOnce(ctx)

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Printf("[%s] Stopping on context cancellation", r.name)
			return
		case <-ticker.C:
			r.runOnce(ctx)
		}
	}
}

// runOnce executes a single run with panic recovery. A panic must not kill
// the goroutine: the runner is long-running and an isolated bug should
// surface in logs, not silently stall the whole enforcement surface.
func (r *Runner) runOnce(ctx context.Context) {
	defer func() {
		if rec := recover(); rec != nil {
			log.Printf("[%s] Panic recovered during run: %v", r.name, rec)
		}
	}()

	start := time.Now()
	if err := r.run(ctx); err != nil {
		log.Printf("[%s] Run error: %v", r.name, err)
	}
	log.Printf("[%s] Run complete duration=%s", r.name, time.Since(start))
}
