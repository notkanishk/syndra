package main

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func swapCheckObservation(v func(context.Context) error) func() {
	orig := checkObservation
	checkObservation = v
	return func() { checkObservation = orig }
}

// Drift must not run before the observation sweep has written anything — see
// driftSched's wiring in main(). This is the one thing that wiring depends
// on: that the wait actually ends once the store answers, and that it does
// not hang forever when the store never will.

func TestAwaitFirstObservation_ReturnsAsSoonAsTheStoreAnswers(t *testing.T) {
	defer swapCheckObservation(func(context.Context) error { return nil })()

	done := make(chan struct{})
	go func() {
		awaitFirstObservation(context.Background())
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("awaitFirstObservation must return immediately once the store already has an observation")
	}
}

func TestAwaitFirstObservation_StopsOnContextCancellationRatherThanBlockingForever(t *testing.T) {
	defer swapCheckObservation(func(context.Context) error { return errors.New("nothing observed yet") })()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already done before the wait starts

	done := make(chan struct{})
	go func() {
		awaitFirstObservation(ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("a cancelled context must not be waited out on the startup retry schedule")
	}
}

// Retention always runs (addon-contract 6.7): per-person points must not
// outlive their retention because STATS_COLLECTION_ENABLED is off or no
// add-on is registered. Only collection is gated, and main starts the
// retention runner unconditionally, at the top level of main.
func TestStatsRetentionRunsWithoutCollection(t *testing.T) {
	for _, collect := range []bool{false, true} {
		collector, retention := statsRunners(collect)
		if retention == nil {
			t.Errorf("collect=%t: no retention runner", collect)
		}
		if (collector != nil) != collect {
			t.Errorf("collect=%t: collector %v", collect, collector)
		}
	}
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), "\n\tgo statsPruneSched.Start(ctx)\n") {
		t.Error("main does not start the retention runner unconditionally")
	}
}
