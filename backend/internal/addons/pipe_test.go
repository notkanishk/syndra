package addons

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"syndra/internal/db"
)

// Guard: the pipe is the only way out. A call site that reaches the transport,
// the breaker or the record writer on its own is the hand-copied triplet this
// file exists to stop — a new endpoint that forgot a step would never trip the
// breaker or leave a trace, and nothing else would notice.
func TestEveryAddonCallGoesThroughThePipe(t *testing.T) {
	// name -> the only files that may call it. credentialFor and the manifest
	// read are registration-time (no Addon, no breaker yet); CircuitOpen only
	// asks the breaker, it sends nothing.
	allowed := map[string][]string{
		"doAuthenticated":        {"pipe.go", "transport.go", "registry.go"},
		"doAuthenticatedWith":    {"pipe.go", "transport.go"},
		"credentialFor":          {"pipe.go", "registry.go", "credentials.go"},
		"allow":                  {"pipe.go", "transport.go"},
		"record":                 {"pipe.go"},
		"dbBeginAddonOperation":  {"pipe.go"},
		"dbSettleAddonOperation": {"pipe.go"},
	}
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	checked := 0
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		checked++
		file, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			var name string
			switch fn := call.Fun.(type) {
			case *ast.Ident:
				name = fn.Name
			case *ast.SelectorExpr:
				name = fn.Sel.Name
				// `record` and `allow` are only the breaker's when called on `.br`.
				if name == "record" || name == "allow" {
					if x, ok := fn.X.(*ast.SelectorExpr); !ok || x.Sel.Name != "br" {
						return true
					}
				}
			}
			files, guarded := allowed[name]
			if !guarded {
				return true
			}
			for _, ok := range files {
				if ok == f {
					return true
				}
			}
			t.Errorf("%s calls %s directly (%s): go through (*Addon).send / operate in pipe.go",
				f, name, fset.Position(call.Pos()))
			return true
		})
	}
	if checked < 10 {
		t.Fatalf("guard scanned %d files; it is looking in the wrong place", checked)
	}
}

type hit struct {
	method, path, key string
}

// recorder is a signed add-on that remembers what reached it, in order, in the
// same list the stubbed row writes append to, so "before" is observable.
func recorder(t *testing.T, status int) (events *[]string, calls *[]hit) {
	t.Helper()
	var mu sync.Mutex
	ev, hs := []string{}, []hit{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		ev = append(ev, "call")
		hs = append(hs, hit{r.Method, r.URL.Path, r.Header.Get(idempotencyHeader)})
		mu.Unlock()
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)
	signedAddon(t, srv.URL, "a-test-secret")
	return &ev, &hs
}

func traceEvents(t *testing.T, events *[]string) {
	t.Helper()
	savedBegin, savedSettle := dbBeginAddonOperation, dbSettleAddonOperation
	dbBeginAddonOperation = func(_ context.Context, p db.AddonOperationParams) (string, error) {
		*events = append(*events, "trace:"+p.Operation)
		return "rec-pipe-" + p.Operation, nil
	}
	dbSettleAddonOperation = func(_ context.Context, id, status string) error {
		*events = append(*events, "settle:"+status)
		return nil
	}
	t.Cleanup(func() { dbBeginAddonOperation, dbSettleAddonOperation = savedBegin, savedSettle })
}

func TestAWriteWithNoTraceIsRefusedBeforeItLeaves(t *testing.T) {
	_, calls := recorder(t, http.StatusOK)
	a, err := Get("truenas")
	if err != nil {
		t.Fatal(err)
	}
	resp, err := a.send(context.Background(), sendSpec{Method: http.MethodPost, Path: "/anything", Write: true})
	if !errors.Is(err, ErrNoCallRecord) || resp.Outcome != OutcomeUnreached || len(*calls) != 0 {
		t.Fatalf("err %v resp %+v calls %d: a write needs its trace first", err, resp, len(*calls))
	}
}

