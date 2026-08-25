package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// 4.16/4.17 — the three-valued lifecycle state.

func TestReadOnlyRefusesMutationsAndKeepsServingReads(t *testing.T) {
	l := newLifecycle(LifecycleReadOnly)

	if _, err := l.Begin(); !errors.Is(err, errLifecycleRefusal) {
		t.Fatalf("read_only must refuse a mutation, got %v", err)
	}
	// Reads are never gated. An add-on that stopped answering /subjects during
	// a maintenance window would make the backend record the target as
	// unreconciled — reporting a deliberate state as an outage.
	if l.InFlight() != 0 {
		t.Fatal("a refused mutation must not count itself in flight")
	}
	state, _ := l.State()
	if state != LifecycleReadOnly {
		t.Fatalf("state = %q", state)
	}
}

// Draining is what makes an API-key rotation or a target upgrade safe: refuse
// new work, let issued work settle, and be able to SAY when it has.
func TestDrainingRefusesNewWorkAndReportsWhenSettled(t *testing.T) {
	l := newLifecycle(LifecycleActive)

	done, err := l.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Set(LifecycleDraining, "rotating the key"); err != nil {
		t.Fatal(err)
	}

	if _, err := l.Begin(); !errors.Is(err, errLifecycleRefusal) {
		t.Fatal("draining must refuse a new mutation")
	}
	if l.Drained() {
		t.Fatal("it is not drained while one is still in flight — this is what an operator waits on")
	}
	done()
	if !l.Drained() {
		t.Fatal("it must report drained once the last one settles")
	}
	// Idempotent: a handler that defers done() and also calls it on an early
	// return must not drive the count negative and report drained early.
	done()
	if l.InFlight() != 0 {
		t.Fatalf("in-flight must not go negative, got %d", l.InFlight())
	}
}

// Neither state is unhealthy, and neither is a reason to stop answering.
func TestNeitherMaintenanceStateIsReportedAsUnhealthy(t *testing.T) {
	for _, state := range []string{LifecycleDraining, LifecycleReadOnly} {
		l := newLifecycle(state)
		if _, err := l.Begin(); !errors.Is(err, errLifecycleRefusal) {
			t.Errorf("%s must refuse a mutation, got %v", state, err)
		}
		got, _ := l.State()
		if got != state {
			t.Errorf("state = %q, want %q", got, state)
		}
	}
}

// An operator who typed `readonly` and got a serving add-on has the opposite of
// what they asked for. Refusing into the safest state is the only direction
// that cannot surprise them.
func TestAnUnrecognisedConfiguredStateFailsSafeRatherThanActive(t *testing.T) {
	l := newLifecycle("readonly") // note: not the vocabulary
	state, reason := l.State()
	if state != LifecycleReadOnly {
		t.Fatalf("want read_only, got %q", state)
	}
	if reason == "" {
		t.Error("and it must say why, or an operator sees a state they did not set with no explanation")
	}
	// The refusal names the vocabulary and never the rejected value: this is
	// written from configuration and read back on an operator surface.
	err := l.Set("nonsense", "manual")
	if err == nil {
		t.Fatal("an unknown state must be refused")
	}
	if strings.Contains(err.Error(), "nonsense") {
		t.Error("the refusal must name the vocabulary, not echo the value")
	}
	for _, v := range []string{LifecycleActive, LifecycleDraining, LifecycleReadOnly} {
		if !strings.Contains(err.Error(), v) {
			t.Errorf("the refusal must name %q", v)
		}
	}
}

// The check and the increment are one critical section. Between a caller being
// allowed and that caller counting itself, a drain must not observe zero.
func TestBeginIsAtomicAgainstADrainObservingZero(t *testing.T) {
	l := newLifecycle(LifecycleActive)

	// Two WaitGroups: the observer must outlive the workers and stop when they
	// are done, and putting it in the same group is a deadlock — it would wait
	// for a signal sent only after it had finished.
	var workers, observer sync.WaitGroup
	stop := make(chan struct{})
	var sawDrainedWithWorkAllowed atomic.Bool

	observer.Add(1)
	go func() {
		defer observer.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			// Once draining, "drained" must never be true while a Begin that
			// was allowed has not finished.
			if l.Drained() && l.InFlight() > 0 {
				sawDrainedWithWorkAllowed.Store(true)
			}
			runtime.Gosched()
		}
	}()

	for range 200 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			done, err := l.Begin()
			if err != nil {
				return
			}
			time.Sleep(time.Microsecond)
			done()
		}()
	}
	_ = l.Set(LifecycleDraining, "test")
	workers.Wait()
	close(stop)
	observer.Wait()

	if sawDrainedWithWorkAllowed.Load() {
		t.Fatal("a drain observed itself finished while an allowed mutation was in flight")
	}
}

// A lifecycle refusal is answered as 503 with Retry-After, which is the
// standard way an HTTP service says "not now" — and what the backend reads as
// queued rather than failed.
func TestALifecycleRefusalIsAnsweredAsRetryable(t *testing.T) {
	rr := httptest.NewRecorder()
	writeLifecycleRefusal(rr, LifecycleDraining)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503, got %d", rr.Code)
	}
	if rr.Header().Get("Retry-After") == "" {
		// Without it the backend cannot tell a deliberate maintenance window
		// from an add-on that fell over mid-apply, and would record the change
		// as indeterminate rather than queued.
		t.Fatal("a lifecycle refusal must carry Retry-After")
	}
	var body map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["lifecycle"] != LifecycleDraining {
		t.Errorf("the refusal must say which state refused it, got %q", body["lifecycle"])
	}
}
