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

	// As MarkApplied does: an applied row carries when Zitadel answered it.
	if _, err := PG.Exec(ctx, `UPDATE propagation_outbox SET completed_at = NOW() WHERE user_id = $1 AND status = 'applied'`, user); err != nil {
		t.Fatal(err)
	}

	got, err := RoleDeliveries(ctx, []string{user})
	if err != nil {
		t.Fatal(err)
	}
	ds := got[user]
	// Queued: the pending replace (latest queued) decides r1, r2 and r3; it
	// names r3. The cancelled add never stood in front of the revoke.
	for role, gives := range map[string]bool{"r1": false, "r2": false, "r3": true} {
		d, ok := ds.LatestQueued("pRD", role)
		if !ok || d.OpType != "replace" || d.Gives(role) != gives {
			t.Fatalf("%s: %+v (ok %v), want the pending replace, giving it: %v", role, d, ok, gives)
		}
	}
	// Settled, apart: the applied replace (after the applied add) decides r1
	// and r2, and carries when Zitadel answered it.
	if d, ok := ds.LatestSettled("pRD", "r1"); !ok || d.OpType != "replace" || d.Gives("r1") || d.CompletedAt == nil {
		t.Fatalf("r1 settled: %+v (ok %v), want the applied replace, taking it, with its time", d, ok)
	}
	if _, ok := ds.LatestQueued("pAddon", "r4"); ok {
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

// QueuedRevocations asks the latest row (Deliveries.Taking): a revoke still
// queued is a removal on its way, one an add was queued behind is not, and a
// sent one is not queued. A queued replace takes the direct-sourced roles its
// set omits and never the ones it names: `replace` names what SURVIVES, and
// reading it as removals would subtract exactly the access being kept.
func TestQueuedRevocationsReadTheLatestRow(t *testing.T) {
	ctx := liveDB(t)
	user := freshSubject(t, ctx)
	t.Cleanup(func() { _, _ = PG.Exec(ctx, `DELETE FROM propagation_outbox WHERE user_id = $1`, user) })
	for _, role := range []string{"kept", "omitted"} {
		if _, err := UpsertDirectGrant(ctx, user, "pRep", role, "op", "", nil); err != nil {
			t.Fatal(err)
		}
	}
	seedOutbox(t, ctx, "revoke", "pending", user, "pQR", "direct", "", []string{"going"})
	seedOutbox(t, ctx, "revoke", "pending", user, "pQR", "direct", "", []string{"back"})
	seedOutbox(t, ctx, "add", "pending", user, "pQR", "bundle", "b1", []string{"back"})
	seedOutbox(t, ctx, "revoke", "applied", user, "pQR", "direct", "", []string{"gone"})
	seedOutbox(t, ctx, "replace", "in_flight", user, "pRep", "direct", "", []string{"kept"})

	got, err := QueuedRevocations(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	want := []RoleRef{{ProjectID: "pQR", RoleKey: "going"}, {ProjectID: "pRep", RoleKey: "omitted"}}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("queued revocations %v, want %v (an add queued behind a revoke, a sent revoke and a role the replace keeps are not on their way out)", got, want)
	}
}

// Rows are not always sent in intent order (an operator sends one by id, a
// retry waits out its backoff): a newer row already sent never hides an older
// revoke still queued, which will still go.
func TestQueuedRevocationsSeeARevokeANewerSentRowDidNotSettle(t *testing.T) {
	ctx := liveDB(t)
	user := freshSubject(t, ctx)
	t.Cleanup(func() { _, _ = PG.Exec(ctx, `DELETE FROM propagation_outbox WHERE user_id = $1`, user) })
	seedOutbox(t, ctx, "revoke", "pending", user, "pOrd", "direct", "", []string{"r"})
	seedOutbox(t, ctx, "add", "applied", user, "pOrd", "bundle", "b1", []string{"r"})

	got, err := QueuedRevocations(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != (RoleRef{ProjectID: "pOrd", RoleKey: "r"}) {
		t.Fatalf("queued revocations %v, want r: its revoke is still to be sent", got)
	}
}
