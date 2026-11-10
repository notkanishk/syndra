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
	svcRoleDeliveries = func(_ context.Context, ids []string) (map[string]db.Deliveries, error) {
		return map[string]db.Deliveries{ids[0]: db.NewDeliveries(rows...)}, nil
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
	svcRoleDeliveries = func(_ context.Context, ids []string) (map[string]db.Deliveries, error) {
		if ids[0] != "u0" {
			return nil, nil
		}
		return map[string]db.Deliveries{"u0": db.NewDeliveries(db.RoleDelivery{ProjectID: "p0", RoleKey: "trained", OpType: "revoke", Status: "pending", IntentSeq: 1})}, nil
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

// The view's holdings are the request check's answer, role for role: a direct
// grant Zitadel doesn't show is still had (giving it again is refused), a
// role being removed is not, and a role sent and not seen back is.
func TestExplainUserAccess_HoldingsAreTheRequestChecksAnswer(t *testing.T) {
	until := time.Now().Add(72 * time.Hour)
	noRecordsFor(t, []models.DirectGrant{
		{ProjectID: "p1", RoleKey: "laser"},
		{ProjectID: "p1", RoleKey: "lathe", ExpiresAt: &until},
		{ProjectID: "p1", RoleKey: "kiln"},
	})
	observedStore(t, db.Observation{ObservedAt: time.Now(), Complete: true}, nil,
		[]db.ObservedGrant{{ProjectID: "p1", RoleKeys: []string{"kiln", "drill"}}})
	deliveries(t,
		db.RoleDelivery{ProjectID: "p1", RoleKey: "kiln", OpType: "revoke", Status: "pending", IntentSeq: 2},
		db.RoleDelivery{ProjectID: "p1", RoleKey: "drill", OpType: "add", Status: "applied", Confirmed: true, IntentSeq: 1},
	)

	view, err := ExplainUserAccess(context.Background(), "dev_admin")
	if err != nil {
		t.Fatal(err)
	}
	p := projectIn(t, view, "p1")
	for key, want := range map[string]string{
		"laser": models.HoldingForGood, // given, not in Zitadel: still had
		"lathe": models.HoldingUntil,
		"kiln":  models.HoldingNone, // being removed
		"drill": models.HoldingForGood,
	} {
		if got := p.Holdings[key].Kind; got != want {
			t.Errorf("%s: holding %q, want %q", key, got, want)
		}
		h, err := RequestHolding(context.Background(), "dev_admin", "p1", key)
		if err != nil {
			t.Fatal(err)
		}
		if h.Kind != p.Holdings[key].Kind {
			t.Errorf("%s: the request check says %q, the view %q", key, h.Kind, p.Holdings[key].Kind)
		}
	}
}

// The person page stands on the person's own basis: after a capped org sweep,
// their own complete read settles what they don't hold, so a given role
// Zitadel lacks reads "not held", not "couldn't check".
func TestExplainUserAccess_TheirOwnCompleteReadSettlesAbsence(t *testing.T) {
	noRecordsFor(t, []models.DirectGrant{{ProjectID: "p1", RoleKey: "laser"}})
	swept := time.Now().Add(-time.Hour)
	observedStore(t, db.Observation{ObservedAt: swept, Complete: false}, nil, nil)
	orig := svcLatestUserObservations
	t.Cleanup(func() { svcLatestUserObservations = orig })
	svcLatestUserObservations = func(context.Context, []string) (map[string]db.Observation, error) {
		return map[string]db.Observation{"dev_admin": {Scope: "user", ObservedAt: time.Now(), Complete: true}}, nil
	}

	view, err := ExplainUserAccess(context.Background(), "dev_admin")
	if err != nil {
		t.Fatal(err)
	}
	assertStanding(t, standingIn(t, projectIn(t, view, "p1"), "laser"), models.StandingNotHeld)
	if !view.Observation.ConcludesAbsence() {
		t.Fatalf("the page's basis %+v is not their own complete read", view.Observation)
	}
}

// A role a bundle gives has a ledger row (every give Syndra queues writes one),
// so it is had even before Zitadel was read: a second ask would duplicate it.
func TestExplainUserAccess_ABundleRoleIsHadThroughItsLedgerRow(t *testing.T) {
	noRecordsFor(t, []models.DirectGrant{{ProjectID: "p1", RoleKey: "laser", Source: "bundle", SourceRef: "b1"}})
	observedStore(t, db.Observation{}, db.ErrNoObservation, nil)

	view, err := ExplainUserAccess(context.Background(), "dev_admin")
	if err != nil {
		t.Fatal(err)
	}
	if got := projectIn(t, view, "p1").Holdings["laser"].Kind; got != models.HoldingForGood {
		t.Fatalf("holding %q, want for_good", got)
	}
}

// A role their bundle gives is had even with no ledger row and Zitadel unread
// (production, 10 Nov 2026: 23 of 24 bundle-held roles have no ledger row), so
// the member catalogue never offers it and a second ask is refused.
func TestRequestHolding_ARecordedRoleIsHadWithoutALedgerRow(t *testing.T) {
	noRecordsFor(t, nil)
	svcGetBundlesForUser = func(context.Context, string) ([]models.Bundle, error) {
		return []models.Bundle{{ID: "b1", Name: "Makers"}}, nil
	}
	svcGetUserBundleRolesGrouped = func(context.Context, string) (map[string][]models.BundleRole, error) {
		return map[string][]models.BundleRole{"b1": {{ProjectID: "p1", RoleKey: "laser"}}}, nil
	}
	observedStore(t, db.Observation{}, db.ErrNoObservation, nil)

	h, err := RequestHolding(context.Background(), "dev_admin", "p1", "laser")
	if err != nil {
		t.Fatal(err)
	}
	if h.Kind != models.HoldingForGood {
		t.Fatalf("holding %+v, want for_good: the bundle gives it", h)
	}
}

// Someone Zitadel shows holding the role with no record in Syndra is listed
// with a standing too: held, or being removed once their removal is queued, the
// same answer their own page gives.
func TestRoleMembers_ObservedOnlyHoldersCarryTheirStanding(t *testing.T) {
	setupSnapshotTestFixtures(t, 2, 0, 1)
	stubRoleFixtures(t)
	observedStore(t, db.Observation{ObservedAt: time.Now(), Complete: true}, nil,
		[]db.ObservedGrant{{ProjectID: "p0", RoleKeys: []string{"trained"}}})
	origCollect := collectUserRolesHook
	t.Cleanup(func() { collectUserRolesHook = origCollect })
	collectUserRolesHook = func(context.Context, string) (map[roleKey]*models.EffectiveRole, []models.Bundle, error) {
		return map[roleKey]*models.EffectiveRole{}, nil, nil
	}
	orig := svcRoleDeliveries
	t.Cleanup(func() { svcRoleDeliveries = orig })
	svcRoleDeliveries = func(_ context.Context, ids []string) (map[string]db.Deliveries, error) {
		if len(ids) != 2 {
			t.Errorf("primed %v, want both holders at once", ids)
		}
		return map[string]db.Deliveries{"u0": db.NewDeliveries(db.RoleDelivery{ProjectID: "p0", RoleKey: "trained", OpType: "revoke", Status: "pending", IntentSeq: 1})}, nil
	}

	view, err := RoleMembers(context.Background(), "p0", "trained")
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Members) != 0 || len(view.ObservedOnly) != 2 {
		t.Fatalf("members %d, observed only %d; want 0 and 2", len(view.Members), len(view.ObservedOnly))
	}
	got := map[string]*models.RoleStanding{}
	for _, m := range view.ObservedOnly {
		if len(m.Reasons) != 0 {
			t.Fatalf("%s: reasons %v, want none (Syndra didn't give it)", m.User.ID, m.Reasons)
		}
		got[m.User.ID] = m.Standing
	}
	assertStanding(t, got["u0"], models.StandingRemoving)
	assertStanding(t, got["u1"], models.StandingHeld)
}
