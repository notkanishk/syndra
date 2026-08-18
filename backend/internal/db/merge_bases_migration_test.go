package db

import (
	"os"
	"strings"
	"testing"
)

// The db package has no live-DB harness, so migration coherence is what it can
// prove: that the columns and constraints this repository assumes are created,
// and that the down migration undoes what the up one did.

func TestMergeBasesMigration_CreatesWhatTheRepositoryAssumes(t *testing.T) {
	up, err := os.ReadFile("../../db/migrations/000041_target_merge_bases.up.sql")
	if err != nil {
		t.Fatalf("read up migration: %v", err)
	}
	sql := string(up)

	for _, want := range []string{
		"CREATE TABLE IF NOT EXISTS target_merge_bases",
		// One base per subject per target: the upsert's conflict target.
		"PRIMARY KEY (target, subject_id)",
		"REFERENCES targets(target)",
		// The value shape belongs to the target, not to this schema.
		"base       JSONB        NOT NULL",
		// The observation's own time, which is what an operator is asking about
		// when they ask what a value used to be.
		"observed_at TIMESTAMPTZ NOT NULL",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("000041 up migration missing %q", want)
		}
	}
}

// An empty base is not a base — it would classify every managed field as
// changed-by-them on the next pass, which is a finding manufactured by
// bookkeeping. The repository refuses one; the schema refuses it too, because a
// second writer will exist eventually and the constraint is what holds then.
func TestMergeBasesMigration_RefusesAnEmptyObservation(t *testing.T) {
	up, err := os.ReadFile("../../db/migrations/000041_target_merge_bases.up.sql")
	if err != nil {
		t.Fatalf("read up migration: %v", err)
	}
	sql := string(up)
	if !strings.Contains(sql, "target_merge_base_is_not_empty") {
		t.Error("the schema must refuse an empty base")
	}
	if !strings.Contains(sql, "base <> '{}'::jsonb") {
		t.Error("the emptiness check must be on the object, not only on its type")
	}
}

func TestMergeBasesMigration_DownDropsWhatUpCreated(t *testing.T) {
	down, err := os.ReadFile("../../db/migrations/000041_target_merge_bases.down.sql")
	if err != nil {
		t.Fatalf("read down migration: %v", err)
	}
	if !strings.Contains(string(down), "DROP TABLE IF EXISTS target_merge_bases") {
		t.Error("the down migration must drop the table the up one created")
	}
}

// A decision is not a settlement (000043).
//
// Keeping Syndra's state queues a convergence; the difference is still there
// when the operator's request returns. Closing the finding then would let the
// next sweep raise a second one about the same field — one decision producing a
// queue that refills itself every six hours until the drain caught up.
func TestMergeFindingsMigration_SeparatesDecisionFromSettlement(t *testing.T) {
	up, err := os.ReadFile("../../db/migrations/000043_a_decision_is_not_a_settlement.up.sql")
	if err != nil {
		t.Fatalf("read up migration: %v", err)
	}
	sql := string(up)

	for _, want := range []string{
		"ADD COLUMN IF NOT EXISTS decision",
		"ADD COLUMN IF NOT EXISTS decided_by",
		"ADD COLUMN IF NOT EXISTS decided_at",
		// A choice with no author is a finding that decided itself — the same
		// rule the resolution columns already carry.
		"merge_finding_decision_is_attributed",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("000043 up migration missing %q", want)
		}
	}
}

// The findings table itself: one standing row per subject and field, with the
// account-level slot occupying its own rather than colliding with a field.
func TestMergeFindingsMigration_DedupesOneStandingFindingPerField(t *testing.T) {
	up, err := os.ReadFile("../../db/migrations/000042_target_merge_findings.up.sql")
	if err != nil {
		t.Fatalf("read up migration: %v", err)
	}
	sql := string(up)

	for _, want := range []string{
		"CREATE TABLE IF NOT EXISTS target_merge_findings",
		"ON target_merge_findings (target, subject_id, field)",
		"WHERE resolved_at IS NULL",
		// Empty rather than NULL, or the uniqueness index treats each
		// account-level row as distinct: NULL is never equal to itself.
		"field      TEXT         NOT NULL DEFAULT ''",
		// Only the three outcomes a pass may not resolve.
		"CHECK (outcome IN ('theirs_only', 'conflict', 'deleted_upstream'))",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("000042 up migration missing %q", want)
		}
	}
}

// The memory that a write landed (000044).
//
// A grant applied and removed between two sweeps has no observation behind it,
// so its absence reads as a write that never happened — and gets replayed,
// restoring access somebody removed on purpose. The outbox cannot answer it:
// terminal rows are pruned, and there is no `confirmed` state by design.
func TestPropagationsMigration_KeepsTheEvidenceTheOutboxDiscards(t *testing.T) {
	up, err := os.ReadFile("../../db/migrations/000044_target_propagations.up.sql")
	if err != nil {
		t.Fatalf("read up migration: %v", err)
	}
	sql := string(up)

	for _, want := range []string{
		"CREATE TABLE IF NOT EXISTS target_propagations",
		// One row per thing written, overwritten by each later success: the
		// question is "when did this last land", not a history of every apply.
		"PRIMARY KEY (target, subject_id, field)",
		"applied_at TIMESTAMPTZ  NOT NULL",
		// The thread back to what authorised the write, kept past the outbox
		// row's own retention.
		"outbox_id  UUID",
		"actor      VARCHAR(255) NOT NULL",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("000044 up migration missing %q", want)
		}
	}
}

func TestPropagationsMigration_DownDropsWhatUpCreated(t *testing.T) {
	down, err := os.ReadFile("../../db/migrations/000044_target_propagations.down.sql")
	if err != nil {
		t.Fatalf("read down migration: %v", err)
	}
	if !strings.Contains(string(down), "DROP TABLE IF EXISTS target_propagations") {
		t.Error("the down migration must drop the table the up one created")
	}
}
