package expiry

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"mkauth/internal/models"
)

// installMinimalNoop wires every injectable to a harmless no-op so scheduler
// lifecycle tests don't exercise sweep internals.
func installMinimalNoop(t *testing.T) *int32 {
	t.Helper()
	var sweepsRun int32
	svcGetExpiredDirectGrants = func(_ context.Context, _ int) ([]models.DirectGrant, error) {
		atomic.AddInt32(&sweepsRun, 1)
		return nil, nil
	}
	svcDeleteExpiredDirectGrantsByIDs = func(_ context.Context, _ string, _ []string) ([]models.DirectGrant, error) {
		return nil, nil
	}
	svcEmitIntentFromScheduler = func(_ context.Context, _, _, _, _, _ string) error { return nil }
	svcInsertAuditLog = func(_ context.Context, _, _, _, _ string) error { return nil }
	cacheInvalidateUser = func(_ context.Context, _ string) error { return nil }
	zitadelRevokeMappingRules = func(_ context.Context, _, _, _ string) error { return nil }
	return &sweepsRun
}

func TestScheduler_CtxCancel_ExitsCleanly(t *testing.T) {
	resetSweepDeps(t)
	_ = installMinimalNoop(t)

	sched := NewScheduler(10*time.Millisecond, 10)
	ctx, cancel := context.WithCancel(context.Background())

	go sched.Start(ctx)

	// Let the initial runOnce + one tick happen, then cancel.
	time.Sleep(30 * time.Millisecond)
	cancel()

	select {
	case <-sched.Done():
		// expected
	case <-time.After(500 * time.Millisecond):
		t.Fatal("scheduler Done() did not close within 500ms of ctx cancel")
	}
}

// TestScheduler_Done_BlocksUntilInFlightSweepCompletes asserts the shutdown
// contract main.go relies on: Done() must not close while a sweep is still
// executing. A slow sweep followed by ctx cancel must still block Done()
// until that sweep returns.
func TestScheduler_Done_BlocksUntilInFlightSweepCompletes(t *testing.T) {
	resetSweepDeps(t)
	_ = installMinimalNoop(t)

	sweepRunning := make(chan struct{}, 1)
	sweepCanFinish := make(chan struct{})
	var sweepExited int32

	svcGetExpiredDirectGrants = func(ctx context.Context, _ int) ([]models.DirectGrant, error) {
		select {
		case sweepRunning <- struct{}{}:
		default:
		}
		<-sweepCanFinish
		atomic.StoreInt32(&sweepExited, 1)
		return nil, nil
	}

	sched := NewScheduler(1*time.Hour, 10) // long interval: only the initial runOnce fires
	ctx, cancel := context.WithCancel(context.Background())
	go sched.Start(ctx)

	<-sweepRunning
	cancel()

	// Done() must remain open while the sweep is mid-flight. Poll briefly
	// to confirm.
	select {
	case <-sched.Done():
		t.Fatal("Done() closed while sweep was still executing — shutdown join would be unsafe")
	case <-time.After(50 * time.Millisecond):
		// expected — sweep still running
	}
	if atomic.LoadInt32(&sweepExited) != 0 {
		t.Fatal("test invariant broken: sweep exited before we released it")
	}

	// Release the sweep; now Done() must close promptly.
	close(sweepCanFinish)
	select {
	case <-sched.Done():
		// expected
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Done() did not close after sweep finished")
	}
}

func TestScheduler_TickerFires_InvokesSweep(t *testing.T) {
	resetSweepDeps(t)
	counter := installMinimalNoop(t)

	sched := NewScheduler(15*time.Millisecond, 10)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go sched.Start(ctx)

	// Wait long enough for the initial sweep + at least 2 ticks.
	time.Sleep(80 * time.Millisecond)
	cancel()

	n := atomic.LoadInt32(counter)
	if n < 2 {
		t.Fatalf("expected sweep to run at least twice (initial + ticks), ran %d times", n)
	}
}

func TestScheduler_PanicRecovered_LoopContinues(t *testing.T) {
	resetSweepDeps(t)
	_ = installMinimalNoop(t)

	var calls int32
	svcGetExpiredDirectGrants = func(_ context.Context, _ int) ([]models.DirectGrant, error) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			panic("simulated bug")
		}
		return nil, nil
	}

	sched := NewScheduler(15*time.Millisecond, 10)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go sched.Start(ctx)
	time.Sleep(80 * time.Millisecond)
	cancel()

	n := atomic.LoadInt32(&calls)
	if n < 2 {
		t.Fatalf("panic in first sweep killed the loop; only %d calls made", n)
	}
}

func TestNewScheduler_ClampsBadInputs(t *testing.T) {
	s := NewScheduler(0, 0)
	if s.interval != 5*time.Minute {
		t.Errorf("zero interval should default to 5m, got %s", s.interval)
	}
	if s.batchSize != 1 {
		t.Errorf("zero batchSize should clamp to 1, got %d", s.batchSize)
	}

	s2 := NewScheduler(-1*time.Second, 100000)
	if s2.interval != 5*time.Minute {
		t.Errorf("negative interval should default to 5m, got %s", s2.interval)
	}
	if s2.batchSize != 10000 {
		t.Errorf("oversized batchSize should clamp to 10000, got %d", s2.batchSize)
	}
}
