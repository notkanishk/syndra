package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"syndra/internal/cache"
	"syndra/internal/claims"
	"syndra/internal/claimtel"
	"syndra/internal/db"
	"syndra/internal/services"
)

// callLog captures every call the data-plane wrapper counts.
type callLog struct {
	mu    sync.Mutex
	calls []claimtel.Call
}

func (l *callLog) last(t *testing.T) claimtel.Call {
	t.Helper()
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.calls) == 0 {
		t.Fatal("no call was counted")
	}
	return l.calls[len(l.calls)-1]
}

func captureCalls(t *testing.T) *callLog {
	t.Helper()
	l := &callLog{}
	orig := recordClaimCall
	recordClaimCall = func(c claimtel.Call) {
		l.mu.Lock()
		l.calls = append(l.calls, c)
		l.mu.Unlock()
	}
	t.Cleanup(func() { recordClaimCall = orig })
	return l
}

// postInject sends a body through the real router, so the wrapper, the
// signature check and every middleware are on the path as in production.
func postInject(t *testing.T, body string, header http.Header) (*httptest.ResponseRecorder, time.Duration) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/action/inject", strings.NewReader(body))
	for k, v := range header {
		req.Header[k] = v
	}
	rr := httptest.NewRecorder()
	start := time.Now()
	NewRouter().ServeHTTP(rr, req)
	return rr, time.Since(start)
}

func devMode(t *testing.T) {
	t.Setenv("ZITADEL_DOMAIN", "")
	t.Setenv("ZITADEL_ACTION_SIGNING_KEY", "")
}

func body(function string, grants string, extra string) string {
	s := fmt.Sprintf(`{"function":%q,"user":{"id":"u1"},"application":{"client_id":"c1"}`, function)
	if grants != "" {
		s += `,"user_grants":` + grants
	}
	return s + extra + "}"
}

const pre = claims.FunctionPreAccessToken

// Every path through the route lands in its own class.
func TestClaimDelivery_EveryPathHasItsOutcome(t *testing.T) {
	devMode(t)
	accessOnly := claims.Builtin()
	accessOnly.Lands = claims.LandsAccessToken

	cases := []struct {
		name    string
		body    string
		facts   func(context.Context, string) (string, error)
		shapes  *services.ProjectShapes
		shapeKV func(context.Context, string) (string, error)
		want    claimtel.Outcome
		status  int
	}{
		{name: "served", body: body(pre, `[{"project_id":"p1","roles":["admin"]}]`, ""), want: claimtel.Served, status: 200},
		{name: "unknown function", body: body("function/somethingnew", `[{"project_id":"p1"}]`, ""), want: claimtel.UnknownFunction, status: 200},
		{name: "not landing", body: body(claims.FunctionPreUserinfo, `[{"project_id":"p1"}]`, ""),
			shapes: &services.ProjectShapes{Default: accessOnly}, want: claimtel.NotLanding, status: 200},
		{name: "no grants (absent, Zitadel's omitempty)", body: body(pre, "", `,"userinfo":{"sub":"u1"}`), want: claimtel.NoGrants, status: 200},
		{name: "no grants (null)", body: body(pre, "null", ""), want: claimtel.NoGrants, status: 200},
		{name: "renamed grants list", body: body(pre, "", `,"user_grant_list":[{"project_id":"p1"}]`), want: claimtel.ContractFault, status: 200},
		{name: "rows name no project (F-0)", body: body(pre, `[{"projectID_v5":"p1"}]`, ""), want: claimtel.ContractFault, status: 200},
		{name: "grants not a list", body: body(pre, `{"project_id":"p1"}`, ""), want: claimtel.ContractFault, status: 200},
		{name: "user.id missing", body: `{"function":"function/preaccesstoken","user":{}}`, want: claimtel.ContractFault, status: 400},
		{name: "cross project", body: body(pre, `[{"project_id":"p1"},{"project_id":"p2"}]`, ""), want: claimtel.RefusedCrossProject, status: 200},
		{name: "no facts", body: body(pre, `[{"project_id":"p1"}]`, ""),
			facts: func(context.Context, string) (string, error) { return "", redis.Nil }, want: claimtel.DegradedNoFacts, status: 200},
		{name: "malformed facts", body: body(pre, `[{"project_id":"p1"}]`, ""),
			facts: func(context.Context, string) (string, error) { return "{{", nil }, want: claimtel.DegradedMalformed, status: 200},
		{name: "shape cache unreadable", body: body(pre, `[{"project_id":"p1"}]`, ""),
			shapeKV: func(context.Context, string) (string, error) { return "", errors.New("connection refused") },
			want:    claimtel.DegradedShapeUnreadable, status: 200},
		{name: "not JSON", body: `not-json`, want: claimtel.BadRequest, status: 400},
		{name: "JSON but not an object", body: `[1]`, want: claimtel.BadRequest, status: 400},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resetActionDeps(t, redisGetClaims, dbGetClaimFailureMode)
			log := captureCalls(t)
			redisGetClaims = func(context.Context, string) (string, error) { return factsJSON("u1", "p1", "admin"), nil }
			if tc.facts != nil {
				redisGetClaims = tc.facts
			}
			if tc.shapes != nil {
				stubProjectShapes(*tc.shapes)
			}
			pgShapes := 0
			if tc.shapeKV != nil {
				redisGetKey = tc.shapeKV
				dbResolveProjectShapes = func(context.Context, string) (services.ProjectShapes, error) {
					pgShapes++
					return services.ProjectShapes{Default: claims.Builtin()}, nil
				}
			}
			dbGetClaimFailureMode = func(context.Context, string) (string, map[string]interface{}, error) { return "fail_closed", nil, nil }

			rr, _ := postInject(t, tc.body, nil)
			got := log.last(t)
			if got.Outcome != tc.want || rr.Code != tc.status {
				t.Fatalf("outcome %s / HTTP %d, want %s / %d (body %s)", got.Outcome, rr.Code, tc.want, tc.status, rr.Body.String())
			}
			if pgShapes != 0 {
				t.Errorf("Redis erroring (not missing) still read Postgres for the shapes (F-10)")
			}
			if tc.want == claimtel.Served && (got.ProjectID != "p1" || got.ClientID != "c1" || got.Function != pre || len(got.Body) == 0) {
				t.Errorf("dimensions not carried: %+v", got)
			}
		})
	}
}

