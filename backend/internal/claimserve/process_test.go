package claimserve

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"syndra/internal/claims"
	"syndra/internal/claimserve/claimservetest"
	"syndra/internal/claimtel"
)

var (
	_ RelayRedis = (*claimservetest.Redis)(nil)
	_ RelayRedis = (*redis.Client)(nil)
)

// process is a claims process over an in-memory Redis with p1 (the built-in
// shape, client c1 its own app) and the canary published, u1's facts and the
// canary's written, in dev mode (no key).
func process(t *testing.T) (*Process, *claimservetest.Redis) {
	t.Helper()
	r := claimservetest.New()
	if _, err := Publish(context.Background(), r, map[string]Project{
		"p1": {Shapes: ProjectShapes{Default: claims.Builtin(), DefaultSource: "builtin",
			ByClient: map[string]AppShape{"c1": {ApplicationID: "a1", Version: 1, Profile: claims.Builtin()}}}, Mode: Mode{Mode: FailClosed}},
		CanaryProjectID: CanaryProject(),
	}); err != nil {
		t.Fatal(err)
	}
	r.SetField(FactsKey("u1"), "p1", facts("admin"))
	cf, _ := json.Marshal(CanaryFacts())
	r.SetField(FactsKey(CanaryUserID), CanaryProjectID, string(cf))
	p := NewProcess(r, "", true)
	if err := p.Plane.Poll(context.Background()); err != nil {
		t.Fatal(err)
	}
	return p, r
}

