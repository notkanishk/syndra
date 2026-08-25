package db

import (
	"os"
	"strings"
	"testing"
)

// Migration coherence for 000032. `internal/db` has no live-database harness, so
// what is asserted is that the schema and the statements written against it say
// the same thing — the properties, in the one place they are stated.

func migration32(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile("../../db/migrations/000032_target_account_bindings.up.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	return string(raw)
}

// One account belongs to one subject. Without both indexes, two subjects could
// be recorded against one account and both told it is theirs — and the second to
// converge would take the first one's groups away.
func TestOneAccountBelongsToOneSubject(t *testing.T) {
	sql := migration32(t)

	if !strings.Contains(sql, "PRIMARY KEY (target, subject_id)") {
		t.Error("a subject may hold one account per target, and the primary key is what says so")
	}
	if !strings.Contains(sql, "idx_target_account_bindings_username\n    ON target_account_bindings(target, username)") {
		t.Error("the name must be unique per target")
	}
	// Partial, because not every target has a stable identity to offer and NULLs
	// are distinct in a plain unique index — which would let a second binding
	// with no uid through while looking like it enforced something.
	if !strings.Contains(sql, "ON target_account_bindings(target, account_uid)\n    WHERE account_uid IS NOT NULL") {
		t.Error("the uid index must be partial, or every binding without one collides")
	}
	for _, index := range []string{"idx_target_account_bindings_username", "idx_target_account_bindings_uid"} {
		if !strings.Contains(sql, "CREATE UNIQUE INDEX IF NOT EXISTS "+index) {
			t.Errorf("%s must be UNIQUE — a non-unique index enforces nothing", index)
		}
	}
}

// The writer's column list is the table's, and the two facts the update must NOT
// overwrite stay out of it.
func TestTheBindingWriteMatchesTheSchemaAndKeepsItsProvenance(t *testing.T) {
	src := readSource(t, "target_bindings.go")
	insert := between(t, src, "INSERT INTO target_account_bindings", "ON CONFLICT")

	for _, col := range []string{"target", "subject_id", "username", "account_uid", "bound_by"} {
		if !strings.Contains(insert, col) {
			t.Errorf("the insert does not name %s", col)
		}
	}

	update := between(t, src, "ON CONFLICT (target, subject_id) DO UPDATE", "`")
	if strings.Contains(update, "bound_by") || strings.Contains(update, "bound_at") {
		// They record who first attached this account to this person, which is
		// the question asked after an adoption turns out to have been wrong.
		// Overwriting them on every convergence replaces that answer with "the
		// last drain".
		t.Error("the update must not overwrite who first bound the account")
	}
	if !strings.Contains(update, "last_seen_at = NOW()") {
		t.Error("a convergence that confirmed the binding must move last_seen_at, or nothing distinguishes a binding confirmed this morning from one nothing has confirmed in a year")
	}
	if !strings.Contains(update, "COALESCE(EXCLUDED.account_uid, target_account_bindings.account_uid)") {
		// An apply that reported no uid must not erase one recorded earlier: the
		// uid is what recognises the account across a rename, and losing it
		// silently downgrades the match to the name.
		t.Error("an update carrying no uid must keep the one already recorded")
	}
}

// The refusals happen before the transaction, which is the only way to assert
// them in a package with no database: the write would panic on a nil one.
func TestABindingIsRefusedBeforeTheWrite(t *testing.T) {
	src := readSource(t, "target_bindings.go")
	validation := strings.Index(src, "ErrInvalidTargetBinding)")
	opening := strings.Index(src, "beginOrJoin(ctx)")
	if validation < 0 || opening < 0 {
		t.Fatal("the shape of RecordTargetBinding has changed; re-read this guard")
	}
	if validation > opening {
		t.Error("a binding is validated after the transaction is opened, so a bad one costs a transaction")
	}
}

// The conflict is reported as itself and not as a constraint name. A unique
// violation quoted back describes the schema to whoever asked, and the operator
// action for it — decide which of two people is meant to have the account — is
// not "look at an index".
func TestABindingConflictIsReportedAsADecision(t *testing.T) {
	src := readSource(t, "target_bindings.go")
	if !strings.Contains(src, "IsUniqueViolation(err)") {
		t.Fatal("the unique violation is no longer translated")
	}
	if !strings.Contains(src, "ErrBindingConflict") {
		t.Error("the conflict must have its own sentinel — its operator action is nothing like a failed write")
	}
}
