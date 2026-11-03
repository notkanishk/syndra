package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"encoding/json"

	"syndra/internal/cache"
	"syndra/internal/claimdebug"
	"syndra/internal/claims"
	"syndra/internal/claimserve"
	"syndra/internal/claimserve/claimservetest"
	"syndra/internal/claimtel"
	"syndra/internal/db"
	"syndra/internal/services"
)

// The process split (claims-delivery-integrity S7): the claims container
// answers, the backend learns of it through Redis only.

// claimsContainer is a claims process (cmd/claims) over r, with the backend's
// relay reading the same Redis.
func claimsContainer(t *testing.T, r *claimservetest.Redis) *claimserve.Process {
	t.Helper()
	rr := claimRelayRedis
	t.Cleanup(func() { claimRelayRedis = rr })
	claimRelayRedis = func() claimserve.RelayRedis { return r }
	p := claimserve.NewProcess(r, "", true)
	if err := p.Plane.Poll(context.Background()); err != nil {
		t.Fatal(err)
	}
	return p
}

func postTo(t *testing.T, h http.Handler, body string) int {
	t.Helper()
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/action/inject", strings.NewReader(body)))
	return rr.Code
}

// A sign-in whose only call was answered by the claims container is
// delivered, not CLAIMS_NOT_CALLED: the correlator reads the container's
// calls through the relay. Without the relay the same sign-in reads not
// called, which is what the relay is for.
func TestCorrelator_SeesACallTheClaimsContainerAnswered(t *testing.T) {
	devMode(t)
	freshRecorder(t) // this process's door saw nothing
	r := builtinPlane(t)
	setFacts(r, "u1", "p1", factsJSON("u1", "p1", "admin"))
	proc := claimsContainer(t, r)

	signIn := time.Now()
	if code := postTo(t, proc.Router(), body(pre, `[{"project_id":"p1"}]`, "")); code != 200 {
		t.Fatalf("claims container: HTTP %d", code)
	}
	proc.Flush(context.Background())

	check := func() claimtel.Verdict {
		clock := signIn.Add(-10 * time.Minute)
		c := claimtel.NewCorrelator(func() time.Time { return clock })
		c.SignIn(claimtel.SignIn{UserID: "u1", ClientID: "c1", At: signIn})
		clock = signIn.Add(3 * time.Minute)
		out := c.Check(claimCalls{}, func(string) (bool, bool) { return true, true })
		if len(out) != 1 {
			t.Fatalf("checked %d", len(out))
		}
		return out[0].Verdict
	}
	if v := check(); v != claimtel.Delivered {
		t.Fatalf("a call the claims container served: %s, want delivered", v)
	}
	claimRelayRedis = func() claimserve.RelayRedis { return nil }
	if v := check(); v != claimtel.NotCalled {
		t.Fatalf("without the relay: %s, want not_called (the control)", v)
	}
}

// Ten backend restarts, each doing what a backend start does to Redis (a
// fresh snapshot, the start-up publish with the canary, the warm's canary
// write, a fold, a drain), while calls keep arriving at the claims container:
// every call is served, and the container's snapshot is never reloaded,
// because nothing it reads changed.
func TestClaimsContainer_TenBackendRestartsDoNotTouchIt(t *testing.T) {
	devMode(t)
	freshRecorder(t)
	r, _ := publishStore(t)
	origPlane := claimPlane
	t.Cleanup(func() { claimPlane = origPlane })
	da, dc := dirProjectIDs, dbClaimConfigProjects
	t.Cleanup(func() { dirProjectIDs, dbClaimConfigProjects = da, dc })
	dirProjectIDs = func(context.Context) ([]string, error) { return []string{"p1"}, nil }
	dbClaimConfigProjects = func(context.Context) ([]string, error) { return nil, nil }
	// One resolution of p1, the same every time: a restart publishes what is
	// already there.
	dbResolveProjectShapes = func(context.Context, string) (services.ProjectShapes, error) {
		return services.ProjectShapes{Default: claims.Builtin(), DefaultSource: "builtin"}, nil
	}
	canary, _ := json.Marshal(claimserve.CanaryFacts())
	// The warm writes the canary first, through the compiler's own facts write
	// (cache.WriteCanary): here, the HSET it amounts to.
	writeCanary := func() {
		r.SetField(claimserve.FactsKey(claimserve.CanaryUserID), claimserve.CanaryProjectID, string(canary))
	}
	writeCanary()
	if err := PublishAllClaimConfig(context.Background()); err != nil {
		t.Fatal(err)
	}
	setFacts(r, "u1", "p1", factsJSON("u1", "p1", "admin"))
	proc := claimsContainer(t, r)
	h := proc.Router()
	reloads := proc.Plane.Status().Reloads

	var served, other atomic.Int64
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			if postTo(t, h, body(pre, `[{"project_id":"p1"}]`, "")) == 200 {
				served.Add(1)
			} else {
				other.Add(1)
			}
		}
	}()
	for i := 0; i < 10; i++ {
		for served.Load()+other.Load() < int64(i+1)*50 { // calls on both sides of every restart
			time.Sleep(time.Millisecond)
		}
		ctx, cancel := context.WithCancel(context.Background())
		StartClaimsPlane(ctx, r) // the backend's own snapshot, fresh
		if err := PublishAllClaimConfig(ctx); err != nil {
			t.Fatal(err)
		}
		writeCanary()
		proc.Flush(ctx)
		_ = FoldClaimRelay(ctx)
		drainClaimSignals(ctx)
		cancel() // the backend stops
		_ = proc.Plane.Poll(context.Background())
	}
	close(stop)
	wg.Wait()

	if other.Load() != 0 || served.Load() == 0 {
		t.Fatalf("across ten backend restarts: %d served, %d not", served.Load(), other.Load())
	}
	if got := proc.Plane.Status().Reloads; got != reloads {
		t.Fatalf("the claims container reloaded its snapshot %d times across ten backend restarts", got-reloads)
	}
	if n := proc.Rec.Recent(10)[claimtel.Served]; n != served.Load() {
		t.Fatalf("container counted %d served, %d answered", n, served.Load())
	}
	t.Logf("measured: %d calls served by the claims container across ten backend restarts, 0 not served, 0 snapshot reloads", served.Load())
}

