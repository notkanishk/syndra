package handlers

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"syndra/internal/addons"
	"syndra/internal/auth"
	"syndra/internal/db"
	"syndra/internal/services"
	"syndra/internal/services/drift"
)

// The backend's half of design §13.7: every read route against a stub add-on
// in each way of being down (§13.1), through the real pipe. No copy is 503
// TARGET_NOT_READ with no data member; a kept copy is 200 with `read` dated by
// its taken_at; and nothing anywhere is a zero date. The routes are read from
// the router, so a route that reaches an add-on read without a row fails.

// readSeams are the handlers' doors to an add-on read: a route that reaches
// one answers from an add-on.
var readSeams = []string{"addonsReadReach", "addonsReadHomes", "addonsReadSettings", "addonsReadValues",
	"addonsSystemHealth", "addonsActivity", "addonsRead", "addonsHealth", "addonsLastHealth",
	"svcTargetInventory", "svcReconcileAddon", "svcDormantAccounts", "svcRehearseEntitlements"}

// notReadRoutes reach an add-on read and are not §13.3's: each says why.
var notReadRoutes = map[string]string{
	"GET /api/v1/targets/{target}/unconfirmed":             "settles Syndra's own rows from a read and serves the rows (survey §3 row 17)",
	"POST /api/v1/me/targets/{target}/credential":          "a write (design §13.4, slice 8d)",
	"POST /api/v1/me/targets/{target}/operations/{name}":   "a write (design §13.4, slice 8d)",
	"POST /api/v1/targets/{target}/accounts/dormant/sweep": "a write (design §13.4, slice 8d)",
}

// readRow is one route of survey §3 rows 1–5, 8–13, 15–16.
type readRow struct {
	path string
	// none is a reply about Syndra's own records beside the add-on's: 200
	// with `read` none and none of the add-on's members (§13.3), not a 503.
	none bool
	// keeps is a read the add-on serves from its copy while its app is down;
	// keptSkip says why the kept answer is checked elsewhere instead.
	keeps    bool
	keptSkip string
	// readOf finds the route's `read` in a 200.
	readOf func(map[string]any) map[string]any
	// absent are the add-on's members a no-copy 200 must not carry.
	absent []string
	setup  func(t *testing.T)
}

const stubTarget = "truenas"

func topRead(body map[string]any) map[string]any { r, _ := body["read"].(map[string]any); return r }

func firstSystem(body map[string]any) map[string]any {
	ts, _ := body["targets"].([]any)
	if len(ts) != 1 {
		return nil
	}
	sys, _ := ts[0].(map[string]any)
	return sys
}

func systemRead(body map[string]any) map[string]any {
	r, _ := firstSystem(body)["read"].(map[string]any)
	return r
}

func readRows() map[string]readRow {
	return map[string]readRow{
		"GET /api/v1/targets/{target}/inventory": {path: "/api/v1/targets/truenas/inventory", readOf: topRead,
			keptSkip: "a kept inventory reads bindings from Postgres: drift's TestTheInventorySaysHowFreshItsReadIs"},
		"GET /api/v1/targets/{target}/reach":      {path: "/api/v1/targets/truenas/reach", keeps: true, readOf: topRead},
		"GET /api/v1/targets/{target}/homes":      {path: "/api/v1/targets/truenas/homes", keeps: true, readOf: topRead, setup: stubHomesRecords},
		"POST /api/v1/targets/{target}/reconcile": {path: "/api/v1/targets/truenas/reconcile", keeps: true, readOf: topRead, setup: stubReconcileOverPipe},
		"GET /api/v1/targets/{target}/health": {path: "/api/v1/targets/truenas/health?fresh=true", none: true, readOf: topRead,
			absent: []string{"product", "reachable", "lifecycle", "log_head"}, setup: stubNoAnchor},
		"GET /api/v1/me/targets": {path: "/api/v1/me/targets", none: true, keeps: true, readOf: systemRead,
			absent: []string{"storage", "standing"}, setup: stubMemberRecords},
		"GET /api/v1/users/{id}/targets": {path: "/api/v1/users/u1/targets", none: true, keeps: true, readOf: systemRead,
			absent: []string{"storage", "standing"}, setup: stubPersonRecords},
		"GET /api/v1/targets/{target}/mappable-values": {path: "/api/v1/targets/truenas/mappable-values?field=group", readOf: topRead},
		"GET /api/v1/targets/{target}/settings":        {path: "/api/v1/targets/truenas/settings", readOf: topRead},
		"GET /api/v1/targets/{target}/system-health":   {path: "/api/v1/targets/truenas/system-health", readOf: topRead},
		"GET /api/v1/targets/{target}/activity":        {path: "/api/v1/targets/truenas/activity?subject=u1", keeps: true, readOf: topRead},
		"GET /api/v1/targets/{target}/accounts/dormant": {path: "/api/v1/targets/truenas/accounts/dormant", readOf: topRead,
			setup: stubDormantRecords},
		"POST /api/v1/targets/{target}/entitlements/rehearse": {path: "/api/v1/targets/truenas/entitlements/rehearse", readOf: topRead,
			keptSkip: "a kept plan is issued into Postgres: services' rehearsal tests", setup: stubRehearsalOverPipe},
	}
}

