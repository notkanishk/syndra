package db

import (
	"testing"
	"time"
)

// A pending delivery carries the outbox row's creation time, and an applied
// row is not pending at all.
func TestPendingDeliveriesCarryTheirQueueTime(t *testing.T) {
	ctx := liveDB(t)
	user := freshSubject(t, ctx)
	t.Cleanup(func() { _, _ = PG.Exec(ctx, `DELETE FROM propagation_outbox WHERE user_id = $1`, user) })

	waiting := seedOutbox(t, ctx, "add", "pending", user, "pPend", "direct", "", []string{"r1"})
	seedOutbox(t, ctx, "add", "applied", user, "pPend", "direct", "", []string{"r2"})
	want := time.Now().Add(-2 * time.Hour).UTC().Truncate(time.Microsecond)
	if _, err := PG.Exec(ctx, `UPDATE propagation_outbox SET created_at = $2 WHERE id = $1`, waiting, want); err != nil {
		t.Fatal(err)
	}

	got, err := PendingDeliveriesForUser(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].RoleKey != "r1" || !got[0].QueuedAt.Equal(want) {
		t.Fatalf("got %+v, want only r1 queued at %v", got, want)
	}
}

// An add-on's unsent row is not a Zitadel grant waiting: "Not sent to Zitadel
// yet" is said only of Zitadel's own rows.
func TestPendingDeliveriesAreZitadelsOnly(t *testing.T) {
	ctx := liveDB(t)
	user := freshSubject(t, ctx)
	t.Cleanup(func() { _, _ = PG.Exec(ctx, `DELETE FROM propagation_outbox WHERE user_id = $1`, user) })
	addon := seedOutbox(t, ctx, "add", "pending", user, "pPend", "direct", "", []string{"r1"})
	if _, err := PG.Exec(ctx, `UPDATE propagation_outbox SET target = 'truenas' WHERE id = $1`, addon); err != nil {
		t.Fatal(err)
	}
	got, err := PendingDeliveriesForUser(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("got %+v, want nothing: the only unsent row is an add-on's", got)
	}
}
