package db

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDropWebhookEventEnrichmentIncomplete_MigrationCoverage is a schema/code
// coherence guard. It cannot connect to a real database in a unit-test
// context, so instead it asserts that the migrations under db/migrations
// permit every (status, source_project) combination the
// DropWebhookEventEnrichmentIncomplete helper writes.
//
// Without this guard, the helper's literal status string and empty
// source_project drifted from migration 000007's CHECK constraints (audit
// regression: every real INSERT silently failed under the 'non-fatal' log
// path, leaving operators with no observable dropped row).
//
// The contract this test enforces:
//  1. Some migration in db/migrations/*.up.sql MUST add
//     WebhookStatusDroppedEnrichmentIncomplete to a status CHECK
//     constraint on webhook_events.
//  2. Some migration MUST relax the source_project NOT-EMPTY check so the
//     dropped status is exempt — searched for via the status string
//     appearing in a CHECK expression that references source_project.
//
// Both branches use string search rather than parsing PG SQL; the cost is
// the rare false-positive (e.g. status name appears in a comment), the
// payoff is no dependency on a real database connection.
func TestDropWebhookEventEnrichmentIncomplete_MigrationCoverage(t *testing.T) {
	migrationsDir := findMigrationsDir(t)

	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		t.Fatalf("read migrations dir: %v", err)
	}

	statusAllowed := false
	sourceProjectRelaxed := false
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".up.sql") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(migrationsDir, e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		body := string(raw)

		// Every check covering the status column that the helper writes must
		// list the status constant. The latest ADD CONSTRAINT wins under
		// migrate-up semantics, so checking presence (not "in original 000007")
		// is the right read.
		if strings.Contains(body, "status IN") && strings.Contains(body, WebhookStatusDroppedEnrichmentIncomplete) {
			statusAllowed = true
		}
		// The relaxation gate: any CHECK that pairs source_project with the
		// dropped status — the only safe way to allow empty source_project
		// for this one status while keeping the original guard on others.
		if strings.Contains(body, "source_project") && strings.Contains(body, WebhookStatusDroppedEnrichmentIncomplete) {
			sourceProjectRelaxed = true
		}
	}

	if !statusAllowed {
		t.Errorf("no migration allows status=%q in webhook_events status CHECK; "+
			"DropWebhookEventEnrichmentIncomplete will fail at runtime",
			WebhookStatusDroppedEnrichmentIncomplete)
	}
	if !sourceProjectRelaxed {
		t.Errorf("no migration exempts status=%q from the source_project non-empty CHECK; "+
			"DropWebhookEventEnrichmentIncomplete writes source_project='' and will fail at runtime",
			WebhookStatusDroppedEnrichmentIncomplete)
	}
}

// findMigrationsDir resolves backend/db/migrations relative to this test's
// package directory (backend/internal/db). Robust against being invoked
// from the repo root or the backend module root.
func findMigrationsDir(t *testing.T) string {
	t.Helper()
	candidates := []string{
		filepath.Join("..", "..", "db", "migrations"),
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			return c
		}
	}
	t.Fatalf("could not locate db/migrations; tried %v", candidates)
	return ""
}
