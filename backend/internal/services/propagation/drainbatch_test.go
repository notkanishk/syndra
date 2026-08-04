package propagation

import (
	"context"
	"strings"
	"testing"

	"syndra/internal/models"
)

// TestDrainBatch_ProcessesOnlyGivenIDs asserts DrainBatch takes the advisory lock once,
// preflights reachability once, and calls claimOne exactly once per given id — a queued
// *manual* row with a different id must never be touched, so an auto cascade drains only its
// own rows. Mirrors drain_test.go's stubDrainDeps(t) idiom (the brief's resetPropagationDeps()
// helper does not exist in this package — drain_test.go uses per-test t.Cleanup swaps instead).
func TestDrainBatch_ProcessesOnlyGivenIDs(t *testing.T) {
	stubDrainDeps(t)
	var claimed []string
	claimOne = func(ctx context.Context, id string) (*models.PendingPropagation, bool, error) {
		claimed = append(claimed, id)
		return &models.PendingPropagation{ID: id, OpType: "add"}, true, nil
	}
	// with empty RoleKeys, alreadyExists' add-branch treats "no roles" as allIndexed=true
	// (vacuous loop), so it short-circuits to applied via applyRow → markApplied.
	grantIndexHasRole = func(ctx context.Context, u, p, r string) (bool, error) { return true, nil }

	res, err := DrainBatch(context.Background(), []string{"a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(claimed, ","); got != "a,b" {
		t.Fatalf("claimed = %q, want a,b", got)
	}
	if res.Applied != 2 {
		t.Fatalf("applied = %d, want 2", res.Applied)
	}
}

func TestDrainBatch_EmptyIDsIsNoop(t *testing.T) {
	stubDrainDeps(t)
	called := false
	claimOne = func(ctx context.Context, id string) (*models.PendingPropagation, bool, error) {
		called = true
		return nil, false, nil
	}
	res, err := DrainBatch(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("claimOne must not be called for an empty id list")
	}
	if res != (DrainResult{}) {
		t.Fatalf("expected zero-value result, got %+v", res)
	}
}

func TestDrainBatch_HaltsWhenLockHeld(t *testing.T) {
	stubDrainDeps(t)
	acquireDrainLock = func(ctx context.Context) (func(), bool, error) { return nil, false, nil }
	claimOne = func(ctx context.Context, id string) (*models.PendingPropagation, bool, error) {
		t.Fatal("must not claim rows when the drain lock is held elsewhere")
		return nil, false, nil
	}

	res, err := DrainBatch(context.Background(), []string{"a"})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Halted || res.Reason != "drain_in_progress" {
		t.Fatalf("expected Halted drain_in_progress, got %+v", res)
	}
}

func TestDrainBatch_HaltsWhenZitadelOffline(t *testing.T) {
	stubDrainDeps(t)
	zitadelReachable = func(ctx context.Context) bool { return false }
	claimOne = func(ctx context.Context, id string) (*models.PendingPropagation, bool, error) {
		t.Fatal("must not claim rows when Zitadel is unreachable")
		return nil, false, nil
	}

	res, err := DrainBatch(context.Background(), []string{"a"})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Halted || res.Reason != "zitadel_offline" {
		t.Fatalf("expected Halted zitadel_offline, got %+v", res)
	}
}

func TestDrainBatch_SkipsNotFoundIDs(t *testing.T) {
	stubDrainDeps(t)
	claimOne = func(ctx context.Context, id string) (*models.PendingPropagation, bool, error) {
		return nil, false, nil // already terminal, gone, or unclaimable
	}
	res, err := DrainBatch(context.Background(), []string{"gone"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Applied != 0 || res.Failed != 0 || res.Requeued != 0 {
		t.Fatalf("expected an all-zero result for a not-found id, got %+v", res)
	}
}
