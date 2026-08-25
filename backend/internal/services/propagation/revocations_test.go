package propagation

import (
	"context"
	"os"
	"regexp"
	"strings"
	"testing"

	"syndra/internal/addons"
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

// The retry budget matters more here than in the operator drain, because
// nobody is watching. A spent row that halted the pass without going terminal
// was re-claimed first on every tick — so every revocation queued behind it
// never drained, for ever, on the one row class where delay IS the exposure.
func TestDrainRevocations_ASpentRowDoesNotBlockTheOnesBehindIt(t *testing.T) {
	stubDrainDeps(t)
	rows := revokeRows("r1", "r2")
	t.Cleanup(swap(&claimRevocations, func(ctx context.Context, target string, limit int) ([]models.PendingPropagation, error) {
		out, err := rows(ctx, target, limit)
		if err != nil {
			return nil, err
		}
		out[0].Attempts = maxRetries
		return out, nil
	}))
	liveUserGrantRoles = func(context.Context, string, string) (map[string]bool, error) {
		return map[string]bool{"r": true}, nil
	}
	zitadelRemoveUserGrant = func(context.Context, string, string) error { return statusErr(503) }
	var requeued []string
	requeue = func(_ context.Context, id, _ string) (int, error) {
		requeued = append(requeued, id)
		return 1, nil
	}
	failed := map[string]string{}
	markFailed = func(_ context.Context, id, msg string) error { failed[id] = msg; return nil }

	res, err := DrainRevocations(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.Halted {
		t.Fatalf("a spent revocation must not stop the runner: %+v", res)
	}
	if res.Exhausted != 1 || failed["r1"] == "" {
		t.Fatalf("the spent row must be terminal with a reason: %+v %v", res, failed)
	}
	if len(requeued) != 1 || requeued[0] != "r2" {
		t.Fatalf("the revocation behind it must still be attempted: %v", requeued)
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

// §17 — the add-on leg. A target revocation queues an `apply` row, and before
// this the background runner claimed only `revoke`: the lock sat in the queue
// while the response told the operator it drains on its own.
//
// The dispatcher is the entitlement one, not the Zitadel one — a TrueNAS row has
// no project and no roles for that path to send.
func TestDrainRevocations_DrainsAddonWithdrawals(t *testing.T) {
	h := stubAddonDrain(t)
	t.Cleanup(swap(&registeredAddons, func() []addons.Registration {
		return []addons.Registration{{Target: "truenas"}}
	}))
	t.Cleanup(swap(&claimRevocations, func(_ context.Context, target string, _ int) ([]models.PendingPropagation, error) {
		if target == db.TargetZitadel {
			return nil, nil
		}
		return []models.PendingPropagation{addonRow("lock-1")}, nil
	}))

	res, err := DrainRevocations(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(h.dispatched) != 1 || h.dispatched[0].Subject != "sub-1" {
		t.Fatalf("the queued lock must be dispatched to its target, got %+v", h.dispatched)
	}
	if res.Applied != 1 {
		t.Fatalf("want the lock applied, got %+v", res)
	}
}

// An unreachable NAS must not stop a door system's revocation, and neither must
// stop Zitadel's. Each leg is a separate deployment with a separate outage.
func TestDrainRevocations_OneTargetsOutageDoesNotStopAnother(t *testing.T) {
	h := stubAddonDrain(t)
	h.reachable = false
	t.Cleanup(swap(&registeredAddons, func() []addons.Registration {
		return []addons.Registration{{Target: "truenas"}}
	}))
	t.Cleanup(swap(&claimRevocations, revokeRows("r1")))
	liveUserGrantRoles = func(context.Context, string, string) (map[string]bool, error) {
		return map[string]bool{"r": true}, nil
	}
	var applied []string
	markApplied = func(_ context.Context, id string) error { applied = append(applied, id); return nil }

	res, err := DrainRevocations(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(applied) != 1 || applied[0] != "r1" {
		t.Fatalf("Zitadel's revoke must still land while the add-on is down, got %v", applied)
	}
	if !res.Halted || res.HaltedTarget != "truenas" || res.Reason != "target_unreachable" {
		t.Errorf("the unreachable target must be named rather than folded into a total: %+v", res)
	}
}
