package handlers

import (
	"context"
	"errors"
	"testing"

	"syndra/internal/db"
)

// The mapping and its cohort are read again under the access lock. The
// handler's read came before it, so a concurrent edit could move the value
// under a review of the old one, or give a mapping holders after it was judged
// to need no citation.
func TestAMappingChangeRechecksTheMappingAndCohortUnderTheLock(t *testing.T) {
	h := stubMappingDeps(t, nil)
	stubResolvedIntent(t)
	seen := db.RoleMapping{ID: "m1", Target: "truenas", ProjectID: "pLab", RoleKey: "maker", Field: "group", Value: "lab_makers"}
	get := dbGetRoleMapping
	t.Cleanup(func() { dbGetRoleMapping = get })
	current := seen
	dbGetRoleMapping = func(context.Context, string) (db.RoleMapping, error) { return current, nil }
	apply := func(plan string) error {
		_, err := applyMappingChange(context.Background(), seen, planSurfaceMappingEdit, "op", plan, "archive")
		return err
	}

	current.Value = "edited_elsewhere"
	if err := apply(""); !errors.Is(err, db.ErrPlanRequestMismatch) {
		t.Fatalf("a value that moved after it was read is a changed request, got %v", err)
	}
	current = seen

	h.holders = []string{"u1"}
	if err := apply(""); !errors.Is(err, errPlanCitationMissing) {
		t.Fatalf("holders that appeared since the handler's read need a citation, got %v", err)
	}
	if len(h.updated) != 0 || len(h.claimed) != 0 {
		t.Fatalf("a refused change must write and spend nothing: %v %v", h.updated, h.claimed)
	}

	if err := apply("plan_1"); err != nil || len(h.updated) != 1 {
		t.Fatalf("an unchanged mapping with its citation applies: %v %v", err, h.updated)
	}
}
