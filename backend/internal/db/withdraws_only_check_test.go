package db

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

// The verified target-agnostic form (addon-contract 2.2), exactly: a
// non-empty object whose every value is JSON false.
const withdrawsOnlyCheck = `CHECK (NOT withdraws_only OR (state_json <> '{}'::jsonb AND NOT jsonb_path_exists(state_json, 'strict $.* ? (@.type() != "boolean" || @ == true)')))`

// Guard: 000089 replaces the 000087 CHECK with the target-agnostic form, names
// no field, validates every existing row (no NOT VALID on the way up), and the
// down migration restores the 000087 form.
func TestMigration89_WithdrawsOnlyCheckNamesNoField(t *testing.T) {
	up, down := readMigrationPair(t, "000089_withdraws_only_by_role")
	if !strings.Contains(up, withdrawsOnlyCheck) {
		t.Errorf("000089 up must add exactly %s", withdrawsOnlyCheck)
	}
	if !strings.Contains(up, "DROP CONSTRAINT IF EXISTS desired_state_snapshots_withdraws_only_check") {
		t.Error("000089 up must drop the 000087 check first")
	}
	stmts := up[strings.Index(up, "ALTER TABLE"):]
	if strings.Contains(stmts, "NOT VALID") || strings.Contains(stmts, "'enabled'") || strings.Contains(stmts, "smb_enabled") {
		t.Error("000089 up must re-validate every row and name no target field")
	}
	if !strings.Contains(down, "state_json -> 'enabled' = 'false'::jsonb") || !strings.Contains(down, "DROP CONSTRAINT IF EXISTS desired_state_snapshots_withdraws_only_check") {
		t.Error("000089 down must restore the 000087 form")
	}
}

// Live: exactly the five rows of task 2.2 under withdraws_only = true, and the
// constraint in force is validated (every existing row passed it).
func TestTheWithdrawsOnlyCheckIsTheSwitchAloneOff(t *testing.T) {
	ctx := liveDB(t)
	subject := freshSubject(t, ctx)
	write := func(state string) error {
		return InTx(ctx, func(tx pgx.Tx) error {
			var m map[string]json.RawMessage
			if err := json.Unmarshal([]byte(state), &m); err != nil {
				t.Fatal(err)
			}
			_, err := WriteDesiredStateSnapshotTx(ctx, tx, subject, "truenas", "system:reconcile", m, true)
			return err
		})
	}
	if err := write(`{"enabled":false,"smb_enabled":false}`); err != nil {
		t.Fatalf("the switch alone, off, must be accepted: %v", err)
	}
	for _, refused := range []string{
		`{"enabled":false,"smb_enabled":false,"group":["admins"]}`,
		`{"enabled":true}`,
		`{"enabled":"false"}`,
		`{}`,
	} {
		if err := write(refused); err == nil {
			t.Errorf("withdraw-only %s must be refused", refused)
		}
	}
	var validated bool
	if err := querier(ctx).QueryRow(ctx, `SELECT convalidated FROM pg_constraint WHERE conname = 'desired_state_snapshots_withdraws_only_check'`).Scan(&validated); err != nil || !validated {
		t.Fatalf("the check must be in force and validated: %v %v", validated, err)
	}
}
