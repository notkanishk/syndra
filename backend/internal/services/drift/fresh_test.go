package drift

import (
	"context"
	"slices"
	"testing"
	"time"

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
		// Skipped only after waiting the retry_after once and asking again.
		if !slices.Equal(h.waited, []time.Duration{12 * time.Second}) || !slices.Equal(h.askedFresh, []bool{true, true}) {
			t.Errorf("%s: waited %v, asked fresh %v, want one wait of 12 s and a second fresh read", name, h.waited, h.askedFresh)
		}
	}
}

// A cooldown answer is waited out once (RFC 9110 §10.2.3), and a current
// second read is a normal pass: one skipped read never leaves the target
// unreconciled until the next sweep.
func TestTheSweepWaitsOutOneCooldownAndReadsAgain(t *testing.T) {
	retry := 12
	h := &addonReconcileHarness{read: currentRead(), reads: []addons.SubjectsResult{
		{Outcome: addons.OutcomeSucceeded, TakenAt: currentRead().TakenAt, Stale: &addons.Stale{Reason: addons.StaleCooldown, RetryAfter: &retry}},
	}}
	stubAddonReconcile(t, h)
	res, err := ReconcileAddon(context.Background(), "truenas")
	if err != nil {
		t.Fatal(err)
	}
	if res.Halted || !res.Current || !h.reconciled || len(h.marked) != 0 {
		t.Errorf("a current second read is a pass: halted %t current %t reconciled %t marked %v", res.Halted, res.Current, h.reconciled, h.marked)
	}
	if !slices.Equal(h.waited, []time.Duration{12 * time.Second}) || !slices.Equal(h.askedFresh, []bool{true, true}) {
		t.Errorf("waited %v, asked fresh %v, want one 12 s wait, then a fresh read", h.waited, h.askedFresh)
	}
}

// The wait is the retry_after, never longer than the cooldown the target
// declares; and an interrupted wait reads nothing more.
func TestTheSweepsWaitIsBoundedByTheDeclaredCooldown(t *testing.T) {
	retry := 3600
	h := &addonReconcileHarness{read: addons.SubjectsResult{Outcome: addons.OutcomeUnreached, Status: 429, RetryAfter: retry}}
	stubAddonReconcile(t, h)
	if _, err := ReconcileAddon(context.Background(), "truenas"); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(h.waited, []time.Duration{30 * time.Second}) {
		t.Errorf("waited %v, want the declared 30 s, not the hour asked", h.waited)
	}

	h = &addonReconcileHarness{read: addons.SubjectsResult{Outcome: addons.OutcomeUnreached, Status: 429, RetryAfter: 5}}
	stubAddonReconcile(t, h)
	waitOut = func(context.Context, time.Duration) bool { return false } // the sweep stopped
	res, _ := ReconcileAddon(context.Background(), "truenas")
	if !res.Halted || res.Reason != db.UnreconciledStaleRead || len(h.askedFresh) != 1 {
		t.Errorf("an interrupted wait: halted %t reason %q reads %d, want a skipped pass and no second read", res.Halted, res.Reason, len(h.askedFresh))
	}

	// A target declaring a day waits the ceiling, however the day is written:
	// twice constants.cooldown_default, the default an undeclared target reads.
	if sweepWaitCeiling != 2*addons.FreshCooldown("no-such-target") {
		t.Errorf("the ceiling is %v, want twice constants.cooldown_default (%v)", sweepWaitCeiling, addons.FreshCooldown("no-such-target"))
	}
	h = &addonReconcileHarness{read: addons.SubjectsResult{Outcome: addons.OutcomeUnreached, Status: 429, RetryAfter: 3600}}
	stubAddonReconcile(t, h)
	freshCooldown = func(string) time.Duration { return 24 * time.Hour }
	if _, err := ReconcileAddon(context.Background(), "truenas"); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(h.waited, []time.Duration{sweepWaitCeiling}) {
		t.Errorf("a day declared, an hour asked: waited %v, want the ceiling %v", h.waited, sweepWaitCeiling)
	}
}

// A cooldown answer with no retry_after (or 0) is waited the declared
// cooldown, never no time at all: asking again at once would only be told
// the same.
func TestASweepToldNoRetryAfterWaitsTheDeclaredCooldown(t *testing.T) {
	h := &addonReconcileHarness{read: addons.SubjectsResult{Outcome: addons.OutcomeSucceeded, TakenAt: currentRead().TakenAt,
		Stale: &addons.Stale{Reason: addons.StaleCooldown}}}
	stubAddonReconcile(t, h)
	freshCooldown = func(string) time.Duration { return 20 * time.Second }
	if _, err := ReconcileAddon(context.Background(), "truenas"); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(h.waited, []time.Duration{20 * time.Second}) {
		t.Errorf("waited %v, want the declared 20 s", h.waited)
	}
}

// "Check now" runs the pass inside a person's request: a cooldown answer is
// a skipped pass at once, saying when to ask again, never a wait.
func TestCheckNowNeverWaitsOutACooldown(t *testing.T) {
	retry := 12
	h := &addonReconcileHarness{read: currentRead(), reads: []addons.SubjectsResult{
		{Outcome: addons.OutcomeSucceeded, TakenAt: currentRead().TakenAt, Stale: &addons.Stale{Reason: addons.StaleCooldown, RetryAfter: &retry}},
	}}
	stubAddonReconcile(t, h)
	res, err := ReconcileAddonNow(context.Background(), "truenas")
	if err != nil {
		t.Fatal(err)
	}
	if len(h.waited) != 0 || !slices.Equal(h.askedFresh, []bool{true}) {
		t.Errorf("check now waited %v and asked fresh %v, want no wait and one fresh read", h.waited, h.askedFresh)
	}
	if !res.Halted || res.Reason != db.UnreconciledStaleRead || res.RetryAfter != 12 {
		t.Errorf("check now inside the cooldown: halted %t reason %q retry_after %d, want a skipped pass saying when to ask again", res.Halted, res.Reason, res.RetryAfter)
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
