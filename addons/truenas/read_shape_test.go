package main

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Every field this add-on decodes off the target, checked against what the
// target actually answered.
//
// This is the guard the branch was missing, and its absence cost the whole life
// of one operation. `audit.query` answers `message_timestamp` as an INTEGER;
// `smbActivity` decoded it as a string; `encoding/json` failed the entire
// response; `activity.get` returned "the audit log could not be read" every
// time it was ever called. Two suites were green, and the recorded fixture
// agreed with the code — because the fixture recorded key NAMES, and the names
// were right. It was the TYPES that disagreed.
//
// So this reads the add-on's own source for the struct behind every
// `nas.call(...)`, and asserts each json tag against `truenas_observed.json`:
// the key exists on the target, and the Go type can hold what the target puts
// in it. It also fails on a method decoded here that the recorder never asked
// about — an unrecorded read is a fixture that cannot disagree with anything.

type recordedRead struct {
	Keys  []string          `json:"keys"`
	Types map[string]string `json:"types"`
}

func loadRecording(t *testing.T) map[string]recordedRead {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "contract", "truenas_observed.json"))
	if err != nil {
		t.Fatalf("the recording is what this guard checks against: %v", err)
	}
	var doc struct {
		Reads map[string]json.RawMessage `json:"reads"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("decode recording: %v", err)
	}
	out := map[string]recordedRead{}
	for method, body := range doc.Reads {
		var r recordedRead
		// A read recorded as a bare value (`system.version` answers a string)
		// has no field shape. Registered as present with no types, so the
		// coverage half still counts it.
		_ = json.Unmarshal(body, &r)
		out[method] = r
	}
	return out
}

// decodeSite is one `nas.call("method", …, &target)` with an inline struct
// behind the target.
type decodeSite struct {
	Method string
	Pos    string
	Fields map[string]string // json tag -> Go type expression
	// Optional names the fields marked `shape-optional:` in the source. The
	// target may legitimately omit these, so their ABSENCE from the recording
	// proves nothing; if the recording does carry one, its type is still
	// checked. The marker requires a reason on the same comment, because
	// "sometimes missing" and "I could not get the guard to pass" look
	// identical without one.
	Optional map[string]bool
}

func findDecodeSites(t *testing.T) []decodeSite {
	t.Helper()
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse package: %v", err)
	}

	var sites []decodeSite
	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				// Source order, so the nearest preceding declaration of a name
				// is the one in scope at the call.
				inScope := map[string]*ast.StructType{}
				ast.Inspect(fn.Body, func(n ast.Node) bool {
					if spec, ok := n.(*ast.ValueSpec); ok {
						for _, name := range spec.Names {
							if st := structBehind(spec.Type); st != nil {
								inScope[name.Name] = st
							}
						}
						return true
					}
					call, ok := n.(*ast.CallExpr)
					if !ok || len(call.Args) != 3 {
						return true
					}
					sel, ok := call.Fun.(*ast.SelectorExpr)
					if !ok || sel.Sel.Name != "call" {
						return true
					}
					lit, ok := call.Args[0].(*ast.BasicLit)
					if !ok || lit.Kind != token.STRING {
						// A method chosen at runtime. Allowed only where
						// nothing is decoded — see the exemption test below.
						return true
					}
					method := strings.Trim(lit.Value, `"`)
					unary, ok := call.Args[2].(*ast.UnaryExpr)
					if !ok || unary.Op != token.AND {
						return true
					}
					ident, ok := unary.X.(*ast.Ident)
					if !ok {
						return true
					}
					st, ok := inScope[ident.Name]
					if !ok {
						// Not an inline struct: a string, a map, a named id
						// type. Nothing with a field shape to disagree about.
						return true
					}
					fields, optional := jsonFields(st)
					sites = append(sites, decodeSite{
						Method:   method,
						Pos:      fset.Position(call.Pos()).String(),
						Fields:   fields,
						Optional: optional,
					})
					return true
				})
			}
		}
	}
	return sites
}

