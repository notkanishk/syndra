package db

import (
	"context"
	"errors"
	"testing"
)

// The straddle this exists to close.
//
// The drift sweep reads how complete the covering observation was, then reads
// the grants that observation left behind, and reports the first as the footing
// for the second. Read as two statements, a sweep committing in between leaves
// it citing one generation's completeness over another generation's rows — a
// clean bill signed for a world nobody looked at.
//
// The test drives the race deterministically: it opens the snapshot, reads the
// observation, then commits a WHOLE new generation (incomplete, different
// grants) from outside, and only then reads the rows. Under read committed the
// second read would see the newcomer's rows beside the first read's
// completeness. Under the snapshot both reads belong to one world.
func TestObservationSnapshot_RowsAndBasisCannotStraddleASweep(t *testing.T) {
	ctx := liveDB(t)
	truncateObservations(t, ctx)

	// Generation one: complete, one grant.
	if err := RecordOrgObservation(ctx,
		[]ObservedGrant{{GrantID: "g1", UserID: "u1", ProjectID: "p1", RoleKeys: []string{"community"}}},
		true, ""); err != nil {
		t.Fatalf("record generation one: %v", err)
	}

	var seenObs Observation
	var seenGrants []ObservedGrant
	err := InReadSnapshot(ctx, func(snapCtx context.Context) error {
		obs, err := LatestOrgObservation(snapCtx)
		if err != nil {
			return err
		}
		seenObs = obs

		// Generation two commits, in full, from outside this snapshot: a
		// different observation AND different rows. This is the scheduled
		// sweep landing between the drift sweep's two reads.
		if err := RecordOrgObservation(ctx,
			[]ObservedGrant{{GrantID: "g2", UserID: "u2", ProjectID: "p2", RoleKeys: []string{"staff"}}},
			false, ""); err != nil {
			return err
		}

		seenGrants, err = AllObservedGrants(snapCtx)
		return err
	})
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}

	if !seenObs.Complete {
		t.Fatal("the snapshot read generation two's completeness, not the one it started in")
	}
	if len(seenGrants) != 1 || seenGrants[0].GrantID != "g1" {
		t.Fatalf("the rows came from a generation the basis does not describe: %+v", seenGrants)
	}

	// And afterwards, outside the snapshot, generation two is plainly visible —
	// so the isolation above is a snapshot, not a stale connection.
	//
	// Both grants are present, deliberately: an INCOMPLETE observation upserts
	// what it reached and deletes nothing, because a row it never got to is not
	// a row it saw gone. That is the same rule the sweep applies one layer up,
	// and it is exactly why the completeness has to describe these rows and not
	// some other read's.
	obs, grants, err := ObservationSnapshot(ctx)
	if err != nil {
		t.Fatalf("read after: %v", err)
	}
	if obs.Complete {
		t.Fatal("generation two was incomplete; a later read must say so")
	}
	if len(grants) != 2 {
		t.Fatalf("an incomplete sweep deletes nothing, so both rows survive; got %+v", grants)
	}
}

// Nobody has looked yet. That is not an outage and not an empty world, and the
// pairing must not flatten it into either — the sweep renders it as
// "not_checked_yet" and refuses to write a clean bill.
func TestObservationSnapshot_NeverObservedTravelsAsItself(t *testing.T) {
	ctx := liveDB(t)
	truncateObservations(t, ctx)

	_, _, err := ObservationSnapshot(ctx)
	if !errors.Is(err, ErrNoObservation) {
		t.Fatalf("want ErrNoObservation to reach the caller, got %v", err)
	}
}

// A write that would ENLIST in the snapshot is refused rather than quietly
// running inside a transaction that holds no access lock. ConfirmFromObservation
// goes through querier(ctx), so it is one of those; a write that opens its own
// transaction is not, and correctly commits outside.
func TestInReadSnapshot_RefusesAWriteThatWouldJoinIt(t *testing.T) {
	ctx := liveDB(t)
	truncateObservations(t, ctx)

	err := InReadSnapshot(ctx, func(snapCtx context.Context) error {
		_, confirmErr := ConfirmFromObservation(snapCtx)
		return confirmErr
	})
	if err == nil {
		t.Fatal("a write inside a read-only snapshot must be refused, not quietly performed")
	}
}
