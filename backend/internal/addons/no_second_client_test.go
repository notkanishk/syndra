package addons

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// The boundary guard (addon-contract 2.5b, design §11): an HTTP client or a
// raw dial is constructed under backend/internal only in these files, each
// with its reason; no line numbers, they go stale. It GUARDS "Syndra talks
// only to add-ons (and Zitadel)", it cannot prove it: a scan cannot see where
// an operator-set URL points. There is no Google client.
var clientAllowed = map[string]string{
	"addons/transport.go":     "the pipe to every add-on",
	"addons/credentials.go":   "the add-on's pinned-TLS client (newAddonClient)",
	"zitadel/client.go":       "the Zitadel Management API client",
	"zitadel/token.go":        "the Zitadel service-account token exchange",
	"zitadel/deps.go":         "the Zitadel client's injectable transport",
	"auth/jwt.go":             "Zitadel's JWKS, to verify tokens",
	"handlers/deps.go":        "the claim probe's client",
	"handlers/claim_probe.go": "the claim probe against Zitadel",
	"handlers/claim_watch.go": "the dead-man ping the owner configures",
}

// isClientConstructor reports a selector that makes a client, sends a request
// or dials, by the IMPORT PATH its package identifier resolves to, so a
// renamed import (`nethttp "net/http"`) is caught: http.Client and
// http.Transport (constructed or named), the package-level helpers and
// DefaultClient, every net.Dial* and net.Dialer, and tls.Dial*.
func isClientConstructor(importPath, name string) bool {
	switch importPath {
	case "net/http":
		switch name {
		case "Client", "Transport", "NewRequest", "NewRequestWithContext", "DefaultClient", "DefaultTransport", "Get", "Post", "PostForm", "Head":
			return true
		}
	case "net", "crypto/tls":
		return strings.HasPrefix(name, "Dial")
	}
	return false
}

// importsOf maps each package identifier a file uses to its import path: the
// alias when one is given, else the path's last element (the convention for
// the three packages this guard watches).
func importsOf(f *ast.File) map[string]string {
	out := map[string]string{}
	for _, spec := range f.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		name := path[strings.LastIndex(path, "/")+1:]
		if spec.Name != nil {
			name = spec.Name.Name
		}
		out[name] = path
	}
	return out
}

// clientsIn lists the client constructions in one file's source.
func clientsIn(t *testing.T, path string, src any) []string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("the guard could not parse %s, so it is watching nothing: %v", path, err)
	}
	imports := importsOf(f)
	var found []string
	ast.Inspect(f, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if pkg, ok := sel.X.(*ast.Ident); ok && isClientConstructor(imports[pkg.Name], sel.Sel.Name) {
			found = append(found, fmt.Sprintf("%s: %s.%s", fset.Position(sel.Pos()), imports[pkg.Name], sel.Sel.Name))
		}
		return true
	})
	return found
}

func TestNoSecondClientIsConstructed(t *testing.T) {
	root := filepath.Join("..")
	used := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		for _, hit := range clientsIn(t, path, nil) {
			if _, allowed := clientAllowed[rel]; allowed {
				used[rel] = true
				continue
			}
			t.Errorf("%s outside the allowlist; Syndra talks to add-ons through the pipe (addons/transport.go), or this file needs an entry with its reason", hit)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("the guard could not read backend/internal, so it is watching nothing: %v", err)
	}
	for file, why := range clientAllowed {
		if !used[file] {
			t.Errorf("%s (%s) constructs no client any more: remove it from the allowlist", file, why)
		}
	}
}

// The guard resolves a package identifier through the file's imports: a
// renamed import is still caught, tls.Dial is a dial, and a local value that
// happens to be called `http` is not the package.
func TestTheClientGuardSeesThroughARenamedImport(t *testing.T) {
	src := `package x

import (
	nethttp "net/http"
	"crypto/tls"
)

type fake struct{ Client int }

func f() {
	_ = &nethttp.Client{}
	_, _ = tls.Dial("tcp", "x:443", nil)
	http := fake{}
	_ = http.Client
}
`
	got := clientsIn(t, "x.go", src)
	if len(got) != 2 || !strings.Contains(got[0], "net/http.Client") || !strings.Contains(got[1], "crypto/tls.Dial") {
		t.Fatalf("want the renamed http.Client and tls.Dial, and not the local value: %v", got)
	}
}
