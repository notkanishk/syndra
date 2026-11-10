package db_test

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"
	"time"

	"syndra/internal/db"
	"syndra/internal/directory"
	"syndra/internal/models"
	"syndra/internal/services"
)

// The access-change planner against a real database (architecture
// deepening, decision 10): for each action, the outbox rows its apply writes
// are exactly the writes its plan showed, and a queued change the plan never
// named makes it stale with nothing written. Zitadel is never called: the
// apply only writes the outbox; sending is the handler's, after the commit.

// plannerDirectory is the directory the plan reads: the fixture's people and
// one project offering the bundle's roles. Anything else panics, which is the
// point: the planner must read nothing more from Zitadel.
type plannerDirectory struct {
	directory.Source
	users   []models.UserProfile
	project models.ProjectCatalog
}

func (d plannerDirectory) Users(context.Context) ([]models.UserProfile, error) { return d.users, nil }
func (d plannerDirectory) FindUser(_ context.Context, id string) (models.UserProfile, bool, error) {
	for _, u := range d.users {
		if u.ID == id {
			return u, true, nil
		}
	}
	return models.UserProfile{}, false, nil
}
func (d plannerDirectory) Projects(context.Context) ([]models.ProjectCatalog, error) {
	return []models.ProjectCatalog{d.project}, nil
}

type plannerFixture struct {
	ctx            context.Context
	project        string
	bundle         string
	becky, bob     string
	actor          string
	member, trains string
}

func newPlannerFixture(t *testing.T) plannerFixture {
	t.Helper()
	ctx := db.LiveDB(t)
	sfx := time.Now().Format("150405.000000000")
	f := plannerFixture{ctx: ctx, project: "plan-p-" + sfx, becky: "plan-a-" + sfx, bob: "plan-b-" + sfx,
		actor: "op@example.org", member: "member", trains: "trained"}

	var err error
	f.bundle, err = db.CreateBundle(ctx, "Planner "+sfx, "", []models.BundleRole{
		{ProjectID: f.project, RoleKey: f.member}, {ProjectID: f.project, RoleKey: f.trains},
	})
	if err != nil {
		t.Fatal(err)
	}
	// Giving "trained" waits for you; "member" goes straight away.
	if _, err := db.MarkRoleWaits(ctx, f.project, f.trains, f.actor); err != nil {
		t.Fatal(err)
	}
	people := []string{f.becky, f.bob}
	t.Cleanup(func() {
		_, _ = db.PG.Exec(ctx, `DELETE FROM user_bundle_assignments WHERE bundle_id = $1`, f.bundle)
		_, _ = db.PG.Exec(ctx, `DELETE FROM bundles WHERE id = $1`, f.bundle)
		_, _ = db.UnmarkRoleWaits(ctx, f.project, f.trains)
		_, _ = db.PG.Exec(ctx, `DELETE FROM direct_role_grants WHERE user_id = ANY($1)`, people)
		_, _ = db.PG.Exec(ctx, `DELETE FROM audit_logs WHERE target_zitadel_user_id = ANY($1)`, people)
	})

	origDir, origCat := directory.Default, services.PlannerCatalogue
	t.Cleanup(func() { directory.Default, services.PlannerCatalogue = origDir, origCat })
	dir := plannerDirectory{
		users: []models.UserProfile{{ID: f.becky, Name: "Ember Moon", Status: "active"}, {ID: f.bob, Name: "Brock Lesnar", Status: "active"}},
		project: models.ProjectCatalog{ID: f.project, Name: "Laser Lab", Roles: []models.ProjectRole{
			{Key: f.member, Label: "Member"}, {Key: f.trains, Label: "Trained"}, {Key: "advanced", Label: "Advanced"},
		}},
	}
	directory.Default = dir
	services.PlannerCatalogue = dir.Projects
	return f
}

func (f plannerFixture) request(op string, people ...string) services.BulkRequest {
	return services.BulkRequest{Op: op, UserIDs: people, BundleID: f.bundle, Reason: "term start", Actor: f.actor}
}

