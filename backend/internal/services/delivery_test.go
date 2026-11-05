package services

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"testing"
	"time"

	"syndra/internal/db"
	"syndra/internal/models"
	"syndra/internal/services/propagation"
)

// deliveryFixture: rows r1..r3; roleWaits marks which carry a waiting role.
func deliveryFixture(t *testing.T, roleWaits map[string]bool, model string, halt string) (sent *[]string, reasons map[string]string) {
	t.Helper()
	origW, origS, origC, origD := svcOutboxRowsWaitForRole, svcSetWaitReason, svcGetConfigSetting, svcDrainBatch
	origR, origP := svcScheduleRetry, svcGetPendingRows
	t.Cleanup(func() {
		svcOutboxRowsWaitForRole, svcSetWaitReason, svcGetConfigSetting, svcDrainBatch = origW, origS, origC, origD
		svcScheduleRetry, svcGetPendingRows = origR, origP
	})
	// A halted send's rows are scheduled for a retry, as db.ScheduleRetry does.
	svcScheduleRetry = func(_ context.Context, ids []string, _ string) ([]string, error) { return ids, nil }
	svcGetPendingRows = func(context.Context) ([]models.PendingPropagation, error) { return nil, nil }
	reasons = map[string]string{}
	var drained []string
	svcOutboxRowsWaitForRole = func(_ context.Context, ids []string) (map[string]bool, error) {
		out := map[string]bool{}
		for _, id := range ids {
			out[id] = roleWaits[id]
		}
		return out, nil
	}
	svcSetWaitReason = func(_ context.Context, ids []string, reason string) error {
		for _, id := range ids {
			reasons[id] = reason
		}
		return nil
	}
	svcGetConfigSetting = func(context.Context, string) (string, error) { return model, nil }
	svcDrainBatch = func(_ context.Context, ids []string) (propagation.DrainResult, error) {
		drained = append(drained, ids...)
		if halt != "" {
			return propagation.DrainResult{Halted: true, Reason: halt}, nil
		}
		return propagation.DrainResult{Applied: len(ids)}, nil
	}
	return &drained, reasons
}

func sorted(s []string) []string { out := append([]string(nil), s...); sort.Strings(out); return out }

// N14 B: a role that waits waits however it arrives; everything else goes.
func TestDeliver_ARoleThatWaitsWaitsAndTheRestGo(t *testing.T) {
	drained, reasons := deliveryFixture(t, map[string]bool{"r2": true}, db.ConfirmationModelRoles, "")
	d := Deliver(context.Background(), []string{"r1", "r2", "r3"}, false)

	if !reflect.DeepEqual(sorted(*drained), []string{"r1", "r3"}) {
		t.Fatalf("sent: want r1 r3, got %v", *drained)
	}
	if !reflect.DeepEqual(d.Waiting, []string{"r2"}) || reasons["r2"] != db.WaitReasonRoleWaits {
		t.Fatalf("r2 waits because its role does: waiting=%v reasons=%v", d.Waiting, reasons)
	}
	if d.WaitReason() != db.WaitReasonRoleWaits {
		t.Fatalf("wait reason: %q", d.WaitReason())
	}
}

// Before the switch over nothing reaches Zitadel sooner than it used to: a
// mode set to wait still holds every row.
func TestDeliver_BeforeTheSwitchTheOldModeStillHolds(t *testing.T) {
	drained, reasons := deliveryFixture(t, map[string]bool{"r2": true}, "", "")
	d := Deliver(context.Background(), []string{"r1", "r2"}, true)
	if len(*drained) != 0 {
		t.Fatalf("nothing sent before the switch when the mode waits; sent %v", *drained)
	}
	if reasons["r1"] != db.WaitReasonManual || reasons["r2"] != db.WaitReasonRoleWaits {
		t.Fatalf("reasons: %v", reasons)
	}
	if len(d.Waiting) != 2 {
		t.Fatalf("both wait, got %v", d.Waiting)
	}
}

// After the switch the old mode is ignored: only the role decides.
func TestDeliver_AfterTheSwitchOnlyTheRoleDecides(t *testing.T) {
	drained, _ := deliveryFixture(t, map[string]bool{}, db.ConfirmationModelRoles, "")
	d := Deliver(context.Background(), []string{"r1"}, true)
	if !reflect.DeepEqual(*drained, []string{"r1"}) || len(d.Waiting) != 0 || d.WaitReason() != "" {
		t.Fatalf("after the switch a manual mode no longer holds: sent=%v waiting=%v", *drained, d.Waiting)
	}
}

