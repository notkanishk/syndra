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
	"reflect"
	"slices"
	"strconv"
	"strings"
)

//go:embed baseline/*.methods.json
var shipped embed.FS

// Baseline is one tested version's capture: `{version, methods: {name:
// {accepts, returns, job}}}`, as the NAS answered.
type Baseline struct {
	Version string                     `json:"version"`
	Methods map[string]json.RawMessage `json:"methods"`
}

// Baselines are the captures shipped in the binary. An error is a build fault:
// the add-on refuses to start (design §8).
func Baselines() ([]Baseline, error) {
	names, err := fs.Glob(shipped, "baseline/*.methods.json")
	if err != nil {
		return nil, err
	}
	var out []Baseline
	for _, name := range names {
		raw, err := shipped.ReadFile(name)
		if err != nil {
			return nil, err
		}
		var b Baseline
		if err := json.Unmarshal(raw, &b); err != nil || b.Version == "" || len(b.Methods) == 0 {
			return nil, fmt.Errorf("%s is not a capture: %v", name, err)
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
	Create  bool     // a create: the defaults of unsent properties count (§4.2)
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
		sends(p, accepts, dep)
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
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
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

// fields descends each json-tagged field of a struct, and the fields of an
// untagged embedded one, as encoding/json reads them.
func fields(p map[string]any, path string, branches []map[string]any, t reflect.Type) {
	for f := range t.Fields() {
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if f.Anonymous && name == "" {
			fields(p, path, branches, f.Type)
			continue
		}
		if name == "" || name == "-" || !f.IsExported() {
			continue
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

// sends keeps the argument count, the schema at each pointer Syndra sends, and
// of every object it sends into: its required list, its additionalProperties
// and, on a create, the default of each property it does not send.
func sends(p map[string]any, accepts []any, dep Dep) {
	p["accepts"] = len(accepts)
	sent := map[string][]string{} // object pointer → the properties sent into it
	for _, ptr := range dep.Sends {
		nodes := resolve(accepts, ptr)
		if len(nodes) == 0 {
			p["accepts"+ptr] = notDeclared
			continue
		}
		p["accepts"+ptr] = nodes
		if i := strings.LastIndex(ptr, "/"); i > 0 {
			sent[ptr[:i]] = append(sent[ptr[:i]], unescape(ptr[i+1:]))
		}
	}
	for obj, names := range sent {
		for _, b := range flatten(resolve(accepts, obj)) {
			props, ok := b["properties"].(map[string]any)
			if !ok {
				continue
			}
			add(p, "accepts"+obj+" required", b["required"])
			add(p, "accepts"+obj+" additionalProperties", b["additionalProperties"])
			if !dep.Create {
				continue
			}
			for prop, s := range props {
				if d, ok := s.(map[string]any)["default"]; ok && !slices.Contains(names, prop) {
					add(p, "accepts"+obj+"/"+prop+" default", d)
				}
			}
		}
	}
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