// issue records the plan the way the bulk endpoint does, and returns its citation.
func (f plannerFixture) issue(t *testing.T, ctx context.Context, req services.BulkRequest, changes []services.Change) db.PlanCitation {
	t.Helper()
	subjects := make([]db.NewPlanSubject, 0, len(changes))
	for _, ch := range changes {
		subjects = append(subjects, db.NewPlanSubject{SubjectID: ch.UserID, Fingerprint: ch.Row.Fingerprint,
			Outcome: db.PlanOutcome{Effect: ch.Row.Effect, CancelIDs: ch.Row.CancelIDs}})
	}
	fp := services.FingerprintBulkRequest(req)
	plan, err := db.CreatePlan(ctx, db.NewPlan{Target: db.TargetZitadel, Surface: "grants.bulk", CreatedBy: f.actor,
		Lifetime: 15 * time.Minute, StateReadAt: time.Now().UTC(), RequestFingerprint: fp, Subjects: subjects})
	if err != nil {
		t.Fatal(err)
	}
	return db.PlanCitation{PlanID: plan.ID, Target: db.TargetZitadel, Surface: "grants.bulk", Actor: f.actor, RequestFingerprint: fp}
}

// queue puts a change in the outbox waiting to be sent, as an earlier act left it.
func (f plannerFixture) queue(t *testing.T, user, op, role, source, ref string) string {
	t.Helper()
	var id string
	if err := db.PG.QueryRow(f.ctx, `
		INSERT INTO propagation_outbox (op_type, user_id, project_id, role_keys, payload_json, idempotency_key,
		                                initiated_by, source, source_ref, target)
		VALUES ($1, $2, $3, ARRAY[$4], '{}', gen_random_uuid(), $5, $6, NULLIF($7, ''), 'zitadel')
		RETURNING id::text`, op, user, f.project, role, f.actor, source, ref).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

// sentRows is the outbox rows ids name, as "op role source ref".
func sentRows(t *testing.T, ctx context.Context, ids []string) []string {
	t.Helper()
	rows, err := db.PG.Query(ctx, `
		SELECT op_type || ' ' || role_keys[1] || ' ' || source || ' ' || COALESCE(source_ref::text, '')
		  FROM propagation_outbox WHERE id = ANY($1::text[]::uuid[]) AND status = 'pending'`, ids)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// shownWrites is what the plan said goes to the outbox: every write but one
// that only cancels a queued change.
func shownWrites(ch services.Change) []string {
	var out []string
	for _, w := range ch.Writes {
		if w.Delivery != services.DeliveryNone {
			out = append(out, w.OpType+" "+w.RoleKeys[0]+" "+w.Source+" "+w.SourceRef)
		}
	}
	sort.Strings(out)
	return out
}

func status(t *testing.T, ctx context.Context, id string) string {
	t.Helper()
	var s string
	if err := db.PG.QueryRow(ctx, `SELECT status FROM propagation_outbox WHERE id = $1::uuid`, id).Scan(&s); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestPlanner_AssignBundle_OutboxIsThePlan(t *testing.T) {
	f := newPlannerFixture(t)
	// Becky has a removal of "member" waiting to be sent; giving the bundle
	// cancels it, and the plan says so. Bob already holds "member" directly.
	revoke := f.queue(t, f.becky, "revoke", f.member, "direct", "")
	if _, err := db.EnqueueDirectGrantPropagation(f.ctx, db.EnqueueParams{UserID: f.bob, ProjectID: f.project,
		RoleKeys: []string{f.member}, GrantedBy: f.actor, OpType: "add", NoPropagation: true, PayloadJSON: "{}"}); err != nil {
		t.Fatal(err)
	}
	ctx, gate := db.GuardCancellations(f.ctx, nil, func(context.Context) bool { return true })
	req := f.request(services.BulkOpAssignBundle, f.becky, f.bob)

	changes, _, err := services.PlanBulk(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	shown := map[string]services.Change{}
	for _, ch := range changes {
		shown[ch.UserID] = ch
	}
	if ids := shown[f.becky].Row.CancelIDs; len(ids) != 1 || ids[0] != revoke {
		t.Fatalf("Becky's row names the removal it cancels: %v", ids)
	}
	if !strings.Contains(shown[f.becky].Row.Consequence, "Cancels a change waiting for you to send") ||
		!strings.Contains(shown[f.becky].Row.Consequence, "Giving Trained in Laser Lab waits for you to send.") {
		t.Fatalf("Becky's row: %q", shown[f.becky].Row.Consequence)
	}

	result, written, err := services.ApplyPlannedBulk(ctx, req, f.issue(t, ctx, req, changes))
	if err != nil {
		t.Fatal(err)
	}
	if len(gate.Refused()) != 0 {
		t.Fatalf("a named cancellation was refused: %+v", gate.Refused())
	}
	deliveries := map[string]int{}
	for _, who := range []string{f.becky, f.bob} {
		got, want := sentRows(t, ctx, outboxIDs(written[who])), shownWrites(shown[who])
		if strings.Join(got, "|") != strings.Join(want, "|") || len(got) == 0 {
			t.Errorf("%s: outbox %v, plan showed %v", who, got, want)
		}
		// Each row as the plan decided its delivery (decision 11): a write
		// that waits lands waiting, with role_waits, and one sent straight
		// away carries no wait. Both with the operator's reason (task 1.8).
		for _, w := range written[who].Writes {
			if w.OutboxID == "" {
				continue
			}
			deliveries[w.Delivery]++
			wait, why := rowWaitAndReason(t, ctx, w.OutboxID)
			wantWait := ""
			if w.Delivery == services.DeliveryWaits {
				wantWait = db.WaitReasonRoleWaits
			}
			if wait != wantWait || why != "term start" {
				t.Errorf("%s %s (%s): wait_reason %q reason %q, want %q and the operator's reason", who, w.RoleKeys[0], w.Delivery, wait, why, wantWait)
			}
		}
		if said := auditReason(t, ctx, who, "bundle.assigned"); said != "term start" {
			t.Errorf("%s: the audit row's reason is %q, want the operator's", who, said)
		}
	}
	// "member" goes straight away, "trained" waits for you.
	if deliveries[services.DeliveryStraightAway] == 0 || deliveries[services.DeliveryWaits] == 0 {
		t.Fatalf("want both deliveries in this plan, got %v", deliveries)
	}
	// The dialog says the reason lands in Change history: it does.
	var cascade string
	if err := db.PG.QueryRow(ctx, `SELECT cascade_id::text FROM propagation_outbox WHERE id = $1::uuid`,
		outboxIDs(written[f.becky])[0]).Scan(&cascade); err != nil {
		t.Fatal(err)
	}
	if groups, err := db.GetCascadeGroups(ctx, 0, cascade); err != nil || len(groups) != 1 || groups[0].Said != "term start" {
		t.Fatalf("Change history for Becky's change: %+v %v, want the operator's reason", groups, err)
	}
	if s := status(t, ctx, revoke); s != "superseded" {
		t.Errorf("the named removal is cancelled, got %s", s)
	}
	if result.Summary.Apply != 2 || !result.Applied {
		t.Errorf("result summary = %+v", result.Summary)
	}
}

func TestPlanner_RemoveBundle_UnnamedCancellationIsStaleThenOutboxIsThePlan(t *testing.T) {
	f := newPlannerFixture(t)
	v, err := db.LatestVersion(f.ctx, f.bundle)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok, err := db.AssignBundleAndEnqueue(f.ctx, f.actor, "", f.becky, f.bundle, v.ID, nil); err != nil || !ok {
		t.Fatalf("assign: %v %v", ok, err)
	}
	ctx, _ := db.GuardCancellations(f.ctx, nil, func(context.Context) bool { return true })
	req := f.request(services.BulkOpRemoveBundle, f.becky)

	changes, _, err := services.PlanBulk(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	cite := f.issue(t, ctx, req, changes)

	// After the preview, someone queues a give of "member" from this bundle.
	// The removal would cancel it, and the plan never named it.
	unnamed := f.queue(t, f.becky, "add", f.member, "bundle", f.bundle)
	before := len(sentRowsFor(t, ctx, f.becky))

	_, _, err = services.ApplyPlannedBulk(ctx, req, cite)
	var stale *services.StalePlan
	if !errors.As(err, &stale) || len(stale.IDs) != 1 || stale.IDs[0] != f.becky {
		t.Fatalf("want the plan stale for Becky, got %v", err)
	}
	if s := status(t, ctx, unnamed); s != "pending" {
		t.Errorf("the unnamed change is untouched, got %s", s)
	}
	if after := len(sentRowsFor(t, ctx, f.becky)); after != before {
		t.Errorf("a stale plan wrote %d outbox row(s)", after-before)
	}
	var held, unspent bool
	_ = db.PG.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM user_bundle_assignments WHERE user_id = $1 AND bundle_id = $2)`, f.becky, f.bundle).Scan(&held)
	_ = db.PG.QueryRow(ctx, `SELECT applied_at IS NULL FROM plans WHERE id = $1::uuid`, cite.PlanID).Scan(&unspent)
	if !held || !unspent {
		t.Fatalf("nothing written and the plan unspent: still holds=%v unspent=%v", held, unspent)
	}

	// Re-planned, the row names it; applied, the outbox is the plan.
	changes, _, err = services.PlanBulk(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if ids := changes[0].Row.CancelIDs; len(ids) != 1 || ids[0] != unnamed {
		t.Fatalf("the fresh plan names the change: %v", ids)
	}
	_, written, err := services.ApplyPlannedBulk(ctx, req, f.issue(t, ctx, req, changes))
	if err != nil {
		t.Fatal(err)
	}
	if said := auditReason(t, ctx, f.becky, "bundle.unassigned"); said != "term start" {
		t.Errorf("the removal's audit row says %q, want the operator's reason", said)
	}
	got, want := sentRows(t, ctx, outboxIDs(written[f.becky])), shownWrites(changes[0])
	if strings.Join(got, "|") != strings.Join(want, "|") || len(got) == 0 {
		t.Errorf("outbox %v, plan showed %v", got, want)
	}
	if s := status(t, ctx, unnamed); s != "superseded" {
		t.Errorf("the now-named change is cancelled, got %s", s)
	}
}

// outboxIDs is the rows the apply wrote for a Change.
func outboxIDs(ch services.Change) []string {
	var ids []string
	for _, w := range ch.Writes {
		if w.OutboxID != "" {
			ids = append(ids, w.OutboxID)
		}
	}
	return ids
}

func rowWaitAndReason(t *testing.T, ctx context.Context, id string) (wait, why string) {
	t.Helper()
	if err := db.PG.QueryRow(ctx, `SELECT COALESCE(wait_reason, ''), COALESCE(reason, '') FROM propagation_outbox WHERE id = $1::uuid`,
		id).Scan(&wait, &why); err != nil {
		t.Fatal(err)
	}
	return wait, why
}

func auditReason(t *testing.T, ctx context.Context, user, action string) string {
	t.Helper()
	var why string
	if err := db.PG.QueryRow(ctx, `SELECT COALESCE(details->>'reason', '') FROM audit_logs WHERE target_zitadel_user_id = $1 AND action = $2`,
		user, action).Scan(&why); err != nil {
		t.Fatal(err)
	}
	return why
}

func sentRowsFor(t *testing.T, ctx context.Context, user string) []string {
	t.Helper()
	var ids []string
	rows, err := db.PG.Query(ctx, `SELECT id::text FROM propagation_outbox WHERE user_id = $1`, user)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	return ids
}

// withRule adds the rule "trained gives advanced" in the fixture's project.
func (f plannerFixture) withRule(t *testing.T) string {
	t.Helper()
	id, _, err := db.CreateMappingRuleAndEnqueue(f.ctx, f.actor, f.project, f.trains, f.project, "advanced", nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.PG.Exec(f.ctx, `DELETE FROM mapping_rules WHERE id = $1::uuid`, id) })
	return id
}

// assignWithRule gives Becky the bundle through the planner and returns the
// rows it queued, none of them sent yet.
func (f plannerFixture) assignWithRule(t *testing.T, ctx context.Context, rule string) []string {
	t.Helper()
	req := f.request(services.BulkOpAssignBundle, f.becky)
	changes, _, err := services.PlanBulk(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	_, written, err := services.ApplyPlannedBulk(ctx, req, f.issue(t, ctx, req, changes))
	if err != nil {
		t.Fatal(err)
	}
	ids := outboxIDs(written[f.becky])
	// The bundle's roles are its rows; the role the rule follows with is the
	// rule's, with the rule (task 1.6), the shape FollowRules writes.
	want := []string{"add advanced rule " + rule, "add member bundle " + f.bundle, "add trained bundle " + f.bundle}
	if got := sentRows(t, ctx, ids); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("assign queued %v, want %v", got, want)
	}
	return ids
}

// A bundle's removal cancels exactly the queued changes the bundle and the
// rules it set off produced, and nothing else: not another bundle's give of the
// same role, not a direct give of the role the rule gives (task 1.6).
func TestPlanner_RemoveBundle_CancelsItsOwnAndItsRulesQueuedRows(t *testing.T) {
	f := newPlannerFixture(t)
	rule := f.withRule(t)
	ctx, gate := db.GuardCancellations(f.ctx, nil, func(context.Context) bool { return true })
	queued := f.assignWithRule(t, ctx, rule)

	// Change history names the change by the bundle, not by the rule row
	// written in the same instant.
	var cascade string
	if err := db.PG.QueryRow(ctx, `SELECT cascade_id::text FROM propagation_outbox WHERE id = $1::uuid`, queued[0]).Scan(&cascade); err != nil {
		t.Fatal(err)
	}
	groups, err := db.GetCascadeGroups(ctx, 0, cascade)
	if err != nil || len(groups) != 1 || groups[0].Source != "bundle" || groups[0].SourceRef != f.bundle || len(groups[0].Writes) != 3 {
		t.Fatalf("Change history: %+v %v, want one bundle entry with its 3 writes", groups, err)
	}

	otherBundle := f.queue(t, f.becky, "add", f.member, "bundle", "another-bundle")
	direct := f.queue(t, f.becky, "add", "advanced", "direct", "")

	req := f.request(services.BulkOpRemoveBundle, f.becky)
	changes, _, err := services.PlanBulk(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	named := append([]string(nil), changes[0].Row.CancelIDs...)
	sort.Strings(named)
	want := append([]string(nil), queued...)
	sort.Strings(want)
	if strings.Join(named, "|") != strings.Join(want, "|") {
		t.Fatalf("the plan names %v, want exactly the assign's rows %v", named, want)
	}
	if !strings.Contains(changes[0].Row.Consequence, "Also loses Advanced in Laser Lab, which the automatic rule from Trained in Laser Lab gave.") {
		t.Errorf("row: %q", changes[0].Row.Consequence)
	}
	_, written, err := services.ApplyPlannedBulk(ctx, req, f.issue(t, ctx, req, changes))
	if err != nil {
		t.Fatal(err)
	}
	if len(gate.Refused()) != 0 {
		t.Fatalf("a named cancellation was refused: %+v", gate.Refused())
	}
	for _, id := range queued {
		if s := status(t, ctx, id); s != "superseded" {
			t.Errorf("the assign's row %s is %s, want superseded", id, s)
		}
	}
	for _, id := range []string{otherBundle, direct} {
		if s := status(t, ctx, id); s != "pending" {
			t.Errorf("an unrelated queued change %s is %s, want pending", id, s)
		}
	}
	// Nothing had reached Zitadel, so nothing is sent to take it back.
	if ids := outboxIDs(written[f.becky]); len(ids) != 0 {
		t.Errorf("the removal queued %v, want nothing", sentRows(t, ctx, ids))
	}
}

// A rule's own retirement cancels the give the rule queued under a bundle's
// change, and leaves the bundle's own rows: before task 1.6 that row said
// 'bundle' and the retirement queued its removal behind it.
func TestPlanner_RuleRetiredCancelsTheRowItSetOffUnderABundle(t *testing.T) {
	f := newPlannerFixture(t)
	rule := f.withRule(t)
	ctx, _ := db.GuardCancellations(f.ctx, nil, func(context.Context) bool { return true })
	queued := f.assignWithRule(t, ctx, rule)
	row := map[string]string{}
	for _, id := range queued {
		row[id] = sentRows(t, ctx, []string{id})[0]
	}

	revoke := db.EnqueueParams{UserID: f.becky, ProjectID: f.project, RoleKeys: []string{"advanced"}, OpType: "revoke",
		Source: "rule", SourceRef: rule, GrantedBy: f.actor, PayloadJSON: "{}"}
	// Not a plan: the retirement runs outside the plan's cancel gate, which
	// refuses any cancellation the plan didn't name.
	ids, err := db.RetireMappingRuleAndEnqueue(f.ctx, f.actor, rule, []db.EnqueueParams{revoke})
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 0 {
		t.Errorf("the retirement queued %v, want nothing: the give it answers was never sent", sentRows(t, ctx, ids))
	}
	for _, id := range queued {
		want := "pending"
		if strings.HasPrefix(row[id], "add advanced") {
			want = "superseded"
		}
		if s := status(t, ctx, id); s != want {
			t.Errorf("row %s is %s, want %s", id, s, want)
		}
	}
}
