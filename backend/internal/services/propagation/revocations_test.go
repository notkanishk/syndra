package propagation

import (
	"context"
	"os"
	"regexp"
	"strings"
	"testing"

	"syndra/internal/db"
	"syndra/internal/models"
)

// 1.24/1.25 — the one background drain, and the properties that make it safe to
// have one at all.

func revokeRows(ids ...string) func(context.Context, string, int) ([]models.PendingPropagation, error) {
	return func(context.Context, string, int) ([]models.PendingPropagation, error) {
		out := make([]models.PendingPropagation, 0, len(ids))
		for _, id := range ids {
			out = append(out, models.PendingPropagation{
				ID: id, Target: db.TargetZitadel, OpType: "revoke",
				UserID: "u", ProjectID: "p", RoleKeys: []string{"r"},
			})
		}
		return out, nil
	}
}

func TestDrainRevocations_DispatchesWithdrawals(t *testing.T) {
	stubDrainDeps(t)
	t.Cleanup(swap(&claimRevocations, revokeRows("r1")))
	// A live grant the revoke has to remove, or alreadyExists short-circuits.
	liveUserGrantRoles = func(context.Context, string, string) (map[string]bool, error) {
		return map[string]bool{"r": true}, nil
	}
	var removed string
	zitadelRemoveUserGrant = func(_ context.Context, _, grantID string) error { removed = "called"; _ = grantID; return nil }
	var applied string
	markApplied = func(_ context.Context, id string) error { applied = id; return nil }

	res, err := DrainRevocations(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if removed == "" || applied != "r1" || res.Applied != 1 {
		t.Fatalf("want the revoke dispatched and settled, got removed=%q applied=%q res=%+v", removed, applied, res)
	}
}

// The runner must be unable to reach a grant. It is asserted as "it never asks
// the question": the operator claim is the only seam that can return a
// conferring row, and the runner does not hold it.
func TestDrainRevocations_NeverUsesTheUnrestrictedClaim(t *testing.T) {
	stubDrainDeps(t)
	t.Cleanup(swap(&claimRevocations, revokeRows()))
	claimPending = func(context.Context, string, int) ([]models.PendingPropagation, error) {
		t.Fatal("the background runner must never use the unrestricted claim: it can return access-conferring rows")
		return nil, nil
	}
	claimOne = func(context.Context, string, string) (*models.PendingPropagation, bool, error) {
		t.Fatal("the background runner must not reach the targeted claim either")
		return nil, false, nil
	}
	if _, err := DrainRevocations(context.Background()); err != nil {
		t.Fatal(err)
	}
}

// Sharing the operator drain's lock is the point: two drains dispatching one
// subject's rows concurrently is the interleaving the lock exists to forbid.
// Losing it is not an error and not a spin — the pass returns and the next tick
// tries again, so a busy operator drain slows this runner instead of starving it.
func TestDrainRevocations_YieldsToAConcurrentDrainWithoutClaiming(t *testing.T) {
	stubDrainDeps(t)
	acquireDrainLock = func(context.Context) (func(), bool, error) { return nil, false, nil }
	t.Cleanup(swap(&claimRevocations, func(context.Context, string, int) ([]models.PendingPropagation, error) {
		t.Fatal("nothing may be claimed without the drain lock")
		return nil, nil
	}))

	res, err := DrainRevocations(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !res.Halted || res.Reason != "drain_in_progress" {
		t.Fatalf("want halted/drain_in_progress, got %+v", res)
	}
}

// One probe, not a retry budget. A switched-off target must cost a reachability
// check, or an outage exhausts every revocation's attempts and halts the runner
// permanently on the row class where a silent permanent stop is worst.
func TestDrainRevocations_UnreachableTargetCostsAProbeNotABudget(t *testing.T) {
	stubDrainDeps(t)
	zitadelReachable = func(context.Context) bool { return false }
	t.Cleanup(swap(&claimRevocations, func(context.Context, string, int) ([]models.PendingPropagation, error) {
		t.Fatal("an unreachable target must be discovered before anything is claimed")
		return nil, nil
	}))
	requeue = func(context.Context, string, string) (int, error) {
		t.Fatal("nothing may spend a retry when nothing was dispatched")
		return 0, nil
	}

	res, err := DrainRevocations(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !res.Halted || res.Reason != "zitadel_offline" {
		t.Fatalf("want halted/zitadel_offline, got %+v", res)
	}
}

// The retry-budget halt is inherited from the operator drain and matters more
// here, because nobody is watching. It stops the pass rather than grinding the
// remaining rows down to zero attempts each.
func TestDrainRevocations_HaltsOnRetryBudgetAndLeavesTheRestQueued(t *testing.T) {
	stubDrainDeps(t)
	t.Cleanup(swap(&claimRevocations, revokeRows("r1", "r2")))
	liveUserGrantRoles = func(context.Context, string, string) (map[string]bool, error) {
		return map[string]bool{"r": true}, nil
	}
	zitadelRemoveUserGrant = func(context.Context, string, string) error { return statusErr(503) }
	var requeued []string
	requeue = func(_ context.Context, id, _ string) (int, error) {
		requeued = append(requeued, id)
		return maxRetries + 1, nil
	}

	res, err := DrainRevocations(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !res.Halted || res.Reason != "max_retries_exceeded" {
		t.Fatalf("want halted/max_retries_exceeded, got %+v", res)
	}
	if len(requeued) != 1 || requeued[0] != "r1" {
		t.Fatalf("the halt must stop the pass, not walk the rest of the batch: %v", requeued)
	}
}

// The seam is not merely named right — it is bound to the restricted claim. A
// deps.go edit pointing it at the unrestricted one would pass every test above,
// because the fakes replace it.
func TestRevocationSeamIsBoundToTheRestrictedClaim(t *testing.T) {
	b, err := os.ReadFile("deps.go")
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`claimRevocations\s*=\s*db\.ClaimPendingRevocations`).Match(b) {
		t.Fatal("claimRevocations must bind to db.ClaimPendingRevocations, the only claim that cannot return a conferring row")
	}
	src, err := os.ReadFile("revocations.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"claimPending(", "claimOne("} {
		if strings.Contains(string(src), forbidden) {
			t.Errorf("the background runner must not call %s", forbidden)
		}
	}
}
