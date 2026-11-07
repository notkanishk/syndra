package db_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"syndra/internal/db"
	"syndra/internal/models"
	"syndra/internal/services"
)

// System work through the planner against a real database (task 1.9): the
// outbox rows each op writes are the writes its plan decided, each written
// waiting or not as its role says, and it cancels only the queued changes its
// plan named. Zitadel is never reached: with no client the send halts at once
// (zitadel_offline) and the rows meant to go are scheduled to retry.

// grant records a direct grant on the ledger only, ending at ends (nil: none).
func (f plannerFixture) grant(t *testing.T, user, role string, ends *time.Time) string {
	t.Helper()
	if _, err := db.EnqueueDirectGrantPropagation(f.ctx, db.EnqueueParams{UserID: user, ProjectID: f.project,
		RoleKeys: []string{role}, GrantedBy: f.actor, OpType: "add", NoPropagation: true, PayloadJSON: "{}"}); err != nil {
		t.Fatal(err)
	}
	var id string
	if err := db.PG.QueryRow(f.ctx, `UPDATE direct_role_grants SET created_at = LEAST(created_at, COALESCE($3, created_at) - interval '1 day'), expires_at = $3
		WHERE user_id = $1 AND zitadel_role_key = $2 RETURNING id::text`, user, role, ends).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

// newRows is user's outbox rows written since before (a set of ids).
func newRows(t *testing.T, ctx context.Context, user string, before []string) []string {
	t.Helper()
	var out []string
	for _, id := range sentRowsFor(t, ctx, user) {
		if !slices.Contains(before, id) {
			out = append(out, id)
		}
	}
	return out
}

// waitsOf is each row's wait_reason, by "op role".
func waitsOf(t *testing.T, ctx context.Context, ids []string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, id := range ids {
		var key, wait string
		if err := db.PG.QueryRow(ctx, `SELECT op_type || ' ' || role_keys[1], COALESCE(wait_reason, '')
			FROM propagation_outbox WHERE id = $1::uuid`, id).Scan(&key, &wait); err != nil {
			t.Fatal(err)
		}
		out[key] = wait
	}
	return out
}

func (f plannerFixture) waitsToo(t *testing.T, role string) {
	t.Helper()
	if _, err := db.MarkRoleWaits(f.ctx, f.project, role, f.actor); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.UnmarkRoleWaits(f.ctx, f.project, role) })
}

// Expiry: the grant's own waiting give is the one queued change it cancels
// (sending it later would restore access that ended); another person's is
// untouched. Its rows are the plan's, written waiting where the role waits.
func TestPlannerSystem_Expiry(t *testing.T) {
	f := newPlannerFixture(t)
	ended := time.Now().Add(-time.Hour)
	g := f.grant(t, f.alice, f.trains, &ended)
	rule := f.withRule(t) // trained gives advanced
	f.waitsToo(t, "advanced")
	own := f.queue(t, f.alice, "add", f.trains, "direct", "")
	other := f.queue(t, f.bob, "add", f.trains, "direct", "")

	res, err := services.ExpireDirectGrant(f.ctx,
		models.DirectGrant{ID: g, UserID: f.alice, ProjectID: f.project, RoleKey: f.trains, ExpiresAt: &ended}, "system:scheduler")
	if err != nil {
		t.Fatal(err)
	}
	got, want := sentRows(t, f.ctx, outboxIDs(res.Change)), shownWrites(res.Change)
	if strings.Join(got, "|") != strings.Join(want, "|") || strings.Join(want, "|") != "revoke advanced rule "+rule {
		t.Fatalf("outbox %v, plan %v", got, want)
	}
	if w := waitsOf(t, f.ctx, outboxIDs(res.Change)); w["revoke advanced"] != db.WaitReasonRoleWaits {
		t.Errorf("advanced waits for you, as planned: %v", w)
	}
	if s := status(t, f.ctx, own); s != "superseded" {
		t.Errorf("her own waiting give is cancelled, got %s", s)
	}
	if s := status(t, f.ctx, other); s != "pending" {
		t.Errorf("a change it didn't plan is untouched, got %s", s)
	}
	var left bool
	_ = db.PG.QueryRow(f.ctx, `SELECT EXISTS (SELECT 1 FROM direct_role_grants WHERE id = $1::uuid)`, g).Scan(&left)
	if left {
		t.Fatal("the ended grant is gone from the ledger")
	}

	// A grant whose end is still ahead is not ended, and nothing is written.
	later := time.Now().Add(time.Hour)
	g2 := f.grant(t, f.bob, f.member, &later)
	before := sentRowsFor(t, f.ctx, f.bob)
	if _, err := services.ExpireDirectGrant(f.ctx,
		models.DirectGrant{ID: g2, UserID: f.bob, ProjectID: f.project, RoleKey: f.member, ExpiresAt: &ended}, "system:scheduler"); !errors.Is(err, db.ErrGrantRenewed) {
		t.Fatalf("a renewed grant is ErrGrantRenewed, got %v", err)
	}
	if rows := newRows(t, f.ctx, f.bob, before); len(rows) != 0 {
		t.Fatalf("a renewed grant writes nothing, got %v", rows)
	}
}

