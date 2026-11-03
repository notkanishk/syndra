package db

import (
	"os"
	"strings"
	"testing"
	"time"

	"syndra/internal/claims"
)

// The column GetClaimMode reads is the one 000073 adds, and the down drops it.
func TestClaimModeQueryMatchesMigration000073(t *testing.T) {
	up, err := os.ReadFile("../../db/migrations/000073_claim_lkg_bound.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	down, err := os.ReadFile("../../db/migrations/000073_claim_lkg_bound.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(up), "ADD COLUMN IF NOT EXISTS claim_lkg_max_age_seconds INTEGER") ||
		!strings.Contains(string(down), "DROP COLUMN IF EXISTS claim_lkg_max_age_seconds") {
		t.Fatal("000073 does not add and drop claim_lkg_max_age_seconds")
	}
}

// A project's whole degraded-mode setting, and every project with claim
// config, read live (SYNDRA_TEST_DATABASE_URL).
func TestClaimMode_Live(t *testing.T) {
	ctx := liveDB(t)
	stamp := time.Now().Format("150405.000000")
	pid, shaped := "pm"+stamp, "ps"+stamp
	t.Cleanup(func() {
		_, _ = PG.Exec(ctx, `DELETE FROM claim_profiles WHERE zitadel_project_id = $1`, pid)
		_, _ = PG.Exec(ctx, `DELETE FROM app_claim_shape_versions WHERE project_id = $1`, shaped)
	})

	if m, err := GetClaimMode(ctx, pid); err != nil || m.Mode != "fail_closed" || m.LKGMaxAgeSeconds != 0 {
		t.Fatalf("no profile: %+v %v", m, err)
	}
	if err := UpsertClaimProfile(ctx, ClaimProfileRow{ProjectID: pid, ClaimName: "roles", FormatType: "array"}); err != nil {
		t.Fatal(err)
	}
	if _, err := PG.Exec(ctx, `UPDATE claim_profiles SET claim_failure_mode = 'minimal_safe',
		minimal_safe_claims = '{"k":"v"}', claim_lkg_max_age_seconds = 900 WHERE zitadel_project_id = $1`, pid); err != nil {
		t.Fatal(err)
	}
	m, err := GetClaimMode(ctx, pid)
	if err != nil || m.Mode != "minimal_safe" || m.MinimalSafeClaims["k"] != "v" || m.LKGMaxAgeSeconds != 900 {
		t.Fatalf("set: %+v %v", m, err)
	}
	if _, err := PG.Exec(ctx, `UPDATE claim_profiles SET claim_lkg_max_age_seconds = 0 WHERE zitadel_project_id = $1`, pid); err == nil {
		t.Fatal("a zero bound was accepted")
	}

	if _, _, err := InsertClaimShapeVersion(ctx, ClaimShapeVersion{ApplicationID: "a" + stamp, ProjectID: shaped,
		Profile: claims.Builtin(), SavedBy: "op"}, false); err != nil {
		t.Fatal(err)
	}
	ids, err := ClaimConfigProjects(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, id := range ids {
		found[id] = true
	}
	if !found[pid] || !found[shaped] {
		t.Fatalf("claim config projects miss %s or %s: %v", pid, shaped, ids)
	}
}
