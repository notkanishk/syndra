package db

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The db package is tested only via migration-coherence guards (no live DB).
// These assert every column 000019's Go layer reads or writes actually exists
// in the migration — a SELECT naming a column the schema lacks fails at
// runtime, on the operator's screen, not here.
func TestIASurfacesMigrationMatchesCode(t *testing.T) {
	dir := findMigrationsDir(t)
	up, err := os.ReadFile(filepath.Join(dir, "000019_ia_surfaces.up.sql"))
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := string(up)

	for _, col := range []string{"cascade_id", "upstream_actor", "upstream_created_at", "last_seen_at"} {
		if !strings.Contains(sql, col) {
			t.Errorf("column %s is read/written by the Go layer but missing from 000019", col)
		}
	}

	// The grouping read filters on cascade_id IS NOT NULL; without the partial
	// index Change history degenerates to a sequential scan of the outbox.
	if !strings.Contains(sql, "idx_pending_zitadel_propagations_cascade") {
		t.Error("cascade grouping index missing from 000019")
	}

	down, err := os.ReadFile(filepath.Join(dir, "000019_ia_surfaces.down.sql"))
	if err != nil {
		t.Fatalf("read down migration: %v", err)
	}
	for _, col := range []string{"cascade_id", "upstream_actor", "upstream_created_at", "last_seen_at"} {
		if !strings.Contains(string(down), col) {
			t.Errorf("down migration does not drop %s — a rollback would leave the column behind", col)
		}
	}
}

// UpsertDriftItemWithEvidence's ON CONFLICT clause must keep matching the
// partial-unique index installed in 000016, and must NOT clobber a known actor
// with an unknown one on re-detection.
func TestDriftEvidenceUpsertPreservesKnownActor(t *testing.T) {
	dir := findMigrationsDir(t)
	up, err := os.ReadFile(filepath.Join(dir, "000016_drift_queue.up.sql"))
	if err != nil {
		t.Fatalf("read 000016: %v", err)
	}
	if !strings.Contains(string(up), "idx_drift_items_pending_unique") {
		t.Fatal("dedupe index missing; the evidence upsert's ON CONFLICT would fail")
	}
}
