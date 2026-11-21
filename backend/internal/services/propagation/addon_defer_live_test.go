package propagation

import (
	"context"
	"errors"
	"net"
	"os"
	"strings"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	pgx5 "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"

	"syndra/internal/addons"
	"syndra/internal/db"
)

// Live, against a throwaway _test database (SYNDRA_TEST_DATABASE_URL): a
// drain into an outage, with the real claim, stamp and defer, leaves the row with no attempt spent and the background
// drain's (ClaimDueRetries) once due. The unit tests stub the defer; this is
// the one that would have seen a release with no not_before.
func TestAnOutageLeavesTheRowToTheBackgroundDrainLive(t *testing.T) {
	dsn := os.Getenv("SYNDRA_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("SYNDRA_TEST_DATABASE_URL is not set; live-database tests skipped")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil || !strings.HasSuffix(cfg.ConnConfig.Database, "_test") {
		t.Fatalf("refusing %q: SYNDRA_TEST_DATABASE_URL must name a database ending in _test (%v)", cfg.ConnConfig.Database, err)
	}
	ctx := context.Background()
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	pg := db.PG
	db.PG = pool
	t.Cleanup(func() { db.PG = pg; pool.Close() })

	// The db package's live harness truncates propagation_outbox around every
	// case, and `go test ./...` runs it in parallel with this package: wait
	// for its lock (internal/db/livedb_test.go), held until that run ends, and
	// hold it here so it cannot start mid-test. Migrate under it, as that
	// harness does, so this test does not depend on which package ran first.
	lock, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lock.Exec(ctx, `SELECT pg_advisory_lock(hashtext('syndra live-db tests'))`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = lock.Exec(ctx, `SELECT pg_advisory_unlock(hashtext('syndra live-db tests'))`)
		lock.Release()
	})
	driver, err := pgx5.WithInstance(stdlib.OpenDB(*cfg.ConnConfig), &pgx5.Config{})
	if err != nil {
		t.Fatal(err)
	}
	m, err := migrate.NewWithDatabaseInstance("file://../../../db/migrations", "postgres", driver)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("migrate: %v", err)
	}

	// A target of its own, so the real claim takes this row and nothing else.
	const target = "defer-live"
	if _, err := pool.Exec(ctx, `INSERT INTO targets (target) VALUES ($1) ON CONFLICT DO NOTHING`, target); err != nil {
		t.Fatalf("seed target: %v", err)
	}
	var id string
	if err := pool.QueryRow(ctx, `
		INSERT INTO propagation_outbox (op_type, user_id, payload_json, idempotency_key, initiated_by, source, target)
		VALUES ('apply', 'u-defer-live', '{}', gen_random_uuid(), 'tester', 'direct', $1) RETURNING id`,
		target).Scan(&id); err != nil {
		t.Fatalf("seed row: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM propagation_outbox WHERE target = $1`, target)
		_, _ = pool.Exec(ctx, `DELETE FROM targets WHERE target = $1`, target)
	})

	h := stubAddonDrain(t)
	t.Cleanup(swap(&claimPending, db.ClaimPendingPropagations))
	t.Cleanup(swap(&markSending, db.MarkPropagationSending))
	t.Cleanup(swap(&deferRow, db.DeferPropagation))
	closed := addons.CallResponse{Outcome: addons.OutcomeUnreached,
		Err: &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")}}
	h.resp = addons.ApplyResponse{Outcome: addons.OutcomeUnreached, Err: closed.Err, Why: addons.Unread(closed)}

	if res, err := DrainAddon(ctx, target); err != nil || res.Requeued != 1 {
		t.Fatalf("drain: %+v %v", res, err)
	}
	var attempts int
	var wait float64
	if err := pool.QueryRow(ctx, `SELECT attempts, EXTRACT(EPOCH FROM not_before - NOW()) FROM propagation_outbox
		WHERE id = $1 AND status = 'pending'`, id).Scan(&attempts, &wait); err != nil {
		t.Fatalf("the row must be pending with a next try: %v", err)
	}
	if attempts != 0 || wait < addons.ProbeInterval().Seconds()-5 {
		t.Fatalf("attempts %d (want 0), due in %.0f s (want about %v)", attempts, wait, addons.ProbeInterval())
	}
	if _, err := pool.Exec(ctx, `UPDATE propagation_outbox SET not_before = NOW() - interval '1 second' WHERE id = $1`, id); err != nil {
		t.Fatal(err)
	}
	got, err := db.ClaimDueRetries(ctx, target, 100)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range got {
		if r.ID == id {
			return
		}
	}
	t.Fatalf("the background drain must claim the deferred row once due, got %d rows", len(got))
}