// The welcome bundle on a first sign-in: the bundle's rows and the rule's,
// each as its role says, sent straight away unless it waits. It cancels the
// waiting removal of a role it gives (planned), and nothing of anyone else's.
func TestPlannerSystem_WelcomeBundle(t *testing.T) {
	f := newPlannerFixture(t)
	rule := f.withRule(t)
	removal := f.queue(t, f.alice, "revoke", f.member, "direct", "")
	other := f.queue(t, f.bob, "revoke", f.member, "direct", "")
	before := sentRowsFor(t, f.ctx, f.alice)

	res, err := services.CascadeBundleAssignedToUser(f.ctx, db.WelcomeActor, f.alice, f.bundle)
	if err != nil {
		t.Fatal(err)
	}
	rows := newRows(t, f.ctx, f.alice, before)
	want := []string{"add advanced rule " + rule, "add member bundle " + f.bundle, "add trained bundle " + f.bundle}
	if got := sentRows(t, f.ctx, rows); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("welcome wrote %v, want %v", got, want)
	}
	w := waitsOf(t, f.ctx, rows)
	if w["add trained"] != db.WaitReasonRoleWaits || w["add member"] == db.WaitReasonRoleWaits {
		t.Errorf("trained waits, member was sent: %v", w)
	}
	if res.Waiting != 1 || res.WaitReason != db.WaitReasonRoleWaits {
		t.Errorf("the result counts what waits: %+v", res)
	}
	if status(t, f.ctx, removal) != "superseded" || status(t, f.ctx, other) != "pending" {
		t.Errorf("cancels only the removal it planned: %s, %s", status(t, f.ctx, removal), status(t, f.ctx, other))
	}
}

// A rule's retirement is the operator's: it cancels only what the operator
// was shown. Shown nothing, the rule's own waiting give stops it with nothing
// written; shown that give, it cancels it and has nothing left to send.
func TestPlannerSystem_RuleRetired(t *testing.T) {
	f := newPlannerFixture(t)
	f.grant(t, f.alice, f.trains, nil)
	rule := f.withRule(t)
	queued := f.queue(t, f.alice, "add", "advanced", "rule", rule)
	old := models.MappingRule{ID: rule, SourceProject: f.project, SourceRole: f.trains, TargetProject: f.project, TargetRole: "advanced"}
	operator := func(context.Context) bool { return true }

	ctx, gate := db.GuardCancellations(f.ctx, nil, operator)
	if _, err := services.CascadeRuleRetired(ctx, f.actor, old); !errors.Is(err, db.ErrWouldCancel) {
		t.Fatalf("an unshown cancellation refuses the edit, got %v", err)
	}
	if len(gate.Refused()) != 1 || gate.Refused()[0].ID != queued || status(t, f.ctx, queued) != "pending" {
		t.Fatalf("refused, naming the give, with nothing written: %+v", gate.Refused())
	}

	before := sentRowsFor(t, f.ctx, f.alice)
	ctx, _ = db.GuardCancellations(f.ctx, []string{queued}, operator)
	if _, err := services.CascadeRuleRetired(ctx, f.actor, old); err != nil {
		t.Fatal(err)
	}
	if status(t, f.ctx, queued) != "superseded" {
		t.Errorf("the shown give is cancelled")
	}
	if rows := newRows(t, f.ctx, f.alice, before); len(rows) != 0 {
		t.Errorf("advanced was never sent, so nothing is owed: %v", sentRows(t, f.ctx, rows))
	}
}

