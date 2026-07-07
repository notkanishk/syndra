package db

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestClaimPendingPropagations_ReclaimsInFlight is a source-coherence guard (the
// db package has no live-DB harness — see the migration coverage test): it reads
// the repository source and asserts the claim query and the pending-worklist
// queries agree on the SAME non-terminal status set. If the claim query only
// took 'pending' rows while GetPending/CountPending report 'pending'+'in_flight'
// (the original bug), a drain that crashed mid-flight would leave in_flight rows
// that the operator sees in the worklist forever but that no drain ever reclaims
// (design.md §Drain: "status in (pending,in_flight)→in_flight", the already-
// exists check "covers crash-recovery").
func TestClaimPendingPropagations_ReclaimsInFlight(t *testing.T) {
	src, err := os.ReadFile("propagations.go")
	if err != nil {
		t.Fatalf("read propagations.go: %v", err)
	}
	// Match a WHERE ... IN (...) that lists BOTH statuses. A plain substring check
	// would be fooled by the claim's `SET status = 'in_flight'` target, so we pin
	// the claimable set (the IN list), not any mention of the literal.
	claimable := regexp.MustCompile(`status\s+IN\s*\([^)]*'pending'[^)]*'in_flight'[^)]*\)`)

	claim := funcBody(t, string(src), "ClaimPendingPropagations")
	if !claimable.MatchString(claim) {
		t.Errorf("ClaimPendingPropagations must select WHERE status IN ('pending','in_flight') so crash-orphaned in_flight work is reclaimed; body:\n%s", claim)
	}

	// The worklist and count MUST cover the same set the claim drains, or the
	// operator's "pending" depth diverges from what a drain can actually clear.
	for _, fn := range []string{"GetPendingPropagations", "CountPendingPropagations"} {
		body := funcBody(t, string(src), fn)
		if !claimable.MatchString(body) {
			t.Errorf("%s must report the same pending+in_flight set the claim drains", fn)
		}
	}
}

// TestReconcileLedgerOnApplied_RevokeIsSourceScoped is a source-coherence guard for the
// sub-phase-3 fix (review P1): an applied revoke must delete direct_role_grants scoped to the
// outbox row's OWN source, not unconditionally. Cascades (source='bundle'|'rule') write no
// ledger rows, so a cascade revoke's reconcile must delete nothing for that triple — an
// unscoped delete would strip an operator's source='direct' row sharing the same
// (user, project, role). The db package has no live-DB harness (see file doc), so this asserts
// the revoke branch's SQL carries an `AND source=$4` (or equivalent) predicate rather than
// exercising it against a real database. The replace branch is intentionally NOT re-checked
// here — it was already source='direct'-scoped before this task.
func TestReconcileLedgerOnApplied_RevokeIsSourceScoped(t *testing.T) {
	src, err := os.ReadFile("propagations.go")
	if err != nil {
		t.Fatalf("read propagations.go: %v", err)
	}
	fn := funcBody(t, string(src), "ReconcileLedgerOnApplied")
	revokeCase := regexp.MustCompile(`(?s)case "revoke":.*?case "replace":`).FindString(fn)
	if revokeCase == "" {
		t.Fatal(`could not isolate the "revoke" case body in ReconcileLedgerOnApplied`)
	}
	if !regexp.MustCompile(`AND\s+source\s*=\s*\$4`).MatchString(revokeCase) {
		t.Errorf("revoke branch must scope its DELETE by source (AND source=$4) so a cascade revoke never strips an operator's direct grant; body:\n%s", revokeCase)
	}
}

// TestEnqueueWrites_WritesSourceAndSourceRef is a coherence guard for the shared
// direct/drift enqueue path (enqueueWrites in propagation_enqueue.go): its outbox INSERT must
// carry source/source_ref, same as the cascade path's enqueueCascadeRows
// (TestEnqueueCascadeRows_WritesSourceAndSourceRef in cascade_migration_test.go). Without this,
// callers that route through enqueueWrites with a non-default Source/SourceRef — e.g.
// handlers/drift.go's *AndEnqueue variant — would have their pending-worklist attribution
// silently dropped (source always 'direct', source_ref always NULL) even though the
// direct_role_grants ledger upsert on the same tx got it right.
func TestEnqueueWrites_WritesSourceAndSourceRef(t *testing.T) {
	src, err := os.ReadFile("propagation_enqueue.go")
	if err != nil {
		t.Fatalf("read propagation_enqueue.go: %v", err)
	}
	fb := funcBody(t, string(src), "enqueueWrites")
	for _, want := range []string{"source", "source_ref"} {
		if !strings.Contains(fb, want) {
			t.Errorf("enqueueWrites must write outbox column %q; body:\n%s", want, fb)
		}
	}
	// Pin it to the actual INSERT INTO pending_zitadel_propagations statement, not just any
	// mention of "source" in the function (e.g. the ledger upsert or the `source :=` default
	// above it also contain that substring).
	insert := regexp.MustCompile(`(?s)INSERT INTO pending_zitadel_propagations.*?RETURNING id`).FindString(fb)
	if insert == "" {
		t.Fatal("could not isolate the pending_zitadel_propagations INSERT in enqueueWrites")
	}
	for _, want := range []string{"source", "source_ref"} {
		if !strings.Contains(insert, want) {
			t.Errorf("pending_zitadel_propagations INSERT in enqueueWrites must list column %q; insert:\n%s", want, insert)
		}
	}
}

// funcBody returns the source text of the named top-level func, from its
// declaration to the next top-level func (or EOF). Good enough for a string
// coherence assertion without pulling in go/parser.
func funcBody(t *testing.T, src, name string) string {
	t.Helper()
	start := regexp.MustCompile(`(?m)^func ` + regexp.QuoteMeta(name) + `\b`).FindStringIndex(src)
	if start == nil {
		t.Fatalf("func %s not found in source", name)
	}
	rest := src[start[1]:]
	if next := regexp.MustCompile(`(?m)^func `).FindStringIndex(rest); next != nil {
		return rest[:next[0]]
	}
	return rest
}

// TestPropagationOutbox_MigrationCoverage is a schema/code coherence guard in
// the same spirit as TestDropWebhookEventEnrichmentIncomplete_MigrationCoverage.
// It cannot connect to a real database in a unit-test context, so it asserts
// that migration 000015 declares every enum value the Go layer writes, and that
// the down migration is symmetric. Without this guard the CHECK constraints can
// silently drift from the string literals the repository/enqueue/drain code
// emits, so every real INSERT would fail under runtime error paths instead of
// in CI.
func TestPropagationOutbox_MigrationCoverage(t *testing.T) {
	dir := findMigrationsDir(t)

	up, err := os.ReadFile(filepath.Join(dir, "000015_zitadel_propagation_outbox.up.sql"))
	if err != nil {
		t.Fatalf("read up migration: %v", err)
	}
	down, err := os.ReadFile(filepath.Join(dir, "000015_zitadel_propagation_outbox.down.sql"))
	if err != nil {
		t.Fatalf("read down migration: %v", err)
	}
	upSQL := string(up)
	downSQL := string(down)

	// 1. The table exists and there is NO `confirmed` status (design Decision 1).
	if !strings.Contains(upSQL, "CREATE TABLE IF NOT EXISTS pending_zitadel_propagations") {
		t.Fatal("up migration must create pending_zitadel_propagations")
	}
	if strings.Contains(upSQL, "'confirmed'") {
		t.Error("outbox must NOT define a 'confirmed' status (design Decision 1)")
	}

	// 2. Every op_type the drain dispatches must be permitted by the CHECK.
	for _, op := range []string{"add", "revoke", "replace"} {
		if !strings.Contains(upSQL, "'"+op+"'") {
			t.Errorf("op_type %q is dispatched by the drain but absent from the migration CHECK", op)
		}
	}

	// 3. Every status the repository transitions through must be permitted.
	for _, st := range []string{"pending", "in_flight", "applied", "failed"} {
		if !strings.Contains(upSQL, "'"+st+"'") {
			t.Errorf("status %q is written by the repository but absent from the migration CHECK", st)
		}
	}

	// 4. The full 5-value source enum must be installed now (sub-phase 3 needs no
	//    further ALTER) and added to direct_role_grants.
	if !strings.Contains(upSQL, "ADD COLUMN IF NOT EXISTS source") {
		t.Error("up migration must add direct_role_grants.source")
	}
	for _, src := range []string{"direct", "bundle", "rule", "external_backfill", "lifecycle_cascade"} {
		if !strings.Contains(upSQL, "'"+src+"'") {
			t.Errorf("source value %q absent from migration CHECK", src)
		}
	}
	if !strings.Contains(upSQL, "ADD COLUMN IF NOT EXISTS source_ref") {
		t.Error("up migration must add direct_role_grants.source_ref")
	}

	// 5. idempotency_key must be UNIQUE — the crash-recovery / double-enqueue net.
	if !strings.Contains(upSQL, "idempotency_key") || !strings.Contains(upSQL, "UNIQUE") {
		t.Error("idempotency_key must be declared UNIQUE")
	}

	// 6. Down migration must reverse all three structural changes.
	for _, want := range []string{
		"DROP TABLE IF EXISTS pending_zitadel_propagations",
		"DROP COLUMN IF EXISTS source",
		"DROP COLUMN IF EXISTS source_ref",
	} {
		if !strings.Contains(downSQL, want) {
			t.Errorf("down migration missing %q", want)
		}
	}
}
