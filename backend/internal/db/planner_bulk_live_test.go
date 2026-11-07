package db_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"syndra/internal/db"
	"syndra/internal/services"
)

// The rest of the access changes against a real database (task 1.3, decision
// 10): bulk give, remove and extend, a bulk decision's approvals, adopting
// drift, publishing and moving a bundle's holders. For each, a queued change
// the plan never named makes the apply stale with nothing written and the
// plan unspent; planned again, the outbox rows it writes are exactly the
// writes its plan showed, and only the cancellations it named happen.

// issueRows records a plan as the endpoints do, its rows keyed by subject.
func (f plannerFixture) issueRows(t *testing.T, ctx context.Context, surface, fp string, changes []services.Change) db.PlanCitation {
	t.Helper()
	subjects := make([]db.NewPlanSubject, 0, len(changes))
	for _, ch := range changes {
		subjects = append(subjects, db.NewPlanSubject{SubjectID: ch.Row.UserID, Person: ch.Row.Person, Fingerprint: ch.Row.Fingerprint,
			Outcome: db.PlanOutcome{Effect: ch.Row.Effect, CancelIDs: ch.Row.CancelIDs}})
	}
	plan, err := db.CreatePlan(ctx, db.NewPlan{Target: db.TargetZitadel, Surface: surface, CreatedBy: f.actor,
		Lifetime: 15 * time.Minute, StateReadAt: time.Now().UTC(), RequestFingerprint: fp, Subjects: subjects})
	if err != nil {
		t.Fatal(err)
	}
	return db.PlanCitation{PlanID: plan.ID, Target: db.TargetZitadel, Surface: surface, Actor: f.actor, RequestFingerprint: fp}
}

