package db

import (
	"testing"
)

// RoleDeliveries returns, per role, only the latest add or revoke, and per
// project only the latest replace; a cancelled row never stands in front of the
// row before it, and an add-on's rows are not Zitadel's.
func TestRoleDeliveriesKeepOnlyTheLatestRowPerRole(t *testing.T) {
	ctx := liveDB(t)
	user := freshSubject(t, ctx)
	t.Cleanup(func() { _, _ = PG.Exec(ctx, `DELETE FROM propagation_outbox WHERE user_id = $1`, user) })

	seedOutbox(t, ctx, "add", "applied", user, "pRD", "direct", "", []string{"r1", "r2"})
	seedOutbox(t, ctx, "revoke", "pending", user, "pRD", "direct", "", []string{"r1"})
	cancelled := seedOutbox(t, ctx, "add", "pending", user, "pRD", "direct", "", []string{"r1"})
	if _, err := PG.Exec(ctx, `UPDATE propagation_outbox SET status = 'superseded' WHERE id = $1`, cancelled); err != nil {
		t.Fatal(err)
	}
	seedOutbox(t, ctx, "replace", "applied", user, "pRD", "direct", "", []string{"r2"})
	seedOutbox(t, ctx, "replace", "pending", user, "pRD", "direct", "", []string{"r3"})
	addon := seedOutbox(t, ctx, "add", "pending", user, "pRD", "direct", "", []string{"r4"})
	if _, err := PG.Exec(ctx, `UPDATE propagation_outbox SET target = 'truenas' WHERE id = $1`, addon); err != nil {
		t.Fatal(err)
	}

	got, err := RoleDeliveries(ctx, []string{user})
	if err != nil {
		t.Fatal(err)
	}
	byKey := map[string]RoleDelivery{}
	for _, d := range got[user] {
		byKey[d.OpType+":"+d.RoleKey] = d
	}
	if _, ok := byKey["add:r4"]; ok || len(got[user]) != 3 {
		t.Fatalf("got %+v, want r1's revoke, r2's add, one replace, nothing for the add-on", got[user])
	}
	if d := byKey["revoke:r1"]; d.Status != "pending" {
		t.Fatalf("r1: %+v, want the pending revoke (the cancelled add never happened)", d)
	}
	if d := byKey["add:r2"]; d.Status != "applied" {
		t.Fatalf("r2: %+v, want the applied add", d)
	}
	if d := byKey["replace:"]; d.OpType != "replace" || d.Status != "pending" || len(d.RoleKeys) != 1 || d.RoleKeys[0] != "r3" {
		t.Fatalf("replace: %+v, want the latest, pending, naming r3", d)
	}
	if byKey["revoke:r1"].IntentSeq >= byKey["replace:"].IntentSeq {
		t.Fatalf("the later replace must carry the larger intent_seq: %+v", got[user])
	}
}
