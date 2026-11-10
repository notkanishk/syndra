package db

import (
	"testing"
)

// RoleDeliveries keeps, per role, only the latest add or revoke and, per
// project, only the latest replace; a cancelled row never stands in front of
// the row before it, and an add-on's rows are not Zitadel's.
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
	addon := seedOutbox(t, ctx, "add", "pending", user, "pAddon", "direct", "", []string{"r4"})
	if _, err := PG.Exec(ctx, `UPDATE propagation_outbox SET target = 'truenas' WHERE id = $1`, addon); err != nil {
		t.Fatal(err)
	}

	got, err := RoleDeliveries(ctx, []string{user})
	if err != nil {
		t.Fatal(err)
	}
	ds := got[user]
	// The pending replace (latest of all) decides r1, r2 and r3; it names r3.
	for role, gives := range map[string]bool{"r1": false, "r2": false, "r3": true} {
		d, ok := ds.Latest("pRD", role)
		if !ok || d.OpType != "replace" || d.Status != "pending" || d.Gives(role) != gives {
			t.Fatalf("%s: %+v (ok %v), want the pending replace, giving it: %v", role, d, ok, gives)
		}
	}
	if _, ok := ds.Latest("pAddon", "r4"); ok {
		t.Fatal("an add-on's row decided a Zitadel role")
	}
}

// deliveredFor: a role is delivered while its latest applied row gives it. A
// later applied replace that leaves the role out takes it away, as a revoke
// does; the first version read only adds and revokes and kept it.
func TestDeliveredForReadsTheLatestAppliedRow(t *testing.T) {
	ctx := liveDB(t)
	user := freshSubject(t, ctx)
	t.Cleanup(func() { _, _ = PG.Exec(ctx, `DELETE FROM propagation_outbox WHERE user_id = $1`, user) })

	seedOutbox(t, ctx, "add", "applied", user, "pDF", "direct", "", []string{"kept", "dropped", "revoked"})
	seedOutbox(t, ctx, "revoke", "applied", user, "pDF", "direct", "", []string{"revoked"})
	seedOutbox(t, ctx, "replace", "applied", user, "pDF", "direct", "", []string{"kept"})
	seedOutbox(t, ctx, "add", "pending", user, "pDF", "direct", "", []string{"later"})

	got, err := deliveredFor(ctx, PG, "zitadel", []string{user})
	if err != nil {
		t.Fatal(err)
	}
	want := map[RoleRef]bool{{ProjectID: "pDF", RoleKey: "kept"}: true}
	if len(got[user]) != len(want) || !got[user][RoleRef{ProjectID: "pDF", RoleKey: "kept"}] {
		t.Fatalf("delivered %v, want only kept (dropped by the replace, revoked, later not applied)", got[user])
	}
}
