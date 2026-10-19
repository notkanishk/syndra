package services

import (
	"context"
	"errors"
	"testing"

	"syndra/internal/db"
	"syndra/internal/models"
)

// includeStubs: b1's working copy includes "safety" pinned to v-s3, whose
// roles are childRoles; the published v2 included what publishedIncludes says.
func includeStubs(t *testing.T, childRoles []models.BundleRole, working, publishedIncludes []models.BundleInclude) {
	t.Helper()
	origW, origV, origR, origS, origL, origSet := svcGetWorkingIncludes, svcGetVersionIncludes, svcGetRolesForVersion,
		svcSnapshotVersionIncludes, svcListIncluders, svcSetWorkingInclude
	t.Cleanup(func() {
		svcGetWorkingIncludes, svcGetVersionIncludes, svcGetRolesForVersion = origW, origV, origR
		svcSnapshotVersionIncludes, svcListIncluders, svcSetWorkingInclude = origS, origL, origSet
	})
	published := svcGetRolesForVersion
	svcGetWorkingIncludes = func(context.Context, string) ([]models.BundleInclude, error) { return working, nil }
	svcGetVersionIncludes = func(context.Context, string) ([]models.BundleInclude, error) { return publishedIncludes, nil }
	svcGetRolesForVersion = func(ctx context.Context, id string) ([]models.BundleRole, error) {
		if id == "v-s3" {
			return childRoles, nil
		}
		return published(ctx, id)
	}
}

var safetyV3 = models.BundleInclude{ChildBundleID: "safety", ChildName: "Safety basics", ChildVersionID: "v-s3", ChildVersion: 3}

// Including a bundle adds what it gives; a role held both ways is held once.
func TestBundleDraft_IncludesCountAsWhatTheNextVersionGives(t *testing.T) {
	resetCascadeDeps(t)
	publishStubs(t,
		[]models.BundleRole{{ProjectID: "p", RoleKey: "door"}},
		[]models.BundleRole{{ProjectID: "p", RoleKey: "door"}},
		nil)
	includeStubs(t,
		[]models.BundleRole{{ProjectID: "p", RoleKey: "door"}, {ProjectID: "p", RoleKey: "lockers"}},
		[]models.BundleInclude{safetyV3}, nil)

	draft, err := BundleDraft(context.Background(), "b1")
	if err != nil {
		t.Fatal(err)
	}
	if len(draft.Added) != 1 || draft.Added[0].RoleKey != "lockers" || len(draft.Removed) != 0 {
		t.Fatalf("only the role it didn't already give is new: +%v -%v", draft.Added, draft.Removed)
	}
	if len(draft.IncludeChanges) != 1 || draft.IncludeChanges[0].To != 3 || draft.IncludeChanges[0].From != 0 {
		t.Fatalf("the include is a change: %+v", draft.IncludeChanges)
	}
	if draft.Empty() {
		t.Fatal("a draft with a new include has something to publish")
	}
}

// Moving an include to a newer version is a change even if its roles match.
func TestBundleDraft_MovingAnIncludeIsAChange(t *testing.T) {
	resetCascadeDeps(t)
	publishStubs(t, nil, []models.BundleRole{{ProjectID: "p", RoleKey: "door", ViaBundleID: "safety"}}, nil)
	older := safetyV3
	older.ChildVersionID, older.ChildVersion = "v-s2", 2
	includeStubs(t, []models.BundleRole{{ProjectID: "p", RoleKey: "door"}}, []models.BundleInclude{safetyV3}, []models.BundleInclude{older})

	draft, err := BundleDraft(context.Background(), "b1")
	if err != nil {
		t.Fatal(err)
	}
	if draft.Empty() || len(draft.IncludeChanges) != 1 || draft.IncludeChanges[0].From != 2 || draft.IncludeChanges[0].To != 3 {
		t.Fatalf("v2 → v3 of the include: %+v", draft.IncludeChanges)
	}
}

