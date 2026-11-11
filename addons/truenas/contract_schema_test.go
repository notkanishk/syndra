package main

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// The registry half of the contract (see ../contract/README.md): the codes this
// add-on writes are the ones registry/errors.json lists, with the registry's
// title and retry class, and every refusal goes through writeError.
// contract2_test.go holds every fixture to its schema.

type errorRegistry struct {
	Codes map[string]struct {
		Title    string `json:"title"`
		Statuses []int  `json:"statuses"`
		Retry    string `json:"retry"`
		Meaning  string `json:"meaning"`
	} `json:"codes"`
}

func loadErrorRegistry(t *testing.T) errorRegistry {
	t.Helper()
	var reg errorRegistry
	if err := json.Unmarshal(v2Read(t, "registry", "errors.json"), &reg); err != nil {
		t.Fatalf("registry/errors.json: %v", err)
	}
	return reg
}

// notWritten is every registered code this add-on never writes, with why. A
// code it adds must be written or listed here.
var notWritten = map[string]string{
	"CALL_UNSETTLED":      "feature calls (physical operations), not declared",
	"FRESH_READ_COOLDOWN": "feature freshness, not declared",
	"TARGET_UNREACHABLE":  "a read with nothing to serve is NO_CURRENT_READ or TARGET_UNREADABLE here",
	"UNKNOWN_METRIC":      "feature stats, not declared",
}

// codesInSource reads this package's own non-test source and returns the codes
// it writes on the wire — the code argument of every writeError and
// writeOutcomeRefusal call — plus every Code* constant declared. A code argument
// that is not a Code* constant is reported: a code written inline is a code
// nothing compares against the registry. So is any composite literal keyed
// "error" or "code", which is a refusal body built beside writeError.
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
	// The code argument of each call that writes or forwards one.
	codeArg := map[string]int{"writeError": 2, "writeOutcomeRefusal": 2, "decodeRequest": 3}
	for _, pkg := range pkgs {
		for _, f := range pkg.Files {
			for _, decl := range f.Decls {
				fd, isFunc := decl.(*ast.FuncDecl)
				ast.Inspect(decl, func(n ast.Node) bool {
					switch n := n.(type) {
					case *ast.CallExpr:
						fn, ok := n.Fun.(*ast.Ident)
						if !ok {
							return true
						}
						at, writes := codeArg[fn.Name]
						if !writes || len(n.Args) <= at {
							return true
						}
						id, isIdent := n.Args[at].(*ast.Ident)
						if code, known := consts[id.Name]; isIdent && known {
							written[code] = true
							return true
						}
						// A forwarding function passing its own parameter on
						// (writeError's code, decodeRequest's malformed).
						if isIdent && isFunc && paramNamed(fd, id.Name) {
							if _, forwards := codeArg[fd.Name.Name]; forwards {
								return true
							}
						}
						inline = append(inline, fset.Position(n.Args[at].Pos()).String())
					case *ast.KeyValueExpr:
						k, ok := n.Key.(*ast.BasicLit)
						builder := isFunc && fd.Name.Name == "writeError"
						if ok && (k.Value == `"error"` || k.Value == `"code"`) && !builder {
							inline = append(inline, fset.Position(n.Pos()).String()+" (a refusal body built beside writeError)")
						}
					}
					return true
				})
			}
		}
	}
	return written, declared, inline
}

func paramNamed(fd *ast.FuncDecl, name string) bool {
	for _, field := range fd.Type.Params.List {
		for _, n := range field.Names {
			if n.Name == name {
				return true
			}
		}
	}
	return false
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
		if !declared[code] && notWritten[code] == "" {
			t.Errorf("registry/errors.json lists %s and the add-on neither writes it nor says why not", code)
		}
		if declared[code] && notWritten[code] != "" {
			t.Errorf("%s is written and also listed as not written", code)
		}
		if !declared[code] {
			continue
		}
		kind := problems[code]
		if kind.title != e.Title || kind.retry != e.Retry {
			t.Errorf("%s: errors.go says title %q retry %q, the registry %q %q", code, kind.title, kind.retry, e.Title, e.Retry)
		}
	}
	if len(problems) != len(declared) {
		t.Errorf("errors.go describes %d codes and declares %d", len(problems), len(declared))
	}
}