// A bundle retired: each holder's rows are what only it gave, waiting as
// their roles say.
func TestPlannerSystem_BundleRetired(t *testing.T) {
	f := newPlannerFixture(t)
	v, _, err := db.LatestVersionRoles(f.ctx, f.bundle)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := db.AssignBundleAndEnqueue(f.ctx, f.actor, "", f.alice, f.bundle, v.ID, nil); err != nil {
		t.Fatal(err)
	}
	before := sentRowsFor(t, f.ctx, f.alice)
	ctx, _ := db.GuardCancellations(f.ctx, nil, func(context.Context) bool { return true })
	res, err := services.CascadeBundleRetired(ctx, f.actor, f.bundle)
	if err != nil {
		t.Fatal(err)
	}
	rows := newRows(t, f.ctx, f.alice, before)
	want := []string{"revoke member bundle " + f.bundle, "revoke trained bundle " + f.bundle}
	if got := sentRows(t, f.ctx, rows); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("retire wrote %v, want %v", got, want)
	}
	if w := waitsOf(t, f.ctx, rows); w["revoke trained"] != db.WaitReasonRoleWaits || res.Waiting != 1 {
		t.Errorf("trained's removal waits for you: %v %+v", w, res)
	}
}

// Drift › Revoke through the planner: stale with nothing written when a
// change was queued since the plan was read; planned again, one row per role
// as decided, and the drift row resolved.
func TestPlannerSystem_DriftRevoke(t *testing.T) {
	f := newPlannerFixture(t)
	drift, _, err := db.UpsertDriftItem(f.ctx, db.TargetZitadel, f.alice, f.project, []string{f.member, f.trains}, "zg-1", "reconciliation_sweep", db.DriftTargetOnly)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.PG.Exec(f.ctx, `DELETE FROM drift_items WHERE id = $1::uuid`, drift) })

	written := actStaleThenThePlan(t, f, services.Action{Op: services.OpRevoke, UserID: f.alice, DriftID: drift},
		services.ActWrite{Actor: f.actor},
		func() string { return f.queue(t, f.alice, "add", f.member, "direct", "") })
	// member's give never went, so its revoke only cancels it; trained's waits.
	if got := sentRows(t, f.ctx, outboxIDs(written)); !slices.Equal(got, []string{"revoke trained direct "}) {
		t.Fatalf("rows = %v", got)
	}
	var st string
	_ = db.PG.QueryRow(f.ctx, `SELECT status FROM drift_items WHERE id = $1::uuid`, drift).Scan(&st)
	if st != "revoked" {
		t.Fatalf("the drift row is resolved as revoked, got %s", st)
	}
}

// Zitadel › Users revoke of a role Syndra gives no way: the same planned act.
func TestPlannerSystem_UsersRevoke(t *testing.T) {
	f := newPlannerFixture(t)
	written := actStaleThenThePlan(t, f, services.Action{Op: services.OpRevoke, UserID: f.alice, ProjectID: f.project, RoleKey: "stray"},
		services.ActWrite{Actor: f.actor},
		func() string { return f.queue(t, f.alice, "add", "stray", "direct", "") })
	if written.Delivery() != services.DeliveryNone {
		t.Fatalf("a revoke of a give that never went only cancels it: %+v", written.Writes)
	}
}
