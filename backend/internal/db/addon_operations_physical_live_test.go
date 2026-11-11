package db

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	pgx5 "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
)

// Slice 6c against a real Postgres: the no_change status, the physical hold on
// Waiting on you, and the two reads the member's page settles from.

const physTarget = "inventory-6c-test"

// physicalLive is a live database with this file's target and none of its
// rows.
func physicalLive(t *testing.T) context.Context {
	t.Helper()
	ctx := liveDB(t)
	if _, err := PG.Exec(ctx, `INSERT INTO targets (target) VALUES ($1) ON CONFLICT DO NOTHING`, physTarget); err != nil {
		t.Fatal(err)
	}
	clear := func() {
		if _, err := PG.Exec(ctx, `DELETE FROM addon_operations WHERE target = $1`, physTarget); err != nil {
			t.Fatal(err)
		}
	}
	clear()
	t.Cleanup(clear)
	return ctx
}

// sentRow begins and claims one record, settles it as status, and dates its
// claim ago.
func sentRow(t *testing.T, ctx context.Context, op, subject, status string, ago time.Duration) string {
	t.Helper()
	id, err := BeginAddonOperation(ctx, AddonOperationParams{Target: physTarget, Operation: op, ActorID: subject, SubjectID: subject, About: "item-0001"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ClaimAddonOperation(ctx, id, physTarget, op, subject); err != nil {
		t.Fatal(err)
	}
	if status != AddonOpDispatching {
		if err := SettleAddonOperation(ctx, id, status); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := PG.Exec(ctx, `UPDATE addon_operations SET claimed_at = NOW() - $2::interval, created_at = NOW() - $2::interval WHERE id = $1`, id, ago.String()); err != nil {
		t.Fatal(err)
	}
	return id
}

// no_change is a terminal status a sent call settles as, and only a sent one:
// an unclaimed record was never sent and settles only as unreached.
func TestLiveANoChangeRowIsTerminalAndClaimed(t *testing.T) {
	ctx := physicalLive(t)
	id := sentRow(t, ctx, "item.checkout", "u-kofi", AddonOpNoChange, 0)
	var status string
	var settled *time.Time
	if err := PG.QueryRow(ctx, `SELECT status, settled_at FROM addon_operations WHERE id = $1`, id).Scan(&status, &settled); err != nil {
		t.Fatal(err)
	}
	if status != AddonOpNoChange || settled == nil {
		t.Fatalf("status %q settled %v", status, settled)
	}
	c, err := CountAddonOperations(ctx, physTarget)
	if err != nil || c.NoChange != 1 || c.Succeeded != 0 || c.Unresolved != 0 {
		t.Errorf("counts %+v %v: no_change is neither done nor in doubt", c, err)
	}

	unsent, err := BeginAddonOperation(ctx, AddonOperationParams{Target: physTarget, Operation: "item.checkout", ActorID: "u-kofi", SubjectID: "u-kofi"})
	if err != nil {
		t.Fatal(err)
	}
	if err := SettleAddonOperation(ctx, unsent, AddonOpNoChange); !errors.Is(err, ErrAddonOperationAlreadySettled) {
		t.Errorf("an unsent record settled as no_change: %v", err)
	}
	if _, err := PG.Exec(ctx, `UPDATE addon_operations SET status = 'no_change', settled_at = NOW() WHERE id = $1`, unsent); err == nil {
		t.Error("the claim CHECK let an unsent record say no_change")
	}
}

// A physical call is the add-on's to settle for the bound: off the list and
// its count until then, and on both after. Every other row waits only the
// grace. The zero Hold keeps nothing back.
func TestLiveAPhysicalRowWaitsOutTheBoundBeforeTheDesk(t *testing.T) {
	ctx := physicalLive(t)
	hold := Hold{Operations: []string{"item.checkout"}, For: time.Hour}
	young := sentRow(t, ctx, "item.checkout", "u-kofi", AddonOpIndeterminate, 30*time.Minute)
	adopt := sentRow(t, ctx, "account.adopt", "u-xavier", AddonOpIndeterminate, 30*time.Minute)

	listed := func(h Hold) map[string]bool {
		t.Helper()
		ops, err := ListUnresolvedAddonOperations(ctx, physTarget, AddonUnresolvedGrace, 100, h)
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]bool{}
		for _, o := range ops {
			out[o.ID] = true
		}
		return out
	}
	counted := func(h Hold) int {
		t.Helper()
		n, err := CountUnresolvedAddonOperations(ctx, AddonUnresolvedGrace, h)
		if err != nil {
			t.Fatal(err)
		}
		return n[physTarget]
	}
	if got := listed(hold); got[young] || !got[adopt] || counted(hold) != 1 {
		t.Errorf("before the bound: listed %v, counted %d; want only the adoption", got, counted(hold))
	}
	if got := listed(Hold{}); !got[young] || !got[adopt] || counted(Hold{}) != 2 {
		t.Errorf("with no hold: listed %v, counted %d", got, counted(Hold{}))
	}
	old := sentRow(t, ctx, "item.checkout", "u-big-e", AddonOpIndeterminate, 2*time.Hour)
	if got := listed(hold); !got[old] || got[young] || counted(hold) != 2 {
		t.Errorf("past the bound: listed %v, counted %d", got, counted(hold))
	}
}

// The member's page reads which call is unsettled, and its poll reads that
// call back, theirs alone, with what was found.
func TestLiveTheUnsettledCallIsNamedAndReadBackByItsSubjectOnly(t *testing.T) {
	ctx := physicalLive(t)
	id := sentRow(t, ctx, "item.checkout", "u-kofi", AddonOpDispatching, time.Second)
	got, at, open, err := UnconfirmedChangeAt(ctx, physTarget, "u-kofi", "item.checkout", 0)
	if err != nil || !open || got != id || at.IsZero() {
		t.Fatalf("UnconfirmedChangeAt %q %v %t %v, want %s open", got, at, open, err, id)
	}
	if _, _, open, _ := UnconfirmedChangeAt(ctx, physTarget, "u-kofi", "item.checkout", AddonUnresolvedGrace); open {
		t.Error("a call a second old is still in flight under the grace")
	}

	if _, err := AddonOperationByID(ctx, physTarget, "u-xavier", id); !errors.Is(err, ErrAddonOperationNotFound) {
		t.Errorf("another member read the call: %v", err)
	}
	if _, err := PG.Exec(ctx, `UPDATE addon_operations SET claimed_at = NOW() - interval '1 hour' WHERE id = $1`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveAddonOperation(ctx, physTarget, id, true, SyndraReader, AddonUnresolvedGrace); err != nil {
		t.Fatal(err)
	}
	o, err := AddonOperationByID(ctx, physTarget, "u-kofi", id)
	if err != nil || o.Resolution != AddonResolutionHappened || o.About != "item-0001" {
		t.Fatalf("%+v %v", o, err)
	}
	if _, _, open, _ := UnconfirmedChangeAt(ctx, physTarget, "u-kofi", "item.checkout", 0); open {
		t.Error("a settled call is still unsettled")
	}
}

// 000092 runs down and up on the real engine: down folds no_change into
// succeeded and restores the narrower CHECK, up widens it again. Left where
// it began.
func TestLiveMigration92RunsDownAndUp(t *testing.T) {
	ctx := physicalLive(t)
	var version int64
	var dirty bool
	if err := PG.QueryRow(ctx, `SELECT version, dirty FROM schema_migrations`).Scan(&version, &dirty); err != nil {
		t.Fatal(err)
	}
	if version != 92 || dirty {
		t.Skipf("the test database is at %d (dirty %t), not this branch's 92", version, dirty)
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
		if v, _, _ := m.Version(); v != 92 {
			_ = m.Migrate(92)
		}
	})

	id := sentRow(t, ctx, "item.checkout", "u-kofi", AddonOpNoChange, 0)
	if err := m.Steps(-1); err != nil {
		t.Fatalf("000092 down: %v", err)
	}
	var status string
	if err := PG.QueryRow(ctx, `SELECT status FROM addon_operations WHERE id = $1`, id).Scan(&status); err != nil || status != AddonOpSucceeded {
		t.Fatalf("after down the row says %q (%v), want succeeded", status, err)
	}
	if _, err := PG.Exec(ctx, `UPDATE addon_operations SET status = 'no_change' WHERE id = $1`, id); err == nil {
		t.Error("after down the CHECK still accepts no_change")
	}
	if err := m.Steps(1); err != nil {
		t.Fatalf("000092 up: %v", err)
	}
	if _, err := PG.Exec(ctx, `UPDATE addon_operations SET status = 'no_change' WHERE id = $1`, id); err != nil {
		t.Errorf("after up the CHECK refuses no_change: %v", err)
	}
}
