package db_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"syndra/internal/db"
	"syndra/internal/services"
)

// The single acts against a real database (task 1.2, decision 10): for each
// act, a queued change the sheet was never shown makes the write stale with
// nothing written; planned again, the outbox rows the act writes are exactly
// the writes its plan showed, and only the cancellations it named happen.

// actStaleThenThePlan runs one act through both halves. queue puts in the
// change the act would cancel, after the sheet read its plan.
func actStaleThenThePlan(t *testing.T, f plannerFixture, a services.Action, w services.ActWrite, queue func() string) services.Change {
	t.Helper()
	ctx, gate := db.GuardCancellations(f.ctx, nil, func(context.Context) bool { return true })

	shown, err := services.PlanAct(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	if shown.Row.Effect != services.EffectApply {
		t.Fatalf("the act goes ahead: %+v", shown.Row)
	}
	unnamed := queue()
	before := len(sentRowsFor(t, ctx, a.UserID))

	_, err = services.ApplyAct(ctx, a, w, &services.Seen{Fingerprint: shown.Row.Fingerprint, CancelIDs: shown.Row.CancelIDs})
	var stale *services.StalePlan
	if !errors.As(err, &stale) {
		t.Fatalf("a change queued since the sheet read its plan makes it stale, got %v", err)
	}
	if s := status(t, ctx, unnamed); s != "pending" {
		t.Errorf("the change nobody was shown is untouched, got %s", s)
	}
	if after := len(sentRowsFor(t, ctx, a.UserID)); after != before {
		t.Fatalf("a stale act wrote %d outbox row(s)", after-before)
	}

	fresh, err := services.PlanAct(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(fresh.Row.CancelIDs, unnamed) {
		t.Fatalf("the fresh plan names the change it cancels: %v", fresh.Row.CancelIDs)
	}
	written, err := services.ApplyAct(ctx, a, w, &services.Seen{Fingerprint: fresh.Row.Fingerprint, CancelIDs: fresh.Row.CancelIDs})
	if err != nil {
		t.Fatal(err)
	}
	if len(gate.Refused()) != 0 {
		t.Fatalf("a named cancellation was refused: %+v", gate.Refused())
	}
	got, want := sentRows(t, ctx, outboxIDs(written)), shownWrites(fresh)
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("outbox %v, plan showed %v", got, want)
	}
	// Each row as the plan decided its delivery (decision 11).
	for _, wr := range written.Writes {
		if wr.OutboxID == "" {
			continue
		}
		wait, _ := rowWaitAndReason(t, ctx, wr.OutboxID)
		if want := map[bool]string{true: db.WaitReasonRoleWaits}[wr.Delivery == services.DeliveryWaits]; wait != want {
			t.Errorf("%s %s (%s): wait_reason %q, want %q", wr.OpType, wr.RoleKeys[0], wr.Delivery, wait, want)
		}
	}
	if s := status(t, ctx, unnamed); s != "superseded" {
		t.Errorf("the now-named change is cancelled, got %s", s)
	}
	return written
}

func TestPlannerAct_Give(t *testing.T) {
	f := newPlannerFixture(t)
	rule := f.withRule(t) // trained gives advanced
	a := services.Action{Op: services.OpGive, UserID: f.becky, ProjectID: f.project, RoleKey: f.trains}
	written := actStaleThenThePlan(t, f, a,
		services.ActWrite{Actor: f.actor, Reason: "capstone", Give: db.EnqueueParams{EndRule: db.EndKept, PayloadJSON: "{}"}},
		func() string { return f.queue(t, f.becky, "revoke", f.trains, "direct", "") })
	// Its own row waits (trained waits for you); the rule's goes straight away.
	if got := sentRows(t, f.ctx, outboxIDs(written)); !slices.Equal(got, []string{"add advanced rule " + rule, "add trained direct "}) {
		t.Fatalf("rows = %v", got)
	}
	var reason string
	if err := db.PG.QueryRow(f.ctx, `SELECT reason FROM direct_role_grants WHERE user_id = $1 AND zitadel_role_key = $2`, f.becky, f.trains).Scan(&reason); err != nil || reason != "capstone" {
		t.Fatalf("the ledger row keeps the operator's reason: %q %v", reason, err)
	}
	// And where the change is shown (task 1.8a): its audit line and every row
	// it queued, so Pending changes can quote it.
	if said := auditReason(t, f.ctx, f.becky, "direct_grant.upserted"); said != "capstone" {
		t.Errorf("the give's audit row says %q, want the operator's reason", said)
	}
	for _, id := range outboxIDs(written) {
		if _, why := rowWaitAndReason(t, f.ctx, id); why != "capstone" {
			t.Errorf("row %s carries %q, want the operator's reason", id, why)
		}
	}
}

func TestPlannerAct_RemoveGrant(t *testing.T) {
	f := newPlannerFixture(t)
	if _, err := db.EnqueueDirectGrantPropagation(f.ctx, db.EnqueueParams{UserID: f.becky, ProjectID: f.project,
		RoleKeys: []string{f.member}, GrantedBy: f.actor, OpType: "add", NoPropagation: true, PayloadJSON: "{}"}); err != nil {
		t.Fatal(err)
	}
	var grant string
	if err := db.PG.QueryRow(f.ctx, `SELECT id::text FROM direct_role_grants WHERE user_id = $1`, f.becky).Scan(&grant); err != nil {
		t.Fatal(err)
	}
	a := services.Action{Op: services.OpRemoveGrant, UserID: f.becky, GrantID: grant}
	// The give still waits to be sent: revoking it only cancels it.
	written := actStaleThenThePlan(t, f, a, services.ActWrite{Actor: f.actor},
		func() string { return f.queue(t, f.becky, "add", f.member, "direct", "") })
	if written.Delivery() != services.DeliveryNone || len(outboxIDs(written)) != 0 {
		t.Fatalf("a revoke that only cancels sends nothing: %+v", written.Writes)
	}
	var left bool
	_ = db.PG.QueryRow(f.ctx, `SELECT EXISTS (SELECT 1 FROM direct_role_grants WHERE id = $1::uuid)`, grant).Scan(&left)
	if left {
		t.Fatal("the grant is gone from the ledger")
	}
}

func TestPlannerAct_Approve(t *testing.T) {
	f := newPlannerFixture(t)
	days := 30
	req, err := db.CreateAccessRequest(f.ctx, f.becky, f.project, f.member, "induction done", &days)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.PG.Exec(f.ctx, `DELETE FROM access_requests WHERE id = $1::uuid`, req) })
	ctx, _ := db.GuardCancellations(f.ctx, nil, func(context.Context) bool { return true })

	// Approve stays on the planner from the decision down (decision 5).
	shown, err := services.PlanAct(ctx, services.Action{Op: services.OpApprove, UserID: f.becky, RequestID: req})
	if err != nil {
		t.Fatal(err)
	}
	unnamed := f.queue(t, f.becky, "revoke", f.member, "direct", "")
	if _, err := services.DecideRequest(ctx, req, "approved", f.actor, "ok",
		&services.Seen{Fingerprint: shown.Row.Fingerprint, CancelIDs: shown.Row.CancelIDs}); !errors.As(err, new(*services.StalePlan)) {
		t.Fatalf("stale, got %v", err)
	}
	var still string
	_ = db.PG.QueryRow(f.ctx, `SELECT status FROM access_requests WHERE id = $1::uuid`, req).Scan(&still)
	if still != "pending" || status(t, ctx, unnamed) != "pending" {
		t.Fatalf("a stale approval decides nothing: request %s", still)
	}

	fresh, err := services.PlanAct(ctx, services.Action{Op: services.OpApprove, UserID: f.becky, RequestID: req})
	if err != nil {
		t.Fatal(err)
	}
	d, err := services.DecideRequest(ctx, req, "approved", f.actor, "ok",
		&services.Seen{Fingerprint: fresh.Row.Fingerprint, CancelIDs: fresh.Row.CancelIDs})
	if err != nil {
		t.Fatal(err)
	}
	got, want := sentRows(t, ctx, outboxIDs(d.Change)), shownWrites(fresh)
	if strings.Join(got, "|") != strings.Join(want, "|") || !slices.Contains(got, "add "+f.member+" direct "+req) {
		t.Errorf("outbox %v, plan showed %v", got, want)
	}
	_ = db.PG.QueryRow(f.ctx, `SELECT status FROM access_requests WHERE id = $1::uuid`, req).Scan(&still)
	if still != "approved" || status(t, ctx, unnamed) != "superseded" || d.Approval.Effect != services.EffectGive {
		t.Fatalf("approved %s, the named removal %s, effect %s", still, status(t, ctx, unnamed), d.Approval.Effect)
	}
}

