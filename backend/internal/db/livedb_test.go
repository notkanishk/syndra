package db

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	pgx5 "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
)

// A real database for the assertions that cannot be made without one.
//
// Every other test in this package is a migration-coherence or SQL-text guard,
// which is enough to catch a statement that changed and not enough to catch a
// statement that was always wrong. Three review findings in a row bottomed out
// here (NEXT.md §4b), and so does the withdrawal path below: it decides whether
// a revoke is owed, and a guard that greps for the call proves only that
// something is called.
//
// Skipped, never failed, when SYNDRA_TEST_DATABASE_URL is unset — `go test
// ./...` stays green on a machine with no Postgres, which is the condition that
// kept this debt open. Point it at a THROWAWAY database: the harness migrates
// it and truncates tables between cases.
//
//	SYNDRA_TEST_DATABASE_URL=postgres://user:pw@host:5432/syndra_test go test ./internal/db/...
var (
	liveOnce sync.Once
	livePool *pgxpool.Pool
	liveErr  error
)

func liveDB(t *testing.T) context.Context {
	t.Helper()
	dsn := os.Getenv("SYNDRA_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("SYNDRA_TEST_DATABASE_URL is not set; live-database tests skipped")
	}

	liveOnce.Do(func() {
		config, err := pgxpool.ParseConfig(dsn)
		if err != nil {
			liveErr = err
			return
		}
		livePool, liveErr = pgxpool.NewWithConfig(context.Background(), config)
		if liveErr != nil {
			return
		}
		if liveErr = livePool.Ping(context.Background()); liveErr != nil {
			return
		}

		path := os.Getenv("SYNDRA_TEST_MIGRATIONS")
		if path == "" {
			path = "file://../../db/migrations"
		}
		sqlDB := stdlib.OpenDB(*config.ConnConfig)
		driver, err := pgx5.WithInstance(sqlDB, &pgx5.Config{})
		if err != nil {
			liveErr = err
			return
		}
		m, err := migrate.NewWithDatabaseInstance(path, "postgres", driver)
		if err != nil {
			liveErr = err
			return
		}
		if err := m.Up(); err != nil && err != migrate.ErrNoChange {
			liveErr = err
			return
		}
	})
	if liveErr != nil {
		t.Fatalf("live database unavailable: %v", liveErr)
	}

	PG = livePool
	ctx := context.Background()

	// The target every outbox row points at. Present already on a database that
	// has been migrated and used; inserted here so a fresh one works too.
	if _, err := PG.Exec(ctx,
		`INSERT INTO targets (target) VALUES ('zitadel') ON CONFLICT DO NOTHING`); err != nil {
		t.Fatalf("seed targets: %v", err)
	}
	truncateOutbox(t, ctx)
	t.Cleanup(func() { truncateOutbox(t, ctx) })
	return ctx
}

func truncateOutbox(t *testing.T, ctx context.Context) {
	t.Helper()
	if _, err := PG.Exec(ctx, `TRUNCATE propagation_outbox`); err != nil {
		t.Fatalf("truncate outbox: %v", err)
	}
}
