package db

import (
	"os"
	"strings"
	"testing"
)

// Guard: the stored refusal cause (wait_reason "refused:<cause>") needs the
// check constraint widened, and the down migration must clear what the old
// constraint would refuse.
func TestMigration83_WidensWaitReasonCheckAndDownClearsIt(t *testing.T) {
	up, err := os.ReadFile("../../db/migrations/000083_outbox_refused_cause.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	down, err := os.ReadFile("../../db/migrations/000083_outbox_refused_cause.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(up), "LIKE 'refused:%'") || !strings.Contains(string(up), "LIKE 'halted:%'") {
		t.Error("000083 up must allow refused:% and keep halted:%")
	}
	if !strings.Contains(string(down), "SET wait_reason = NULL WHERE wait_reason LIKE 'refused:%'") {
		t.Error("000083 down must clear refused:% before restoring the old check")
	}
	if strings.Contains(string(down), "LIKE 'refused:%')") {
		t.Error("000083 down must not keep refused:% in the restored check")
	}
}
