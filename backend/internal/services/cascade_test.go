package services

import (
	"context"
	"errors"
	"strings"
	"testing"

	"mkauth/internal/db"
	"mkauth/internal/models"
	"mkauth/internal/services/propagation"
)

// resetCascadeDeps captures and restores every cascade injectable, mirroring
// resetOnboardingDeps's t.Cleanup idiom.
func resetCascadeDeps(t *testing.T) {
	t.Helper()
	origGetBundle := svcGetBundleByID
	origGetRoles := svcCascGetRolesForBundle
	origGetUsersForBundle := svcGetUsersForBundle
	origGetAllKnownUserIDs := svcGetAllKnownUserIDs
	origGetRule := svcGetMappingRuleByID
	origDrainBatch := svcDrainBatch
	origAssignAndEnqueue := svcAssignBundleAndEnqueue
	origAddRoleAndEnqueue := svcAddRoleToBundleAndEnqueue
	origCreateRuleAndEnqueue := svcCreateRuleAndEnqueue
	// Closure-diff helpers' own injectables — shared with the governance layer (services/deps.go),
	// so cascade tests must snapshot/restore them too.
	origGetDirectGrants := svcGetDirectGrantsForUser
	origGetBundlesForUser := svcGetBundlesForUser
	origUserBundleRoles := svcGetUserBundleRolesGrouped
	origLatestVersion := svcLatestVersion
	origRolesForVersion := svcGetRolesForVersion
	origHoldersByVersion := svcGetBundleHoldersByVersion
	origListVersions := svcListBundleVersions
	origBelongsTo := svcVersionBelongsTo
	origLatestVersionRoles := svcLatestVersionRoles
	origPublishAndEnqueue := svcPublishVersionAndEnqueue
	origMoveHolders := svcMoveHoldersAndEnqueue
	origGetActiveRules := svcGetActiveMappingRules
	// Revoke-side + rule-update atomic mutation+enqueue (Task 21).
	origRemoveBundleAndEnqueue := svcRemoveBundleFromUserAndEnqueue
	origRemoveRoleAndEnqueue := svcRemoveRoleFromBundleAndEnqueue
	origUpdateRuleAndEnqueue := svcUpdateRuleAndEnqueue
	t.Cleanup(func() {
		svcGetBundleByID = origGetBundle
		svcCascGetRolesForBundle = origGetRoles
		svcGetUsersForBundle = origGetUsersForBundle
		svcGetAllKnownUserIDs = origGetAllKnownUserIDs
		svcGetMappingRuleByID = origGetRule
		svcDrainBatch = origDrainBatch
		svcAssignBundleAndEnqueue = origAssignAndEnqueue
		svcAddRoleToBundleAndEnqueue = origAddRoleAndEnqueue
		svcCreateRuleAndEnqueue = origCreateRuleAndEnqueue
		svcGetDirectGrantsForUser = origGetDirectGrants
		svcGetBundlesForUser = origGetBundlesForUser
		svcGetUserBundleRolesGrouped = origUserBundleRoles
		svcLatestVersion = origLatestVersion
		svcGetRolesForVersion = origRolesForVersion
		svcGetBundleHoldersByVersion = origHoldersByVersion
		svcListBundleVersions = origListVersions
		svcVersionBelongsTo = origBelongsTo
		svcLatestVersionRoles = origLatestVersionRoles
		svcPublishVersionAndEnqueue = origPublishAndEnqueue
		svcMoveHoldersAndEnqueue = origMoveHolders
		svcGetActiveMappingRules = origGetActiveRules
		svcRemoveBundleFromUserAndEnqueue = origRemoveBundleAndEnqueue
		svcRemoveRoleFromBundleAndEnqueue = origRemoveRoleAndEnqueue
		svcUpdateRuleAndEnqueue = origUpdateRuleAndEnqueue
	})

	// Same default as the governance harness: closures resolve bundle roles
	// through each person's pinned version, so the unstubbed path must not be
	// the real database.
	svcGetUserBundleRolesGrouped = func(context.Context, string) (map[string][]models.BundleRole, error) {
		return nil, nil
	}
}

// noBundles/noDirects/noRules are the common "holds nothing else" stubs used by most closure-diff
// tests below, to keep each test's arrange section focused on what it actually varies.
func noDirects(ctx context.Context, u string, inc bool) ([]models.DirectGrant, error) {
	return nil, nil
}
func noBundles(ctx context.Context, u string) ([]models.Bundle, error) { return nil, nil }

// noBundleRoles is the version-aware "holds nothing via any bundle" stub. It
// replaces noBundles in every closure test: closures resolve a person's bundle
// roles through the version they are pinned to, so stubbing the bundle LIST no
// longer stubs what they hold.
func noBundleRoles(ctx context.Context, u string) (map[string][]models.BundleRole, error) {
	return nil, nil
}
func noRules(ctx context.Context) ([]models.MappingRule, error) { return nil, nil }

// --- CascadeBundleAssignedToUser ---

func TestCascadeBundleAssignedToUser_AutoEnqueuesPerRoleAndDrains(t *testing.T) {
	resetCascadeDeps(t)
	svcGetBundleByID = func(ctx context.Context, id string) (models.Bundle, error) {
		return models.Bundle{ID: id, ConfirmationMode: "auto"}, nil
	}
	svcLatestVersionRoles = func(ctx context.Context, id string) (models.BundleVersion, []models.BundleRole, error) {
		return models.BundleVersion{ID: "v-latest", Version: 2}, []models.BundleRole{
			{ProjectID: "p1", RoleKey: "r1"}, {ProjectID: "p1", RoleKey: "r2"},
		}, nil
	}
	svcGetDirectGrantsForUser = noDirects
	svcGetUserBundleRolesGrouped = noBundleRoles
	svcGetActiveMappingRules = noRules
	var enqueued []db.EnqueueParams
	svcAssignBundleAndEnqueue = func(ctx context.Context, actor, userID, bundleID, versionID string, ps []db.EnqueueParams) ([]string, bool, error) {
		enqueued = ps
		return []string{"o1", "o2"}, true, nil
	}
	var drainedIDs []string
	svcDrainBatch = func(ctx context.Context, ids []string) (propagation.DrainResult, error) {
		drainedIDs = ids
		return propagation.DrainResult{Applied: 2}, nil
	}

	res, err := CascadeBundleAssignedToUser(context.Background(), "admin", "u1", "b1")
	if err != nil {
		t.Fatal(err)
	}
	if len(enqueued) != 2 {
		t.Fatalf("enqueued %d params, want 2", len(enqueued))
	}
	for _, p := range enqueued {
		if p.Source != "bundle" || p.SourceRef != "b1" || p.OpType != "add" || p.UserID != "u1" {
			t.Fatalf("bad param: %+v", p)
		}
	}
	if strings.Join(drainedIDs, ",") != "o1,o2" {
		t.Fatalf("drained %v", drainedIDs)
	}
	if res.Mode != "auto" {
		t.Fatalf("mode = %q", res.Mode)
	}
	if res.Enqueued != 2 {
		t.Fatalf("enqueued count = %d, want 2", res.Enqueued)
	}
}

