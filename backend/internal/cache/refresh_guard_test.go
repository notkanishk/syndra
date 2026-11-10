package cache

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
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

// The seams outside this package that refresh the cache: one per package, each
// bound to cache.Refresh (through whatever name the file imports this package
// as). to is "Refresh" for cache.Refresh, else the function a seam is bound to.
var seams = []struct{ file, name, to string }{
	{"../services/propagation/deps.go", "refreshClaims", "Refresh"},
	{"../services/service_accounts.go", "svcRefresh", "Refresh"},
	{"../handlers/deps.go", "cacheRefresh", "Refresh"},
	{"../services/expiry/deps.go", "cacheRefresh", "Refresh"},
	// The detached rebuild after a planned write: a seam over a handler
	// helper, which reaches cacheRefresh through refreshOrLog (below).
	{"../handlers/role_members.go", "rebuildUserCacheDetachedFn", "rebuildUserCacheDetached"},
}

// Every function on a write path that changes what facts carry, and the call
// in its own body that refreshes them (a seam, or the helper that calls one).
// Checked in that function's body, so a call elsewhere in the file, a comment
// or the function's own definition never satisfies it. A new write path is
// added here.
var writePaths = []struct{ file, fn, calls string }{
	{"../services/propagation/drain.go", "refreshAfterRow", "refreshClaims"},
	{"../services/service_accounts.go", "refreshServiceAccount", "svcRefresh"},
	{"../services/expiry/sweep.go", "processUser", "cacheRefresh"},
	{"../handlers/bundles.go", "refreshBundleHolders", "cacheRefresh"},
	{"../handlers/claim_delivery.go", "repair", "cacheRefresh"},
	{"../handlers/access.go", "refreshOrLog", "cacheRefresh"},
	{"../handlers/access.go", "rebuildUserCacheOrSkip", "refreshOrLog"},
	{"../handlers/access.go", "rebuildUserCacheDetached", "rebuildUserCacheOrSkip"},
	{"../handlers/webhook.go", "processGrantAdded", "refreshOrLog"},
	{"../handlers/webhook.go", "processGrantRemoved", "refreshOrLog"},
	{"../handlers/webhook.go", "processUserDeactivated", "refreshOrLog"},
	{"../handlers/bulk.go", "applyPlanned", "rebuildUserCacheDetachedFn"},
	{"../handlers/act.go", "removeGrantPlanned", "rebuildUserCacheDetachedFn"},
	{"../handlers/act.go", "bundlePlanned", "rebuildUserCacheDetachedFn"},
}

const cacheImport = "syndra/internal/cache"

func parseGo(t *testing.T, path string) *ast.File {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return f
}

// importName is the name f refers to this package by, "" when it doesn't
// import it.
func importName(f *ast.File) string {
	for _, imp := range f.Imports {
		if strings.Trim(imp.Path.Value, `"`) != cacheImport {
			continue
		}
		if imp.Name != nil {
			return imp.Name.Name
		}
		return "cache"
	}
	return ""
}

// boundTo is what name is assigned in f: a package-level var or an assignment
// anywhere in it; nil when it is never bound.
func boundTo(f *ast.File, name string) ast.Expr {
	var found ast.Expr
	ast.Inspect(f, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.ValueSpec:
			for i, id := range n.Names {
				if id.Name == name && i < len(n.Values) {
					found = n.Values[i]
				}
			}
		case *ast.AssignStmt:
			for i, lhs := range n.Lhs {
				if id, ok := lhs.(*ast.Ident); ok && id.Name == name && i < len(n.Rhs) {
					found = n.Rhs[i]
				}
			}
		}
		return true
	})
	return found
}

// callsIn says fn's own body calls name (a plain call, name(...)).
func callsIn(f *ast.File, fn, name string) (declared, calls bool) {
	for _, d := range f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Name.Name != fn || fd.Body == nil {
			continue
		}
		declared = true
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			if c, ok := n.(*ast.CallExpr); ok {
				if id, ok := c.Fun.(*ast.Ident); ok && id.Name == name {
					calls = true
				}
			}
			return !calls
		})
	}
	return declared, calls
}

func TestEachSeamIsBoundToTheOneRefresh(t *testing.T) {
	for _, s := range seams {
		f := parseGo(t, s.file)
		got := boundTo(f, s.name)
		ok := false
		switch e := got.(type) {
		case *ast.SelectorExpr: // cache.Refresh, under the file's own import name
			x, isIdent := e.X.(*ast.Ident)
			ok = s.to == "Refresh" && isIdent && x.Name == importName(f) && e.Sel.Name == "Refresh"
		case *ast.Ident:
			ok = s.to != "Refresh" && e.Name == s.to
		}
		if !ok {
			t.Errorf("%s: %s must be bound to %s, is %#v", s.file, s.name, s.to, got)
		}
	}
}

func TestEachWritePathCallsTheRefresh(t *testing.T) {
	for _, w := range writePaths {
		declared, calls := callsIn(parseGo(t, w.file), w.fn, w.calls)
		switch {
		case !declared:
			t.Errorf("%s: no function %s (moved or renamed? update writePaths)", w.file, w.fn)
		case !calls:
			t.Errorf("%s: %s no longer calls %s: a write path that skips the refresh leaves stale facts until the 12-hour warm", w.file, w.fn, w.calls)
		}
	}
}
