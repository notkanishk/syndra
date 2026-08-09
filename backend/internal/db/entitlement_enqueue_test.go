package db

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// outboxColumns is every column the outbox has after all migrations: the
// original CREATE TABLE plus every ADD COLUMN under either of its two names.
func outboxColumns(t *testing.T) map[string]bool {
	t.Helper()
	files, err := filepath.Glob("../../db/migrations/*.up.sql")
	if err != nil || len(files) == 0 {
		t.Fatalf("glob migrations: %v", err)
	}

	cols := map[string]bool{}
	create := regexp.MustCompile(`(?is)CREATE TABLE IF NOT EXISTS pending_zitadel_propagations \((.*?)\n\);`)
	add := regexp.MustCompile(`(?is)ALTER TABLE (?:IF EXISTS )?(?:pending_zitadel_propagations|propagation_outbox)\s+ADD COLUMN (?:IF NOT EXISTS )?(\w+)`)
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		body := stripSQLComments(string(src))
		if m := create.FindStringSubmatch(body); m != nil {
			for _, line := range strings.Split(m[1], "\n") {
				if fields := strings.Fields(strings.TrimSpace(line)); len(fields) > 1 {
					cols[fields[0]] = true
				}
			}
		}
		for _, m := range add.FindAllStringSubmatch(body, -1) {
			cols[m[1]] = true
		}
	}
	if !cols["op_type"] {
		t.Fatal("could not read the outbox column set — the guards below would prove nothing")
	}
	return cols
}

// insertShape splits the entitlement INSERT into its column list and the
// expressions selected into it.
func insertShape(t *testing.T) (columns, selected []string) {
	t.Helper()
	src, err := os.ReadFile("entitlement_enqueue.go")
	if err != nil {
		t.Fatalf("read entitlement_enqueue.go: %v", err)
	}
	m := regexp.MustCompile(`(?s)INSERT INTO propagation_outbox\s*\n\s*\((.*?)\)\s*\n\s*SELECT (.*?)\n`).FindStringSubmatch(string(src))
	if m == nil {
		t.Fatal("could not isolate the entitlement INSERT's column and select lists")
	}
	for _, c := range strings.Split(m[1], ",") {
		columns = append(columns, strings.TrimSpace(c))
	}
	for _, v := range strings.Split(m[2], ",") {
		selected = append(selected, strings.TrimSpace(v))
	}
	return columns, selected
}

func entitlementInsert(t *testing.T) string {
	t.Helper()
	src, err := os.ReadFile("entitlement_enqueue.go")
	if err != nil {
		t.Fatalf("read entitlement_enqueue.go: %v", err)
	}
	m := regexp.MustCompile(`(?s)INSERT INTO propagation_outbox(.*?)RETURNING`).FindStringSubmatch(string(src))
	if m == nil {
		t.Fatal("could not isolate the entitlement outbox INSERT")
	}
	return m[1]
}

// 2.9 — the row is written to the target it is for. `target` carries
// `DEFAULT 'zitadel'`, which makes omitting it silent: the write succeeds and
// the row queues a TrueNAS convergence against Zitadel, where the drain will
// find no project and no roles.
func TestAnEntitlementRowNamesItsTargetAndItsApproval(t *testing.T) {
	insert := entitlementInsert(t)
	columns, selected := insertShape(t)

	// The COLUMN LIST, not merely the statement text. `p.target` appears in the
	// SELECT whether or not the row has a target column to land in, so a
	// substring check over the whole statement passes while the write is
	// malformed — or, worse, well-formed and defaulting to zitadel.
	for _, col := range []string{"target", "plan_subject_id"} {
		if !slices.Contains(columns, col) {
			t.Errorf("the entitlement enqueue must write %q explicitly: `target` defaults to zitadel, so omitting it is a wrong write that raises nothing, and without the plan subject the drain has no fingerprint to re-verify", col)
		}
	}
	// And the two halves line up. A column list and a select list of different
	// lengths is a statement that fails the moment it is prepared, inside an
	// apply that has already claimed the plan.
	if len(columns) != len(selected) {
		t.Errorf("the insert names %d columns and selects %d values: %v vs %v", len(columns), len(selected), columns, selected)
	}
	// The op_type is a literal in the statement, not a parameter, so there is
	// no caller able to queue an entitlement change as something else.
	if !strings.Contains(insert, "'apply'") {
		t.Error("op_type must be the literal 'apply' — a level-triggered convergence has no add/revoke/replace to choose between")
	}
	// Same for the payload: the column is NOT NULL and the intent lives in the
	// snapshot, so the row says nothing rather than carrying a second copy that
	// a future writer could fill with an add-on's response.
	if !strings.Contains(insert, "'{}'::jsonb") {
		t.Error("payload_json must be written as an empty literal, with no parameter able to reach it")
	}
}

