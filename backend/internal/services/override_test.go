package services

import (
	"context"
	"testing"
	"time"

	"syndra/internal/db"
)

// An operator's per-person value replaces what the roles give: one member's
// larger home folder limit, over the role's.
func TestAPerPersonValueReplacesTheRoles(t *testing.T) {
	f := &resolverFixture{
		roles: []db.RoleRef{{ProjectID: "pNet", RoleKey: "community"}},
		mappings: []db.RoleMapping{
			mapping("community", "group", "Community"),
			mapping("community", "home_quota", "10737418240"),
		},
		allowances: []db.Allowance{
			{ID: "a1", Field: "home_quota", Value: "53687091200", Direction: db.AllowanceSet, CreatedAt: time.Now()},
		},
	}
	f.install(t)
	set, err := ResolveEntitlements(context.Background(), "u1", "truenas")
	if err != nil {
		t.Fatal(err)
	}
	if got := set.Fields["home_quota"]; len(got) != 1 || got[0] != "53687091200" {
		t.Fatalf("the person's own limit replaces the role's, got %v", got)
	}
	if got := set.Fields["group"]; len(got) != 1 || got[0] != "Community" {
		t.Fatalf("other fields are untouched, got %v", got)
	}
}

// Not for someone the target does not reach: an exception for a person with no
// mapped role is a setting for an account that should not exist.
func TestAPerPersonValueNeedsAReason(t *testing.T) {
	f := &resolverFixture{
		allowances: []db.Allowance{
			{ID: "a1", Field: "home_quota", Value: "53687091200", Direction: db.AllowanceSet, CreatedAt: time.Now()},
		},
	}
	f.install(t)
	set, err := ResolveEntitlements(context.Background(), "u1", "truenas")
	if err != nil {
		t.Fatal(err)
	}
	if _, set := set.Fields["home_quota"]; set {
		t.Fatal("a person the target does not reach gets no per-person value")
	}
	if set.Lifecycle.Enabled {
		t.Fatal("and is not enabled by it")
	}
}
