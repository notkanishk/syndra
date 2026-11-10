package db

import (
	"encoding/json"
	"testing"
)

// A whole observation replaces the base: that is what keeps the Zitadel
// sweep's bases self-pruning (a revoke Syndra performed leaves no stale key to
// read against a later re-grant). A Partial one, a withdraw-only apply's two
// lifecycle fields, updates only its own and keeps the groups a person held
// while they are away.
func TestAPartialObservationKeepsTheFieldsItDidNotSee(t *testing.T) {
	ctx := liveDB(t)
	subject := freshSubject(t, ctx)
	t.Cleanup(func() { _ = ForgetMergeBase(ctx, "truenas", subject) })
	record := func(base map[string]json.RawMessage, partial bool) map[string]json.RawMessage {
		t.Helper()
		if err := RecordMergeBase(ctx, MergeBase{Target: "truenas", SubjectID: subject, Base: base, Partial: partial}); err != nil {
			t.Fatal(err)
		}
		bases, err := MergeBasesFor(ctx, "truenas")
		if err != nil {
			t.Fatal(err)
		}
		return bases[subject].Base
	}
	full := map[string]json.RawMessage{"group": json.RawMessage(`["lab"]`), "enabled": json.RawMessage(`true`)}
	lock := map[string]json.RawMessage{"enabled": json.RawMessage(`false`), "smb_enabled": json.RawMessage(`false`)}

	record(full, false)
	if got := record(lock, true); string(got["group"]) != `["lab"]` || string(got["enabled"]) != "false" {
		b, _ := json.Marshal(got)
		t.Fatalf("partial: base = %s, want group kept and the lifecycle fields updated", b)
	}
	if got := record(lock, false); got["group"] != nil {
		b, _ := json.Marshal(got)
		t.Fatalf("whole: base = %s, want it replaced, group gone", b)
	}
}

// Whether a desired state can only take access away is recorded with the state
// (desired_state_snapshots.withdraws_only), and the outbox row copies it from
// the snapshot its approval cites: no caller passes it to the enqueue, so the
// row, the drain's intent (and so MergeBase.Partial) and the runner all read
// one fact, on the system path and the operator's alike.
func TestTheRowCopiesWithdrawsOnlyFromItsSnapshot(t *testing.T) {
	ctx := liveDB(t)
	if _, err := PG.Exec(ctx, `INSERT INTO targets (target) VALUES ('truenas') ON CONFLICT DO NOTHING`); err != nil {
		t.Fatal(err)
	}
	lock := map[string]json.RawMessage{"enabled": json.RawMessage(`false`), "smb_enabled": json.RawMessage(`false`)}
	for _, want := range []bool{true, false} {
		subject := freshSubject(t, ctx)
		t.Cleanup(func() { _, _ = PG.Exec(ctx, `DELETE FROM propagation_outbox WHERE user_id = $1`, subject) })
		_, outboxID, err := RecordSystemConvergence(ctx, SystemConvergence{
			Target: "truenas", SubjectID: subject, Actor: "system:reconcile", Reason: "test",
			Desired: lock, WithdrawsOnly: want,
		})
		if err != nil {
			t.Fatal(err)
		}
		var row, snapshot bool
		if err := PG.QueryRow(ctx, `
			SELECT p.withdraws_only, s.withdraws_only
			  FROM propagation_outbox p
			  JOIN plan_subjects ps ON ps.id = p.plan_subject_id
			  JOIN desired_state_snapshots s ON s.id = ps.snapshot_id
			 WHERE p.id = $1`, outboxID).Scan(&row, &snapshot); err != nil {
			t.Fatal(err)
		}
		intent, err := ReadEntitlementIntent(ctx, outboxID)
		if err != nil {
			t.Fatal(err)
		}
		if snapshot != want || row != want || intent.WithdrawsOnly != want {
			t.Fatalf("withdraws-only %v: snapshot %v, row %v, intent %v", want, snapshot, row, intent.WithdrawsOnly)
		}
	}
}
