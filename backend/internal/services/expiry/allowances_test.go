package expiry

import (
	"context"
	"errors"
	"testing"
	"time"

	"syndra/internal/db"
)

// 8.7/8.8 — a lapsed suspension restores access, and the restoration is
// recorded.

type allowanceHarness struct {
	lapsed     []db.Allowance
	lapsedErr  error
	resolved   []string
	resolveErr map[string]error
	converged  []string
	convErr    error
}

func (h *allowanceHarness) install(t *testing.T) {
	t.Helper()
	l, r, c := dbLapsedAllowances, dbResolveLapsedAllowance, reconvergeSubject
	dbLapsedAllowances = func(context.Context, int) ([]db.Allowance, error) { return h.lapsed, h.lapsedErr }
	dbResolveLapsedAllowance = func(_ context.Context, id string) error {
		if err := h.resolveErr[id]; err != nil {
			return err
		}
		h.resolved = append(h.resolved, id)
		return nil
	}
	reconvergeSubject = func(_ context.Context, subject, target string) error {
		h.converged = append(h.converged, subject+"@"+target)
		return h.convErr
	}
	t.Cleanup(func() { dbLapsedAllowances, dbResolveLapsedAllowance, reconvergeSubject = l, r, c })
}

func lapsedAllowance(id, subject string) db.Allowance {
	past := time.Now().Add(-time.Hour)
	return db.Allowance{
		ID: id, SubjectID: subject, Target: "truenas", Field: "group", Value: "lab_makers",
		Direction: db.AllowanceDeny, ActorID: "op_1", Reason: "safety review", ExpiresAt: &past,
	}
}

func TestALapsedAllowanceIsResolvedAndTheSubjectReconverges(t *testing.T) {
	h := &allowanceHarness{lapsed: []db.Allowance{lapsedAllowance("a1", "u1"), lapsedAllowance("a2", "u2")}}
	h.install(t)

	SweepAllowances(context.Background(), 100)

	if len(h.resolved) != 2 {
		t.Fatalf("both must be resolved, got %v", h.resolved)
	}
	// Resolution alone leaves Syndra right and the target wrong: the resolver
	// already excludes a lapsed allowance, so nothing else would ever tell the
	// target the suspension is over.
	if len(h.converged) != 2 || h.converged[0] != "u1@truenas" {
		t.Fatalf("each subject must re-converge on the target the suspension was on, got %v", h.converged)
	}
}

// The record is the conditional write. A renewal landing in the window takes
// the whole thing with it, and nothing re-converges a subject whose suspension
// was just extended.
func TestARenewedAllowanceIsNotReconverged(t *testing.T) {
	h := &allowanceHarness{
		lapsed:     []db.Allowance{lapsedAllowance("a1", "u1")},
		resolveErr: map[string]error{"a1": errors.New("no matching row")},
	}
	h.install(t)

	SweepAllowances(context.Background(), 100)

	if len(h.converged) != 0 {
		t.Fatalf("a suspension that did not end must not restore access: %v", h.converged)
	}
}

// One subject's failure must not cost another's. The suspension has ended in
// Syndra either way; the failure is that the target has not been told, which
// the drift sweep raises.
func TestAFailedReconvergenceCostsOnlyItsOwnSubject(t *testing.T) {
	h := &allowanceHarness{
		lapsed:  []db.Allowance{lapsedAllowance("a1", "u1"), lapsedAllowance("a2", "u2")},
		convErr: errors.New("target unreachable"),
	}
	h.install(t)

	SweepAllowances(context.Background(), 100)

	if len(h.resolved) != 2 || len(h.converged) != 2 {
		t.Fatalf("the pass must continue past a failure: resolved=%v converged=%v", h.resolved, h.converged)
	}
}

// Cancellation stops where it is rather than finishing the list, which would
// fail every remaining write one by one and read as an outage.
func TestCancellationStopsTheAllowanceSweepMidList(t *testing.T) {
	h := &allowanceHarness{lapsed: []db.Allowance{
		lapsedAllowance("a1", "u1"), lapsedAllowance("a2", "u2"), lapsedAllowance("a3", "u3"),
	}}
	h.install(t)

	ctx, cancel := context.WithCancel(context.Background())
	orig := dbResolveLapsedAllowance
	dbResolveLapsedAllowance = func(ctx context.Context, id string) error {
		err := orig(ctx, id)
		cancel() // cancelled after the first
		return err
	}

	SweepAllowances(ctx, 100)

	if len(h.resolved) != 1 {
		t.Fatalf("the sweep must stop at the cancellation, got %v", h.resolved)
	}
}
