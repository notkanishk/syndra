package cache

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRefresh_EachPersonOnceAndOneFailureDoesNotStopTheRest(t *testing.T) {
	orig := refreshOne
	t.Cleanup(func() { refreshOne = orig })
	var got []string
	refreshOne = func(_ context.Context, id string, p ...string) error {
		got = append(got, id+":"+strings.Join(p, ","))
		if id == "a" {
			return errors.New("redis is down")
		}
		return nil
	}
	err := Refresh(context.Background(), []string{"a", "", "b", "a"}, "p1")
	if err == nil || !strings.Contains(err.Error(), "a: redis is down") {
		t.Fatalf("the failure must be reported with who it was for: %v", err)
	}
	if strings.Join(got, " ") != "a:p1 b:p1" {
		t.Fatalf("each person once, blanks skipped, the rest still refreshed: %v", got)
	}
}

// Every write path that changes what facts carry routes through Refresh. A new
// path is added here; deleting a listed path's call fails this test.
// file -> {the seam bound to cache.Refresh, a call of it}
var writePaths = map[string][2]string{
	"../services/propagation/drain.go": {"deps.go", "refreshClaims("},
	"../services/service_accounts.go":  {"", "svcRefresh("},
	"../handlers/bundles.go":           {"", "bundleRefresh("},
	"../handlers/webhook.go":           {"deps.go", "cacheRebuildUser("},
	"../handlers/access.go":            {"deps.go", "cacheRebuildUser("},
	"../handlers/bulk.go":              {"deps.go", "rebuildUserCacheDetachedFn("},
	"../handlers/role_members.go":      {"deps.go", "rebuildUserCacheDetachedFn"},
	"../handlers/claim_delivery.go":    {"deps.go", "cacheRefreshUser("},
	"../services/expiry/sweep.go":      {"deps.go", "cacheRefreshUser("},
}

func TestWritePathsCallTheOneRefresh(t *testing.T) {
	for path, w := range writePaths {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if !strings.Contains(string(b), w[1]) {
			t.Errorf("%s no longer calls %s: a write path that skips the refresh leaves stale facts", path, w[1])
		}
		bind := path
		if w[0] != "" {
			bind = filepath.Join(filepath.Dir(path), w[0])
		}
		bb, _ := os.ReadFile(bind)
		if !strings.Contains(string(bb), "cache.Refresh") {
			t.Errorf("%s must bind its seam to cache.Refresh", bind)
		}
	}
}

// cache.Refresh is the only way outside this package to refresh the cache.
// There is no allow-list: a new raw refresh fails here.
func TestNoRawRefreshOutsideThePackage(t *testing.T) {
	err := filepath.Walk("..", func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return err
		}
		if strings.HasPrefix(p, "../cache/") {
			return nil
		}
		b, _ := os.ReadFile(p)
		for _, raw := range []string{"cache.RefreshUser", "cache.RebuildUserCache"} {
			if strings.Contains(string(b), raw) {
				t.Errorf("%s uses %s directly: use cache.Refresh", p, raw)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
