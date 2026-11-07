package services

import (
	"context"
	"errors"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"syndra/internal/db"
	"syndra/internal/models"
	"syndra/internal/services/propagation"
)

// deliveryFixture stubs the send: every row it is given goes (or the send
// halts with halt), and what is recorded on a row after the commit is kept.
func deliveryFixture(t *testing.T, halt string) (sent *[]string, reasons map[string]string) {
	t.Helper()
	origS, origD := svcSetWaitReason, svcDrainBatch
	origR, origP := svcScheduleRetry, svcGetPendingRows
	t.Cleanup(func() {
		svcSetWaitReason, svcDrainBatch = origS, origD
		svcScheduleRetry, svcGetPendingRows = origR, origP
	})
	// A halted send's rows are scheduled for a retry, as db.ScheduleRetry does.
	svcScheduleRetry = func(_ context.Context, ids []string, _ string) ([]string, error) { return ids, nil }
	svcGetPendingRows = func(context.Context) ([]models.PendingPropagation, error) { return nil, nil }
	reasons = map[string]string{}
	var drained []string
	svcSetWaitReason = func(_ context.Context, ids []string, reason string) error {
		for _, id := range ids {
			reasons[id] = reason
		}
		return nil
	}
	svcDrainBatch = func(_ context.Context, ids []string) (propagation.DrainResult, error) {
		drained = append(drained, ids...)
		if halt != "" {
			return propagation.DrainResult{Halted: true, Reason: halt}, nil
		}
		return propagation.DrainResult{Applied: len(ids)}, nil
	}
	return &drained, reasons
}

// planned is a written Change of rows ids, those in waits marked by the plan
// to wait for you (written with role_waits), the rest to go straight away.
func planned(ids []string, waits map[string]bool) Change {
	var ch Change
	for _, id := range ids {
		w := Write{Delivery: DeliveryStraightAway, OutboxID: id}
		if waits[id] {
			w.Delivery, w.WaitReason = DeliveryWaits, db.WaitReasonRoleWaits
		}
		ch.Writes = append(ch.Writes, w)
	}
	return ch
}

func sorted(s []string) []string { out := append([]string(nil), s...); sort.Strings(out); return out }

// N14 B: a write the plan said waits waits; everything else goes.
func TestDeliverChange_ARoleThatWaitsWaitsAndTheRestGo(t *testing.T) {
	drained, _ := deliveryFixture(t, "")
	d := DeliverChange(context.Background(), planned([]string{"r1", "r2", "r3"}, map[string]bool{"r2": true}))

	if !reflect.DeepEqual(sorted(*drained), []string{"r1", "r3"}) {
		t.Fatalf("sent: want r1 r3, got %v", *drained)
	}
	if !reflect.DeepEqual(d.Waiting, []string{"r2"}) || d.Reason["r2"] != db.WaitReasonRoleWaits {
		t.Fatalf("r2 waits because its role does: waiting=%v reasons=%v", d.Waiting, d.Reason)
	}
	if d.WaitReason() != db.WaitReasonRoleWaits {
		t.Fatalf("wait reason: %q", d.WaitReason())
	}
}

// A send that halts before anything went (Zitadel unreachable) is retried on
// its own, like a 503 after a send (owner, 5 Nov 2026): the rows it left get a
// next try, and the outcome says halted:retrying with when and which try,
// never "it goes when you send it".
func TestDeliverChange_AHaltedSendIsRetriedOnItsOwn(t *testing.T) {
	deliveryFixture(t, "zitadel_offline")
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
	d := DeliverChange(context.Background(), planned([]string{"r1", "r2"}, map[string]bool{"r2": true}))
	if !reflect.DeepEqual(scheduled, []string{"r1"}) {
		t.Fatalf("only the row meant to go now is scheduled, never one whose role waits: %v", scheduled)
	}
	if d.Reason["r1"] != "halted:retrying" || d.Reason["r2"] != db.WaitReasonRoleWaits {
		t.Fatalf("reasons: %v", d.Reason)
	}
	// The role that waits still outranks it for the action's one reason.
	if d.WaitReason() != db.WaitReasonRoleWaits {
		t.Fatalf("WaitReason = %q", d.WaitReason())
	}
	if r := d.Drain.Retry; r == nil || r.State != "retrying" || r.Try != 2 || !r.NextTry.Equal(next) {
		t.Fatalf("Retry = %+v, want retrying try 2 at %v", r, next)
	}

	// Alone, it says it is retrying.
	deliveryFixture(t, "zitadel_offline")
	if got := DeliverChange(context.Background(), planned([]string{"r1"}, nil)).WaitReason(); got != "halted:retrying" {
		t.Fatalf("WaitReason = %q, want halted:retrying", got)
	}
}

// If the schedule can't be written, the halt reason is recorded and the rows
// wait for Send: nothing claims a retry that was never scheduled.
func TestDeliverChange_AnUnscheduledHaltSaysWhy(t *testing.T) {
	_, reasons := deliveryFixture(t, "zitadel_offline")
	svcScheduleRetry = func(context.Context, []string, string) ([]string, error) { return nil, errors.New("db down") }
	d := DeliverChange(context.Background(), planned([]string{"r1"}, nil))
	if reasons["r1"] != "halted:zitadel_offline" || d.WaitReason() != "halted:zitadel_offline" || d.Drain.Retry != nil {
		t.Fatalf("halted reason not recorded: %v %q %+v", reasons, d.WaitReason(), d.Drain.Retry)
	}
}

// Decision 11, task 1.9: the after-commit re-read of which roles wait is
// gone. Nothing in services decides delivery from the role_waits table after
// a commit: no Deliver, no read of a row's roles against it. Every access
// change is planned and sent by DeliverChange.
func TestTheAfterCommitRoleWaitsReadIsGone(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		src := string(b)
		for _, gone := range []string{"func Deliver(", "Deliver(ctx,", "OutboxRowsWaitForRole", "applyMode(", "switchedOver("} {
			if strings.Contains(src, gone) {
				t.Errorf("services/%s still has %q: delivery is decided only by the planner", name, gone)
			}
		}
	}
}
