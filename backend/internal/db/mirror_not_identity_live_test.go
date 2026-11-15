package db

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	pgx5 "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
)

// 7t-6: the mirror is not an identity (000098), and PLAN_STALE's own wait
// reason (000099). Gated on SYNDRA_TEST_DATABASE_URL.

const staleCleared = "binding.truenas.stale_cleared"

func cleanStaleAudit(t *testing.T) {
	t.Helper()
	clean := func() {
		_, _ = PG.Exec(context.Background(), `DELETE FROM audit_logs WHERE action = $1`, staleCleared)
	}
	clean()
	t.Cleanup(clean)
}

func mirrored(t *testing.T, ctx context.Context) map[string]string {
	t.Helper()
	rows, err := PG.Query(ctx, `SELECT subject_id, username || '/' || COALESCE(account_id, '-')
		FROM target_account_bindings WHERE target = 'truenas'`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var subject, row string
		if err := rows.Scan(&subject, &row); err != nil {
			t.Fatal(err)
		}
		out[subject] = row
	}
	return out
}

type staleAudit struct {
	actor, subject, username string
	details                  map[string]any
}

func staleAudits(t *testing.T, ctx context.Context) []staleAudit {
	t.Helper()
	rows, err := PG.Query(ctx, `SELECT actor_zitadel_user_id, target_zitadel_user_id, resource_id, details
		FROM audit_logs WHERE action = $1 ORDER BY id`, staleCleared)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []staleAudit
	for rows.Next() {
		var a staleAudit
		var raw []byte
		if err := rows.Scan(&a.actor, &a.subject, &a.username, &raw); err != nil {
			t.Fatal(err)
		}
		_ = json.Unmarshal(raw, &a.details)
		out = append(out, a)
	}
	return out
}

// TrueNAS hands A's freed uid to a new account and the add-on binds person B
// to it: B's apply records, A's stale row on that number is cleared in the
// same write, and the clear is audited against A, by B's write's actor. A row
// on the same name with another id is another account whose mirrored name is
// out of date, and is kept; a name is the key only where a side has no id. A
// write that reports no id clears nothing on the id, and nothing writes
// target_binding_conflicts.
func TestLiveAReusedNumberOrNameMovesTheMirrorAndIsAudited(t *testing.T) {
	ctx := liveDB(t)
	seedAddonTarget(t, ctx)
	cleanStaleAudit(t)
	var conflictsBefore int
	if err := PG.QueryRow(ctx, `SELECT count(*) FROM target_binding_conflicts`).Scan(&conflictsBefore); err != nil {
		t.Fatal(err)
	}
	record := func(subject, username string, id *string, by string) {
		t.Helper()
		if err := RecordTargetBinding(ctx, TargetBinding{Target: "truenas", SubjectID: subject, Username: username,
			AccountID: id, BoundBy: by}); err != nil {
			t.Fatalf("%s's write is the add-on's answer and must record: %v", subject, err)
		}
	}
	record("sub-a", "rey_mysterio", strp("3051"), "apply")
	record("sub-x", "dolph_ziggler", nil, "apply")

	// The number: B's apply lands on uid 3051 under its own name.
	record("sub-b", "kofi_kingston", strp("3051"), "operator-1")
	if got := mirrored(t, ctx); got["sub-b"] != "kofi_kingston/3051" || got["sub-a"] != "" {
		t.Fatalf("B's row is recorded and A's stale row on the number is gone: %v", got)
	}
	audits := staleAudits(t, ctx)
	if len(audits) != 1 || audits[0].subject != "sub-a" || audits[0].username != "rey_mysterio" ||
		audits[0].actor != "operator-1" || audits[0].details["now_bound_to"] != "sub-b" || audits[0].details["account_id"] != "3051" {
		t.Fatalf("one audit entry naming A, its old account, and who it went to: %+v", audits)
	}

	// The same name on another id: B's account was renamed on the target and
	// the add-on follows it by number; C's new account took the freed name.
	record("sub-c", "kofi_kingston", strp("3099"), "apply")
	if got := mirrored(t, ctx); got["sub-c"] != "kofi_kingston/3099" || got["sub-b"] != "kofi_kingston/3051" {
		t.Fatalf("C's row is recorded and B's row on another account is kept: %v", got)
	}
	// A name with no id on one side is the key: a contract-1 era row is cleared.
	if _, err := PG.Exec(ctx, `INSERT INTO target_account_bindings (target, subject_id, username, bound_by)
		VALUES ('truenas', 'sub-old', 'big_e', 'apply')`); err != nil {
		t.Fatal(err)
	}
	record("sub-d", "big_e", strp("3111"), "apply")
	if got := mirrored(t, ctx); got["sub-d"] != "big_e/3111" || got["sub-old"] != "" {
		t.Fatalf("D's row is recorded and the id-less row on its name is gone: %v", got)
	}

	// No id reported: nothing is matched on the id, and the row with none stays.
	record("sub-y", "edge", nil, "apply")
	if got := mirrored(t, ctx); got["sub-x"] != "dolph_ziggler/-" || got["sub-y"] != "edge/-" {
		t.Fatalf("a write with no id must not clear another row with none: %v", got)
	}
	// The subject's own row is updated, never cleared or audited.
	record("sub-c", "kofi_kingston", strp("3099"), "apply")
	if n := len(staleAudits(t, ctx)); n != 2 {
		t.Fatalf("want exactly the two clears audited, got %d", n)
	}
	var conflictsAfter int
	if err := PG.QueryRow(ctx, `SELECT count(*) FROM target_binding_conflicts`).Scan(&conflictsAfter); err != nil || conflictsAfter != conflictsBefore {
		t.Fatalf("nothing writes the retired table: %d before, %d after (%v)", conflictsBefore, conflictsAfter, err)
	}
}

