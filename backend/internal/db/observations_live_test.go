package db

import (
	"context"
	"errors"
	"testing"
)

// The property this store exists for: an incomplete answer may not delete
// anything.
//
// Presence can be concluded from any observation. ABSENCE can only be concluded
// from a whole one — and an absence is what makes Syndra revoke access, report
// drift, and tell an operator somebody has lost something. A truncated listing
// that removed the rows it never reached would manufacture exactly that.

func seedObserved(t *testing.T, ctx context.Context, grantID, userID, project string, roles []string) {
	t.Helper()
	if err := RecordUserObservation(ctx, userID,
		[]ObservedGrant{{GrantID: grantID, UserID: userID, ProjectID: project, RoleKeys: roles}},
		true, ""); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

func observedCount(t *testing.T, ctx context.Context) int {
	t.Helper()
	var n int
	if err := PG.QueryRow(ctx, `SELECT count(*) FROM zitadel_grants_index`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	return n
}

func TestAnIncompleteSweepRemovesNothing(t *testing.T) {
	ctx := liveDB(t)
	truncateObservations(t, ctx)
	seedObserved(t, ctx, "g1", "u1", "p1", []string{"member"})
	seedObserved(t, ctx, "g2", "u2", "p1", []string{"member"})

	// A sweep that saw only one of them and knows it did not finish.
	if err := RecordOrgObservation(ctx,
		[]ObservedGrant{{GrantID: "g1", UserID: "u1", ProjectID: "p1", RoleKeys: []string{"member"}}},
		false, "the read reached its safety limit"); err != nil {
		t.Fatalf("RecordOrgObservation: %v", err)
	}

	if n := observedCount(t, ctx); n != 2 {
		t.Fatalf("an incomplete sweep removed a row it never reached: %d left of 2", n)
	}
}

// And a complete one is the only thing that may.
func TestACompleteSweepReplacesTheStore(t *testing.T) {
	ctx := liveDB(t)
	truncateObservations(t, ctx)
	seedObserved(t, ctx, "g1", "u1", "p1", []string{"member"})
	seedObserved(t, ctx, "gone", "u2", "p1", []string{"member"})

	if err := RecordOrgObservation(ctx,
		[]ObservedGrant{{GrantID: "g1", UserID: "u1", ProjectID: "p1", RoleKeys: []string{"member"}}},
		true, ""); err != nil {
		t.Fatalf("RecordOrgObservation: %v", err)
	}

	if n := observedCount(t, ctx); n != 1 {
		t.Fatalf("a complete sweep did not replace the store: %d rows", n)
	}
}

// A read of one person may clear that person and nobody else, whatever it
// happened to contain.
func TestAUserObservationTouchesOnlyThatPerson(t *testing.T) {
	ctx := liveDB(t)
	truncateObservations(t, ctx)
	seedObserved(t, ctx, "g1", "u1", "p1", []string{"member"})
	seedObserved(t, ctx, "g2", "u2", "p1", []string{"member"})

	// u1 now holds nothing, completely observed.
	if err := RecordUserObservation(ctx, "u1", nil, true, ""); err != nil {
		t.Fatalf("RecordUserObservation: %v", err)
	}

	if n := observedCount(t, ctx); n != 1 {
		t.Fatalf("a read of one person changed another's rows: %d left", n)
	}
	left, err := ObservedGrantsFor(ctx, "u2")
	if err != nil || len(left) != 1 {
		t.Fatalf("the other person's grant did not survive: %v %+v", err, left)
	}
}

// A read scoped to one person that returns somebody else's grant is refused
// rather than written. Accepting it would let a mis-scoped call corrupt the
// store for somebody nobody was asking about.
func TestAUserObservationRefusesAnotherPersonsGrant(t *testing.T) {
	ctx := liveDB(t)
	truncateObservations(t, ctx)

	err := RecordUserObservation(ctx, "u1",
		[]ObservedGrant{{GrantID: "gx", UserID: "u2", ProjectID: "p1", RoleKeys: []string{"member"}}},
		true, "")
	if err == nil {
		t.Fatal("a read of one person wrote a row about another")
	}
}

// "Nobody has looked" must never be readable as "nothing is there".
func TestNothingObservedIsItsOwnAnswer(t *testing.T) {
	ctx := liveDB(t)
	truncateObservations(t, ctx)

	if _, err := LatestObservation(ctx, "u1"); !errors.Is(err, ErrNoObservation) {
		t.Fatalf("an unobserved person reported something other than 'not looked at': %v", err)
	}
}

// The newer of the two candidates wins: an org sweep covers everybody, and a
// person's own read covers them alone.
func TestThePersonsLatestObservationIsTheNewerOfTheTwo(t *testing.T) {
	ctx := liveDB(t)
	truncateObservations(t, ctx)

	if err := RecordOrgObservation(ctx, nil, true, ""); err != nil {
		t.Fatalf("org: %v", err)
	}
	if err := RecordUserObservation(ctx, "u1", nil, true, ""); err != nil {
		t.Fatalf("user: %v", err)
	}
	o, err := LatestObservation(ctx, "u1")
	if err != nil {
		t.Fatalf("LatestObservation: %v", err)
	}
	if o.Scope != "user" {
		t.Fatalf("the older org sweep won over this person's own newer read: %+v", o)
	}
}

func truncateObservations(t *testing.T, ctx context.Context) {
	t.Helper()
	if _, err := PG.Exec(ctx, `TRUNCATE zitadel_observations`); err != nil {
		t.Fatalf("truncate observations: %v", err)
	}
	if _, err := PG.Exec(ctx, `DELETE FROM zitadel_grants_index`); err != nil {
		t.Fatalf("clear grants index: %v", err)
	}
}
