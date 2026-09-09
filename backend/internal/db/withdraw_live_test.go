package db

import (
	"context"
	"testing"
)

// What a revoke is owed, against a real database.
//
// The rule under test: a revoke answering a delivery that never left the queue
// cancels that delivery instead of queueing its opposite. Getting it wrong in
// one direction costs a no-op call to Zitadel; getting it wrong in the other
// leaves access in place that somebody asked to remove. So every case below is
// written from the second direction — what must STILL be revoked.

func seedOutbox(t *testing.T, ctx context.Context, opType, status, user, project, source, ref string, roles []string) string {
	t.Helper()
	key, err := newOutboxIdempotencyKey()
	if err != nil {
		t.Fatalf("idempotency key: %v", err)
	}
	var id string
	err = PG.QueryRow(ctx, `
		INSERT INTO propagation_outbox
			(op_type, user_id, project_id, role_keys, payload_json, idempotency_key,
			 initiated_by, source, source_ref, target, status)
		VALUES ($1,$2,$3,$4,'{}',$5,'tester',$6,NULLIF($7,''),'zitadel',$8)
		RETURNING id`,
		opType, user, project, roles, key, source, ref, status).Scan(&id)
	if err != nil {
		t.Fatalf("seed outbox row: %v", err)
	}
	return id
}

func statusOf(t *testing.T, ctx context.Context, id string) (string, string) {
	t.Helper()
	var status string
	var reason *string
	if err := PG.QueryRow(ctx,
		`SELECT status, last_error FROM propagation_outbox WHERE id=$1`, id).
		Scan(&status, &reason); err != nil {
		t.Fatalf("read row: %v", err)
	}
	if reason == nil {
		return status, ""
	}
	return status, *reason
}

