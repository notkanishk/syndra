package db

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	pgx5 "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
)

var PG *pgxpool.Pool

// defaultMaxConns is the pool size when DB_DSN names none. pgxpool's own
// default is max(4, NumCPU), shared by drains, sweeps, warms and webhook
// bursts over thousands of people (assessment F-24, SC2). pool_max_conns in
// DB_DSN overrides it.
const defaultMaxConns = 20

// ConnectPostgres connects, migrates and sets PG. A database that does not
// answer is an error the caller may retry (the claims data plane is already
// serving and needs no Postgres); a DSN that does not parse, or a migration
// that fails, is fatal.
func ConnectPostgres() error {
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		log.Fatal("DB_DSN environment variable is required")
	}

	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		log.Fatalf("Unable to parse DB_DSN: %v", err)
	}
	if !strings.Contains(dsn, "pool_max_conns") {
		config.MaxConns = defaultMaxConns
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}

	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		return fmt.Errorf("database ping: %w", err)
	}

	// -------------------------------------------------------------
	// Run schema migrations automatically on startup
	// -------------------------------------------------------------
	db := stdlib.OpenDB(*config.ConnConfig)
	driver, err := pgx5.WithInstance(db, &pgx5.Config{})
	if err != nil {
		log.Fatalf("Failed to create migration driver: %v", err)
	}

	migPath := os.Getenv("MIGRATION_PATH")
	if migPath == "" {
		migPath = "file:///app/db/migrations"
	}

	m, err := migrate.NewWithDatabaseInstance(
		migPath,
		"postgres", driver)
	if err != nil {
		log.Fatalf("Migration initialization failed: %v", err)
	}

	newest, err := newestMigration(migPath)
	if err != nil {
		log.Fatalf("Migration files unreadable: %v", err)
	}
	if err := migrateUp(m, newest); err != nil {
		log.Fatalf("An error occurred while running migrations: %v", err)
	}
	fmt.Println("Database schemas successfully migrated and synced.")

	PG = pool
	// Re-run at every start, not only by its migration: rows a rolled-back
	// contract-1 backend wrote carry account_uid alone (task 1.7).
	if err := BackfillAccountIDs(context.Background()); err != nil {
		log.Printf("[STARTUP] account_id backfill: %v (retried at the next start)", err)
	}
	fmt.Printf("Connected to PostgreSQL successfully (pool max %d).\n", config.MaxConns)
	return nil
}

// migrateUp runs every migration this binary has, except when the database is
// ahead of it: a clean (not dirty) schema at a version newer than the newest
// migration file this binary knows is a ROLLBACK of the backend across an
// expand-only migration (000088 adds columns an older backend ignores), so it
// is logged loudly and the binary starts on it as it is. It never migrates
// down. A dirty schema, or any other state, fails exactly as Up does.
//
// This tolerance is only in backends built from this commit on: rolling back
// to a backend built before it needs schema_migrations reset by hand first
// (DEPLOY.md, Rollback).
func migrateUp(m *migrate.Migrate, newest uint) error {
	version, dirty, err := m.Version()
	if aheadOfThisBinary(version, dirty, err, newest) {
		log.Printf("[STARTUP] the database is at migration %d and this backend knows %d at most: "+
			"a rollback across an additive migration. Starting on the newer schema as it is; nothing migrates down.", version, newest)
		return nil
	}
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return nil
}

// aheadOfThisBinary is the one state migrateUp starts without migrating: a
// schema version that was read, is clean, and is newer than this binary's.
func aheadOfThisBinary(version uint, dirty bool, err error, newest uint) bool {
	return err == nil && !dirty && version > newest
}

// newestMigration is the highest NNNNNN_*.up.sql version under a file://
// migration path.
func newestMigration(migPath string) (uint, error) {
	entries, err := os.ReadDir(strings.TrimPrefix(migPath, "file://"))
	if err != nil {
		return 0, err
	}
	var newest uint
	for _, e := range entries {
		num, _, found := strings.Cut(e.Name(), "_")
		if !found || !strings.HasSuffix(e.Name(), ".up.sql") {
			continue
		}
		if v, err := strconv.ParseUint(num, 10, 64); err == nil && uint(v) > newest {
			newest = uint(v)
		}
	}
	if newest == 0 {
		return 0, fmt.Errorf("no migrations in %s", migPath)
	}
	return newest, nil
}
