package main

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strconv"
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
		{"what an unsent argument accepts", "group.delete", func(e map[string]any) {
			e["accepts"].([]any)[1].(map[string]any)["additionalProperties"] = true
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

// schemaKeywords are the keys a captured schema may hold outside a
// `properties` map: JSON Schema 2020-12's Core, Applicator, Validation,
// Format-annotation and Meta-data vocabularies (json-schema.org/draft/2020-12/
// json-schema-validation §6, §7, §9), OpenAPI's `discriminator` and `nullable`,
// which pydantic may emit, and TrueNAS's own `_attrs_order_`, `_name_`,
// `_required_`. The Meta-data prose, `description` and `examples`, and Core's
// `$comment` are not here: the recorder drops them (design §4.1, slice 9a).
var schemaKeywords = map[string]bool{
	"type": true, "properties": true, "items": true, "prefixItems": true,
	"additionalProperties": true, "patternProperties": true, "propertyNames": true,
	"required": true, "anyOf": true, "oneOf": true, "allOf": true, "not": true,
	"enum": true, "const": true, "default": true, "format": true, "title": true,
	"minimum": true, "maximum": true, "exclusiveMinimum": true, "exclusiveMaximum": true,
	"multipleOf": true, "minLength": true, "maxLength": true, "pattern": true,
	"minItems": true, "maxItems": true, "uniqueItems": true,
	"minProperties": true, "maxProperties": true, "deprecated": true,
	"readOnly": true, "writeOnly": true, "discriminator": true, "nullable": true,
	"$ref": true, "$defs": true,
	"_attrs_order_": true, "_name_": true, "_required_": true,
}

// notSchema names each key in s that is not a schema keyword. Keys of a
// `properties`, `$defs` or `patternProperties` map are names, and default,
// enum, const and discriminator hold values, so neither is walked as a keyword.
// The recorder walks the same way (VALUES and NAMES in record-truenas-methods.py).
func notSchema(at string, s any) []string {
	var bad []string
	switch x := s.(type) {
	case []any:
		for i, c := range x {
			bad = append(bad, notSchema(at+"/"+strconv.Itoa(i), c)...)
		}
	case map[string]any:
		for k, v := range x {
			switch {
			case !schemaKeywords[k]:
				bad = append(bad, fmt.Sprintf("%s: %q is not a schema keyword", at, k))
			case k == "properties" || k == "$defs" || k == "patternProperties":
				if names, ok := v.(map[string]any); ok {
					for n, c := range names {
						bad = append(bad, notSchema(at+"/"+k+"/"+n, c)...)
					}
				}
			case k == "default" || k == "enum" || k == "const" || k == "discriminator":
			default:
				bad = append(bad, notSchema(at+"/"+k, v)...)
			}
		}
	}
	return bad
}

// A harmless keyword pydantic may emit passes; prose and the entry's other
// keys never do, at any depth.
func TestTheSchemaKeywordsAreTheRecordersSet(t *testing.T) {
	ok := map[string]any{"type": "object", "readOnly": true, "writeOnly": false, "nullable": true,
		"deprecated": false, "format": "date-time", "pattern": "^x$", "const": map[string]any{"description": 1},
		"discriminator": map[string]any{"propertyName": "kind", "mapping": map[string]any{"description": "#/$defs/a"}},
		"properties":    map[string]any{"description": map[string]any{"type": "string", "minLength": 1}}}
	if bad := notSchema("", ok); len(bad) > 0 {
		t.Errorf("refused: %v", bad)
	}
	for _, k := range []string{"description", "examples", "$comment", "roles", "cli_description"} {
		nested := map[string]any{"type": "array", "items": map[string]any{"type": "string", k: "The Undertaker"}}
		if bad := notSchema("", nested); len(bad) != 1 {
			t.Errorf("%s: got %v, want it refused", k, bad)
		}
	}
}

// Slice 9a: every capture holds exactly the tables' methods, each with
// accepts, returns and job and nothing else, and its schemas hold schema
// keywords, property names and types only.
func TestTheCaptureHoldsOnlyWhatTheCheckReads(t *testing.T) {
	baselines, err := compat.Baselines()
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range baselines {
		want := slices.Sorted(maps.Keys(deps))
		want = append(want, slices.Sorted(maps.Keys(elevatedDeps))...)
		slices.Sort(want)
		if got := slices.Sorted(maps.Keys(b.Methods)); !slices.Equal(got, want) {
			t.Errorf("%s methods: %v, want deps and elevatedDeps: %v", b.Version, got, want)
		}
		for name, raw := range b.Methods {
			var entry map[string]any
			if err := json.Unmarshal(raw, &entry); err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			if got := slices.Sorted(maps.Keys(entry)); !slices.Equal(got, []string{"accepts", "job", "returns"}) {
				t.Errorf("%s holds %v, want accepts, job, returns", name, got)
			}
			if _, ok := entry["job"].(bool); !ok {
				t.Errorf("%s: job is not a bool", name)
			}
			for _, k := range []string{"accepts", "returns"} {
				if _, ok := entry[k].([]any); !ok {
					t.Errorf("%s: %s is not a list", name, k)
				}
				for _, e := range notSchema(name+"/"+k, entry[k]) {
					t.Error(e)
				}
			}
		}
	}
}