// structBehind unwraps `struct{…}` and `[]struct{…}` and nothing else.
func structBehind(expr ast.Expr) *ast.StructType {
	switch v := expr.(type) {
	case *ast.StructType:
		return v
	case *ast.ArrayType:
		if st, ok := v.Elt.(*ast.StructType); ok {
			return st
		}
	}
	return nil
}

func jsonFields(st *ast.StructType) (fields map[string]string, optional map[string]bool) {
	fields, optional = map[string]string{}, map[string]bool{}
	for _, f := range st.Fields.List {
		if f.Tag == nil {
			continue
		}
		tag := reflect.StructTag(strings.Trim(f.Tag.Value, "`")).Get("json")
		name := strings.Split(tag, ",")[0]
		if name == "" || name == "-" {
			continue
		}
		fields[name] = typeName(f.Type)
		for _, group := range []*ast.CommentGroup{f.Doc, f.Comment} {
			if group == nil {
				continue
			}
			for _, c := range group.List {
				if i := strings.Index(c.Text, "shape-optional:"); i >= 0 {
					// The reason is mandatory. A bare marker is a silenced
					// check, which is the thing this guard exists to prevent.
					if strings.TrimSpace(c.Text[i+len("shape-optional:"):]) != "" {
						optional[name] = true
					}
				}
			}
		}
	}
	return fields, optional
}

func typeName(expr ast.Expr) string {
	switch v := expr.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.StarExpr:
		return typeName(v.X)
	case *ast.SelectorExpr:
		return typeName(v.X) + "." + v.Sel.Name
	case *ast.ArrayType:
		return "[]" + typeName(v.Elt)
	case *ast.MapType:
		return "map"
	case *ast.StructType:
		return "struct"
	case *ast.InterfaceType:
		return "any"
	}
	return "?"
}

// canHold answers whether a Go type can receive what the target puts there.
//
// Deliberately strict about the one pair that has already cost a whole feature:
// a string field never holds a number, whatever the number looks like. Loose
// where looseness is safe — `json.RawMessage` holds anything by construction,
// and that is how a payload whose shape varies is decoded safely.
func canHold(goType, jsonType string) bool {
	if goType == "json.RawMessage" || goType == "any" {
		return true
	}
	for _, jt := range strings.Split(jsonType, "|") {
		if !holdsOne(goType, jt) {
			return false
		}
	}
	return true
}

func holdsOne(goType, jsonType string) bool {
	switch jsonType {
	case "null":
		// The field was null in every row sampled. Nothing was observed, so
		// nothing is asserted — Go decodes a JSON null into any type's zero.
		return true
	case "str":
		return goType == "string"
	case "int":
		switch goType {
		case "int", "int8", "int16", "int32", "int64", "uint", "uint64", "float32", "float64", "json.Number":
			return true
		}
		return false
	case "float":
		// NOT an integer type. `json` refuses a fractional number into an int,
		// and refusing it fails the WHOLE response, not the field.
		return goType == "float32" || goType == "float64" || goType == "json.Number"
	case "bool":
		return goType == "bool"
	case "dict":
		// A struct, a map, or a named type that is one of those. Never a
		// scalar and never a slice: both fail the whole response.
		return !isScalar(goType) && !strings.HasPrefix(goType, "[]")
	case "list":
		return strings.HasPrefix(goType, "[]") || goType == "map"
	}
	return true
}

// isScalar names the Go types that cannot receive a JSON object or array.
func isScalar(goType string) bool {
	switch goType {
	case "string", "bool",
		"int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64",
		"float32", "float64":
		return true
	}
	return false
}

