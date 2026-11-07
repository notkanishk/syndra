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
	alice, bob     string
	actor          string
	member, trains string
}

func newPlannerFixture(t *testing.T) plannerFixture {
	t.Helper()
	ctx := db.LiveDB(t)
	sfx := time.Now().Format("150405.000000000")
	f := plannerFixture{ctx: ctx, project: "plan-p-" + sfx, alice: "plan-a-" + sfx, bob: "plan-b-" + sfx,
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
	people := []string{f.alice, f.bob}
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
		users: []models.UserProfile{{ID: f.alice, Name: "Alice Example", Status: "active"}, {ID: f.bob, Name: "Bob Example", Status: "active"}},
		project: models.ProjectCatalog{ID: f.project, Name: "Laser Lab", Roles: []models.ProjectRole{
			{Key: f.member, Label: "Member"}, {Key: f.trains, Label: "Trained"},
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
	// Alice has a removal of "member" waiting to be sent; giving the bundle
	// cancels it, and the plan says so. Bob already holds "member" directly.
	revoke := f.queue(t, f.alice, "revoke", f.member, "direct", "")
	if _, err := db.EnqueueDirectGrantPropagation(f.ctx, db.EnqueueParams{UserID: f.bob, ProjectID: f.project,
		RoleKeys: []string{f.member}, GrantedBy: f.actor, OpType: "add", NoPropagation: true, PayloadJSON: "{}"}); err != nil {
		t.Fatal(err)
	}
	ctx, gate := db.GuardCancellations(f.ctx, nil, func(context.Context) bool { return true })
	req := f.request(services.BulkOpAssignBundle, f.alice, f.bob)

	changes, _, err := services.PlanBulk(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	shown := map[string]services.Change{}
	for _, ch := range changes {
		shown[ch.UserID] = ch
	}
	if ids := shown[f.alice].Row.CancelIDs; len(ids) != 1 || ids[0] != revoke {
		t.Fatalf("Alice's row names the removal it cancels: %v", ids)
	}
	if !strings.Contains(shown[f.alice].Row.Consequence, "Cancels a change waiting for you to send") ||
		!strings.Contains(shown[f.alice].Row.Consequence, "Giving Trained in Laser Lab waits for you to send.") {
		t.Fatalf("Alice's row: %q", shown[f.alice].Row.Consequence)
	}

	result, sent, err := services.ApplyPlannedBulk(ctx, req, f.issue(t, ctx, req, changes))
	if err != nil {
		t.Fatal(err)
	}
	if len(gate.Refused()) != 0 {
		t.Fatalf("a named cancellation was refused: %+v", gate.Refused())
	}
	for _, who := range []string{f.alice, f.bob} {
		got, want := sentRows(t, ctx, sent[who]), shownWrites(shown[who])
		if strings.Join(got, "|") != strings.Join(want, "|") || len(got) == 0 {
			t.Errorf("%s: outbox %v, plan showed %v", who, got, want)
		}
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
	if _, ok, err := db.AssignBundleAndEnqueue(f.ctx, f.actor, f.alice, f.bundle, v.ID, nil); err != nil || !ok {
		t.Fatalf("assign: %v %v", ok, err)
	}
	ctx, _ := db.GuardCancellations(f.ctx, nil, func(context.Context) bool { return true })
	req := f.request(services.BulkOpRemoveBundle, f.alice)

	changes, _, err := services.PlanBulk(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	cite := f.issue(t, ctx, req, changes)

	// After the preview, someone queues a give of "member" from this bundle.
	// The removal would cancel it, and the plan never named it.
	unnamed := f.queue(t, f.alice, "add", f.member, "bundle", f.bundle)
	before := len(sentRowsFor(t, ctx, f.alice))

	_, _, err = services.ApplyPlannedBulk(ctx, req, cite)
	var stale *services.StalePlan
	if !errors.As(err, &stale) || len(stale.IDs) != 1 || stale.IDs[0] != f.alice {
		t.Fatalf("want the plan stale for Alice, got %v", err)
	}
	if s := status(t, ctx, unnamed); s != "pending" {
		t.Errorf("the unnamed change is untouched, got %s", s)
	}
	if after := len(sentRowsFor(t, ctx, f.alice)); after != before {
		t.Errorf("a stale plan wrote %d outbox row(s)", after-before)
	}
	var held, unspent bool
	_ = db.PG.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM user_bundle_assignments WHERE user_id = $1 AND bundle_id = $2)`, f.alice, f.bundle).Scan(&held)
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
	_, sent, err := services.ApplyPlannedBulk(ctx, req, f.issue(t, ctx, req, changes))
	if err != nil {
		t.Fatal(err)
	}
	got, want := sentRows(t, ctx, sent[f.alice]), shownWrites(changes[0])
	if strings.Join(got, "|") != strings.Join(want, "|") || len(got) == 0 {
		t.Errorf("outbox %v, plan showed %v", got, want)
	}
	if s := status(t, ctx, unnamed); s != "superseded" {
		t.Errorf("the now-named change is cancelled, got %s", s)
	}
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