func TestCascadeBundleAssignedToUser_ManualQueuesWithoutDrain(t *testing.T) {
	resetCascadeDeps(t)
	svcGetBundleByID = func(ctx context.Context, id string) (models.Bundle, error) {
		return models.Bundle{ID: id, ConfirmationMode: "manual"}, nil
	}
	svcLatestVersionRoles = func(ctx context.Context, id string) (models.BundleVersion, []models.BundleRole, error) {
		return models.BundleVersion{ID: "v-latest", Version: 2}, []models.BundleRole{{ProjectID: "p1", RoleKey: "r1"}}, nil
	}
	svcGetDirectGrantsForUser = noDirects
	svcGetUserBundleRolesGrouped = noBundleRoles
	svcGetActiveMappingRules = noRules
	svcAssignBundleAndEnqueue = func(ctx context.Context, actor, userID, bundleID, versionID string, ps []db.EnqueueParams) ([]string, bool, error) {
		return []string{"o1"}, true, nil
	}
	drainCalled := false
	svcDrainBatch = func(ctx context.Context, ids []string) (propagation.DrainResult, error) {
		drainCalled = true
		return propagation.DrainResult{}, nil
	}
	res, err := CascadeBundleAssignedToUser(context.Background(), "admin", "u1", "b1")
	if err != nil {
		t.Fatal(err)
	}
	if drainCalled {
		t.Fatal("manual mode must not drain")
	}
	if res.Mode != "manual" {
		t.Fatalf("mode = %q", res.Mode)
	}
}

func TestCascadeBundleAssignedToUser_EnqueueErrorPropagates(t *testing.T) {
	resetCascadeDeps(t)
	svcGetBundleByID = func(ctx context.Context, id string) (models.Bundle, error) {
		return models.Bundle{ID: id, ConfirmationMode: "auto"}, nil
	}
	svcLatestVersionRoles = func(ctx context.Context, id string) (models.BundleVersion, []models.BundleRole, error) {
		return models.BundleVersion{ID: "v-latest", Version: 2}, []models.BundleRole{{ProjectID: "p1", RoleKey: "r1"}}, nil
	}
	svcGetDirectGrantsForUser = noDirects
	svcGetUserBundleRolesGrouped = noBundleRoles
	svcGetActiveMappingRules = noRules
	wantErr := errors.New("assign tx failed")
	svcAssignBundleAndEnqueue = func(ctx context.Context, actor, userID, bundleID, versionID string, ps []db.EnqueueParams) ([]string, bool, error) {
		return nil, false, wantErr
	}
	drainCalled := false
	svcDrainBatch = func(ctx context.Context, ids []string) (propagation.DrainResult, error) {
		drainCalled = true
		return propagation.DrainResult{}, nil
	}

	_, err := CascadeBundleAssignedToUser(context.Background(), "admin", "u1", "b1")
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected the enqueue error to propagate, got %v", err)
	}
	if drainCalled {
		t.Fatal("must not drain when the atomic assign+enqueue failed (nothing committed)")
	}
}

// TestCascadeBundleAssignedToUser_ClosureDiffIncludesRuleDerivedTarget is brief test 1 (P1a add):
// bundle B grants role A; an active rule A→B2 fires off it. Assigning B to a user who holds
// nothing else must enqueue BOTH A (the bundle's literal role) and B2 (the rule-derived target) —
// proving the cascade now projects the effective closure, not just the bundle's literal roles.
func TestCascadeBundleAssignedToUser_ClosureDiffIncludesRuleDerivedTarget(t *testing.T) {
	resetCascadeDeps(t)
	svcGetBundleByID = func(ctx context.Context, id string) (models.Bundle, error) {
		return models.Bundle{ID: id, ConfirmationMode: "auto"}, nil
	}
	svcLatestVersionRoles = func(ctx context.Context, id string) (models.BundleVersion, []models.BundleRole, error) {
		return models.BundleVersion{ID: "v-latest", Version: 2}, []models.BundleRole{{ProjectID: "p1", RoleKey: "A"}}, nil
	}
	svcGetDirectGrantsForUser = noDirects
	svcGetUserBundleRolesGrouped = noBundleRoles
	svcGetActiveMappingRules = func(ctx context.Context) ([]models.MappingRule, error) {
		return []models.MappingRule{{ID: "rule1", SourceProject: "p1", SourceRole: "A", TargetProject: "p1", TargetRole: "B2"}}, nil
	}
	var enqueued []db.EnqueueParams
	svcAssignBundleAndEnqueue = func(ctx context.Context, actor, userID, bundleID, versionID string, ps []db.EnqueueParams) ([]string, bool, error) {
		enqueued = ps
		return []string{"o1", "o2"}, true, nil
	}
	svcDrainBatch = func(ctx context.Context, ids []string) (propagation.DrainResult, error) {
		return propagation.DrainResult{}, nil
	}

	if _, err := CascadeBundleAssignedToUser(context.Background(), "admin", "u1", "b1"); err != nil {
		t.Fatal(err)
	}
	if len(enqueued) != 2 {
		t.Fatalf("enqueued %d params, want 2 (A + rule-derived B2): %+v", len(enqueued), enqueued)
	}
	got := map[string]bool{}
	for _, p := range enqueued {
		if p.Source != "bundle" || p.SourceRef != "b1" || p.OpType != "add" {
			t.Fatalf("bad param: %+v", p)
		}
		got[p.RoleKeys[0]] = true
	}
	if !got["A"] || !got["B2"] {
		t.Fatalf("expected adds for A and B2, got %+v", enqueued)
	}
}

// TestCascadeBundleAssignedToUser_IdempotentWhenAlreadyEffectivelyGranted is brief test 7: the user
// already effectively holds every role the bundle would grant (via another bundle) — the delta
// must be empty, but the atomic assign still happens (mutation is independent of projection).
func TestCascadeBundleAssignedToUser_IdempotentWhenAlreadyEffectivelyGranted(t *testing.T) {
	resetCascadeDeps(t)
	svcGetBundleByID = func(ctx context.Context, id string) (models.Bundle, error) {
		return models.Bundle{ID: id, ConfirmationMode: "auto"}, nil
	}
	svcLatestVersionRoles = func(ctx context.Context, id string) (models.BundleVersion, []models.BundleRole, error) {
		return models.BundleVersion{ID: "v-latest", Version: 2}, []models.BundleRole{{ProjectID: "p1", RoleKey: "r1"}}, nil
	}
	svcGetDirectGrantsForUser = func(ctx context.Context, u string, inc bool) ([]models.DirectGrant, error) {
		return []models.DirectGrant{{UserID: u, ProjectID: "p1", RoleKey: "r1"}}, nil // already holds r1 directly
	}
	svcGetUserBundleRolesGrouped = noBundleRoles
	svcGetActiveMappingRules = noRules
	var enqueued []db.EnqueueParams
	assignCalled := false
	svcAssignBundleAndEnqueue = func(ctx context.Context, actor, userID, bundleID, versionID string, ps []db.EnqueueParams) ([]string, bool, error) {
		assignCalled = true
		enqueued = ps
		return nil, true, nil
	}

	res, err := CascadeBundleAssignedToUser(context.Background(), "admin", "u1", "b1")
	if err != nil {
		t.Fatal(err)
	}
	if !assignCalled {
		t.Fatal("the atomic assign must still run even with an empty delta")
	}
	if len(enqueued) != 0 {
		t.Fatalf("expected 0 enqueued params, got %+v", enqueued)
	}
	if res.Enqueued != 0 {
		t.Fatalf("enqueued = %d, want 0", res.Enqueued)
	}
}

