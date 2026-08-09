package services

import (
	"context"

	"syndra/internal/directory"
	"testing"

	"syndra/internal/models"
)

// diffRoles' result is marshalled straight to the console on four routes, and a Go nil slice
// becomes JSON `null` there.
//
// This is a regression guard for a production outage: Review › Bundles rendered the error boundary
// for every bundle whose working copy matched its published version — the normal resting state —
// because the console called `.length` on `added`. Nothing caught it here, because the empty case
// was never asserted, and nothing caught it in the console either, because the two tests that
// touch the count both mock the function that reads these fields.
func TestDiffRoles_NeverReturnsNilSlices(t *testing.T) {
	role := models.BundleRole{ProjectID: "pLaser", RoleKey: "trained"}

	cases := []struct {
		name       string
		prev, next []models.BundleRole
	}{
		{"both empty", nil, nil},
		// The resting state: published and working agree, so there is nothing to add or drop.
		{"identical", []models.BundleRole{role}, []models.BundleRole{role}},
		// An unpublished bundle: everything is an addition, nothing is a removal. `removed` was
		// nil here too, so even a brand-new bundle took the page down.
		{"only additions", nil, []models.BundleRole{role}},
		{"only removals", []models.BundleRole{role}, nil},
	}

	for _, tc := range cases {
		added, removed := diffRoles(tc.prev, tc.next)
		if added == nil {
			t.Errorf("%s: added must be an empty slice, not nil — nil marshals to JSON null", tc.name)
		}
		if removed == nil {
			t.Errorf("%s: removed must be an empty slice, not nil — nil marshals to JSON null", tc.name)
		}
	}
}

// And it still has to report the actual difference.
func TestDiffRoles_ReportsTheDifference(t *testing.T) {
	kept := models.BundleRole{ProjectID: "pLaser", RoleKey: "trained"}
	gone := models.BundleRole{ProjectID: "pWood", RoleKey: "member"}
	fresh := models.BundleRole{ProjectID: "pStudio", RoleKey: "door"}

	added, removed := diffRoles(
		[]models.BundleRole{kept, gone},
		[]models.BundleRole{kept, fresh},
	)

	if len(added) != 1 || added[0] != fresh {
		t.Errorf("expected only the new role added, got %v", added)
	}
	if len(removed) != 1 || removed[0] != gone {
		t.Errorf("expected only the dropped role removed, got %v", removed)
	}
}

// Names are the whole point of decoration: a rehearsal that lists subject ids
// is a list nobody can review. Asserted behaviourally, because the guard in
// internal/db can only see that the lookup is written in this function — not
// that its result reaches the row.
func TestDecoratePlan_FillsNamesFromTheDirectory(t *testing.T) {
	orig := directory.Default
	t.Cleanup(func() { directory.Default = orig })
	directory.Default = directory.NewDemoSource()

	demo, err := directory.Default.Users(context.Background())
	if err != nil || len(demo) == 0 {
		t.Skipf("no demo directory to decorate from: %v", err)
	}
	known := demo[0]

	plan := BulkPlan{Outcomes: []BulkOutcome{
		{UserID: known.ID},
		{UserID: "nobody-by-that-name"},
	}}
	DecoratePlan(context.Background(), &plan)

	if plan.Outcomes[0].Name != known.Name || plan.Outcomes[0].Email != known.Email {
		t.Fatalf("a known subject must be named, got %+v", plan.Outcomes[0])
	}
	// A lookup that finds nothing leaves the row identified by the id it
	// already carried, rather than blanking it or failing the rehearsal.
	if plan.Outcomes[1].UserID != "nobody-by-that-name" || plan.Outcomes[1].Name != "" {
		t.Fatalf("an unknown subject must keep its id and gain no name, got %+v", plan.Outcomes[1])
	}
}
