// Package compat is the version check's pure core (design-version-check.md
// §3, §4): it compares the parts of TrueNAS's method schemas the add-on relies
// on, as the NAS answers `core.get_methods`, with the same parts as captured on
// a tested version. No network, no store, no clock.
package compat

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"maps"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
)

//go:embed baseline/*.methods.json
var shipped embed.FS

// Baseline is one tested version's capture: `{version, methods: {name:
// {accepts, returns, job}}}`, as the NAS answered. Synthetic marks a stand-in
// written before the real capture; only tests may load one.
type Baseline struct {
	Synthetic bool                       `json:"_synthetic"`
	Version   string                     `json:"version"`
	Methods   map[string]json.RawMessage `json:"methods"`
}

// Baselines are the captures shipped in the binary. An error is a build fault:
// the add-on refuses to start (design §8), so a synthetic capture cannot ship.
// testing.Testing is the standard library's own "is this a test binary".
func Baselines() ([]Baseline, error) { return load(shipped, testing.Testing()) }

func load(fsys fs.FS, synthetic bool) ([]Baseline, error) {
	names, err := fs.Glob(fsys, "baseline/*.methods.json")
	if err != nil {
		return nil, err
	}
	var out []Baseline
	for _, name := range names {
		raw, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil, err
		}
		var b Baseline
		switch err := json.Unmarshal(raw, &b); {
		case err != nil:
			return nil, fmt.Errorf("%s is not a capture: %w", name, err)
		case b.Version == "" || len(b.Methods) == 0:
			return nil, fmt.Errorf("%s is not a capture: no version or no methods", name)
		case b.Synthetic && !synthetic:
			return nil, fmt.Errorf("%s is synthetic: only tests may load it; ship the real capture (slice 9a)", name)
		}
		out = append(out, b)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no capture is shipped")
	}
	return out, nil
}

// Dep is what Syndra relies on in one TrueNAS method.
type Dep struct {
	Replies []any    // zero values of every Go type a call site decodes into
	Sends   []string // JSON Pointers (RFC 6901) into the arguments Syndra sends
}

// State is a method's verdict (§4.4).
type State string

const (
	Unchanged State = "unchanged"
	Changed   State = "changed"
	Missing   State = "missing"
	Unread    State = "unread" // no method list was read: every method at once
)

// Verdict is one method's state; Detail names each changed path, "was / now".
type Verdict struct {
	State  State
	Detail string
}

// Report holds a verdict for every method of the table it was checked with,
// and none for any other.
type Report map[string]Verdict

func (r Report) Verdict(method string) Verdict { return r[method] }

// Check compares each method of deps, as methods (the NAS's answer, nil when
// none was read) has it, with every baseline: unchanged when it equals any.
func Check(baselines []Baseline, deps map[string]Dep, methods map[string]json.RawMessage) Report {
	r := make(Report, len(deps))
	for name, dep := range deps {
		r[name] = verdict(name, dep, baselines, methods)
	}
	return r
}

func verdict(name string, dep Dep, baselines []Baseline, methods map[string]json.RawMessage) Verdict {
	if methods == nil {
		return Verdict{State: Unread}
	}
	entry, ok := methods[name]
	if !ok {
		return Verdict{State: Missing}
	}
	now, err := project(entry, dep)
	if err != nil {
		return Verdict{Changed, err.Error()}
	}
	nowBytes, _ := json.Marshal(now)
	var was map[string]any
	for _, b := range baselines {
		raw, ok := b.Methods[name]
		if !ok {
			continue
		}
		p, err := project(raw, dep)
		if err != nil {
			continue
		}
		if wasBytes, _ := json.Marshal(p); bytes.Equal(wasBytes, nowBytes) && declared(now) {
			return Verdict{State: Unchanged}
		}
		if was == nil {
			was = p
		}
	}
	if was == nil {
		return Verdict{Changed, "no tested version lists " + name}
	}
	return Verdict{Changed, diff(was, now)}
}

