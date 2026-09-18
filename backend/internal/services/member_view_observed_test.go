package services

import (
	"context"
	"testing"
	"time"

	"syndra/internal/db"
	"syndra/internal/models"
)

// noRecordsFor makes collectUserRoles find nothing but the direct grants given.
func noRecordsFor(t *testing.T, direct []models.DirectGrant) {
	t.Helper()
	resetGovernanceDeps(t)
	svcGetDirectGrantsForUser = func(context.Context, string, bool) ([]models.DirectGrant, error) {
		return direct, nil
	}
	svcGetBundlesForUser = func(context.Context, string) ([]models.Bundle, error) { return nil, nil }
	svcGetActiveMappingRules = func(context.Context) ([]models.MappingRule, error) { return nil, nil }
	svcGetUserBundleRolesGrouped = func(context.Context, string) (map[string][]models.BundleRole, error) {
		return nil, nil
	}
	svcGetRolesForBundle = func(context.Context, string) ([]models.BundleRole, error) { return nil, nil }
}

func observedStore(t *testing.T, obs db.Observation, obsErr error, grants []db.ObservedGrant) {
	t.Helper()
	origObs, origGrants := svcLatestOrgObservation, svcObservedGrantsFor
	t.Cleanup(func() { svcLatestOrgObservation, svcObservedGrantsFor = origObs, origGrants })
	svcLatestOrgObservation = func(context.Context) (db.Observation, error) { return obs, obsErr }
	svcObservedGrantsFor = func(context.Context, string) ([]db.ObservedGrant, error) { return grants, nil }
}

func projectIn(t *testing.T, view models.UserAccessView, projectID string) models.ProjectAccessView {
	t.Helper()
	for _, p := range view.Projects {
		if p.ProjectID == projectID {
			return p
		}
	}
	t.Fatalf("project %q missing from the view", projectID)
	return models.ProjectAccessView{}
}

// Nothing has ever been observed. The person's own page may not answer "what
// can you use" with a number, because nobody has looked — and a zero here is
// the sentence "you can use nothing", which is a different and possibly false
// statement.
func TestExplainUserAccess_NeverObservedIsNotAnEmptyHolding(t *testing.T) {
	noRecordsFor(t, []models.DirectGrant{{ProjectID: "p1", RoleKey: "laser"}})
	observedStore(t, db.Observation{}, db.ErrNoObservation, nil)

	view, err := ExplainUserAccess(context.Background(), "dev_admin")
	if err != nil {
		t.Fatalf("ExplainUserAccess: %v", err)
	}
	if view.ObservedRoleCount != nil {
		t.Fatalf("nothing observed, yet a count was reported: %d", *view.ObservedRoleCount)
	}
	if view.Observation.ReadAt != nil {
		t.Fatal("a basis with no observation must carry no read time")
	}
	if got := projectIn(t, view, "p1"); got.ObservedRoleKeys != nil {
		t.Fatalf("nothing observed, yet the project claims a holding: %v", got.ObservedRoleKeys)
	}
}

// A covering read exists. The count is what Zitadel holds — not what Syndra's
// records say was decided. The member landing summed the records and put "two
// permissions" above two rows that both said the role had not arrived.
func TestExplainUserAccess_CountsWhatZitadelHoldsNotWhatWasDecided(t *testing.T) {
	noRecordsFor(t, []models.DirectGrant{
		{ProjectID: "p1", RoleKey: "laser"},
		{ProjectID: "p1", RoleKey: "mill"}, // recorded, never delivered
	})
	observedStore(t,
		db.Observation{ObservedAt: time.Now(), Complete: true},
		nil,
		[]db.ObservedGrant{{GrantID: "g1", UserID: "dev_admin", ProjectID: "p1", RoleKeys: []string{"laser"}}},
	)

	view, err := ExplainUserAccess(context.Background(), "dev_admin")
	if err != nil {
		t.Fatalf("ExplainUserAccess: %v", err)
	}
	if view.ObservedRoleCount == nil || *view.ObservedRoleCount != 1 {
		t.Fatalf("want 1 role observed, got %v (records said 2)", view.ObservedRoleCount)
	}
	got := projectIn(t, view, "p1")
	if len(got.EffectiveRoleKeys) != 2 {
		t.Fatalf("the records must still be reported in full, got %v", got.EffectiveRoleKeys)
	}
	if len(got.ObservedRoleKeys) != 1 || got.ObservedRoleKeys[0] != "laser" {
		t.Fatalf("want only the delivered role observed, got %v", got.ObservedRoleKeys)
	}
}

// A project the person holds in Zitadel that no Syndra record explains. The
// operator sees it as "unexplained"; the person holding it saw nothing at all,
// because the page was built entirely from the records.
func TestExplainUserAccess_ShowsAccessNoRecordExplains(t *testing.T) {
	noRecordsFor(t, nil)
	observedStore(t,
		db.Observation{ObservedAt: time.Now(), Complete: true},
		nil,
		[]db.ObservedGrant{{GrantID: "g9", UserID: "dev_admin", ProjectID: "p9", RoleKeys: []string{"kiln", "anvil"}}},
	)

	view, err := ExplainUserAccess(context.Background(), "dev_admin")
	if err != nil {
		t.Fatalf("ExplainUserAccess: %v", err)
	}
	got := projectIn(t, view, "p9")
	if len(got.SourceRoles) != 0 || len(got.DerivedRoles) != 0 {
		t.Fatal("nothing explains this access; it must carry no reasons")
	}
	// Sorted, so two reads of the same holding render identically.
	if len(got.ObservedRoleKeys) != 2 || got.ObservedRoleKeys[0] != "anvil" || got.ObservedRoleKeys[1] != "kiln" {
		t.Fatalf("want the held roles, sorted, got %v", got.ObservedRoleKeys)
	}
	if view.ObservedRoleCount == nil || *view.ObservedRoleCount != 2 {
		t.Fatalf("want both held roles counted, got %v", view.ObservedRoleCount)
	}
}

// A project whose records exist and whose observation shows nothing: the read
// happened and saw no grant, so an empty list is a checked absence and may be
// rendered as one.
func TestExplainUserAccess_ObservedNothingIsAnEmptyListNotNil(t *testing.T) {
	noRecordsFor(t, []models.DirectGrant{{ProjectID: "p1", RoleKey: "laser"}})
	observedStore(t, db.Observation{ObservedAt: time.Now(), Complete: true}, nil, nil)

	view, err := ExplainUserAccess(context.Background(), "dev_admin")
	if err != nil {
		t.Fatalf("ExplainUserAccess: %v", err)
	}
	got := projectIn(t, view, "p1")
	if got.ObservedRoleKeys == nil {
		t.Fatal("the read happened; an absence it established must not read as 'never checked'")
	}
	if len(got.ObservedRoleKeys) != 0 {
		t.Fatalf("want an empty holding, got %v", got.ObservedRoleKeys)
	}
	if view.ObservedRoleCount == nil || *view.ObservedRoleCount != 0 {
		t.Fatalf("want a checked zero, got %v", view.ObservedRoleCount)
	}
}
