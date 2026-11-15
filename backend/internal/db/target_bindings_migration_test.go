package db

import (
	"os"
	"regexp"
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

// The mirror is not an identity (7t-6, 000098): one row per subject (the
// primary key, from 000032), and no unique index on the name or the number,
// which made a reused uid refuse the add-on's correct answer. The down puts
// all three back, and resolves duplicates first by the up's own rule (the
// newest row wins), audited, or a rollback would fail on the rows a rolled-back
// backend wrote.
func TestTheMirrorIsNotAnIdentity(t *testing.T) {
	if !strings.Contains(migration32(t), "PRIMARY KEY (target, subject_id)") {
		t.Error("a subject may hold one account per target, and the primary key is what says so")
	}
	up, down := readMigrationPair(t, "000098_mirror_is_not_an_identity")
	indexes := []string{"idx_target_account_bindings_username", "idx_target_account_bindings_uid", "idx_target_account_bindings_account_id"}
	for _, index := range indexes {
		if !strings.Contains(up, "DROP INDEX IF EXISTS "+index+";") {
			t.Errorf("000098 up must drop %s", index)
		}
		if !strings.Contains(down, "CREATE UNIQUE INDEX IF NOT EXISTS "+index) {
			t.Errorf("000098 down must restore %s as UNIQUE", index)
		}
	}
	statements := regexp.MustCompile(`(?m)^--.*$`).ReplaceAllString(up, "")
	if strings.Contains(strings.ToUpper(statements), "CREATE UNIQUE") || strings.Contains(statements, "target_binding_conflicts") {
		t.Error("000098 up adds no unique index and leaves target_binding_conflicts for the rollback (dropped with 1.7b)")
	}
	// The duplicates go before the first index, newest kept, each audited.
	first := strings.Index(down, "CREATE UNIQUE INDEX")
	for _, key := range []string{"PARTITION BY target, username", "PARTITION BY target, account_uid", "PARTITION BY target, account_id"} {
		at := strings.Index(down, key)
		if at < 0 || at > first {
			t.Errorf("000098 down must resolve duplicates on %q before it restores the indexes", key)
		}
	}
	if strings.Count(down, "ORDER BY last_seen_at DESC, bound_at DESC, subject_id DESC") != 3 ||
		strings.Count(down, "'binding.' || target || '.stale_cleared'") != 3 {
		t.Error("each pass keeps the newest row (a total order) and audits what it clears")
	}
}

// The writer's column list is the table's, and the two facts the update must NOT
// overwrite stay out of it.
func TestTheBindingWriteMatchesTheSchemaAndKeepsItsProvenance(t *testing.T) {
	src := readSource(t, "target_bindings.go")
	insert := between(t, src, "INSERT INTO target_account_bindings", "ON CONFLICT")

	for _, col := range []string{"target", "subject_id", "username", "account_id", "account_uid", "bound_by"} {
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
	if !strings.Contains(update, "COALESCE(EXCLUDED.account_id, target_account_bindings.account_id)") ||
		!strings.Contains(update, "CASE WHEN EXCLUDED.account_id IS NOT NULL THEN EXCLUDED.account_uid ELSE target_account_bindings.account_uid END") {
		// An apply that reported no id must not erase one recorded earlier: the
		// id is what recognises the account across a rename, and losing it
		// silently downgrades the match to the name. And account_uid follows
		// the id it carries (migration 000088), or the backfill would put an
		// old number back.
		t.Error("an update carrying no account id must keep the one recorded, and account_uid must follow the id")
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

// The write clears another subject's row on the same account in its own
// transaction and audits it; nothing on this path reads a unique violation any
// more, and it takes no advisory lock (ResolveBindingCall locks the row first).
func TestABindingWriteClearsTheStaleRowInItsTransaction(t *testing.T) {
	src := funcBody(t, readSource(t, "target_bindings.go"), "RecordTargetBinding")
	clear := strings.Index(src, "clearStale, b.Target")
	audit := strings.Index(src, `".stale_cleared"`)
	write := strings.Index(src, "upsert, b.Target")
	if clear < 0 || audit < 0 || write < 0 || !(clear < audit && audit < write) {
		t.Error("clear, audit, then write, all on the one transaction")
	}
	if strings.Contains(src, "pg_advisory_xact_lock") {
		t.Error("a per-target lock taken after ResolveBindingCall's row lock inverts the lock order")
	}
	if strings.Contains(src, "IsUniqueViolation") {
		t.Error("a binding write no longer meets a unique index to violate")
	}
	stale := between(t, src, "const clearStale", "RETURNING")
	if !strings.Contains(stale, "subject_id <> $2") || !strings.Contains(stale, "username = $3 AND ($4::text IS NULL OR") {
		t.Error("only another subject's row, and the name only where either side has no id")
	}
}