// 2.9 — every column written exists. A statement naming a column that is not
// there fails at runtime, inside an operator's apply, after the plan has
// already been claimed in the same transaction.
func TestTheEntitlementEnqueueWritesOnlyDeclaredColumns(t *testing.T) {
	declared := outboxColumns(t)
	src, err := os.ReadFile("entitlement_enqueue.go")
	if err != nil {
		t.Fatalf("read entitlement_enqueue.go: %v", err)
	}
	m := regexp.MustCompile(`(?s)INSERT INTO propagation_outbox\s*\n\s*\((.*?)\)`).FindStringSubmatch(string(src))
	if m == nil {
		t.Fatal("could not isolate the entitlement outbox column list")
	}
	for _, col := range strings.Split(m[1], ",") {
		col = strings.TrimSpace(col)
		if col != "" && !declared[col] {
			t.Errorf("the entitlement enqueue writes column %q, which no migration declares", col)
		}
	}
}

// 2.9 — the sources Go will write and the sources the database accepts are one
// vocabulary. A value this check approves and the CHECK constraint refuses is
// an apply that fails as a constraint violation with the plan already claimed.
func TestTheOutboxSourceVocabularyMatchesTheConstraint(t *testing.T) {
	up, _ := addonMigrationSQL(t)
	m := regexp.MustCompile(`(?is)ADD CONSTRAINT propagation_outbox_source_check\s+CHECK \(source IN \((.*?)\)\)`).FindStringSubmatch(up)
	if m == nil {
		t.Fatal("could not isolate the outbox source CHECK")
	}

	accepted := map[string]bool{}
	for _, raw := range strings.Split(m[1], ",") {
		accepted[strings.Trim(strings.TrimSpace(raw), "'")] = true
	}
	if len(accepted) < 2 {
		t.Fatal("read no source vocabulary from the CHECK")
	}

	for source := range accepted {
		if !validOutboxSource(source) {
			t.Errorf("the database accepts source %q and this package refuses it", source)
		}
	}
	// And nothing beyond it. Sampled rather than exhaustive — the direction
	// that matters is the one where Go approves what the database will reject.
	for _, invented := range []string{"", "apply", "plan", "entitlement", "manual", "addon"} {
		if accepted[invented] {
			continue
		}
		if validOutboxSource(invented) {
			t.Errorf("this package accepts source %q, which the CHECK would refuse — the apply would fail as a constraint violation with the plan already claimed", invented)
		}
	}
}

// 2.9 — `apply` is a legal op_type. It was installed by the same migration that
// widened the outbox, and the literal in the statement above depends on it.
func TestApplyIsAnAcceptedOpType(t *testing.T) {
	up, _ := addonMigrationSQL(t)
	if !regexp.MustCompile(`(?is)propagation_outbox_op_type_check\s+CHECK \(op_type IN \([^)]*'apply'`).MatchString(up) {
		t.Error("the outbox op_type CHECK must accept 'apply'")
	}
}