func migrator(t *testing.T) *migrate.Migrate {
	t.Helper()
	config, err := pgxpool.ParseConfig(os.Getenv("SYNDRA_TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	driver, err := pgx5.WithInstance(stdlib.OpenDB(*config.ConnConfig), &pgx5.Config{})
	if err != nil {
		t.Fatal(err)
	}
	m, err := migrate.NewWithDatabaseInstance("file://../../db/migrations", "postgres", driver)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// atLeast skips unless the test database is clean at version or later, and
// leaves it at the version it found.
func atLeast(t *testing.T, ctx context.Context, version int64) *migrate.Migrate {
	t.Helper()
	var found int64
	var dirty bool
	if err := PG.QueryRow(ctx, `SELECT version, dirty FROM schema_migrations`).Scan(&found, &dirty); err != nil {
		t.Fatal(err)
	}
	if found < version || dirty {
		t.Skipf("the test database is at %d (dirty %t), before this branch's %d", found, dirty, version)
	}
	m := migrator(t)
	t.Cleanup(func() {
		if v, _, _ := m.Version(); v != uint(found) {
			_ = m.Migrate(uint(found))
		}
	})
	return m
}

func uniqueIndexes(t *testing.T, ctx context.Context) []string {
	t.Helper()
	rows, err := PG.Query(ctx, `SELECT indexname FROM pg_indexes
		WHERE tablename = 'target_account_bindings' AND indexdef LIKE 'CREATE UNIQUE INDEX%' AND indexname <> 'target_account_bindings_pkey'
		ORDER BY indexname`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		out = append(out, name)
	}
	return out
}

// 000098 down with duplicates on all three keys: each is resolved by the up's
// rule (the newest row kept), every cleared row audited, the three unique
// indexes come back; up drops them again. target_binding_conflicts and its
// rows are untouched both ways (kept for the rollback, dropped with 1.7b).
func TestLiveMigration98DownResolvesDuplicatesAndUpDropsTheIndexes(t *testing.T) {
	ctx := liveDB(t)
	m := atLeast(t, ctx, 98)
	seedAddonTarget(t, ctx)
	cleanStaleAudit(t)
	if got := uniqueIndexes(t, ctx); len(got) != 0 {
		t.Fatalf("at 98 the mirror has no unique index but its key: %v", got)
	}
	// Two rows on one name, two on one account_uid with no account_id (a
	// contract-1 backend's write), two on one account_id, oldest first.
	seed := []struct {
		subject, username string
		id                *string
		uid               *int64
		age               string
	}{
		{"name-old", "rey_mysterio", strp("4001"), nil, "3 hours"},
		{"name-new", "rey_mysterio", strp("4002"), nil, "1 hour"},
		{"uid-old", "big_e", nil, ptrInt(5001), "3 hours"},
		{"uid-new", "xavier_woods", nil, ptrInt(5001), "1 hour"},
		{"id-old", "sasha_banks", strp("7a2c-uuid"), nil, "3 hours"},
		{"id-new", "bayley", strp("7a2c-uuid"), nil, "1 hour"},
		{"alone", "becky_lynch", strp("6001"), nil, "2 hours"},
	}
	for _, s := range seed {
		uid := s.uid
		if s.id != nil {
			uid = legacyUID(s.id)
		}
		if _, err := PG.Exec(ctx, `INSERT INTO target_account_bindings
			(target, subject_id, username, account_id, account_uid, bound_by, bound_at, last_seen_at)
			VALUES ('truenas', $1, $2, $3, $4, 'seed', NOW() - $5::interval, NOW() - $5::interval)`,
			s.subject, s.username, s.id, uid, s.age); err != nil {
			t.Fatalf("seed %s: %v", s.subject, err)
		}
	}
	clearRetired := func() {
		_, _ = PG.Exec(context.Background(), `DELETE FROM target_binding_conflicts WHERE target = 'truenas' AND username = 'retired_row'`)
	}
	clearRetired()
	t.Cleanup(clearRetired)
	if _, err := PG.Exec(ctx, `INSERT INTO target_binding_conflicts
		(target, username, converged_subject_id, bound_subject_id, outbox_id)
		VALUES ('truenas', 'retired_row', 'sub-p', 'sub-q', '6f1b2c3d-4e5f-4a6b-8c7d-9e0f1a2b3c4d')`); err != nil {
		t.Fatal(err)
	}
	retired := func() (n int) {
		if err := PG.QueryRow(ctx, `SELECT count(*) FROM target_binding_conflicts WHERE target = 'truenas'`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	if err := m.Migrate(97); err != nil {
		t.Fatalf("000098 down over duplicates: %v", err)
	}
	got := mirrored(t, ctx)
	for _, keep := range []string{"name-new", "uid-new", "id-new", "alone"} {
		if got[keep] == "" {
			t.Errorf("the newest row on each key is kept: %s missing from %v", keep, got)
		}
	}
	for _, gone := range []string{"name-old", "uid-old", "id-old"} {
		if got[gone] != "" {
			t.Errorf("an older duplicate is cleared: %s still in %v", gone, got)
		}
	}
	audited := map[string]bool{}
	for _, a := range staleAudits(t, ctx) {
		if a.actor != "system:migration" || !strings.HasPrefix(a.details["why"].(string), "migration 000098 down") {
			t.Errorf("a migration clear is audited as the migration's: %+v", a)
		}
		audited[a.subject] = true
	}
	if len(audited) != 3 || !audited["name-old"] || !audited["uid-old"] || !audited["id-old"] {
		t.Errorf("each cleared row is audited once: %v", audited)
	}
	if got := uniqueIndexes(t, ctx); strings.Join(got, ",") != "idx_target_account_bindings_account_id,idx_target_account_bindings_uid,idx_target_account_bindings_username" {
		t.Fatalf("down restores the three unique indexes: %v", got)
	}
	if retired() != 1 {
		t.Fatal("000098 down must leave target_binding_conflicts as it found it")
	}

	if err := m.Migrate(98); err != nil {
		t.Fatalf("000098 up: %v", err)
	}
	if got := uniqueIndexes(t, ctx); len(got) != 0 {
		t.Fatalf("up drops them again: %v", got)
	}
	if retired() != 1 {
		t.Fatal("000098 up must leave target_binding_conflicts as it found it")
	}
}

func ptrInt(n int64) *int64 { return &n }

// Guard: 000099 widens the wait_reason check with plan_stale, keeping every
// value 000097 allowed, and its down clears those rows' reason (what they had
// before) before restoring exactly the 000097 check.
func TestMigration99_AllowsPlanStaleAndDownClearsIt(t *testing.T) {
	up, down := readMigrationPair(t, "000099_outbox_plan_stale")
	added := up[strings.LastIndex(up, "ADD CONSTRAINT"):]
	for _, keep := range []string{"'role_waits'", "'manual'", "'" + WaitReasonTargetRefused + "'", "'" + WaitReasonBindingConflict + "'",
		"'" + WaitReasonPlanStale + "'", "LIKE 'halted:%'", "LIKE 'refused:%'"} {
		if !strings.Contains(added, keep) {
			t.Errorf("000099 up must allow %s", keep)
		}
	}
	if !strings.Contains(down, "SET wait_reason = NULL WHERE wait_reason = '"+WaitReasonPlanStale+"'") {
		t.Error("000099 down must clear plan_stale before restoring the old check")
	}
	restored := down[strings.LastIndex(down, "ADD CONSTRAINT"):]
	if strings.Contains(restored, WaitReasonPlanStale) || !strings.Contains(restored, WaitReasonBindingConflict) {
		t.Error("000099 down must restore exactly the 000097 check")
	}
}

// A PLAN_STALE row settles failed with its own wait reason and the sentence as
// given; 000099 runs down (the row keeps its sentence and loses the reason, as
// such a row was before) and up again. Left at the version it began at.
func TestLiveMigration99RunsDownAndUp(t *testing.T) {
	ctx := liveDB(t)
	m := atLeast(t, ctx, 99)
	seedAddonTarget(t, ctx)
	key, err := newOutboxIdempotencyKey()
	if err != nil {
		t.Fatal(err)
	}
	var id string
	if err := PG.QueryRow(ctx, `
		INSERT INTO propagation_outbox
			(op_type, user_id, payload_json, idempotency_key, initiated_by, source, target, status, started_at)
		VALUES ('apply', 'stale', '{}', $1, 'tester', 'direct', 'truenas', 'in_flight', NOW())
		RETURNING id::text`, key).Scan(&id); err != nil {
		t.Fatal(err)
	}
	const sentence = "Their account changed after this change was approved, so it was not sent. Make the change again to send it."
	if err := MarkPropagationPlanStale(ctx, id, sentence); err != nil {
		t.Fatal(err)
	}
	read := func() (status, reason, msg string) {
		t.Helper()
		if err := PG.QueryRow(ctx, `SELECT status, COALESCE(wait_reason, ''), COALESCE(last_error, '') FROM propagation_outbox WHERE id = $1`, id).
			Scan(&status, &reason, &msg); err != nil {
			t.Fatal(err)
		}
		return
	}
	if status, reason, msg := read(); status != "failed" || reason != WaitReasonPlanStale || msg != sentence {
		t.Fatalf("got %s / %s / %q", status, reason, msg)
	}
	if err := m.Migrate(98); err != nil {
		t.Fatalf("000099 down: %v", err)
	}
	if status, reason, msg := read(); status != "failed" || reason != "" || msg != sentence {
		t.Fatalf("after down: %s / %q / %q, want failed with no reason and the sentence kept", status, reason, msg)
	}
	if _, err := PG.Exec(ctx, `UPDATE propagation_outbox SET wait_reason = 'plan_stale' WHERE id = $1`, id); err == nil {
		t.Fatal("after down the check must refuse plan_stale")
	}
	if err := m.Migrate(99); err != nil {
		t.Fatalf("000099 up: %v", err)
	}
	if _, err := PG.Exec(ctx, `UPDATE propagation_outbox SET wait_reason = 'plan_stale' WHERE id = $1`, id); err != nil {
		t.Fatalf("after up the check must accept plan_stale: %v", err)
	}
}
