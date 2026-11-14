package db

import (
	"strings"
	"testing"
)

// Guard: 000088 adds account_id BESIDE account_uid in both binding tables
// (decision 10: dual-written for one release, account_uid dropped by 1.7b),
// the backfill fills only what is empty (re-runnable), the unique index is on
// the bindings table ONLY (target_binding_conflicts keeps resolved history),
// field_roles is added to targets, and the down migration drops exactly that.
func TestMigration88_AddsAccountIDBesideAccountUIDAndFieldRoles(t *testing.T) {
	up, down := readMigrationPair(t, "000088_addon_contract_2")
	for _, want := range []string{
		"ALTER TABLE target_account_bindings ADD COLUMN IF NOT EXISTS account_id TEXT",
		"ALTER TABLE target_binding_conflicts ADD COLUMN IF NOT EXISTS account_id TEXT",
		"SET account_id = account_uid::text\n WHERE account_uid IS NOT NULL AND account_id IS DISTINCT FROM account_uid::text",
		"ON target_account_bindings(target, account_id)\n    WHERE account_id IS NOT NULL",
		"ALTER TABLE targets ADD COLUMN IF NOT EXISTS field_roles JSONB",
	} {
		if !strings.Contains(up, want) {
			t.Errorf("000088 up must contain %q", want)
		}
	}
	if n := strings.Count(up, "SET account_id = account_uid::text\n WHERE account_uid IS NOT NULL AND account_id IS DISTINCT FROM account_uid::text"); n != 2 {
		t.Errorf("000088 up must backfill both tables where account_id differs from account_uid, does %d", n)
	}
	if strings.Contains(up, "DROP COLUMN") || strings.Contains(up, "account_uid BIGINT") {
		t.Error("000088 up must be additive: account_uid stays, a rolled-back backend reads it")
	}
	if strings.Contains(up, "ON target_binding_conflicts(target, account_id)") {
		t.Error("000088 must not make account_id unique on target_binding_conflicts: a second conflict on an account must insert once the first is resolved")
	}
	for _, want := range []string{
		"DROP COLUMN IF EXISTS field_roles", "DROP INDEX IF EXISTS idx_target_account_bindings_account_id",
		"target_binding_conflicts DROP COLUMN IF EXISTS account_id", "target_account_bindings DROP COLUMN IF EXISTS account_id",
	} {
		if !strings.Contains(down, want) {
			t.Errorf("000088 down must contain %q", want)
		}
	}
	if strings.Contains(down, "account_uid") && !strings.Contains(down, "account_uid was written") {
		t.Error("000088 down must leave account_uid alone")
	}
}

// legacyUID is the dual write's other half: a number when account_id is one,
// NULL when it is not (a UUID never had an account_uid).
func TestLegacyUIDIsTheNumberOrNothing(t *testing.T) {
	s := func(v string) *string { return &v }
	for in, want := range map[string]string{"3051": "3051", "0": "0", "7f3a2c1b-uuid": "", "03051": "", " 3051": "", "99999999999999999999": ""} {
		got := legacyUID(s(in))
		if (got == nil) != (want == "") || (got != nil && want != "" && *got != mustInt(t, want)) {
			t.Errorf("legacyUID(%q) = %v, want %q", in, got, want)
		}
	}
	if legacyUID(nil) != nil {
		t.Error("no account id is no account_uid")
	}
}

func mustInt(t *testing.T, s string) int64 {
	t.Helper()
	var n int64
	for _, c := range s {
		n = n*10 + int64(c-'0')
	}
	return n
}

// The backfill the backend runs at every start is the migration's own
// statement: one rule for what account_uid may correct.
func TestTheStartupBackfillIsTheMigrations(t *testing.T) {
	up, _ := readMigrationPair(t, "000088_addon_contract_2")
	src := readSource(t, "target_bindings.go")
	const where = "WHERE account_uid IS NOT NULL AND account_id IS DISTINCT FROM account_uid::text"
	if !strings.Contains(strings.Join(strings.Fields(up), " "), where) || !strings.Contains(strings.Join(strings.Fields(src), " "), where) {
		t.Errorf("BackfillAccountIDs and 000088 must both backfill %q", where)
	}
}

// Guard: 000090 adds retired_fields beside field_roles, additive (an older
// backend ignores it), and the down migration drops exactly that.
func TestMigration90_AddsRetiredFields(t *testing.T) {
	up, down := readMigrationPair(t, "000090_field_roles_retired")
	if !strings.Contains(up, "ALTER TABLE targets ADD COLUMN IF NOT EXISTS retired_fields TEXT[] NOT NULL DEFAULT '{}'") {
		t.Error("000090 up must add retired_fields TEXT[] NOT NULL DEFAULT '{}' to targets")
	}
	if strings.Contains(up, "DROP") || strings.Contains(up, "UPDATE") {
		t.Error("000090 up must be additive")
	}
	if !strings.Contains(down, "DROP COLUMN IF EXISTS retired_fields") {
		t.Error("000090 down must drop retired_fields")
	}
}

// Guard: 000095 adds accepted_shape beside field_roles, nullable and additive
// (an older backend ignores it), and the down migration drops exactly that.
func TestMigration95_AddsAcceptedShape(t *testing.T) {
	up, down := readMigrationPair(t, "000095_target_accepted_shape")
	if !strings.Contains(up, "ALTER TABLE targets ADD COLUMN IF NOT EXISTS accepted_shape JSONB;") {
		t.Error("000095 up must add a nullable targets.accepted_shape JSONB")
	}
	if strings.Contains(up, "DROP") || strings.Contains(up, "UPDATE") || strings.Contains(up, "NOT NULL") {
		t.Error("000095 up must be additive")
	}
	if strings.TrimSpace(down) != "ALTER TABLE targets DROP COLUMN IF EXISTS accepted_shape;" {
		t.Errorf("000095 down must drop accepted_shape and nothing else: %q", down)
	}
}
