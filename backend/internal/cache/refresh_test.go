package cache

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"syndra/internal/models"
)

// fakeRedis swaps the three Redis seams for an in-memory map.
func fakeRedis(t *testing.T) map[string]string {
	t.Helper()
	origSet, origDel, origScan := redisSet, redisDel, redisScanKeys
	t.Cleanup(func() { redisSet, redisDel, redisScanKeys = origSet, origDel, origScan })
	kv := map[string]string{}
	redisSet = func(_ context.Context, key, value string, _ time.Duration) error {
		kv[key] = value
		return nil
	}
	redisDel = func(_ context.Context, keys ...string) error {
		for _, k := range keys {
			delete(kv, k)
		}
		return nil
	}
	redisScanKeys = func(_ context.Context, pattern string) ([]string, error) {
		prefix := strings.TrimSuffix(pattern, "*")
		var keys []string
		for k := range kv {
			if strings.HasPrefix(k, prefix) {
				keys = append(keys, k)
			}
		}
		return keys, nil
	}
	return kv
}

func cachedRoles(t *testing.T, kv map[string]string, key string) []string {
	t.Helper()
	raw, ok := kv[key]
	if !ok {
		return nil
	}
	var f struct{ Roles []string }
	if err := json.Unmarshal([]byte(raw), &f); err != nil {
		t.Fatalf("parse %s: %v", key, err)
	}
	return f.Roles
}

func holding(grants ...models.DirectGrant) {
	dbGetDirectGrantsForUser = func(context.Context, string, bool) ([]models.DirectGrant, error) { return grants, nil }
	dbGetActiveMappingRules = func(context.Context) ([]models.MappingRule, error) { return nil, nil }
}

// The token path serves only what is stored, so a change has to leave the new
// claims in place, not a missing key. A grant in one project must also leave
// the user's other projects alone, and a revoke must take the role out of the
// stored claims while the rest stay.
func TestRefreshUser_RecompilesInsteadOfInvalidating(t *testing.T) {
	resetCacheDeps(t)
	kv := fakeRedis(t)
	ctx := context.Background()

	holding(
		models.DirectGrant{ProjectID: "pA", RoleKey: "viewer"},
		models.DirectGrant{ProjectID: "pB", RoleKey: "editor"},
		models.DirectGrant{ProjectID: "pB", RoleKey: "admin"},
	)
	if err := RefreshUser(ctx, "u1"); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if got := cachedRoles(t, kv, "mapping:u1:pA"); len(got) != 1 {
		t.Fatalf("a held project must be compiled without being named: %v", got)
	}

	// A new grant in pC: naming only pC must not disturb pA or pB.
	holding(
		models.DirectGrant{ProjectID: "pA", RoleKey: "viewer"},
		models.DirectGrant{ProjectID: "pB", RoleKey: "editor"},
		models.DirectGrant{ProjectID: "pB", RoleKey: "admin"},
		models.DirectGrant{ProjectID: "pC", RoleKey: "member"},
	)
	if err := RefreshUser(ctx, "u1", "pC"); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	for key, want := range map[string]int{"mapping:u1:pA": 1, "mapping:u1:pB": 2, "mapping:u1:pC": 1} {
		if got := cachedRoles(t, kv, key); len(got) != want {
			t.Errorf("%s after a grant in pC: want %d roles, got %v", key, want, got)
		}
	}

	// Revoke admin in pB and the only role in pA.
	holding(
		models.DirectGrant{ProjectID: "pB", RoleKey: "editor"},
		models.DirectGrant{ProjectID: "pC", RoleKey: "member"},
	)
	if err := RefreshUser(ctx, "u1", "pB"); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if got := cachedRoles(t, kv, "mapping:u1:pB"); len(got) != 1 || got[0] != "editor" {
		t.Errorf("the revoked role must be gone and the other kept: %v", got)
	}
	if _, stale := kv["mapping:u1:pA"]; stale {
		t.Errorf("a project the user holds nothing in must not keep serving its old roles")
	}
	if got := cachedRoles(t, kv, "mapping:u1:pC"); len(got) != 1 {
		t.Errorf("an untouched project must survive: %v", got)
	}
}

// A compile that fails must not leave the old claims in place: they may hold
// the role that was just revoked. The data plane's degraded path is the safe
// answer.
func TestRefreshUser_ACompileFailureDropsWhatItCouldNotRefresh(t *testing.T) {
	resetCacheDeps(t)
	kv := fakeRedis(t)
	kv["mapping:u1:pA"] = `{"roles":["admin"]}`
	kv["mapping:u2:pA"] = `{"roles":["admin"]}`
	dbGetDirectGrantsForUser = func(context.Context, string, bool) ([]models.DirectGrant, error) {
		return nil, errors.New("db down")
	}

	if err := RefreshUser(context.Background(), "u1", "pA"); err == nil {
		t.Fatal("a failed compile must be reported")
	}
	if _, ok := kv["mapping:u1:pA"]; ok {
		t.Error("the unrefreshable key kept serving")
	}
	if _, ok := kv["mapping:u2:pA"]; !ok {
		t.Error("another user's key was touched")
	}
}

// The simulator must compute without writing: a preview that stored its own
// answer would repair the one project being looked at and hide a stale cache.
func TestCompileFacts_WritesNothing(t *testing.T) {
	resetCacheDeps(t)
	kv := fakeRedis(t)
	holding(models.DirectGrant{ProjectID: "pA", RoleKey: "viewer"})

	facts, err := CompileFacts(context.Background(), "u1", "pA")
	if err != nil || len(facts.Roles) != 1 {
		t.Fatalf("facts=%+v err=%v", facts, err)
	}
	if len(kv) != 0 {
		t.Errorf("a preview wrote to the token cache: %v", kv)
	}
}

// Warm covers everyone Syndra knows, so a restart does not leave the token
// path degraded for people whose access never changed.
func TestWarm_CompilesEveryKnownUser(t *testing.T) {
	resetCacheDeps(t)
	kv := fakeRedis(t)
	orig := dbGetAllKnownUserIDs
	t.Cleanup(func() { dbGetAllKnownUserIDs = orig })
	dbGetAllKnownUserIDs = func(context.Context) ([]string, error) { return []string{"u1", "u2"}, nil }
	holding(models.DirectGrant{ProjectID: "pA", RoleKey: "viewer"})

	if err := Warm(context.Background()); err != nil {
		t.Fatalf("warm: %v", err)
	}
	if len(cachedRoles(t, kv, "mapping:u1:pA")) != 1 || len(cachedRoles(t, kv, "mapping:u2:pA")) != 1 {
		t.Errorf("not everyone was warmed: %v", kv)
	}
}
