package db

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMarkOnboardingTriggerUnconfigured_MigrationCoverage is a schema/code
// coherence guard, same shape as
// TestDropWebhookEventEnrichmentIncomplete_MigrationCoverage: the db package
// has no live-DB harness, so this asserts by string search that some
// migration actually permits the status MarkOnboardingTriggerUnconfigured
// writes, rather than trusting that migration 000046 stayed in sync with the
// Go constant it was written for.
func TestMarkOnboardingTriggerUnconfigured_MigrationCoverage(t *testing.T) {
	migrationsDir := findMigrationsDir(t)

	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		t.Fatalf("read migrations dir: %v", err)
	}

	statusAllowed := false
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".up.sql") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(migrationsDir, e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		body := string(raw)
		if strings.Contains(body, "status IN") && strings.Contains(body, OnboardingStatusUnconfigured) {
			statusAllowed = true
		}
	}

	if !statusAllowed {
		t.Errorf("no migration allows status=%q in onboarding_triggers' status CHECK; "+
			"MarkOnboardingTriggerUnconfigured will fail at runtime", OnboardingStatusUnconfigured)
	}
}