// The container's signals reach the backend: a facts miss is a recompile, a
// mismatch a keyring read, a watched sign-in a debug capture.
func TestClaimSignals_ReachTheBackend(t *testing.T) {
	devMode(t)
	r := builtinPlane(t)
	proc := claimsContainer(t, r)
	ctx := context.Background()
	explain := svcExplainSignIn
	t.Cleanup(func() { svcExplainSignIn = explain; claimDebug.Stop("aSplit", "test") })
	svcExplainSignIn = func(context.Context, string, string, string) (services.Explanation, error) {
		return services.Explanation{}, nil
	}
	claimDebug.Start("aSplit", "c1", "uMissSplit", "test")
	cr := claimConfigRedis
	t.Cleanup(func() { claimConfigRedis = cr })
	claimConfigRedis = func() claimserve.Redis { return r }
	publishDebugWatch(ctx)
	_ = proc.Plane.Poll(ctx)

	// Drain the keyring nudge channel so the test sees this one.
	select {
	case <-keyring.nudge:
	default:
	}
	postTo(t, proc.Router(), `{"function":"function/preaccesstoken","user":{"id":"uMissSplit"},"user_grants":[{"project_id":"p1"}],"application":{"client_id":"c1"}}`)
	proc.Out.Signal(claimserve.Signal{Kind: "nudge"})
	proc.Flush(ctx)
	if n := drainClaimSignals(ctx); n != 3 {
		t.Fatalf("drained %d signals, want repair, debug and nudge", n)
	}
	if _, ok := claimRepair.pending.Load("uMissSplit"); !ok {
		t.Fatal("the container's facts miss did not reach the repair queue")
	}
	claimRepair.pending.Delete("uMissSplit")
	select {
	case <-keyring.nudge:
	default:
		t.Fatal("the container's mismatch did not nudge the keyring")
	}
	deadline := time.Now().Add(2 * time.Second)
	for len(claimDebug.SignIns("aSplit", 0)) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("the container's debug capture never reached debug mode")
		}
		time.Sleep(10 * time.Millisecond)
	}
	var _ claimdebug.Entry = claimDebug.SignIns("aSplit", 0)[0]
}

// The backend's /healthz/claims counts the container's calls: a hard outcome
// answered over there fails the health here.
func TestClaimsHealth_CountsTheClaimsContainer(t *testing.T) {
	freshRecorder(t)
	stubHealthDeps(t, nil, &cache.WarmReport{At: time.Now(), Refreshed: 1, Of: 1}, 0)
	stubClaimFindings(t)
	claimFindingsOpen.Store(&[]db.ClaimFinding{})
	t.Setenv("ZITADEL_DOMAIN", "zitadel.example.edu")
	t.Setenv("ZITADEL_ACTION_SIGNING_KEY", "")
	r := claimservetest.New()
	proc := claimserve.NewProcess(r, strings.Repeat("k", 32), false)
	rr := claimRelayRedis
	t.Cleanup(func() { claimRelayRedis = rr })
	claimRelayRedis = func() claimserve.RelayRedis { return r }

	if _, h, _ := getHealth(t); h.Status != "ok" {
		t.Fatalf("before: %+v", h)
	}
	// Signed with a key the container does not hold: hard on any door.
	req := httptest.NewRequest(http.MethodPost, "/api/action/inject", strings.NewReader("{}"))
	req.Header.Set(claimserve.SignatureHeader, claimserve.SignatureHeaderValue(time.Now(), []byte("{}"), strings.Repeat("x", 32)))
	proc.Router().ServeHTTP(httptest.NewRecorder(), req)
	proc.Flush(context.Background())
	code, h, _ := getHealth(t)
	if code != 503 || !strings.Contains(strings.Join(h.Reasons, ","), "OUTCOME:sig_mismatch") || h.Outcomes10m["sig_mismatch"] != 1 {
		t.Fatalf("a mismatch on the claims container: HTTP %d %+v", code, h)
	}
}
