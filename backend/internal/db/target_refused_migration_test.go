package db

import (
	"strings"
	"testing"
)

// Guard: an add-on refusal (wait_reason WaitReasonTargetRefused) needs the
// check widened, keeping every value 000083 allowed, and the down migration
// clears it before restoring exactly the 000083 check.
func TestMigration86_AllowsTargetRefusedAndDownClearsIt(t *testing.T) {
	up, down := readMigrationPair(t, "000086_outbox_target_refused")
	for _, keep := range []string{"'role_waits'", "'manual'", "LIKE 'halted:%'", "LIKE 'refused:%'", "'" + WaitReasonTargetRefused + "'"} {
		if !strings.Contains(up, keep) {
			t.Errorf("000086 up must allow %s", keep)
		}
	}
	if !strings.Contains(down, "SET wait_reason = NULL WHERE wait_reason = '"+WaitReasonTargetRefused+"'") {
		t.Error("000086 down must clear target_refused before restoring the old check")
	}
	restored := down[strings.LastIndex(down, "ADD CONSTRAINT"):]
	if strings.Contains(restored, WaitReasonTargetRefused) || !strings.Contains(restored, "LIKE 'refused:%'") {
		t.Error("000086 down must restore exactly the 000083 check")
	}
}

// Guard: the snapshot's withdraws_only defaults false (what every existing
// snapshot was) and the down migration drops it.
func TestMigration87_AddsSnapshotWithdrawsOnly(t *testing.T) {
	up, down := readMigrationPair(t, "000087_snapshot_withdraws_only")
	if !strings.Contains(up, "ADD COLUMN IF NOT EXISTS withdraws_only boolean NOT NULL DEFAULT false") {
		t.Error("000087 up must add withdraws_only NOT NULL DEFAULT false to desired_state_snapshots")
	}
	if !strings.Contains(up, "CHECK (NOT withdraws_only OR (") {
		t.Error("000087 up must hold withdraws_only to its meaning: the switch alone, off")
	}
	if !strings.Contains(down, "DROP COLUMN IF EXISTS withdraws_only") {
		t.Error("000087 down must drop the column")
	}
}
