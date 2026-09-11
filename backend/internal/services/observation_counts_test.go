package services

import (
	"context"
	"testing"
	"time"

	"syndra/internal/db"
	"syndra/internal/directory"
	"syndra/internal/models"
)

// withConfirmationDeps swaps the directory and the observation-store seams
// for the duration of a test, restoring all three on cleanup.
func withConfirmationDeps(
	t *testing.T,
	users []models.UserProfile,
	roleMaps map[string]map[roleKey]*models.EffectiveRole,
	obs func(context.Context) (db.Observation, error),
	grants func(context.Context, string) ([]db.ObservedGrant, error),
) {
	t.Helper()
	origDir := directory.Default
	origCollect := collectUserRolesHook
	origObs := svcLatestOrgObservation
	origGrants := svcObservedGrantsFor
	t.Cleanup(func() {
		directory.Default = origDir
		collectUserRolesHook = origCollect
		svcLatestOrgObservation = origObs
		svcObservedGrantsFor = origGrants
	})

	directory.Default = &snapshotFixtureDirectory{users: users}
	collectUserRolesHook = func(_ context.Context, userID string) (map[roleKey]*models.EffectiveRole, []models.Bundle, error) {
		return roleMaps[userID], nil, nil
	}
	svcLatestOrgObservation = obs
	svcObservedGrantsFor = grants
}

