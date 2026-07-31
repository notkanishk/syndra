package db

import (
	"os"
	"strings"
	"testing"

	"mkauth/internal/claims"
)

// The claim-shaping tables are the operator's control over what a token
// carries. A drift between the CHECK constraint and the Go format constants
// would surface as a 500 on save, so the two are pinned together here — the
// db package has no live-DB harness, migration coherence is what it can prove.
func TestMigration000018_ClaimShapingMatchesGo(t *testing.T) {
	up, err := os.ReadFile("../../db/migrations/000018_claim_shaping.up.sql")
	if err != nil {
		t.Fatalf("read up migration: %v", err)
	}
	s := string(up)

	for _, want := range []string{
		"ALTER TABLE claim_profiles",
		"ADD COLUMN IF NOT EXISTS attribute_claims JSONB",
		"ADD COLUMN IF NOT EXISTS static_claims JSONB",
		"CREATE TABLE IF NOT EXISTS app_claim_overrides",
		"application_id VARCHAR(255) UNIQUE NOT NULL",
		"ck_app_claim_overrides_format_type",
		// Two applications on one project may not claim the same key.
		"idx_app_claim_overrides_project_claim",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("000018 up migration missing %q", want)
		}
	}

	// Every format the Go shaper accepts must be permitted by the constraint.
	for _, format := range []string{claims.FormatArray, claims.FormatCSV, claims.FormatSpaceDelimited} {
		if !strings.Contains(s, "'"+format+"'") {
			t.Errorf("format CHECK does not cover Go literal %q", format)
		}
	}

	down, err := os.ReadFile("../../db/migrations/000018_claim_shaping.down.sql")
	if err != nil {
		t.Fatalf("read down migration: %v", err)
	}
	d := string(down)
	for _, want := range []string{
		"DROP TABLE IF EXISTS app_claim_overrides",
		"DROP COLUMN IF EXISTS attribute_claims",
		"DROP COLUMN IF EXISTS static_claims",
	} {
		if !strings.Contains(d, want) {
			t.Errorf("000018 down migration missing %q", want)
		}
	}
}

// The columns the queries read must be the columns the migration creates.
// A rename on either side turns into a scan error at request time.
func TestClaimProfileQueriesMatchMigration(t *testing.T) {
	up, err := os.ReadFile("../../db/migrations/000018_claim_shaping.up.sql")
	if err != nil {
		t.Fatalf("read up migration: %v", err)
	}
	init, err := os.ReadFile("../../db/migrations/000001_init_schema.up.sql")
	if err != nil {
		t.Fatalf("read init migration: %v", err)
	}
	schema := string(init) + string(up)

	for _, column := range strings.Split(claimProfileColumns, ", ") {
		if !strings.Contains(schema, strings.TrimSpace(column)) {
			t.Errorf("claim profile SELECT reads %q, which no migration creates", column)
		}
	}
}
