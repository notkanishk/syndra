package claimtel

import (
	"fmt"
	"math/rand"
	"sync/atomic"
	"testing"
	"time"
)

// clock is a settable now.
type clock struct{ ns atomic.Int64 }

func newClock(at time.Time) *clock   { c := &clock{}; c.set(at); return c }
func (c *clock) set(at time.Time)    { c.ns.Store(at.UnixNano()) }
func (c *clock) add(d time.Duration) { c.ns.Add(int64(d)) }
func (c *clock) now() time.Time      { return time.Unix(0, c.ns.Load()).UTC() }

var openid = []string{"openid", "profile"}

func jwtApps(string) (bool, bool) { return true, true }

func call(r *Recorder, at time.Time, user, client, fn string, o Outcome) {
	r.Record(Call{Outcome: o, UserID: user, ClientID: client, Function: fn, ProjectID: "p1", At: at})
}

func verdicts(ch []Checked) map[Verdict]int {
	out := map[Verdict]int{}
	for _, c := range ch {
		out[c.Verdict]++
	}
	return out
}

func TestCorrelator_NoCallIsNotCalledAfterTwoMinutes(t *testing.T) {
	clk := newClock(t0)
	r, c := New(clk.now), NewCorrelator(clk.now)
	clk.add(5 * time.Minute)
	signedIn := clk.now()
	c.SignIn(SignIn{At: signedIn, UserID: "u1", ClientID: "app-1", Scopes: openid})

	clk.add(time.Minute + 59*time.Second)
	if got := c.Check(r, jwtApps); len(got) != 0 {
		t.Fatalf("checked before the event could have settled: %+v", got)
	}
	clk.add(time.Second)
	got := c.Check(r, jwtApps)
	if len(got) != 1 || got[0].Verdict != NotCalled {
		t.Fatalf("got %+v, want one not_called", got)
	}
	if len(got[0].Missing) != 2 {
		t.Errorf("a JWT app with openid expects both functions, missing = %v", got[0].Missing)
	}
	obs := Observe(got)
	if len(obs) != 1 || obs[0].Kind != KindNotCalled || obs[0].Subject != "app-1" || !obs[0].Fail {
		t.Errorf("observations = %+v", obs)
	}
	if again := c.Check(r, jwtApps); len(again) != 0 {
		t.Errorf("a sign-in is checked once: %+v", again)
	}
}

func TestCorrelator_DegradedCallIsNotServed(t *testing.T) {
	clk := newClock(t0)
	r, c := New(clk.now), NewCorrelator(clk.now)
	clk.add(5 * time.Minute)
	at := clk.now()
	call(r, at, "u1", "app-1", FnPreAccessToken, Served)
	call(r, at.Add(time.Second), "u1", "app-1", FnPreUserinfo, DegradedNoFacts)
	call(r, at.Add(2*time.Second), "u1", "app-1", FnPreUserinfo, Served) // the worst in the window wins
	c.SignIn(SignIn{At: at, UserID: "u1", ClientID: "app-1", Scopes: openid})
	clk.add(3 * time.Minute)

	got := c.Check(r, jwtApps)
	if len(got) != 1 || got[0].Verdict != NotServed || got[0].Outcome != DegradedNoFacts {
		t.Fatalf("got %+v, want not_served degraded_no_facts", got)
	}
	ups := Fold(Observe(got))
	if len(ups) != 2 || ups[1].Kind != KindNotServed || ups[1].Fails != 1 || ups[1].Sample["outcome"] != "degraded_no_facts" {
		t.Errorf("updates = %+v", ups)
	}
	if ups[0].Kind != KindNotCalled || ups[0].Fails != 0 || ups[0].TrailingClean != 1 {
		t.Errorf("a call that was made is a clean look at not_called: %+v", ups[0])
	}
}

func TestCorrelator_LagTolerance(t *testing.T) {
	for _, tc := range []struct {
		offset time.Duration
		want   Verdict
	}{
		{-119 * time.Second, Delivered},
		{-30 * time.Second, Delivered},
		{90 * time.Second, Delivered},
		{120 * time.Second, Delivered},
		{-125 * time.Second, NotCalled},
		{150 * time.Second, NotCalled},
	} {
		t.Run(tc.offset.String(), func(t *testing.T) {
			clk := newClock(t0)
			r, c := New(clk.now), NewCorrelator(clk.now)
			clk.add(10 * time.Minute)
			at := clk.now()
			call(r, at.Add(tc.offset), "u1", "app-1", FnPreAccessToken, Served)
			c.SignIn(SignIn{At: at, UserID: "u1", ClientID: "app-1", Scopes: []string{}})
			clk.add(5 * time.Minute)
			got := c.Check(r, jwtApps)
			if len(got) != 1 || got[0].Verdict != tc.want {
				t.Fatalf("call at %s: got %+v, want %s", tc.offset, got, tc.want)
			}
		})
	}
}

