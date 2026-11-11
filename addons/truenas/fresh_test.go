package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Fresh reads (feature freshness, fresh.go): the copy without `fresh`, a read
// of the NAS with it, inside a cooldown, one read for concurrent asks, the copy
// labelled unreachable when the NAS cannot be read, and every write forgetting
// every copy.

// keepCopy makes val the copy of key, read at at.
func keepCopy(s *server, key string, val any, at time.Time) {
	_, _, _ = s.fresh.read(key, true, func() (any, time.Time, error) { return val, at, nil })
}

// ageCopies moves the fresher's clock by d, so every copy and cooldown is d
// older.
func ageCopies(s *server, d time.Duration) {
	s.fresh.now = func() time.Time { return time.Now().Add(d) }
}

// at pins the fresher's clock to a moment the test moves.
func at(s *server, start time.Time) *time.Time {
	now := start
	s.fresh.now = func() time.Time { return now }
	return &now
}

func getSubjects(t *testing.T, s *server, fresh bool) (*httptest.ResponseRecorder, SubjectsResponse) {
	t.Helper()
	target := "/subjects"
	if fresh {
		target += "?fresh=true"
	}
	rr := httptest.NewRecorder()
	s.handleSubjects(rr, httptest.NewRequest(http.MethodGet, target, nil), nil)
	var out SubjectsResponse
	if rr.Code == http.StatusOK {
		assertSchema(t, "subjects.json", rr.Body.Bytes(), "/subjects "+target) // the invariant, both ways
		if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
	}
	return rr, out
}

func userReads(f *fakeRPC) int { return strings.Count(strings.Join(f.calls, " "), "user.query") }

func TestAReadWithoutFreshIsTheCopyAndTouchesNothing(t *testing.T) {
	fake := &fakeRPC{users: fixtureUsers, groups: fixtureGroups}
	s := testServer(t, fake)
	if _, first := getSubjects(t, s, true); !first.Current || first.Stale != nil {
		t.Fatalf("a fresh read is current with no stale: %+v", first.Observed)
	}
	reads, counted := userReads(fake), s.nas.reads.Load()

	rr, copy := getSubjects(t, s, false)
	if rr.Code != http.StatusOK || copy.Current || copy.Stale == nil || copy.Stale.Reason != "cached" {
		t.Fatalf("a read without fresh answers the copy, current false, cached: %d %+v", rr.Code, copy.Observed)
	}
	if userReads(fake) != reads || s.nas.reads.Load() != counted {
		t.Errorf("the copy read the NAS: user.query %d -> %d, target_reads %d -> %d",
			reads, userReads(fake), counted, s.nas.reads.Load())
	}
	if len(copy.Subjects) != 2 {
		t.Errorf("the copy lost the accounts: %+v", copy.Subjects)
	}
}

func TestAFreshReadInsideTheCooldownIsTheCopySaidSo(t *testing.T) {
	fake := &fakeRPC{users: fixtureUsers, groups: fixtureGroups}
	s := testServer(t, fake)
	now := at(s, time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC))
	getSubjects(t, s, true)
	reads := userReads(fake)

	*now = now.Add(10*time.Second + 500*time.Millisecond)
	_, again := getSubjects(t, s, true)
	if again.Current || again.Stale == nil || again.Stale.Reason != "cooldown" {
		t.Fatalf("a fresh read 10.5 s after one is the copy, cooldown: %+v", again.Observed)
	}
	// Rounded up, never down: told 19, a caller would arrive half a second
	// early and be told to wait again.
	if again.Stale.RetryAfter != 20 {
		t.Errorf("retry_after = %d, want 20 (30 s less 10.5 s, rounded up)", again.Stale.RetryAfter)
	}
	if userReads(fake) != reads {
		t.Error("a fresh read inside the cooldown read the NAS")
	}

	*now = now.Add(20 * time.Second)
	if _, later := getSubjects(t, s, true); !later.Current || userReads(fake) != reads+1 {
		t.Errorf("after the cooldown a fresh read reads the NAS: current=%v reads %d", later.Current, userReads(fake)-reads)
	}
}

