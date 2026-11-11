package addons

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync/atomic"
	"testing"
)

// Fresh reads (feature freshness, design §4): `fresh` reaches the wire only
// for an add-on declaring the feature and only on the reads in scope; a
// cooldown is the add-on answering, never a breaker failure; a reply whose
// current and stale disagree is not a reading.

// declare adds feature to the installed add-on's manifest.
func declare(t *testing.T, feature string) {
	t.Helper()
	a, err := Get("truenas")
	if err != nil {
		t.Fatal(err)
	}
	registryMu.Lock()
	a.manifest.Features = append(slices.Clone(a.manifest.Features), feature)
	registryMu.Unlock()
}

func TestFreshReachesTheWireOnlyForAnAddOnDeclaringIt(t *testing.T) {
	for _, declared := range []bool{false, true} {
		got := addonAnswering(t, "subjects.json")
		if declared {
			declare(t, "freshness")
		}
		Subjects(context.Background(), "truenas", true)
		if want := map[bool]string{true: "fresh=true"}[declared]; got.rawQuery != want {
			t.Errorf("declared=%t: GET /subjects?%s, want ?%s", declared, got.rawQuery, want)
		}

		got = addonAnswering(t, "operation_response_result.json")
		declare(t, "results")
		if declared {
			declare(t, "freshness")
		}
		Read(context.Background(), "truenas", "storage.status", "s", "s", nil, true)
		var body map[string]any
		_ = json.Unmarshal(got.body, &body)
		if fresh, has := body["fresh"]; has != declared || has && fresh != true {
			t.Errorf("declared=%t: the operation body says fresh=%v (present %t)", declared, fresh, has)
		}
	}
}

func TestAReadWithoutFreshNeverSendsIt(t *testing.T) {
	got := addonAnswering(t, "subjects.json")
	declare(t, "freshness")
	Subjects(context.Background(), "truenas", false)
	if got.rawQuery != "" {
		t.Errorf("a read nobody asked to be fresh sent ?%s", got.rawQuery)
	}
	for _, read := range []func(){
		func() { ReadReach(context.Background(), "truenas", false) },
		func() { ReadHomes(context.Background(), "truenas", false) },
	} {
		read()
		if got.rawQuery != "" {
			t.Errorf("%s sent ?%s", got.path, got.rawQuery)
		}
	}
}

// Never on /plan, /apply or /stats (design §4): the pipe refuses it there
// before anything leaves, whoever asks.
func TestFreshIsRefusedOutsideTheReadsInScope(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { hits.Add(1) }))
	t.Cleanup(srv.Close)
	signedAddon(t, srv.URL, "a-test-secret")
	declare(t, "freshness")
	declare(t, "stats")
	a, _ := Get("truenas")
	for _, s := range []sendSpec{
		{Method: http.MethodPost, Path: "/plan", Body: []byte(`{}`)},
		{Method: http.MethodPost, Path: "/apply", Body: []byte(`{}`), Write: true, TraceID: "call-1"},
		{Method: http.MethodPost, Path: "/operations/password.set", Body: []byte(`{}`), Write: true, TraceID: "call-2"},
		{Method: http.MethodGet, Path: "/stats"},
		{Method: http.MethodGet, Path: "/health"},
	} {
		s.Fresh = true
		if _, err := a.send(context.Background(), s); !errors.Is(err, ErrFreshOutOfScope) {
			t.Errorf("%s %s with fresh: %v, want ErrFreshOutOfScope", s.Method, s.Path, err)
		}
	}
	if n := hits.Load(); n != 0 {
		t.Errorf("%d requests left with fresh outside its scope", n)
	}
}

// The backend's read body with fresh is the contract's fixture.
func TestAFreshReadOperationSendsTheFixture(t *testing.T) {
	got := addonAnswering(t, "operation_response_result.json")
	declare(t, "results")
	declare(t, "freshness")
	Read(context.Background(), "truenas", "storage.status", "289471021834760193", "289471021834760193", nil, true)
	sameJSON(t, got.body, contractFixture(t, "operation_request_fresh.json"), "a fresh read operation")
}

// The planner's re-plan is a /plan call: against an add-on declaring
// freshness it carries no query and no fresh key.
func TestAPlanToADeclaringAddOnCarriesNoFresh(t *testing.T) {
	got := addonAnswering(t, "plan_response.json")
	declare(t, "freshness")
	Plan(context.Background(), "truenas", []PlanSubject{{Subject: "s", Email: "rey_mysterio@example.org"}})
	if got.path != "/plan" || got.rawQuery != "" || bytes.Contains(got.body, []byte(`"fresh"`)) {
		t.Errorf("a plan sent %s?%s %s", got.path, got.rawQuery, got.body)
	}
}

// Five presses of "Read again" inside the cooldown are the add-on answering on
// purpose: the breaker stays closed for everybody else.
func TestACooldownIsNeverABreakerFailure(t *testing.T) {
	withBreaker(t, 3, breakerCooldown)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.Header().Set("Retry-After", "17")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write(contractFixture(t, "error_fresh_read_cooldown.json"))
	}))
	t.Cleanup(srv.Close)
	signedAddon(t, srv.URL, "a-test-secret")
	declare(t, "freshness")
	for range breakerThreshold + 2 {
		r := Subjects(context.Background(), "truenas", true)
		if wait, ok := r.CooledDown(); !ok || wait != 17 {
			t.Fatalf("a 429 cooldown reads as cooled down for 17 s: %d %t %+v", wait, ok, r)
		}
	}
	a, _ := Get("truenas")
	if a.CircuitOpen() {
		t.Error("cooldown answers opened the breaker")
	}
}

