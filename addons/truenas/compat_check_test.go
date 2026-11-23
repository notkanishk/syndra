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
		{"a sent field's default", "user.create", func(e map[string]any) {
			property(e, "accepts", "shell", func(p map[string]any) { p["default"] = "/usr/bin/bash" })
		}},
		{"a docstring on a sent field", "user.update", func(e map[string]any) {
			property(e, "accepts", "locked", func(p map[string]any) { p["description"] = "Lock the account." })
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
		{"an unsent property's default on an update", "user.update", func(e map[string]any) {
			property(e, "accepts", "ssh_password_enabled", func(p map[string]any) { p["default"] = true })
		}, "accepts/1/ssh_password_enabled default: was absent, now [true]"},
		{"a flipped default in an unsent argument", "group.delete", func(e map[string]any) {
			property(e, "accepts", "delete_users", func(p map[string]any) { p["default"] = true })
		}, "accepts/1/delete_users default: was [false], now [true]"},
		{"an unsent optional argument made required", "group.delete", func(e map[string]any) {
			e["accepts"].([]any)[1].(map[string]any)["_required_"] = true
		}, "accepts/1 _required_: was absent, now true"},
		{"a property of an unsent argument made required", "group.delete", func(e map[string]any) {
			e["accepts"].([]any)[1].(map[string]any)["required"] = []any{"delete_users"}
		}, `accepts/1 required: was absent, now [["delete_users"]]`},
		{"what is sent into accepts more", "user.update", func(e map[string]any) {
			e["accepts"].([]any)[1].(map[string]any)["additionalProperties"] = true
		}, "accepts/1 additionalProperties: was [false], now [true]"},
		{"what is sent into an array item accepts more", "pool.dataset.set_quota", func(e map[string]any) {
			visit(e["accepts"], func(m map[string]any) {
				if props, ok := m["properties"].(map[string]any); ok && props["quota_type"] != nil {
					m["additionalProperties"] = true
				}
			})
		}, "accepts/1/0 additionalProperties: was [false], now [true]"},
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

// user.delete's unsent options are checked as group.delete's are (§4.2).
func TestAFlippedDeleteGroupDefaultIsChanged(t *testing.T) {
	v := checkEdited(t, elevatedDeps, "user.delete", func(e map[string]any) {
		property(e, "accepts", "delete_group", func(p map[string]any) { p["default"] = false })
	})
	if want := "accepts/1/delete_group default: was [true], now [false]"; v.State != compat.Changed || !strings.Contains(v.Detail, want) {
		t.Errorf("got %s (%s), want changed naming %q", v.State, v.Detail, want)
	}
}

// A relied-on field's enum is kept: a value dropped from it is a change.
func TestAReliedOnEnumChangeIsChanged(t *testing.T) {
	type share struct {
		Purpose string `json:"purpose"`
	}
	table := map[string]compat.Dep{"sharing.smb.query": {Replies: []any{[]share{}}}}
	v := checkEdited(t, table, "sharing.smb.query", func(e map[string]any) {
		property(e, "returns", "purpose", func(p map[string]any) {
			p["enum"] = []any{"DEFAULT_SHARE", "TIMEMACHINE_SHARE"}
		})
	})
	if v.State != compat.Changed || !strings.Contains(v.Detail, "returns/[]/purpose enum") {
		t.Errorf("got %s (%s), want changed naming the enum", v.State, v.Detail)
	}
}

// Reply types are read as encoding/json reads them: an embedded *T promotes
// T's fields, an embedded non-struct is a field named by its type, and an
// untagged or name-less tagged field is named by its Go name.
func TestReplyTypesAreReadAsEncodingJSONDoes(t *testing.T) {
	type Named struct {
		Username string `json:"username"`
	}
	type Smb bool
	type promoted struct{ *Named }
	type embeddedScalar struct{ Smb }
	type untagged struct{ Locked bool }
	type nameless struct {
		Builtin bool `json:",omitempty"`
	}
	for _, c := range []struct {
		reply any
		want  string // "" is unchanged
	}{
		{[]promoted{}, ""},
		{[]embeddedScalar{}, "returns/[]/Smb: was not declared"},
		{[]untagged{}, "returns/[]/Locked: was not declared"},
		{[]nameless{}, "returns/[]/Builtin: was not declared"},
	} {
		table := map[string]compat.Dep{"user.query": {Replies: []any{c.reply}}}
		v := checkEdited(t, table, "user.query", func(map[string]any) {})
		if c.want == "" && v.State != compat.Unchanged || c.want != "" && !strings.Contains(v.Detail, c.want) {
			t.Errorf("%T: %s (%s), want %q", c.reply, v.State, v.Detail, c.want)
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

// Every read the add-on makes is in deps, but the check's own reads and
// core.ping (nas.go): they run before there is a report, so nothing could
// hold them.
func TestDepsNameEveryRead(t *testing.T) {
	own := map[string]bool{"system.version": true, "auth.me": true, "core.get_methods": true, "core.ping": true}
	for m := range readMethods {
		if _, ok := deps[m]; !ok && !own[m] {
			t.Errorf("%s is read but not in deps", m)
		}
	}
}

// awaiting9e are the reads whose call sites decode only into inline anonymous
// structs, so nothing of their replies is checked yet. Slice 9e names those
// types, registers them in deps and empties this list; 9d must not ship
// before it (tasks.md 9b, 9d).
var awaiting9e = map[string]bool{
	"alert.list": true, "filesystem.getacl": true, "filesystem.listdir": true,
	"filesystem.stat": true, "pool.dataset.get_quota": true, "pool.dataset.query": true,
	"pool.query": true, "service.query": true, "system.info": true,
}

func TestEveryReadNamesItsReplies(t *testing.T) {
	for m, d := range deps {
		if readMethods[m] && len(d.Replies) == 0 && !awaiting9e[m] {
			t.Errorf("%s is read but no reply type is registered", m)
		}
	}
	for m := range awaiting9e {
		if len(deps[m].Replies) > 0 {
			t.Errorf("%s registers its replies: take it off awaiting9e", m)
		}
	}
}