// With nothing to serve inside the cooldown, 429 with the seconds left.
func TestAFreshReadInsideTheCooldownWithNothingToServeIs429(t *testing.T) {
	fake := &fakeRPC{users: fixtureUsers, groups: fixtureGroups, err: errReadAfterWrite}
	s := testServer(t, fake)
	now := at(s, time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC))
	get := func() *httptest.ResponseRecorder {
		rr := httptest.NewRecorder()
		s.handleHomes(rr, httptest.NewRequest(http.MethodGet, "/homes?fresh=true", nil), nil)
		return rr
	}
	if rr := get(); rr.Code == http.StatusOK {
		t.Fatalf("a NAS that cannot be read with nothing kept is not a list: %s", rr.Body)
	}
	*now = now.Add(5 * time.Second)
	rr := get()
	if rr.Code != http.StatusTooManyRequests || !strings.Contains(rr.Body.String(), CodeFreshReadCooldown) {
		t.Fatalf("want 429 %s, got %d %s", CodeFreshReadCooldown, rr.Code, rr.Body)
	}
	if got := rr.Header().Get("Retry-After"); got != "25" {
		t.Errorf("Retry-After = %q, want the 25 s left", got)
	}
	assertSchema(t, "error.json", rr.Body.Bytes(), "a cooldown refusal")
}

// Two concurrent fresh reads of one thing are one read of the NAS. The
// second arrived after the read began, so it is not current (B1).
func TestConcurrentFreshReadsAreOneRead(t *testing.T) {
	var f fresher
	var reads atomic.Int32
	started, release := make(chan struct{}), make(chan struct{})
	live := func() (any, time.Time, error) {
		if reads.Add(1) == 1 {
			close(started)
		}
		<-release
		return "accounts", time.Now(), nil
	}
	var wg sync.WaitGroup
	obs := make([]Observed, 2)
	errs := make([]error, 2)
	wg.Add(1)
	go func() { defer wg.Done(); _, obs[0], errs[0] = f.read("subjects", true, live) }()
	<-started
	wg.Add(1)
	go func() { defer wg.Done(); _, obs[1], errs[1] = f.read("subjects", true, live) }()
	time.Sleep(50 * time.Millisecond) // the second is waiting on the first
	close(release)
	wg.Wait()

	if n := reads.Load(); n != 1 {
		t.Fatalf("two concurrent fresh reads made %d reads of the NAS", n)
	}
	if errs[0] != nil || errs[1] != nil {
		t.Fatalf("both are answered from the one read: %v, %v", errs[0], errs[1])
	}
	if !obs[0].Current || obs[1].Current || obs[1].Stale == nil || obs[1].Stale.Reason != "cooldown" || obs[1].Stale.RetryAfter < 1 {
		t.Errorf("the first is current, the joiner the read it joined, cooldown: %+v / %+v", obs[0], obs[1].Stale)
	}
}

// One subject's fresh read does not cool another's (a per-subject key).
func TestOneSubjectsCooldownIsNotAnothers(t *testing.T) {
	s, m := applyServer(t, `[{"username":"charlotte","id":11,"uid":3001,"locked":false,"smb":true,"password_disabled":false,"groups":[]},`+
		`{"username":"asuka","id":12,"uid":3002,"locked":false,"smb":true,"password_disabled":false,"groups":[]}]`)
	for _, b := range []Binding{{SubjectID: "sub-1", Username: "charlotte", UID: 3001}, {SubjectID: "sub-2", Username: "asuka", UID: 3002}} {
		if err := s.store.PutBinding(b); err != nil {
			t.Fatal(err)
		}
	}
	status := func(subject string) Observed {
		rr := postOperation(t, s, "storage.status", `{"call_id":"read-1","operation":"storage.status","subject":"`+subject+`","fresh":true}`)
		if rr.Code != http.StatusOK {
			t.Fatalf("storage.status %s: %d %s", subject, rr.Code, rr.Body)
		}
		assertSchema(t, "operation_response.json", rr.Body.Bytes(), "storage.status")
		var out struct{ Result Observed }
		_ = json.Unmarshal(rr.Body.Bytes(), &out)
		return out.Result
	}
	status("sub-1")
	before := userReads(&m.fakeRPC)
	if got := status("sub-2"); !got.Current || userReads(&m.fakeRPC) == before {
		t.Errorf("sub-2's fresh read was cooled by sub-1's: %+v", got)
	}
	if got := status("sub-1"); got.Current || got.Stale == nil || got.Stale.Reason != "cooldown" {
		t.Errorf("sub-1's own second fresh read is in its cooldown: %+v", got)
	}
}

