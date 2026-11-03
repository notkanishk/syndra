package db

import (
	"context"
	"fmt"
	"log"
	"os"
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

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		log.Fatalf("An error occurred while running migrations: %v", err)
	}
	fmt.Println("Database schemas successfully migrated and synced.")

	PG = pool
	fmt.Printf("Connected to PostgreSQL successfully (pool max %d).\n", config.MaxConns)
	return nil
}