// 2.9 — every bound field of the row comes from the approval, not from a
// caller. A foreign key would have proved only that the plan subject exists:
// not that it is this person's, not that its plan was ever claimed, and not
// that its target still takes work. Each of those is in the predicate, so a row
// that should not exist is never written rather than written and argued about.
func TestTheOutboxRowIsDerivedFromTheApproval(t *testing.T) {
	insert := entitlementInsert(t)

	// Derived, not parameterised. A $n in any of these positions is a value a
	// caller chose, and a caller that chooses the subject can queue one
	// person's work under another person's approval.
	for _, derived := range []string{"ps.subject_id", "p.created_by", "p.target", "ps.id"} {
		if !strings.Contains(insert, derived) {
			t.Errorf("the row must take its %s from the approval, not from a parameter", derived)
		}
	}
	for _, predicate := range []struct{ frag, why string }{
		{"p.applied_at IS NOT NULL", "work may only be queued under an approval that was actually claimed"},
		{"t.state = 'active'", "a target the deployment dropped must take no work, enforced at the write and not only at the caller"},
		{"p.target <> $3", "the built-in target's rows carry project and role columns this path cannot fill"},
		{"ON CONFLICT (plan_subject_id) WHERE plan_subject_id IS NOT NULL DO NOTHING", "one approval, one queued convergence — and via the index, so a concurrent second caller gets the same typed refusal rather than a raised constraint violation that kills its transaction"},
	} {
		if !strings.Contains(insert, predicate.frag) {
			t.Errorf("the insert must require %q: %s", predicate.frag, predicate.why)
		}
	}
	// And the caller supplies nothing that identifies a person or a target.
	for _, field := range []string{"SubjectID", "Target", "InitiatedBy"} {
		if _, ok := reflect.TypeOf(EntitlementApply{}).FieldByName(field); ok {
			t.Errorf("EntitlementApply.%s lets a caller name what the approval already names", field)
		}
	}
}

// 2.9 — the predicate is the authority and the pure explainer only says why it
// matched nothing, so the two must refuse on the same set of conditions.
func TestTheEnqueuePredicateAndItsExplainerRefuseTheSameThings(t *testing.T) {
	insert := entitlementInsert(t)

	queueable := approvalRef{found: true, target: "truenas", targetState: TargetActive, claimed: true}
	if err := enqueueRefusal(queueable, sampleUUID); err != nil {
		t.Fatalf("the baseline approval is not queueable (%v) — every case below would pass vacuously", err)
	}

	for _, tc := range []struct {
		name      string
		mutate    func(*approvalRef)
		want      error
		predicate string
	}{
		{"no such approval", func(r *approvalRef) { r.found = false }, ErrNoClaimedApproval, "ps.id = $4"},
		{"the plan was never claimed", func(r *approvalRef) { r.claimed = false }, ErrNoClaimedApproval, "p.applied_at IS NOT NULL"},
		{"the target was dropped", func(r *approvalRef) { r.targetState = TargetDisabled }, ErrTargetNotActive, "t.state = 'active'"},
		{"the built-in target", func(r *approvalRef) { r.target = TargetZitadel }, ErrNotAnEntitlementTarget, "p.target <> $3"},
		{"already queued", func(r *approvalRef) { r.alreadyQueued = true }, ErrAlreadyQueued, "DO NOTHING"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := queueable
			tc.mutate(&r)
			if err := enqueueRefusal(r, sampleUUID); !errors.Is(err, tc.want) {
				t.Errorf("enqueueRefusal = %v, want %v", err, tc.want)
			}
			if !strings.Contains(insert, tc.predicate) {
				t.Errorf("the insert does not require %q, so the database would write what the explainer calls a refusal", tc.predicate)
			}
		})
	}
}

// P2 — the duplicate a predicate cannot see gets the same answer as the one it
// can. A concurrent second caller passes any NOT EXISTS check, hits the unique
// index, and raises 23505 — which aborts its transaction, so the explainer that
// would have said ErrAlreadyQueued never runs and the caller gets an error
// about an index instead. Absorbing the conflict is what keeps the promise.
func TestAConcurrentDuplicateIsRefusedRatherThanRaised(t *testing.T) {
	insert := entitlementInsert(t)

	if strings.Contains(insert, "NOT EXISTS") {
		t.Error("a NOT EXISTS predicate cannot see an uncommitted concurrent insert, so it cannot be what enforces one row per approval")
	}
	if !strings.Contains(insert, "DO NOTHING") {
		t.Error("the conflict must be absorbed into no-row-returned, or the loser of a race gets a raised constraint violation and a dead transaction")
	}
	// Named, and named with the partial index's own predicate: an unqualified
	// DO NOTHING would also swallow an idempotency-key collision, which means
	// the key generator repeated itself and must not be quietly ignored.
	if !strings.Contains(insert, "ON CONFLICT (plan_subject_id) WHERE plan_subject_id IS NOT NULL") {
		t.Error("the conflict target must name the plan-subject index and its predicate, so no other uniqueness is absorbed with it")
	}
	// And the no-row case still ends at the typed refusal.
	if err := enqueueRefusal(approvalRef{found: true, target: "truenas", targetState: TargetActive, claimed: true, alreadyQueued: true}, sampleUUID); !errors.Is(err, ErrAlreadyQueued) {
		t.Errorf("enqueueRefusal = %v, want ErrAlreadyQueued", err)
	}
}