// A NAS that cannot be read yields the copy, unreachable, never an error.
func TestAnUnreadableNASAnswersTheCopyNotAnError(t *testing.T) {
	fake := &fakeRPC{users: fixtureUsers, groups: fixtureGroups}
	s := testServer(t, fake)
	now := at(s, time.Now())
	getSubjects(t, s, true)
	*now = now.Add(time.Minute)
	fake.err = errReadAfterWrite
	s.store.db.Close() // the disk mirror out of reach: the copy in memory answers
	rr, out := getSubjects(t, s, true)
	if rr.Code != http.StatusOK || out.Current || out.Stale == nil || out.Stale.Reason != "unreachable" || len(out.Subjects) != 2 {
		t.Fatalf("want 200, the copy, unreachable: %d %s", rr.Code, rr.Body)
	}
}

// Nothing kept, no mirror, and no request reached the NAS: 503
// TARGET_UNREACHABLE with Retry-After (B3).
func TestNothingToServeAndNothingSentIsTargetUnreachable(t *testing.T) {
	s := testServer(t, &fakeRPC{})
	s.nas = newNAS(func() (rpc, error) { return nil, errors.New("connection refused") }, []string{"25.04"})
	rr, _ := getSubjects(t, s, true)
	if rr.Code != http.StatusServiceUnavailable || !strings.Contains(rr.Body.String(), CodeTargetUnreachable) || rr.Header().Get("Retry-After") == "" {
		t.Fatalf("want 503 %s with Retry-After, got %d %q %s", CodeTargetUnreachable, rr.Code, rr.Header().Get("Retry-After"), rr.Body)
	}
}

// /plan is never cooled down and never answered from the copy: straight after
// a fresh /subjects it reads the NAS and is current.
func TestPlanAfterAFreshReadIsCurrent(t *testing.T) {
	s, m := applyServer(t, `[{"username":"charlotte","id":11,"uid":3001,"locked":false,"smb":true,"groups":[]}]`)
	getSubjects(t, s, true)
	before := userReads(&m.fakeRPC)
	rr := httptest.NewRecorder()
	planInto(t, s, rr, `{"subjects":[{"subject":"sub-1","email":"charlotte@example.org","desired":{"enabled":true}}]}`)
	var plan PlanResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &plan); err != nil || rr.Code != http.StatusOK || !plan.Current {
		t.Fatalf("a plan straight after a fresh read is current: %d %s", rr.Code, rr.Body)
	}
	if userReads(&m.fakeRPC) == before {
		t.Error("the plan did not read the NAS")
	}
}

// A write forgets every copy: the member waiting on their own password sees
// it on the next poll, which does not ask for a fresh read.
func TestAWriteForgetsEveryCopy(t *testing.T) {
	fake := &fakeRPC{users: fixtureUsers, groups: fixtureGroups}
	s := testServer(t, fake)
	getSubjects(t, s, true)
	if err := s.mutate(caller{"password.set", "sub-1", "sub-1", "call-1"}, "store.binding", func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	reads := userReads(fake)
	if _, out := getSubjects(t, s, false); !out.Current || userReads(fake) != reads+1 {
		t.Errorf("the first read after a write reads the NAS: current=%v", out.Current)
	}
}

// A read that began before a write never becomes the copy after it.
func TestAReadThatBeganBeforeAWriteIsNotKept(t *testing.T) {
	var f fresher
	release := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _, _ = f.read("subjects", false, func() (any, time.Time, error) { <-release; return "before", time.Now(), nil })
	}()
	for {
		f.mu.Lock()
		running := f.flights["subjects"] != nil
		f.mu.Unlock()
		if running {
			break
		}
		time.Sleep(time.Millisecond)
	}
	f.forget()
	close(release)
	<-done
	val, obs, _ := f.read("subjects", false, func() (any, time.Time, error) { return "after", time.Now(), nil })
	if val != "after" || !obs.Current {
		t.Errorf("the read from before the write was kept: %v %+v", val, obs)
	}
}