// bulkStaleThenThePlan runs one bulk op through both halves. queue puts in the
// change it would cancel, after the plan was read. It returns the Changes as
// written, by subject.
func bulkStaleThenThePlan(t *testing.T, f plannerFixture, req services.BulkRequest, surface string, queue func() string) map[string]services.Change {
	t.Helper()
	ctx, gate := db.GuardCancellations(f.ctx, nil, func(context.Context) bool { return true })
	plan := func() db.PlanCitation {
		changes, _, err := services.PlanBulk(ctx, req)
		if err != nil {
			t.Fatal(err)
		}
		for _, ch := range changes {
			if ch.Row.Effect != services.EffectApply {
				t.Fatalf("every row goes ahead: %+v", ch.Row)
			}
		}
		// A publish's and a move's approval binds what the snapshot says
		// (their contents and cohort); the rest bind the request.
		fp := services.FingerprintBulkRequest(req)
		if req.Op == services.OpPublish || req.Op == services.OpMove {
			preview, err := services.RehearseBulk(ctx, req)
			if err != nil {
				t.Fatal(err)
			}
			fp = preview.RequestFingerprint
		}
		return f.issueRows(t, ctx, surface, fp, changes)
	}

	stale := plan()
	unnamed := queue()
	before := len(sentRowsFor(t, ctx, f.alice))
	if _, _, err := services.ApplyPlannedBulk(ctx, req, stale); !errors.As(err, new(*services.StalePlan)) {
		t.Fatalf("a change queued since the plan was read makes it stale, got %v", err)
	}
	if s := status(t, ctx, unnamed); s != "pending" {
		t.Errorf("the change nobody was shown is untouched, got %s", s)
	}
	if after := len(sentRowsFor(t, ctx, f.alice)); after != before {
		t.Fatalf("a stale plan wrote %d outbox row(s)", after-before)
	}

	changes, _, err := services.PlanBulk(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	_, written, err := services.ApplyPlannedBulk(ctx, req, plan())
	if err != nil {
		t.Fatal(err)
	}
	if len(gate.Refused()) != 0 {
		t.Fatalf("a named cancellation was refused: %+v", gate.Refused())
	}
	named := false
	for _, ch := range changes {
		got, want := sentRows(t, ctx, outboxIDs(written[ch.Row.UserID])), shownWrites(ch)
		if strings.Join(got, "|") != strings.Join(want, "|") {
			t.Errorf("%s: outbox %v, plan showed %v", ch.Row.UserID, got, want)
		}
		named = named || slices.Contains(ch.Row.CancelIDs, unnamed)
	}
	if !named {
		t.Fatalf("the fresh plan names the change it cancels")
	}
	if s := status(t, ctx, unnamed); s != "superseded" {
		t.Errorf("the now-named change is cancelled, got %s", s)
	}
	return written
}

func TestPlannerBulk_AssignRole(t *testing.T) {
	f := newPlannerFixture(t)
	req := services.BulkRequest{Op: services.BulkOpAssignRole, UserIDs: []string{f.alice}, ProjectID: f.project, RoleKey: f.member,
		DurationDays: 30, Reason: "term start", Actor: f.actor}
	written := bulkStaleThenThePlan(t, f, req, "grants.bulk",
		func() string { return f.queue(t, f.alice, "revoke", f.member, "direct", "") })
	var ends *time.Time
	if err := db.PG.QueryRow(f.ctx, `SELECT expires_at FROM direct_role_grants WHERE user_id = $1 AND zitadel_role_key = $2`,
		f.alice, f.member).Scan(&ends); err != nil || ends == nil {
		t.Fatalf("the give is on the ledger with its end: %v %v", ends, err)
	}
	_ = written
}

func TestPlannerBulk_RemoveRole(t *testing.T) {
	f := newPlannerFixture(t)
	if _, err := db.EnqueueDirectGrantPropagation(f.ctx, db.EnqueueParams{UserID: f.alice, ProjectID: f.project,
		RoleKeys: []string{f.member}, GrantedBy: f.actor, OpType: "add", NoPropagation: true, PayloadJSON: "{}"}); err != nil {
		t.Fatal(err)
	}
	req := services.BulkRequest{Op: services.BulkOpRemoveRole, UserIDs: []string{f.alice}, ProjectID: f.project, RoleKey: f.member,
		Reason: "left the course", Actor: f.actor}
	bulkStaleThenThePlan(t, f, req, "grants.bulk",
		func() string { return f.queue(t, f.alice, "add", f.member, "direct", "") })
	var left bool
	_ = db.PG.QueryRow(f.ctx, `SELECT EXISTS (SELECT 1 FROM direct_role_grants WHERE user_id = $1)`, f.alice).Scan(&left)
	if left {
		t.Fatal("the grant is gone from the ledger")
	}
}

// Extending is a ledger edit (settled 5 Nov): no outbox row, unless the new
// end cancels a removal still waiting, which it names.
func TestPlannerBulk_ExtendWritesNoOutboxRow(t *testing.T) {
	f := newPlannerFixture(t)
	soon := time.Now().Add(3 * 24 * time.Hour)
	if _, err := db.EnqueueDirectGrantPropagation(f.ctx, db.EnqueueParams{UserID: f.alice, ProjectID: f.project,
		RoleKeys: []string{f.member}, GrantedBy: f.actor, OpType: "add", NoPropagation: true, ExpiresAt: &soon, PayloadJSON: "{}"}); err != nil {
		t.Fatal(err)
	}
	ctx, _ := db.GuardCancellations(f.ctx, nil, func(context.Context) bool { return true })
	req := services.BulkRequest{Op: services.BulkOpExtend, UserIDs: []string{f.alice}, DurationDays: 30, Reason: "term", Actor: f.actor}
	changes, _, err := services.PlanBulk(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	before := len(sentRowsFor(t, ctx, f.alice))
	_, written, err := services.ApplyPlannedBulk(ctx, req, f.issueRows(t, ctx, "grants.bulk", services.FingerprintBulkRequest(req), changes))
	if err != nil {
		t.Fatal(err)
	}
	if after := len(sentRowsFor(t, ctx, f.alice)); after != before || len(outboxIDs(written[f.alice])) != 0 {
		t.Fatalf("an extension sends nothing to Zitadel: %d new row(s)", after-before)
	}
	var ends time.Time
	if err := db.PG.QueryRow(f.ctx, `SELECT expires_at FROM direct_role_grants WHERE user_id = $1`, f.alice).Scan(&ends); err != nil ||
		!ends.After(soon.Add(24*time.Hour)) {
		t.Fatalf("the ledger's end moved: %v %v", ends, err)
	}

	// The one case it reaches the outbox: a removal still waiting is
	// cancelled, and named.
	bulkStaleThenThePlan(t, f, services.BulkRequest{Op: services.BulkOpExtend, UserIDs: []string{f.alice}, DurationDays: 60,
		Reason: "term", Actor: f.actor}, "grants.bulk",
		func() string { return f.queue(t, f.alice, "revoke", f.member, "direct", "") })
}

func TestPlannerBulk_Approve(t *testing.T) {
	f := newPlannerFixture(t)
	days := 30
	req1, err := db.CreateAccessRequest(f.ctx, f.alice, f.project, f.member, "induction done", &days)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.PG.Exec(f.ctx, `DELETE FROM access_requests WHERE id = $1::uuid`, req1) })
	req := services.BulkRequest{Op: services.OpApprove, UserIDs: []string{req1}, Actor: f.actor, Note: "ok"}
	bulkStaleThenThePlan(t, f, req, "requests.bulk_decision",
		func() string { return f.queue(t, f.alice, "revoke", f.member, "direct", "") })
	var st string
	_ = db.PG.QueryRow(f.ctx, `SELECT status FROM access_requests WHERE id = $1::uuid`, req1).Scan(&st)
	if st != "approved" {
		t.Fatalf("approved, got %s", st)
	}
	var audited int
	_ = db.PG.QueryRow(f.ctx, `SELECT count(*) FROM audit_logs WHERE target_zitadel_user_id = $1 AND action = 'access_request.approved'`, f.alice).Scan(&audited)
	if audited != 1 {
		t.Fatalf("one decision audited, got %d", audited)
	}
}

