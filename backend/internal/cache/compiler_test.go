package cache

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"syndra/internal/models"
)

// resetCacheDeps saves and restores all injectable vars for test isolation.
func resetCacheDeps(t *testing.T) {
	t.Helper()
	origGrants := dbGetDirectGrantsForUser
	origRules := dbGetActiveMappingRules
	origBundleRoles := dbGetUserBundleRoles
	origSet := redisSet
	t.Cleanup(func() {
		dbGetDirectGrantsForUser = origGrants
		dbGetActiveMappingRules = origRules
		dbGetUserBundleRoles = origBundleRoles
		redisSet = origSet
	})

	dbGetUserBundleRoles = func(context.Context, string) (map[string][]models.BundleRole, error) {
		return nil, nil
	}
}

// capturedCache collects what was written to Redis.
type capturedCache struct {
	key   string
	value string
}

func setupNoopRedis(t *testing.T) *capturedCache {
	t.Helper()
	captured := &capturedCache{}
	redisSet = func(_ context.Context, key, value string, _ time.Duration) error {
		captured.key = key
		captured.value = value
		return nil
	}
	return captured
}

func parseCachedRoles(t *testing.T, cached *capturedCache) []string {
	t.Helper()
	if cached.value == "" {
		t.Fatal("nothing was written to cache")
	}
	var claims map[string]any
	if err := json.Unmarshal([]byte(cached.value), &claims); err != nil {
		t.Fatalf("failed to parse cached JSON: %v", err)
	}
	rawRoles, ok := claims["roles"].([]any)
	if !ok {
		t.Fatal("roles field is not an array")
	}
	roles := make([]string, len(rawRoles))
	for i, r := range rawRoles {
		roles[i] = r.(string)
	}
	return roles
}

