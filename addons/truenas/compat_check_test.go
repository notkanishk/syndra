package main

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"syndra/addons/truenas/compat"
)

// The version check's verdicts (design-version-check.md §4, slice 9b), each
// from the shipped capture with one edit, through the add-on's own tables:
// the relied-on fields come from the Go reply types in deps, never a list.

// checkEdited runs Check with the capture as the NAS's answer, one entry
// edited first. A nil edit deletes the entry.
func checkEdited(t *testing.T, table map[string]compat.Dep, method string, edit func(entry map[string]any)) compat.Verdict {
	t.Helper()
	baselines, err := compat.Baselines()
	if err != nil {
		t.Fatal(err)
	}
	methods := map[string]json.RawMessage{}
	for name, raw := range baselines[0].Methods {
		methods[name] = raw
	}
	if edit == nil {
		delete(methods, method)
	} else {
		var entry map[string]any
		if err := json.Unmarshal(methods[method], &entry); err != nil {
			t.Fatalf("%s: %v", method, err)
		}
		edit(entry)
		raw, err := json.Marshal(entry)
		if err != nil {
			t.Fatal(err)
		}
		methods[method] = raw
	}
	return compat.Check(baselines, table, methods).Verdict(method)
}

// visit calls f on every object in a JSON tree.
func visit(v any, f func(map[string]any)) {
	switch x := v.(type) {
	case map[string]any:
		f(x)
		for _, c := range x {
			visit(c, f)
		}
	case []any:
		for _, c := range x {
			visit(c, f)
		}
	}
}

// property edits every schema in the tree under key that declares the named
// property.
func property(entry map[string]any, key, name string, f func(prop map[string]any)) {
	visit(entry[key], func(m map[string]any) {
		if props, ok := m["properties"].(map[string]any); ok {
			if p, ok := props[name].(map[string]any); ok {
				f(p)
			}
		}
	})
}

func TestTheCaptureCheckedAgainstItselfIsUnchanged(t *testing.T) {
	baselines, err := compat.Baselines()
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range []map[string]compat.Dep{deps, elevatedDeps} {
		report := compat.Check(baselines, table, baselines[0].Methods)
		for name := range table {
			if v := report.Verdict(name); v.State != compat.Unchanged {
				t.Errorf("%s: %s (%s); every path a reply type reads must resolve in the capture", name, v.State, v.Detail)
			}
		}
	}
}

func TestEditsNothingReliesOnAreUnchanged(t *testing.T) {
	for _, c := range []struct {
		name   string
		method string
		edit   func(map[string]any)
	}{
		{"a docstring", "user.query", func(e map[string]any) {
			e["description"] = "Query users, now with more words."
			visit(e, func(m map[string]any) {
				if d, ok := m["description"].(string); ok {
					m["description"] = d + " Reworded."
				}
			})
		}},
		{"an unrelated new property", "user.query", func(e map[string]any) {
			visit(e["returns"], func(m map[string]any) {
				if props, ok := m["properties"].(map[string]any); ok && props["username"] != nil {
					props["ring_name"] = map[string]any{"type": "string"}
				}
			})
		}},
		{"the FIXME keys removed", "user.create", func(e map[string]any) {
			visit(e, func(m map[string]any) {
				for _, k := range []string{"_attrs_order_", "_name_", "_required_", "title"} {
					if _, isSchema := m[k].(map[string]any); !isSchema {
						delete(m, k)
					}
				}
				if items, ok := m["items"].([]any); ok && len(items) == 1 {
					m["items"] = items[0]
				}
			})
		}},
		{"every anyOf and required reordered", "user.create", func(e map[string]any) {
			visit(e, func(m map[string]any) {
				for _, k := range []string{"anyOf", "required"} {
					if l, ok := m[k].([]any); ok {
						slices.Reverse(l)
					}
				}
			})
		}},
		{"a new argument to a read", "user.query", func(e map[string]any) {
			e["accepts"] = append(e["accepts"].([]any), map[string]any{"type": "boolean", "title": "fresh"})
		}},
		{"an unsent property's default on an update", "user.update", func(e map[string]any) {
			property(e, "accepts", "ssh_password_enabled", func(p map[string]any) { p["default"] = true })
		}},
	} {
		if v := checkEdited(t, deps, c.method, c.edit); v.State != compat.Unchanged {
			t.Errorf("%s: %s (%s), want unchanged", c.name, v.State, v.Detail)
		}
	}
}

