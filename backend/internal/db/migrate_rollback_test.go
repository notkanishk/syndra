package db

import (
	"errors"
	"os"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	pgx5 "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
)

// A backend rolled back across an expand-only migration starts on the newer
// schema; a dirty schema, or none, never takes that path (task 1.10).
func TestOnlyACleanNewerSchemaIsStartedOnWithoutMigrating(t *testing.T) {
	for _, tc := range []struct {
		name    string
		version uint
		dirty   bool
		err     error
		ahead   bool
	}{
		{"one migration newer, clean", 89, false, nil, true},
		{"one migration newer, dirty", 89, true, nil, false},
		{"the same version", 88, false, nil, false},
		{"older: migrate up", 87, false, nil, false},
		{"a fresh database", 0, false, migrate.ErrNilVersion, false},
		{"unreadable", 89, false, errors.New("boom"), false},
	} {
		if got := aheadOfThisBinary(tc.version, tc.dirty, tc.err, 88); got != tc.ahead {
			t.Errorf("%s: ahead %t, want %t", tc.name, got, tc.ahead)
		}
	}
	newest, err := newestMigration("file://../../db/migrations")
	if err != nil || newest < 88 {
		t.Fatalf("newest migration %d: %v", newest, err)
	}
}

// Live: the real migrate engine against a database one migration ahead.
// Clean: the backend starts and the version is left as it was. Dirty: it
// fails, as it always has.
func TestABackendStartsOnACleanNewerSchemaAndNotOnADirtyOne(t *testing.T) {
	ctx := liveDB(t)
	newest, err := newestMigration("file://../../db/migrations")
	if err != nil {
		t.Fatal(err)
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
	set := func(v uint, dirty bool) {
		if _, err := PG.Exec(ctx, `UPDATE schema_migrations SET version = $1, dirty = $2`, int64(v), dirty); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { set(newest, false) })

	set(newest+1, false)
	if err := migrateUp(m, newest); err != nil {
		t.Fatalf("a clean schema one migration ahead must start: %v", err)
	}
	if v, dirty, _ := m.Version(); v != newest+1 || dirty {
		t.Fatalf("the newer schema was touched: version %d dirty %t", v, dirty)
	}
	set(newest+1, true)
	if err := migrateUp(m, newest); err == nil {
		t.Fatal("a dirty schema must fail, as it always has")
	}
}