// declared is false when a relied-on path is in no branch of the schema.
func declared(p map[string]any) bool {
	for _, v := range p {
		if v == notDeclared {
			return false
		}
	}
	return true
}

// notDeclared marks a relied-on path no branch of the schema holds: the Go
// type names something TrueNAS does not declare, which is never unchanged.
const notDeclared = "not declared"

// project is the part of one `core.get_methods` entry Syndra relies on (§4.2),
// as a flat map from path to what is kept there. Its json.Marshal is the
// canonical form (§4.3): Go writes map keys sorted.
func project(entry json.RawMessage, dep Dep) (map[string]any, error) {
	var e struct {
		Accepts []any `json:"accepts"`
		Returns []any `json:"returns"`
		Job     *bool `json:"job"`
	}
	if err := json.Unmarshal(entry, &e); err != nil {
		return nil, fmt.Errorf("the entry is not readable")
	}
	switch {
	case e.Accepts == nil:
		return nil, fmt.Errorf("accepts: not listed")
	case len(e.Returns) != 1:
		return nil, fmt.Errorf("returns: not listed as one schema")
	case e.Job == nil:
		return nil, fmt.Errorf("job: not listed")
	}
	p := map[string]any{"job": *e.Job}
	returns := normalise(e.Returns[0])
	for _, r := range dep.Replies {
		replies(p, "returns", []any{returns}, reflect.TypeOf(r))
	}
	if len(dep.Sends) > 0 {
		accepts := make([]any, len(e.Accepts))
		for i, a := range e.Accepts {
			accepts[i] = normalise(a)
		}
		sends(p, e.Accepts, accepts, dep)
	}
	return p, nil
}

var (
	rawMessage  = reflect.TypeFor[json.RawMessage]()
	unmarshaler = reflect.TypeFor[json.Unmarshaler]()
)

// replies keeps, at every path the Go type t reads, the facts of every branch
// there, then descends the branches that can hold the next step: a slice the
// array branches, a struct the object branches that declare the field.
func replies(p map[string]any, path string, nodes []any, t reflect.Type) {
	t = deref(t)
	branches := flatten(nodes)
	if len(branches) == 0 {
		p[path] = notDeclared
		return
	}
	facts(p, path, branches)
	if t == rawMessage || reflect.PointerTo(t).Implements(unmarshaler) {
		return // the type reads the token as it comes
	}
	switch t.Kind() {
	case reflect.Slice, reflect.Array:
		var items []any
		for _, b := range branches {
			if hasType(b, "array") && b["items"] != nil {
				items = append(items, b["items"])
			}
		}
		replies(p, path+"/[]", items, t.Elem())
	case reflect.Struct:
		fields(p, path, branches, t)
	}
}

// fields descends a struct's fields as encoding/json reads them: an untagged
// embedded struct, or pointer to one, promotes its fields; any other field is
// named by its tag, else by its Go name.
func fields(p map[string]any, path string, branches []map[string]any, t reflect.Type) {
	for f := range t.Fields() {
		tag := f.Tag.Get("json")
		name, _, _ := strings.Cut(tag, ",")
		if tag == "-" {
			continue
		}
		if f.Anonymous && name == "" {
			if et := f.Type; et.Kind() == reflect.Struct || et.Kind() == reflect.Pointer && et.Elem().Kind() == reflect.Struct {
				fields(p, path, branches, deref(et))
				continue
			}
		}
		if !f.IsExported() {
			continue
		}
		if name == "" {
			name = f.Name
		}
		var next []any
		for _, b := range branches {
			if props, ok := b["properties"].(map[string]any); ok && props[name] != nil {
				next = append(next, props[name])
			}
		}
		replies(p, path+"/"+name, next, f.Type)
	}
}

func deref(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t
}

