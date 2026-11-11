package db

import (
	"encoding/json"
	"testing"
)

// A field retired while rows naming it wait (addon-contract 2.1): the add-on
// refuses a field outside its schema, so a departure lock queued with the
// dropped channel would be refused, and, withdraw-only, hold the person's
// next withdrawal. Retiring the field supersedes exactly the pending rows that
// name it, in the same transaction, with the reason and an audit row: (a) the
// lock is superseded, and the lock the next sweep queues without the field
// sends on its own; (b) a row not naming it is untouched; (c) a claimed
// (in_flight) row is never touched. Saving the same retired list again
// supersedes nothing more.
func TestRetiringAFieldSupersedesTheRowsThatNameIt(t *testing.T) {
	ctx := liveDB(t)
	seedAddonTarget(t, ctx)
	roles := map[string]string{"enabled": "account", "smb_enabled": "channel", "group": "membership"}
	if err := SaveTargetFieldRoles(ctx, "truenas", FieldRoleMap{Roles: roles}); err != nil {
		t.Fatal(err)
	}
	raw := func(s string) json.RawMessage { return json.RawMessage(s) }
	queue := func(desired map[string]json.RawMessage, withdraws bool) (subject, id string) {
		t.Helper()
		subject = freshSubject(t, ctx)
		t.Cleanup(func() { _, _ = PG.Exec(ctx, `DELETE FROM propagation_outbox WHERE user_id = $1`, subject) })
		_, id, err := RecordSystemConvergence(ctx, SystemConvergence{
			Target: "truenas", SubjectID: subject, Actor: "system:reconcile", Reason: "test",
			Desired: desired, WithdrawsOnly: withdraws,
		})
		if err != nil {
			t.Fatal(err)
		}
		return subject, id
	}
	status := func(id string) (status, reason string) {
		t.Helper()
		if err := PG.QueryRow(ctx, `SELECT status, COALESCE(last_error,'') FROM propagation_outbox WHERE id = $1`, id).Scan(&status, &reason); err != nil {
			t.Fatal(err)
		}
		return status, reason
	}

	departed, lock := queue(map[string]json.RawMessage{"enabled": raw(`false`), "smb_enabled": raw(`false`)}, true)
	_, other := queue(map[string]json.RawMessage{"enabled": raw(`true`), "group": raw(`["staff"]`)}, false)
	_, claimed := queue(map[string]json.RawMessage{"enabled": raw(`false`), "smb_enabled": raw(`false`)}, true)
	if _, err := PG.Exec(ctx, `UPDATE propagation_outbox SET status = 'in_flight', started_at = NOW() WHERE id = $1`, claimed); err != nil {
		t.Fatal(err)
	}

	if err := SaveTargetFieldRoles(ctx, "truenas", FieldRoleMap{Roles: roles, Retired: []string{"smb_enabled"}}); err != nil {
		t.Fatal(err)
	}
	if st, reason := status(lock); st != "superseded" || reason == "" {
		t.Fatalf("(a) the lock naming smb_enabled must be superseded with its reason: %s %q", st, reason)
	}
	var audited int
	if err := PG.QueryRow(ctx, `SELECT COUNT(*) FROM audit_logs WHERE resource_id = $1 AND action = 'entitlement.truenas.superseded'`, lock).Scan(&audited); err != nil || audited != 1 {
		t.Fatalf("(a) the supersede must leave one audit row: %d %v", audited, err)
	}
	if st, _ := status(other); st != "pending" {
		t.Fatalf("(b) a row not naming the field is untouched: %s", st)
	}
	if st, _ := status(claimed); st != "in_flight" {
		t.Fatalf("(c) a claimed row is never touched: %s", st)
	}

	// The next sweep's lock, without the retired field, sends on its own: the
	// superseded row holds nothing.
	_, again, err := RecordSystemConvergence(ctx, SystemConvergence{
		Target: "truenas", SubjectID: departed, Actor: "system:reconcile", Reason: "test",
		Desired: map[string]json.RawMessage{"enabled": raw(`false`)}, WithdrawsOnly: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	rows, err := GetPendingPropagations(ctx)
	if err != nil {
		t.Fatal(err)
	}
	sends := false
	for _, r := range rows {
		sends = sends || (r.ID == again && r.SendsOnItsOwn)
	}
	if !sends {
		t.Fatal("(a) the re-queued lock must send on its own")
	}

	// Saving the same retired list again supersedes nothing more.
	if err := SaveTargetFieldRoles(ctx, "truenas", FieldRoleMap{Roles: roles, Retired: []string{"smb_enabled"}}); err != nil {
		t.Fatal(err)
	}
	_, still := queue(map[string]json.RawMessage{"enabled": raw(`false`), "smb_enabled": raw(`false`)}, true)
	if err := SaveTargetFieldRoles(ctx, "truenas", FieldRoleMap{Roles: roles, Retired: []string{"smb_enabled"}}); err != nil {
		t.Fatal(err)
	}
	if st, _ := status(still); st != "pending" {
		t.Fatalf("only a newly retired field supersedes: %s", st)
	}
}