// A send that halts before anything went (Zitadel unreachable) is retried on
// its own, like a 503 after a send (owner, 5 Nov 2026): the rows it left get a
// next try, and the outcome says halted:retrying with when and which try,
// never "it goes when you send it".
func TestDeliver_AHaltedSendIsRetriedOnItsOwn(t *testing.T) {
	_, reasons := deliveryFixture(t, map[string]bool{"r2": true}, db.ConfirmationModelRoles, "zitadel_offline")
	var scheduled []string
	svcScheduleRetry = func(_ context.Context, ids []string, why string) ([]string, error) {
		if why != "halted: zitadel_offline" {
			t.Fatalf("last_error = %q", why)
		}
		scheduled = append(scheduled, ids...)
		return ids, nil
	}
	next := time.Date(2026, 11, 5, 8, 35, 0, 0, time.UTC)
	svcGetPendingRows = func(context.Context) ([]models.PendingPropagation, error) {
		return []models.PendingPropagation{{ID: "r1", Status: "pending", OpType: "add", Attempts: 1, NotBefore: &next}}, nil
	}
	d := Deliver(context.Background(), []string{"r1", "r2"}, false)
	if !reflect.DeepEqual(scheduled, []string{"r1"}) {
		t.Fatalf("only the row meant to go now is scheduled, never one whose role waits: %v", scheduled)
	}
	if d.Reason["r1"] != "halted:retrying" || reasons["r2"] != db.WaitReasonRoleWaits {
		t.Fatalf("reasons: delivery %v recorded %v", d.Reason, reasons)
	}
	// The role that waits still outranks it for the action's one reason.
	if d.WaitReason() != db.WaitReasonRoleWaits {
		t.Fatalf("WaitReason = %q", d.WaitReason())
	}
	if r := d.Drain.Retry; r == nil || r.State != "retrying" || r.Try != 2 || !r.NextTry.Equal(next) {
		t.Fatalf("Retry = %+v, want retrying try 2 at %v", r, next)
	}

	// Alone, it says it is retrying.
	deliveryFixture(t, map[string]bool{}, db.ConfirmationModelRoles, "zitadel_offline")
	if got := Deliver(context.Background(), []string{"r1"}, false).WaitReason(); got != "halted:retrying" {
		t.Fatalf("WaitReason = %q, want halted:retrying", got)
	}
}

// If the schedule can't be written, the halt reason is recorded and the rows
// wait for Send: nothing claims a retry that was never scheduled.
func TestDeliver_AnUnscheduledHaltSaysWhy(t *testing.T) {
	_, reasons := deliveryFixture(t, map[string]bool{}, db.ConfirmationModelRoles, "zitadel_offline")
	svcScheduleRetry = func(context.Context, []string, string) ([]string, error) { return nil, errors.New("db down") }
	d := Deliver(context.Background(), []string{"r1"}, false)
	if reasons["r1"] != "halted:zitadel_offline" || d.WaitReason() != "halted:zitadel_offline" || d.Drain.Retry != nil {
		t.Fatalf("halted reason not recorded: %v %q %+v", reasons, d.WaitReason(), d.Drain.Retry)
	}
}

// Unable to tell whether a role waits, Syndra sends nothing: the rows are
// committed and stay pending, the safe way to be wrong.
func TestDeliver_UnreadableFlagsSendNothing(t *testing.T) {
	drained, _ := deliveryFixture(t, nil, db.ConfirmationModelRoles, "")
	svcOutboxRowsWaitForRole = func(context.Context, []string) (map[string]bool, error) {
		return nil, errors.New("db down")
	}
	d := Deliver(context.Background(), []string{"r1"}, false)
	if len(*drained) != 0 || len(d.Waiting) != 1 {
		t.Fatalf("nothing may be sent when the flags can't be read: sent=%v waiting=%v", *drained, d.Waiting)
	}
}

// Why the bundles' and rules' own mode could be dropped (migration 000078):
// under roles it changed nothing. For any marks and any send outcome, a mode
// set to wait (legacyWait true) and one set to go deliver identically: the
// same rows sent, the same rows waiting, the same reasons recorded. applyMode
// now always passes false, which is therefore the same delivery as before.
func TestDeliver_UnderRolesTheOldModeChangedNothing(t *testing.T) {
	for _, halt := range []string{"", "zitadel_offline"} {
		for _, waits := range []map[string]bool{{}, {"r1": true}, {"r1": true, "r2": true}} {
			run := func(legacy bool) (Delivery, []string, map[string]string) {
				drained, reasons := deliveryFixture(t, waits, db.ConfirmationModelRoles, halt)
				d := Deliver(context.Background(), []string{"r1", "r2"}, legacy)
				return d, append([]string(nil), *drained...), reasons
			}
			manual, manualSent, manualWhy := run(true)
			auto, autoSent, autoWhy := run(false)
			if !reflect.DeepEqual(manual, auto) || !reflect.DeepEqual(manualSent, autoSent) || !reflect.DeepEqual(manualWhy, autoWhy) {
				t.Fatalf("halt=%q waits=%v: manual %+v sent %v why %v; auto %+v sent %v why %v",
					halt, waits, manual, manualSent, manualWhy, auto, autoSent, autoWhy)
			}
		}
	}
}
