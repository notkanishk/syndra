package db_test

import (
	"context"
	"errors"
	"maps"
	"testing"
	"time"

	"syndra/internal/cache"
	"syndra/internal/db"
	"syndra/internal/models"
	_ "syndra/internal/services" // wires db.FollowRules, as in the binary
)

// Rules on the pipeline, end to end against a real database: Syndra's own grant
// of A, with rules A→B and B→C, puts B and C in the outbox as rule rows, in the
// grant's transaction, before anything is sent — and those rows are exactly the
// chain the token compiler derives. Zitadel is never called: the outbox is what
// the drain sends, and the drain's own tests cover sending it.

type pipelineFixture struct {
	ctx        context.Context
	pa, pb, pc string
	ruleAB     string
	ruleBC     string
}

func newPipelineFixture(t *testing.T) pipelineFixture {
	t.Helper()
	ctx := db.LiveDB(t)
	sfx := time.Now().Format("150405.000000000")
	f := pipelineFixture{ctx: ctx, pa: "pipe-a-" + sfx, pb: "pipe-b-" + sfx, pc: "pipe-c-" + sfx}
	t.Cleanup(func() {
		for _, p := range []string{f.pa, f.pb, f.pc} {
			_, _ = db.PG.Exec(ctx, `DELETE FROM mapping_rules WHERE source_zitadel_project_id = $1`, p)
			_, _ = db.PG.Exec(ctx, `DELETE FROM direct_role_grants WHERE zitadel_project_id = $1`, p)
		}
	})
	var err error
	if f.ruleAB, _, err = db.CreateMappingRuleAndEnqueue(ctx, "op@example.org", f.pa, "a", f.pb, "b", nil); err != nil {
		t.Fatal(err)
	}
	if f.ruleBC, _, err = db.CreateMappingRuleAndEnqueue(ctx, "op@example.org", f.pb, "b", f.pc, "c", nil); err != nil {
		t.Fatal(err)
	}
	return f
}

func (f pipelineFixture) give(user, project, role, op string) (db.EnqueueResult, error) {
	return db.EnqueueDirectGrantPropagation(f.ctx, db.EnqueueParams{
		UserID: user, ProjectID: project, RoleKeys: []string{role}, GrantedBy: "op@example.org",
		Source: "direct", OpType: op, PayloadJSON: "{}",
	})
}

type outboxRow struct {
	op, project, role, source, ref, status string
}

func outboxFor(t *testing.T, ctx context.Context, user string) map[string]outboxRow {
	t.Helper()
	rows, err := db.PG.Query(ctx, `
		SELECT id, op_type, project_id, role_keys[1], source, COALESCE(source_ref, ''), status
		  FROM propagation_outbox WHERE user_id = $1`, user)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]outboxRow{}
	for rows.Next() {
		var id string
		var r outboxRow
		if err := rows.Scan(&id, &r.op, &r.project, &r.role, &r.source, &r.ref, &r.status); err != nil {
			t.Fatal(err)
		}
		out[id] = r
	}
	return out
}

func TestRulesOnThePipeline_OwnGrantSendsZitadelExactlyTheCompiledChain(t *testing.T) {
	f := newPipelineFixture(t)
	user := "pipe-u-" + time.Now().Format("150405.000000000")

	res, err := f.give(user, f.pa, "a", "add")
	if err != nil {
		t.Fatal(err)
	}
	if res.OutboxID == "" || len(res.RuleOutboxIDs) != 2 {
		t.Fatalf("want the grant's own row and two rule rows, got %+v", res)
	}

	// The trace exists before any call: every row is committed and pending,
	// and nothing has been sent.
	rows := outboxFor(t, f.ctx, user)
	if len(rows) != 3 {
		t.Fatalf("outbox for %s: %+v", user, rows)
	}
	sent := map[models.HeldRole]bool{}
	for id, r := range rows {
		if r.status != "pending" || r.op != "add" {
			t.Fatalf("row %s is %+v; nothing may be sent before it is traced", id, r)
		}
		sent[models.HeldRole{ProjectID: r.project, RoleKey: r.role}] = true
	}
	for _, id := range res.RuleOutboxIDs {
		r := rows[id]
		wantRef := map[string]string{f.pb: f.ruleAB, f.pc: f.ruleBC}[r.project]
		if r.source != "rule" || r.ref != wantRef {
			t.Fatalf("rule row %s = %+v, want source rule ref %s", id, r, wantRef)
		}
	}
	// Rule rows are cascade rows: no ledger row of their own.
	var ledger int
	if err := db.PG.QueryRow(f.ctx, `SELECT count(*) FROM direct_role_grants WHERE user_id = $1`, user).Scan(&ledger); err != nil || ledger != 1 {
		t.Fatalf("ledger rows = %d (%v), want only the direct grant", ledger, err)
	}

	// Exactly the compiler's closure: what Zitadel is sent is what the token says.
	compiled := map[models.HeldRole]bool{}
	for _, p := range []string{f.pa, f.pb, f.pc} {
		facts, err := cache.CompileFacts(f.ctx, user, p)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range facts.Roles {
			compiled[models.HeldRole{ProjectID: p, RoleKey: r}] = true
		}
	}
	if !maps.Equal(sent, compiled) {
		t.Fatalf("Zitadel is sent %v, the token carries %v", sent, compiled)
	}

	// Giving it again (a retried request after a crash before the response)
	// queues nothing new: the queued add is the answer, and the closure has not
	// moved, so the rules owe nothing more.
	again, err := f.give(user, f.pa, "a", "add")
	if err != nil {
		t.Fatal(err)
	}
	if again.OutboxID != res.OutboxID || len(again.RuleOutboxIDs) != 0 || len(outboxFor(t, f.ctx, user)) != 3 {
		t.Fatalf("a retry must not duplicate: %+v, outbox %d rows", again, len(outboxFor(t, f.ctx, user)))
	}
}