// target_reads counts every read of the NAS (NAS.call), and nothing else: a
// write is not a read.
func TestTargetReadsCountsEveryReadAndNoWrite(t *testing.T) {
	fake := &fakeRPC{users: fixtureUsers, groups: fixtureGroups}
	s := testServer(t, fake)
	var out []nasGroup
	if err := s.nas.call("group.query", []any{}, &out); err != nil { // and the session's probes, each a read
		t.Fatal(err)
	}
	before := s.nas.reads.Load()
	if err := s.nas.call("group.query", []any{}, &out); err != nil {
		t.Fatal(err)
	}
	if got := s.nas.reads.Load() - before; got != 1 {
		t.Errorf("one read counted %d", got)
	}
	_ = s.nas.sendWrite("user.update", []any{1, map[string]any{}}, nil)
	if got := s.nas.reads.Load() - before; got != 1 {
		t.Errorf("a write was counted as a read: %d", got)
	}
	rr := httptest.NewRecorder()
	s.handleHealth(rr, httptest.NewRequest(http.MethodGet, "/health", nil), nil)
	var h Health
	_ = json.Unmarshal(rr.Body.Bytes(), &h)
	if h.TargetReads != s.nas.reads.Load() || h.TargetReads == 0 {
		t.Errorf("health says %d reads, the add-on made %d", h.TargetReads, s.nas.reads.Load())
	}
}

// Every reply in scope carries the invariant whichever path answered it.
func TestReachAndHomesSayHowFreshTheyAre(t *testing.T) {
	fake := reachFake()
	s := testServer(t, fake)
	for _, tc := range []struct {
		path, schema string
		handle       func(http.ResponseWriter, *http.Request, []byte)
	}{
		{"/reach", "profiles/storage/reach.json", s.handleReach},
		{"/homes", "profiles/storage/homes.json", s.handleHomes},
	} {
		for i, want := range []string{"", "cached", "cooldown"} {
			target := tc.path
			if i != 1 {
				target += "?fresh=true"
			}
			rr := httptest.NewRecorder()
			tc.handle(rr, httptest.NewRequest(http.MethodGet, target, nil), nil)
			assertSchema(t, tc.schema, rr.Body.Bytes(), target)
			var o Observed
			_ = json.Unmarshal(rr.Body.Bytes(), &o)
			got := ""
			if o.Stale != nil {
				got = o.Stale.Reason
			}
			if got != want {
				t.Errorf("%s #%d: stale %q, want %q: %s", tc.path, i, got, want, rr.Body)
			}
		}
	}
}

// The manifest declares freshness with its cooldown, and health carries the
// count the kit proves it with.
func TestFreshnessIsDeclaredWithItsCooldownAndItsCount(t *testing.T) {
	s := testServer(t, &fakeRPC{users: fixtureUsers, groups: fixtureGroups})
	m := manifest(productName, "25.04.2", s, nil, s.fresh.cooldownISO())
	if !slices.Contains(m.Features, "freshness") || m.Limits.FreshReadCooldown != "PT30S" {
		t.Errorf("features %v, cooldown %q", m.Features, m.Limits.FreshReadCooldown)
	}
	rr := httptest.NewRecorder()
	s.handleHealth(rr, httptest.NewRequest(http.MethodGet, "/health", nil), nil)
	if !strings.Contains(rr.Body.String(), `"target_reads":`) {
		t.Errorf("health carries no target_reads: %s", rr.Body)
	}
}

// A read the add-on refuses on its merits (an operation's 4xx: the account is
// gone) is answered as refused, never covered by the copy: only a NAS that
// cannot be read yields the copy.
func TestARefusalIsNotCoveredByTheCopy(t *testing.T) {
	s, m := applyServer(t, `[{"username":"charlotte","id":11,"uid":3001,"locked":false,"smb":true,"password_disabled":false,"groups":[]}]`)
	if err := s.store.PutBinding(Binding{SubjectID: "sub-1", Username: "charlotte", UID: 3001}); err != nil {
		t.Fatal(err)
	}
	now := at(s, time.Now())
	read := func() *httptest.ResponseRecorder {
		return postOperation(t, s, "storage.status", `{"call_id":"read-1","operation":"storage.status","subject":"sub-1","fresh":true}`)
	}
	if rr := read(); rr.Code != http.StatusOK {
		t.Fatalf("first read: %d %s", rr.Code, rr.Body)
	}
	m.fakeRPC.users = `[]` // removed on the NAS by hand
	*now = now.Add(time.Minute)
	if rr := read(); rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("an account gone from the NAS answered %d %s, want the 422 refusal", rr.Code, rr.Body)
	}
}
