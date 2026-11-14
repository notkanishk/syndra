// Package strictjson decodes a request body the way the contract's schemas
// read it, which Go's encoding/json does not.
//
// encoding/json matches a member name to a field case-insensitively, under
// Unicode simple folding (so "ſubject" lands in `json:"subject"` and the
// Kelvin sign in a "k"), and of two members with one name the last wins. The
// standards the contract follows point the other way:
//
//   - RFC 8259 §4 and §8.3: member names are strings, compared code point for
//     code point once escapes are undone;
//   - RFC 7493 (I-JSON) §2.3: an object MUST NOT carry two members with one name;
//   - the contract's JSON Schemas (2020-12) match property names exactly, so a
//     case variant is an unknown member, which `additionalProperties: false`
//     refuses.
//
// So Decode first walks the document token by token against the target type,
// refusing any member name that is not byte-equal to a field's JSON name, and
// any repeated name, then decodes as before with DisallowUnknownFields.
//
// The add-ons import this, and only this, from the contract module: a rule of
// the wire every add-on states once, rather than a Go type of the contract
// (design §1 rejects that). It is a decoding rule of RFC 8259 and RFC 7493,
// not a contract fact, so it cannot skew against a contract version.
package strictjson

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode"
)

// UnknownMemberError is a member name the target type has no field for,
// exactly: a case or fold variant of a field's name is one.
type UnknownMemberError struct {
	Object string // RFC 6901 pointer to the object that carries it; "" is the top level
	Name   string
}

func (e *UnknownMemberError) Error() string { return "unknown member " + where(e.Object, e.Name) }

// DuplicateMemberError is a name that appears twice in one object.
type DuplicateMemberError struct {
	Object string
	Name   string
}

func (e *DuplicateMemberError) Error() string { return "duplicate member " + where(e.Object, e.Name) }

func where(object, name string) string {
	if object == "" {
		return strconv.Quote(name)
	}
	return strconv.Quote(name) + " in " + object
}

// Decode decodes exactly one JSON value from data into v (a non-nil pointer),
// refusing an unknown member (*UnknownMemberError, matched exactly, never
// folded), a duplicate member (*DuplicateMemberError), and anything after the
// value. Struct fields are named as encoding/json names them: the tag, else
// the field name; `-` is no field; embedded structs are promoted; a type with
// its own UnmarshalJSON (json.RawMessage among them), an interface and a map
// take any member name, still never twice.
func Decode(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber() // a number is only skipped here; never refused for its range
	if err := walk(dec, reflect.TypeOf(v), ""); err != nil {
		return err
	}
	if _, err := dec.Token(); err != io.EOF {
		return errors.New("the body carries more than one JSON value")
	}
	strict := json.NewDecoder(bytes.NewReader(data))
	strict.DisallowUnknownFields()
	return strict.Decode(v)
}

var unmarshaler = reflect.TypeFor[json.Unmarshaler]()

// walk reads one value and checks every object in it against t (nil: any
// names, duplicates still refused). ptr is the value's RFC 6901 pointer.
func walk(dec *json.Decoder, t reflect.Type, ptr string) error {
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	for t != nil && t.Kind() == reflect.Pointer && !t.Implements(unmarshaler) {
		t = t.Elem()
	}
	if t != nil && (t.Implements(unmarshaler) || reflect.PointerTo(t).Implements(unmarshaler)) {
		t = nil // opaque: its own UnmarshalJSON reads it
	}
	switch tok {
	case json.Delim('['):
		var elem reflect.Type
		if t != nil && (t.Kind() == reflect.Slice || t.Kind() == reflect.Array) {
			elem = t.Elem()
		}
		for i := 0; dec.More(); i++ {
			if err := walk(dec, elem, ptr+"/"+strconv.Itoa(i)); err != nil {
				return err
			}
		}
		_, err = dec.Token()
		return err
	case json.Delim('{'):
		var known map[string]reflect.Type
		var elem reflect.Type
		switch {
		case t != nil && t.Kind() == reflect.Struct:
			known = fieldsOf(t)
		case t != nil && t.Kind() == reflect.Map:
			elem = t.Elem()
		}
		// ponytail: types are static; an interface field holding a struct
		// pointer before decoding takes any names here (and folded ones in
		// encoding/json). No request type does that; walk the dynamic value if one ever must.
		seen := map[string]bool{}
		for dec.More() {
			tok, err := dec.Token()
			if err != nil {
				return err
			}
			name := tok.(string) // the decoder yields a string for every member name
			if seen[name] {
				return &DuplicateMemberError{Object: ptr, Name: name}
			}
			seen[name] = true
			next := elem
			if known != nil {
				ft, ok := known[name]
				if !ok {
					return &UnknownMemberError{Object: ptr, Name: name}
				}
				next = ft
			}
			if err := walk(dec, next, ptr+"/"+escape(name)); err != nil {
				return err
			}
		}
		_, err = dec.Token()
		return err
	}
	return nil // a scalar
}