// --- EditBundleWorkingCopy ---

// The behaviour versioning changed: editing a bundle used to reach every holder
// the moment it saved. It must now reach nobody — the consequence belongs to
// publishing, which is rehearsed.

func TestEditBundleWorkingCopy_AddEnqueuesNothing(t *testing.T) {
	resetCascadeDeps(t)
	var enqueued []db.EnqueueParams
	called := false
	svcAddRoleToBundleAndEnqueue = func(ctx context.Context, actor, bundleID, projectID, roleKey string, ps []db.EnqueueParams) ([]string, error) {
		called = true
		enqueued = ps
		return nil, nil
	}
	drained := false
	svcDrainBatch = func(ctx context.Context, ids []string) (propagation.DrainResult, error) {
		drained = true
		return propagation.DrainResult{}, nil
	}

	if err := EditBundleWorkingCopy(context.Background(), "admin", "b1", "p1", "r1", true); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("the working copy was not written")
	}
	if len(enqueued) != 0 {
		t.Fatalf("a working-copy edit must enqueue nothing, got %d rows", len(enqueued))
	}
	if drained {
		t.Fatal("a working-copy edit must not drain")
	}
}

func TestEditBundleWorkingCopy_RemoveEnqueuesNothing(t *testing.T) {
	resetCascadeDeps(t)
	var enqueued []db.EnqueueParams
	svcRemoveRoleFromBundleAndEnqueue = func(ctx context.Context, actor, bundleID, projectID, roleKey string, ps []db.EnqueueParams) ([]string, error) {
		enqueued = ps
		return nil, nil
	}

	if err := EditBundleWorkingCopy(context.Background(), "admin", "b1", "p1", "r1", false); err != nil {
		t.Fatal(err)
	}
	if len(enqueued) != 0 {
		t.Fatalf("a working-copy edit must enqueue nothing, got %d rows", len(enqueued))
	}
}

// --- CascadeRuleCreated ---

// TestCascadeRuleCreated_DiscoversHolderAbsentFromGrantIndex is brief test 4 (P1b holder
// discovery): a user holds the new rule's source role via a bundle — present in MkAuth's own
// tables, but the test never stubs anything resembling the Zitadel grant index. Discovery must
// come from GetAllKnownUserIDs + userBaseHoldings (MkAuth-side), not a grant-index lookup, so the
// user still gets the rule's target.
func TestCascadeRuleCreated_DiscoversHolderAbsentFromGrantIndex(t *testing.T) {
	resetCascadeDeps(t)
	svcGetAllKnownUserIDs = func(ctx context.Context) ([]string, error) {
		return []string{"u1"}, nil
	}
	svcGetDirectGrantsForUser = noDirects
	svcGetBundlesForUser = func(ctx context.Context, u string) ([]models.Bundle, error) {
		return []models.Bundle{{ID: "b1"}}, nil
	}
	svcGetUserBundleRolesGrouped = func(ctx context.Context, u string) (map[string][]models.BundleRole, error) {
		return map[string][]models.BundleRole{"b1": {{ProjectID: "sp", RoleKey: "sr"}}}, nil
	}
	svcGetActiveMappingRules = noRules
	var gotParams []db.EnqueueParams
	svcCreateRuleAndEnqueue = func(ctx context.Context, actor, sp, sr, tp, tr, mode string, params []db.EnqueueParams) (string, []string, error) {
		gotParams = params
		return "rule-1", []string{"o1"}, nil
	}
	svcDrainBatch = func(ctx context.Context, ids []string) (propagation.DrainResult, error) {
		return propagation.DrainResult{}, nil
	}

	ruleID, res, err := CascadeRuleCreated(context.Background(), "admin", "sp", "sr", "tp", "tr", "auto")
	if err != nil {
		t.Fatal(err)
	}
	if ruleID != "rule-1" {
		t.Fatalf("ruleID = %q", ruleID)
	}
	if len(gotParams) != 1 || gotParams[0].OpType != "add" || gotParams[0].UserID != "u1" ||
		gotParams[0].ProjectID != "tp" || gotParams[0].RoleKeys[0] != "tr" || gotParams[0].Source != "rule" {
		t.Fatalf("bad param: %+v", gotParams)
	}
	if res.Mode != "auto" {
		t.Fatalf("mode = %q", res.Mode)
	}
}

func TestCascadeRuleCreated_SkipsUsersWithEmptyDelta(t *testing.T) {
	resetCascadeDeps(t)
	svcGetAllKnownUserIDs = func(ctx context.Context) ([]string, error) {
		return []string{"u1", "u2"}, nil // u1 holds the source, u2 holds nothing
	}
	svcGetDirectGrantsForUser = func(ctx context.Context, u string, inc bool) ([]models.DirectGrant, error) {
		if u == "u1" {
			return []models.DirectGrant{{UserID: u, ProjectID: "sp", RoleKey: "sr"}}, nil
		}
		return nil, nil
	}
	svcGetUserBundleRolesGrouped = noBundleRoles
	svcGetActiveMappingRules = noRules
	var gotParams []db.EnqueueParams
	svcCreateRuleAndEnqueue = func(ctx context.Context, actor, sp, sr, tp, tr, mode string, params []db.EnqueueParams) (string, []string, error) {
		gotParams = params
		return "rule-2", nil, nil
	}

	if _, _, err := CascadeRuleCreated(context.Background(), "admin", "sp", "sr", "tp", "tr", "manual"); err != nil {
		t.Fatal(err)
	}
	if len(gotParams) != 1 || gotParams[0].UserID != "u1" {
		t.Fatalf("expected exactly one param for u1 (u2 has empty delta), got %+v", gotParams)
	}
}

func TestCascadeRuleCreated_ManualQueuesWithoutDrain(t *testing.T) {
	resetCascadeDeps(t)
	svcGetAllKnownUserIDs = func(ctx context.Context) ([]string, error) {
		return []string{"u1"}, nil
	}
	svcGetDirectGrantsForUser = func(ctx context.Context, u string, inc bool) ([]models.DirectGrant, error) {
		return []models.DirectGrant{{UserID: u, ProjectID: "sp", RoleKey: "sr"}}, nil
	}
	svcGetUserBundleRolesGrouped = noBundleRoles
	svcGetActiveMappingRules = noRules
	svcCreateRuleAndEnqueue = func(ctx context.Context, actor, sp, sr, tp, tr, mode string, params []db.EnqueueParams) (string, []string, error) {
		return "rule-3", []string{"o1"}, nil
	}
	drainCalled := false
	svcDrainBatch = func(ctx context.Context, ids []string) (propagation.DrainResult, error) {
		drainCalled = true
		return propagation.DrainResult{}, nil
	}

	ruleID, res, err := CascadeRuleCreated(context.Background(), "admin", "sp", "sr", "tp", "tr", "manual")
	if err != nil {
		t.Fatal(err)
	}
	if ruleID != "rule-3" {
		t.Fatalf("ruleID = %q", ruleID)
	}
	if drainCalled {
		t.Fatal("manual mode must not drain")
	}
	if res.Mode != "manual" {
		t.Fatalf("mode = %q", res.Mode)
	}
}