func TestPlannerAct_AssignBundle(t *testing.T) {
	f := newPlannerFixture(t)
	a := services.Action{Op: services.BulkOpAssignBundle, UserID: f.becky, BundleID: f.bundle}
	actStaleThenThePlan(t, f, a, services.ActWrite{Actor: f.actor},
		func() string { return f.queue(t, f.becky, "revoke", f.member, "direct", "") })
	var held bool
	_ = db.PG.QueryRow(f.ctx, `SELECT EXISTS (SELECT 1 FROM user_bundle_assignments WHERE user_id = $1 AND bundle_id = $2)`, f.becky, f.bundle).Scan(&held)
	if !held {
		t.Fatal("assigned")
	}
}

func TestPlannerAct_RemoveBundle(t *testing.T) {
	f := newPlannerFixture(t)
	v, err := db.LatestVersion(f.ctx, f.bundle)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok, err := db.AssignBundleAndEnqueue(f.ctx, f.actor, "", f.becky, f.bundle, v.ID, nil); err != nil || !ok {
		t.Fatalf("assign: %v %v", ok, err)
	}
	a := services.Action{Op: services.BulkOpRemoveBundle, UserID: f.becky, BundleID: f.bundle}
	actStaleThenThePlan(t, f, a, services.ActWrite{Actor: f.actor},
		func() string { return f.queue(t, f.becky, "add", f.member, "bundle", f.bundle) })
}
