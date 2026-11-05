package cache

import (
	"context"
	"testing"
	"time"

	"syndra/internal/models"
)

// A role a rule gives ends when its source ends (W2a). The compiler used to
// mark every rule target as never ending, so the token said a rule role
// outlived the grant it came from. Down a chain, from the later of two
// sources, and never when a source never ends.
func TestCompileFacts_ARuleRoleEndsWhenItsSourceEnds(t *testing.T) {
	resetCacheDeps(t)
	origFind := cacheFindUser
	t.Cleanup(func() { cacheFindUser = origFind })
	cacheFindUser = func(context.Context, string) (models.UserProfile, bool, error) {
		return models.UserProfile{}, false, nil
	}

	june := time.Date(2027, 6, 30, 12, 0, 0, 0, time.UTC)
	dec := time.Date(2027, 12, 31, 12, 0, 0, 0, time.UTC)
	dbGetDirectGrantsForUser = func(context.Context, string, bool) ([]models.DirectGrant, error) {
		return []models.DirectGrant{
			{ProjectID: "net", RoleKey: "trained", ExpiresAt: &june},
			{ProjectID: "net", RoleKey: "senior", ExpiresAt: &dec},
			{ProjectID: "net", RoleKey: "admin"},
		}, nil
	}
	dbGetActiveMappingRules = func(context.Context) ([]models.MappingRule, error) {
		return []models.MappingRule{
			{SourceProject: "net", SourceRole: "trained", TargetProject: "net", TargetRole: "laser"},
			{SourceProject: "net", SourceRole: "laser", TargetProject: "net", TargetRole: "cutter"},
			{SourceProject: "net", SourceRole: "senior", TargetProject: "net", TargetRole: "keys"},
			{SourceProject: "net", SourceRole: "trained", TargetProject: "net", TargetRole: "keys"},
			{SourceProject: "net", SourceRole: "admin", TargetProject: "net", TargetRole: "door"},
		}, nil
	}

	f, err := CompileFacts(context.Background(), "u1", "net")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"trained": "2027-06-30", "laser": "2027-06-30", "cutter": "2027-06-30", // down the chain
		"senior": "2027-12-31", "keys": "2027-12-31", // the later of two sources
		// admin and door: no end
	}
	if len(f.RoleEnds) != len(want) {
		t.Fatalf("role ends = %v, want %v", f.RoleEnds, want)
	}
	for role, day := range want {
		if f.RoleEnds[role] != day {
			t.Errorf("%s ends %q, want %q (all: %v)", role, f.RoleEnds[role], day, f.RoleEnds)
		}
	}
	if len(f.Roles) != 7 {
		t.Errorf("roles = %v, want all seven", f.Roles)
	}
}
