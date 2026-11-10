package services

import (
	"context"
	"testing"
	"time"

	"syndra/internal/models"
)

// snapshotWith builds an accessSnapshot without a directory or database: the
// People index is pure aggregation over role resolution, and the interesting
// behaviour is in what it aggregates, not where it came from.
func snapshotWith(users []models.UserProfile, roles map[string]map[roleKey]*models.EffectiveRole,
	bundles map[string][]models.Bundle) *accessSnapshot {
	// basisComputed with a zero basis: this fixture has observed nothing, which
	// is the honest default and keeps Basis() from reaching a nil pool. A test
	// that needs a covering read sets snap.basis itself.
	snap := &accessSnapshot{
		ctx: context.Background(), users: users, roles: map[string]userRoles{},
		basisComputed: true,
	}
	for _, u := range users {
		snap.roles[u.ID] = userRoles{roleMap: roles[u.ID], bundles: bundles[u.ID]}
	}
	return snap
}

func role(projectID, key, projectName string) (roleKey, *models.EffectiveRole) {
	return roleKey{projectID: projectID, roleKey: key},
		&models.EffectiveRole{ProjectID: projectID, ProjectName: projectName, RoleKey: key}
}

// The "needs attention" column is the reason this list exists rather than being
// a plain directory. Each signal has to reach the row it belongs to — and only
// that row.
func TestPeopleIndex_CarriesNeedsAttentionPerPerson(t *testing.T) {
	k1, r1 := role("p_laser", "trained", "Laser Lab")
	users := []models.UserProfile{
		{ID: "u_kevin", Name: "Kevin Owens"},
		{ID: "u_amara", Name: "Apollo Crews"},
	}
	snap := snapshotWith(users,
		map[string]map[roleKey]*models.EffectiveRole{
			"u_kevin": {k1: r1},
			"u_amara": {},
		},
		map[string][]models.Bundle{
			"u_kevin": {{ID: "b1", Name: "Lab Tech"}, {ID: "b2", Name: "Studio Member"}},
		})

	soon := time.Now().Add(48 * time.Hour)
	attention := attentionIndex{
		expiring:    map[string]int{"u_kevin": 1},
		soonest:     map[string]time.Time{"u_kevin": soon},
		requests:    map[string]int{"u_amara": 1},
		unexplained: map[string]int{},
	}

	items, err := listUsersFromSnapshot(snap, "", attention)
	if err != nil {
		t.Fatal(err)
	}

	byID := map[string]models.UserListItem{}
	for _, item := range items {
		byID[item.User.ID] = item
	}

	kevin := byID["u_kevin"]
	if kevin.ExpiringCount != 1 || kevin.SoonestExpiry == nil {
		t.Errorf("Kevin has an expiring grant; got count=%d soonest=%v", kevin.ExpiringCount, kevin.SoonestExpiry)
	}
	if kevin.OpenRequestCount != 0 {
		t.Errorf("Apollo's request must not land on Kevin's row, got %d", kevin.OpenRequestCount)
	}
	if len(kevin.BundleNames) != 2 || kevin.BundleNames[0] != "Lab Tech" {
		t.Errorf("bundle names are pills, not a count: got %v", kevin.BundleNames)
	}
	if kevin.ProjectCount != 1 {
		t.Errorf("project count = %d, want 1", kevin.ProjectCount)
	}

	amara := byID["u_amara"]
	if amara.OpenRequestCount != 1 {
		t.Errorf("Apollo has one open request, got %d", amara.OpenRequestCount)
	}
	if amara.ExpiringCount != 0 || amara.SoonestExpiry != nil {
		t.Error("Apollo has nothing expiring; the row must say so with a dash, not borrow Kevin's date")
	}
}

// Search matches role keys as well as names and emails: "who has trained in the
// laser lab" is typed here before anyone thinks to go to Roles.
func TestPeopleIndex_SearchMatchesRoleKeys(t *testing.T) {
	k1, r1 := role("p_laser", "trained", "Laser Lab")
	users := []models.UserProfile{
		{ID: "u_holder", Name: "Nia Jax", Email: "nia@example.org"},
		{ID: "u_other", Name: "Shinsuke Nakamura", Email: "shinsuke@example.org"},
	}
	snap := snapshotWith(users,
		map[string]map[roleKey]*models.EffectiveRole{
			"u_holder": {k1: r1},
			"u_other":  {},
		}, nil)

	items, err := listUsersFromSnapshot(snap, "trained", attentionIndex{
		expiring: map[string]int{}, soonest: map[string]time.Time{},
		requests: map[string]int{}, unexplained: map[string]int{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].User.ID != "u_holder" {
		t.Fatalf("role-key search must return only the holder, got %d rows", len(items))
	}
}

// A name search still works, and must not be broadened into "anyone whose role
// key happens to contain these letters".
func TestPeopleIndex_SearchStillMatchesNames(t *testing.T) {
	users := []models.UserProfile{
		{ID: "u1", Name: "Nia Jax", Email: "nia@example.org"},
		{ID: "u2", Name: "Shinsuke Nakamura", Email: "shinsuke@example.org"},
	}
	snap := snapshotWith(users, map[string]map[roleKey]*models.EffectiveRole{"u1": {}, "u2": {}}, nil)

	items, err := listUsersFromSnapshot(snap, "nia", attentionIndex{
		expiring: map[string]int{}, soonest: map[string]time.Time{},
		requests: map[string]int{}, unexplained: map[string]int{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].User.ID != "u1" {
		t.Fatalf("name search returned %d rows", len(items))
	}
}

// A-19 P10.5: "Sent, not seen back" on Home opens People filtered, so each row
// has to say how many of ITS writes are unseen, and only its own.
func TestLoadAttention_CountsUnseenWritesPerPerson(t *testing.T) {
	origE, origR, origD, origU := svcGetExpiringDirectGrants, svcGetAccessRequests, svcGetPendingDriftItems, svcGetUnconfirmedWrites
	t.Cleanup(func() {
		svcGetExpiringDirectGrants, svcGetAccessRequests, svcGetPendingDriftItems, svcGetUnconfirmedWrites = origE, origR, origD, origU
	})
	svcGetExpiringDirectGrants = func(context.Context, time.Duration) ([]models.DirectGrant, error) { return nil, nil }
	svcGetAccessRequests = func(context.Context, string) ([]models.AccessRequest, error) { return nil, nil }
	svcGetPendingDriftItems = func(context.Context) ([]models.DriftItem, error) { return nil, nil }
	svcGetUnconfirmedWrites = func(context.Context) (models.UnconfirmedWriteSummary, error) {
		return models.UnconfirmedWriteSummary{Count: 3, Users: map[string]int{"u_kevin": 2, "u_amara": 1}}, nil
	}

	idx, err := loadAttention(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if idx.unseen["u_kevin"] != 2 || idx.unseen["u_amara"] != 1 || idx.unseen["u_nobody"] != 0 {
		t.Errorf("unseen = %v", idx.unseen)
	}

	// The read failing is one filter's worth of zeros, never an error on the People page.
	svcGetUnconfirmedWrites = func(context.Context) (models.UnconfirmedWriteSummary, error) {
		return models.UnconfirmedWriteSummary{}, context.DeadlineExceeded
	}
	idx, err = loadAttention(context.Background())
	if err != nil || len(idx.unseen) != 0 {
		t.Errorf("a failed read must leave the index usable: err=%v unseen=%v", err, idx.unseen)
	}
}
