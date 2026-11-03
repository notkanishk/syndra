package cache

import (
	"context"
	"encoding/json"
	"errors"
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
	origBundles, origService := dbGetAllBundles, cacheIsServiceAccount
	t.Cleanup(func() {
		dbGetDirectGrantsForUser = origGrants
		dbGetActiveMappingRules = origRules
		dbGetUserBundleRoles = origBundleRoles
		redisSet = origSet
		dbGetAllBundles, cacheIsServiceAccount = origBundles, origService
	})

	dbGetUserBundleRoles = func(context.Context, string) (map[string][]models.BundleRole, error) {
		return nil, nil
	}
	dbGetAllBundles = func(context.Context) ([]models.Bundle, error) { return nil, nil }
	cacheIsServiceAccount = func(_ context.Context, u models.UserProfile) (bool, error) { return u.IsMachine(), nil }
}

// Facts carry what a claim row may project: the person's names, the bundles
// giving them a role in this project (by name), each role's end only when
// every source of it ends, and whether they are a service account.
func TestCompileFacts_NamesBundlesEndsAndServiceAccount(t *testing.T) {
	resetCacheDeps(t)
	origFind := cacheFindUser
	t.Cleanup(func() { cacheFindUser = origFind })

	end := time.Date(2027, 5, 31, 12, 0, 0, 0, time.UTC)
	dbGetDirectGrantsForUser = func(context.Context, string, bool) ([]models.DirectGrant, error) {
		return []models.DirectGrant{
			{ProjectID: "net", RoleKey: "staff", ExpiresAt: &end},
			{ProjectID: "net", RoleKey: "trained", ExpiresAt: &end},
			{ProjectID: "net", RoleKey: "admin"},
			{ProjectID: "other", RoleKey: "x"},
		}, nil
	}
	dbGetActiveMappingRules = func(context.Context) ([]models.MappingRule, error) { return nil, nil }
	dbGetUserBundleRoles = func(context.Context, string) (map[string][]models.BundleRole, error) {
		return map[string][]models.BundleRole{
			"b1": {{BundleID: "b1", ProjectID: "net", RoleKey: "trained"}},
			"b2": {{BundleID: "b2", ProjectID: "other", RoleKey: "y"}},
		}, nil
	}
	dbGetAllBundles = func(context.Context) ([]models.Bundle, error) {
		return []models.Bundle{{ID: "b1", Name: "Makers"}, {ID: "b2", Name: "Elsewhere"}}, nil
	}
	cacheFindUser = func(context.Context, string) (models.UserProfile, bool, error) {
		return models.UserProfile{ID: "u1", Name: "Ada Lovelace", GivenName: "Ada", FamilyName: "Lovelace", Username: "ada", Kind: models.UserKindMachine}, true, nil
	}

	f, err := CompileFacts(context.Background(), "u1", "net")
	if err != nil {
		t.Fatal(err)
	}
	if f.GivenName != "Ada" || f.FamilyName != "Lovelace" || f.Username != "ada" || !f.IsServiceAccount {
		t.Fatalf("profile facts: %+v", f)
	}
	if len(f.Bundles) != 1 || f.Bundles[0] != "Makers" {
		t.Fatalf("bundles are this project's, by name: %v", f.Bundles)
	}
	// trained also comes from a bundle, which never ends; admin has no end.
	if len(f.RoleEnds) != 1 || f.RoleEnds["staff"] != "2027-05-31" {
		t.Fatalf("role ends: %v", f.RoleEnds)
	}

	// A failed read of the service-account marks fails the profile part, so a
	// warm keeps the old entry instead of storing "not a service account".
	cacheIsServiceAccount = func(context.Context, models.UserProfile) (bool, error) { return false, errors.New("db down") }
	u, err := compileUser(context.Background(), "u1")
	if err != nil || u.profileErr == nil {
		t.Fatalf("want a profile error, got err=%v profileErr=%v", err, u.profileErr)
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

// A bundle-role read that fails used to be logged and treated as "belongs to no
// bundle": the compile carried on and wrote a cache holding only direct grants.
// The token minted from it is missing every role the person's bundles give
// them, and it stays that way for the life of the entry — a door that opened
// yesterday refuses them today because a database blip coincided with a login.
// Its two neighbours in this function (base grants, mapping rules) already fail
// the compile; there is no reason this read alone should decide, on its own, to
// answer a question it could not read.
func TestCompileUserCache_AnUnreadableBundleIsNotAnEmptyOne(t *testing.T) {
	resetCacheDeps(t)
	cached := setupNoopRedis(t)

	dbGetDirectGrantsForUser = func(_ context.Context, _ string, _ bool) ([]models.DirectGrant, error) {
		return []models.DirectGrant{{ProjectID: "proj1", RoleKey: "staff"}}, nil
	}
	dbGetActiveMappingRules = func(_ context.Context) ([]models.MappingRule, error) { return nil, nil }
	dbGetUserBundleRoles = func(_ context.Context, _ string) (map[string][]models.BundleRole, error) {
		return nil, errors.New("connection refused")
	}

	err := CompileUserCache(context.Background(), "user1", "proj1")
	if err == nil {
		t.Fatal("a cache was compiled from roles that could not be read")
	}
	if cached.value != "" {
		t.Fatalf("nothing may be written when the facts are incomplete, wrote %q", cached.value)
	}
}
