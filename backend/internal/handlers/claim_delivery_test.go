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
	"sync/atomic"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"syndra/internal/cache"
	"syndra/internal/claims"
	"syndra/internal/claimserve"
	"syndra/internal/claimserve/claimservetest"
	"syndra/internal/claimtel"
	"syndra/internal/db"
	"syndra/internal/models"
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
	withTeam := claims.ActionsV1()
	withTeam.Claims = append(withTeam.Claims, claims.Claim{Path: []string{"team"}, Kind: claims.KindAttribute, Source: claims.AttrTeam})

	cases := []struct {
		name   string
		body   string
		facts  *string // nil: the person's facts are stored
		shapes *services.ProjectShapes
		want   claimtel.Outcome
		status int
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
		{name: "no facts, nothing the shape asks for is only Syndra's", body: body(pre, `[{"project_id":"p1","roles":["admin"]}]`, ""), facts: ptr(""), want: claimtel.Served, status: 200},
		{name: "no facts, the shape asks for a fact", body: body(pre, `[{"project_id":"p1","roles":["admin"]}]`, ""), facts: ptr(""),
			shapes: &services.ProjectShapes{Default: withTeam}, want: claimtel.ServedRolesOnly, status: 200},
		{name: "malformed facts", body: body(pre, `[{"project_id":"p1"}]`, ""), facts: ptr("{{"), want: claimtel.DegradedMalformed, status: 200},
		{name: "project never published", body: body(pre, `[{"project_id":"pNew"}]`, ""), want: claimtel.DegradedShapeUnreadable, status: 200},
		{name: "not JSON", body: `not-json`, want: claimtel.BadRequest, status: 400},
		{name: "JSON but not an object", body: `[1]`, want: claimtel.BadRequest, status: 400},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ps := services.ProjectShapes{}
			if tc.shapes != nil {
				ps = *tc.shapes
			}
			r := testPlane(t, map[string]claimserve.Project{"p1": project(ps, failClosed), "p2": project(services.ProjectShapes{}, failClosed)})
			log := captureCalls(t)
			switch {
			case tc.facts == nil:
				setFacts(r, "u1", "p1", factsJSON("u1", "p1", "admin"))
			case *tc.facts != "":
				setFacts(r, "u1", "p1", *tc.facts)
			}

			rr, _ := postInject(t, tc.body, nil)
			got := log.last(t)
			if got.Outcome != tc.want || rr.Code != tc.status {
				t.Fatalf("outcome %s / HTTP %d, want %s / %d (body %s)", got.Outcome, rr.Code, tc.want, tc.status, rr.Body.String())
			}
			if tc.want == claimtel.Served && (got.ProjectID != "p1" || got.ClientID != "c1" || got.Function != pre || len(got.Body) == 0 || got.Gen == 0) {
				t.Errorf("dimensions not carried: %+v", got)
			}
		})
	}
}

func ptr(s string) *string { return &s }

