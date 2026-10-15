package services

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"testing"

	"syndra/internal/db"
	"syndra/internal/services/propagation"
)

// deliveryFixture: rows r1..r3; roleWaits marks which carry a waiting role.
func deliveryFixture(t *testing.T, roleWaits map[string]bool, model string, halt string) (sent *[]string, reasons map[string]string) {
	t.Helper()
	origW, origS, origC, origD := svcOutboxRowsWaitForRole, svcSetWaitReason, svcGetConfigSetting, svcDrainBatch
	t.Cleanup(func() {
		svcOutboxRowsWaitForRole, svcSetWaitReason, svcGetConfigSetting, svcDrainBatch = origW, origS, origC, origD
	})
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

// A send that halts says why on the rows it left, so the outcome can say
// "Zitadel didn't answer" with the link to send it.
func TestDeliver_AHaltedSendSaysWhy(t *testing.T) {
	_, reasons := deliveryFixture(t, map[string]bool{}, db.ConfirmationModelRoles, "zitadel_offline")
	d := Deliver(context.Background(), []string{"r1"}, false)
	if reasons["r1"] != "halted:zitadel_offline" || d.WaitReason() != "halted:zitadel_offline" {
		t.Fatalf("halted reason not recorded: %v %q", reasons, d.WaitReason())
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
