package drift

import (
	"context"
	"log"
	"time"
)

// Scheduler drives periodic reconciliation sweeps. Mirrors expiry.Scheduler:
// immediate sweep on boot + tick on interval + graceful Done() for shutdown.
// Single-instance assumption (single-LXC); no leader election.
type Scheduler struct {
	interval time.Duration
	done     chan struct{}
}

func NewScheduler(interval time.Duration) *Scheduler {
	if interval <= 0 {
		interval = 6 * time.Hour
	}
	return &Scheduler{interval: interval, done: make(chan struct{})}
}

func (s *Scheduler) Done() <-chan struct{} { return s.done }

func (s *Scheduler) Start(ctx context.Context) {
	log.Printf("[DRIFT] Starting reconciliation scheduler: interval=%s", s.interval)
	defer close(s.done)

	s.runOnce(ctx)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Println("[DRIFT] Stopping on context cancellation")
			return
		case <-ticker.C:
			s.runOnce(ctx)
		}
	}
}

func (s *Scheduler) runOnce(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[DRIFT] Panic recovered during sweep: %v", r)
		}
	}()
	if _, err := Sweep(ctx); err != nil {
		log.Printf("[DRIFT] Sweep error: %v", err)
	}
}