// way is one way of the add-on being down (design §13.1), or the app down
// with the add-on's copy (kept).
type way struct {
	name          string
	reason, cause string
	kept          bool
	serve         func(t *testing.T, secret string) string // the add-on's base URL
	// noKey leaves Syndra without the add-on's secret on disk.
	noKey bool
	// after runs once the add-on is installed, before the route is asked.
	after func(t *testing.T)
	// skip says why a row is not this way's, by route.
	skip map[string]string
}

// downThenUp is an add-on that answers an uncoded 500 until up is called,
// then serves every read from its fixtures: what the route then gets is what
// Syndra holds, not what the add-on would say.
func downThenUp() (serve func(t *testing.T, secret string) string, up func()) {
	var ok atomic.Bool
	serve = func(t *testing.T, secret string) string {
		ok.Store(false)
		healthy := keptCopiesHandler(t, false)
		return stubServer(t, secret, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !ok.Load() {
				w.WriteHeader(500)
				return
			}
			healthy.ServeHTTP(w, r)
		})).URL
	}
	return serve, func() { ok.Store(true) }
}

func ways() []way {
	ws := []way{
		{name: "closed port", reason: addons.ReasonAddonUnreachable, cause: addons.CauseNoAnswer, serve: func(t *testing.T, secret string) string {
			srv := stubServer(t, secret, http.NotFoundHandler())
			srv.Close()
			return srv.URL
		}},
		{name: "blackhole", reason: addons.ReasonAddonUnreachable, cause: addons.CauseNoAnswer, serve: blackhole},
		{name: "wrong key", reason: addons.ReasonAddonUnreachable, cause: addons.CauseWrongHost, serve: func(t *testing.T, _ string) string {
			return stubServer(t, "another-deployments-secret", http.NotFoundHandler()).URL
		}},
		{name: "hang", reason: addons.ReasonAddonUnreachable, cause: addons.CauseTimeout, serve: func(t *testing.T, secret string) string {
			return stubServer(t, secret, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				select { // past the call's deadline, then let the server close
				case <-r.Context().Done():
				case <-time.After(3 * time.Second):
				}
			})).URL
		}},
		{name: "uncoded 500", reason: addons.ReasonAddonUnreachable, cause: addons.CauseFault, serve: func(t *testing.T, secret string) string {
			return stubServer(t, secret, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(500) })).URL
		}},
		{name: "app down, copy kept", reason: addons.ReasonAppUnreachable, kept: true, serve: keptCopies},
		{name: "no key", reason: addons.ReasonAddonUnreachable, cause: addons.CauseNoKey, noKey: true,
			serve: func(t *testing.T, secret string) string { return stubServer(t, secret, http.NotFoundHandler()).URL }},
		{name: "cooldown, nothing kept", reason: addons.ReasonCooldown, serve: func(t *testing.T, secret string) string {
			return stubServer(t, secret, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/problem+json")
				w.Header().Set("Retry-After", "9")
				w.WriteHeader(http.StatusTooManyRequests)
				fmt.Fprint(w, `{"type":"urn:syndra:addon:error:FRESH_READ_COOLDOWN","title":"Read moments ago","status":429,"code":"FRESH_READ_COOLDOWN","retry":"after"}`)
			})).URL
		}},
	}
	// Held: the monitor's probes missed, and the add-on is back before its
	// next probe. A read is answered with the monitor's why, undialed; a live
	// health read is the probe's own kind, which finds the add-on back.
	heldServe, heldUp := downThenUp()
	ws = append(ws, way{name: "held by the monitor", reason: addons.ReasonAddonUnreachable, cause: addons.CauseFault, serve: heldServe,
		after: func(t *testing.T) {
			for range 3 { // the monitor's failureThreshold, below the breaker's
				_ = addons.ProbeAll(context.Background())
			}
			heldUp()
		},
		skip: map[string]string{"GET /api/v1/targets/{target}/health": "a live health read is the probe's kind: it dials, and answers"}})
	// Open breaker: five reads failed, and the add-on is back inside the
	// breaker's cooldown. Every call is refused undialed with what opened it.
	openServe, openUp := downThenUp()
	ws = append(ws, way{name: "breaker open", reason: addons.ReasonAddonUnreachable, cause: addons.CauseFault, serve: openServe,
		after: func(t *testing.T) {
			for range 5 { // breakerThreshold
				_ = addons.Subjects(context.Background(), stubTarget, false)
			}
			openUp()
		}})
	// Each coded 503 registry/errors.json allows: the app's three are the
	// app's outage, the rest the add-on answering about the request.
	for code, reason := range map[string]string{
		"TARGET_UNREACHABLE": addons.ReasonAppUnreachable, "NO_CURRENT_READ": addons.ReasonAppUnreachable,
		"TARGET_UNREADABLE": addons.ReasonAppUnreachable, "TARGET_CHANGED": addons.ReasonReadRefused,
		"LIFECYCLE_REFUSAL": addons.ReasonReadRefused, "OPERATION_FAILED": addons.ReasonReadRefused, "APPLY_FAILED": addons.ReasonReadRefused,
	} {
		ws = append(ws, way{name: "coded 503 " + code, reason: reason, serve: func(t *testing.T, secret string) string {
			return stubServer(t, secret, problem503(code)).URL
		}})
	}
	return ws
}