// A miss is a repair signal; Redis erroring is not (a compile cannot help it).
func TestClaimDelivery_MissEnqueuesARepair(t *testing.T) {
	devMode(t)
	r := builtinPlane(t)
	captureCalls(t)

	postInject(t, `{"function":"function/preaccesstoken","user":{"id":"uMiss"},"user_grants":[{"project_id":"p1"}]}`, nil)
	if _, ok := claimRepair.pending.Load("uMiss"); !ok {
		t.Error("a facts miss did not ask for a recompile")
	}
	r.Fail(errors.New("i/o timeout"))
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
	r := builtinPlane(t)
	setFacts(r, "u1", "p1", factsJSON("u1", "p1", "admin"))
	log := captureCalls(t)
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
		for _, door := range []struct {
			name string
			h    http.Handler
		}{{"admin router", NewRouter()}, {"claims router", NewClaimsRouter()}} {
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/action/inject", strings.NewReader(b))
			for k, v := range tc.header {
				req.Header[k] = v
			}
			door.h.ServeHTTP(rr, req)
			got := log.last(t)
			if got.Outcome != tc.want || rr.Code != tc.status {
				t.Fatalf("%s via %s: outcome %s / HTTP %d, want %s / %d", tc.name, door.name, got.Outcome, rr.Code, tc.want, tc.status)
			}
			if tc.status == 401 && (got.ClientID != "" || got.Function != "" || got.KeySlot != "") {
				t.Errorf("%s: a refused call carried dimensions: %+v", tc.name, got)
			}
			// No keyring read yet in this test: the env key verified.
			if tc.want == claimtel.Served && got.KeySlot != claimtel.KeySlotEnv {
				t.Errorf("verified call key slot = %q", got.KeySlot)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Fault injection (assessment §4h): under any single dependency fault the
// route answers well inside Zitadel's 3s, with a defined outcome. Each case
// logs its measured time.
// ---------------------------------------------------------------------------

// splitRedis serves the snapshot's config from Redis and the facts reads
// from facts, which a test swaps for a real client against a faulty address.
type splitRedis struct {
	claimserve.Redis
	facts claimserve.Redis
}

func (s *splitRedis) HGet(ctx context.Context, key, field string) *redis.StringCmd {
	return s.facts.HGet(ctx, key, field)
}

func (s *splitRedis) Get(ctx context.Context, key string) *redis.StringCmd {
	if key == claimserve.GenKey {
		return s.Redis.Get(ctx, key)
	}
	return s.facts.Get(ctx, key)
}

// realFacts is a plane that served u1 once from a healthy Redis (so the
// last-known-good copy holds them), whose facts reads then go through the
// production client options (db.NewRedisClient) to addr.
func realFacts(t *testing.T, addr string, mode claimserve.Mode) {
	t.Helper()
	mem := claimservetest.New()
	_, _ = claimserve.Publish(context.Background(), mem, map[string]claimserve.Project{"p1": project(services.ProjectShapes{}, mode)})
	setFacts(mem, "u1", "p1", factsJSON("u1", "p1", "admin"))
	split := &splitRedis{Redis: mem, facts: mem}
	p := claimserve.New(split, nil)
	_ = p.Poll(context.Background())
	orig := claimPlane
	claimPlane = p
	t.Cleanup(func() { claimPlane = orig })
	if res := p.Answer(context.Background(), "u1", "p1", "c1", pre, []string{"admin"}, claims.Identity{}); res.Outcome != claimtel.Served {
		t.Fatalf("warm-up: %+v", res)
	}
	client := db.NewRedisClient(addr)
	t.Cleanup(func() { _ = client.Close() })
	split.facts = client
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

// failPostgres makes every Postgres read the write side owns fail, slowly.
// The data plane must not notice: it reads none of them.
func failPostgres(t *testing.T) *atomic.Int64 {
	t.Helper()
	rs, cm, cp := dbResolveProjectShapes, dbGetClaimMode, dbClaimConfigProjects
	t.Cleanup(func() { dbResolveProjectShapes, dbGetClaimMode, dbClaimConfigProjects = rs, cm, cp })
	var reads atomic.Int64
	slow := func(ctx context.Context) error {
		reads.Add(1)
		select {
		case <-ctx.Done():
		case <-time.After(5 * time.Second):
		}
		return errors.New("connection refused")
	}
	dbResolveProjectShapes = func(ctx context.Context, _ string) (services.ProjectShapes, error) {
		return services.ProjectShapes{}, slow(ctx)
	}
	dbGetClaimMode = func(ctx context.Context, _ string) (db.ClaimMode, error) { return db.ClaimMode{}, slow(ctx) }
	dbClaimConfigProjects = func(ctx context.Context) ([]string, error) { return nil, slow(ctx) }
	return &reads
}

const faultBudget = 300 * time.Millisecond

func TestClaimDelivery_FaultInjection(t *testing.T) {
	devMode(t)
	b := body(pre, `[{"project_id":"p1","roles":["admin"]}]`, "")
	guest := claimserve.Mode{Mode: claimserve.MinimalSafe, MinimalSafeClaims: map[string]any{"role": "guest"}}

	for _, tc := range []struct {
		name      string
		setup     func(t *testing.T)
		user      string
		want      claimtel.Outcome
		status    int
		wantRoles bool // the person's roles still reached the token
	}{
		{name: "redis hung, person served before", setup: func(t *testing.T) { realFacts(t, hungRedis(t), guest) },
			user: "u1", want: claimtel.ServedLKG, status: 200, wantRoles: true},
		{name: "redis refused, person served before", setup: func(t *testing.T) { realFacts(t, refusedRedis(t), guest) },
			user: "u1", want: claimtel.ServedLKG, status: 200, wantRoles: true},
		{name: "redis hung, person never served", setup: func(t *testing.T) { realFacts(t, hungRedis(t), guest) },
			user: "u2", want: claimtel.Served, status: 200, wantRoles: true},
		{name: "postgres down and slow", setup: func(t *testing.T) {
			r := testPlane(t, map[string]claimserve.Project{"p1": project(services.ProjectShapes{}, guest)})
			setFacts(r, "u1", "p1", factsJSON("u1", "p1", "admin"))
			reads := failPostgres(t)
			t.Cleanup(func() {
				if reads.Load() != 0 {
					t.Errorf("the data plane read Postgres %d times", reads.Load())
				}
			})
		}, user: "u1", want: claimtel.Served, status: 200, wantRoles: true},
		{name: "panic in the shaper", setup: func(t *testing.T) {
			r := builtinPlane(t)
			setFacts(r, "u1", "p1", factsJSON("u1", "p1", "admin"))
			claimPlane.Shaper = func(claims.Profile, claims.Facts) *claims.Object { panic("shaper bug") }
		}, user: "u1", want: claimtel.Panic, status: 500},
	} {
		t.Run(tc.name, func(t *testing.T) {
			log := captureCalls(t)
			tc.setup(t)

			body := strings.Replace(b, `"id":"u1"`, fmt.Sprintf(`"id":%q`, tc.user), 1)
			for _, door := range []struct {
				name string
				h    http.Handler
			}{{"admin router", NewRouter()}, {"claims router", NewClaimsRouter()}} {
				rr := httptest.NewRecorder()
				start := time.Now()
				door.h.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/action/inject", strings.NewReader(body)))
				elapsed := time.Since(start)
				got := log.last(t)
				t.Logf("measured: %s via %s answered HTTP %d as %s in %s", tc.name, door.name, rr.Code, got.Outcome, elapsed.Round(time.Millisecond))
				if elapsed > faultBudget {
					t.Errorf("answered in %s, budget %s", elapsed, faultBudget)
				}
				if got.Outcome != tc.want || rr.Code != tc.status {
					t.Fatalf("outcome %s / HTTP %d, want %s / %d (%s)", got.Outcome, rr.Code, tc.want, tc.status, rr.Body.String())
				}
				var resp ActionV2Response
				_ = json.Unmarshal(rr.Body.Bytes(), &resp)
				if _, ok := claimByKey(resp.AppendClaims, "roles"); tc.wantRoles && !ok {
					t.Errorf("the roles did not reach the token: %s", rr.Body.String())
				}
			}
		})
	}
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

	orig := cacheRefresh
	t.Cleanup(func() { cacheRefresh = orig })
	var mu sync.Mutex
	var at []time.Time
	cacheRefresh = func(_ context.Context, ids []string, _ ...string) error {
		id := ids[0]
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
	builtinPlane(t) // a loaded snapshot with one project
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
		r.Record(claimtel.Call{Outcome: claimtel.Served, UserID: "charlotte-secret-id", ClientID: "client-secret", Function: pre})
		r.Record(claimtel.Call{Outcome: claimtel.SigMissing}) // a scanner, or the deploy check
		r.Record(claimtel.Call{Outcome: claimtel.NoGrants, Function: pre})
		code, h, raw := getHealth(t)
		if code != 200 || h.Status != "ok" || len(h.Reasons) != 0 || h.Calls10m != 3 || h.LastServedAgeS == nil {
			t.Fatalf("HTTP %d %+v", code, h)
		}
		if strings.Contains(raw, "charlotte-secret-id") || strings.Contains(raw, "client-secret") {
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
	// The snapshot (S2): none loaded is failing (every call answers fail
	// closed); loaded but empty, or not confirmed by Redis for 10s, degraded.
	t.Run("the snapshot", func(t *testing.T) {
		freshRecorder(t)
		stubHealthDeps(t, nil, goodWarm, 0)
		orig := claimPlane
		t.Cleanup(func() { claimPlane = orig })

		claimPlane = claimserve.New(nil, nil)
		if code, h, _ := getHealth(t); code != 503 || h.Reasons[0] != "SNAPSHOT_NOT_LOADED" || h.Snapshot.Loaded {
			t.Fatalf("not loaded: %d %+v", code, h)
		}
		testPlane(t, nil)
		if _, h, _ := getHealth(t); h.Status != "degraded" || h.Reasons[0] != "SNAPSHOT_EMPTY" {
			t.Fatalf("empty: %+v", h)
		}
		r := claimservetest.New()
		_, _ = claimserve.Publish(context.Background(), r, map[string]claimserve.Project{"p1": project(services.ProjectShapes{}, failClosed)})
		at := time.Now()
		claimPlane = claimserve.New(r, func() time.Time { return at })
		_ = claimPlane.Poll(context.Background())
		r.Fail(errors.New("down"))
		at = at.Add(11 * time.Second)
		_ = claimPlane.Poll(context.Background())
		if _, h, _ := getHealth(t); h.Status != "degraded" || h.Reasons[0] != "SNAPSHOT_STALE" || h.Snapshot.Failures != 1 || h.Snapshot.Gen != 1 {
			t.Fatalf("stale: %+v", h)
		}
	})
}

// ---------------------------------------------------------------------------
// GET /api/v1/claims/delivery
// ---------------------------------------------------------------------------

func TestClaimDelivery_Endpoint(t *testing.T) {
	w, l, f := dbClaimDeliveryWindow, dbClaimDeliveryLast, dbClaimPayloadFingerprints
	cf, ls, da := dbClaimFindings, dbLastSignInPerApp, directoryApplications
	t.Cleanup(func() {
		dbClaimDeliveryWindow, dbClaimDeliveryLast, dbClaimPayloadFingerprints = w, l, f
		dbClaimFindings, dbLastSignInPerApp, directoryApplications = cf, ls, da
	})
	dbClaimFindings = func(context.Context, time.Time) ([]db.ClaimFinding, error) {
		return []db.ClaimFinding{{Kind: claimtel.KindNotCalled, Subject: "c1", FirstSeen: time.Now(), LastSeen: time.Now(), Count: 2,
			Sample: map[string]any{"missing": []any{"function/preaccesstoken"}}}}, nil
	}
	dbLastSignInPerApp = func(context.Context) (map[string]time.Time, error) {
		return map[string]time.Time{"a1": time.Now()}, nil
	}
	directoryApplications = func(context.Context) ([]models.ApplicationCatalog, error) {
		return []models.ApplicationCatalog{{ID: "a1", Name: "NetBird", ProjectID: "p1", ClientID: "c1"}, {ID: "a2", Name: "Wiki", ProjectID: "p1", ClientID: "c2"}}, nil
	}
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
		Apps     []liveApp `json:"apps"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &got)
	if len(got.Totals) != len(claimtel.Outcomes) || got.Totals["served"] != 100 {
		t.Errorf("totals = %v", got.Totals)
	}
	if got.Rows[0].P50MS == nil || *got.Rows[0].P50MS != 1 || got.Rows[0].P99MS == nil || *got.Rows[0].P99MS != 1 {
		t.Errorf("row = %+v", got.Rows[0])
	}
	if len(got.Findings) != 2 || got.Findings[0].Kind != claimtel.KindNotCalled || got.Findings[0].Severity != "hard" ||
		got.Findings[1].Kind != "PAYLOAD_SHAPE_CHANGED" || !strings.Contains(got.Findings[1].Subject, "new") {
		t.Errorf("findings = %+v", got.Findings)
	}
	// Every app is listed, called or not, so Live does not move with traffic.
	if len(got.Apps) != 2 || got.Apps[0].Name != "NetBird" || got.Apps[0].Calls != 100 || got.Apps[0].P99MS == nil ||
		got.Apps[0].LastSignInAt == nil || got.Apps[1].Name != "Wiki" || got.Apps[1].Calls != 0 {
		t.Errorf("apps = %+v", got.Apps)
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
