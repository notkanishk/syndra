package addons

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
)

// needSpec is a hand copy of what each result schema requires. This walks the
// schemas themselves (following $ref, allOf and items) and compares every
// path's required keys with it, so the copy cannot drift from the contract.

// required maps a path ("" is the root; "records.standing" is a record's
// standing) to the keys the schema requires there.
func schemaRequired(t *testing.T, file string, node map[string]any, path string, out map[string][]string) {
	t.Helper()
	if ref, ok := node["$ref"].(string); ok {
		name, frag, _ := strings.Cut(ref, "#")
		if name != "" {
			file = filepath.Join(filepath.Dir(file), name)
		}
		node = loadSchemaNode(t, file, frag)
	}
	if req, ok := node["required"].([]any); ok {
		for _, k := range req {
			out[path] = append(out[path], k.(string))
		}
	}
	if all, ok := node["allOf"].([]any); ok {
		for _, sub := range all {
			schemaRequired(t, file, sub.(map[string]any), path, out)
		}
	}
	if items, ok := node["items"].(map[string]any); ok {
		schemaRequired(t, file, items, path, out)
	}
	props, _ := node["properties"].(map[string]any)
	for name, sub := range props {
		next := name
		if path != "" {
			next = path + "." + name
		}
		schemaRequired(t, file, sub.(map[string]any), next, out)
	}
}

func loadSchemaNode(t *testing.T, file, pointer string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var node any
	if err := json.Unmarshal(raw, &node); err != nil {
		t.Fatal(err)
	}
	for _, part := range strings.Split(strings.TrimPrefix(pointer, "/"), "/") {
		if part == "" {
			continue
		}
		node = node.(map[string]any)[part]
	}
	return node.(map[string]any)
}

func specRequired(spec needSpec, path string, out map[string][]string) {
	if len(spec.keys) > 0 {
		out[path] = slices.Clone(spec.keys)
	}
	for name, sub := range spec.nested {
		next := name
		if path != "" {
			next = path + "." + name
		}
		specRequired(sub, next, out)
	}
}

func TestNeedSpecMatchesTheSchemas(t *testing.T) {
	for file, shape := range shapes {
		t.Run(file, func(t *testing.T) {
			path := filepath.Join(v2Dir, "schema", file)
			want := map[string][]string{}
			schemaRequired(t, path, loadSchemaNode(t, path, ""), "", want)
			got := map[string][]string{}
			specRequired(shape.need, "", got)
			for _, m := range []map[string][]string{want, got} {
				for k := range m {
					sort.Strings(m[k])
					m[k] = slices.Compact(m[k])
				}
			}
			for p, w := range want {
				if !slices.Equal(w, got[p]) {
					t.Errorf("%q: schema requires %v, needSpec says %v", p, w, got[p])
				}
			}
			for p, g := range got {
				if _, ok := want[p]; !ok {
					t.Errorf("%q: needSpec requires %v, schema requires nothing there", p, g)
				}
			}
		})
	}
}

func TestAStaleWithoutAReasonIsNotAnAnswer(t *testing.T) {
	good := `{"records":[],"taken_at":"2026-11-10T09:30:00Z","current":false,"stale":{"reason":"cached"}}`
	if err := recordsNeed.check(json.RawMessage(good)); err != nil {
		t.Fatalf("a stale with its reason: %v", err)
	}
	bad := strings.Replace(good, `"reason":"cached"`, `"note":"x"`, 1)
	if err := recordsNeed.check(json.RawMessage(bad)); err == nil {
		t.Fatal("a stale with no reason was accepted")
	}
}