func post(t *testing.T, h http.Handler, body string, hdr http.Header) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/action/inject", strings.NewReader(body))
	for k, v := range hdr {
		req.Header[http.CanonicalHeaderKey(k)] = v
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

const u1Call = `{"function":"function/preaccesstoken","user":{"id":"u1"},"user_grants":[{"project_id":"p1"}],"application":{"client_id":"c1"}}`

// The canary's expectation is written out; the shaper must still produce it.
// A shaper change that alters the canary's answer fails here, not silently
// in the probe.
func TestCanaryExpected(t *testing.T) {
	got, _ := json.Marshal(Envelope(claims.Shape(CanaryProject().Shapes.ByClient[CanaryClientID].Profile, CanaryFacts())))
	if string(got) != CanaryExpected {
		t.Fatalf("the canary shapes to %s, CanaryExpected is %s", got, CanaryExpected)
	}
	// Through the claims process's own door: exactly that, counted as a probe
	// and nowhere else.
	p, _ := process(t)
	rr := post(t, p.Router(), string(CanaryPayload()), nil)
	var out Response
	_ = json.Unmarshal(rr.Body.Bytes(), &out)
	if b, _ := json.Marshal(out.AppendClaims); rr.Code != 200 || string(b) != CanaryExpected {
		t.Fatalf("canary through the door: HTTP %d %s", rr.Code, b)
	}
	if n := p.Rec.Probes()[claimtel.Served]; n != 1 {
		t.Fatalf("probe count %v", p.Rec.Probes())
	}
	if len(p.Rec.Recent(10)) != 0 || p.Rec.ActivePairs() != 0 || !p.Rec.LastServed().IsZero() {
		t.Fatal("the probe reached the SLIs or the correlator's pairs")
	}
	p.Out.mu.Lock()
	n := p.Out.n
	p.Out.mu.Unlock()
	if n != 0 {
		t.Fatal("the probe's call was queued for the correlator")
	}
	// The snapshot's project count leaves it out.
	if st := p.Plane.Status(); st.Projects != 1 {
		t.Fatalf("projects %d, want 1 (the canary is not an app)", st.Projects)
	}
	if c := WithoutCanary(p.Plane.Config()); len(c.Projects) != 1 || c.ClientProject[CanaryClientID] != "" {
		t.Fatalf("WithoutCanary kept it: %+v", c.Projects)
	}
}

// The relay: counts reach Postgres exactly once, through a fold that keeps
// a minute whose write failed; recent outcomes and the last-served time
// reach the backend's health within a flush.
func TestRelay_TelemetryFoldsOnce(t *testing.T) {
	ctx := context.Background()
	p, r := process(t)
	h := p.Router()
	for i := 0; i < 5; i++ {
		if rr := post(t, h, u1Call, nil); rr.Code != 200 {
			t.Fatalf("call %d: HTTP %d", i, rr.Code)
		}
	}
	post(t, h, `{"function":"function/preaccesstoken","user":{"id":"u2"},"user_grants":[{"project_id":"p1"}],"application":{"client_id":"c1"}}`, nil)
	now := time.Now()
	if err := p.Out.Flush(ctx, r, p.Rec, now); err != nil {
		t.Fatal(err)
	}

	recent, err := RemoteRecent(ctx, r, now, 10)
	if err != nil || recent[claimtel.Served] != 5 || recent[claimtel.ServedRolesOnly] != 1 {
		t.Fatalf("recent %v %v", recent, err)
	}
	served, called, err := RemoteLast(ctx, r)
	if err != nil || served.IsZero() || called.IsZero() {
		t.Fatalf("last served %v called %v %v", served, called, err)
	}

	// Not yet closed: nothing folds.
	var got []claimtel.Batch
	write := func(_ context.Context, b claimtel.Batch) error { got = append(got, b); return nil }
	if n, _ := FoldTelemetry(ctx, r, now, write); n != 0 {
		t.Fatalf("folded an open minute")
	}
	// The write fails: kept for the next pass.
	later := now.Add(3 * time.Minute)
	if _, err := FoldTelemetry(ctx, r, later, func(context.Context, claimtel.Batch) error { return errors.New("pg down") }); err == nil {
		t.Fatal("a failed write was not reported")
	}
	if n, err := FoldTelemetry(ctx, r, later, write); n != 1 || err != nil {
		t.Fatalf("retry folded %d: %v", n, err)
	}
	if n, _ := FoldTelemetry(ctx, r, later, write); n != 0 || len(got) != 1 {
		t.Fatalf("folded twice: %d batches", len(got))
	}
	b := got[0]
	var servedN, histN int64
	for k, c := range b.Counts {
		if k.Outcome == claimtel.Served {
			servedN += c.N
			for _, n := range c.Hist {
				histN += n
			}
			if k.ProjectID != "p1" || k.ClientID != "c1" || k.Function != claims.FunctionPreAccessToken {
				t.Fatalf("key %+v", k)
			}
		}
	}
	if servedN != 5 || histN != 5 {
		t.Fatalf("served %d, histogram %d, want 5 and 5", servedN, histN)
	}
	cl := b.Clients[claimtel.ClientKey{ClientID: "c1", Function: claims.FunctionPreAccessToken}]
	if cl.At.IsZero() || cl.ServedAt.IsZero() || !cl.ClientKnown {
		t.Fatalf("client last %+v", cl)
	}
	if len(b.Fingerprints) != 1 {
		t.Fatalf("fingerprints %d, want the one payload shape", len(b.Fingerprints))
	}
}

// What the claims process cannot do itself reaches the backend as a signal: a
// facts miss (recompile), a signature no key matched (read the keyring), and
// a watched sign-in (debug mode).
func TestRelay_Signals(t *testing.T) {
	ctx := context.Background()
	r := claimservetest.New()
	if _, err := Publish(ctx, r, map[string]Project{"p1": {Shapes: ProjectShapes{Default: claims.Builtin(), DefaultSource: "builtin"}, Mode: Mode{Mode: FailClosed}}}); err != nil {
		t.Fatal(err)
	}
	if err := PublishDebug(ctx, r, map[string]DebugWatch{"c1": {UserID: "u9", Ends: time.Now().Add(time.Minute).Unix()}}); err != nil {
		t.Fatal(err)
	}
	key := strings.Repeat("k", 32)
	p := NewProcess(r, key, false)
	_ = p.Plane.Poll(ctx)
	h := p.Router()

	miss := `{"function":"function/preaccesstoken","user":{"id":"u9"},"user_grants":[{"project_id":"p1"}],"application":{"client_id":"c1"}}`
	ts := time.Now()
	post(t, h, miss, http.Header{SignatureHeader: {SignatureHeaderValue(ts, []byte(miss), key)}})
	post(t, h, miss, http.Header{SignatureHeader: {SignatureHeaderValue(ts, []byte(miss), strings.Repeat("x", 32))}})
	if err := p.Out.Flush(ctx, r, p.Rec, time.Now()); err != nil {
		t.Fatal(err)
	}
	sigs, err := DrainSignals(ctx, r, 100)
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]Signal{}
	for _, s := range sigs {
		kinds[s.Kind] = s
	}
	if kinds["repair"].UserID != "u9" || kinds["nudge"].Kind == "" || kinds["debug"].Capture == nil || kinds["debug"].Capture.UserID != "u9" {
		t.Fatalf("signals %+v", sigs)
	}
	if again, _ := DrainSignals(ctx, r, 100); len(again) != 0 {
		t.Fatalf("drained twice: %+v", again)
	}
}

