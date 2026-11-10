package addons

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// One add-on call is one extcall.Do (W1d, one-path 1.9). These pin that the
// transport hands retry/timeout/status rules to extcall and keeps none of its
// own.

func withAddonAttempts(t *testing.T, n int) {
	t.Helper()
	a, b := addonAttempts, addonBackoff
	addonAttempts, addonBackoff = n, time.Millisecond
	t.Cleanup(func() { addonAttempts, addonBackoff = a, b })
}

func credFor(t *testing.T, srvURL string) *credential {
	t.Helper()
	signedAddon(t, srvURL, "an-extcall-path-secret")
	registryMu.Lock()
	a := registry["truenas"]
	registryMu.Unlock()
	cred, err := credentialFor(a.Registration)
	if err != nil {
		t.Fatal(err)
	}
	return cred
}

// By default an add-on call is sent once: the rule before W1d, kept as an
// explicit extcall option.
func TestAnAddonCallIsSentOnceByDefault(t *testing.T) {
	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	cred := credFor(t, srv.URL)
	resp := doAuthenticated(context.Background(), cred, http.MethodGet, srv.URL+"/capabilities", nil, time.Second)
	if n.Load() != 1 || resp.Outcome != OutcomeIndeterminate {
		t.Fatalf("sent %d times, outcome %s; want once, indeterminate", n.Load(), resp.Outcome)
	}
}

// Through extcall: a read is retried on a retryable status, then answered.
func TestAnAddonReadRetriesARetryableStatusThroughExtcall(t *testing.T) {
	withAddonAttempts(t, 3)
	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if n.Add(1) < 3 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	cred := credFor(t, srv.URL)
	resp := doAuthenticated(context.Background(), cred, http.MethodGet, srv.URL+"/capabilities", nil, time.Second)
	if n.Load() != 3 || resp.Outcome != OutcomeSucceeded {
		t.Fatalf("sent %d times, outcome %s; want 3, succeeded", n.Load(), resp.Outcome)
	}
}

// A non-retryable status is final at once, and a write is never repeated after
// an ambiguous 5xx (extcall's safety rule).
func TestAnAddonCallStopsOnANonRetryableStatusAndNeverRepeatsAnAmbiguousWrite(t *testing.T) {
	withAddonAttempts(t, 3)
	for _, tc := range []struct {
		name, method string
		status       int
		want         Outcome
	}{
		{"read 422", http.MethodGet, http.StatusUnprocessableEntity, OutcomeRejected},
		{"write 422", http.MethodPost, http.StatusUnprocessableEntity, OutcomeRejected},
		{"write 500", http.MethodPost, http.StatusInternalServerError, OutcomeIndeterminate},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var n atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				n.Add(1)
				w.WriteHeader(tc.status)
			}))
			defer srv.Close()
			cred := credFor(t, srv.URL)
			resp := doAuthenticated(context.Background(), cred, tc.method, srv.URL+"/x", []byte(`{}`), time.Second)
			if n.Load() != 1 || resp.Outcome != tc.want {
				t.Fatalf("sent %d times, outcome %s; want once, %s", n.Load(), resp.Outcome, tc.want)
			}
		})
	}
}

type OutcomeWant = Outcome

// Guard: the transport never grows its own retry, sleep or timeout again. The
// only dial is the one inside the func handed to extcall.Do.
func TestTransportHasNoRetryLoopSleepOrTimeoutOfItsOwn(t *testing.T) {
	for _, file := range []string{"transport.go", "pipe.go"} {
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		var ops [][2]token.Pos // bodies of the funcs handed to extcall.Do
		var dials []token.Pos
		ast.Inspect(f, func(c ast.Node) bool {
			switch x := c.(type) {
			case *ast.ForStmt:
				if file == "transport.go" {
					t.Errorf("%s: a for loop; retries belong to extcall", fset.Position(x.Pos()))
				}
			case *ast.CallExpr:
				sel, ok := x.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				pkg, _ := sel.X.(*ast.Ident)
				if pkg != nil {
					switch pkg.Name + "." + sel.Sel.Name {
					case "time.Sleep", "time.After", "time.NewTimer", "time.Tick", "time.AfterFunc", "context.WithTimeout", "context.WithDeadline":
						t.Errorf("%s: %s.%s; waits and timeouts belong to extcall", fset.Position(x.Pos()), pkg.Name, sel.Sel.Name)
					case "extcall.Do":
						for _, a := range x.Args {
							if lit, ok := a.(*ast.FuncLit); ok {
								ops = append(ops, [2]token.Pos{lit.Pos(), lit.End()})
							}
						}
						return true
					}
				}
				if sel.Sel.Name == "Do" {
					dials = append(dials, x.Pos())
				}
			}
			return true
		})
		for _, d := range dials {
			in := false
			for _, o := range ops {
				in = in || (d >= o[0] && d < o[1])
			}
			if !in {
				t.Errorf("%s: an HTTP Do outside extcall.Do's op", fset.Position(d))
			}
		}
	}
}
