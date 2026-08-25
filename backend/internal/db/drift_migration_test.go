package db

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// The db package is tested only via migration-coherence guards (no live DB).
// This asserts every drift_type / detection_source / status literal the Go
// layer writes is permitted by the CHECK constraints, and vice-versa.
//
// Reads every up-migration rather than 000016 alone. A constraint does not stay
// in the migration that introduced it — the MkAuth -> Syndra rename moved the
// drift_type values in 000025 — so pinning to one file asserts what the schema
// used to be, and passes while the running database disagrees.
func TestDriftMigrationEnumsMatchCode(t *testing.T) {
	dir := findMigrationsDir(t)
	ups, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	if err != nil || len(ups) == 0 {
		t.Fatalf("glob migrations: %v (found %d)", err, len(ups))
	}
	sort.Strings(ups)
	var b strings.Builder
	for _, p := range ups {
		content, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read migration %s: %v", filepath.Base(p), err)
		}
		b.Write(content)
		b.WriteString("\n")
	}
	sql := b.String()

	// Values the Go code actually writes (UpsertDriftItem, the *AndEnqueue triage
	// helpers, sweep, webhook).
	for _, v := range []string{"'webhook'", "'reconciliation_sweep'"} {
		if !strings.Contains(sql, v) {
			t.Errorf("detection_source %s written by code but missing from 000016 CHECK", v)
		}
	}
	// `target_only` is the post-add-on name for what 000016 called zitadel_only:
	// the value stopped naming its target once the target became a column beside
	// it (000026). Reading every up migration is what lets this assertion follow
	// the value across the rename instead of asserting the schema of 2026-06.
	for _, v := range []string{"'target_only'", "'syndra_only'"} {
		if !strings.Contains(sql, v) {
			t.Errorf("drift_type %s is written by code but permitted by no migration's CHECK", v)
		}
	}
	for _, v := range []string{"'pending_triage'", "'attributed'", "'revoked'", "'marked_external'"} {
		if !strings.Contains(sql, v) {
			t.Errorf("status %s written by code but missing from 000016 CHECK", v)
		}
	}
	// The partial-unique dedupe index must exist or UpsertDriftItem's ON CONFLICT breaks.
	if !strings.Contains(sql, "idx_drift_items_pending_unique") {
		t.Error("partial-unique dedupe index missing; UpsertDriftItem ON CONFLICT would fail")
	}
}
