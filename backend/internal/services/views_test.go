package services

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"mkauth/internal/models"
)

// resetGovernanceDeps captures and restores all governance/lineage injectable vars.
func resetGovernanceDeps(t *testing.T) {
	t.Helper()
	origGetRequests := svcGetAccessRequests
	origGetExpiring := svcGetExpiringDirectGrants
	origGetAllBundles := svcGetAllBundles
	origGetBundlesForUser := svcGetBundlesForUser
	origGetRolesForBundle := svcGetRolesForBundle
	origGetDirectGrants := svcGetDirectGrantsForUser
	origGetRules := svcGetActiveMappingRules
	t.Cleanup(func() {
		svcGetAccessRequests = origGetRequests
		svcGetExpiringDirectGrants = origGetExpiring
		svcGetAllBundles = origGetAllBundles
		svcGetBundlesForUser = origGetBundlesForUser
		svcGetRolesForBundle = origGetRolesForBundle
		svcGetDirectGrantsForUser = origGetDirectGrants
		svcGetActiveMappingRules = origGetRules
	})
}

func TestFormatRolesContract(t *testing.T) {
	roles := []string{"admin", "viewer"}

	gotDefault := formatRoles(roles, "array")
	defaultRoles, ok := gotDefault.([]string)
	if !ok {
		t.Fatalf("expected []string for default format, got %T", gotDefault)
	}
	if !reflect.DeepEqual(defaultRoles, roles) {
		t.Fatalf("default format mismatch: got %#v want %#v", defaultRoles, roles)
	}

	gotCSV := formatRoles(roles, "csv")
	if gotCSV != "admin,viewer" {
		t.Fatalf("csv mismatch: got %v", gotCSV)
	}

	gotSpace := formatRoles(roles, "space_delimited")
	if gotSpace != "admin viewer" {
		t.Fatalf("space_delimited mismatch: got %v", gotSpace)
	}
}

func TestReadRolesFromClaims(t *testing.T) {
	input := []interface{}{"admin", 12, "viewer", true}
	got := readRoles(input)
	want := []string{"admin", "viewer"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("readRoles mismatch: got %#v want %#v", got, want)
	}
}

// --- Governance tests ---