// db.ErrNoObservation must never read as "nothing is there" — a confirmed
// count computed on top of it would be exactly that mistake, so
// ConfirmedHolderCounts must refuse to produce one.
func TestRoleHolderConfirmation_NeverObservedIsNilNotZero(t *testing.T) {
	withConfirmationDeps(t,
		[]models.UserProfile{{ID: "u1"}},
		map[string]map[roleKey]*models.EffectiveRole{
			"u1": {{projectID: "p1", roleKey: "member"}: {ProjectID: "p1", RoleKey: "member"}},
		},
		func(context.Context) (db.Observation, error) { return db.Observation{}, db.ErrNoObservation },
		func(context.Context, string) ([]db.ObservedGrant, error) { return nil, nil },
	)

	counts, basis, err := RoleHolderConfirmation(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if counts != nil {
		t.Fatalf("expected nil counts when the org has never been observed, got %v — nil is not the same fact as zero", counts)
	}
	if basis.ReadAt != nil {
		t.Fatalf("expected ReadAt nil (\"not checked yet\"), got %v", basis.ReadAt)
	}
}

// Recorded (what Syndra decided) and Confirmed (what the observation store
// backs up) are two different, both-useful facts. One person's decided role
// with no matching observed grant must lower the confirmed count without
// touching the recorded one — neither number is wrong.
func TestRoleHolderConfirmation_ConfirmsOnlyASubsetOfRecorded(t *testing.T) {
	at := time.Now()
	withConfirmationDeps(t,
		[]models.UserProfile{{ID: "u1"}, {ID: "u2"}},
		map[string]map[roleKey]*models.EffectiveRole{
			"u1": {{projectID: "p1", roleKey: "member"}: {ProjectID: "p1", RoleKey: "member"}},
			"u2": {{projectID: "p1", roleKey: "member"}: {ProjectID: "p1", RoleKey: "member"}},
		},
		func(context.Context) (db.Observation, error) {
			return db.Observation{Scope: "org", ObservedAt: at, Complete: true, GrantsSeen: 1}, nil
		},
		func(_ context.Context, userID string) ([]db.ObservedGrant, error) {
			if userID == "u1" {
				// Zitadel confirms u1's grant.
				return []db.ObservedGrant{{GrantID: "g1", UserID: "u1", ProjectID: "p1", RoleKeys: []string{"member"}}}, nil
			}
			// u2 is recorded as holding the role, but nothing observed it —
			// Undelivered, or drift, but not "In force".
			return nil, nil
		},
	)

	recorded, err := RoleHolderCounts(context.Background())
	if err != nil {
		t.Fatalf("RoleHolderCounts: %v", err)
	}
	if recorded["p1:member"] != 2 {
		t.Fatalf("expected 2 recorded holders, got %d", recorded["p1:member"])
	}

	confirmed, basis, err := RoleHolderConfirmation(context.Background())
	if err != nil {
		t.Fatalf("RoleHolderConfirmation: %v", err)
	}
	if basis.ReadAt == nil || !basis.Current || basis.Truncated {
		t.Fatalf("expected a current, complete basis, got %+v", basis)
	}
	if confirmed["p1:member"] != 1 {
		t.Fatalf("expected 1 confirmed holder (u2 recorded but not observed), got %d", confirmed["p1:member"])
	}
	if recorded["p1:member"] == confirmed["p1:member"] {
		t.Fatalf("recorded (%d) and confirmed (%d) must differ here — that difference is the point of the test",
			recorded["p1:member"], confirmed["p1:member"])
	}
}

// A failed or capped sweep is real about what it saw and silent about the
// rest. The basis has to say so plainly — Current false on a failed read,
// Truncated true on a capped one — so no surface can present a confirmed
// count from it as exhaustive.
func TestObservationBasis_NamesFailureAndTruncationSeparately(t *testing.T) {
	withConfirmationDeps(t,
		nil, nil,
		func(context.Context) (db.Observation, error) {
			return db.Observation{Scope: "org", ObservedAt: time.Now(), Complete: false, Error: "zitadel unreachable"}, nil
		},
		func(context.Context, string) ([]db.ObservedGrant, error) { return nil, nil },
	)

	_, basis, err := RoleHolderConfirmation(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if basis.ReadAt == nil {
		t.Fatalf("a failed attempt is still a checked-at fact, not \"never observed\"")
	}
	if basis.Current {
		t.Fatalf("expected Current=false — the sweep itself failed")
	}
	if !basis.Truncated {
		t.Fatalf("expected Truncated=true — an incomplete read may not be presented as exhaustive")
	}
}

// A holder Zitadel shows that Syndra never gave must still be counted as a
// holder. Given and Observed are different readings of one snapshot; the old
// "confirmed" number was their intersection and hid every unexplained holder
// from every count on every screen.
func TestRoleHolderFacts_ObservedCountsWhoeverGaveIt(t *testing.T) {
	now := time.Now()
	withConfirmationDeps(t,
		[]models.UserProfile{{ID: "u1"}, {ID: "u2"}},
		map[string]map[roleKey]*models.EffectiveRole{
			"u1": {{projectID: "p1", roleKey: "member"}: {ProjectID: "p1", RoleKey: "member"}},
			"u2": {},
		},
		func(context.Context) (db.Observation, error) {
			return db.Observation{ObservedAt: now, Complete: true}, nil
		},
		func(_ context.Context, userID string) ([]db.ObservedGrant, error) {
			switch userID {
			case "u1":
				return []db.ObservedGrant{{UserID: "u1", ProjectID: "p1", RoleKeys: []string{"member", "admin"}}}, nil
			case "u2":
				return []db.ObservedGrant{{UserID: "u2", ProjectID: "p1", RoleKeys: []string{"member"}}}, nil
			}
			return nil, nil
		},
	)

	facts, err := RoleHolderFacts(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := facts.Given["p1:member"]; got != 1 {
		t.Fatalf("given p1:member = %d, want 1", got)
	}
	if got := facts.Confirmed["p1:member"]; got != 1 {
		t.Fatalf("confirmed p1:member = %d, want 1", got)
	}
	if got := facts.Observed["p1:member"]; got != 2 {
		t.Fatalf("observed p1:member = %d, want 2 — u2 holds it in Zitadel with no Syndra record", got)
	}
	if got := facts.Observed["p1:admin"]; got != 1 {
		t.Fatalf("observed p1:admin = %d, want 1", got)
	}
}

// Never observed is not "nobody holds it". A nil Observed map must not let a
// role be called unused; only a complete, current read may.
func TestHolderFacts_NobodyObservedNeedsACompleteRead(t *testing.T) {
	never := HolderFacts{}
	if never.NobodyObserved("p1:member") {
		t.Fatalf("a never-observed role was called unheld")
	}
	truncated := HolderFacts{Observed: map[string]int{}, Basis: models.ObservationBasis{Current: true, Truncated: true}}
	if truncated.NobodyObserved("p1:member") {
		t.Fatalf("a truncated read was allowed to conclude an absence")
	}
	complete := HolderFacts{Observed: map[string]int{}, Basis: models.ObservationBasis{Current: true}}
	if !complete.NobodyObserved("p1:member") {
		t.Fatalf("a complete read that does not show the role should conclude nobody holds it")
	}
}