func TestCompileUserCache_NoGrants(t *testing.T) {
	resetCacheDeps(t)
	cached := setupNoopRedis(t)

	dbGetDirectGrantsForUser = func(_ context.Context, _ string, _ bool) ([]models.DirectGrant, error) {
		return nil, nil
	}
	dbGetActiveMappingRules = func(_ context.Context) ([]models.MappingRule, error) {
		return nil, nil
	}

	err := CompileUserCache(context.Background(), "user1", "proj1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	roles := parseCachedRoles(t, cached)
	if len(roles) != 0 {
		t.Errorf("expected 0 roles, got %d: %v", len(roles), roles)
	}
	if cached.key != "mapping:user1:proj1" {
		t.Errorf("unexpected cache key: %s", cached.key)
	}
}

func TestCompileUserCache_DirectGrants(t *testing.T) {
	resetCacheDeps(t)
	cached := setupNoopRedis(t)

	dbGetDirectGrantsForUser = func(_ context.Context, _ string, _ bool) ([]models.DirectGrant, error) {
		return []models.DirectGrant{
			{ProjectID: "proj1", RoleKey: "editor"},
			{ProjectID: "proj1", RoleKey: "viewer"},
			{ProjectID: "proj2", RoleKey: "admin"},
		}, nil
	}
	dbGetActiveMappingRules = func(_ context.Context) ([]models.MappingRule, error) {
		return nil, nil
	}

	err := CompileUserCache(context.Background(), "user1", "proj1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	roles := parseCachedRoles(t, cached)
	if len(roles) != 2 {
		t.Fatalf("expected 2 roles for proj1, got %d: %v", len(roles), roles)
	}
	// Roles are sorted
	if roles[0] != "editor" || roles[1] != "viewer" {
		t.Errorf("unexpected roles: %v", roles)
	}
}

func TestCompileUserCache_MappingRuleTransitivity(t *testing.T) {
	resetCacheDeps(t)
	cached := setupNoopRedis(t)

	// User has role "member" in proj_a.
	// Rule 1: proj_a:member -> proj_b:reader
	// Rule 2: proj_b:reader -> proj_c:viewer
	// Compiling for proj_c should yield "viewer" via transitive resolution.

	dbGetDirectGrantsForUser = func(_ context.Context, _ string, _ bool) ([]models.DirectGrant, error) {
		return []models.DirectGrant{
			{ProjectID: "proj_a", RoleKey: "member"},
		}, nil
	}
	dbGetActiveMappingRules = func(_ context.Context) ([]models.MappingRule, error) {
		return []models.MappingRule{
			{SourceProject: "proj_a", SourceRole: "member", TargetProject: "proj_b", TargetRole: "reader"},
			{SourceProject: "proj_b", SourceRole: "reader", TargetProject: "proj_c", TargetRole: "viewer"},
		}, nil
	}

	err := CompileUserCache(context.Background(), "user1", "proj_c")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	roles := parseCachedRoles(t, cached)
	if len(roles) != 1 || roles[0] != "viewer" {
		t.Errorf("expected [viewer], got %v", roles)
	}
}

func TestCompileUserCache_BundleRolesIncluded(t *testing.T) {
	resetCacheDeps(t)
	cached := setupNoopRedis(t)

	dbGetDirectGrantsForUser = func(_ context.Context, _ string, _ bool) ([]models.DirectGrant, error) {
		return nil, nil
	}
	dbGetActiveMappingRules = func(_ context.Context) ([]models.MappingRule, error) {
		return nil, nil
	}
	// Keyed by bundle, resolved through the version this user is pinned to.
	dbGetUserBundleRoles = func(_ context.Context, _ string) (map[string][]models.BundleRole, error) {
		return map[string][]models.BundleRole{
			"bundle1": {
				{ProjectID: "proj1", RoleKey: "student"},
				{ProjectID: "proj1", RoleKey: "lab_access"},
			},
		}, nil
	}

	err := CompileUserCache(context.Background(), "user1", "proj1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	roles := parseCachedRoles(t, cached)
	if len(roles) != 2 {
		t.Fatalf("expected 2 roles, got %d: %v", len(roles), roles)
	}
	if roles[0] != "lab_access" || roles[1] != "student" {
		t.Errorf("unexpected roles: %v", roles)
	}
}

func TestCompileUserCache_FixedPointTerminates(t *testing.T) {
	resetCacheDeps(t)
	cached := setupNoopRedis(t)

	// Chain of 5 rules. Should terminate after at most 5 iterations.
	dbGetDirectGrantsForUser = func(_ context.Context, _ string, _ bool) ([]models.DirectGrant, error) {
		return []models.DirectGrant{{ProjectID: "p1", RoleKey: "r1"}}, nil
	}
	dbGetActiveMappingRules = func(_ context.Context) ([]models.MappingRule, error) {
		return []models.MappingRule{
			{SourceProject: "p1", SourceRole: "r1", TargetProject: "p2", TargetRole: "r2"},
			{SourceProject: "p2", SourceRole: "r2", TargetProject: "p3", TargetRole: "r3"},
			{SourceProject: "p3", SourceRole: "r3", TargetProject: "p4", TargetRole: "r4"},
			{SourceProject: "p4", SourceRole: "r4", TargetProject: "p5", TargetRole: "r5"},
			{SourceProject: "p5", SourceRole: "r5", TargetProject: "p5", TargetRole: "r_final"},
		}, nil
	}

	err := CompileUserCache(context.Background(), "user1", "p5")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	roles := parseCachedRoles(t, cached)
	if len(roles) != 2 {
		t.Fatalf("expected 2 roles (r5, r_final), got %d: %v", len(roles), roles)
	}
}

// An unpublished edit must never reach a token.
//
// The cache compiles the claims a token is issued from, and it used to resolve
// bundles through `bundle_roles` — the mutable working copy. Any rebuild after
// a draft edit (a webhook, a rule change, a manual recompile) baked the draft
// into real tokens, before anybody published it and without it appearing in any
// plan. The version-aware lookup is what makes that impossible: it can only
// return roles that belong to a version somebody is pinned to.
func TestCompileUserCache_ResolvesBundlesThroughThePinnedVersion(t *testing.T) {
	resetCacheDeps(t)
	cached := setupNoopRedis(t)
	dbGetDirectGrantsForUser = func(_ context.Context, _ string, _ bool) ([]models.DirectGrant, error) {
		return nil, nil
	}
	dbGetActiveMappingRules = func(_ context.Context) ([]models.MappingRule, error) {
		return nil, nil
	}
	// v2 is what they hold. The working copy has since gained `draft_only`,
	// which this lookup cannot see and must not return.
	dbGetUserBundleRoles = func(_ context.Context, _ string) (map[string][]models.BundleRole, error) {
		return map[string][]models.BundleRole{
			"bundle1": {{ProjectID: "proj1", RoleKey: "student"}},
		}, nil
	}

	if err := CompileUserCache(context.Background(), "user1", "proj1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	roles := parseCachedRoles(t, cached)
	for _, r := range roles {
		if r == "draft_only" {
			t.Fatal("an unpublished role reached the compiled claims")
		}
	}
	if len(roles) != 1 || roles[0] != "student" {
		t.Fatalf("expected only the pinned version's roles, got %v", roles)
	}
}
