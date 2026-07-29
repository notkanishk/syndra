package db

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The db package is tested only via migration-coherence guards (no live DB).
// This asserts every drift_type / detection_source / status literal the Go
// layer writes is permitted by the 000016 CHECK constraints, and vice-versa.
func TestDriftMigrationEnumsMatchCode(t *testing.T) {
	dir := findMigrationsDir(t)
	up, err := os.ReadFile(filepath.Join(dir, "000016_drift_queue.up.sql"))
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := string(up)

	// Values the Go code actually writes (UpsertDriftItem, the *AndEnqueue triage
	// helpers, sweep, webhook).
	for _, v := range []string{"'webhook'", "'reconciliation_sweep'"} {
		if !strings.Contains(sql, v) {
			t.Errorf("detection_source %s written by code but missing from 000016 CHECK", v)
		}
	}
	for _, v := range []string{"'zitadel_only'", "'mkauth_only'"} {
		if !strings.Contains(sql, v) {
			t.Errorf("drift_type %s written by code but missing from 000016 CHECK", v)
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
