package db

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The db package is tested only via migration-coherence guards (no live DB).
//
// DeleteBundleAndEnqueue issues a bare DELETE and relies entirely on the schema to take the
// bundle's roles, versions and assignments with it. Postgres would refuse the DELETE outright if
// anything still referenced the row without ON DELETE CASCADE — so this asserts the one reference
// that deliberately does NOT cascade has had its foreign key removed instead.
func TestBundleDeleteHasNoBlockingReferences(t *testing.T) {
	dir := findMigrationsDir(t)

	up, err := os.ReadFile(filepath.Join(dir, "000021_bundle_lifecycle.up.sql"))
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := string(up)
	if !strings.Contains(sql, "DROP CONSTRAINT IF EXISTS onboarding_triggers_bundle_id_fkey") {
		t.Error("onboarding_triggers.bundle_id still references bundles(id) without ON DELETE; " +
			"every bundle that ever onboarded somebody would be undeletable")
	}
	// The column stays. A trigger row saying "onboarded, and given this bundle" is still true
	// after the bundle is retired; dropping or nulling it would rewrite history rather than
	// record it.
	if strings.Contains(sql, "DROP COLUMN") || strings.Contains(sql, "SET NULL") {
		t.Error("the bundle_id column and its value must survive the bundle's deletion")
	}

	// Every other reference must cascade, or the DELETE fails on it.
	for file, table := range map[string]string{
		"000001_init_schema.up.sql":     "bundle_roles",
		"000002_user_bundles.up.sql":    "user_bundle_assignments",
		"000020_bundle_versions.up.sql": "bundle_versions",
	} {
		b, err := os.ReadFile(filepath.Join(dir, file))
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		if !strings.Contains(string(b), "REFERENCES bundles(id) ON DELETE CASCADE") {
			t.Errorf("%s must reference bundles(id) ON DELETE CASCADE (%s)", table, file)
		}
	}
}

// WithdrawAccessRequest writes status='withdrawn' with reviewer_user_id left NULL and
// resolved_at set. Both CHECK constraints on access_requests have to admit exactly that shape —
// the first would reject the status, the second the missing reviewer.
func TestWithdrawnRequestShapeIsPermittedByConstraints(t *testing.T) {
	dir := findMigrationsDir(t)
	up, err := os.ReadFile(filepath.Join(dir, "000022_request_withdrawal.up.sql"))
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := string(up)

	if !strings.Contains(sql, "'pending', 'approved', 'rejected', 'withdrawn'") {
		t.Error("ck_access_requests_status_enum must admit 'withdrawn'")
	}
	if !strings.Contains(sql, "status = 'withdrawn' AND reviewer_user_id IS NULL AND resolved_at IS NOT NULL") {
		t.Error("the resolution invariant must admit a withdrawal: resolved, with no reviewer")
	}
	// Guard against the lazier version of this migration — widening the decided branch to
	// include 'withdrawn' would let a withdrawal be written as a decision somebody took.
	if strings.Contains(sql, "'approved', 'rejected', 'withdrawn') AND reviewer_user_id IS NOT NULL") {
		t.Error("a withdrawal must not be admitted as a reviewed decision")
	}
}
