package db

import (
	"os"
	"strings"
	"testing"
)

// 11.6/11.7 — the vault holds no credential, and the guard is paired with the
// migration that made it true.
//
// `internal/db` has no live-database harness, so what is asserted is that the
// schema and the statements written against it agree: no column can hold a
// hash, and nothing selects, inserts or updates one.

func TestNoColumnOnTheVaultCanHoldACredential(t *testing.T) {
	src := readSource(t, "vault.go")

	// The three columns the reduction dropped, named individually rather than
	// scanned for by keyword: a guard that looks for "hash" would pass the day
	// somebody calls the column `secret`.
	for _, gone := range []string{"credential_hash", "salt_params", "argon2"} {
		if strings.Contains(src, gone) {
			t.Errorf("vault.go still names %q; the column it referred to is gone and a statement naming it fails at runtime", gone)
		}
	}
	// And the closed list, so a column added later has to be argued for here.
	// One column: the member. Everything else on the row is a timestamp the
	// database sets or a flag the conflict clause clears.
	// `between` keeps the marker it started at, so the comparison includes it.
	columns := between(t, src, "INSERT INTO shadow_credentials (", ")")
	if strings.TrimSpace(columns) != "INSERT INTO shadow_credentials (user_id" {
		t.Errorf("the insert names more than the member: (%s)", columns)
	}
}

func TestTheMigrationDropsTheHashAndKeepsTheMetadata(t *testing.T) {
	raw, err := os.ReadFile("../../db/migrations/000034_retire_the_lldap_bridge.up.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := string(raw)

	for _, dropped := range []string{"credential_hash", "algorithm", "salt_params"} {
		if !strings.Contains(sql, "DROP COLUMN IF EXISTS "+dropped) {
			t.Errorf("the migration does not drop %s", dropped)
		}
	}
	// The queue goes whole. A table nothing writes to and nothing reads is a
	// table somebody wires back up.
	if !strings.Contains(sql, "DROP TABLE IF EXISTS provisioning_intents") {
		t.Error("the intent queue must be dropped, not emptied")
	}
	// And the rows survive, marked. Deleting them would lose the difference
	// between "you enrolled before the change" and "you have never set one",
	// which are different sentences to somebody who remembers doing it.
	if strings.Contains(sql, "DELETE FROM shadow_credentials") {
		t.Error("the surviving rows are how a member is told to re-enrol; deleting them loses that")
	}
	if !strings.Contains(sql, "enrolled_before_cutover") {
		t.Error("nothing marks the pre-cutover rows")
	}
	// The default flips after the backfill. Left at TRUE it would mark every
	// future enrolment as pre-cutover — the same class of mistake as a `target`
	// column defaulting to 'zitadel'.
	backfill := strings.Index(sql, "UPDATE shadow_credentials SET enrolled_before_cutover = TRUE")
	flip := strings.Index(sql, "ALTER COLUMN enrolled_before_cutover SET DEFAULT FALSE")
	if backfill < 0 || flip < 0 || flip < backfill {
		t.Error("the default must be flipped to FALSE after the backfill, or every later enrolment is marked")
	}
}

// A pre-cutover row is not a usable credential, and the read must say so. The
// hash it described is gone; reporting it as set tells somebody they have a
// password that the next connection attempt will refuse.
func TestAPreCutoverRowReadsAsNeedingReEnrolment(t *testing.T) {
	src := readSource(t, "vault.go")
	if !strings.Contains(src, "HasCredential:    !beforeCutover") {
		t.Error("a pre-cutover row must not read as an enrolled member")
	}
	if !strings.Contains(src, "NeedsReEnrolment: beforeCutover") {
		t.Error("the surface has nothing to tell them with")
	}
}

// 11.4 — nothing calls the removed intent pipeline. A source guard rather than
// a compile error, because the compile error is what a deletion produces on the
// day it happens and this is what stops one being re-added.
func TestNothingReachesForTheRetiredBridge(t *testing.T) {
	for _, name := range []string{"vault.go", "propagations.go", "cascade.go", "grants.go"} {
		src := readSource(t, name)
		for _, gone := range []string{"provisioning_intents", "InsertProvisioningIntent", "lldap_group"} {
			if strings.Contains(src, gone) {
				t.Errorf("%s names %q, which no longer exists", name, gone)
			}
		}
	}
}
