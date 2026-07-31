package services

import (
	"context"
	"testing"
	"time"

	"mkauth/internal/db"
	"mkauth/internal/models"
)

func withTriageDeps(t *testing.T, items []models.DriftItem, users map[string]models.UserProfile) {
	t.Helper()
	origDrift := svcGetPendingDriftItems
	origLocal := svcDbGetAllLocalRoles
	origUsage := svcDbGetRoleUsageCounts
	origAssigned := svcDbGetAssignedUserCounts
	origRefs := svcDbGetAllReferencedRoleKeys
	origFind := directoryFindUser
	t.Cleanup(func() {
		svcGetPendingDriftItems = origDrift
		svcDbGetAllLocalRoles = origLocal
		svcDbGetRoleUsageCounts = origUsage
		svcDbGetAssignedUserCounts = origAssigned
		svcDbGetAllReferencedRoleKeys = origRefs
		directoryFindUser = origFind
	})

	svcGetPendingDriftItems = func(context.Context) ([]models.DriftItem, error) { return items, nil }
	svcDbGetAllLocalRoles = func(context.Context) ([]models.Role, error) {
		return []models.Role{
			{ProjectID: "p_laser", RoleKey: "operator", DisplayName: "Bay operator", Group: "Safety-gated"},
			{ProjectID: "p_wiki", RoleKey: "read", DisplayName: "Wiki reader", Group: "Open bench"},
		}, nil
	}
	svcDbGetRoleUsageCounts = func(context.Context) (map[string]db.RoleUsage, error) {
		return map[string]db.RoleUsage{}, nil
	}
	svcDbGetAssignedUserCounts = func(context.Context) (map[string]int, error) { return map[string]int{}, nil }
	svcDbGetAllReferencedRoleKeys = func(context.Context) ([][2]string, error) { return nil, nil }
	directoryFindUser = func(_ context.Context, id string) (models.UserProfile, bool, error) {
		u, ok := users[id]
		return u, ok, nil
	}
}

// A safety-gated role found yesterday must outrank a wiki role found last week.
// Age alone is the wrong order for a queue where one row is a laser cutter.
func TestDriftTriageQueue_OrdersByRiskThenAge(t *testing.T) {
	old := time.Now().Add(-7 * 24 * time.Hour)
	recent := time.Now().Add(-1 * 24 * time.Hour)
	withTriageDeps(t, []models.DriftItem{
		{ID: "wiki", UserID: "u1", ProjectID: "p_wiki", RoleKeys: []string{"read"}, DetectedAt: old},
		{ID: "laser", UserID: "u2", ProjectID: "p_laser", RoleKeys: []string{"operator"}, DetectedAt: recent},
	}, nil)

	got, err := DriftTriageQueue(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 rows, got %d", len(got))
	}
	if got[0].ID != "laser" {
		t.Fatalf("safety-gated row must sort first, got %q", got[0].ID)
	}
	if got[0].RoleGroup != "Safety-gated" {
		t.Fatalf("risk pill needs the role group, got %q", got[0].RoleGroup)
	}
}

// Within the same risk tier, the oldest item comes first: it has been
// unexplained longest.
func TestDriftTriageQueue_OldestFirstWithinTier(t *testing.T) {
	older := time.Now().Add(-9 * 24 * time.Hour)
	newer := time.Now().Add(-2 * 24 * time.Hour)
	withTriageDeps(t, []models.DriftItem{
		{ID: "newer", UserID: "u1", ProjectID: "p_wiki", RoleKeys: []string{"read"}, DetectedAt: newer},
		{ID: "older", UserID: "u2", ProjectID: "p_wiki", RoleKeys: []string{"read"}, DetectedAt: older},
	}, nil)

	got, err := DriftTriageQueue(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got[0].ID != "older" {
		t.Fatalf("oldest must lead within a tier, got %q", got[0].ID)
	}
}

// A role MkAuth no longer knows about outranks routine drift: adopting it would
// recreate something somebody deliberately retired.
func TestDriftTriageQueue_UncataloguedRoleOutranksKnownRoutineRole(t *testing.T) {
	same := time.Now().Add(-3 * 24 * time.Hour)
	withTriageDeps(t, []models.DriftItem{
		{ID: "known", UserID: "u1", ProjectID: "p_wiki", RoleKeys: []string{"read"}, DetectedAt: same},
		{ID: "retired", UserID: "u2", ProjectID: "p_wiki", RoleKeys: []string{"legacy-op"}, DetectedAt: same},
	}, nil)

	got, err := DriftTriageQueue(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got[0].ID != "retired" {
		t.Fatalf("uncatalogued role must outrank a known routine one, got %q", got[0].ID)
	}
	if got[0].RoleInCatalogue {
		t.Error("legacy-op is not in the catalogue and must not be reported as such")
	}
}

// "2 more items for this person" is the context that changes a revoke decision,
// and it counts OTHER items — never the row you are looking at.
func TestDriftTriageQueue_CountsOtherItemsForTheSamePerson(t *testing.T) {
	now := time.Now()
	withTriageDeps(t, []models.DriftItem{
		{ID: "a", UserID: "marta", ProjectID: "p_wiki", RoleKeys: []string{"read"}, DetectedAt: now},
		{ID: "b", UserID: "marta", ProjectID: "p_wiki", RoleKeys: []string{"read2"}, DetectedAt: now},
		{ID: "c", UserID: "marta", ProjectID: "p_wiki", RoleKeys: []string{"read3"}, DetectedAt: now},
		{ID: "d", UserID: "solo", ProjectID: "p_wiki", RoleKeys: []string{"read"}, DetectedAt: now},
	}, nil)

	got, err := DriftTriageQueue(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range got {
		want := 2
		if item.UserID == "solo" {
			want = 0
		}
		if item.OtherItemsForUser != want {
			t.Errorf("%s: other items = %d, want %d", item.ID, item.OtherItemsForUser, want)
		}
	}
}

// A machine account is not a person: "adopt" is the wrong verb for something an
// integration re-creates on every deploy, so the row has to say what it is.
func TestDriftTriageQueue_MarksServiceAccounts(t *testing.T) {
	withTriageDeps(t,
		[]models.DriftItem{{ID: "x", UserID: "svc1", ProjectID: "p_wiki", RoleKeys: []string{"read"}, DetectedAt: time.Now()}},
		map[string]models.UserProfile{
			"svc1": {ID: "svc1", Name: "svc-bookings", Email: "svc-bookings@makerspace.local", Status: "active"},
		})

	got, err := DriftTriageQueue(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !got[0].UserIsServiceAccount {
		t.Error("svc-bookings must be recognised as a service account")
	}
}

// An empty queue returns an empty slice, not nil: "everything is explained" is
// an answer, and the caller must not have to distinguish it from a failure.
func TestDriftTriageQueue_EmptyReturnsEmptySlice(t *testing.T) {
	withTriageDeps(t, nil, nil)
	got, err := DriftTriageQueue(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("want empty slice, got %#v", got)
	}
}