// The real handlers, driven into their refusals: each answers a status its
// registry entry lists and a problem that satisfies error.json, with the
// registry's title and Retry-After exactly when the class promises one.
func TestRealRefusalsMatchTheRegistryAndTheErrorSchema(t *testing.T) {
	reg := loadErrorRegistry(t)
	s, _ := applyServer(t, fixtureUsers)
	read := func(h func(http.ResponseWriter, *http.Request, []byte), path, value string) func(*httptest.ResponseRecorder) {
		return func(rr *httptest.ResponseRecorder) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.SetPathValue("field", value)
			req.SetPathValue("name", value)
			h(rr, req, nil)
		}
	}
	for _, tc := range []struct {
		code  string
		drive func(rr *httptest.ResponseRecorder)
	}{
		{CodeBadRequest, func(rr *httptest.ResponseRecorder) {
			postTo(s.handleApply, rr, "/apply", `{"contract_version":2,"call_id":" ","subject":"s","fingerprint":"f","desired":{}}`)
		}},
		{CodeUnknownRequestField, func(rr *httptest.ResponseRecorder) { postTo(s.handleApply, rr, "/apply", `{"surprise":1}`) }},
		{CodeContractVersionMismatch, func(rr *httptest.ResponseRecorder) {
			postTo(s.handlePlan, rr, "/plan", `{"contract_version":1,"subjects":[]}`)
		}},
		{CodeFingerprintRequired, func(rr *httptest.ResponseRecorder) {
			postTo(s.handleApply, rr, "/apply", `{"contract_version":2,"call_id":"c","subject":"s","desired":{}}`)
		}},
		{CodeOperationMismatch, func(rr *httptest.ResponseRecorder) {
			req := httptest.NewRequest(http.MethodPost, "/operations/health.get", nil)
			req.SetPathValue("name", "health.get")
			s.handleOperation(rr, req, []byte(`{"contract_version":2,"call_id":"c","operation":"account.purge"}`))
		}},
		{CodeReasonRequired, func(rr *httptest.ResponseRecorder) {
			postTo(s.handleLifecycle, rr, "/lifecycle", `{"contract_version":2,"call_id":"c","state":"read_only","reason":" "}`)
		}},
		{CodeUnknownLifecycleState, func(rr *httptest.ResponseRecorder) {
			postTo(s.handleLifecycle, rr, "/lifecycle", `{"contract_version":2,"call_id":"c","state":"asleep","reason":"x"}`)
		}},
		{CodeUnknownField, read(s.handleValues, "/values/nope", "nope")},
		{CodeNoSuchSetting, func(rr *httptest.ResponseRecorder) {
			req := httptest.NewRequest(http.MethodPut, "/settings/nope", nil)
			req.SetPathValue("name", "nope")
			s.handlePutSetting(rr, req, []byte(`{}`))
		}},
		{CodeInvalidBody, func(rr *httptest.ResponseRecorder) {
			req := httptest.NewRequest(http.MethodPut, "/settings/home_root", nil)
			req.SetPathValue("name", "home_root")
			s.handlePutSetting(rr, req, []byte(`[`))
		}},
		{CodeUnauthenticated, func(rr *httptest.ResponseRecorder) {
			s.auth.authenticated(s.handleHealth)(rr, httptest.NewRequest(http.MethodGet, "/health", nil))
		}},
		{CodeLifecycleRefusal, func(rr *httptest.ResponseRecorder) { writeLifecycleRefusal(rr, LifecycleReadOnly) }},
		{CodePlanTooLarge, func(rr *httptest.ResponseRecorder) {
			subjects := strings.Repeat(`{"subject":"s","email":"","desired":{}},`, perRequestSubjectCap+1)
			postTo(s.handlePlan, rr, "/plan", `{"contract_version":2,"subjects":[`+strings.TrimSuffix(subjects, ",")+`]}`)
		}},
	} {
		t.Run(tc.code, func(t *testing.T) {
			rr := httptest.NewRecorder()
			tc.drive(rr)
			if got := problemCode(t, rr); got != tc.code {
				t.Fatalf("answered %d %q, want code %s", rr.Code, rr.Body.String(), tc.code)
			}
			entry := reg.Codes[tc.code]
			if !slices.Contains(entry.Statuses, rr.Code) {
				t.Errorf("%s answered %d; registry/errors.json lists %v", tc.code, rr.Code, entry.Statuses)
			}
			if rr.Header().Get("Content-Type") != "application/problem+json" {
				t.Errorf("%s: Content-Type %q", tc.code, rr.Header().Get("Content-Type"))
			}
			if (rr.Header().Get("Retry-After") != "") != (entry.Retry == "retry_after") {
				t.Errorf("%s: Retry-After %q with retry class %s", tc.code, rr.Header().Get("Retry-After"), entry.Retry)
			}
			assertSchema(t, "error.json", rr.Body.Bytes(), tc.code+" reply")
		})
	}
}

func postTo(h func(http.ResponseWriter, *http.Request, []byte), rr *httptest.ResponseRecorder, path, body string) {
	h(rr, httptest.NewRequest(http.MethodPost, path, strings.NewReader(body)), []byte(body))
}
