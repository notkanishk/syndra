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
		svcGetActiveMappingRules = origGetActiveRules
		svcRemoveBundleFromUserAndEnqueue = origRemoveBundleAndEnqueue
		svcRemoveRoleFromBundleAndEnqueue = origRemoveRoleAndEnqueue
		svcUpdateRuleAndEnqueue = origUpdateRuleAndEnqueue
	})
}

// noBundles/noDirects/noRules are the common "holds nothing else" stubs used by most closure-diff
// tests below, to keep each test's arrange section focused on what it actually varies.
func noDirects(ctx context.Context, u string, inc bool) ([]models.DirectGrant, error) {
	return nil, nil
}
func noBundles(ctx context.Context, u string) ([]models.Bundle, error) { return nil, nil }
func noRules(ctx context.Context) ([]models.MappingRule, error)        { return nil, nil }

// --- CascadeBundleAssignedToUser ---

func TestCascadeBundleAssignedToUser_AutoEnqueuesPerRoleAndDrains(t *testing.T) {
	resetCascadeDeps(t)
	svcGetBundleByID = func(ctx context.Context, id string) (models.Bundle, error) {
		return models.Bundle{ID: id, ConfirmationMode: "auto"}, nil
	}
	svcCascGetRolesForBundle = func(ctx context.Context, id string) ([]models.BundleRole, error) {
		return []models.BundleRole{
			{ProjectID: "p1", RoleKey: "r1"}, {ProjectID: "p1", RoleKey: "r2"},
		}, nil
	}
	svcGetDirectGrantsForUser = noDirects
	svcGetBundlesForUser = noBundles
	svcGetActiveMappingRules = noRules
	var enqueued []db.EnqueueParams
	svcAssignBundleAndEnqueue = func(ctx context.Context, actor, userID, bundleID string, ps []db.EnqueueParams) ([]string, error) {
		enqueued = ps
		return []string{"o1", "o2"}, nil
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
	svcCascGetRolesForBundle = func(ctx context.Context, id string) ([]models.BundleRole, error) {
		return []models.BundleRole{{ProjectID: "p1", RoleKey: "r1"}}, nil
	}
	svcGetDirectGrantsForUser = noDirects
	svcGetBundlesForUser = noBundles
	svcGetActiveMappingRules = noRules
	svcAssignBundleAndEnqueue = func(ctx context.Context, actor, userID, bundleID string, ps []db.EnqueueParams) ([]string, error) {
		return []string{"o1"}, nil
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
	svcCascGetRolesForBundle = func(ctx context.Context, id string) ([]models.BundleRole, error) {
		return []models.BundleRole{{ProjectID: "p1", RoleKey: "r1"}}, nil
	}
	svcGetDirectGrantsForUser = noDirects
	svcGetBundlesForUser = noBundles
	svcGetActiveMappingRules = noRules
	wantErr := errors.New("assign tx failed")
	svcAssignBundleAndEnqueue = func(ctx context.Context, actor, userID, bundleID string, ps []db.EnqueueParams) ([]string, error) {
		return nil, wantErr
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
	svcCascGetRolesForBundle = func(ctx context.Context, id string) ([]models.BundleRole, error) {
		return []models.BundleRole{{ProjectID: "p1", RoleKey: "A"}}, nil
	}
	svcGetDirectGrantsForUser = noDirects
	svcGetBundlesForUser = noBundles
	svcGetActiveMappingRules = func(ctx context.Context) ([]models.MappingRule, error) {
		return []models.MappingRule{{ID: "rule1", SourceProject: "p1", SourceRole: "A", TargetProject: "p1", TargetRole: "B2"}}, nil
	}
	var enqueued []db.EnqueueParams
	svcAssignBundleAndEnqueue = func(ctx context.Context, actor, userID, bundleID string, ps []db.EnqueueParams) ([]string, error) {
		enqueued = ps
		return []string{"o1", "o2"}, nil
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
	svcCascGetRolesForBundle = func(ctx context.Context, id string) ([]models.BundleRole, error) {
		return []models.BundleRole{{ProjectID: "p1", RoleKey: "r1"}}, nil
	}
	svcGetDirectGrantsForUser = func(ctx context.Context, u string, inc bool) ([]models.DirectGrant, error) {
		return []models.DirectGrant{{UserID: u, ProjectID: "p1", RoleKey: "r1"}}, nil // already holds r1 directly
	}
	svcGetBundlesForUser = noBundles
	svcGetActiveMappingRules = noRules
	var enqueued []db.EnqueueParams
	assignCalled := false
	svcAssignBundleAndEnqueue = func(ctx context.Context, actor, userID, bundleID string, ps []db.EnqueueParams) ([]string, error) {
		assignCalled = true
		enqueued = ps
		return nil, nil
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

// --- CascadeRoleAddedToBundle ---

func TestCascadeRoleAddedToBundle_AutoEnqueuesPerMemberAndDrains(t *testing.T) {
	resetCascadeDeps(t)
	svcGetBundleByID = func(ctx context.Context, id string) (models.Bundle, error) {
		return models.Bundle{ID: id, ConfirmationMode: "auto"}, nil
	}
	svcGetUsersForBundle = func(ctx context.Context, id string) ([]string, error) {
		return []string{"u1", "u2"}, nil
	}
	svcGetDirectGrantsForUser = noDirects
	svcGetBundlesForUser = noBundles
	svcGetActiveMappingRules = noRules
	var enqueued []db.EnqueueParams
	svcAddRoleToBundleAndEnqueue = func(ctx context.Context, actor, bundleID, projectID, roleKey string, ps []db.EnqueueParams) ([]string, error) {
		enqueued = ps
		return []string{"o1", "o2"}, nil
	}
	var drainedIDs []string
	svcDrainBatch = func(ctx context.Context, ids []string) (propagation.DrainResult, error) {
		drainedIDs = ids
		return propagation.DrainResult{Applied: 2}, nil
	}

	res, err := CascadeRoleAddedToBundle(context.Background(), "admin", "b1", "p1", "r1")
	if err != nil {
		t.Fatal(err)
	}
	if len(enqueued) != 2 {
		t.Fatalf("enqueued %d params, want 2", len(enqueued))
	}
	for _, p := range enqueued {
		if p.Source != "bundle" || p.SourceRef != "b1" || p.OpType != "add" || p.ProjectID != "p1" || p.RoleKeys[0] != "r1" {
			t.Fatalf("bad param: %+v", p)
		}
	}
	if strings.Join(drainedIDs, ",") != "o1,o2" {
		t.Fatalf("drained %v", drainedIDs)
	}
	if res.Mode != "auto" {
		t.Fatalf("mode = %q", res.Mode)
	}
}

func TestCascadeRoleAddedToBundle_ManualQueuesWithoutDrain(t *testing.T) {
	resetCascadeDeps(t)
	svcGetBundleByID = func(ctx context.Context, id string) (models.Bundle, error) {
		return models.Bundle{ID: id, ConfirmationMode: "manual"}, nil
	}
	svcGetUsersForBundle = func(ctx context.Context, id string) ([]string, error) {
		return []string{"u1"}, nil
	}
	svcGetDirectGrantsForUser = noDirects
	svcGetBundlesForUser = noBundles
	svcGetActiveMappingRules = noRules
	svcAddRoleToBundleAndEnqueue = func(ctx context.Context, actor, bundleID, projectID, roleKey string, ps []db.EnqueueParams) ([]string, error) {
		return []string{"o1"}, nil
	}
	drainCalled := false
	svcDrainBatch = func(ctx context.Context, ids []string) (propagation.DrainResult, error) {
		drainCalled = true
		return propagation.DrainResult{}, nil
	}

	res, err := CascadeRoleAddedToBundle(context.Background(), "admin", "b1", "p1", "r1")
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
	svcCascGetRolesForBundle = func(ctx context.Context, id string) ([]models.BundleRole, error) {
		return []models.BundleRole{{ProjectID: "sp", RoleKey: "sr"}}, nil
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
	svcGetBundlesForUser = noBundles
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
	svcGetBundlesForUser = noBundles
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
	svcCascGetRolesForBundle = func(ctx context.Context, id string) ([]models.BundleRole, error) {
		return []models.BundleRole{{ProjectID: "p1", RoleKey: "A"}}, nil // both b1 and b2 grant A
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
	svcCascGetRolesForBundle = func(ctx context.Context, id string) ([]models.BundleRole, error) {
		return []models.BundleRole{{ProjectID: "p1", RoleKey: "A"}}, nil
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
	svcCascGetRolesForBundle = func(ctx context.Context, id string) ([]models.BundleRole, error) {
		return []models.BundleRole{{ProjectID: "p1", RoleKey: "r1"}}, nil
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
	svcCascGetRolesForBundle = func(ctx context.Context, id string) ([]models.BundleRole, error) {
		return []models.BundleRole{{ProjectID: "p1", RoleKey: "r1"}}, nil
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

// --- CascadeRoleRemovedFromBundle ---

func TestCascadeRoleRemoved_SuppressesRevokeWhenCoveredByAnotherRule(t *testing.T) {
	resetCascadeDeps(t)
	svcGetBundleByID = func(ctx context.Context, id string) (models.Bundle, error) {
		return models.Bundle{ID: id, ConfirmationMode: "auto"}, nil
	}
	svcGetUsersForBundle = func(ctx context.Context, id string) ([]string, error) {
		return []string{"u1"}, nil
	}
	svcGetDirectGrantsForUser = func(ctx context.Context, u string, inc bool) ([]models.DirectGrant, error) {
		return []models.DirectGrant{{UserID: u, ProjectID: "sp", RoleKey: "sr"}}, nil
	}
	svcGetBundlesForUser = noBundles
	svcGetActiveMappingRules = func(ctx context.Context) ([]models.MappingRule, error) {
		return []models.MappingRule{{ID: "rule-1", SourceProject: "sp", SourceRole: "sr", TargetProject: "p1", TargetRole: "r1"}}, nil
	}
	var passed []db.EnqueueParams
	svcRemoveRoleFromBundleAndEnqueue = func(ctx context.Context, actor, bundleID, projectID, roleKey string, ps []db.EnqueueParams) ([]string, error) {
		passed = ps
		return nil, nil
	}

	res, err := CascadeRoleRemovedFromBundle(context.Background(), "admin", "b1", "p1", "r1")
	if err != nil {
		t.Fatal(err)
	}
	if len(passed) != 0 {
		t.Fatalf("role still covered by an active rule must not enqueue a revoke, got %+v", passed)
	}
	if res.Enqueued != 0 {
		t.Fatalf("enqueued = %d, want 0", res.Enqueued)
	}
}

func TestCascadeRoleRemoved_RevokesUncoveredMembers(t *testing.T) {
	resetCascadeDeps(t)
	svcGetBundleByID = func(ctx context.Context, id string) (models.Bundle, error) {
		return models.Bundle{ID: id, ConfirmationMode: "manual"}, nil
	}
	svcGetUsersForBundle = func(ctx context.Context, id string) ([]string, error) {
		return []string{"u1", "u2"}, nil
	}
	svcGetDirectGrantsForUser = noDirects
	svcGetBundlesForUser = func(ctx context.Context, u string) ([]models.Bundle, error) {
		return []models.Bundle{{ID: "b1"}}, nil
	}
	svcCascGetRolesForBundle = func(ctx context.Context, id string) ([]models.BundleRole, error) {
		return []models.BundleRole{{ProjectID: "p1", RoleKey: "r1"}}, nil
	}
	svcGetActiveMappingRules = noRules
	var got []db.EnqueueParams
	svcRemoveRoleFromBundleAndEnqueue = func(ctx context.Context, actor, bundleID, projectID, roleKey string, ps []db.EnqueueParams) ([]string, error) {
		got = ps
		return []string{"o1", "o2"}, nil
	}

	res, err := CascadeRoleRemovedFromBundle(context.Background(), "admin", "b1", "p1", "r1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected a revoke per uncovered member, got %+v", got)
	}
	for _, p := range got {
		if p.OpType != "revoke" || p.Source != "bundle" || p.SourceRef != "b1" || p.ProjectID != "p1" || p.RoleKeys[0] != "r1" {
			t.Fatalf("bad revoke param: %+v", p)
		}
	}
	if res.Mode != "manual" {
		t.Fatalf("mode = %q, want manual", res.Mode)
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
	svcGetBundlesForUser = noBundles
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
	svcCascGetRolesForBundle = func(ctx context.Context, id string) ([]models.BundleRole, error) {
		return []models.BundleRole{{ProjectID: "tp", RoleKey: "trOld"}}, nil
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
	svcGetBundlesForUser = noBundles
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
	svcGetBundlesForUser = noBundles
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
