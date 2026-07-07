package db

import (
	"os"
	"strings"
	"testing"
)

func TestMigration000017_ConfirmationModeEnumMatchesGo(t *testing.T) {
	sql, err := os.ReadFile("../../db/migrations/000017_confirmation_mode.up.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	s := string(sql)
	for _, want := range []string{
		"confirmation_mode IN ('auto', 'manual')",
		"config_settings",
		"global.default_rule_confirmation_mode",
		"ALTER TABLE pending_zitadel_propagations", // outbox source/source_ref attribution
		"ADD COLUMN IF NOT EXISTS source",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("000017 up migration missing %q", want)
		}
	}
	// The Go layer only ever writes these two mode literals:
	for _, mode := range []string{confirmationModeAuto, confirmationModeManual} {
		if !strings.Contains(s, "'"+mode+"'") {
			t.Errorf("migration CHECK does not cover Go literal %q", mode)
		}
	}
}