// --- applyMode: drain failure is non-fatal ---

func TestApplyMode_AutoDrainFailureIsNonFatal(t *testing.T) {
	resetCascadeDeps(t)
	svcDrainBatch = func(ctx context.Context, ids []string) (propagation.DrainResult, error) {
		return propagation.DrainResult{}, errors.New("zitadel unreachable")
	}

	res, err := applyMode(context.Background(), "auto", []string{"o1"})
	if err != nil {
		t.Fatalf("a drain failure must not be returned as an error (rows stay pending): %v", err)
	}
	if !res.Drain.Halted {
		t.Fatal("expected res.Drain.Halted to surface the drain failure")
	}
	if res.Drain.Reason == "" {
		t.Fatal("expected a non-empty halt reason")
	}
}

// --- CascadeBundleRemovedFromUser ---

// TestCascadeBundleRemoved_ClosureCoverageSuppressesRevoke is brief test 3 (replaces the old
// OtherSourceCovers test): the user is in bundles b1 and b2, both granting role A. Removing b1
// must NOT revoke A — it is still in the post-removal closure via b2 — so nothing is enqueued.
func TestCascadeBundleRemoved_ClosureCoverageSuppressesRevoke(t *testing.T) {
	resetCascadeDeps(t)
	svcGetBundleByID = func(ctx context.Context, id string) (models.Bundle, error) {
		return models.Bundle{ID: id, ConfirmationMode: "auto"}, nil
	}
	svcGetDirectGrantsForUser = noDirects
	svcGetBundlesForUser = func(ctx context.Context, u string) ([]models.Bundle, error) {
		return []models.Bundle{{ID: "b1"}, {ID: "b2"}}, nil // still assigned to both at read time
	}
	svcGetUserBundleRolesGrouped = func(ctx context.Context, u string) (map[string][]models.BundleRole, error) {
		// Both b1 and b2 grant A, each through the version this user is pinned to.
		return map[string][]models.BundleRole{
			"b1": {{ProjectID: "p1", RoleKey: "A"}},
			"b2": {{ProjectID: "p1", RoleKey: "A"}},
		}, nil
	}
	svcGetActiveMappingRules = noRules
	var passed []db.EnqueueParams
	svcRemoveBundleFromUserAndEnqueue = func(ctx context.Context, actor, userID, bundleID string, ps []db.EnqueueParams) ([]string, error) {
		passed = ps // the atomic fn still deletes the assignment even when ps is empty
		return nil, nil
	}

	res, err := CascadeBundleRemovedFromUser(context.Background(), "admin", "u1", "b1")
	if err != nil {
		t.Fatal(err)
	}
	if len(passed) != 0 {
		t.Fatalf("role still covered by b2 must not enqueue a revoke, got %+v", passed)
	}
	if res.Enqueued != 0 {
		t.Fatalf("enqueued = %d, want 0", res.Enqueued)
	}
}

// TestCascadeBundleRemoved_ClosureRevokeSymmetry is brief test 2: U holds role A via bundle B,
// which fires a rule A→B2. Removing B (no other source of A) must revoke BOTH A and the
// rule-derived B2 — proving the revoke side discovers rule-derived targets just like the add side.
func TestCascadeBundleRemoved_ClosureRevokeSymmetry(t *testing.T) {
	resetCascadeDeps(t)
	svcGetBundleByID = func(ctx context.Context, id string) (models.Bundle, error) {
		return models.Bundle{ID: id, ConfirmationMode: "auto"}, nil
	}
	svcGetDirectGrantsForUser = noDirects
	svcGetBundlesForUser = func(ctx context.Context, u string) ([]models.Bundle, error) {
		return []models.Bundle{{ID: "b1"}}, nil
	}
	svcGetUserBundleRolesGrouped = func(ctx context.Context, u string) (map[string][]models.BundleRole, error) {
		return map[string][]models.BundleRole{"b1": {{ProjectID: "p1", RoleKey: "A"}}}, nil
	}
	svcGetActiveMappingRules = func(ctx context.Context) ([]models.MappingRule, error) {
		return []models.MappingRule{{ID: "rule1", SourceProject: "p1", SourceRole: "A", TargetProject: "p1", TargetRole: "B2"}}, nil
	}
	var got []db.EnqueueParams
	svcRemoveBundleFromUserAndEnqueue = func(ctx context.Context, actor, userID, bundleID string, ps []db.EnqueueParams) ([]string, error) {
		got = ps
		return []string{"o1", "o2"}, nil
	}
	svcDrainBatch = func(ctx context.Context, ids []string) (propagation.DrainResult, error) {
		return propagation.DrainResult{}, nil
	}

	res, err := CascadeBundleRemovedFromUser(context.Background(), "admin", "u1", "b1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 revokes (A + rule-derived B2), got %+v", got)
	}
	revoked := map[string]bool{}
	for _, p := range got {
		if p.OpType != "revoke" || p.Source != "bundle" || p.SourceRef != "b1" {
			t.Fatalf("bad revoke param: %+v", p)
		}
		revoked[p.RoleKeys[0]] = true
	}
	if !revoked["A"] || !revoked["B2"] {
		t.Fatalf("expected revokes for A and B2, got %+v", got)
	}
	if res.Enqueued != 2 {
		t.Fatalf("enqueued = %d, want 2", res.Enqueued)
	}
}

func TestCascadeBundleRemoved_RevokesWhenUncovered(t *testing.T) {
	resetCascadeDeps(t)
	svcGetBundleByID = func(ctx context.Context, id string) (models.Bundle, error) {
		return models.Bundle{ID: id, ConfirmationMode: "auto"}, nil
	}
	svcGetDirectGrantsForUser = noDirects
	svcGetBundlesForUser = func(ctx context.Context, u string) ([]models.Bundle, error) {
		return []models.Bundle{{ID: "b1"}}, nil
	}
	svcGetUserBundleRolesGrouped = func(ctx context.Context, u string) (map[string][]models.BundleRole, error) {
		return map[string][]models.BundleRole{"b1": {{ProjectID: "p1", RoleKey: "r1"}}}, nil
	}
	svcGetActiveMappingRules = noRules
	var got []db.EnqueueParams
	svcRemoveBundleFromUserAndEnqueue = func(ctx context.Context, actor, userID, bundleID string, ps []db.EnqueueParams) ([]string, error) {
		got = ps
		return []string{"o1"}, nil
	}
	svcDrainBatch = func(ctx context.Context, ids []string) (propagation.DrainResult, error) {
		return propagation.DrainResult{}, nil
	}

	res, err := CascadeBundleRemovedFromUser(context.Background(), "admin", "u1", "b1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].OpType != "revoke" || got[0].Source != "bundle" || got[0].SourceRef != "b1" {
		t.Fatalf("bad revoke params: %+v", got)
	}
	if res.Enqueued != 1 {
		t.Fatalf("enqueued = %d, want 1", res.Enqueued)
	}
}

