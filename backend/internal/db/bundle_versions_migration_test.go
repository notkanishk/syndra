package db

import (
	"os"
	"strings"
	"testing"
)

// The db package has no live-DB harness, so migration coherence is what it can
// prove: that the columns and constraints the repository layer assumes are
// actually created, and that the down migration undoes what the up one did.

func TestBundleVersionsMigration_CreatesWhatTheRepositoryAssumes(t *testing.T) {
	up, err := os.ReadFile("../../db/migrations/000020_bundle_versions.up.sql")
	if err != nil {
		t.Fatalf("read up migration: %v", err)
	}
	sql := string(up)

	for _, want := range []string{
		"CREATE TABLE IF NOT EXISTS bundle_versions",
		"CREATE TABLE IF NOT EXISTS bundle_version_roles",
		// Per-bundle numbering. A global sequence would make "Lab Tech v2"
		// meaningless as something a person says out loud.
		"UNIQUE (bundle_id, version)",
		"ADD COLUMN IF NOT EXISTS version_id UUID",
		// Every assignment must resolve through a version; a nullable pin is an
		// assignment whose access cannot be computed.
		"ALTER COLUMN version_id SET NOT NULL",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("000020 up migration missing %q", want)
		}
	}
}

func TestBundleVersionsMigration_BackfillsBeforeRequiringTheColumn(t *testing.T) {
	up, err := os.ReadFile("../../db/migrations/000020_bundle_versions.up.sql")
	if err != nil {
		t.Fatalf("read up migration: %v", err)
	}
	sql := string(up)

	backfill := strings.Index(sql, "UPDATE user_bundle_assignments uba")
	notNull := strings.Index(sql, "ALTER COLUMN version_id SET NOT NULL")
	if backfill == -1 || notNull == -1 {
		t.Fatal("expected both a backfill and a NOT NULL step")
	}
	// Order is the whole correctness of this migration: requiring the column
	// before filling it fails on any deployment with an existing assignment.
	if backfill > notNull {
		t.Error("the backfill must run before version_id is made NOT NULL")
	}

	v1 := strings.Index(sql, "INSERT INTO bundle_versions")
	if v1 == -1 || v1 > backfill {
		t.Error("v1 must exist before assignments can be pinned to it")
	}
}

func TestBundleVersionsMigration_DownLeavesTheWorkingCopyAlone(t *testing.T) {
	down, err := os.ReadFile("../../db/migrations/000020_bundle_versions.down.sql")
	if err != nil {
		t.Fatalf("read down migration: %v", err)
	}
	sql := string(down)

	for _, want := range []string{
		"DROP COLUMN IF EXISTS version_id",
		"DROP TABLE IF EXISTS bundle_version_roles",
		"DROP TABLE IF EXISTS bundle_versions",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("000020 down migration missing %q", want)
		}
	}
	// bundle_roles is the working copy and predates versioning. Dropping it on
	// the way down would delete every bundle's contents.
	if strings.Contains(sql, "DROP TABLE IF EXISTS bundle_roles") {
		t.Error("the down migration must not drop the working copy")
	}
}
