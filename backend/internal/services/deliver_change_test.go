package services

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"syndra/internal/db"
	"syndra/internal/models"
)

// Delivery decided only by the planner (architecture deepening, decision
// 11): the apply sends a Change's "straight away" writes and leaves its
// "waits for you" writes waiting, as the plan said, without asking again
// which roles wait. A role marked to wait between the plan and the send can't
// hold a write the plan showed going.
func TestDeliverChange_SendsWhatThePlanSaidAndNothingElse(t *testing.T) {
	// Nothing reads which roles wait again (TestTheAfterCommitRoleWaitsReadIsGone).
	drained, recorded := deliveryFixture(t, "")
	write := func(id, delivery, wait string) Write {
		w := Write{Delivery: delivery, OutboxID: id}
		w.WaitReason = wait
		return w
	}
	ch := Change{UserID: "u1", Writes: []Write{
		write("go", DeliveryStraightAway, ""),
		write("", DeliveryNone, ""), // only cancelled a queued change: no row
		write("wait", DeliveryWaits, db.WaitReasonRoleWaits),
	}}

	d := DeliverChange(context.Background(), ch)
	if !reflect.DeepEqual(*drained, []string{"go"}) || !reflect.DeepEqual(d.Sent, []string{"go"}) {
		t.Fatalf("sent %v (drained %v), want only the straight-away row", d.Sent, *drained)
	}
	if !reflect.DeepEqual(d.Waiting, []string{"wait"}) || d.Reason["wait"] != db.WaitReasonRoleWaits {
		t.Fatalf("waiting %v reasons %v, want the waiting row with role_waits", d.Waiting, d.Reason)
	}
	// Written waiting already: nothing is recorded on it after the commit.
	if len(recorded) != 0 {
		t.Errorf("recorded %v after the commit, want nothing", recorded)
	}
	// The words the bulk row reads stay true.
	if d.WaitReason() != db.WaitReasonRoleWaits || d.ReasonFor("go") != "" {
		t.Errorf("WaitReason %q, ReasonFor(go) %q", d.WaitReason(), d.ReasonFor("go"))
	}
}

// A straight-away write Zitadel didn't take is retried on its own, through
// the same send as every other row.
func TestDeliverChange_AHaltedSendIsRetried(t *testing.T) {
	deliveryFixture(t, "zitadel_offline")
	svcGetPendingRows = func(context.Context) ([]models.PendingPropagation, error) { return nil, nil }
	d := DeliverChange(context.Background(), Change{Writes: []Write{{Delivery: DeliveryStraightAway, OutboxID: "go"}}})
	if d.WaitReason() != db.WaitReasonHalted(db.HaltRetrying) {
		t.Fatalf("WaitReason %q, want halted:retrying", d.WaitReason())
	}
}

// The apply pairs the outbox's rows with the Change's writes, and refuses
// (rolling back) when the outbox wrote other rows than the plan said.
func TestChangeWritten_PairsRowsOrRefuses(t *testing.T) {
	ch := Change{Writes: []Write{{Delivery: DeliveryStraightAway}, {Delivery: DeliveryNone}, {Delivery: DeliveryWaits}}}
	got, err := ch.written([]string{"a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	if ids := []string{got.Writes[0].OutboxID, got.Writes[1].OutboxID, got.Writes[2].OutboxID}; !reflect.DeepEqual(ids, []string{"a", "", "b"}) {
		t.Fatalf("paired %v", ids)
	}
	if ch.Writes[0].OutboxID != "" {
		t.Error("the planned Change must not be changed in place")
	}
	for _, ids := range [][]string{{"a"}, {"a", "b", "c"}} {
		if _, err := ch.written(ids); !errors.Is(err, errWrittenOtherwise) {
			t.Errorf("%v: want errWrittenOtherwise, got %v", ids, err)
		}
	}
}

// The bulk reason (task 1.8): the operator's words go to the assignment's
// audit row and every row it queues, not a fixed label.
func TestWriteChange_CarriesTheOperatorsReason(t *testing.T) {
	origA, origR := svcAssignBundleAndEnqueue, svcRemoveBundleFromUserAndEnqueue
	t.Cleanup(func() { svcAssignBundleAndEnqueue, svcRemoveBundleFromUserAndEnqueue = origA, origR })
	var gotReason string
	var gotParams []db.EnqueueParams
	svcAssignBundleAndEnqueue = func(_ context.Context, _, reason, _, _, _ string, ps []db.EnqueueParams) ([]string, bool, error) {
		gotReason, gotParams = reason, ps
		return []string{"r1"}, true, nil
	}
	svcRemoveBundleFromUserAndEnqueue = func(_ context.Context, _, reason, _, _ string, ps []db.EnqueueParams) ([]string, error) {
		gotReason, gotParams = reason, ps
		return []string{"r1"}, nil
	}
	ch := Change{UserID: "u1", Writes: []Write{{Delivery: DeliveryWaits, EnqueueParams: db.EnqueueParams{
		UserID: "u1", ProjectID: "p", RoleKeys: []string{"r"}, OpType: "add", Source: "bundle", SourceRef: "b1",
		WaitReason: db.WaitReasonRoleWaits}}}}
	ctx := context.WithValue(context.Background(), pendingKey, &pendingConvergences{})
	for _, op := range []string{BulkOpAssignBundle, BulkOpRemoveBundle} {
		written, err := writeChange(ctx, BulkRequest{Op: op, BundleID: "b1", Actor: "op", Reason: "term start"}, ch)
		if err != nil {
			t.Fatal(err)
		}
		if gotReason != "term start" || len(gotParams) != 1 || gotParams[0].Reason != "term start" || gotParams[0].GrantedBy != "op" {
			t.Fatalf("%s: reason %q params %+v", op, gotReason, gotParams)
		}
		if gotParams[0].WaitReason != db.WaitReasonRoleWaits || written.Writes[0].OutboxID != "r1" {
			t.Fatalf("%s: the write is queued waiting and paired with its row: %+v / %+v", op, gotParams[0], written.Writes[0])
		}
	}
}
