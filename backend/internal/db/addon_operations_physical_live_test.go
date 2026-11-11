package db

import (
	"context"
	"errors"
	"os"
	"sync"
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
	got, at, open, err := OpenCall(ctx, physTarget, "u-kofi", "item.checkout")
	if err != nil || !open || got != id || at.IsZero() {
		t.Fatalf("OpenCall %q %v %t %v, want %s open", got, at, open, err, id)
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
	if _, _, open, _ := OpenCall(ctx, physTarget, "u-kofi", "item.checkout"); open {
		t.Error("a settled call is still unsettled")
	}
}

// CALL_UNSETTLED in the insert: confirms of one physical call on one subject,
// sent together, leave exactly one record; the rest are refused naming it.
// Rounds, because a race lost once is luck.
func TestLiveConcurrentPhysicalConfirmsLeaveOneRecord(t *testing.T) {
	ctx := physicalLive(t)
	const n = 12
	for round := range 5 {
		if _, err := PG.Exec(ctx, `DELETE FROM addon_operations WHERE target = $1`, physTarget); err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		start := make(chan struct{})
		ids, refused := make(chan string, n), make(chan string, n)
		for range n {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				id, err := BeginAddonOperation(ctx, AddonOperationParams{Target: physTarget, Operation: "item.checkout",
					ActorID: "u-kofi", SubjectID: "u-kofi", Physical: true})
				var open *OpenCallError
				switch {
				case err == nil:
					ids <- id
				case errors.As(err, &open):
					refused <- open.OperationID
				default:
					t.Error(err)
				}
			}()
		}
		close(start)
		wg.Wait()
		close(ids)
		close(refused)
		var rows int
		if err := PG.QueryRow(ctx, `SELECT count(*) FROM addon_operations WHERE target = $1`, physTarget).Scan(&rows); err != nil {
			t.Fatal(err)
		}
		winner := <-ids
		if rows != 1 || len(ids) != 0 || len(refused) != n-1 {
			t.Fatalf("round %d: %d records, %d more ids, %d refused; want exactly one record", round, rows, len(ids)+1, len(refused))
		}
		for id := range refused {
			if id != winner {
				t.Fatalf("round %d: a refusal named %s, not the open %s", round, id, winner)
			}
		}
	}
}

// Every open row refuses, not the newest alone: one in doubt behind a later
// refused one, and one never claimed. A settled or answered one does not, and
// another subject, operation or a non-physical record is never asked.
func TestLiveAnyOpenPhysicalRowRefusesTheNext(t *testing.T) {
	ctx := physicalLive(t)
	begin := func(op, subject string, physical bool) (string, error) {
		return BeginAddonOperation(ctx, AddonOperationParams{Target: physTarget, Operation: op,
			ActorID: subject, SubjectID: subject, Physical: physical})
	}
	doubt := sentRow(t, ctx, "item.checkout", "u-kofi", AddonOpIndeterminate, time.Hour)
	if _, err := PG.Exec(ctx, `INSERT INTO addon_operations (target, operation, actor_id, subject_id, status, claimed_at, settled_at)
		VALUES ($1, 'item.checkout', 'u-kofi', 'u-kofi', 'rejected', NOW(), NOW())`, physTarget); err != nil {
		t.Fatal(err)
	}
	var open *OpenCallError
	if _, err := begin("item.checkout", "u-kofi", true); !errors.As(err, &open) || open.OperationID != doubt || open.SentAt.IsZero() {
		t.Fatalf("behind a rejected row: %v, want %s named", err, doubt)
	}
	for _, other := range [][2]string{{"item.checkout", "u-xavier"}, {"item.return", "u-kofi"}} {
		if _, err := begin(other[0], other[1], true); err != nil {
			t.Errorf("%v: %v", other, err)
		}
	}
	if _, err := begin("item.checkout", "u-kofi", false); err != nil {
		t.Errorf("a non-physical record was asked: %v", err)
	}

	if _, err := ResolveAddonOperation(ctx, physTarget, doubt, false, SyndraReader, AddonUnresolvedGrace); err != nil {
		t.Fatal(err)
	}
	if _, err := PG.Exec(ctx, `DELETE FROM addon_operations WHERE target = $1 AND subject_id = 'u-kofi' AND id <> $2`, physTarget, doubt); err != nil {
		t.Fatal(err)
	}
	unclaimed, err := begin("item.checkout", "u-kofi", true)
	if err != nil {
		t.Fatalf("an answered row still refused: %v", err)
	}
	if _, err := begin("item.checkout", "u-kofi", true); !errors.As(err, &open) || open.OperationID != unclaimed {
		t.Errorf("an unclaimed row (about to be sent) did not refuse: %v", err)
	}
}

// A crash between record and claim leaves a row never sent. The member's page
// shows it as unsettled and the next confirm is refused, naming the same row;
// once the unsent settle closes it as unreached, neither does.
func TestLiveAnUnclaimedRowIsUnsettledOnThePageAndRefusesTheNext(t *testing.T) {
	ctx := physicalLive(t)
	begin := func() (string, error) {
		return BeginAddonOperation(ctx, AddonOperationParams{Target: physTarget, Operation: "item.checkout",
			ActorID: "u-kofi", SubjectID: "u-kofi", Physical: true})
	}
	unclaimed, err := begin()
	if err != nil {
		t.Fatal(err)
	}
	shown, at, found, err := OpenCall(ctx, physTarget, "u-kofi", "item.checkout")
	if err != nil || !found || shown != unclaimed {
		t.Fatalf("the page shows %q %t %v, want the unclaimed %s", shown, found, err, unclaimed)
	}
	var open *OpenCallError
	if _, err := begin(); !errors.As(err, &open) || open.OperationID != shown || !open.SentAt.Equal(at) {
		t.Fatalf("the next confirm: %v, want refused naming %s sent %v", err, shown, at)
	}

	if _, err := PG.Exec(ctx, `UPDATE addon_operations SET created_at = NOW() - $2::interval WHERE id = $1`,
		unclaimed, (2 * AddonUnresolvedGrace).String()); err != nil {
		t.Fatal(err)
	}
	if n, err := SettleUnclaimedAddonOperations(ctx, physTarget, AddonUnresolvedGrace); err != nil || n != 1 {
		t.Fatalf("settled %d %v", n, err)
	}
	if _, _, found, _ := OpenCall(ctx, physTarget, "u-kofi", "item.checkout"); found {
		t.Error("an unreached row is still unsettled on the page")
	}
	if _, err := begin(); err != nil {
		t.Errorf("an unreached row still refused: %v", err)
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
