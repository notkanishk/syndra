package db_test

import (
	"context"
	"errors"
	"maps"
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
	g := f.grant(t, f.ember, f.trains, &ended)
	rule := f.withRule(t) // trained gives advanced
	f.waitsToo(t, "advanced")
	own := f.queue(t, f.ember, "add", f.trains, "direct", "")
	other := f.queue(t, f.brock, "add", f.trains, "direct", "")

	res, err := services.ExpireDirectGrant(f.ctx,
		models.DirectGrant{ID: g, UserID: f.ember, ProjectID: f.project, RoleKey: f.trains, ExpiresAt: &ended}, "system:scheduler")
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
	g2 := f.grant(t, f.brock, f.member, &later)
	before := sentRowsFor(t, f.ctx, f.brock)
	if _, err := services.ExpireDirectGrant(f.ctx,
		models.DirectGrant{ID: g2, UserID: f.brock, ProjectID: f.project, RoleKey: f.member, ExpiresAt: &ended}, "system:scheduler"); !errors.Is(err, db.ErrGrantRenewed) {
		t.Fatalf("a renewed grant is ErrGrantRenewed, got %v", err)
	}
	if rows := newRows(t, f.ctx, f.brock, before); len(rows) != 0 {
		t.Fatalf("a renewed grant writes nothing, got %v", rows)
	}
}

// The welcome bundle on a first sign-in: the bundle's rows and the rule's,
// each as its role says, sent straight away unless it waits. It cancels the
// waiting removal of a role it gives (planned), and nothing of anyone else's.
func TestPlannerSystem_WelcomeBundle(t *testing.T) {
	f := newPlannerFixture(t)
	rule := f.withRule(t)
	removal := f.queue(t, f.ember, "revoke", f.member, "direct", "")
	other := f.queue(t, f.brock, "revoke", f.member, "direct", "")
	before := sentRowsFor(t, f.ctx, f.ember)

	res, err := services.CascadeBundleAssignedToUser(f.ctx, db.WelcomeActor, f.ember, f.bundle)
	if err != nil {
		t.Fatal(err)
	}
	rows := newRows(t, f.ctx, f.ember, before)
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
	f.grant(t, f.ember, f.trains, nil)
	rule := f.withRule(t)
	queued := f.queue(t, f.ember, "add", "advanced", "rule", rule)
	old := models.MappingRule{ID: rule, SourceProject: f.project, SourceRole: f.trains, TargetProject: f.project, TargetRole: "advanced"}
	operator := func(context.Context) bool { return true }

	ctx, gate := db.GuardCancellations(f.ctx, nil, operator)
	if _, err := services.CascadeRuleRetired(ctx, f.actor, old); !errors.Is(err, db.ErrWouldCancel) {
		t.Fatalf("an unshown cancellation refuses the edit, got %v", err)
	}
	if len(gate.Refused()) != 1 || gate.Refused()[0].ID != queued || status(t, f.ctx, queued) != "pending" {
		t.Fatalf("refused, naming the give, with nothing written: %+v", gate.Refused())
	}

	before := sentRowsFor(t, f.ctx, f.ember)
	ctx, _ = db.GuardCancellations(f.ctx, []string{queued}, operator)
	if _, err := services.CascadeRuleRetired(ctx, f.actor, old); err != nil {
		t.Fatal(err)
	}
	if status(t, f.ctx, queued) != "superseded" {
		t.Errorf("the shown give is cancelled")
	}
	if rows := newRows(t, f.ctx, f.ember, before); len(rows) != 0 {
		t.Errorf("advanced was never sent, so nothing is owed: %v", sentRows(t, f.ctx, rows))
	}
}

