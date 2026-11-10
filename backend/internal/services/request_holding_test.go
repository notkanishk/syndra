package services

import (
	"context"
	"testing"
	"time"

	"syndra/internal/db"
	"syndra/internal/models"
)

// holdingWith stubs the three reads RequestHolding rests on: the person's
// direct grants, what Zitadel shows, and their outbox rows.
func holdingWith(t *testing.T, direct []models.DirectGrant, seen []string, rows ...db.RoleDelivery) func(role string) models.RequestHolding {
	t.Helper()
	noRecordsFor(t, direct)
	observedStore(t, db.Observation{ObservedAt: time.Now(), Complete: true}, nil,
		[]db.ObservedGrant{{ProjectID: "pLaser", RoleKeys: seen}})
	deliveries(t, rows...)
	return func(role string) models.RequestHolding {
		t.Helper()
		h, err := RequestHolding(context.Background(), "u1", "pLaser", role)
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
}

func queued(op, role string, seq int64, keys ...string) db.RoleDelivery {
	return db.RoleDelivery{ProjectID: "pLaser", RoleKey: role, OpType: op, RoleKeys: keys, Status: "pending", IntentSeq: seq}
}

// Grants and Zitadel: nothing, an ending grant, a grant for good, and a role
// Zitadel shows with no grant from Syndra (a bundle, a rule, or given there).
func TestRequestHolding_ReadsGrantsAndZitadel(t *testing.T) {
	until := time.Now().Add(48 * time.Hour)
	read := holdingWith(t, []models.DirectGrant{
		{ProjectID: "pLaser", RoleKey: "trained", ExpiresAt: &until},
		{ProjectID: "pLaser", RoleKey: "door"},
	}, []string{"staff"})
	if h := read("nothing"); h.Kind != models.HoldingNone {
		t.Fatalf("nothing held, got %+v", h)
	}
	if h := read("trained"); h.Kind != models.HoldingUntil || h.Until == nil {
		t.Fatalf("an ending grant is held until it ends, got %+v", h)
	}
	if h := read("door"); h.Kind != models.HoldingForGood {
		t.Fatalf("a grant with no end is held for good, got %+v", h)
	}
	if h := read("staff"); h.Kind != models.HoldingForGood || h.Waiting {
		t.Fatalf("seen in Zitadel, nothing queued: held for good, not waiting, got %+v", h)
	}
}

// Given and not sent is had and says so; a role whose removal is queued is not
// had, even while Zitadel still shows it or a direct grant records it.
func TestRequestHolding_ReadsWhatIsQueued(t *testing.T) {
	read := holdingWith(t, []models.DirectGrant{{ProjectID: "pLaser", RoleKey: "saw"}}, []string{"door"},
		queued("add", "trained", 1),
		queued("revoke", "door", 2),
	)
	if h := read("trained"); h.Kind != models.HoldingForGood || !h.Waiting {
		t.Fatalf("a queued add is had and waiting, got %+v", h)
	}
	if h := read("door"); h.Kind != models.HoldingNone {
		t.Fatalf("a role whose removal is queued is not had, got %+v", h)
	}
	if h := read("saw"); h.Kind != models.HoldingForGood {
		t.Fatalf("a revoke of another role leaves this one had, got %+v", h)
	}
}

// A queued replace states the project's whole set: it removes a role it omits
// (even one a direct grant records) and brings in one it names.
func TestRequestHolding_AReplaceThatOmitsARoleRemovesIt(t *testing.T) {
	read := holdingWith(t, []models.DirectGrant{{ProjectID: "pLaser", RoleKey: "saw"}}, nil,
		queued("replace", "", 3, "keep"),
	)
	if h := read("saw"); h.Kind != models.HoldingNone {
		t.Fatalf("a queued replace that omits the role removes it, got %+v", h)
	}
	if h := read("keep"); h.Kind != models.HoldingForGood || !h.Waiting {
		t.Fatalf("a role the replace names is on its way in, got %+v", h)
	}
}

// Owner, 10 Nov 2026: sent and not seen back yet is on its way, so it is had;
// asking for it again is refused rather than made twice.
func TestRequestHolding_SentAndNotSeenBackIsHad(t *testing.T) {
	read := holdingWith(t, nil, nil, db.RoleDelivery{ProjectID: "pLaser", RoleKey: "trained", OpType: "add", Status: "applied", IntentSeq: 1})
	if h := read("trained"); h.Kind != models.HoldingForGood {
		t.Fatalf("a give sent and not seen back is had, got %+v", h)
	}
}