// keptAt is when the stub's copy was read: minutes old, in whole seconds.
var keptAt = time.Now().Add(-7 * time.Minute).UTC().Truncate(time.Second)

func TestEveryReadAnswersOneWay(t *testing.T) {
	rows := readRows()
	for route := range addonReadRoutes(t) {
		if _, ok := rows[route]; !ok && notReadRoutes[route] == "" {
			t.Errorf("%s reaches an add-on read and has no row: add it to readRows, or say why in notReadRoutes", route)
		}
	}
	for route := range rows {
		if !addonReadRoutes(t)[route] {
			t.Errorf("%s has a row and the router does not reach an add-on read from it", route)
		}
	}
	callTimeout := 1500 * time.Millisecond
	t.Cleanup(addons.HandshakeTimeoutForTesting(200 * time.Millisecond))
	for _, w := range ways() {
		for route, row := range rows {
			t.Run(w.name+"/"+route, func(t *testing.T) {
				if w.kept && !row.keeps && row.keptSkip != "" {
					t.Skip(row.keptSkip)
				}
				if why := w.skip[route]; why != "" {
					t.Skip(why)
				}
				secret := "a-reachability-secret"
				url := w.serve(t, secret)
				keyFile := filepath.Join(t.TempDir(), "truenas.key")
				reg := addons.Registration{Target: stubTarget, BaseURL: url, SecretPath: keyFile}
				if !w.noKey {
					if err := os.WriteFile(keyFile, []byte(secret), 0o600); err != nil {
						t.Fatal(err)
					}
					reg.Secret = []byte(secret)
				}
				t.Cleanup(addons.InstallForTesting(reg, stubManifest(t)))
				stubRouteRecords(t)
				if row.setup != nil {
					row.setup(t)
				}
				if w.after != nil {
					w.after(t)
				}
				method, _, _ := strings.Cut(route, " ")
				body := ""
				if method == http.MethodPost {
					body = `{"subject_ids":["u1"]}`
				}
				ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
				defer cancel()
				req := httptest.NewRequestWithContext(withPrincipal(ctx, &auth.Principal{Subject: "u1"}), method, row.path, strings.NewReader(body))
				req.SetPathValue("target", stubTarget)
				req.SetPathValue("id", "u1")
				rr := httptest.NewRecorder()
				handlerOf(t, route)(rr, req)

				var got map[string]any
				if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
					t.Fatalf("%d %s: %v", rr.Code, rr.Body, err)
				}
				noFalseZero(t, "", got)
				switch {
				case w.kept && row.keeps:
					// A kept copy: 200, `read` kept, dated by the add-on's taken_at.
					read := row.readOf(got)
					if rr.Code != http.StatusOK || read["state"] != addons.ReadKept || read["reason"] != addons.ReasonAppUnreachable {
						t.Fatalf("a kept copy: want 200 read kept app_unreachable, got %d %s", rr.Code, rr.Body)
					}
					at, err := time.Parse(time.RFC3339, fmt.Sprint(read["taken_at"]))
					if err != nil || at.Sub(keptAt).Abs() > 3*time.Second {
						t.Fatalf("a kept copy is dated by its taken_at %s, got %v", keptAt, read["taken_at"])
					}
				case row.none:
					read := row.readOf(got)
					if rr.Code != http.StatusOK || read["state"] != addons.ReadNone || read["reason"] != w.reason || fmt.Sprint(read["cause"]) != orNil(w.cause) {
						t.Fatalf("no copy beside Syndra's records: want 200 read none %s/%s, got %d %s", w.reason, w.cause, rr.Code, rr.Body)
					}
					holder := got
					if sys := firstSystem(got); sys != nil {
						holder = sys
					}
					for _, k := range row.absent {
						if _, ok := holder[k]; ok {
							t.Errorf("no copy carries the add-on's %s: %s", k, rr.Body)
						}
					}
					noEmptyValue(t, "", read)
				default:
					assertNotRead(t, rr, got, w.reason, w.cause)
				}
			})
		}
	}
}

