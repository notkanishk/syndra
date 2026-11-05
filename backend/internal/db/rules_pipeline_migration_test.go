package db

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const (
	m78Up   = "../../db/migrations/000078_drop_confirmation_mode_and_app_overrides.up.sql"
	m78Down = "../../db/migrations/000078_drop_confirmation_mode_and_app_overrides.down.sql"
)

func readMigration(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

// 000078 drops two things whose loss is only safe under a condition, and the
// condition is checked by the migration itself, before anything is dropped.
func TestMigration000078_RefusesBeforeItDrops(t *testing.T) {
	up := readMigration(t, m78Up)
	guard := strings.Index(up, "RAISE EXCEPTION")
	firstDrop := strings.Index(up, "DROP ")
	if guard < 0 || firstDrop < 0 || guard > firstDrop {
		t.Fatal("the refusals must come before the first DROP")
	}
	for _, want := range []string{
		"key = 'confirmation.model'), '') <> 'roles'",
		"IF EXISTS (SELECT 1 FROM app_claim_overrides)",
		"ALTER TABLE bundles       DROP COLUMN IF EXISTS confirmation_mode",
		"ALTER TABLE mapping_rules DROP COLUMN IF EXISTS confirmation_mode",
		"DELETE FROM config_settings WHERE key = 'global.default_rule_confirmation_mode'",
		"DROP TABLE IF EXISTS app_claim_overrides",
	} {
		if !strings.Contains(up, want) {
			t.Errorf("000078 up is missing %q", want)
		}
	}
	down := readMigration(t, m78Down)
	for _, want := range []string{
		"bundles\n    ADD COLUMN IF NOT EXISTS confirmation_mode TEXT NOT NULL DEFAULT 'auto'",
		"mapping_rules\n    ADD COLUMN IF NOT EXISTS confirmation_mode TEXT NOT NULL DEFAULT 'auto'",
		"CHECK (confirmation_mode IN ('auto', 'manual'))",
		"'global.default_rule_confirmation_mode', 'auto'",
		"CREATE TABLE IF NOT EXISTS app_claim_overrides",
		"ck_app_claim_overrides_format_type",
		"idx_app_claim_overrides_project_claim",
		"idx_app_claim_overrides_project\n",
	} {
		if !strings.Contains(down, want) {
			t.Errorf("000078 down is missing %q", want)
		}
	}
}

// Nothing in this package reads or writes what 000078 drops. A query naming a
// dropped column fails at run time on every call, so it is caught here.
func TestNoQueryNamesWhat000078Dropped(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	gone := regexp.MustCompile(`confirmation_mode|app_claim_overrides|default_rule_confirmation_mode`)
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		if loc := gone.FindString(readDBSource(t, f)); loc != "" {
			t.Errorf("%s still names %q, which migration 000078 drops", f, loc)
		}
	}
}

// The refusals, against a real database. Each case runs in a transaction that
// is rolled back: 000078's down puts the schema back, the case sets up the
// state the up must refuse, and the up runs. Nothing outlives the case.
func TestMigration000078_RefusalsAgainstTheSchema(t *testing.T) {
	ctx := liveDB(t)
	up, down := readMigration(t, m78Up), readMigration(t, m78Down)

	run := func(name, setup, wantErr string) {
		t.Run(name, func(t *testing.T) {
			tx, err := PG.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(context.Background())
			if _, err := tx.Exec(ctx, down); err != nil {
				t.Fatalf("down: %v", err)
			}
			if setup != "" {
				if _, err := tx.Exec(ctx, setup); err != nil {
					t.Fatalf("setup: %v", err)
				}
			}
			_, err = tx.Exec(ctx, up)
			switch {
			case wantErr == "" && err != nil:
				t.Fatalf("up refused a state it must accept: %v", err)
			case wantErr != "" && (err == nil || !strings.Contains(err.Error(), wantErr)):
				t.Fatalf("up must refuse with %q, got %v", wantErr, err)
			}
		})
	}

	const roles = `INSERT INTO config_settings (key, value, updated_by) VALUES ('confirmation.model', 'roles', 'test')
		ON CONFLICT (key) DO UPDATE SET value = 'roles';`
	run("switched over, no overrides: drops", roles, "")
	run("an override row: refuses", roles+`
		INSERT INTO app_claim_overrides (application_id, zitadel_project_id, claim_name, format_type)
		VALUES ('app-1.example.org', 'p1', 'roles', 'array');`, "app_claim_overrides has 1 row(s)")
	run("not switched over: refuses", `DELETE FROM config_settings WHERE key = 'confirmation.model';`,
		"confirmation.model is not 'roles'")
	run("switched to something else: refuses", `INSERT INTO config_settings (key, value, updated_by)
		VALUES ('confirmation.model', 'bundles', 'test') ON CONFLICT (key) DO UPDATE SET value = 'bundles';`,
		"confirmation.model is not 'roles'")
}
