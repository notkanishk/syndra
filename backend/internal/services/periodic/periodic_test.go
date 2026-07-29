package periodic

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunner_CtxCancel_ExitsCleanly(t *testing.T) {
	r := New("TEST", 10*time.Millisecond, time.Minute, func(context.Context) error { return nil })
	ctx, cancel := context.WithCancel(context.Background())

	go r.Start(ctx)

	// Let the initial run + one tick happen, then cancel.
	time.Sleep(30 * time.Millisecond)
	cancel()

	select {
	case <-r.Done():
		// expected
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Done() did not close within 500ms of ctx cancel")
	}
}

// TestRunner_Done_BlocksUntilInFlightRunCompletes asserts the shutdown
// contract main.go relies on: Done() must not close while a run is still
// executing — main joins on Done() before closing shared DB/Redis clients.
func TestRunner_Done_BlocksUntilInFlightRunCompletes(t *testing.T) {
	runStarted := make(chan struct{}, 1)
	runCanFinish := make(chan struct{})
	var runExited int32

	r := New("TEST", time.Hour, time.Minute, func(context.Context) error {
		select {
		case runStarted <- struct{}{}:
		default:
		}
		<-runCanFinish
		atomic.StoreInt32(&runExited, 1)
		return nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	go r.Start(ctx)

	<-runStarted
	cancel()

	select {
	case <-r.Done():
		t.Fatal("Done() closed while run was still executing — shutdown join would be unsafe")
	case <-time.After(50 * time.Millisecond):
		// expected — run still in flight
	}
	if atomic.LoadInt32(&runExited) != 0 {
		t.Fatal("test invariant broken: run exited before we released it")
	}

	close(runCanFinish)
	select {
	case <-r.Done():
		// expected
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Done() did not close after run finished")
	}
}

func TestRunner_TickerFires_InvokesRun(t *testing.T) {
	var runs int32
	r := New("TEST", 15*time.Millisecond, time.Minute, func(context.Context) error {
		atomic.AddInt32(&runs, 1)
		return nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go r.Start(ctx)

	// Wait long enough for the initial run + at least 2 ticks.
	time.Sleep(80 * time.Millisecond)
	cancel()

	if n := atomic.LoadInt32(&runs); n < 2 {
		t.Fatalf("expected run to fire at least twice (initial + ticks), ran %d times", n)
	}
}

func TestRunner_PanicRecovered_LoopContinues(t *testing.T) {
	var calls int32
	r := New("TEST", 15*time.Millisecond, time.Minute, func(context.Context) error {
		if atomic.AddInt32(&calls, 1) == 1 {
			panic("simulated bug")
		}
		return nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go r.Start(ctx)
	time.Sleep(80 * time.Millisecond)
	cancel()

	if n := atomic.LoadInt32(&calls); n < 2 {
		t.Fatalf("panic in first run killed the loop; only %d calls made", n)
	}
}

func TestNew_FallsBackOnNonPositiveInterval(t *testing.T) {
	if r := New("TEST", 0, 5*time.Minute, nil); r.interval != 5*time.Minute {
		t.Errorf("zero interval should fall back to 5m, got %s", r.interval)
	}
	if r := New("TEST", -time.Second, 6*time.Hour, nil); r.interval != 6*time.Hour {
		t.Errorf("negative interval should fall back to 6h, got %s", r.interval)
	}
}
