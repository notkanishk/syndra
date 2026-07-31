package services

import (
	"context"
	"testing"
	"time"

	"mkauth/internal/models"
)

// snapshotWith builds an accessSnapshot without a directory or database: the
// People index is pure aggregation over role resolution, and the interesting
// behaviour is in what it aggregates, not where it came from.
func snapshotWith(users []models.UserProfile, roles map[string]map[roleKey]*models.EffectiveRole,
	bundles map[string][]models.Bundle) *accessSnapshot {
	snap := &accessSnapshot{ctx: context.Background(), users: users, roles: map[string]userRoles{}}
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
		{ID: "u_tomas", Name: "Tomas Beck"},
		{ID: "u_amara", Name: "Amara Osei"},
	}
	snap := snapshotWith(users,
		map[string]map[roleKey]*models.EffectiveRole{
			"u_tomas": {k1: r1},
			"u_amara": {},
		},
		map[string][]models.Bundle{
			"u_tomas": {{ID: "b1", Name: "Lab Tech"}, {ID: "b2", Name: "Studio Member"}},
		})

	soon := time.Now().Add(48 * time.Hour)
	attention := attentionIndex{
		expiring:    map[string]int{"u_tomas": 1},
		soonest:     map[string]time.Time{"u_tomas": soon},
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

	tomas := byID["u_tomas"]
	if tomas.ExpiringCount != 1 || tomas.SoonestExpiry == nil {
		t.Errorf("Tomas has an expiring grant; got count=%d soonest=%v", tomas.ExpiringCount, tomas.SoonestExpiry)
	}
	if tomas.OpenRequestCount != 0 {
		t.Errorf("Amara's request must not land on Tomas's row, got %d", tomas.OpenRequestCount)
	}
	if len(tomas.BundleNames) != 2 || tomas.BundleNames[0] != "Lab Tech" {
		t.Errorf("bundle names are pills, not a count: got %v", tomas.BundleNames)
	}
	if tomas.ProjectCount != 1 {
		t.Errorf("project count = %d, want 1", tomas.ProjectCount)
	}

	amara := byID["u_amara"]
	if amara.OpenRequestCount != 1 {
		t.Errorf("Amara has one open request, got %d", amara.OpenRequestCount)
	}
	if amara.ExpiringCount != 0 || amara.SoonestExpiry != nil {
		t.Error("Amara has nothing expiring; the row must say so with a dash, not borrow Tomas's date")
	}
}

// Search matches role keys as well as names and emails: "who has trained in the
// laser lab" is typed here before anyone thinks to go to Roles.
func TestPeopleIndex_SearchMatchesRoleKeys(t *testing.T) {
	k1, r1 := role("p_laser", "trained", "Laser Lab")
	users := []models.UserProfile{
		{ID: "u_holder", Name: "Sofia Marchetti", Email: "sofia@example.org"},
		{ID: "u_other", Name: "Wen-Li Chao", Email: "wenli@example.org"},
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
		{ID: "u1", Name: "Sofia Marchetti", Email: "sofia@example.org"},
		{ID: "u2", Name: "Wen-Li Chao", Email: "wenli@example.org"},
	}
	snap := snapshotWith(users, map[string]map[roleKey]*models.EffectiveRole{"u1": {}, "u2": {}}, nil)

	items, err := listUsersFromSnapshot(snap, "sofia", attentionIndex{
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