// 2.9 — a second row under one approval is refused by a predicate for a clean
// answer and by an index for the concurrent case a predicate cannot see. Two
// callers racing on one plan subject both read no existing row.
func TestOneApprovalCanQueueOnlyOneRow(t *testing.T) {
	up, down := addonMigrationSQL(t)

	if !regexp.MustCompile(`(?is)CREATE UNIQUE INDEX IF NOT EXISTS idx_propagation_outbox_plan_subject\s+ON propagation_outbox \(plan_subject_id\) WHERE plan_subject_id IS NOT NULL`).MatchString(up) {
		t.Error("a unique index must back the NOT EXISTS: without it two concurrent applies each see no row and each insert one, dispatching one reviewed change twice")
	}
	if !strings.Contains(down, "idx_propagation_outbox_plan_subject") {
		t.Error("the down migration must drop the index it added")
	}
}

// P1 — the target state is read under a row lock. An unlocked read races the
// registry reconciliation: it can return active, the reconciliation can commit
// a disable, and the apply can then commit the permanently undrainable row the
// check exists to refuse.
func TestTheTargetStateIsReadUnderALock(t *testing.T) {
	src, err := os.ReadFile("entitlement_enqueue.go")
	if err != nil {
		t.Fatalf("read entitlement_enqueue.go: %v", err)
	}
	if !regexp.MustCompile(`SELECT state FROM targets WHERE target = \$1 FOR UPDATE`).MatchString(string(src)) {
		t.Error("LockTargetStateTx must hold the target row, or an apply and a disable can interleave")
	}

	// And the reconciliation it races is the write that takes that lock.
	tgt, err := os.ReadFile("targets.go")
	if err != nil {
		t.Fatalf("read targets.go: %v", err)
	}
	if !strings.Contains(string(tgt), "UPDATE targets SET state = 'disabled'") {
		t.Error("the disable this lock serialises against is missing — the lock may be guarding nothing")
	}
}

// 2.9 — what an entitlement row refuses to become, decided before the database
// is touched. The nil transaction is the proof: a check that ran after the
// first statement would panic here rather than fail.
func TestAnEntitlementEnqueueRefusesAnUnqueueableRow(t *testing.T) {
	good := EntitlementApply{PlanSubjectID: sampleUUID}

	cases := []struct {
		name   string
		mutate func(*EntitlementApply)
	}{
		{"no approval behind it", func(p *EntitlementApply) { p.PlanSubjectID = "" }},
		{"a malformed approval reference", func(p *EntitlementApply) { p.PlanSubjectID = "plan-subject-7" }},
		{"a source the database would refuse", func(p *EntitlementApply) { p.Source = "whatever" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := good
			tc.mutate(&p)
			_, err := EnqueueEntitlementApplyTx(context.Background(), nil, p)
			if !errors.Is(err, ErrInvalidEnqueue) {
				t.Fatalf("EnqueueEntitlementApplyTx = %v, want ErrInvalidEnqueue", err)
			}
		})
	}
}

// 2.9 — and a Zitadel row cannot be written through this path at all. Its
// intent is its own project and role columns, which nothing here can fill; the
// table's shape CHECK would refuse the write, and learning that as a constraint
// violation mid-apply is worse than being told.
func TestTheEntitlementPathCannotWriteAZitadelRow(t *testing.T) {
	err := enqueueRefusal(approvalRef{found: true, target: TargetZitadel, targetState: TargetActive, claimed: true}, sampleUUID)
	if !errors.Is(err, ErrNotAnEntitlementTarget) {
		t.Fatalf("err = %v, want ErrNotAnEntitlementTarget", err)
	}
	if !strings.Contains(err.Error(), TargetZitadel) {
		t.Errorf("the refusal does not name the target it refused: %v", err)
	}

	// The constraint it would otherwise have hit, so the two stay in step.
	up, _ := addonMigrationSQL(t)
	if !strings.Contains(up, "propagation_outbox_zitadel_shape_check") {
		t.Error("the shape CHECK this refusal anticipates is missing from the migration")
	}
}