// escape is RFC 6901 §3: "~" is "~0" and "/" is "~1".
func escape(name string) string {
	return strings.NewReplacer("~", "~0", "/", "~1").Replace(name)
}

var cache sync.Map // reflect.Type → map[string]reflect.Type

type field struct {
	name   string
	depth  int
	tagged bool
	typ    reflect.Type
}

// fieldsOf is a struct's JSON member names and their types, resolved as
// encoding/json's typeFields resolves them: a field promoted from an embedded
// struct loses to a shallower one of the same name; at one depth, a tagged
// field wins over untagged ones; two of a kind at one depth cancel, and the
// name is no field at all.
func fieldsOf(t reflect.Type) map[string]reflect.Type {
	if m, ok := cache.Load(t); ok {
		return m.(map[string]reflect.Type)
	}
	var all []field
	visited := map[reflect.Type]bool{}
	level := []reflect.Type{t}
	for depth := 0; len(level) > 0; depth++ {
		var next []reflect.Type
		for _, st := range level {
			if visited[st] {
				// Reached again at a deeper level: the shallower copy dominates.
				// The same type twice at one level is walked twice, so its
				// names meet themselves and cancel, as encoding/json's do.
				// ponytail: a type reached twice one level further down
				// (a diamond: A1 and A2 both embed B, B embeds C) is walked
				// twice here, where encoding/json queues it once, so C's names
				// cancel: over-strict, never folded. No request type nests so.
				continue
			}
			for i := range st.NumField() {
				f := st.Field(i)
				ft := f.Type
				if f.Anonymous {
					if ft.Kind() == reflect.Pointer {
						ft = ft.Elem()
					}
					if !f.IsExported() && ft.Kind() != reflect.Struct {
						continue
					}
				} else if !f.IsExported() {
					continue
				}
				tag := f.Tag.Get("json")
				if tag == "-" {
					continue
				}
				name, _, _ := strings.Cut(tag, ",")
				if !validTag(name) {
					name = ""
				}
				if name == "" && f.Anonymous && ft.Kind() == reflect.Struct {
					next = append(next, ft)
					continue
				}
				tagged := name != ""
				if !tagged {
					name = f.Name
				}
				all = append(all, field{name: name, depth: depth, tagged: tagged, typ: f.Type})
			}
		}
		for _, st := range level {
			visited[st] = true
		}
		level = next
	}
	sort.SliceStable(all, func(i, j int) bool {
		if all[i].name != all[j].name {
			return all[i].name < all[j].name
		}
		if all[i].depth != all[j].depth {
			return all[i].depth < all[j].depth
		}
		return all[i].tagged && !all[j].tagged
	})
	out := map[string]reflect.Type{}
	for i := 0; i < len(all); {
		j := i + 1
		for j < len(all) && all[j].name == all[i].name {
			j++
		}
		group := all[i:j]
		// group[0] is the shallowest, tagged first. It wins unless another
		// field at its depth is as tagged as it is.
		if len(group) == 1 || group[1].depth > group[0].depth || group[0].tagged && !group[1].tagged {
			out[group[0].name] = group[0].typ
		}
		i = j
	}
	cache.Store(t, out)
	return out
}

// validTag is encoding/json's isValidTag: a tag name of anything else is
// ignored, and the field keeps its Go name.
func validTag(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		switch {
		case strings.ContainsRune("!#$%&()*+-./:;<=>?@[]^_{|}~ ", c):
		case !unicode.IsLetter(c) && !unicode.IsDigit(c):
			return false
		}
	}
	return true
}
