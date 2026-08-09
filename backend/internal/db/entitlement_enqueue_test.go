package db

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
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

func entitlementInsert(t *testing.T) string {
	t.Helper()
	src, err := os.ReadFile("entitlement_enqueue.go")
	if err != nil {
		t.Fatalf("read entitlement_enqueue.go: %v", err)
	}
	m := regexp.MustCompile(`(?s)INSERT INTO propagation_outbox\s*\n\s*\((.*?)\)\s*\n\s*VALUES \((.*?)\)`).FindStringSubmatch(string(src))
	if m == nil {
		t.Fatal("could not isolate the entitlement outbox INSERT")
	}
	return m[1] + " || " + m[2]
}

// 2.9 — the row is written to the target it is for. `target` carries
// `DEFAULT 'zitadel'`, which makes omitting it silent: the write succeeds and
// the row queues a TrueNAS convergence against Zitadel, where the drain will
// find no project and no roles.
func TestAnEntitlementRowNamesItsTargetAndItsApproval(t *testing.T) {
	insert := entitlementInsert(t)

	if !strings.Contains(insert, "target") {
		t.Error("the entitlement enqueue must write `target` explicitly — the column defaults to zitadel, so leaving it out is a wrong write that raises nothing")
	}
	if !strings.Contains(insert, "plan_subject_id") {
		t.Error("the entitlement enqueue must cite the plan subject: without it the drain has no fingerprint to re-verify and no record of what was approved")
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

// 2.9 — what an entitlement row refuses to become, decided before the database
// is touched. The nil transaction is the proof: a check that ran after the
// first statement would panic here rather than fail.
func TestAnEntitlementEnqueueRefusesAnUnqueueableRow(t *testing.T) {
	good := EntitlementApply{
		Target:        "truenas",
		SubjectID:     "u1",
		PlanSubjectID: sampleUUID,
		InitiatedBy:   "operator-1",
	}

	cases := []struct {
		name   string
		mutate func(*EntitlementApply)
	}{
		{"no target", func(p *EntitlementApply) { p.Target = " " }},
		{"the built-in target", func(p *EntitlementApply) { p.Target = TargetZitadel }},
		{"no subject", func(p *EntitlementApply) { p.SubjectID = "" }},
		{"no approval behind it", func(p *EntitlementApply) { p.PlanSubjectID = "" }},
		{"a malformed approval reference", func(p *EntitlementApply) { p.PlanSubjectID = "plan-subject-7" }},
		{"nobody initiating it", func(p *EntitlementApply) { p.InitiatedBy = "" }},
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
	_, err := EnqueueEntitlementApplyTx(context.Background(), nil, EntitlementApply{
		Target: TargetZitadel, SubjectID: "u1", PlanSubjectID: sampleUUID, InitiatedBy: "operator-1",
	})
	if !errors.Is(err, ErrInvalidEnqueue) {
		t.Fatalf("err = %v, want ErrInvalidEnqueue", err)
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