// assertNotRead is §13.3's no copy: 503, Retry-After, TARGET_NOT_READ in the
// one envelope with details {reason, cause}, and no data member at all.
func assertNotRead(t *testing.T, rr *httptest.ResponseRecorder, got map[string]any, reason, cause string) {
	t.Helper()
	if rr.Code != http.StatusServiceUnavailable || rr.Header().Get("Retry-After") == "" || got["error"] != codeTargetNotRead {
		t.Fatalf("no copy: want 503 TARGET_NOT_READ with Retry-After, got %d %q %s", rr.Code, rr.Header().Get("Retry-After"), rr.Body)
	}
	details, _ := got["details"].(map[string]any)
	if details["reason"] != reason || fmt.Sprint(details["cause"]) != orNil(cause) {
		t.Fatalf("no copy: want %s/%s, got %v", reason, cause, details)
	}
	for k := range got {
		if k != "error" && k != "message" && k != "details" && k != "retry_after" {
			t.Errorf("no copy carries a data member %q: %s", k, rr.Body)
		}
	}
	if m, _ := got["message"].(string); m == "" {
		t.Error("no copy says why in a sentence")
	}
}

// noEmptyValue walks a none `read`: a value not read is absent, never a 0,
// an empty list or "" standing in for it.
func noEmptyValue(t *testing.T, at string, v any) {
	t.Helper()
	switch x := v.(type) {
	case map[string]any:
		for k, c := range x {
			noEmptyValue(t, at+"."+k, c)
		}
	case []any:
		if len(x) == 0 {
			t.Errorf("%s is an empty list", at)
		}
		for i, c := range x {
			noEmptyValue(t, at+"["+strconv.Itoa(i)+"]", c)
		}
	case string:
		if x == "" {
			t.Errorf("%s is empty", at)
		}
	case float64:
		if x == 0 {
			t.Errorf("%s is 0", at)
		}
	}
}

