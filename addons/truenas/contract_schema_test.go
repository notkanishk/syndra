package main

import (
	"bytes"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// The schema half of the contract (see ../contract/README.md).
//
// Every fixture validates against its schema, with format assertion ON: in JSON
// Schema 2020-12 `format` is an annotation unless the validator is told
// otherwise, so a `date-time` that is never asserted is a comment. Schemas load
// from disk by `$id` and never over the network, so a test run needs nothing
// that is not in the checkout.

const contractDir = "../contract"

// contractMessages pairs every fixture with the schema it must satisfy. A
// fixture on disk that is not listed here fails TestEveryFixtureIsCovered, so
// adding a document without saying what it is cannot pass.
var contractMessages = []struct{ fixture, schema string }{
	{"manifest.json", "manifest.json"},
	{"health.json", "health.json"},
	{"subjects.json", "subjects.json"},
	{"values.json", "values.json"},
	{"reach.json", "profiles/storage/reach.json"},
	{"settings.json", "settings.json"},
	{"setting.json", "setting.json"},
	{"setting_put_request.json", "setting_put_request.json"},
	{"homes.json", "profiles/storage/homes.json"},
	{"plan_request.json", "plan_request.json"},
	{"plan_response.json", "plan_response.json"},
	{"apply_request.json", "apply_request.json"},
	{"apply_response.json", "apply_response.json"},
	{"operation_request.json", "operation_request.json"},
	{"operation_response.json", "operation_response.json"},
	{"operation_response_activity.json", "operation_response.json"},
	{"operation_response_storage.json", "operation_response.json"},
	{"operation_response_health.json", "operation_response.json"},
	{"lifecycle_request.json", "lifecycle_request.json"},
	{"lifecycle_response.json", "lifecycle_response.json"},
	{"error.json", "error.json"},
}

var (
	schemaOnce     sync.Once
	schemaCompiler *jsonschema.Compiler
	schemaErr      error
)

// compiledSchema returns the schema at a path under contract/schema, loading
// every schema file by its own `$id` so relative `$ref`s resolve without a fetch.
func compiledSchema(t *testing.T, path string) *jsonschema.Schema {
	t.Helper()
	schemaOnce.Do(func() {
		schemaCompiler = jsonschema.NewCompiler()
		schemaCompiler.AssertFormat()
		root := filepath.Join(contractDir, "schema")
		schemaErr = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || filepath.Ext(p) != ".json" {
				return err
			}
			raw, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
			if err != nil {
				return err
			}
			id, _ := doc.(map[string]any)["$id"].(string)
			return schemaCompiler.AddResource(id, doc)
		})
	})
	if schemaErr != nil {
		t.Fatalf("load contract schemas: %v", schemaErr)
	}
	sch, err := schemaCompiler.Compile(schemaBase + path)
	if err != nil {
		t.Fatalf("compile schema %s: %v", path, err)
	}
	return sch
}

const schemaBase = "https://syndra.example.org/addon-contract/1/schema/"

// assertSchema validates a document and names the schema and the document in
// the failure, with the validator's own path to the offending field.
func assertSchema(t *testing.T, schema string, doc []byte, what string) {
	t.Helper()
	v, err := jsonschema.UnmarshalJSON(bytes.NewReader(doc))
	if err != nil {
		t.Fatalf("%s: not JSON: %v", what, err)
	}
	if err := compiledSchema(t, schema).Validate(v); err != nil {
		t.Errorf("%s does not satisfy schema/%s:\n%v", what, schema, err)
	}
}

func TestEveryFixtureValidatesAgainstItsSchema(t *testing.T) {
	for _, m := range contractMessages {
		t.Run(m.fixture, func(t *testing.T) {
			assertSchema(t, m.schema, contractFixture(t, m.fixture), "fixtures/"+m.fixture)
		})
	}
}