func TestCorrelator_RestartIsUncheckedNeverFailed(t *testing.T) {
	clk := newClock(t0)
	r, c := New(clk.now), NewCorrelator(clk.now) // the process started at t0
	// A sign-in from just before the start, delivered late; its calls went to
	// the process before this one.
	c.SignIn(SignIn{At: t0.Add(-30 * time.Second), UserID: "u1", ClientID: "app-1", Scopes: openid})
	// One from inside the first two minutes: its window starts before t0.
	c.SignIn(SignIn{At: t0.Add(90 * time.Second), UserID: "u2", ClientID: "app-1", Scopes: openid})
	clk.add(5 * time.Minute)
	got := c.Check(r, jwtApps)
	if v := verdicts(got); v[Unchecked] != 2 || len(got) != 2 {
		t.Fatalf("got %v, want both unchecked", v)
	}
	if obs := Observe(got); len(obs) != 0 {
		t.Errorf("an unchecked sign-in is no evidence: %+v", obs)
	}
	if c.Since() != t0 {
		t.Errorf("since = %s", c.Since())
	}
}

func TestCorrelator_TooLateIsUnchecked(t *testing.T) {
	clk := newClock(t0)
	r, c := New(clk.now), NewCorrelator(clk.now)
	clk.add(30 * time.Minute)
	// An event that arrives 20 minutes after it happened.
	c.SignIn(SignIn{At: clk.now().Add(-20 * time.Minute), UserID: "u1", ClientID: "app-1", Scopes: openid})
	if got := c.Check(r, jwtApps); len(got) != 1 || got[0].Verdict != Unchecked {
		t.Fatalf("got %+v", got)
	}
}

func TestCorrelator_FirstSignInBeforeAccessIsCountedNotAlarmed(t *testing.T) {
	clk := newClock(t0)
	r, c := New(clk.now), NewCorrelator(clk.now)
	clk.add(10 * time.Minute)
	at := clk.now()
	c.Created("new", at.Add(-40*time.Second))
	call(r, at, "new", "app-1", FnPreAccessToken, NoGrants)
	call(r, at, "old", "app-1", FnPreAccessToken, NoGrants)
	c.SignIn(SignIn{At: at, UserID: "new", ClientID: "app-1", Scopes: []string{}})
	c.SignIn(SignIn{At: at, UserID: "old", ClientID: "app-1", Scopes: []string{}})
	clk.add(3 * time.Minute)
	got := c.Check(r, jwtApps)
	v := verdicts(got)
	if v[FirstSignIn] != 1 || v[NotServed] != 1 {
		t.Fatalf("got %v, want one first sign-in and one not served", v)
	}
	for _, o := range Observe(got) {
		if o.Fail && o.Sample["user_id"] == "new" {
			t.Errorf("a first sign-in raised %s", o.Kind)
		}
	}
	counts, _, _ := c.Summary()
	if counts[FirstSignIn] != 1 || counts[NotServed] != 1 {
		t.Errorf("summary = %v", counts)
	}
}

// With the welcome switch on, a new member's first sign-in is delivered.
func TestCorrelator_WelcomeAnswerIsDelivered(t *testing.T) {
	clk := newClock(t0)
	r, c := New(clk.now), NewCorrelator(clk.now)
	clk.add(10 * time.Minute)
	at := clk.now()
	c.Created("new", at.Add(-40*time.Second))
	call(r, at, "new", "app-1", FnPreAccessToken, ServedWelcome)
	c.SignIn(SignIn{At: at, UserID: "new", ClientID: "app-1", Scopes: []string{}})
	clk.add(3 * time.Minute)
	if v := verdicts(c.Check(r, jwtApps)); v[Delivered] != 1 {
		t.Fatalf("got %v, want delivered", v)
	}
}