func orNil(s string) string {
	if s == "" {
		return "<nil>"
	}
	return s
}

// noFalseZero walks a reply: no date anywhere is Go's zero time, and no
// taken_at is missing its value (design §13.7's walker, the backend's half).
func noFalseZero(t *testing.T, at string, v any) {
	t.Helper()
	switch x := v.(type) {
	case map[string]any:
		for k, c := range x {
			if k == "taken_at" && (c == nil || c == "") {
				t.Errorf("%s.taken_at is empty", at)
			}
			noFalseZero(t, at+"."+k, c)
		}
	case []any:
		for i, c := range x {
			noFalseZero(t, at+"["+strconv.Itoa(i)+"]", c)
		}
	case string:
		if ts, err := time.Parse(time.RFC3339, x); err == nil && ts.Year() <= 1 {
			t.Errorf("%s is the zero date %q", at, x)
		}
	}
}

// ---- the router ----

// addonReadRoutes is every route of router.go whose handler reaches a read
// seam, within this package.
func addonReadRoutes(t *testing.T) map[string]bool {
	t.Helper()
	funcs, router := parsePackage(t)
	seams := map[string]bool{}
	for _, s := range readSeams {
		seams[s] = true
	}
	memo := map[string]bool{}
	var reaches func(name string, seen map[string]bool) bool
	reaches = func(name string, seen map[string]bool) bool {
		if v, ok := memo[name]; ok {
			return v
		}
		seen[name] = true
		found := false
		ast.Inspect(funcs[name], func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && !found {
				found = seams[id.Name] || (funcs[id.Name] != nil && !seen[id.Name] && reaches(id.Name, seen))
			}
			return !found
		})
		memo[name] = found
		return found
	}
	out := map[string]bool{}
	for route, handler := range routesOf(router) {
		if funcs[handler] != nil && reaches(handler, map[string]bool{}) {
			out[route] = true
		}
	}
	return out
}

var parsed struct {
	sync.Once
	funcs  map[string]*ast.FuncDecl
	router *ast.File
}

func parsePackage(t *testing.T) (map[string]*ast.FuncDecl, *ast.File) {
	t.Helper()
	parsed.Do(func() {
		files, _ := filepath.Glob("*.go")
		fset := token.NewFileSet()
		parsed.funcs = map[string]*ast.FuncDecl{}
		for _, f := range files {
			if strings.HasSuffix(f, "_test.go") {
				continue
			}
			af, err := parser.ParseFile(fset, f, nil, 0)
			if err != nil {
				panic(err)
			}
			if f == "router.go" {
				parsed.router = af
			}
			for _, d := range af.Decls {
				if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv == nil {
					parsed.funcs[fd.Name.Name] = fd
				}
			}
		}
	})
	return parsed.funcs, parsed.router
}

// routesOf is each `mux.HandleFunc("METHOD /path", ...handleX...)`: route to
// the handler's name.
func routesOf(router *ast.File) map[string]string {
	out := map[string]string{}
	ast.Inspect(router, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) < 2 {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		lit, isLit := call.Args[0].(*ast.BasicLit)
		if !ok || !isLit || sel.Sel.Name != "HandleFunc" {
			return true
		}
		route, _ := strconv.Unquote(lit.Value)
		ast.Inspect(call.Args[1], func(m ast.Node) bool {
			if id, ok := m.(*ast.Ident); ok && strings.HasPrefix(id.Name, "handle") {
				out[route] = id.Name
			}
			return true
		})
		return true
	})
	return out
}

// handlerOf is the route's handler, by the router's name for it.
func handlerOf(t *testing.T, route string) http.HandlerFunc {
	t.Helper()
	_, router := parsePackage(t)
	h, ok := map[string]http.HandlerFunc{
		"handleTargetInventory": handleTargetInventory, "handleTargetReach": handleTargetReach,
		"handleTargetHomes": handleTargetHomes, "handleReconcileTarget": handleReconcileTarget,
		"handleTargetHealth": handleTargetHealth, "handleMyTargets": handleMyTargets, "handleUserTargets": handleUserTargets,
		"handleMappableValues": handleMappableValues, "handleTargetSettings": handleTargetSettings,
		"handleTargetSystemHealth": handleTargetSystemHealth, "handleTargetActivity": handleTargetActivity,
		"handleDormantAccounts": handleDormantAccounts, "handleRehearseEntitlements": handleRehearseEntitlements,
	}[routesOf(router)[route]]
	if !ok {
		t.Fatalf("%s: no handler known by the router's name for it", route)
	}
	return h
}