func TestEveryFixtureIsCovered(t *testing.T) {
	listed := map[string]bool{}
	for _, m := range contractMessages {
		listed[m.fixture] = true
	}
	entries, err := os.ReadDir(filepath.Join(contractDir, "fixtures"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !listed[e.Name()] {
			t.Errorf("fixtures/%s is not paired with a schema in contractMessages", e.Name())
		}
	}
}

// The schemas must be able to say no. A schema that accepts everything would
// pass the test above for every fixture, so each rule is shown a document that
// breaks it. A nil value drops the key; a func edits the decoded document.
func TestSchemasRefuseWhatTheyShould(t *testing.T) {
	set := func(key string, value any) func(map[string]any) {
		return func(d map[string]any) {
			if value == nil {
				delete(d, key)
				return
			}
			d[key] = value
		}
	}
	for _, tc := range []struct {
		name, schema, fixture string
		break_                func(map[string]any)
	}{
		{"request with an unknown field", "apply_request.json", "apply_request.json", set("surprise", true)},
		{"wrong contract version", "plan_request.json", "plan_request.json", set("contract_version", 2)},
		{"negative log_records", "health.json", "health.json", set("log_records", -1)},
		{"timestamp that is not RFC 3339", "subjects.json", "subjects.json", set("taken_at", "yesterday")},
		{"effect outside the closed set", "apply_response.json", "apply_response.json", set("effect", "maybe")},
		{"apply without a fingerprint", "apply_request.json", "apply_request.json", set("fingerprint", nil)},
		{"apply with a blank fingerprint", "apply_request.json", "apply_request.json", set("fingerprint", "  ")},
		{"apply with a blank call_id", "apply_request.json", "apply_request.json", set("call_id", " ")},
		{"apply with a blank subject", "apply_request.json", "apply_request.json", set("subject", "")},
		{"operation with a blank call_id", "operation_request.json", "operation_request.json", set("call_id", " ")},
		{"lifecycle with a blank reason", "lifecycle_request.json", "lifecycle_request.json", set("reason", " ")},
		{"desired list field null", "apply_request.json", "apply_request.json", func(d map[string]any) {
			d["desired"].(map[string]any)["group"] = nil
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var doc map[string]any
			if err := json.Unmarshal(contractFixture(t, tc.fixture), &doc); err != nil {
				t.Fatal(err)
			}
			tc.break_(doc)
			broken, _ := json.Marshal(doc)
			v, _ := jsonschema.UnmarshalJSON(bytes.NewReader(broken))
			if err := compiledSchema(t, tc.schema).Validate(v); err == nil {
				t.Errorf("schema/%s accepted %s", tc.schema, broken)
			}
		})
	}
}

// ---- the error registry -------------------------------------------------

type errorRegistry struct {
	Codes map[string]struct {
		Statuses []int  `json:"statuses"`
		Retry    string `json:"retry"`
		Meaning  string `json:"meaning"`
	} `json:"codes"`
}

func loadErrorRegistry(t *testing.T) errorRegistry {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(contractDir, "registry", "errors.json"))
	if err != nil {
		t.Fatal(err)
	}
	var reg errorRegistry
	if err := json.Unmarshal(raw, &reg); err != nil {
		t.Fatalf("registry/errors.json: %v", err)
	}
	return reg
}

// codesInSource reads this package's own non-test source and returns the codes
// it writes on the wire — every value of an `"error":` key — plus every Code*
// constant declared. An value after `"error":` that is not a Code* constant is reported: a
// code written inline is a code nothing compares against the registry.
func codesInSource(t *testing.T) (written, declared map[string]bool, inline []string) {
	t.Helper()
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi fs.FileInfo) bool { return !strings.HasSuffix(fi.Name(), "_test.go") }, 0)
	if err != nil {
		t.Fatal(err)
	}
	consts := map[string]string{}
	written, declared = map[string]bool{}, map[string]bool{}
	for _, pkg := range pkgs {
		for _, f := range pkg.Files {
			ast.Inspect(f, func(n ast.Node) bool {
				if vs, ok := n.(*ast.ValueSpec); ok {
					for i, name := range vs.Names {
						if i >= len(vs.Values) {
							continue
						}
						if lit, ok := vs.Values[i].(*ast.BasicLit); ok && strings.HasPrefix(name.Name, "Code") {
							v, _ := strconv.Unquote(lit.Value)
							consts[name.Name], declared[v] = v, true
						}
					}
				}
				return true
			})
		}
	}
	for _, pkg := range pkgs {
		for _, f := range pkg.Files {
			ast.Inspect(f, func(n ast.Node) bool {
				kv, ok := n.(*ast.KeyValueExpr)
				if !ok {
					return true
				}
				if k, ok := kv.Key.(*ast.BasicLit); !ok || k.Value != `"error"` {
					return true
				}
				// Only a declared Code* constant is a registered code. A literal,
				// a variable, a call, a parameter: anything else is a code the
				// registry cannot be compared against, so it is reported.
				if id, ok := kv.Value.(*ast.Ident); ok {
					if code, known := consts[id.Name]; known {
						written[code] = true
						return true
					}
				}
				inline = append(inline, fset.Position(kv.Value.Pos()).String())
				return true
			})
		}
	}
	return written, declared, inline
}

