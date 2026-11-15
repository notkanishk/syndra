package db

import (
	"os"
	"strings"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	pgx5 "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
)

// Guard: 000097 widens the wait_reason check with binding_conflict, keeping every
// value 000086 allowed, and its down maps those rows to target_refused (what
// they were before) before restoring exactly the 000086 check.
func TestMigration97_AllowsBindingConflictAndDownMapsItBack(t *testing.T) {
	up, down := readMigrationPair(t, "000097_outbox_binding_conflict")
	for _, keep := range []string{"'role_waits'", "'manual'", "'" + WaitReasonTargetRefused + "'", "'" + WaitReasonBindingConflict + "'", "LIKE 'halted:%'", "LIKE 'refused:%'"} {
		if !strings.Contains(up, keep) {
			t.Errorf("000097 up must allow %s", keep)
		}
	}
	if !strings.Contains(down, "SET wait_reason = '"+WaitReasonTargetRefused+"' WHERE wait_reason = '"+WaitReasonBindingConflict+"'") {
		t.Error("000097 down must map binding_conflict back to target_refused before restoring the old check")
	}
	restored := down[strings.LastIndex(down, "ADD CONSTRAINT"):]
	if strings.Contains(restored, WaitReasonBindingConflict) || !strings.Contains(restored, WaitReasonTargetRefused) {
		t.Error("000097 down must restore exactly the 000086 check")
	}
}

// A change that landed and could not be attributed settles failed with its own
// wait_reason, and the check accepts it; 000097 runs down (the row becomes a
// target_refused one, the check refuses the new value) and up again on the real
// engine. Left at the version it began at.
func TestLiveMigration97RunsDownAndUp(t *testing.T) {
	ctx := liveDB(t)
	var version int64
	var dirty bool
	if err := PG.QueryRow(ctx, `SELECT version, dirty FROM schema_migrations`).Scan(&version, &dirty); err != nil {
		t.Fatal(err)
	}
	if version < 97 || dirty {
		t.Skipf("the test database is at %d (dirty %t), before this branch's 97", version, dirty)
	}
	seedAddonTarget(t, ctx)
	insert := func(user string) string {
		t.Helper()
		key, err := newOutboxIdempotencyKey()
		if err != nil {
			t.Fatal(err)
		}
		var id string
		if err := PG.QueryRow(ctx, `
			INSERT INTO propagation_outbox
				(op_type, user_id, payload_json, idempotency_key, initiated_by, source, target, status, started_at)
			VALUES ('apply', $1, '{}', $2, 'tester', 'direct', 'truenas', 'in_flight', NOW())
			RETURNING id::text`, user, key).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	read := func(id string) (status, reason, msg string) {
		t.Helper()
		if err := PG.QueryRow(ctx, `SELECT status, COALESCE(wait_reason, ''), COALESCE(last_error, '') FROM propagation_outbox WHERE id = $1`, id).
			Scan(&status, &reason, &msg); err != nil {
			t.Fatal(err)
		}
		return
	}

	id := insert("landed")
	if err := MarkPropagationBindingConflict(ctx, id, "two records disagree"); err != nil {
		t.Fatal(err)
	}
	if status, reason, msg := read(id); status != "failed" || reason != WaitReasonBindingConflict || msg != "two records disagree" {
		t.Fatalf("got %s / %s / %q", status, reason, msg)
	}

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
	t.Cleanup(func() {
		if v, _, _ := m.Version(); v != uint(version) {
			_ = m.Migrate(uint(version))
		}
	})

	if err := m.Migrate(96); err != nil {
		t.Fatalf("000097 down: %v", err)
	}
	if _, reason, _ := read(id); reason != WaitReasonTargetRefused {
		t.Fatalf("down must map the row back to target_refused, got %q", reason)
	}
	other := insert("after-down")
	if err := MarkPropagationBindingConflict(ctx, other, "x"); err == nil {
		t.Fatal("after down, the 000086 check must refuse binding_conflict")
	}

	if err := m.Migrate(97); err != nil {
		t.Fatalf("000097 up: %v", err)
	}
	if err := MarkPropagationBindingConflict(ctx, other, "x"); err != nil {
		t.Fatalf("after up, binding_conflict must be accepted: %v", err)
	}
}