func TestEveryDecodedFieldMatchesWhatTheTargetAnswered(t *testing.T) {
	recording := loadRecording(t)
	sites := findDecodeSites(t)
	if len(sites) == 0 {
		t.Fatal("no decode sites found — this guard has stopped looking at anything")
	}

	for _, site := range sites {
		read, recorded := recording[site.Method]
		if !recorded {
			t.Errorf("%s decodes %s and the recorder never asks the target about it. "+
				"Add it to scripts/lib/record-truenas.py and re-record: an unrecorded "+
				"read is a fixture that cannot disagree with the target.", site.Pos, site.Method)
			continue
		}
		if len(read.Types) == 0 {
			continue // recorded as a bare value; no field shape to check
		}
		for field, goType := range site.Fields {
			jsonType, present := read.Types[field]
			if !present {
				if site.Optional[field] {
					// Marked as one the target may omit, with a reason. Nothing
					// was observed, so nothing is asserted.
					continue
				}
				t.Errorf("%s decodes %s.%s and the target does not answer with that key "+
					"(it answers %v). If the target only sometimes sends it, mark the "+
					"field `shape-optional: <why>` and say which case omits it.",
					site.Pos, site.Method, field, read.Keys)
				continue
			}
			if !canHold(goType, jsonType) {
				t.Errorf("%s decodes %s.%s into a %s and the target answers %s. "+
					"encoding/json fails the WHOLE response on this, not the field — "+
					"which is exactly how activity.get stayed broken for its entire life.",
					site.Pos, site.Method, field, goType, jsonType)
			}
		}
	}
}

// A method name chosen at runtime is allowed, and only where nothing is
// decoded. `targetHealth` picks its four methods from a table and reads each
// into a `json.RawMessage`, which has no shape to check; a dynamic method
// decoding into a struct would slip past the guard above in silence.
func TestADynamicMethodNameDecodesNothingStructured(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse package: %v", err)
	}
	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			ast.Inspect(file, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok || len(call.Args) != 3 {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "call" {
					return true
				}
				if _, literal := call.Args[0].(*ast.BasicLit); literal {
					return true
				}
				// Dynamic. The destination must be something with no shape.
				unary, ok := call.Args[2].(*ast.UnaryExpr)
				if !ok {
					return true
				}
				ident, ok := unary.X.(*ast.Ident)
				if !ok || (ident.Name != "raw" && ident.Name != "out") {
					t.Errorf("%s calls a runtime-chosen method and decodes into %s. "+
						"The shape guard cannot see which method that is, so the "+
						"destination must be a json.RawMessage.",
						fset.Position(call.Pos()), typeName(unary.X))
				}
				return true
			})
		}
	}
}

// The type rule, asserted directly.
//
// A mutation test cannot reach it from the decode sites: every field there is
// used downstream in a type-constrained way, so changing one to the wrong type
// fails the compiler before it reaches this guard. That is a good property and
// it is not the property being checked here — the guard has to reject a
// mismatch that WOULD compile, which is what a fresh struct on a new read is.
func TestTheTypeRuleRejectsWhatEncodingJSONRejects(t *testing.T) {
	rejects := []struct{ goType, jsonType string }{
		// The one that cost a whole operation.
		{"string", "int"},
		{"string", "bool"},
		{"int64", "str"},
		// A fractional number into an integer fails the WHOLE response.
		{"int64", "float"},
		{"bool", "str"},
		{"string", "dict"},
		{"[]string", "str"},
	}
	for _, c := range rejects {
		if canHold(c.goType, c.jsonType) {
			t.Errorf("a %s must not be offered %s", c.goType, c.jsonType)
		}
	}

	accepts := []struct{ goType, jsonType string }{
		{"string", "str"},
		{"int64", "int"},
		{"float64", "float"},
		{"float64", "int"},
		{"bool", "bool"},
		{"json.RawMessage", "dict"},
		{"json.RawMessage", "int"},
		{"struct", "dict"},
		{"[]string", "list"},
		// Null in every row sampled: nothing was observed, nothing is claimed.
		{"string", "null"},
		// Genuinely two types on the wire; only a type that holds both passes.
		{"json.RawMessage", "str|int"},
	}
	for _, c := range accepts {
		if !canHold(c.goType, c.jsonType) {
			t.Errorf("a %s must accept %s", c.goType, c.jsonType)
		}
	}
}
