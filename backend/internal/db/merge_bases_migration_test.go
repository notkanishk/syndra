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
