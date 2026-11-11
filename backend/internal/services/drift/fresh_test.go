package drift

import (
	"context"
	"slices"
	"testing"

	"syndra/internal/addons"
	"syndra/internal/db"
)

// Fresh reads (feature freshness, design §4): the sweep is the one scheduled
// sender, and a cooldown is a skipped pass, never an outage.

func TestTheSweepAlwaysAsksForAFreshRead(t *testing.T) {
	h := &addonReconcileHarness{read: currentRead()}
	stubAddonReconcile(t, h)
	if _, err := ReconcileAddon(context.Background(), "truenas"); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(h.askedFresh, []bool{true}) {
		t.Errorf("the sweep's reads asked fresh %v, want [true]", h.askedFresh)
	}
}

func TestACooldownIsASkippedPassNeverAnOutage(t *testing.T) {
	retry := 12
	for name, read := range map[string]addons.SubjectsResult{
		"refused, nothing to serve": {Outcome: addons.OutcomeUnreached, Status: 429, RetryAfter: retry},
		"answered with the copy": {Outcome: addons.OutcomeSucceeded, TakenAt: currentRead().TakenAt,
			Stale: &addons.Stale{Reason: addons.StaleCooldown, RetryAfter: &retry}},
	} {
		h := &addonReconcileHarness{read: read}
		stubAddonReconcile(t, h)
		res, err := ReconcileAddon(context.Background(), "truenas")
		if err != nil {
			t.Fatal(err)
		}
		if !res.Halted || res.Reason != db.UnreconciledStaleRead || !slices.Equal(h.marked, []string{db.UnreconciledStaleRead}) {
			t.Errorf("%s: halted %t reason %q marked %v, want a skipped pass recorded as a stale read", name, res.Halted, res.Reason, h.marked)
		}
		if len(h.converged) != 0 || h.reconciled {
			t.Errorf("%s: a skipped pass concluded something", name)
		}
	}
}

// A page load accepts the copy; "Read again" asks for a fresh read.
func TestTheInventoryAsksFreshOnlyWhenAPersonDoes(t *testing.T) {
	h := &addonReconcileHarness{read: currentRead()}
	stubAddonReconcile(t, h)
	for _, fresh := range []bool{false, true} {
		if _, err := Inventory(context.Background(), "truenas", fresh); err != nil {
			t.Fatal(err)
		}
	}
	if !slices.Equal(h.askedFresh, []bool{false, true}) {
		t.Errorf("the inventory asked fresh %v, want [false true]", h.askedFresh)
	}
}

func TestTheInventorySaysHowFreshItsReadIs(t *testing.T) {
	cached := &addons.Stale{Reason: "cached"}
	h := &addonReconcileHarness{read: addons.SubjectsResult{Outcome: addons.OutcomeSucceeded, TakenAt: currentRead().TakenAt, Stale: cached}}
	stubAddonReconcile(t, h)
	res, _ := Inventory(context.Background(), "truenas", false)
	if res.Halted || res.Current || res.ReadStale != cached || res.Reason != db.UnreconciledStaleRead {
		t.Errorf("a copy is shown, dated and labelled: %+v", res)
	}

	h.read = addons.SubjectsResult{Outcome: addons.OutcomeUnreached, Status: 429, RetryAfter: 9}
	res, _ = Inventory(context.Background(), "truenas", true)
	if !res.Halted || res.RetryAfter != 9 || res.Reason != db.UnreconciledStaleRead {
		t.Errorf("a cooldown with nothing to serve is a countdown, not an outage: %+v", res)
	}
	if len(h.marked) != 0 {
		t.Error("a listing recorded a reconciliation state")
	}
}