func TestCascadeBundleRemoved_EnqueueErrorPropagates(t *testing.T) {
	resetCascadeDeps(t)
	svcGetBundleByID = func(ctx context.Context, id string) (models.Bundle, error) {
		return models.Bundle{ID: id, ConfirmationMode: "auto"}, nil
	}
	svcGetDirectGrantsForUser = noDirects
	svcGetBundlesForUser = func(ctx context.Context, u string) ([]models.Bundle, error) {
		return []models.Bundle{{ID: "b1"}}, nil
	}
	svcGetUserBundleRolesGrouped = func(ctx context.Context, u string) (map[string][]models.BundleRole, error) {
		return map[string][]models.BundleRole{"b1": {{ProjectID: "p1", RoleKey: "r1"}}}, nil
	}
	svcGetActiveMappingRules = noRules
	wantErr := errors.New("remove tx failed")
	svcRemoveBundleFromUserAndEnqueue = func(ctx context.Context, actor, userID, bundleID string, ps []db.EnqueueParams) ([]string, error) {
		return nil, wantErr
	}

	_, err := CascadeBundleRemovedFromUser(context.Background(), "admin", "u1", "b1")
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected the enqueue error to propagate, got %v", err)
	}
}

// Assigning a bundle projects the version the assignment PINS — the latest
// published one — not the working copy.
//
// The two are written in the same transaction: AssignBundleAndEnqueue pins the
// latest version while the caller supplies the outbox rows. Building those rows
// from `bundle_roles` meant a new member was pinned to v2 and simultaneously
// granted whatever unpublished edit was sitting in the working copy.
func TestCascadeBundleAssigned_ProjectsThePublishedVersionNotTheWorkingCopy(t *testing.T) {
	resetCascadeDeps(t)
	svcGetBundleByID = func(ctx context.Context, id string) (models.Bundle, error) {
		return models.Bundle{ID: id, ConfirmationMode: "manual"}, nil
	}
	svcGetDirectGrantsForUser = noDirects
	svcGetUserBundleRolesGrouped = noBundleRoles
	svcGetActiveMappingRules = noRules

	// The working copy has an unpublished addition. Reading it here is the bug.
	svcCascGetRolesForBundle = func(ctx context.Context, id string) ([]models.BundleRole, error) {
		return []models.BundleRole{
			{ProjectID: "p1", RoleKey: "published"},
			{ProjectID: "p1", RoleKey: "draft_only"},
		}, nil
	}
	svcLatestVersionRoles = func(ctx context.Context, id string) (models.BundleVersion, []models.BundleRole, error) {
		return models.BundleVersion{ID: "v-latest", Version: 2}, []models.BundleRole{{ProjectID: "p1", RoleKey: "published"}}, nil
	}

	var passed []db.EnqueueParams
	svcAssignBundleAndEnqueue = func(ctx context.Context, actor, userID, bundleID, versionID string, ps []db.EnqueueParams) ([]string, bool, error) {
		passed = ps
		return nil, true, nil
	}

	if _, err := CascadeBundleAssignedToUser(context.Background(), "admin", "u1", "b1"); err != nil {
		t.Fatal(err)
	}
	if len(passed) != 1 {
		t.Fatalf("expected exactly the published role, got %+v", passed)
	}
	if passed[0].RoleKeys[0] != "published" {
		t.Fatalf("an unpublished role was projected to a new holder: %+v", passed)
	}
}

// --- Publishing a version ---
//
// These are the two suppression properties that used to be tested against
// CascadeRoleRemovedFromBundle. The behaviour moved to publish, so the tests
// moved with it: what must survive is that a revoke is only projected when
// NOTHING else still grants the role.

func publishStubs(t *testing.T, working, published []models.BundleRole, holders []models.BundleHolder) {
	t.Helper()
	svcGetBundleByID = func(ctx context.Context, id string) (models.Bundle, error) {
		return models.Bundle{ID: id, ConfirmationMode: "manual"}, nil
	}
	svcLatestVersion = func(ctx context.Context, id string) (models.BundleVersion, error) {
		return models.BundleVersion{ID: "v-old", BundleID: id, Version: 2}, nil
	}
	svcCascGetRolesForBundle = func(ctx context.Context, id string) ([]models.BundleRole, error) {
		return working, nil
	}
	svcGetRolesForVersion = func(ctx context.Context, id string) ([]models.BundleRole, error) {
		return published, nil
	}
	svcGetBundleHoldersByVersion = func(ctx context.Context, id string) ([]models.BundleHolder, error) {
		return holders, nil
	}
	svcGetUsersForBundle = func(ctx context.Context, id string) ([]string, error) {
		ids := make([]string, 0, len(holders))
		for _, h := range holders {
			ids = append(ids, h.UserID)
		}
		return ids, nil
	}
	svcListBundleVersions = func(ctx context.Context, id string) ([]models.BundleVersion, error) {
		return []models.BundleVersion{{ID: "v-old", BundleID: id, Version: 2}}, nil
	}
	svcVersionBelongsTo = func(ctx context.Context, bundleID, versionID string) (bool, error) {
		return versionID == "v-old", nil
	}
}