// A dispatcher that dies mid-send leaves its row in_flight; the next claim
// takes it again, and once applied nothing claims it a third time.
func TestRulesOnThePipeline_ACrashMidSendIsRetriedOnce(t *testing.T) {
	f := newPipelineFixture(t)
	user := "pipe-crash-" + time.Now().Format("150405.000000000")
	res, err := f.give(user, f.pa, "a", "add")
	if err != nil || len(res.RuleOutboxIDs) != 2 {
		t.Fatalf("%+v %v", res, err)
	}
	id := res.RuleOutboxIDs[0]
	if _, found, err := db.ClaimPropagationByID(f.ctx, db.TargetZitadel, id); err != nil || !found {
		t.Fatalf("first claim: %v %v", found, err)
	}
	// ...the process dies here, before the row is settled.
	row, found, err := db.ClaimPropagationByID(f.ctx, db.TargetZitadel, id)
	if err != nil || !found || row.ID != id {
		t.Fatalf("the orphaned row must be claimable again: %v %v", found, err)
	}
	if err := db.MarkPropagationApplied(f.ctx, id); err != nil {
		t.Fatal(err)
	}
	if _, found, err := db.ClaimPropagationByID(f.ctx, db.TargetZitadel, id); err != nil || found {
		t.Fatalf("an applied row must not be sent again: %v %v", found, err)
	}
}

// A failure after the write leaves nothing: the grant, its audit row and its
// rule rows share one transaction.
func TestRulesOnThePipeline_AFailureLeavesNoHalfWrite(t *testing.T) {
	f := newPipelineFixture(t)
	user := "pipe-fail-" + time.Now().Format("150405.000000000")
	orig := db.FollowRules
	t.Cleanup(func() { db.FollowRules = orig })
	db.FollowRules = func(ctx context.Context, p db.EnqueueParams, write func() error) ([]db.EnqueueParams, error) {
		if _, err := orig(ctx, p, write); err != nil {
			return nil, err
		}
		return nil, errors.New("crash after the write")
	}
	if _, err := f.give(user, f.pa, "a", "add"); err == nil {
		t.Fatal("expected the failure")
	}
	var ledger int
	_ = db.PG.QueryRow(f.ctx, `SELECT count(*) FROM direct_role_grants WHERE user_id = $1`, user).Scan(&ledger)
	if ledger != 0 || len(outboxFor(t, f.ctx, user)) != 0 {
		t.Fatalf("a failed write left ledger=%d outbox=%v", ledger, outboxFor(t, f.ctx, user))
	}
}

// Removing A takes B and C back, through the same rows, unless something else
// still gives them: here a direct grant of C stays, so only B goes.
func TestRulesOnThePipeline_RemovalFollowsTheChainAndKeepsWhatIsHeldOtherwise(t *testing.T) {
	f := newPipelineFixture(t)
	user := "pipe-rm-" + time.Now().Format("150405.000000000")
	if _, err := f.give(user, f.pa, "a", "add"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.give(user, f.pc, "c", "add"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.PG.Exec(f.ctx, `TRUNCATE propagation_outbox`); err != nil { // as if all were sent
		t.Fatal(err)
	}
	res, err := f.give(user, f.pa, "a", "revoke")
	if err != nil {
		t.Fatal(err)
	}
	rows := outboxFor(t, f.ctx, user)
	var revoked []string
	for _, id := range res.RuleOutboxIDs {
		r := rows[id]
		if r.op != "revoke" || r.source != "rule" {
			t.Fatalf("rule row %+v", r)
		}
		revoked = append(revoked, r.project+"/"+r.role)
	}
	if len(revoked) != 1 || revoked[0] != f.pb+"/b" {
		t.Fatalf("revoked %v, want only %s/b (c is still given directly)", revoked, f.pb)
	}
}