// runWithdraw calls the unit under test inside a committed transaction, which
// is the only way its UPDATE is visible to the assertions afterwards.
func runWithdraw(t *testing.T, ctx context.Context, p EnqueueParams) bool {
	t.Helper()
	tx, err := PG.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	owed, err := withdrawUndelivered(ctx, tx, "zitadel", p)
	if err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("withdrawUndelivered: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
	return owed
}

func revokeParams(user, project, role, source, ref string) EnqueueParams {
	return EnqueueParams{
		UserID: user, ProjectID: project, RoleKeys: []string{role},
		OpType: "revoke", Source: source, SourceRef: ref,
	}
}

// The case the operator hit: assigned, then removed before anything was sent.
func TestAQueuedDeliveryIsCancelledRatherThanCompensated(t *testing.T) {
	ctx := liveDB(t)
	add := seedOutbox(t, ctx, "add", "pending", "u1", "p1", "bundle", "b1", []string{"community"})

	if owed := runWithdraw(t, ctx, revokeParams("u1", "p1", "community", "bundle", "b1")); owed {
		t.Fatal("a revoke was still owed for a delivery that never left the queue")
	}

	status, reason := statusOf(t, ctx, add)
	if status != "superseded" {
		t.Fatalf("the queued delivery is still %q; it must not be dispatched after being undone", status)
	}
	if reason != withdrawnReason {
		t.Fatalf("row carries reason %q; an operator reading the row has to be told why it will not run", reason)
	}
}

// In flight is not the same as queued. The call may already be with Zitadel,
// so its effect still has to be undone.
func TestAnInFlightDeliveryIsNeverCancelled(t *testing.T) {
	ctx := liveDB(t)
	add := seedOutbox(t, ctx, "add", "in_flight", "u1", "p1", "bundle", "b1", []string{"community"})

	if owed := runWithdraw(t, ctx, revokeParams("u1", "p1", "community", "bundle", "b1")); !owed {
		t.Fatal("the revoke was dropped against an in-flight delivery — the grant may already exist")
	}
	if status, _ := statusOf(t, ctx, add); status != "in_flight" {
		t.Fatalf("an in-flight row was moved to %q", status)
	}
}

// A re-delivery. The earlier one landed, so the grant is real and only this
// revoke takes it back.
func TestARevokeSurvivesWhenTheGrantWasDeliveredBefore(t *testing.T) {
	ctx := liveDB(t)
	seedOutbox(t, ctx, "add", "applied", "u1", "p1", "bundle", "b1", []string{"community"})
	pending := seedOutbox(t, ctx, "add", "pending", "u1", "p1", "bundle", "b1", []string{"community"})

	if owed := runWithdraw(t, ctx, revokeParams("u1", "p1", "community", "bundle", "b1")); !owed {
		t.Fatal("the revoke was dropped although this grant had already been delivered once")
	}
	// The undelivered re-delivery is still cancelled: dispatching it after the
	// revoke would re-create what the revoke just removed.
	if status, _ := statusOf(t, ctx, pending); status != "superseded" {
		t.Fatalf("the queued re-delivery is still %q and would run after the revoke", status)
	}
}

// A bundle's removal must never spare a grant some other route delivered.
func TestAnotherSourcesDeliveryIsNeitherCancelledNorSpared(t *testing.T) {
	ctx := liveDB(t)
	direct := seedOutbox(t, ctx, "add", "pending", "u1", "p1", "direct", "", []string{"community"})

	if owed := runWithdraw(t, ctx, revokeParams("u1", "p1", "community", "bundle", "b1")); !owed {
		t.Fatal("a bundle's removal dropped a revoke against a delivery the bundle did not queue")
	}
	if status, _ := statusOf(t, ctx, direct); status != "pending" {
		t.Fatalf("another source's queued delivery was moved to %q", status)
	}
}

// A row carrying more than the role being taken away is left alone: cancelling
// it would withdraw deliveries nobody countermanded.
func TestAMultiRoleDeliveryIsLeftAlone(t *testing.T) {
	ctx := liveDB(t)
	multi := seedOutbox(t, ctx, "add", "pending", "u1", "p1", "direct", "",
		[]string{"community", "operator"})

	if owed := runWithdraw(t, ctx, revokeParams("u1", "p1", "community", "direct", "")); !owed {
		t.Fatal("the revoke was dropped against a row that also carries a role nobody removed")
	}
	if status, _ := statusOf(t, ctx, multi); status != "pending" {
		t.Fatalf("a row carrying an uncountermanded role was moved to %q", status)
	}
}

// Somebody else's row, and a different project's row, are not this revoke's
// business. Written as one case because the failure is identical: a WHERE
// clause missing a column.
func TestTheCancellationIsScopedToOnePersonAndOneProject(t *testing.T) {
	ctx := liveDB(t)
	other := seedOutbox(t, ctx, "add", "pending", "u2", "p1", "bundle", "b1", []string{"community"})
	elsewhere := seedOutbox(t, ctx, "add", "pending", "u1", "p2", "bundle", "b1", []string{"community"})
	mine := seedOutbox(t, ctx, "add", "pending", "u1", "p1", "bundle", "b1", []string{"community"})

	runWithdraw(t, ctx, revokeParams("u1", "p1", "community", "bundle", "b1"))

	if status, _ := statusOf(t, ctx, other); status != "pending" {
		t.Fatalf("another person's queued delivery was cancelled (%q)", status)
	}
	if status, _ := statusOf(t, ctx, elsewhere); status != "pending" {
		t.Fatalf("another project's queued delivery was cancelled (%q)", status)
	}
	if status, _ := statusOf(t, ctx, mine); status != "superseded" {
		t.Fatalf("the row this revoke answers was left %q", status)
	}
}

// An add is not a revoke. Nothing else may take this path.
func TestOnlyARevokeWithdrawsAnything(t *testing.T) {
	ctx := liveDB(t)
	add := seedOutbox(t, ctx, "add", "pending", "u1", "p1", "bundle", "b1", []string{"community"})

	p := revokeParams("u1", "p1", "community", "bundle", "b1")
	p.OpType = "add"
	if owed := runWithdraw(t, ctx, p); !owed {
		t.Fatal("a non-revoke param was reported as not owed, which would drop the row entirely")
	}
	if status, _ := statusOf(t, ctx, add); status != "pending" {
		t.Fatalf("an add cancelled another add (%q)", status)
	}
}