// ---- the stub add-on ----

func stubManifest(t *testing.T) addons.Manifest {
	t.Helper()
	var m addons.Manifest
	if err := json.Unmarshal(fixture(t, "manifest_truenas.json"), &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "addons", "contract", "v2", "fixtures", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// stubServer is an add-on on TLS with the key derived from secret, as the
// add-on serves it at boot.
func stubServer(t *testing.T, secret string, h http.Handler) *httptest.Server {
	t.Helper()
	cert, err := addons.CertificateForTesting([]byte(secret), stubTarget)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewUnstartedServer(h)
	srv.TLS = &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS13}
	srv.StartTLS()
	t.Cleanup(srv.Close)
	return srv
}

// blackhole accepts the connection and never says a word.
func blackhole(t *testing.T, _ string) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var held []net.Conn
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			held = append(held, c)
			mu.Unlock()
		}
	}()
	t.Cleanup(func() {
		ln.Close()
		mu.Lock()
		for _, c := range held {
			c.Close()
		}
		mu.Unlock()
	})
	return "https://" + ln.Addr().String()
}

func problem503(code string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintf(w, `{"type":"urn:syndra:addon:error:%s","title":"Not now","status":503,"code":%q,"retry":"backoff"}`, code, code)
	})
}

// keptCopies is the add-on with its app down: the reads it keeps a copy of
// answer it (current false, stale unreachable, the copy's taken_at); the rest
// answer TARGET_UNREACHABLE.
func keptCopies(t *testing.T, secret string) string {
	t.Helper()
	return stubServer(t, secret, keptCopiesHandler(t, true)).URL
}

// keptCopiesHandler serves the fixtures: as kept copies with the app down
// (kept), or as read now.
func keptCopiesHandler(t *testing.T, stale bool) http.Handler {
	t.Helper()
	kept := func(name string, inResult bool) []byte {
		var doc map[string]any
		if err := json.Unmarshal(fixture(t, name), &doc); err != nil {
			t.Fatal(err)
		}
		r := doc
		if inResult {
			r = doc["result"].(map[string]any)
		}
		if stale {
			r["current"], r["taken_at"], r["stale"] = false, keptAt.Format(time.RFC3339), map[string]any{"reason": "unreachable"}
		}
		b, _ := json.Marshal(doc)
		return b
	}
	copies := map[string][]byte{
		"/subjects":                  kept("subjects_stale.json", false),
		"/reach":                     kept("reach.json", false),
		"/homes":                     kept("homes.json", false),
		"/operations/storage.status": kept("operation_response_result.json", true),
		"/operations/activity.get":   kept("operation_response_activity_result.json", true),
	}
	down := problem503("TARGET_UNREACHABLE")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if body, ok := copies[r.URL.Path]; ok {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(body)
			return
		}
		down.ServeHTTP(w, r)
	})
}

// ---- Syndra's own records, which no row here is about ----

func stubRouteRecords(t *testing.T) {
	t.Helper()
	shape := addonsAcceptedShape
	t.Cleanup(func() { addonsAcceptedShape = shape })
	addonsAcceptedShape = func(context.Context, string) (addons.Shape, error) {
		return addons.Shape{Kind: "storage", EntitlementSchema: truenasSchema}, nil
	}
}

