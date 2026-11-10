package services

import (
	"context"
	"testing"
	"time"

	"syndra/internal/db"
	"syndra/internal/models"
)

func deliveries(t *testing.T, rows ...db.RoleDelivery) {
	t.Helper()
	orig := svcRoleDeliveries
	t.Cleanup(func() { svcRoleDeliveries = orig })
	svcRoleDeliveries = func(_ context.Context, ids []string) (map[string][]db.RoleDelivery, error) {
		return map[string][]db.RoleDelivery{ids[0]: rows}, nil
	}
}

func standingIn(t *testing.T, p models.ProjectAccessView, key string) *models.RoleStanding {
	t.Helper()
	s, ok := p.Standings[key]
	if !ok {
		t.Fatalf("no standing sent for %s on %s: %v", key, p.ProjectID, p.Standings)
	}
	return s
}

// The person page carries one standing for every role it shows: a given role
// whose removal is queued is still held (being removed), a given role sent and
// not seen back is "sent", and a role Zitadel shows that Syndra never gave is
// held with no reasons.
func TestExplainUserAccess_EveryRoleCarriesItsStanding(t *testing.T) {
	noRecordsFor(t, []models.DirectGrant{{ProjectID: "p1", RoleKey: "laser"}, {ProjectID: "p1", RoleKey: "lathe"}})
	observedStore(t, db.Observation{ObservedAt: time.Now(), Complete: true}, nil,
		[]db.ObservedGrant{{ProjectID: "p1", RoleKeys: []string{"laser", "kiln"}}})
	deliveries(t,
		db.RoleDelivery{ProjectID: "p1", RoleKey: "laser", OpType: "revoke", Status: "pending", IntentSeq: 2},
		db.RoleDelivery{ProjectID: "p1", RoleKey: "lathe", OpType: "add", Status: "applied", IntentSeq: 1},
	)

	view, err := ExplainUserAccess(context.Background(), "dev_admin")
	if err != nil {
		t.Fatal(err)
	}
	p := projectIn(t, view, "p1")
	assertStanding(t, standingIn(t, p, "laser"), models.StandingRemoving)
	assertStanding(t, standingIn(t, p, "lathe"), models.StandingSent)
	assertStanding(t, standingIn(t, p, "kiln"), models.StandingHeld)
}

// Zitadel never read: a given role with nothing in the outbox has no standing
// (absence can't be concluded), and one waiting to be sent still says so.
func TestExplainUserAccess_NeverReadLeavesSettledRolesWithoutAStanding(t *testing.T) {
	noRecordsFor(t, []models.DirectGrant{{ProjectID: "p1", RoleKey: "laser"}, {ProjectID: "p1", RoleKey: "lathe"}})
	observedStore(t, db.Observation{}, db.ErrNoObservation, nil)
	deliveries(t, db.RoleDelivery{ProjectID: "p1", RoleKey: "lathe", OpType: "add", Status: "pending", IntentSeq: 1})

	view, err := ExplainUserAccess(context.Background(), "dev_admin")
	if err != nil {
		t.Fatal(err)
	}
	p := projectIn(t, view, "p1")
	assertStanding(t, standingIn(t, p, "laser"), "")
	assertStanding(t, standingIn(t, p, "lathe"), models.StandingWaiting)
}

// Two grant rows on one project: the page keeps the roles of both (the old
// fold kept only the last row's).
func TestExplainUserAccess_TwoGrantRowsOnOneProjectKeepBoth(t *testing.T) {
	noRecordsFor(t, nil)
	observedStore(t, db.Observation{ObservedAt: time.Now(), Complete: true}, nil, []db.ObservedGrant{
		{GrantID: "g1", ProjectID: "p1", RoleKeys: []string{"laser"}},
		{GrantID: "g2", ProjectID: "p1", RoleKeys: []string{"kiln"}},
	})

	view, err := ExplainUserAccess(context.Background(), "dev_admin")
	if err != nil {
		t.Fatal(err)
	}
	got := projectIn(t, view, "p1").ObservedRoleKeys
	if len(got) != 2 || got[0] != "kiln" || got[1] != "laser" {
		t.Fatalf("observed %v, want both rows' roles [kiln laser]", got)
	}
}

// A role's holder list gives each member the same standing their own page
// does: a member whose removal is queued is still held, being removed.
func TestRoleMembers_EachMemberCarriesTheirStanding(t *testing.T) {
	setupSnapshotTestFixtures(t, 2, 0, 1)
	stubRoleFixtures(t)
	observedStore(t, db.Observation{ObservedAt: time.Now(), Complete: true}, nil,
		[]db.ObservedGrant{{ProjectID: "p0", RoleKeys: []string{"trained"}}})
	origCollect := collectUserRolesHook
	t.Cleanup(func() { collectUserRolesHook = origCollect })
	collectUserRolesHook = func(context.Context, string) (map[roleKey]*models.EffectiveRole, []models.Bundle, error) {
		return map[roleKey]*models.EffectiveRole{
			{projectID: "p0", roleKey: "trained"}: {ProjectID: "p0", RoleKey: "trained", IsSource: true, Reasons: []models.RoleReason{{Kind: "direct"}}},
		}, nil, nil
	}
	orig := svcRoleDeliveries
	t.Cleanup(func() { svcRoleDeliveries = orig })
	svcRoleDeliveries = func(_ context.Context, ids []string) (map[string][]db.RoleDelivery, error) {
		if ids[0] != "u0" {
			return nil, nil
		}
		return map[string][]db.RoleDelivery{"u0": {{ProjectID: "p0", RoleKey: "trained", OpType: "revoke", Status: "pending", IntentSeq: 1}}}, nil
	}

	view, err := RoleMembers(context.Background(), "p0", "trained")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]*models.RoleStanding{}
	for _, m := range view.Members {
		got[m.User.ID] = m.Standing
	}
	assertStanding(t, got["u0"], models.StandingRemoving)
	assertStanding(t, got["u1"], models.StandingHeld)
}