// A miss is a repair signal; Redis erroring is not (a compile cannot help it).
func TestClaimDelivery_MissEnqueuesARepair(t *testing.T) {
	devMode(t)
	resetActionDeps(t, redisGetClaims, dbGetClaimFailureMode)
	captureCalls(t)
	dbGetClaimFailureMode = func(context.Context, string) (string, map[string]interface{}, error) { return "fail_closed", nil, nil }

	redisGetClaims = func(context.Context, string) (string, error) { return "", redis.Nil }
	postInject(t, `{"function":"function/preaccesstoken","user":{"id":"uMiss"},"user_grants":[{"project_id":"p1"}]}`, nil)
	if _, ok := claimRepair.pending.Load("uMiss"); !ok {
		t.Error("a facts miss did not ask for a recompile")
	}
	redisGetClaims = func(context.Context, string) (string, error) { return "", errors.New("i/o timeout") }
	postInject(t, `{"function":"function/preaccesstoken","user":{"id":"uDown"},"user_grants":[{"project_id":"p1"}]}`, nil)
	if _, ok := claimRepair.pending.Load("uDown"); ok {
		t.Error("Redis being down asked for a recompile")
	}
	claimRepair.pending.Delete("uMiss")
}

// The signature check is inside the counted route: each way it fails has its
// class, and a refused call names no app (its body is nobody's word).
func TestClaimDelivery_SignatureOutcomes(t *testing.T) {
	const key = "0123456789abcdef0123456789abcdef"
	t.Setenv("ZITADEL_DOMAIN", "zitadel.example.edu")
	t.Setenv("ZITADEL_ACTION_SIGNING_KEY", key)
	resetActionDeps(t, redisGetClaims, dbGetClaimFailureMode)
	log := captureCalls(t)
	redisGetClaims = func(context.Context, string) (string, error) { return factsJSON("u1", "p1", "admin"), nil }
	b := body(pre, `[{"project_id":"p1"}]`, "")
	sign := func(at time.Time, secret string) http.Header {
		return http.Header{"Zitadel-Signature": {fmt.Sprintf("t=%d,v1=%x", at.Unix(), computeZitadelActionSignature(at, []byte(b), secret))}}
	}

	for _, tc := range []struct {
		name   string
		header http.Header
		want   claimtel.Outcome
		status int
	}{
		{"unsigned", nil, claimtel.SigMissing, 401},
		{"stale", sign(time.Now().Add(-10*time.Minute), key), claimtel.SigStale, 401},
		{"wrong key", sign(time.Now(), "another-key-another-key-another!!"), claimtel.SigMismatch, 401},
		{"garbled", http.Header{"Zitadel-Signature": {"t=abc,v1=zz"}}, claimtel.SigMismatch, 401},
		{"good", sign(time.Now(), key), claimtel.Served, 200},
	} {
		rr, _ := postInject(t, b, tc.header)
		got := log.last(t)
		if got.Outcome != tc.want || rr.Code != tc.status {
			t.Fatalf("%s: outcome %s / HTTP %d, want %s / %d", tc.name, got.Outcome, rr.Code, tc.want, tc.status)
		}
		if tc.status == 401 && (got.ClientID != "" || got.Function != "" || got.KeySlot != "") {
			t.Errorf("%s: a refused call carried dimensions: %+v", tc.name, got)
		}
		if tc.want == claimtel.Served && got.KeySlot != claimtel.KeySlotCurrent {
			t.Errorf("verified call key slot = %q", got.KeySlot)
		}
	}
}