// A child's publish needs an answer for every bundle that includes it, and
// "update" only writes that bundle's working copy.
func TestPublish_AsksEveryIncludingBundleAndUpdatesOnlyDrafts(t *testing.T) {
	resetCascadeDeps(t)
	publishStubs(t, []models.BundleRole{{ProjectID: "p", RoleKey: "x"}}, nil, nil)
	includeStubs(t, nil, nil, nil)
	svcGetActiveMappingRules = noRules
	svcListIncluders = func(context.Context, string) ([]db.Includer, error) {
		return []db.Includer{
			{BundleID: "new-members", Name: "New members", IncludedVersion: 2},
			{BundleID: "staff", Name: "Staff", IncludedVersion: 2},
		}, nil
	}
	svcPublishVersionAndEnqueue = func(ctx context.Context, actor, bundleID, note string, roles []models.BundleRole, moved []string, ps []db.EnqueueParams) (models.BundleVersion, []string, error) {
		return models.BundleVersion{ID: "v-new", Version: 3}, nil, nil
	}
	var updated []string
	svcSetWorkingInclude = func(_ context.Context, _, parent, child string, version int) (models.BundleInclude, error) {
		updated = append(updated, parent)
		if child != "b1" || version != 3 {
			t.Fatalf("update pins the new version of this bundle, got %s v%d", child, version)
		}
		return models.BundleInclude{}, nil
	}

	_, _, err := PublishBundleVersion(context.Background(), "admin", PublishRequest{BundleID: "b1",
		Includers: []IncluderChoice{{BundleID: "new-members", Action: "update"}}})
	var unanswered *ErrIncludersUnanswered
	if !errors.As(err, &unanswered) || len(unanswered.Names) != 1 || unanswered.Names[0] != "Staff" {
		t.Fatalf("Staff was not answered: %v", err)
	}

	_, _, err = PublishBundleVersion(context.Background(), "admin", PublishRequest{BundleID: "b1",
		Includers: []IncluderChoice{{BundleID: "new-members", Action: "update"}, {BundleID: "staff", Action: "leave"}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(updated) != 1 || updated[0] != "new-members" {
		t.Fatalf("only the bundle told to update takes the new version: %v", updated)
	}
}

// Retiring a bundle that others include is refused, naming them.
func TestCascadeBundleRetired_RefusedWhileIncluded(t *testing.T) {
	resetCascadeDeps(t)
	publishStubs(t, nil, nil, nil)
	orig := svcRetireBlockers
	t.Cleanup(func() { svcRetireBlockers = orig })
	svcRetireBlockers = func(context.Context, string) ([]string, error) { return []string{"New members", "Staff"}, nil }

	_, err := CascadeBundleRetired(context.Background(), "admin", "safety")
	var included *ErrBundleIncluded
	if !errors.As(err, &included) || len(included.Includers) != 2 || !errors.Is(err, db.ErrBundleIncluded) {
		t.Fatalf("want refusal naming both, got %v", err)
	}
}

// The plan an operator reads gives what the apply writes: a holder gains the
// roles an include brings, in the rehearsal as in the publish.
func TestRehearsePublish_CountsWhatAnIncludeBrings(t *testing.T) {
	resetCascadeDeps(t)
	publishStubs(t, nil, nil, []models.BundleHolder{{UserID: "u1", VersionID: "v-old", Version: 2}})
	includeStubs(t, []models.BundleRole{{ProjectID: "p", RoleKey: "lockers"}}, []models.BundleInclude{safetyV3}, nil)
	svcGetDirectGrantsForUser = noDirects
	svcGetUserBundleRolesGrouped = noBundleRoles
	svcGetActiveMappingRules = noRules

	plan, _, err := RehearseBundlePublish(context.Background(), PublishRequest{BundleID: "b1", Migrate: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Outcomes) != 1 || plan.Outcomes[0].Effect != EffectApply {
		t.Fatalf("the holder gains what the include brings: %+v", plan.Outcomes)
	}
}

// An approval is for one version: moving an include to another version of
// the included bundle makes it a different version, even with the same roles.
func TestPublishFingerprint_CoversIncludes(t *testing.T) {
	roles := []models.BundleRole{{ProjectID: "p", RoleKey: "door"}}
	v2 := safetyV3
	v2.ChildVersionID = "v-s2"
	a := FingerprintBundlePublish("b1", true, publishedContents(DraftDiff{WorkingEffective: roles, Includes: []models.BundleInclude{safetyV3}}), nil)
	b := FingerprintBundlePublish("b1", true, publishedContents(DraftDiff{WorkingEffective: roles, Includes: []models.BundleInclude{v2}}), nil)
	if a == b {
		t.Fatal("a different included version must not carry the same approval")
	}
}