func TestPlannerBulk_Adopt(t *testing.T) {
	f := newPlannerFixture(t)
	rule := f.withRule(t) // trained gives advanced
	drift, _, err := db.UpsertDriftItem(f.ctx, db.TargetZitadel, f.alice, f.project, []string{f.trains}, "zg-1", "reconciliation_sweep", db.DriftTargetOnly)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.PG.Exec(f.ctx, `DELETE FROM drift_items WHERE id = $1::uuid`, drift) })
	req := services.BulkRequest{Op: services.OpAdopt, UserIDs: []string{drift}, Actor: f.actor}
	written := bulkStaleThenThePlan(t, f, req, "drift.bulk_attribute",
		func() string { return f.queue(t, f.alice, "revoke", "advanced", "rule", rule) })
	// The adopted role is a ledger row and nothing else; the rule's row is the rule's.
	if got := sentRows(t, f.ctx, outboxIDs(written[drift])); slices.ContainsFunc(got, func(s string) bool { return strings.HasPrefix(s, "add trained") }) {
		t.Fatalf("no outbox row for the adopted role: %v", got)
	}
	var st string
	_ = db.PG.QueryRow(f.ctx, `SELECT status FROM drift_items WHERE id = $1::uuid`, drift).Scan(&st)
	if st != "attributed" {
		t.Fatalf("the row is adopted, got %s", st)
	}
}

// publishWithAdvanced puts Alice on v1 and adds "advanced" to the working copy.
func (f plannerFixture) publishWithAdvanced(t *testing.T) {
	t.Helper()
	v, err := db.LatestVersion(f.ctx, f.bundle)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok, err := db.AssignBundleAndEnqueue(f.ctx, f.actor, "", f.alice, f.bundle, v.ID, nil); err != nil || !ok {
		t.Fatalf("assign: %v %v", ok, err)
	}
	if _, err := db.AddRoleToBundleAndEnqueue(f.ctx, f.actor, f.bundle, f.project, "advanced", nil); err != nil {
		t.Fatal(err)
	}
}

func TestPlannerBulk_Publish(t *testing.T) {
	f := newPlannerFixture(t)
	f.publishWithAdvanced(t)
	req := services.BulkRequest{Op: services.OpPublish, BundleID: f.bundle, Migrate: true, Note: "adds advanced", Actor: f.actor}
	bulkStaleThenThePlan(t, f, req, "bundles.publish",
		func() string { return f.queue(t, f.alice, "revoke", "advanced", "direct", "") })
}

func TestPlannerBulk_Move(t *testing.T) {
	f := newPlannerFixture(t)
	f.publishWithAdvanced(t)
	working, err := db.GetRolesForBundle(f.ctx, f.bundle)
	if err != nil {
		t.Fatal(err)
	}
	v2, _, err := db.PublishVersionAndEnqueue(f.ctx, f.actor, f.bundle, "v2", working, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	req := services.BulkRequest{Op: services.OpMove, BundleID: f.bundle, VersionID: v2.ID, UserIDs: []string{f.alice}, Actor: f.actor}
	bulkStaleThenThePlan(t, f, req, "bundles.move_holders",
		func() string { return f.queue(t, f.alice, "revoke", "advanced", "direct", "") })
}