// ---------------------------------------------------------------------------
// Fault injection (assessment §4h): under any single dependency fault the
// route answers well inside Zitadel's 3s, with a defined outcome. Each case
// logs its measured time.
// ---------------------------------------------------------------------------

// realRedis points the data plane's real Redis closures at addr, through the
// production client options (db.NewRedisClient).
func realRedis(t *testing.T, addr string) {
	t.Helper()
	orig := db.Redis
	db.Redis = db.NewRedisClient(addr)
	t.Cleanup(func() { _ = db.Redis.Close(); db.Redis = orig })
}

// hungRedis accepts connections and never answers: a stopped process, an AOF
// fsync stall, a half-open TCP connection.
func hungRedis(t *testing.T) string {
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
		_ = ln.Close()
		mu.Lock()
		for _, c := range held {
			_ = c.Close()
		}
		mu.Unlock()
	})
	return ln.Addr().String()
}

// refusedRedis is an address nothing listens on.
func refusedRedis(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	return addr
}

// slowUntilCancelled is Postgres that answers only after the caller gave up:
// a lock, a saturated pool, a stalled disk.
func slowShapes(ctx context.Context, _ string) (services.ProjectShapes, error) {
	select {
	case <-ctx.Done():
		return services.ProjectShapes{}, ctx.Err()
	case <-time.After(5 * time.Second):
		return services.ProjectShapes{Default: claims.Builtin()}, nil
	}
}

func slowMode(ctx context.Context, _ string) (string, map[string]interface{}, error) {
	select {
	case <-ctx.Done():
		return "", nil, ctx.Err()
	case <-time.After(5 * time.Second):
		return "minimal_safe", map[string]interface{}{"role": "guest"}, nil
	}
}

const faultBudget = 300 * time.Millisecond