// facts keeps what a node says of its value, over every branch, descended or
// not: so a field made nullable, or a reply that lost a form, is seen.
func facts(p map[string]any, path string, branches []map[string]any) {
	var types, enums, formats, consts []string
	for _, b := range branches {
		switch t := b["type"].(type) {
		case string:
			types = append(types, t)
		case []any:
			for _, x := range t {
				types = append(types, fmt.Sprint(x))
			}
		}
		if e, ok := b["enum"].([]any); ok {
			for _, x := range e {
				enums = append(enums, canon(x))
			}
		}
		if f, ok := b["format"].(string); ok {
			formats = append(formats, f)
		}
		if c, ok := b["const"]; ok {
			consts = append(consts, canon(c))
		}
	}
	for fact, set := range map[string][]string{"type": types, "enum": enums, "format": formats, "const": consts} {
		if len(set) > 0 {
			slices.Sort(set)
			p[path+" "+fact] = strings.Join(slices.Compact(set), " or ")
		}
	}
}

// sends keeps the argument count and the schema at each pointer Syndra sends,
// its default dropped (what is sent is not defaulted), then walks every
// argument for the rest (§4.2).
func sends(p map[string]any, raw, accepts []any, dep Dep) {
	p["accepts"] = len(accepts)
	for _, ptr := range dep.Sends {
		nodes := resolve(accepts, ptr)
		if len(nodes) == 0 {
			p["accepts"+ptr] = notDeclared
			continue
		}
		for i, n := range nodes {
			if m, ok := n.(map[string]any); ok {
				m = maps.Clone(m)
				delete(m, "default")
				nodes[i] = m
			}
		}
		p["accepts"+ptr] = nodes
	}
	for i, a := range accepts {
		ptr := "/" + strconv.Itoa(i)
		if r, _ := raw[i].(map[string]any); r["_required_"] == true && !sentInto(dep.Sends, ptr) && !slices.Contains(dep.Sends, ptr) {
			// TrueNAS marks a required argument only with _required_: kept
			// where Syndra sends nothing, so an optional made required is seen.
			p["accepts"+ptr+" _required_"] = true
		}
		unsent(p, []any{a}, ptr, dep.Sends)
	}
}

// unsent walks the arguments below the pointers Syndra sends. At each object
// it keeps the required list, and where Syndra sends into it the
// additionalProperties; at each node Syndra sends nothing to, the default,
// since TrueNAS fills that in on every write.
func unsent(p map[string]any, nodes []any, ptr string, sent []string) {
	if slices.Contains(sent, ptr) {
		return
	}
	into := sentInto(sent, ptr)
	for _, b := range flatten(nodes) {
		if !into {
			add(p, "accepts"+ptr+" default", b["default"])
		}
		if hasType(b, "array") && b["items"] != nil {
			unsent(p, []any{b["items"]}, ptr+"/0", sent)
		}
		props, ok := b["properties"].(map[string]any)
		if !ok {
			continue
		}
		add(p, "accepts"+ptr+" required", b["required"])
		if into {
			add(p, "accepts"+ptr+" additionalProperties", b["additionalProperties"])
		}
		for _, name := range slices.Sorted(maps.Keys(props)) {
			step := strings.ReplaceAll(strings.ReplaceAll(name, "~", "~0"), "/", "~1")
			unsent(p, []any{props[name]}, ptr+"/"+step, sent)
		}
	}
}

// sentInto is whether Syndra sends anything below ptr.
func sentInto(sent []string, ptr string) bool {
	return slices.ContainsFunc(sent, func(s string) bool { return strings.HasPrefix(s, ptr+"/") })
}

// add appends one branch's value at key; nothing when the branch has none.
func add(p map[string]any, key string, v any) {
	if v == nil {
		return
	}
	list, _ := p[key].([]any)
	p[key] = append(list, v)
}

// resolve follows a JSON Pointer into the arguments (an array of argument
// schemas) through the schemas: a name through properties, an index through
// items, at every branch that can hold the step.
func resolve(accepts []any, ptr string) []any {
	steps := strings.Split(ptr, "/")[1:]
	i, err := strconv.Atoi(steps[0])
	if err != nil || i < 0 || i >= len(accepts) {
		return nil
	}
	nodes := []any{accepts[i]}
	for _, step := range steps[1:] {
		step = unescape(step)
		var next []any
		for _, b := range flatten(nodes) {
			if props, ok := b["properties"].(map[string]any); ok && props[step] != nil {
				next = append(next, props[step])
			} else if _, err := strconv.Atoi(step); err == nil && hasType(b, "array") && b["items"] != nil {
				next = append(next, b["items"])
			}
		}
		nodes = next
	}
	return nodes
}