func TestCorrelator_ExpectedFunctions(t *testing.T) {
	bearer := func(string) (bool, bool) { return false, true }
	unknown := func(string) (bool, bool) { return false, false }
	for _, tc := range []struct {
		name   string
		apps   AppSettings
		scopes []string
		calls  []string
		want   Verdict
	}{
		{"jwt without openid needs preaccesstoken", jwtApps, []string{"profile"}, []string{FnPreUserinfo}, NotCalled},
		{"jwt without openid, called", jwtApps, []string{"profile"}, []string{FnPreAccessToken}, Delivered},
		{"bearer with openid needs preuserinfo only", bearer, openid, []string{FnPreUserinfo}, Delivered},
		{"bearer with openid, not called", bearer, openid, nil, NotCalled},
		{"bearer without openid calls nothing at sign-in", bearer, []string{"profile"}, nil, Unchecked},
		{"settings unknown: any call counts", unknown, nil, []string{FnPreUserinfo}, Delivered},
		{"settings unknown: no call at all", unknown, nil, nil, NotCalled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			clk := newClock(t0)
			r, c := New(clk.now), NewCorrelator(clk.now)
			clk.add(10 * time.Minute)
			at := clk.now()
			for _, f := range tc.calls {
				call(r, at, "u1", "app-1", f, Served)
			}
			c.SignIn(SignIn{At: at, UserID: "u1", ClientID: "app-1", Scopes: tc.scopes})
			clk.add(3 * time.Minute)
			if got := c.Check(r, tc.apps); len(got) != 1 || got[0].Verdict != tc.want {
				t.Fatalf("got %+v, want %s", got, tc.want)
			}
		})
	}
}

// A shape that lands only in the access token answers preuserinfo with
// not_landing: by design, and still delivered.
func TestCorrelator_NotLandingIsDelivered(t *testing.T) {
	clk := newClock(t0)
	r, c := New(clk.now), NewCorrelator(clk.now)
	clk.add(10 * time.Minute)
	at := clk.now()
	call(r, at, "u1", "app-1", FnPreAccessToken, Served)
	call(r, at, "u1", "app-1", FnPreUserinfo, NotLanding)
	c.SignIn(SignIn{At: at, UserID: "u1", ClientID: "app-1", Scopes: openid})
	clk.add(3 * time.Minute)
	if got := c.Check(r, jwtApps); got[0].Verdict != Delivered {
		t.Fatalf("got %+v", got)
	}
}

// An app calling userinfo every few seconds fills a pair's ring; a sign-in
// whose calls were pushed out is unchecked, not "not called".
func TestCorrelator_RingOverflowIsUnchecked(t *testing.T) {
	clk := newClock(t0)
	r, c := New(clk.now), NewCorrelator(clk.now)
	clk.add(10 * time.Minute)
	at := clk.now()
	call(r, at, "u1", "app-1", FnPreAccessToken, Served)
	for i := 1; i <= 2*spansPerPair; i++ {
		call(r, at.Add(time.Duration(i)*(spanMax+time.Second)), "u1", "app-1", FnPreAccessToken, Served)
	}
	c.SignIn(SignIn{At: at, UserID: "u1", ClientID: "app-1", Scopes: []string{}})
	clk.add(3 * time.Minute)
	if got := c.Check(r, jwtApps); got[0].Verdict != Unchecked {
		t.Fatalf("got %+v, want unchecked", got)
	}
}

// Term start: 1,000 people sign in to 20 apps inside ten minutes, each with
// an access token and an ID token, beside unrelated refresh traffic. Not one
// false finding.
func TestCorrelator_ThousandSignInBurstNoFalseFindings(t *testing.T) {
	clk := newClock(t0)
	r, c := New(clk.now), NewCorrelator(clk.now)
	clk.add(10 * time.Minute)
	start := clk.now()
	rng := rand.New(rand.NewSource(1))
	type ev struct {
		at           time.Time
		user, client string
	}
	var evs []ev
	for i := 0; i < 1000; i++ {
		at := start.Add(time.Duration(rng.Int63n(int64(10 * time.Minute))))
		evs = append(evs, ev{at, fmt.Sprintf("u%04d", i), fmt.Sprintf("app-%02d", i%20)})
	}
	for _, e := range evs {
		// The calls land within a few seconds of the session; refreshes and
		// userinfo reads follow.
		call(r, e.at.Add(time.Duration(rng.Int63n(int64(3*time.Second)))), e.user, e.client, FnPreAccessToken, Served)
		call(r, e.at.Add(time.Duration(rng.Int63n(int64(3*time.Second)))), e.user, e.client, FnPreUserinfo, Served)
		for k := 0; k < 3; k++ {
			call(r, e.at.Add(time.Duration(rng.Int63n(int64(5*time.Minute)))), e.user, e.client, FnPreUserinfo, Served)
		}
		call(r, e.at, e.user+"-other", e.client, FnPreAccessToken, DegradedNoFacts) // someone else's trouble
	}
	// Events arrive up to 90s late, out of order.
	rng.Shuffle(len(evs), func(i, j int) { evs[i], evs[j] = evs[j], evs[i] })
	for _, e := range evs {
		c.SignIn(SignIn{At: e.at, UserID: e.user, ClientID: e.client, Scopes: openid})
		c.SignIn(SignIn{At: e.at, UserID: e.user, ClientID: e.client, Scopes: openid}) // redelivered
	}
	var all []Checked
	for m := 0; m < 15; m++ {
		clk.add(time.Minute)
		all = append(all, c.Check(r, jwtApps)...)
	}
	v := verdicts(all)
	if v[Delivered] != 1000 || len(all) != 1000 {
		t.Fatalf("verdicts = %v over %d checked, want 1000 delivered", v, len(all))
	}
	for _, u := range Fold(Observe(all)) {
		if u.Fails > 0 {
			t.Fatalf("false finding %+v", u)
		}
	}
}