func TestTheTraceIsWrittenBeforeTheCallAndSettledAfter(t *testing.T) {
	events, calls := recorder(t, http.StatusOK)
	traceEvents(t, events)

	if _, ok := PutSetting(context.Background(), "truenas", "home_root", "/mnt/p/home", "op-1"); !ok {
		t.Fatal("setting not answered")
	}
	if _, err := SetLifecycle(context.Background(), "truenas", "draining", "maintenance", "op-1"); err != nil {
		t.Fatal(err)
	}
	want := []string{"trace:setting.put", "call", "settle:succeeded", "trace:lifecycle.set", "call", "settle:succeeded"}
	if strings.Join(*events, ",") != strings.Join(want, ",") {
		t.Fatalf("events = %v, want %v", *events, want)
	}
	// The idempotency key is the row's id, so a retry is recognisable as one call.
	wantKeys := []string{"rec-pipe-setting.put", "rec-pipe-lifecycle.set"}
	for i, c := range *calls {
		if c.key != wantKeys[i] {
			t.Errorf("%s %s carried key %q, want the row id %q", c.method, c.path, c.key, wantKeys[i])
		}
	}
}

// One key, one send: the claim is the row's, and a row can be claimed once, so
// replaying the same key through the pipe reaches the target at most once.
func TestOneIdempotencyKeyReachesTheTargetOnce(t *testing.T) {
	_, calls := recorder(t, http.StatusOK)
	a, _ := Get("truenas")
	rec := OperationRecord("rec-once", "truenas", "setting.put", "home_root")
	spec := sendSpec{Method: http.MethodPut, Path: "/settings/home_root", Write: true,
		Claim: rec.consume, IdempotencyKey: "rec-once"}

	if _, err := a.send(context.Background(), spec); err != nil {
		t.Fatalf("first send: %v", err)
	}
	if _, err := a.send(context.Background(), spec); !errors.Is(err, ErrNoCallRecord) {
		t.Fatalf("second send err = %v, want the spent record refused", err)
	}
	if len(*calls) != 1 || (*calls)[0].key != "rec-once" {
		t.Fatalf("calls = %+v, want exactly one, carrying the key", *calls)
	}
}

func TestApplyCarriesItsCallIdAsTheKey(t *testing.T) {
	_, calls := recorder(t, http.StatusOK)
	Apply(context.Background(), ApplyRequest{Target: "truenas", CallID: "outbox-9", Subject: "s", Fingerprint: "fp"})
	if len(*calls) != 1 || (*calls)[0].path != "/apply" || (*calls)[0].key != "outbox-9" {
		t.Fatalf("calls = %+v", *calls)
	}
}

func TestLifecycleLeavesNoCallWithoutItsRecord(t *testing.T) {
	_, calls := recorder(t, http.StatusOK)
	saved := dbBeginAddonOperation
	dbBeginAddonOperation = func(context.Context, db.AddonOperationParams) (string, error) { return "", errors.New("db down") }
	t.Cleanup(func() { dbBeginAddonOperation = saved })

	if _, err := SetLifecycle(context.Background(), "truenas", "read_only", "why", "op-1"); err == nil {
		t.Fatal("a flip with no record must fail")
	}
	if len(*calls) != 0 {
		t.Fatalf("%d calls left with no record", len(*calls))
	}
}

// The breaker is the pipe's, so a read after the circuit opens never leaves —
// and lifecycle, the operator's way to stop the add-on, still does, without
// feeding the breaker.
func TestBreakerIsConsultedByThePipeAndLifecycleIsExempt(t *testing.T) {
	withBreaker(t, 1, time.Hour)
	var hits int32
	var failing atomic.Bool
	failing.Store(true)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		if failing.Load() && r.URL.Path != "/lifecycle" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	signedAddon(t, srv.URL, "a-test-secret")
	events := []string{}
	traceEvents(t, &events)

	Subjects(context.Background(), "truenas") // opens the circuit (threshold 1)
	before := atomic.LoadInt32(&hits)
	if res := Subjects(context.Background(), "truenas"); !errors.Is(res.Err, ErrCircuitOpen) || atomic.LoadInt32(&hits) != before {
		t.Fatalf("res %+v hits %d->%d: an open circuit must refuse without sending", res, before, hits)
	}
	if _, err := SetLifecycle(context.Background(), "truenas", "draining", "maintenance", "op-1"); err != nil {
		t.Fatalf("lifecycle through an open circuit: %v", err)
	}
	if atomic.LoadInt32(&hits) != before+1 {
		t.Fatalf("lifecycle did not reach the target")
	}
}