func TestClaimDelivery_FaultInjection(t *testing.T) {
	devMode(t)
	b := body(pre, `[{"project_id":"p1","roles":["admin"]}]`, "")
	minimalSafe := func(context.Context, string) (string, map[string]interface{}, error) {
		return "minimal_safe", map[string]interface{}{"role": "guest"}, nil
	}

	for _, tc := range []struct {
		name      string
		setup     func(t *testing.T)
		want      claimtel.Outcome
		status    int
		wantGuest bool // the project's minimal_safe answer still reached the token
	}{
		{name: "redis hung", setup: func(t *testing.T) {
			realRedis(t, hungRedis(t))
			dbResolveProjectShapes = slowShapes // must never be reached: F-10
			dbGetClaimFailureMode = minimalSafe
		}, want: claimtel.DegradedShapeUnreadable, status: 200, wantGuest: true},
		{name: "redis refused", setup: func(t *testing.T) {
			realRedis(t, refusedRedis(t))
			dbResolveProjectShapes = slowShapes
			dbGetClaimFailureMode = minimalSafe
		}, want: claimtel.DegradedShapeUnreadable, status: 200, wantGuest: true},
		{name: "postgres slow", setup: func(t *testing.T) {
			stubMissingRedis()
			dbResolveProjectShapes = slowShapes
			dbGetClaimFailureMode = slowMode
		}, want: claimtel.DeadlineExceeded, status: 200},
		{name: "postgres failing", setup: func(t *testing.T) {
			stubMissingRedis()
			dbResolveProjectShapes = func(context.Context, string) (services.ProjectShapes, error) {
				return services.ProjectShapes{}, errors.New("connection refused")
			}
			dbGetClaimFailureMode = func(context.Context, string) (string, map[string]interface{}, error) {
				return "", nil, errors.New("connection refused")
			}
		}, want: claimtel.DegradedShapeUnreadable, status: 200},
		{name: "panic in the shaper", setup: func(t *testing.T) {
			stubMissingRedis()
			redisGetClaims = func(context.Context, string) (string, error) { return factsJSON("u1", "p1", "admin"), nil }
			stubProjectShapes(services.ProjectShapes{Default: claims.Builtin()})
			shapeClaims = func(claims.Profile, claims.Facts) *claims.Object { panic("shaper bug") }
		}, want: claimtel.Panic, status: 500},
	} {
		t.Run(tc.name, func(t *testing.T) {
			saveDataPlaneDeps(t)
			log := captureCalls(t)
			tc.setup(t)

			rr, elapsed := postInject(t, b, nil)
			got := log.last(t)
			t.Logf("measured: %s answered HTTP %d as %s in %s", tc.name, rr.Code, got.Outcome, elapsed.Round(time.Millisecond))
			if elapsed > faultBudget {
				t.Errorf("answered in %s, budget %s", elapsed, faultBudget)
			}
			if got.Outcome != tc.want || rr.Code != tc.status {
				t.Fatalf("outcome %s / HTTP %d, want %s / %d (%s)", got.Outcome, rr.Code, tc.want, tc.status, rr.Body.String())
			}
			if tc.wantGuest {
				var resp ActionV2Response
				_ = json.Unmarshal(rr.Body.Bytes(), &resp)
				if c, ok := claimByKey(resp.AppendClaims, "role"); !ok || c.Value != "guest" {
					t.Errorf("the failure mode did not answer: %s", rr.Body.String())
				}
			}
		})
	}
}

// saveDataPlaneDeps restores every data-plane seam a fault test may replace.
func saveDataPlaneDeps(t *testing.T) {
	t.Helper()
	g, gk, sk, gm, sm := redisGetClaims, redisGetKey, redisSetKey, redisGetClaimMode, redisSetClaimMode
	rs, fm, sh := dbResolveProjectShapes, dbGetClaimFailureMode, shapeClaims
	t.Cleanup(func() {
		redisGetClaims, redisGetKey, redisSetKey, redisGetClaimMode, redisSetClaimMode = g, gk, sk, gm, sm
		dbResolveProjectShapes, dbGetClaimFailureMode, shapeClaims = rs, fm, sh
	})
}

// stubMissingRedis is a Redis that answers at once and holds nothing.
func stubMissingRedis() {
	redisGetClaims = func(context.Context, string) (string, error) { return "", redis.Nil }
	redisGetKey = func(context.Context, string) (string, error) { return "", redis.Nil }
	redisSetKey = func(context.Context, string, string, int) error { return nil }
	redisGetClaimMode = func(context.Context, string) (string, error) { return "", redis.Nil }
	redisSetClaimMode = func(context.Context, string, string, int) error { return nil }
}

// ---------------------------------------------------------------------------
// Repair on miss
// ---------------------------------------------------------------------------