func TestCorrelator_EventsMissing(t *testing.T) {
	clk := newClock(t0)
	c := NewCorrelator(clk.now)
	clk.add(23 * time.Hour)
	if c.EventsMissing(clk.now()) {
		t.Fatal("blind before 24h of watching")
	}
	clk.add(2 * time.Hour)
	if !c.EventsMissing(clk.now().Add(-time.Hour)) {
		t.Fatal("calls for a day and no sign-in event: blind")
	}
	if c.EventsMissing(time.Time{}) {
		t.Error("no calls at all is not the detector going blind")
	}
	c.SignIn(SignIn{At: clk.now(), UserID: "u", ClientID: "a"})
	if c.EventsMissing(clk.now()) {
		t.Error("a sign-in event arrived")
	}
}

func TestFold_SettlesOnTrailingCleans(t *testing.T) {
	at := t0
	obs := []Observation{
		{Kind: KindNotCalled, Subject: "a", Fail: true, At: at, Sample: map[string]any{"n": 1}},
		{Kind: KindNotCalled, Subject: "b", At: at},
		{Kind: KindNotCalled, Subject: "a", Fail: true, At: at.Add(time.Minute), Sample: map[string]any{"n": 2}},
		{Kind: KindNotCalled, Subject: "a", At: at.Add(2 * time.Minute)},
		{Kind: KindNotCalled, Subject: "a", At: at.Add(3 * time.Minute)},
	}
	ups := Fold(obs)
	if len(ups) != 2 {
		t.Fatalf("ups = %+v", ups)
	}
	a := ups[0]
	if a.Fails != 2 || a.TrailingClean != 2 || a.Sample["n"] != 2 || !a.FirstFail.Equal(at) || !a.LastFail.Equal(at.Add(time.Minute)) {
		t.Errorf("a = %+v", a)
	}
	if b := ups[1]; b.Fails != 0 || b.TrailingClean != 1 {
		t.Errorf("b = %+v", b)
	}
}

func TestFindingReasonCarriesNoIDs(t *testing.T) {
	if got := FindingReason(KindConfigDrift, "R5:app-123"); got != "CONFIG_DRIFT:R5" {
		t.Errorf("got %q", got)
	}
	if got := FindingReason(KindNotCalled, "123@project"); got != KindNotCalled {
		t.Errorf("got %q", got)
	}
	if FindingSeverity(KindNotServed, map[string]any{"outcome": "sig_mismatch"}) != Hard ||
		FindingSeverity(KindNotServed, map[string]any{"outcome": "degraded_no_facts"}) != Soft {
		t.Error("not served takes its outcome's severity")
	}
}

// Zitadel listing no grants is the right answer when Syndra's own facts say
// the person holds no role in the app's project (seen on prod 3 Nov 2026: an
// operator with no role in Syndra's own project signing in to Syndra). It is
// a failure only when Syndra knows they hold one.
func TestClassify_NoGrantsIsOnlyAFailureWhenThePersonHoldsRoles(t *testing.T) {
	s := SignIn{UserID: "ada", ClientID: "c1", Scopes: []string{"openid"}}
	worst := map[string]Outcome{FnPreUserinfo: NoGrants}
	if got := Classify(s, worst, false, false, true, false, true).Verdict; got != NothingToDeliver {
		t.Fatalf("holds no role: got %s, want %s", got, NothingToDeliver)
	}
	if got := Classify(s, worst, false, false, true, false, false).Verdict; got != NotServed {
		t.Fatalf("holds a role (or unknown): got %s, want %s", got, NotServed)
	}
	if obs := Observe([]Checked{{SignIn: s, Verdict: NothingToDeliver}}); len(obs) != 2 || obs[0].Fail || obs[1].Fail {
		t.Fatalf("nothing to deliver must count clean: %+v", obs)
	}
}