func TestRetryAfterIsReadAsDelaySeconds(t *testing.T) {
	for header, want := range map[string]int{"17": 17, "": 0, "soon": 0, "-3": 0, "Wed, 21 Oct 2015 07:28:00 GMT": 0} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if header != "" {
				w.Header().Set("Retry-After", header)
			}
			w.WriteHeader(http.StatusTooManyRequests)
		}))
		signedAddon(t, srv.URL, "a-test-secret")
		a, _ := Get("truenas")
		resp, _ := a.send(context.Background(), sendSpec{Method: http.MethodGet, Path: "/subjects"})
		srv.Close()
		if resp.RetryAfter != want {
			t.Errorf("Retry-After %q read as %d, want %d", header, resp.RetryAfter, want)
		}
	}
}

// A cooldown answered with the copy is cooled down too, and is never usable.
func TestACooldownAnswerIsCooledDownAndNotUsable(t *testing.T) {
	addonAnswering(t, "subjects_cooldown.json")
	r := Subjects(context.Background(), "truenas", true)
	if wait, ok := r.CooledDown(); !ok || wait != 20 || r.Usable() {
		t.Errorf("a cooldown answer: wait %d cooled %t usable %t", wait, ok, r.Usable())
	}
}

// current == false ⇔ stale present, failed either way, is not a reading.
func TestAReplyBreakingTheStaleInvariantIsNotAReading(t *testing.T) {
	currentWithStale := func(d map[string]any) { d["current"] = true; d["stale"] = map[string]any{"reason": "cached"} }
	notCurrentNoStale := func(d map[string]any) { d["current"] = false; delete(d, "stale") }
	for name, edit := range map[string]func(map[string]any){"current with stale": currentWithStale, "not current, no stale": notCurrentNoStale} {
		addonServing(t, withJSON(t, contractFixture(t, "subjects.json"), edit))
		if r := Subjects(context.Background(), "truenas", false); r.Usable() {
			t.Errorf("subjects, %s: usable", name)
		}
		addonServing(t, withJSON(t, contractFixture(t, "reach.json"), edit))
		if r := ReadReach(context.Background(), "truenas", false); r.Read {
			t.Errorf("reach, %s: read", name)
		}
		addonServing(t, withJSON(t, contractFixture(t, "homes.json"), edit))
		if h := ReadHomes(context.Background(), "truenas", false); h.Read {
			t.Errorf("homes, %s: read", name)
		}
		addonServing(t, withJSON(t, contractFixture(t, "operation_response_result.json"), func(d map[string]any) { edit(d["result"].(map[string]any)) }))
		declare(t, "results")
		if v := Read(context.Background(), "truenas", "storage.status", "s", "s", nil, false).View(); v.Readable {
			t.Errorf("storage.status, %s: readable", name)
		}
	}
	// And each untouched fixture is a reading, so the edits above are what failed.
	addonAnswering(t, "subjects_stale.json")
	if r := Subjects(context.Background(), "truenas", false); r.Outcome != OutcomeSucceeded || r.Truncated {
		t.Errorf("an honest copy was not read: %+v", r)
	}
}

// A 429 cooldown with nothing to serve reaches every view as its countdown.
func TestACooldownWithNothingToServeReachesEveryView(t *testing.T) {
	addonServingStatus(t, http.StatusTooManyRequests, contractFixture(t, "error_fresh_read_cooldown.json"))
	declare(t, "freshness")
	declare(t, "results")
	// No Retry-After header from this server: still a cooldown, at least 1 s.
	if r := ReadReach(context.Background(), "truenas", true); r.RetryAfter != 1 || r.Read {
		t.Errorf("reach: %+v", r)
	}
	if h := ReadHomes(context.Background(), "truenas", true); h.RetryAfter != 1 || h.Read {
		t.Errorf("homes: %+v", h)
	}
	if v := Read(context.Background(), "truenas", "storage.status", "s", "s", nil, true).View(); v.RetryAfter != 1 || v.Readable {
		t.Errorf("read view: %+v", v)
	}
	if a := Activity(context.Background(), "truenas", "s", "s", "", true); a.RetryAfter != 1 {
		t.Errorf("activity: %+v", a)
	}
	// Any other refusal is not a cooldown.
	addonServingStatus(t, http.StatusServiceUnavailable, contractFixture(t, "error_target_unreachable.json"))
	if r := Subjects(context.Background(), "truenas", true); r.RetryAfter != 0 {
		t.Errorf("an unreachable target read as a cooldown: %+v", r)
	}
}

func withJSON(t *testing.T, raw []byte, edit func(map[string]any)) []byte {
	t.Helper()
	var d map[string]any
	if err := json.Unmarshal(raw, &d); err != nil {
		t.Fatal(err)
	}
	edit(d)
	out, _ := json.Marshal(d)
	return out
}
