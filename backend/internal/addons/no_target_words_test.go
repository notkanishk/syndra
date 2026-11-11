package addons

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// The core forgets the names (addon-contract 2.5a, design §11): no TrueNAS
// field name is a string literal in the backend's generic code, which reads
// fields by role from targets.field_roles. Stricter than "at manifest-lookup
// sites": ANY such literal outside a struct tag fails, so a constant, a
// comparison or a map key cannot carry a name back in either. The words of
// the storage kind's own profile live in handlers/kinds/storage.go only.
//
// A repo-wide ban is unbuildable: "group" and "enabled" are also Zitadel's
// and the UI's own words. Outside the scanned directories they are not
// checked (zitadel/client.go, models/models.go); inside them, each legitimate
// use is allowlisted by file with its reason.
var targetWords = []string{"group", "home", "home_quota", "enabled", "smb_enabled", "smb"}

var targetWordAllowed = map[string]string{
	"handlers/kinds/storage.go\x00smb":   "the storage kind's view builds smb:// from connection.protocol (design §4)",
	"services/roles.go\x00group":         "a Zitadel role's display group, in its audit details",
	"services/insights_lens.go\x00group": "an insights lens kind (a group of roles), not a target field",
	"handlers/insights.go\x00group":      "the insights lens's own query parameter and kind",
	"handlers/claim_probe.go\x00enabled": "the claim probe's status JSON",
}

func TestNoTargetFieldNameIsALiteralInTheCore(t *testing.T) {
	used := map[string]bool{}
	for _, hit := range scanLiterals(t, func(file, value string) bool {
		for _, w := range targetWords {
			if value == w {
				_, ok := targetWordAllowed[file+"\x00"+w]
				used[file+"\x00"+w] = ok
				return !ok
			}
		}
		// The product's name: only the registry's configuration parsing may
		// spell it (and that in comments, ADDON_<TARGET>_* is generic).
		return strings.Contains(strings.ToLower(value), "truenas") && file != "addons/registry.go"
	}) {
		t.Errorf("%s: target word %q in the core; read the field by role (addons.Roles) or move it to its kind's view", hit[0], hit[1])
	}
	// An allowance nothing uses is one a later literal could hide behind.
	for key, why := range targetWordAllowed {
		if !used[key] {
			t.Errorf("allowlisted %q (%s) no longer occurs: remove it", strings.ReplaceAll(key, "\x00", " "), why)
		}
	}
}

// scanLiterals walks the non-test Go files under services/, handlers/ and
// addons/ (drift is services/drift) and returns every string literal outside
// a struct tag for which bad says so, as file:line and value.
func scanLiterals(t *testing.T, bad func(file, value string) bool) [][2]string {
	t.Helper()
	root := filepath.Join("..")
	var hits [][2]string
	for _, dir := range []string{"services", "handlers", "addons"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			rel, _ := filepath.Rel(root, path)
			rel = filepath.ToSlash(rel)
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
			if err != nil {
				return err
			}
			tags := map[*ast.BasicLit]bool{}
			ast.Inspect(f, func(n ast.Node) bool {
				if field, ok := n.(*ast.Field); ok && field.Tag != nil {
					tags[field.Tag] = true
				}
				return true
			})
			ast.Inspect(f, func(n ast.Node) bool {
				lit, ok := n.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING || tags[lit] {
					return true
				}
				if v, err := strconv.Unquote(lit.Value); err == nil && bad(rel, v) {
					hits = append(hits, [2]string{rel + ":" + strconv.Itoa(fset.Position(lit.Pos()).Line), v})
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatalf("the guard could not read %s, so it is watching nothing: %v", dir, err)
		}
	}
	return hits
}