// P1 — deregistering a target resolves the work it strands, in the same
// transaction that deregisters it.
//
// The lock only orders the two. An apply that wins the race still commits a row
// against a target nothing will ever drain, and a row that never drains counts
// as queued — which reads as "recorded". So the disable has to sweep, and the
// sweep has to be in the same transaction as the state change: split across
// two, an apply committing between them lands in exactly the gap the sweep just
// left.
func TestDeregisteringATargetResolvesTheWorkItStrands(t *testing.T) {
	src, err := os.ReadFile("targets.go")
	if err != nil {
		t.Fatalf("read targets.go: %v", err)
	}
	body := string(src)

	if !strings.Contains(body, "InTx(ctx, func(tx pgx.Tx) error {") {
		t.Error("the disable must run in one transaction with its sweep — between two transactions, an apply commits into the gap")
	}
	stateChange := strings.Index(body, "UPDATE targets SET state = 'disabled'")
	sweep := strings.Index(body, "abandonQueuedWorkTx(ctx, tx,")
	if stateChange < 0 || sweep < 0 {
		t.Fatal("could not locate the state change and the sweep")
	}
	if sweep < stateChange {
		t.Error("the sweep must follow the state change: rows committed before the target row is locked are exactly the ones it exists to catch")
	}

	// Both unresolved states, because both are stranded. `pending` was never
	// sent; `in_flight` may have been, and the row records which by the column
	// that already means it.
	m := regexp.MustCompile(`(?s)UPDATE propagation_outbox\s+SET status = 'abandoned'(.*?)RETURNING (.*?)` + "`").FindStringSubmatch(body)
	if m == nil {
		t.Fatal("could not isolate the abandon statement")
	}
	for _, frag := range []string{"status IN ('pending', 'in_flight')", "completed_at = NOW()", "target = $1"} {
		if !strings.Contains(m[1], frag) {
			t.Errorf("the sweep must contain %q", frag)
		}
	}
	if !strings.Contains(m[2], "started_at IS NOT NULL") {
		t.Error("the sweep must report which rows were already in flight — that is the difference between work that certainly did not happen and work nobody can account for")
	}
	// Terminated, never deleted: the subject, the approval, and now a reason
	// have to survive, or an operator asking what happened to their change gets
	// nothing.
	if strings.Contains(body, "DELETE FROM propagation_outbox") {
		t.Error("deregistration must terminate stranded rows, not delete them")
	}
	// The audit write is part of the same statement, not a loop over its
	// results. A loop is skippable — by an empty slice, by an early return, by
	// an edit — and a data-modifying CTE runs to completion whether or not the
	// primary query reads it, so terminating a row and recording that it was
	// terminated become one write.
	if !regexp.MustCompile(`(?s)WITH abandoned AS \(.*?\), audited AS \(\s*INSERT INTO audit_logs.*?SELECT 'system', a\.user_id, \$2, a\.id FROM abandoned a\s*\)`).MatchString(body) {
		t.Error("the audit rows must be written by the same statement that terminates the work: a change that silently stopped existing is the failure this plane is built against")
	}
	if !strings.Contains(body, `"entitlement."+target+".abandoned"`) {
		t.Error("the audit action must name the target whose deregistration abandoned the work")
	}
}

// P1 — `abandoned` is a status the database accepts, and it is its own status.
// `failed` claims an attempt was made and did not work; `superseded` claims a
// later decision won. Neither happened: the target went away.
func TestAbandonedIsATerminalStatusOfItsOwn(t *testing.T) {
	up, _ := addonMigrationSQL(t)
	m := regexp.MustCompile(`(?is)ADD CONSTRAINT propagation_outbox_status_check\s+CHECK \(status IN \((.*?)\)\)`).FindStringSubmatch(up)
	if m == nil {
		t.Fatal("could not isolate the outbox status CHECK")
	}
	for _, status := range []string{"pending", "in_flight", "applied", "failed", "superseded", "abandoned"} {
		if !strings.Contains(m[1], "'"+status+"'") {
			t.Errorf("the status CHECK must accept %q", status)
		}
	}
}