func TestRepairQueue_DedupedBoundedRateLimited(t *testing.T) {
	q := &repairQueue{ch: make(chan string, 3)}
	q.enqueue("a")
	q.enqueue("a")
	q.enqueue("b")
	q.enqueue("c")
	q.enqueue("d") // full
	if st := q.stats(); st.Queued != 3 || st.Dropped != 1 {
		t.Fatalf("stats = %+v, want 3 queued (a deduped) and 1 dropped", st)
	}

	orig := cacheRefreshUser
	t.Cleanup(func() { cacheRefreshUser = orig })
	var mu sync.Mutex
	var at []time.Time
	cacheRefreshUser = func(_ context.Context, id string, _ ...string) error {
		mu.Lock()
		at = append(at, time.Now())
		mu.Unlock()
		if id == "b" {
			panic("compile bug")
		}
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { q.run(ctx, 40*time.Millisecond); close(done) }()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		n := len(at)
		mu.Unlock()
		if n == 3 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	<-done
	mu.Lock()
	defer mu.Unlock()
	if len(at) != 3 {
		t.Fatalf("compiled %d people, want 3 (a panic must not stop the worker)", len(at))
	}
	if gap := at[2].Sub(at[0]); gap < 70*time.Millisecond {
		t.Errorf("three compiles in %s: not rate-limited", gap)
	}
	if _, ok := q.pending.Load("a"); ok {
		t.Error("a repaired person is still pending, so a later miss could never ask again")
	}
}

// ---------------------------------------------------------------------------
// /healthz/claims
// ---------------------------------------------------------------------------

func freshRecorder(t *testing.T) *claimtel.Recorder {
	t.Helper()
	origR, origRec := claimDelivery, recordClaimCall
	claimDelivery = claimtel.New(time.Now)
	recordClaimCall = claimDelivery.Record
	t.Cleanup(func() { claimDelivery, recordClaimCall = origR, origRec })
	return claimDelivery
}

func stubHealthDeps(t *testing.T, redisErr error, warm *cache.WarmReport, panics int64) {
	t.Helper()
	p, w, b := redisPing, cacheLastWarm, backgroundPanics
	t.Cleanup(func() { redisPing, cacheLastWarm, backgroundPanics = p, w, b })
	redisPing = func(context.Context) error { return redisErr }
	cacheLastWarm = func() (cache.WarmReport, bool) {
		if warm == nil {
			return cache.WarmReport{}, false
		}
		return *warm, true
	}
	backgroundPanics = func() int64 { return panics }
}

func getHealth(t *testing.T) (int, claimsHealth, string) {
	t.Helper()
	rr := httptest.NewRecorder()
	NewRouter().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/healthz/claims", nil))
	var h claimsHealth
	if err := json.Unmarshal(rr.Body.Bytes(), &h); err != nil {
		t.Fatalf("decode: %v (%s)", err, rr.Body.String())
	}
	return rr.Code, h, rr.Body.String()
}

func TestClaimsHealth(t *testing.T) {
	goodWarm := &cache.WarmReport{At: time.Now().Add(-time.Hour), Refreshed: 10, Of: 10}

	t.Run("ok", func(t *testing.T) {
		r := freshRecorder(t)
		stubHealthDeps(t, nil, goodWarm, 0)
		r.Record(claimtel.Call{Outcome: claimtel.Served, UserID: "ada-secret-id", ClientID: "client-secret", Function: pre})
		r.Record(claimtel.Call{Outcome: claimtel.SigMissing}) // a scanner, or the deploy check
		r.Record(claimtel.Call{Outcome: claimtel.NoGrants, Function: pre})
		code, h, raw := getHealth(t)
		if code != 200 || h.Status != "ok" || len(h.Reasons) != 0 || h.Calls10m != 3 || h.LastServedAgeS == nil {
			t.Fatalf("HTTP %d %+v", code, h)
		}
		if strings.Contains(raw, "ada-secret-id") || strings.Contains(raw, "client-secret") {
			t.Fatalf("health leaked an id: %s", raw)
		}
	})
	t.Run("a wrong key is failing", func(t *testing.T) {
		r := freshRecorder(t)
		stubHealthDeps(t, nil, goodWarm, 0)
		r.Record(claimtel.Call{Outcome: claimtel.SigMismatch})
		code, h, _ := getHealth(t)
		if code != 503 || h.Status != "failing" || h.Reasons[0] != "OUTCOME:sig_mismatch" {
			t.Fatalf("HTTP %d %+v", code, h)
		}
	})
	t.Run("redis down is failing", func(t *testing.T) {
		freshRecorder(t)
		stubHealthDeps(t, errors.New("refused"), goodWarm, 0)
		code, h, _ := getHealth(t)
		if code != 503 || h.Status != "failing" || h.Redis != "down" {
			t.Fatalf("HTTP %d %+v", code, h)
		}
	})
	t.Run("degraded paths, a skipped warm and a recovered panic are degraded", func(t *testing.T) {
		r := freshRecorder(t)
		stubHealthDeps(t, nil, &cache.WarmReport{At: time.Now(), Skipped: true, Of: 10}, 1)
		r.Record(claimtel.Call{Outcome: claimtel.DegradedNoFacts, Function: pre})
		code, h, _ := getHealth(t)
		want := "BACKGROUND_PANIC,OUTCOME:degraded_no_facts,WARM_SKIPPED"
		if code != 200 || h.Status != "degraded" || strings.Join(h.Reasons, ",") != want {
			t.Fatalf("HTTP %d %+v, want reasons %s", code, h, want)
		}
	})
	t.Run("a stale warm is degraded", func(t *testing.T) {
		freshRecorder(t)
		stubHealthDeps(t, nil, &cache.WarmReport{At: time.Now().Add(-14 * time.Hour), Refreshed: 1, Of: 1}, 0)
		if _, h, _ := getHealth(t); h.Status != "degraded" || h.Reasons[0] != "WARM_STALE" {
			t.Fatalf("%+v", h)
		}
	})
	t.Run("a failing flush is degraded", func(t *testing.T) {
		r := freshRecorder(t)
		stubHealthDeps(t, nil, goodWarm, 0)
		r.Record(claimtel.Call{Outcome: claimtel.Served, Function: pre})
		_ = r.Flush(context.Background(), func(context.Context, claimtel.Batch) error { return errors.New("down") })
		if _, h, _ := getHealth(t); h.Status != "degraded" || h.TelemetryFailed != 1 {
			t.Fatalf("%+v", h)
		}
	})
}

// ---------------------------------------------------------------------------
// GET /api/v1/claims/delivery
// ---------------------------------------------------------------------------

func TestClaimDelivery_Endpoint(t *testing.T) {
	w, l, f := dbClaimDeliveryWindow, dbClaimDeliveryLast, dbClaimPayloadFingerprints
	t.Cleanup(func() { dbClaimDeliveryWindow, dbClaimDeliveryLast, dbClaimPayloadFingerprints = w, l, f })
	var since time.Time
	hist := make([]int64, claimtel.HistBuckets)
	hist[0], hist[6] = 99, 1
	dbClaimDeliveryWindow = func(_ context.Context, s time.Time) ([]db.ClaimDeliveryRow, error) {
		since = s
		return []db.ClaimDeliveryRow{{ProjectID: "p1", ClientID: "c1", Function: pre, KeySlot: "current", Outcome: claimtel.Served, N: 100, Hist: hist}}, nil
	}
	dbClaimDeliveryLast = func(context.Context) ([]db.ClaimDeliveryLastRow, error) { return nil, nil }
	old, recent := time.Now().Add(-30*24*time.Hour), time.Now().Add(-time.Hour)
	dbClaimPayloadFingerprints = func(context.Context) ([]db.ClaimPayloadFingerprint, error) {
		return []db.ClaimPayloadFingerprint{
			{FP: "base", Function: pre, FirstSeen: old},
			{FP: "new", Function: pre, FirstSeen: recent, Changed: true},
		}, nil
	}

	rr := httptest.NewRecorder()
	handleClaimDelivery(rr, httptest.NewRequest(http.MethodGet, "/api/v1/claims/delivery?window=7d", nil))
	if rr.Code != 200 {
		t.Fatalf("HTTP %d %s", rr.Code, rr.Body.String())
	}
	if d := time.Since(since); d < 7*24*time.Hour-time.Minute || d > 7*24*time.Hour+time.Minute {
		t.Errorf("window read from %s ago", d)
	}
	var got struct {
		Totals   map[string]int64 `json:"totals"`
		Rows     []deliveryRow    `json:"rows"`
		Findings []deliveryFinding
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &got)
	if len(got.Totals) != len(claimtel.Outcomes) || got.Totals["served"] != 100 {
		t.Errorf("totals = %v", got.Totals)
	}
	if got.Rows[0].P50MS == nil || *got.Rows[0].P50MS != 1 || got.Rows[0].P99MS == nil || *got.Rows[0].P99MS != 1 {
		t.Errorf("row = %+v", got.Rows[0])
	}
	if len(got.Findings) != 1 || got.Findings[0].Kind != "PAYLOAD_SHAPE_CHANGED" || !strings.Contains(got.Findings[0].Subject, "new") {
		t.Errorf("findings = %+v", got.Findings)
	}

	for _, bad := range []string{"abc", "100d", "-1h", "0m"} {
		rr := httptest.NewRecorder()
		handleClaimDelivery(rr, httptest.NewRequest(http.MethodGet, "/api/v1/claims/delivery?window="+bad, nil))
		if rr.Code != 400 {
			t.Errorf("window=%s: HTTP %d", bad, rr.Code)
		}
	}
}

// The delivery view is an operator's; the health summary is anyone's.
func TestClaimDelivery_Gating(t *testing.T) {
	t.Setenv("ZITADEL_DOMAIN", "zitadel.example.edu")
	t.Setenv("ZITADEL_AUDIENCE", "syndra")
	rr := httptest.NewRecorder()
	NewRouter().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/claims/delivery", nil))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated delivery view: HTTP %d", rr.Code)
	}
}
