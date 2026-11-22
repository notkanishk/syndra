package addons

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"syndra/internal/db"
)

// holdDown makes the monitor hold target addon_unreachable (design §13.2:
// from checking, failureThreshold misses).
func holdDown(t *testing.T, target string) {
	t.Helper()
	m := monitor
	monitor = newHealthMonitor()
	t.Cleanup(func() { monitor = m })
	// wrong_host, not no_answer: the fallback for a reply with no status is
	// no_answer, so only another cause shows the monitor's why travelling.
	for range failureThreshold {
		monitor.observe(target, TargetHealth{Outcome: OutcomeUnreached, Why: Why{Reason: ReasonAddonUnreachable, Cause: CauseWrongHost}}, timeNow().UTC())
	}
}

// Design §13.4: an interactive write to a system the monitor holds
// addon_unreachable is refused at once, undialed, and leaves no record; its
// why is the monitor's, so the page names the cause the roster does.
func TestAWriteToAHeldSystemIsRefusedUndialed(t *testing.T) {
	events, calls := recorder(t, http.StatusOK)
	traceEvents(t, events)
	holdDown(t, "truenas")

	resp, ok := PutSetting(context.Background(), "truenas", "home_root", "/mnt/p/home", "op-1")
	if ok || resp.Outcome != OutcomeUnreached {
		t.Fatalf("setting: ok %v resp %+v", ok, resp)
	}
	if why := Unread(resp); why.Reason != ReasonAddonUnreachable || why.Cause != CauseWrongHost {
		t.Fatalf("setting why = %+v, want the monitor's", why)
	}
	_, err := SetLifecycle(context.Background(), "truenas", "draining", "maintenance", "op-1")
	var nl *NotLandedError
	if !errors.As(err, &nl) || nl.Outcome != OutcomeUnreached || nl.Why.Cause != CauseWrongHost {
		t.Fatalf("lifecycle err = %v, want NotLandedError unreached no_answer", err)
	}
	if len(*calls) != 0 || len(*events) != 0 {
		t.Fatalf("calls %d events %v: nothing may be recorded or sent", len(*calls), *events)
	}
	if why, held := Held("truenas"); !held || why.Reason != ReasonAddonUnreachable {
		t.Fatalf("Held = %+v %v", why, held)
	}
}

// App down is not held: the add-on answers, and its own refusal says so.
func TestAnAppOutageIsNotHeld(t *testing.T) {
	m := monitor
	monitor = newHealthMonitor()
	t.Cleanup(func() { monitor = m })
	observeSeq(monitor, "truenas", timeNow().UTC(), probeAppOff, probeAppOff, probeAppOff)
	if st := HealthOf("truenas"); st.State != StateAppUnreachable {
		t.Fatalf("three app misses are app_unreachable, got %+v", st)
	}
	if _, held := Held("truenas"); held {
		t.Fatal("app_unreachable is the add-on's to answer, not refused undialed")
	}
}

// A write that went out and lost its answer is indeterminate, said apart from
// one that never left.
func TestALifecycleWhoseAnswerWasLostIsIndeterminate(t *testing.T) {
	recorder(t, http.StatusInternalServerError)
	traceEvents(t, &[]string{})
	_, err := SetLifecycle(context.Background(), "truenas", "draining", "maintenance", "op-1")
	var nl *NotLandedError
	if !errors.As(err, &nl) || nl.Outcome != OutcomeIndeterminate {
		t.Fatalf("err = %v, want NotLandedError indeterminate", err)
	}
}

// The preview of a physical action is refused the same way, with the same why.
func TestAPreviewOfAHeldSystemIsNotSent(t *testing.T) {
	hits := inventoryAddon(t, reply(200, `{}`))
	holdDown(t, "inventory")
	r := Preview(context.Background(), "inventory", "item.checkout", "s", "s", map[string]any{"item_id": "item-2207"})
	if r.Outcome != OutcomeUnreached || hits.Load() != 0 || r.Why().Cause != CauseWrongHost {
		t.Fatalf("outcome %s, %d requests, why %+v", r.Outcome, hits.Load(), r.Why())
	}
}

// Owner 28.3: only a revocation that can't reach its system waits without
// escalating. HeldTargets is that set: the held system, and not one whose
// add-on answers its probe (its failing calls age like Zitadel's).
func TestHeldTargetsIsOnlyTheSystemsHeldUnreachable(t *testing.T) {
	recorder(t, http.StatusOK)
	holdDown(t, "truenas")
	if got := HeldTargets(); len(got) != 1 || got[0] != "truenas" {
		t.Fatalf("held = %v, want [truenas]", got)
	}
	monitor.observe("truenas", probeAppOff, timeNow().UTC())
	if got := HeldTargets(); len(got) != 0 {
		t.Fatalf("an add-on that answers is not held: %v", got)
	}
}

// Syndra's own database failing before the record is Syndra's fault, not an
// unreachable add-on: ErrOwnFault, never a NotLandedError that says "check
// its container".
func TestARecordThatDidNotCommitIsSyndrasOwnFault(t *testing.T) {
	_, calls := recorder(t, http.StatusOK)
	saved := dbBeginAddonOperation
	t.Cleanup(func() { dbBeginAddonOperation = saved })
	dbBeginAddonOperation = func(context.Context, db.AddonOperationParams) (string, error) {
		return "", errors.New("connection reset")
	}
	_, err := SetLifecycle(context.Background(), "truenas", "draining", "maintenance", "op-1")
	var nl *NotLandedError
	if !errors.Is(err, ErrOwnFault) || errors.As(err, &nl) {
		t.Fatalf("lifecycle err = %v, want ErrOwnFault and no NotLandedError", err)
	}
	resp, _ := PutSetting(context.Background(), "truenas", "home_root", "/mnt/p/home", "op-1")
	if !errors.Is(resp.Err, ErrOwnFault) {
		t.Fatalf("setting err = %v, want ErrOwnFault", resp.Err)
	}
	if len(*calls) != 0 {
		t.Fatalf("nothing may be sent without a record: %d calls", len(*calls))
	}
}