func TestPublish_SuppressesRevokeWhenAnotherRuleStillGrantsIt(t *testing.T) {
	resetCascadeDeps(t)
	// v2 had r1; the working copy drops it. u1 also holds sp/sr directly, and a
	// mapping rule turns that into p1/r1 — so the revoke must not be projected.
	publishStubs(t,
		nil,
		[]models.BundleRole{{ProjectID: "p1", RoleKey: "r1"}},
		[]models.BundleHolder{{UserID: "u1", VersionID: "v-old", Version: 2}})
	svcGetDirectGrantsForUser = func(ctx context.Context, u string, inc bool) ([]models.DirectGrant, error) {
		return []models.DirectGrant{{UserID: u, ProjectID: "sp", RoleKey: "sr"}}, nil
	}
	svcGetUserBundleRolesGrouped = noBundleRoles
	svcGetActiveMappingRules = func(ctx context.Context) ([]models.MappingRule, error) {
		return []models.MappingRule{{ID: "rule-1", SourceProject: "sp", SourceRole: "sr", TargetProject: "p1", TargetRole: "r1"}}, nil
	}

	var passed []db.EnqueueParams
	svcPublishVersionAndEnqueue = func(ctx context.Context, actor, bundleID, note string, roles []models.BundleRole, moved []string, ps []db.EnqueueParams) (models.BundleVersion, []string, error) {
		passed = ps
		return models.BundleVersion{ID: "v-new", Version: 3}, nil, nil
	}

	plan, _, err := PublishBundleVersion(context.Background(), "admin",
		PublishRequest{BundleID: "b1", Migrate: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(passed) != 0 {
		t.Fatalf("role still produced by an active rule must not be revoked, got %+v", passed)
	}
	if plan.Summary.NoChange != 1 {
		t.Fatalf("the holder should read as no-change, got %+v", plan.Summary)
	}
}

func TestPublish_RevokesHoldersNothingElseCovers(t *testing.T) {
	resetCascadeDeps(t)
	publishStubs(t,
		nil,
		[]models.BundleRole{{ProjectID: "p1", RoleKey: "r1"}},
		[]models.BundleHolder{
			{UserID: "u1", VersionID: "v-old", Version: 2},
			{UserID: "u2", VersionID: "v-old", Version: 2},
		})
	svcGetDirectGrantsForUser = noDirects
	svcGetUserBundleRolesGrouped = func(ctx context.Context, u string) (map[string][]models.BundleRole, error) {
		return map[string][]models.BundleRole{"b1": {{BundleID: "b1", ProjectID: "p1", RoleKey: "r1"}}}, nil
	}
	svcGetActiveMappingRules = noRules

	var passed []db.EnqueueParams
	svcPublishVersionAndEnqueue = func(ctx context.Context, actor, bundleID, note string, roles []models.BundleRole, moved []string, ps []db.EnqueueParams) (models.BundleVersion, []string, error) {
		passed = ps
		return models.BundleVersion{ID: "v-new", Version: 3}, []string{"o1", "o2"}, nil
	}

	plan, _, err := PublishBundleVersion(context.Background(), "admin",
		PublishRequest{BundleID: "b1", Migrate: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(passed) != 2 {
		t.Fatalf("expected one revoke per uncovered holder, got %+v", passed)
	}
	for _, p := range passed {
		if p.OpType != "revoke" || p.ProjectID != "p1" || p.RoleKeys[0] != "r1" {
			t.Fatalf("bad param: %+v", p)
		}
	}
	// Applied, not Apply: the plan has been through the apply pass.
	if plan.Summary.Succeeded != 2 {
		t.Fatalf("both holders should be acted on, got %+v", plan.Summary)
	}
}

// "Leave them where they are" is a real answer, not a deferral: the version is
// still written, and not one outbox row is queued.
func TestPublish_WithoutMigrateWritesTheVersionAndTouchesNobody(t *testing.T) {
	resetCascadeDeps(t)
	publishStubs(t,
		[]models.BundleRole{{ProjectID: "p1", RoleKey: "r1"}, {ProjectID: "p2", RoleKey: "r2"}},
		[]models.BundleRole{{ProjectID: "p1", RoleKey: "r1"}},
		[]models.BundleHolder{{UserID: "u1", VersionID: "v-old", Version: 2}})
	svcGetDirectGrantsForUser = noDirects
	svcGetUserBundleRolesGrouped = noBundleRoles
	svcGetActiveMappingRules = noRules

	var passed []db.EnqueueParams
	var movedUsers []string
	var snapshot []models.BundleRole
	svcPublishVersionAndEnqueue = func(ctx context.Context, actor, bundleID, note string, roles []models.BundleRole, moved []string, ps []db.EnqueueParams) (models.BundleVersion, []string, error) {
		passed, movedUsers, snapshot = ps, moved, roles
		return models.BundleVersion{ID: "v-new", Version: 3}, nil, nil
	}

	plan, version, err := PublishBundleVersion(context.Background(), "admin",
		PublishRequest{BundleID: "b1", Migrate: false})
	if err != nil {
		t.Fatal(err)
	}
	if version.Version != 3 {
		t.Fatalf("the version must still be published, got v%d", version.Version)
	}
	if len(passed) != 0 || len(movedUsers) != 0 {
		t.Fatalf("nobody was migrated, so nothing may be enqueued or repinned: %v / %v", passed, movedUsers)
	}
	// The snapshot is the caller's set, from the same read the plan was built
	// from — not a fresh SELECT that a concurrent edit could have moved.
	if len(snapshot) != 2 {
		t.Fatalf("the version must be snapshotted from the working copy the plan used, got %+v", snapshot)
	}
	if plan.Summary.NoChange != 1 {
		t.Fatalf("the holder stays put, got %+v", plan.Summary)
	}
}

// Publishing a bundle that matches its latest version is refused rather than
// writing an identical v4 — a version list where half the entries changed
// nothing is a list nobody reads.
func TestPublish_RefusesWhenThereIsNothingToPublish(t *testing.T) {
	resetCascadeDeps(t)
	same := []models.BundleRole{{ProjectID: "p1", RoleKey: "r1"}}
	publishStubs(t, same, same, nil)
	svcGetDirectGrantsForUser = noDirects
	svcGetUserBundleRolesGrouped = noBundleRoles
	svcGetActiveMappingRules = noRules
	svcPublishVersionAndEnqueue = func(ctx context.Context, actor, bundleID, note string, roles []models.BundleRole, moved []string, ps []db.EnqueueParams) (models.BundleVersion, []string, error) {
		t.Fatal("must not write a version when nothing changed")
		return models.BundleVersion{}, nil, nil
	}

	if _, _, err := PublishBundleVersion(context.Background(), "admin",
		PublishRequest{BundleID: "b1", Migrate: true}); err == nil {
		t.Fatal("expected publishing an unchanged bundle to be refused")
	}
}

// --- CascadeRuleUpdated ---

// TestCascadeRuleUpdated_TargetChangeClosureDiff is brief test 5: old sp,sr→tp,trOld; new
// sp,sr→tp,trNew. U holds sp,sr. Must add trNew and revoke trOld, both attributed to old.ID.
func TestCascadeRuleUpdated_TargetChangeClosureDiff(t *testing.T) {
	resetCascadeDeps(t)
	old := models.MappingRule{ID: "rule1", SourceProject: "sp", SourceRole: "sr",
		TargetProject: "tp", TargetRole: "trOld", ConfirmationMode: "auto"}
	svcGetAllKnownUserIDs = func(ctx context.Context) ([]string, error) { return []string{"u1"}, nil }
	svcGetActiveMappingRules = func(ctx context.Context) ([]models.MappingRule, error) {
		return []models.MappingRule{old}, nil
	}
	svcGetDirectGrantsForUser = func(ctx context.Context, u string, inc bool) ([]models.DirectGrant, error) {
		return []models.DirectGrant{{UserID: u, ProjectID: "sp", RoleKey: "sr"}}, nil
	}
	svcGetUserBundleRolesGrouped = noBundleRoles
	var got []db.EnqueueParams
	svcUpdateRuleAndEnqueue = func(ctx context.Context, actor, id, sp, sr, tp, tr string, ps []db.EnqueueParams) ([]string, error) {
		got = ps
		return []string{"o1", "o2"}, nil
	}
	svcDrainBatch = func(ctx context.Context, ids []string) (propagation.DrainResult, error) {
		return propagation.DrainResult{}, nil
	}

	if _, err := CascadeRuleUpdated(context.Background(), "admin", old, "sp", "sr", "tp", "trNew"); err != nil {
		t.Fatal(err)
	}
	var addNew, revokeOld int
	for _, p := range got {
		if p.OpType == "add" && p.RoleKeys[0] == "trNew" {
			addNew++
		}
		if p.OpType == "revoke" && p.RoleKeys[0] == "trOld" {
			revokeOld++
		}
		if p.Source != "rule" || p.SourceRef != "rule1" {
			t.Fatalf("bad source attribution: %+v", p)
		}
	}
	if addNew != 1 || revokeOld != 1 {
		t.Fatalf("addNew=%d revokeOld=%d, want 1/1 (%+v)", addNew, revokeOld, got)
	}
}

// TestCascadeRuleUpdated_OldTargetStillCoveredByBundle_NoRevoke: the old target is still granted
// by a bundle independent of the rule, so the post-update closure still contains it — no revoke.
func TestCascadeRuleUpdated_OldTargetStillCoveredByBundle_NoRevoke(t *testing.T) {
	resetCascadeDeps(t)
	old := models.MappingRule{ID: "rule1", SourceProject: "sp", SourceRole: "sr",
		TargetProject: "tp", TargetRole: "trOld", ConfirmationMode: "auto"}
	svcGetAllKnownUserIDs = func(ctx context.Context) ([]string, error) { return []string{"u1"}, nil }
	svcGetActiveMappingRules = func(ctx context.Context) ([]models.MappingRule, error) {
		return []models.MappingRule{old}, nil
	}
	svcGetDirectGrantsForUser = func(ctx context.Context, u string, inc bool) ([]models.DirectGrant, error) {
		return []models.DirectGrant{{UserID: u, ProjectID: "sp", RoleKey: "sr"}}, nil
	}
	svcGetBundlesForUser = func(ctx context.Context, u string) ([]models.Bundle, error) {
		return []models.Bundle{{ID: "b1"}}, nil // still covers the old target via a bundle
	}
	svcGetUserBundleRolesGrouped = func(ctx context.Context, u string) (map[string][]models.BundleRole, error) {
		return map[string][]models.BundleRole{"b1": {{ProjectID: "tp", RoleKey: "trOld"}}}, nil
	}
	var got []db.EnqueueParams
	svcUpdateRuleAndEnqueue = func(ctx context.Context, actor, id, sp, sr, tp, tr string, ps []db.EnqueueParams) ([]string, error) {
		got = ps
		return nil, nil
	}

	if _, err := CascadeRuleUpdated(context.Background(), "admin", old, "sp", "sr", "tp", "trNew"); err != nil {
		t.Fatal(err)
	}
	for _, p := range got {
		if p.OpType == "revoke" {
			t.Fatalf("old target still covered by a bundle — must not revoke, got %+v", got)
		}
	}
}

func TestCascadeRuleUpdated_SameTripleReAdded_NoChurnForKeptUser(t *testing.T) {
	resetCascadeDeps(t)
	// Source-only change: source project changes but target stays the same. u1 holds BOTH the old
	// and new source directly, so tp/tr is already in the closure via the old rule before the
	// update, and stays in the closure via the new rule after — identical before/after, no churn.
	old := models.MappingRule{ID: "rule1", SourceProject: "spOld", SourceRole: "sr",
		TargetProject: "tp", TargetRole: "tr", ConfirmationMode: "auto"}
	svcGetAllKnownUserIDs = func(ctx context.Context) ([]string, error) { return []string{"u1"}, nil }
	svcGetActiveMappingRules = func(ctx context.Context) ([]models.MappingRule, error) {
		return []models.MappingRule{old}, nil
	}
	svcGetDirectGrantsForUser = func(ctx context.Context, u string, inc bool) ([]models.DirectGrant, error) {
		return []models.DirectGrant{
			{UserID: u, ProjectID: "spOld", RoleKey: "sr"},
			{UserID: u, ProjectID: "spNew", RoleKey: "sr"},
		}, nil
	}
	svcGetUserBundleRolesGrouped = noBundleRoles
	var got []db.EnqueueParams
	svcUpdateRuleAndEnqueue = func(ctx context.Context, actor, id, sp, sr, tp, tr string, ps []db.EnqueueParams) ([]string, error) {
		got = ps
		return nil, nil
	}

	if _, err := CascadeRuleUpdated(context.Background(), "admin", old, "spNew", "sr", "tp", "tr"); err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("user holds the new source directly and already has tr — expected empty delta, got %+v", got)
	}
}

func TestCascadeRuleUpdated_SkipsUsersWithEmptyDelta(t *testing.T) {
	resetCascadeDeps(t)
	old := models.MappingRule{ID: "rule1", SourceProject: "sp", SourceRole: "sr",
		TargetProject: "tp", TargetRole: "trOld", ConfirmationMode: "manual"}
	svcGetAllKnownUserIDs = func(ctx context.Context) ([]string, error) { return []string{"u1", "u2"}, nil }
	svcGetActiveMappingRules = func(ctx context.Context) ([]models.MappingRule, error) {
		return []models.MappingRule{old}, nil
	}
	svcGetDirectGrantsForUser = func(ctx context.Context, u string, inc bool) ([]models.DirectGrant, error) {
		if u == "u1" {
			return []models.DirectGrant{{UserID: u, ProjectID: "sp", RoleKey: "sr"}}, nil
		}
		return nil, nil // u2 holds nothing — empty delta, must be skipped
	}
	svcGetUserBundleRolesGrouped = noBundleRoles
	var got []db.EnqueueParams
	svcUpdateRuleAndEnqueue = func(ctx context.Context, actor, id, sp, sr, tp, tr string, ps []db.EnqueueParams) ([]string, error) {
		got = ps
		return nil, nil
	}

	if _, err := CascadeRuleUpdated(context.Background(), "admin", old, "sp", "sr", "tp", "trNew"); err != nil {
		t.Fatal(err)
	}
	for _, p := range got {
		if p.UserID == "u2" {
			t.Fatalf("u2 has an empty delta and must be skipped, got %+v", got)
		}
	}
}

// A failed read of the working copy used to be indistinguishable from an empty
// bundle, and an empty bundle plans a revoke of everything. The error has to
// come back.
func TestPublish_ReadFailureIsNotAnEmptyBundle(t *testing.T) {
	resetCascadeDeps(t)
	publishStubs(t, nil, []models.BundleRole{{ProjectID: "p1", RoleKey: "r1"}},
		[]models.BundleHolder{{UserID: "u1", VersionID: "v-old", Version: 2}})
	svcCascGetRolesForBundle = func(ctx context.Context, id string) ([]models.BundleRole, error) {
		return nil, errors.New("connection reset")
	}
	svcGetDirectGrantsForUser = noDirects
	svcGetUserBundleRolesGrouped = noBundleRoles
	svcGetActiveMappingRules = noRules
	svcPublishVersionAndEnqueue = func(ctx context.Context, actor, bundleID, note string, roles []models.BundleRole, moved []string, ps []db.EnqueueParams) (models.BundleVersion, []string, error) {
		t.Fatal("must not publish on a failed working-copy read")
		return models.BundleVersion{}, nil, nil
	}

	if _, _, err := PublishBundleVersion(context.Background(), "admin",
		PublishRequest{BundleID: "b1", Migrate: true}); err == nil {
		t.Fatal("expected the read error to propagate")
	}
}

// A version from another bundle produced a plan reading "v2 → v0" and was only
// rejected on apply — a nonsense plan somebody had already approved.
func TestMoveHolders_RehearsalRejectsAForeignVersion(t *testing.T) {
	resetCascadeDeps(t)
	publishStubs(t, nil, nil, nil)
	svcVersionBelongsTo = func(ctx context.Context, bundleID, versionID string) (bool, error) {
		return false, nil
	}

	if _, err := RehearseMoveHolders(context.Background(),
		MoveHoldersRequest{BundleID: "b1", VersionID: "v-other", UserIDs: []string{"u1"}}); err == nil {
		t.Fatal("expected a version from another bundle to be refused before any plan is built")
	}
}

// Nobody stands on a version the moment it is published, so inferring its
// number from its holders yielded v0.
func TestMoveHolders_TargetVersionComesFromTheVersionList(t *testing.T) {
	resetCascadeDeps(t)
	publishStubs(t, nil, []models.BundleRole{{ProjectID: "p1", RoleKey: "r1"}},
		[]models.BundleHolder{{UserID: "u1", VersionID: "v-old", Version: 2}})
	svcGetDirectGrantsForUser = noDirects
	svcGetUserBundleRolesGrouped = noBundleRoles
	svcGetActiveMappingRules = noRules
	svcVersionBelongsTo = func(ctx context.Context, bundleID, versionID string) (bool, error) {
		return true, nil
	}
	svcListBundleVersions = func(ctx context.Context, id string) ([]models.BundleVersion, error) {
		return []models.BundleVersion{
			{ID: "v-new", BundleID: id, Version: 3},
			{ID: "v-old", BundleID: id, Version: 2},
		}, nil
	}
	svcGetRolesForVersion = func(ctx context.Context, id string) ([]models.BundleRole, error) {
		return []models.BundleRole{{ProjectID: "p1", RoleKey: "r1"}}, nil
	}

	plan, err := RehearseMoveHolders(context.Background(),
		MoveHoldersRequest{BundleID: "b1", VersionID: "v-new", UserIDs: []string{"u1"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Outcomes) != 1 {
		t.Fatalf("expected one row, got %+v", plan.Outcomes)
	}
	if strings.Contains(plan.Outcomes[0].Detail, "v0") {
		t.Fatalf("the target version was inferred from holders and came out as v0: %q", plan.Outcomes[0].Detail)
	}
	if !strings.Contains(plan.Outcomes[0].Detail, "v2 → v3") {
		t.Fatalf("expected the move to name both versions, got %q", plan.Outcomes[0].Detail)
	}
}

// The assignment must pin the version it PROJECTED, not whatever is latest when
// the write transaction runs.
//
// Both reads used to resolve "latest" independently: the service read v2's
// roles, then the transaction selected the latest version to pin. A publish
// committing between them left the member pinned to v3 while the outbox carried
// v2's roles — and afterwards neither row looks wrong on its own, so nothing
// downstream can detect it. Passing the version id through is the only way the
// two can be the same version by construction.
func TestCascadeBundleAssigned_PinsTheVersionItProjected(t *testing.T) {
	resetCascadeDeps(t)
	svcGetBundleByID = func(ctx context.Context, id string) (models.Bundle, error) {
		return models.Bundle{ID: id, ConfirmationMode: "manual"}, nil
	}
	svcGetDirectGrantsForUser = noDirects
	svcGetUserBundleRolesGrouped = noBundleRoles
	svcGetActiveMappingRules = noRules
	svcLatestVersionRoles = func(ctx context.Context, id string) (models.BundleVersion, []models.BundleRole, error) {
		return models.BundleVersion{ID: "v2-id", BundleID: id, Version: 2},
			[]models.BundleRole{{ProjectID: "p1", RoleKey: "r1"}}, nil
	}

	var pinned string
	var projected []db.EnqueueParams
	svcAssignBundleAndEnqueue = func(ctx context.Context, actor, userID, bundleID, versionID string, ps []db.EnqueueParams) ([]string, bool, error) {
		pinned, projected = versionID, ps
		return nil, true, nil
	}

	if _, err := CascadeBundleAssignedToUser(context.Background(), "admin", "u1", "b1"); err != nil {
		t.Fatal(err)
	}
	if pinned != "v2-id" {
		t.Fatalf("the pin must be the version whose roles were projected, got %q", pinned)
	}
	if len(projected) != 1 || projected[0].RoleKeys[0] != "r1" {
		t.Fatalf("unexpected projection: %+v", projected)
	}
}

// Re-assigning a bundle somebody already holds must change nothing.
//
// The insert conflicts on (user_id, bundle_id) and preserves their existing
// pin — so a person on v1 stays on v1. The delta, though, was computed against
// the LATEST version, and enqueuing it handed them v2's access while every
// record still said v1: newer access than their pin, with nothing on any screen
// able to show the discrepancy.
//
// Moving somebody forward is its own rehearsed action. It must not happen as a
// side effect of an assign that was meant to be idempotent.
func TestCascadeBundleAssigned_ExistingHolderIsANoOp(t *testing.T) {
	resetCascadeDeps(t)
	svcGetBundleByID = func(ctx context.Context, id string) (models.Bundle, error) {
		return models.Bundle{ID: id, ConfirmationMode: "auto"}, nil
	}
	svcGetDirectGrantsForUser = noDirects
	svcGetActiveMappingRules = noRules
	// They are on v1, which granted only r1.
	svcGetUserBundleRolesGrouped = func(ctx context.Context, u string) (map[string][]models.BundleRole, error) {
		return map[string][]models.BundleRole{"b1": {{ProjectID: "p1", RoleKey: "r1"}}}, nil
	}
	// The latest version is v2, which also grants r2.
	svcLatestVersionRoles = func(ctx context.Context, id string) (models.BundleVersion, []models.BundleRole, error) {
		return models.BundleVersion{ID: "v2-id", BundleID: id, Version: 2},
			[]models.BundleRole{{ProjectID: "p1", RoleKey: "r1"}, {ProjectID: "p1", RoleKey: "r2"}}, nil
	}

	// The transaction reports the conflict: nothing was inserted.
	var offered []db.EnqueueParams
	svcAssignBundleAndEnqueue = func(ctx context.Context, actor, userID, bundleID, versionID string, ps []db.EnqueueParams) ([]string, bool, error) {
		offered = ps
		return nil, false, nil
	}
	drained := false
	svcDrainBatch = func(ctx context.Context, ids []string) (propagation.DrainResult, error) {
		drained = true
		return propagation.DrainResult{}, nil
	}

	res, err := CascadeBundleAssignedToUser(context.Background(), "admin", "u1", "b1")
	if err != nil {
		t.Fatal(err)
	}
	// The service still computes a delta — it cannot know about the conflict
	// until the transaction answers — but nothing may be projected from it.
	if len(offered) == 0 {
		t.Fatal("expected the delta to have been computed and offered to the tx")
	}
	if res.Enqueued != 0 {
		t.Fatalf("an existing holder must not be projected against: enqueued = %d", res.Enqueued)
	}
	if drained {
		t.Fatal("nothing was written, so nothing may be drained")
	}
}