func TestEditsSomethingReliesOnAreChangedWithThePath(t *testing.T) {
	for _, c := range []struct {
		name   string
		method string
		edit   func(map[string]any)
		path   string
	}{
		{"a relied-on type", "user.query", func(e map[string]any) {
			property(e, "returns", "smb", func(p map[string]any) { p["type"] = "string" })
		}, "returns/[]/smb type: was boolean, now string"},
		{"a field made nullable", "user.query", func(e map[string]any) {
			property(e, "returns", "home", func(p map[string]any) {
				was := map[string]any{}
				for k, v := range p {
					was[k] = v
					delete(p, k)
				}
				p["anyOf"] = []any{was, map[string]any{"type": "null"}}
			})
		}, "returns/[]/home type: was string, now null or string"},
		{"a sent field's type", "user.update", func(e map[string]any) {
			property(e, "accepts", "locked", func(p map[string]any) { p["type"] = "string" })
		}, "accepts/1/locked"},
		{"a new required argument", "user.update", func(e map[string]any) {
			e["accepts"] = append(e["accepts"].([]any), map[string]any{"type": "boolean", "title": "force"})
		}, "accepts: was 2, now 3"},
		{"a new required field in what is sent", "user.update", func(e map[string]any) {
			visit(e["accepts"], func(m map[string]any) {
				if props, ok := m["properties"].(map[string]any); ok && props["locked"] != nil {
					m["required"] = []any{"locked", "attributes"}
				}
			})
		}, "accepts/1 required"},
		{"a changed default on an unsent user.create property", "user.create", func(e map[string]any) {
			property(e, "accepts", "random_password", func(p map[string]any) { p["default"] = true })
		}, "accepts/0/random_password default: was [false], now [true]"},
		{"a flipped job", "user.update", func(e map[string]any) { e["job"] = true }, "job: was false, now true"},
		{"an entry without returns", "user.update", func(e map[string]any) { delete(e, "returns") }, "returns"},
		{"user.query without its list branch", "user.query", func(e map[string]any) {
			returns := e["returns"].([]any)[0].(map[string]any)
			var kept []any
			for _, b := range returns["anyOf"].([]any) {
				if b.(map[string]any)["type"] != "array" {
					kept = append(kept, b)
				}
			}
			returns["anyOf"] = kept
		}, "returns type: was array or integer or object, now integer or object"},
	} {
		v := checkEdited(t, deps, c.method, c.edit)
		if v.State != compat.Changed || !strings.Contains(v.Detail, c.path) {
			t.Errorf("%s: %s (%s), want changed naming %q", c.name, v.State, v.Detail, c.path)
		}
	}
}

// A reply type naming a field TrueNAS never declared is never unchanged: the
// struct and the schema disagree before any NAS is asked.
func TestAFieldNoSchemaDeclaresIsChanged(t *testing.T) {
	type wrestler struct {
		RingName string `json:"ring_name"`
	}
	table := map[string]compat.Dep{"user.query": {Replies: []any{[]wrestler{}}}}
	if v := checkEdited(t, table, "user.query", func(map[string]any) {}); v.State != compat.Changed ||
		!strings.Contains(v.Detail, "returns/[]/ring_name: was not declared, now not declared") {
		t.Errorf("got %s (%s), want changed naming the field", v.State, v.Detail)
	}
}

func TestARemovedEntryIsMissing(t *testing.T) {
	if v := checkEdited(t, deps, "audit.query", nil); v.State != compat.Missing {
		t.Errorf("got %s, want missing", v.State)
	}
}

func TestNoMethodsIsUnread(t *testing.T) {
	baselines, err := compat.Baselines()
	if err != nil {
		t.Fatal(err)
	}
	report := compat.Check(baselines, deps, nil)
	for name := range deps {
		if v := report.Verdict(name); v.State != compat.Unread {
			t.Errorf("%s: %s, want unread", name, v.State)
		}
	}
}

// The add-on's key cannot see user.delete (design §3.1), so it is checked
// only on the elevated session, against its own table.
func TestUserDeleteIsCheckedOnlyUnderElevatedDeps(t *testing.T) {
	if v := checkEdited(t, elevatedDeps, "user.delete", func(map[string]any) {}); v.State != compat.Unchanged {
		t.Errorf("under elevatedDeps: %s (%s), want unchanged", v.State, v.Detail)
	}
	if v := checkEdited(t, deps, "user.delete", func(map[string]any) {}); v != (compat.Verdict{}) {
		t.Errorf("under deps: %+v, want absent from the report", v)
	}
}

// Every read the add-on makes is in deps, but the check's own three: they run
// before there is a report, so nothing could hold them.
func TestDepsNameEveryRead(t *testing.T) {
	own := map[string]bool{"system.version": true, "auth.me": true, "core.get_methods": true}
	for m := range readMethods {
		if _, ok := deps[m]; !ok && !own[m] {
			t.Errorf("%s is read but not in deps", m)
		}
	}
}