// A rule created in a chain (task 1.6a): each row names the rule one step
// before its role and keeps the role that set it off; a later edit to the
// rule's first role renames neither Pending changes nor Change history; and
// retiring the rule still cancels the gives it queued, as before.
func TestPlannerSystem_RuleChainNamesEachHop(t *testing.T) {
	f := newPlannerFixture(t)
	f.grant(t, f.ember, f.member, nil)
	r2 := f.withRule(t) // trained gives advanced
	operator := func(context.Context) bool { return true }

	before := sentRowsFor(t, f.ctx, f.ember)
	ctx, _ := db.GuardCancellations(f.ctx, nil, operator)
	r1, _, err := services.CascadeRuleCreated(ctx, f.actor, f.project, f.member, f.project, f.trains)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.PG.Exec(f.ctx, `DELETE FROM mapping_rules WHERE id = $1::uuid`, r1) })
	rows := newRows(t, f.ctx, f.ember, before)
	if got, want := sentRows(t, f.ctx, rows), []string{"add advanced rule " + r2, "add trained rule " + r1}; !slices.Equal(got, want) {
		t.Fatalf("create wrote %v, want %v (advanced is the next rule's row, not the new rule's)", got, want)
	}

	// setOff is what Change history and Pending changes say set each row off.
	setOff := func() (string, map[string]string) {
		var cascade string
		if err := db.PG.QueryRow(f.ctx, `SELECT cascade_id::text FROM propagation_outbox WHERE id = $1::uuid`, rows[0]).Scan(&cascade); err != nil {
			t.Fatal(err)
		}
		groups, err := db.GetCascadeGroups(f.ctx, 0, cascade)
		if err != nil || len(groups) != 1 {
			t.Fatalf("Change history: %+v %v", groups, err)
		}
		from := map[string]string{}
		for _, w := range groups[0].Writes {
			if w.RuleFrom != nil {
				from["history "+w.RoleKeys[0]] = w.RuleFrom.RoleKey
			}
		}
		pending, err := db.GetPendingPropagations(f.ctx)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range pending {
			if p.UserID == f.ember && p.RuleFrom != nil {
				from["pending "+p.RoleKeys[0]] = p.RuleFrom.RoleKey
			}
		}
		return groups[0].SourceRef, from
	}
	want := map[string]string{"history advanced": f.trains, "history trained": f.member, "pending advanced": f.trains, "pending trained": f.member}
	if named, from := setOff(); named != r1 || !maps.Equal(from, want) {
		t.Fatalf("the entry is the edited rule's (%s, want %s); each row names its own hop: %v, want %v", named, r1, from, want)
	}

	// The rule's first role edited afterwards: the rows still say member.
	if _, err := db.UpdateMappingRuleAndEnqueue(f.ctx, f.actor, r1, f.project, "visitor", f.project, f.trains, nil); err != nil {
		t.Fatal(err)
	}
	if _, from := setOff(); !maps.Equal(from, want) {
		t.Fatalf("an edit to the rule renamed its rows: %v, want %v", from, want)
	}
	if _, err := db.UpdateMappingRuleAndEnqueue(f.ctx, f.actor, r1, f.project, f.member, f.project, f.trains, nil); err != nil {
		t.Fatal(err)
	}

	// Retired: a rule's revoke answers any queued rule give of its role, so
	// both gives are cancelled, the hop's too, and nothing is owed.
	before = sentRowsFor(t, f.ctx, f.ember)
	ctx, _ = db.GuardCancellations(f.ctx, rows, operator)
	old := models.MappingRule{ID: r1, SourceProject: f.project, SourceRole: f.member, TargetProject: f.project, TargetRole: f.trains}
	if _, err := services.CascadeRuleRetired(ctx, f.actor, old); err != nil {
		t.Fatal(err)
	}
	for _, id := range rows {
		if s := status(t, f.ctx, id); s != "superseded" {
			t.Errorf("queued give %s is %s, want superseded", id, s)
		}
	}
	if got := newRows(t, f.ctx, f.ember, before); len(got) != 0 {
		t.Errorf("nothing was ever sent, so nothing is owed: %v", sentRows(t, f.ctx, got))
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
	if _, _, err := db.AssignBundleAndEnqueue(f.ctx, f.actor, "", f.ember, f.bundle, v.ID, nil); err != nil {
		t.Fatal(err)
	}
	before := sentRowsFor(t, f.ctx, f.ember)
	ctx, _ := db.GuardCancellations(f.ctx, nil, func(context.Context) bool { return true })
	res, err := services.CascadeBundleRetired(ctx, f.actor, f.bundle)
	if err != nil {
		t.Fatal(err)
	}
	rows := newRows(t, f.ctx, f.ember, before)
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
	drift, _, err := db.UpsertDriftItem(f.ctx, db.TargetZitadel, f.ember, f.project, []string{f.member, f.trains}, "zg-1", "reconciliation_sweep", db.DriftTargetOnly)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.PG.Exec(f.ctx, `DELETE FROM drift_items WHERE id = $1::uuid`, drift) })

	written := actStaleThenThePlan(t, f, services.Action{Op: services.OpRevoke, UserID: f.ember, DriftID: drift},
		services.ActWrite{Actor: f.actor},
		func() string { return f.queue(t, f.ember, "add", f.member, "direct", "") })
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
	written := actStaleThenThePlan(t, f, services.Action{Op: services.OpRevoke, UserID: f.ember, ProjectID: f.project, RoleKey: "stray"},
		services.ActWrite{Actor: f.actor},
		func() string { return f.queue(t, f.ember, "add", "stray", "direct", "") })
	if written.Delivery() != services.DeliveryNone {
		t.Fatalf("a revoke of a give that never went only cancels it: %+v", written.Writes)
	}
}