func TestEveryCodeTheAddOnWritesIsInTheRegistry(t *testing.T) {
	reg := loadErrorRegistry(t)
	written, declared, inline := codesInSource(t)
	for _, at := range inline {
		t.Errorf("%s writes an error code that is not a Code* constant from errors.go", at)
	}
	if len(written) == 0 {
		t.Fatal("found no error codes in the source, so this guard proved nothing")
	}
	for code := range written {
		if _, ok := reg.Codes[code]; !ok {
			t.Errorf("the add-on writes %s and registry/errors.json does not list it", code)
		}
	}
	for code := range declared {
		if !written[code] {
			t.Errorf("errors.go declares %s and nothing writes it", code)
		}
	}
	for code, e := range reg.Codes {
		if !declared[code] {
			t.Errorf("registry/errors.json lists %s and the add-on does not write it", code)
		}
		if len(e.Statuses) == 0 || e.Meaning == "" || !slices.Contains([]string{"never", "backoff", "retry_after", "replan", "by_status"}, e.Retry) {
			t.Errorf("registry entry %s is incomplete: %+v", code, e)
		}
	}
}

// The real handlers, driven into their refusals: each answers a status its
// registry entry lists and a body that satisfies error.json.
func TestRealRefusalsMatchTheRegistryAndTheErrorSchema(t *testing.T) {
	reg := loadErrorRegistry(t)
	s, _ := applyServer(t, fixtureUsers)
	for _, tc := range []struct {
		name, code string
		drive      func(rr *httptest.ResponseRecorder)
	}{
		{"malformed apply", CodeBadRequest, func(rr *httptest.ResponseRecorder) {
			postTo(s.handleApply, rr, http.MethodPost, "/apply", `{"surprise":1}`)
		}},
		{"wrong contract version", CodeContractVersionMismatch, func(rr *httptest.ResponseRecorder) {
			postTo(s.handlePlan, rr, http.MethodPost, "/plan", `{"contract_version":2,"subjects":[]}`)
		}},
		{"apply without a fingerprint", CodeFingerprintRequired, func(rr *httptest.ResponseRecorder) {
			postTo(s.handleApply, rr, http.MethodPost, "/apply", `{"contract_version":1,"call_id":"c","subject":"s","desired":{}}`)
		}},
		{"operation named differently in the body", CodeOperationMismatch, func(rr *httptest.ResponseRecorder) {
			req := httptest.NewRequest(http.MethodPost, "/operations/health.get", nil)
			req.SetPathValue("name", "health.get")
			body := []byte(`{"contract_version":1,"call_id":"c","operation":"account.purge"}`)
			s.handleOperation(rr, req, body)
		}},
		{"lifecycle without a reason", CodeReasonRequired, func(rr *httptest.ResponseRecorder) {
			postTo(s.handleLifecycle, rr, http.MethodPost, "/lifecycle", `{"contract_version":1,"state":"read_only","reason":" "}`)
		}},
		{"unknown lifecycle state", CodeUnknownLifecycleState, func(rr *httptest.ResponseRecorder) {
			postTo(s.handleLifecycle, rr, http.MethodPost, "/lifecycle", `{"contract_version":1,"state":"asleep","reason":"x"}`)
		}},
		{"unknown field", CodeUnknownField, func(rr *httptest.ResponseRecorder) {
			req := httptest.NewRequest(http.MethodGet, "/values/nope", nil)
			req.SetPathValue("field", "nope")
			s.handleValues(rr, req, nil)
		}},
		{"unknown setting", CodeNoSuchSetting, func(rr *httptest.ResponseRecorder) {
			req := httptest.NewRequest(http.MethodPut, "/settings/nope", nil)
			req.SetPathValue("name", "nope")
			s.handlePutSetting(rr, req, []byte(`{}`))
		}},
		{"setting body with an unknown field", CodeInvalidBody, func(rr *httptest.ResponseRecorder) {
			req := httptest.NewRequest(http.MethodPut, "/settings/home_root", nil)
			req.SetPathValue("name", "home_root")
			s.handlePutSetting(rr, req, []byte(`{"surprise":1}`))
		}},
		{"unsigned request", CodeUnauthenticated, func(rr *httptest.ResponseRecorder) {
			s.auth.authenticated(s.handleHealth)(rr, httptest.NewRequest(http.MethodGet, "/health", nil))
		}},
	} {
		t.Run(tc.code, func(t *testing.T) {
			rr := httptest.NewRecorder()
			tc.drive(rr)
			var body struct {
				Error string `json:"error"`
			}
			if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil || body.Error != tc.code {
				t.Fatalf("%s answered %d %q, want code %s", tc.name, rr.Code, rr.Body.String(), tc.code)
			}
			entry, ok := reg.Codes[tc.code]
			if !ok || !slices.Contains(entry.Statuses, rr.Code) {
				t.Errorf("%s answered %d; registry/errors.json lists %v", tc.code, rr.Code, entry.Statuses)
			}
			assertSchema(t, "error.json", rr.Body.Bytes(), tc.code+" reply")
		})
	}
}

func postTo(h func(http.ResponseWriter, *http.Request, []byte), rr *httptest.ResponseRecorder, method, path, body string) {
	h(rr, httptest.NewRequest(method, path, strings.NewReader(body)), []byte(body))
}