func stubHomesRecords(t *testing.T) {
	t.Helper()
	bindings, resolve, inForce, forSubject := dbListTargetBindings, svcResolveEntitlementSet, dbAllowancesInForce, dbAllowancesForSubject
	t.Cleanup(func() {
		dbListTargetBindings, svcResolveEntitlementSet, dbAllowancesInForce, dbAllowancesForSubject = bindings, resolve, inForce, forSubject
	})
	dbListTargetBindings = func(context.Context, string) ([]db.TargetBinding, error) { return nil, nil }
	svcResolveEntitlementSet = func(context.Context, string, string) (services.EntitlementSet, error) {
		return services.EntitlementSet{}, nil
	}
	dbAllowancesInForce = func(context.Context, string, string) ([]db.Allowance, error) { return nil, nil }
	dbAllowancesForSubject = func(context.Context, string) ([]db.Allowance, error) { return nil, nil }
}

func stubNoAnchor(t *testing.T) {
	t.Helper()
	anchor := dbGetLogAnchor
	t.Cleanup(func() { dbGetLogAnchor = anchor })
	dbGetLogAnchor = func(context.Context, string) (db.LogAnchor, bool, error) { return db.LogAnchor{}, false, nil }
}

// stubMemberRecords is a member with an account: what Syndra records of them,
// with every add-on read left real.
func stubMemberRecords(t *testing.T) {
	t.Helper()
	read, reads, reach, health, healthOf, callable := addonsRead, addonsMemberReads, addonsReadReach, addonsHealth, addonsHealthOf, addonsCallable
	stubMyStorage(t, &storageHarness{entitled: true, bound: true})
	addonsRead, addonsMemberReads, addonsReadReach, addonsHealth, addonsHealthOf, addonsCallable = read, reads, reach, health, healthOf, callable
	bound := time.Date(2026, 9, 12, 10, 0, 0, 0, time.UTC)
	dbGetTargetBinding = func(context.Context, string, string) (db.TargetBinding, bool, error) {
		return db.TargetBinding{Target: stubTarget, SubjectID: "u1", Username: "charlotte", BoundAt: bound}, true, nil
	}
}

func stubPersonRecords(t *testing.T) {
	t.Helper()
	read, reads, reach, health, healthOf, callable := addonsRead, addonsMemberReads, addonsReadReach, addonsHealth, addonsHealthOf, addonsCallable
	registered := addonsRegistered
	stubPersonStorage(t, &personStorageHarness{storageHarness: storageHarness{entitled: true, bound: true}})
	addonsRead, addonsMemberReads, addonsReadReach, addonsHealth, addonsHealthOf, addonsCallable = read, reads, reach, health, healthOf, callable
	addonsRegistered = registered
	dbAllowancesInForce = func(context.Context, string, string) ([]db.Allowance, error) { return nil, nil }
}

func stubDormantRecords(t *testing.T) {
	t.Helper()
	// The listing reads the add-on first and Postgres only from a usable read,
	// which no way here gives.
}

// stubReconcileOverPipe and stubRehearsalOverPipe stand in for the two
// services whose first step is Postgres (the read's start on the database's
// clock; the cohort's entitlement sets): each makes the service's one add-on
// read through the real pipe and maps it as the service does, so the pipe's
// classes and the handler's answer are what is tested here. The services'
// own mapping is tested where they live (drift, services).
func stubReconcileOverPipe(t *testing.T) {
	t.Helper()
	orig := svcReconcileAddon
	t.Cleanup(func() { svcReconcileAddon = orig })
	svcReconcileAddon = func(ctx context.Context, target string) (drift.AddonReconcileResult, error) {
		read := addons.Subjects(ctx, target, true)
		return drift.AddonReconcileResult{Target: target, Read: read.Read(), Halted: !read.Usable()}, nil
	}
}

func stubRehearsalOverPipe(t *testing.T) {
	t.Helper()
	orig := svcRehearseEntitlements
	t.Cleanup(func() { svcRehearseEntitlements = orig })
	svcRehearseEntitlements = func(ctx context.Context, req services.EntitlementRehearsal) (services.EntitlementPlan, error) {
		answer := addons.Plan(ctx, req.Target, []addons.PlanSubject{{Subject: "u1"}})
		if answer.Outcome != addons.OutcomeSucceeded {
			return services.EntitlementPlan{}, fmt.Errorf("%w: %w", services.ErrTargetUnplannable, addons.NotRead{Why: answer.Why})
		}
		return services.EntitlementPlan{}, fmt.Errorf("a plan was answered on a way that has none")
	}
}
