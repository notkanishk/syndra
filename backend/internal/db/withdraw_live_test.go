package db

import (
	"context"
	"errors"
	"testing"
	"time"
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
	// Applied and never taken back: the grant is live in Zitadel right now.
	seedOutbox(t, ctx, "add", "applied", "u1", "p1", "bundle", "b1", []string{"community"})
	pending := seedOutbox(t, ctx, "add", "pending", "u1", "p1", "bundle", "b1", []string{"community"})

	if owed := runWithdraw(t, ctx, revokeParams("u1", "p1", "community", "bundle", "b1")); !owed {
		t.Fatal("the revoke was dropped although this grant is still standing in Zitadel")
	}
	// The undelivered re-delivery is still cancelled: dispatching it after the
	// revoke would re-create what the revoke just removed.
	if status, _ := statusOf(t, ctx, pending); status != "superseded" {
		t.Fatalf("the queued re-delivery is still %q and would run after the revoke", status)
	}
}

// The sequence the operator actually had, and the one the first version of
// this rule got wrong: granted, revoked, granted again, all before the second
// grant was ever sent.
//
// `EXISTS(applied add)` is true throughout — there are two of them in the
// history — so the revoke was queued every time, and the six annihilating rows
// came straight back with only the wording changed. What decides it is which
// of the two came LAST.
func TestARevokeIsDroppedWhenAnEarlierGrantWasAlreadyTakenBack(t *testing.T) {
	ctx := liveDB(t)
	seedOutbox(t, ctx, "add", "applied", "u1", "p1", "bundle", "b1", []string{"community"})
	seedOutbox(t, ctx, "revoke", "applied", "u1", "p1", "bundle", "b1", []string{"community"})
	pending := seedOutbox(t, ctx, "add", "pending", "u1", "p1", "bundle", "b1", []string{"community"})

	if owed := runWithdraw(t, ctx, revokeParams("u1", "p1", "community", "bundle", "b1")); owed {
		t.Fatal("a revoke was queued for a grant that an earlier revocation had already taken back")
	}
	if status, _ := statusOf(t, ctx, pending); status != "superseded" {
		t.Fatalf("the queued re-delivery is still %q", status)
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
func TestAnAddNeverCancelsAnotherAdd(t *testing.T) {
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

// Removed, then given back before the removal was sent (15 Oct): the add
// cancels the queued revoke, and owes nothing while the grant still stands.
func TestGivingBackCancelsAQueuedRemoval(t *testing.T) {
	ctx := liveDB(t)
	user := "live-wd-" + time.Now().Format("150405.000000000")
	t.Cleanup(func() { _, _ = PG.Exec(ctx, `DELETE FROM propagation_outbox WHERE user_id = $1`, user) })
	seedOutbox(t, ctx, "add", "applied", user, "p1", "direct", "", []string{"community"})
	revoke := seedOutbox(t, ctx, "revoke", "pending", user, "p1", "direct", "old-grant", []string{"community"})
	add := EnqueueParams{UserID: user, ProjectID: "p1", RoleKeys: []string{"community"}, OpType: "add", Source: "direct"}
	if owed := runWithdraw(t, ctx, add); owed {
		t.Fatal("the grant still stands in Zitadel, so giving it back owes no add")
	}
	if status, _ := statusOf(t, ctx, revoke); status != "superseded" {
		t.Fatalf("the queued removal is cancelled, got %s", status)
	}
	// Once the removal has gone out, giving it back is owed.
	seedOutbox(t, ctx, "revoke", "applied", user, "p1", "direct", "", []string{"community"})
	if owed := runWithdraw(t, ctx, add); !owed {
		t.Fatal("nothing queued to cancel and nothing standing: the add is owed")
	}
}

// What the operator is told is what happens: the preview names exactly the
// rows the action then cancels, and each cancellation is in Change history.
// Covers the direct case that used to slip: an unsent direct grant (no
// reference) removed by its grant id.
func TestThePreviewNamesWhatTheActionCancels(t *testing.T) {
	ctx := liveDB(t)
	user := "live-wd-" + time.Now().Format("150405.000000000")
	t.Cleanup(func() {
		_, _ = PG.Exec(ctx, `DELETE FROM propagation_outbox WHERE user_id = $1`, user)
		_, _ = PG.Exec(ctx, `DELETE FROM audit_logs WHERE target_zitadel_user_id = $1`, user)
	})
	add := seedOutbox(t, ctx, "add", "pending", user, "p1", "direct", "", []string{"community"})
	other := seedOutbox(t, ctx, "add", "pending", user, "p1", "direct", "", []string{"staff"})
	removal := revokeParams(user, "p1", "community", "direct", "grant-123")
	removal.GrantedBy = "op"

	preview, err := WouldCancel(ctx, removal)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview) != 1 || preview[0].ID != add || preview[0].OpType != "add" {
		t.Fatalf("the preview names the one queued add, got %+v", preview)
	}
	if owed := runWithdraw(t, ctx, removal); owed {
		t.Fatal("never sent, so nothing is owed")
	}
	if status, _ := statusOf(t, ctx, add); status != "superseded" {
		t.Fatalf("the add the preview named is cancelled, got %s", status)
	}
	if status, _ := statusOf(t, ctx, other); status != "pending" {
		t.Fatalf("another role's add is untouched, got %s", status)
	}
	var audited int
	_ = PG.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE resource_id=$1 AND action='propagation.cancelled_unsent' AND actor_zitadel_user_id='op'`, add).Scan(&audited)
	if audited != 1 {
		t.Fatalf("the cancellation is audited once, by the operator, got %d", audited)
	}
}

// Extending before the first send reuses the queued add: one row, not two.
func TestAnIdenticalQueuedAddIsReused(t *testing.T) {
	ctx := liveDB(t)
	user := "live-wd-" + time.Now().Format("150405.000000000")
	t.Cleanup(func() {
		_, _ = PG.Exec(ctx, `DELETE FROM propagation_outbox WHERE user_id = $1`, user)
		_, _ = PG.Exec(ctx, `DELETE FROM direct_role_grants WHERE user_id = $1`, user)
		_, _ = PG.Exec(ctx, `DELETE FROM audit_logs WHERE target_zitadel_user_id = $1`, user)
	})
	give := func(rule EndRule, days int) string {
		t.Helper()
		end := time.Now().Add(time.Duration(days) * 24 * time.Hour).UTC()
		res, err := EnqueueDirectGrantPropagation(ctx, EnqueueParams{
			UserID: user, ProjectID: "p1", RoleKeys: []string{"community"}, GrantedBy: "op",
			Reason: "r", ExpiresAt: &end, OpType: "add", PayloadJSON: `{}`, EndRule: rule,
		})
		if err != nil {
			t.Fatal(err)
		}
		return res.OutboxID
	}
	first := give(EndKept, 10)
	second := give(EndSet, 40)
	if first != second {
		t.Fatalf("the extension rides the queued add, got %s then %s", first, second)
	}
	var n int
	_ = PG.QueryRow(ctx, `SELECT count(*) FROM propagation_outbox WHERE user_id=$1 AND status='pending'`, user).Scan(&n)
	if n != 1 {
		t.Fatalf("one queued add, got %d", n)
	}
}

// On an operator's request, a cancellation they weren't shown is refused and
// nothing moves; shown that exact change, it is cancelled.
func TestAnUnshownCancellationIsRefusedAndNothingMoves(t *testing.T) {
	ctx := liveDB(t)
	user := "live-wd-" + time.Now().Format("150405.000000000")
	t.Cleanup(func() {
		_, _ = PG.Exec(ctx, `DELETE FROM propagation_outbox WHERE user_id = $1`, user)
		_, _ = PG.Exec(ctx, `DELETE FROM audit_logs WHERE target_zitadel_user_id = $1`, user)
	})
	revoke := seedOutbox(t, ctx, "revoke", "pending", user, "p1", "direct", "old", []string{"community"})
	add := EnqueueParams{UserID: user, ProjectID: "p1", RoleKeys: []string{"community"}, OpType: "add", Source: "direct", GrantedBy: "op"}

	gated, gate := GuardCancellations(ctx, nil, nil)
	tx, err := PG.Begin(gated)
	if err != nil {
		t.Fatal(err)
	}
	_, err = withdrawUndelivered(gated, tx, "zitadel", add)
	_ = tx.Rollback(gated)
	if !errors.Is(err, ErrWouldCancel) || len(gate.Refused()) != 1 || gate.Refused()[0].ID != revoke {
		t.Fatalf("refused, naming the queued removal: err %v, refused %+v", err, gate.Refused())
	}
	if status, _ := statusOf(t, ctx, revoke); status != "pending" {
		t.Fatalf("nothing moves on a refusal, got %s", status)
	}

	shown, _ := GuardCancellations(ctx, []string{revoke}, nil)
	tx, err = PG.Begin(shown)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := withdrawUndelivered(shown, tx, "zitadel", add); err != nil {
		_ = tx.Rollback(shown)
		t.Fatalf("shown exactly this, it goes ahead: %v", err)
	}
	if err := tx.Commit(shown); err != nil {
		t.Fatal(err)
	}
	if status, _ := statusOf(t, ctx, revoke); status != "superseded" {
		t.Fatalf("cancelled once acknowledged, got %s", status)
	}
}