func unescape(step string) string {
	return strings.ReplaceAll(strings.ReplaceAll(step, "~1", "/"), "~0", "~")
}

// flatten is the nodes and every anyOf or oneOf branch inside them.
func flatten(nodes []any) []map[string]any {
	var out []map[string]any
	for _, n := range nodes {
		m, ok := n.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, m)
		for _, k := range []string{"anyOf", "oneOf"} {
			if branches, ok := m[k].([]any); ok {
				out = append(out, flatten(branches)...)
			}
		}
	}
	return out
}

func hasType(b map[string]any, want string) bool {
	switch t := b["type"].(type) {
	case string:
		return t == want
	case []any:
		return slices.Contains(t, any(want))
	}
	return false
}

// annotations carry no behaviour (JSON Schema 2020-12, Validation §9), and the
// keys TrueNAS adds and marks FIXME (research §3).
var annotations = map[string]bool{
	"description": true, "title": true, "examples": true, "deprecated": true, "$comment": true,
	"_attrs_order_": true, "_name_": true, "_required_": true,
}

// normalise is the canonical schema (§4.3): annotations dropped, a one-element
// items list unwrapped, required, enum and a type list sorted as sets, anyOf
// and oneOf sorted by their canonical bytes. Only schema positions are walked,
// so a property named "title" and the data in enum, const and default are
// kept as they are.
func normalise(v any) any {
	m, ok := v.(map[string]any)
	if !ok {
		return v
	}
	out := make(map[string]any, len(m))
	for k, x := range m {
		if annotations[k] {
			continue
		}
		switch k {
		case "properties", "patternProperties":
			props, _ := x.(map[string]any)
			np := make(map[string]any, len(props))
			for name, s := range props {
				np[name] = normalise(s)
			}
			out[k] = np
		case "items":
			if l, ok := x.([]any); ok && len(l) == 1 {
				x = l[0]
			}
			if l, ok := x.([]any); ok {
				out[k] = each(l, false)
			} else {
				out[k] = normalise(x)
			}
		case "anyOf", "oneOf":
			out[k] = each(asList(x), true)
		case "allOf", "prefixItems":
			out[k] = each(asList(x), false)
		case "additionalProperties", "not", "contains":
			out[k] = normalise(x)
		case "required", "enum", "type":
			if l, ok := x.([]any); ok {
				l = slices.Clone(l)
				slices.SortFunc(l, func(a, b any) int { return strings.Compare(canon(a), canon(b)) })
				x = l
			}
			out[k] = x
		default:
			out[k] = x
		}
	}
	return out
}

func each(l []any, sorted bool) []any {
	out := make([]any, len(l))
	for i, s := range l {
		out[i] = normalise(s)
	}
	if sorted {
		slices.SortFunc(out, func(a, b any) int { return strings.Compare(canon(a), canon(b)) })
	}
	return out
}

func asList(x any) []any { l, _ := x.([]any); return l }

func canon(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// diff names each path whose kept value differs, "was / now".
func diff(was, now map[string]any) string {
	var keys []string
	for k := range was {
		keys = append(keys, k)
	}
	for k := range now {
		if _, ok := was[k]; !ok {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)
	var out []string
	for _, k := range keys {
		if a, b := show(was, k), show(now, k); a != b || b == notDeclared {
			out = append(out, k+": was "+a+", now "+b)
		}
	}
	return strings.Join(out, "; ")
}

func show(p map[string]any, k string) string {
	switch v := p[k].(type) {
	case nil:
		return "absent"
	case string:
		return v
	default:
		return canon(v)
	}
}