// A call the claims process answered is in the window the correlator reads,
// with the same span rules as the in-process recorder; a pair whose relay
// list is full cannot be called "not called" before its oldest call.
func TestRelay_Window(t *testing.T) {
	ctx := context.Background()
	p, r := process(t)
	at := time.Now()
	post(t, p.Router(), u1Call, nil)
	if w, _ := RemoteWindow(ctx, r, "u1", "c1", at.Add(-time.Minute), at.Add(time.Minute)); len(w) != 0 {
		t.Fatal("seen before the flush")
	}
	_ = p.Out.Flush(ctx, r, p.Rec, time.Now())
	w, over := RemoteWindow(ctx, r, "u1", "c1", at.Add(-time.Minute), at.Add(time.Minute))
	if w[claims.FunctionPreAccessToken] != claimtel.Served || over {
		t.Fatalf("window %v overflowed %v", w, over)
	}
	if w, _ := RemoteWindow(ctx, r, "u1", "c1", at.Add(5*time.Minute), at.Add(6*time.Minute)); len(w) != 0 {
		t.Fatalf("a call outside the window: %v", w)
	}

	for i := 0; i < relayCallsKept+5; i++ {
		p.Out.Call(claimtel.Call{UserID: "u7", ClientID: "c1", Function: claims.FunctionPreAccessToken, Outcome: claimtel.Served, At: at.Add(time.Duration(i) * time.Second)})
	}
	_ = p.Out.Flush(ctx, r, nil, time.Now())
	if _, over := RemoteWindow(ctx, r, "u7", "c1", at.Add(-time.Minute), at.Add(time.Minute)); !over {
		t.Fatal("a truncated relay read as complete")
	}
	r.Fail(errors.New("down"))
	if _, over := RemoteWindow(ctx, r, "u1", "c1", at.Add(-time.Minute), at.Add(time.Minute)); !over {
		t.Fatal("a failed read must leave the sign-in unchecked, never not-called")
	}
}

// Redis out at flush time: the calls and signals are kept, not lost, and the
// next flush writes them.
func TestRelay_FailedFlushKeeps(t *testing.T) {
	ctx := context.Background()
	p, r := process(t)
	at := time.Now()
	post(t, p.Router(), u1Call, nil)
	p.Out.Signal(Signal{Kind: "nudge"})
	r.Fail(errors.New("down"))
	if err := p.Out.Flush(ctx, r, p.Rec, time.Now()); err == nil || p.Out.Failures() != 1 {
		t.Fatalf("a failed flush: %v, failures %d", err, p.Out.Failures())
	}
	r.Fail(nil)
	if err := p.Out.Flush(ctx, r, p.Rec, time.Now()); err != nil || p.Out.Failures() != 0 {
		t.Fatal(err)
	}
	if w, _ := RemoteWindow(ctx, r, "u1", "c1", at.Add(-time.Minute), at.Add(time.Minute)); w[claims.FunctionPreAccessToken] != claimtel.Served {
		t.Fatal("the call was lost across a failed flush")
	}
	if s, _ := DrainSignals(ctx, r, 10); len(s) != 1 {
		t.Fatalf("signals %v", s)
	}
}

// The claims container's health: its own door, its relay, no detectors.
func TestProcess_Health(t *testing.T) {
	p, r := process(t)
	get := func() (int, Health) {
		rr := httptest.NewRecorder()
		p.Router().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/healthz/claims", nil))
		var h Health
		_ = json.Unmarshal(rr.Body.Bytes(), &h)
		return rr.Code, h
	}
	if code, h := get(); code != 200 || h.Status != "ok" || !h.Snapshot.Loaded {
		t.Fatalf("healthy: %d %+v", code, h)
	}
	r.Fail(errors.New("down"))
	if code, h := get(); code != 503 || h.Redis != "down" {
		t.Fatalf("redis down: %d %+v", code, h)
	}
}

func TestKeyProblem(t *testing.T) {
	for key, bad := range map[string]bool{
		"":                                 true,
		strings.Repeat("a", 31):            true,
		strings.Repeat("a", 32):            false,
		"AbC123" + strings.Repeat("z", 26): false,
		strings.Repeat("a", 31) + "\n":     true,
		strings.Repeat("a", 16) + " " + "b1234567890123456": true,
		strings.Repeat("a", 31) + "é":                       true,
	} {
		if got := KeyProblem(key) != ""; got != bad {
			t.Errorf("KeyProblem(%q) bad=%v, want %v", key, got, bad)
		}
	}
}

// A roles-only answer is a delivered token: health reads it as soft only when
// it is persistent, a handful of them outnumbering the full answers.
func TestAssess_RolesOnlyIsSoftOnlyWhenPersistent(t *testing.T) {
	snap := Status{Loaded: true, Projects: 1, CheckedAgo: new(int64)}
	for _, c := range []struct {
		rolesOnly, served int64
		soft              bool
	}{{4, 0, false}, {5, 10, false}, {5, 4, true}} {
		h, hard, soft := Assess(time.Now(), map[claimtel.Outcome]int64{claimtel.ServedRolesOnly: c.rolesOnly, claimtel.Served: c.served}, time.Time{}, nil, snap)
		if code := h.Verdict(hard, soft); code != http.StatusOK || (h.Status == "degraded") != c.soft ||
			(c.soft && strings.Join(h.Reasons, ",") != "OUTCOME:served_roles_only") {
			t.Errorf("%d roles-only, %d served: %s %v", c.rolesOnly, c.served, h.Status, h.Reasons)
		}
	}
}