func TestGovernance_NilSafeCollections(t *testing.T) {
	resetGovernanceDeps(t)

	svcGetAccessRequests = func(ctx context.Context, status string) ([]models.AccessRequest, error) {
		return nil, nil
	}
	svcGetExpiringDirectGrants = func(ctx context.Context, within time.Duration) ([]models.DirectGrant, error) {
		return nil, nil
	}
	svcGetAllBundles = func(ctx context.Context) ([]models.Bundle, error) {
		return []models.Bundle{}, nil
	}
	svcGetBundlesForUser = func(ctx context.Context, userID string) ([]models.Bundle, error) {
		return nil, nil
	}
	svcGetRolesForBundle = func(ctx context.Context, bundleID string) ([]models.BundleRole, error) {
		return nil, nil
	}

	summary, err := Governance(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify nil-safety by marshaling to JSON — nil slices serialize as "null"
	data, err := json.Marshal(summary)
	if err != nil {
		t.Fatalf("failed to marshal summary: %v", err)
	}
	body := string(data)
	if strings.Contains(body, `"pending_requests":null`) {
		t.Fatalf("PendingRequests serialized as null: %s", body)
	}
	if strings.Contains(body, `"expiring_grants":null`) {
		t.Fatalf("ExpiringGrants serialized as null: %s", body)
	}
}

func TestGovernance_PendingCountMatchesSliceLength(t *testing.T) {
	resetGovernanceDeps(t)

	svcGetAccessRequests = func(ctx context.Context, status string) ([]models.AccessRequest, error) {
		return []models.AccessRequest{
			{ID: "r1", Status: "pending"},
			{ID: "r2", Status: "pending"},
			{ID: "r3", Status: "pending"},
		}, nil
	}
	svcGetExpiringDirectGrants = func(ctx context.Context, within time.Duration) ([]models.DirectGrant, error) {
		return []models.DirectGrant{}, nil
	}
	svcGetAllBundles = func(ctx context.Context) ([]models.Bundle, error) {
		return []models.Bundle{}, nil
	}

	summary, err := Governance(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(summary.PendingRequests) != 3 {
		t.Fatalf("expected 3 pending requests, got %d", len(summary.PendingRequests))
	}
}

func TestGovernance_UnusedBundleHintGenerated(t *testing.T) {
	resetGovernanceDeps(t)

	svcGetAccessRequests = func(ctx context.Context, status string) ([]models.AccessRequest, error) {
		return []models.AccessRequest{}, nil
	}
	svcGetExpiringDirectGrants = func(ctx context.Context, within time.Duration) ([]models.DirectGrant, error) {
		return []models.DirectGrant{}, nil
	}
	svcGetAllBundles = func(ctx context.Context) ([]models.Bundle, error) {
		return []models.Bundle{{ID: "b1", Name: "Orphan Bundle"}}, nil
	}
	svcGetBundlesForUser = func(ctx context.Context, userID string) ([]models.Bundle, error) {
		// No users are assigned to any bundle.
		return []models.Bundle{}, nil
	}
	svcGetRolesForBundle = func(ctx context.Context, bundleID string) ([]models.BundleRole, error) {
		return []models.BundleRole{}, nil
	}

	summary, err := Governance(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	hasUnusedHint := false
	for _, hint := range summary.CleanupHints {
		if strings.Contains(hint, "Orphan Bundle") {
			hasUnusedHint = true
			break
		}
	}
	if !hasUnusedHint {
		t.Fatalf("expected cleanup hint for unused bundle, got hints: %v", summary.CleanupHints)
	}
}

// --- ExplainUserAccess / lineage tests ---

func TestExplainUserAccess_NilSafeCollections(t *testing.T) {
	resetGovernanceDeps(t)

	svcGetDirectGrantsForUser = func(ctx context.Context, userID string, includeExpired bool) ([]models.DirectGrant, error) {
		return nil, nil
	}
	svcGetBundlesForUser = func(ctx context.Context, userID string) ([]models.Bundle, error) {
		return nil, nil
	}
	svcGetActiveMappingRules = func(ctx context.Context) ([]models.MappingRule, error) {
		return nil, nil
	}
	svcGetRolesForBundle = func(ctx context.Context, bundleID string) ([]models.BundleRole, error) {
		return nil, nil
	}

	view, err := ExplainUserAccess(context.Background(), "dev_admin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if view.Bundles == nil {
		t.Fatalf("Bundles should not be nil")
	}
	for _, proj := range view.Projects {
		if proj.SourceRoles == nil {
			t.Fatalf("SourceRoles should not be nil for project %s", proj.ProjectID)
		}
		if proj.DerivedRoles == nil {
			t.Fatalf("DerivedRoles should not be nil for project %s", proj.ProjectID)
		}
	}
}

func TestExplainUserAccess_SourceVsDerivedLabeling(t *testing.T) {
	resetGovernanceDeps(t)

	svcGetDirectGrantsForUser = func(ctx context.Context, userID string, includeExpired bool) ([]models.DirectGrant, error) {
		return []models.DirectGrant{
			{UserID: userID, ProjectID: "p1", RoleKey: "r1"},
		}, nil
	}
	svcGetBundlesForUser = func(ctx context.Context, userID string) ([]models.Bundle, error) {
		return []models.Bundle{}, nil
	}
	svcGetActiveMappingRules = func(ctx context.Context) ([]models.MappingRule, error) {
		return []models.MappingRule{
			{ID: "rule-1", SourceProject: "p1", SourceRole: "r1", TargetProject: "p2", TargetRole: "r2"},
		}, nil
	}
	svcGetRolesForBundle = func(ctx context.Context, bundleID string) ([]models.BundleRole, error) {
		return nil, nil
	}

	view, err := ExplainUserAccess(context.Background(), "dev_admin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var p1Bucket, p2Bucket *models.ProjectAccessView
	for i := range view.Projects {
		switch view.Projects[i].ProjectID {
		case "p1":
			p1Bucket = &view.Projects[i]
		case "p2":
			p2Bucket = &view.Projects[i]
		}
	}

	if p1Bucket == nil {
		t.Fatalf("expected project p1 in view")
	}
	if p2Bucket == nil {
		t.Fatalf("expected project p2 in view (from derived rule)")
	}

	if len(p1Bucket.SourceRoles) == 0 || p1Bucket.SourceRoles[0].RoleKey != "r1" {
		t.Fatalf("expected p1:r1 in SourceRoles, got %v", p1Bucket.SourceRoles)
	}
	if !p1Bucket.SourceRoles[0].IsSource {
		t.Fatalf("expected p1:r1 IsSource=true")
	}

	if len(p2Bucket.DerivedRoles) == 0 || p2Bucket.DerivedRoles[0].RoleKey != "r2" {
		t.Fatalf("expected p2:r2 in DerivedRoles, got %v", p2Bucket.DerivedRoles)
	}
	if p2Bucket.DerivedRoles[0].IsSource {
		t.Fatalf("expected p2:r2 IsSource=false")
	}
}

func TestExplainUserAccess_BundleGrantLabeled(t *testing.T) {
	resetGovernanceDeps(t)

	svcGetDirectGrantsForUser = func(ctx context.Context, userID string, includeExpired bool) ([]models.DirectGrant, error) {
		return []models.DirectGrant{}, nil
	}
	svcGetBundlesForUser = func(ctx context.Context, userID string) ([]models.Bundle, error) {
		return []models.Bundle{{ID: "b1", Name: "Engineering"}}, nil
	}
	svcGetRolesForBundle = func(ctx context.Context, bundleID string) ([]models.BundleRole, error) {
		return []models.BundleRole{{BundleID: "b1", ProjectID: "p1", RoleKey: "r1"}}, nil
	}
	svcGetActiveMappingRules = func(ctx context.Context) ([]models.MappingRule, error) {
		return []models.MappingRule{}, nil
	}

	view, err := ExplainUserAccess(context.Background(), "dev_admin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var p1Bucket *models.ProjectAccessView
	for i := range view.Projects {
		if view.Projects[i].ProjectID == "p1" {
			p1Bucket = &view.Projects[i]
			break
		}
	}
	if p1Bucket == nil {
		t.Fatalf("expected project p1 in view")
	}
	if len(p1Bucket.SourceRoles) == 0 {
		t.Fatalf("expected bundle-assigned role in SourceRoles")
	}
	role := p1Bucket.SourceRoles[0]
	if role.RoleKey != "r1" {
		t.Fatalf("expected r1, got %s", role.RoleKey)
	}
	if len(role.Reasons) == 0 || role.Reasons[0].Kind != "bundle" {
		t.Fatalf("expected reason kind=bundle, got %v", role.Reasons)
	}
}

func TestExplainUserAccess_UnknownUser_ReturnsError(t *testing.T) {
	resetGovernanceDeps(t)

	_, err := ExplainUserAccess(context.Background(), "user-does-not-exist-xyz")
	if err == nil {
		t.Fatalf("expected error for unknown user, got nil")
	}
}

func TestExplainUserAccess_MultiHopDerivation(t *testing.T) {
	resetGovernanceDeps(t)

	svcGetDirectGrantsForUser = func(ctx context.Context, userID string, includeExpired bool) ([]models.DirectGrant, error) {
		return []models.DirectGrant{
			{UserID: userID, ProjectID: "p1", RoleKey: "r1"},
		}, nil
	}
	svcGetBundlesForUser = func(ctx context.Context, userID string) ([]models.Bundle, error) {
		return []models.Bundle{}, nil
	}
	svcGetActiveMappingRules = func(ctx context.Context) ([]models.MappingRule, error) {
		return []models.MappingRule{
			{ID: "rule-1", SourceProject: "p1", SourceRole: "r1", TargetProject: "p2", TargetRole: "r2"},
			{ID: "rule-2", SourceProject: "p2", SourceRole: "r2", TargetProject: "p3", TargetRole: "r3"},
		}, nil
	}
	svcGetRolesForBundle = func(ctx context.Context, bundleID string) ([]models.BundleRole, error) {
		return nil, nil
	}

	view, err := ExplainUserAccess(context.Background(), "dev_admin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	foundP3 := false
	for _, proj := range view.Projects {
		if proj.ProjectID == "p3" {
			for _, role := range proj.DerivedRoles {
				if role.RoleKey == "r3" {
					foundP3 = true
				}
			}
		}
	}
	if !foundP3 {
		t.Fatalf("expected p3:r3 in derived roles after multi-hop, got projects: %v", view.Projects)
	}
}

// --- Simulation (pure logic) ---

func TestFormatRoles_UnknownType_FallsBackToArray(t *testing.T) {
	roles := []string{"admin", "viewer"}
	result := formatRoles(roles, "unknown_format_type")
	got, ok := result.([]string)
	if !ok {
		t.Fatalf("expected []string for unknown format type, got %T", result)
	}
	if !reflect.DeepEqual(got, roles) {
		t.Fatalf("expected %v, got %v", roles, got)
	}
}

func TestUpsertRoleDeduplicatesReason(t *testing.T) {
	roleMap := map[roleKey]*models.EffectiveRole{}
	key := roleKey{projectID: "p1", roleKey: "admin"}
	reason := models.RoleReason{
		Kind:        "bundle",
		Description: "Granted by bundle Engineering",
		BundleID:    "b1",
		BundleName:  "Engineering",
	}

	ctx := context.Background()
	firstAdded := upsertRole(ctx, roleMap, key, true, reason)
	secondAdded := upsertRole(ctx, roleMap, key, true, reason)

	if !firstAdded {
		t.Fatalf("expected first insert to add role")
	}
	if secondAdded {
		t.Fatalf("expected duplicate reason to be ignored")
	}

	current := roleMap[key]
	if current == nil {
		t.Fatalf("expected role to exist after insert")
	}
	if len(current.Reasons) != 1 {
		t.Fatalf("expected exactly one reason, got %d", len(current.Reasons))
	}
	if !current.IsSource {
		t.Fatalf("expected source flag to remain true")
	}
}
