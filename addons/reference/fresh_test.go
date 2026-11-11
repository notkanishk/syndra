package reference

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

// An unreachable app is the half of freshness a black-box kit cannot take
// offline (B4), so it is held here: a read answers the last value it read with
// stale unreachable, never an error, and only with nothing kept is it 503
// TARGET_UNREACHABLE with Retry-After.
func TestAnUnreachableAppAnswersTheLastValueNotAnError(t *testing.T) {
	var down atomic.Bool
	a, err := New(Config{Target: "reference", Secret: []byte("s"), Features: []string{"freshness"}, Down: down.Load})
	if err != nil {
		t.Fatal(err)
	}
	read := func(fresh bool) (reply, map[string]any) {
		a.mu.Lock()
		a.fresh.began = time.Time{} // past the cooldown, without waiting it out
		a.mu.Unlock()
		rep := a.freshSubjects(fresh)
		var doc map[string]any
		_ = json.Unmarshal(rep.body, &doc)
		return rep, doc
	}

	down.Store(true)
	if rep, doc := read(true); rep.status != 503 || doc["code"] != "TARGET_UNREACHABLE" || rep.retryAfter <= 0 {
		t.Fatalf("nothing kept and the app down answered %d %v (Retry-After %d), want 503 TARGET_UNREACHABLE with Retry-After",
			rep.status, doc["code"], rep.retryAfter)
	}

	down.Store(false)
	_, live := read(true)
	if live["current"] != true {
		t.Fatalf("a fresh read of a reachable app answered %v", live)
	}

	down.Store(true)
	for _, fresh := range []bool{true, false} {
		rep, doc := read(fresh)
		stale, _ := doc["stale"].(map[string]any)
		want := map[bool]string{true: "unreachable", false: "cached"}[fresh]
		if rep.status != 200 || doc["current"] != false || stale["reason"] != want || doc["taken_at"] != live["taken_at"] {
			t.Errorf("fresh=%v with the app down answered %d %v, want 200, the last value (taken_at %v), stale %s",
				fresh, rep.status, doc, live["taken_at"], want)
		}
	}
}

// fresh in an operation's body is a request field of feature freshness: an
// add-on that does not declare it refuses it like any unknown field, and one
// that does answers the read.
func TestFreshInAnOperationIsAFieldOfTheFeature(t *testing.T) {
	body := []byte(`{"contract_version":2,"call_id":"read-1","operation":"health.get","subject":"","actor":"","params":{},"fresh":true}`)
	for features, want := range map[string]int{"": 400, "freshness": 200} {
		a, err := New(Config{Target: "reference", Secret: []byte("s"), Features: strings.Fields(features)})
		if err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest(http.MethodPost, "/operations/health.get", nil)
		r.SetPathValue("name", "health.get")
		if rep := a.operation(r, body); rep.status != want {
			t.Errorf("features %q: fresh: true answered %d %s, want %d", features, rep.status, rep.body, want)
		}
	}
}

// A fresh read that arrives while another is reading the app waits for it and
// answers its value (request collapsing): one read of the app, and the joiner
// is answered, not refused, even with nothing kept before. It is cooldown, not
// current: that read began before it arrived.
func TestAConcurrentFreshReadJoinsTheReadInFlight(t *testing.T) {
	a, err := New(Config{Target: "reference", Secret: []byte("s"), Features: []string{"freshness"}})
	if err != nil {
		t.Fatal(err)
	}
	var docs [2]map[string]any
	var codes [2]int
	done := make(chan int)
	for i := range docs {
		go func() {
			rep := a.freshSubjects(true)
			codes[i] = rep.status
			_ = json.Unmarshal(rep.body, &docs[i])
			done <- i
		}()
		time.Sleep(readDwell / 3) // the second arrives while the first reads
	}
	<-done
	<-done
	if a.fresh.reads != 1 || codes != [2]int{200, 200} {
		t.Fatalf("two fresh reads read the app %d times and answered %v, want one read and two answers", a.fresh.reads, codes)
	}
	stale, _ := docs[1]["stale"].(map[string]any)
	if docs[0]["current"] != true || stale["reason"] != "cooldown" || docs[1]["taken_at"] != docs[0]["taken_at"] {
		t.Errorf("the first answered %v, the joiner %v: want current, then the same read as cooldown", docs[0], docs[1])
	}
}

// A read arriving after a write while a read begun before it is still running
// reads the app again: never that read's answer, never its cooldown. And the
// earlier read, ending, leaves the new one in flight for the next to join.
func TestAReadAfterAWriteNeverJoinsAReadBegunBeforeIt(t *testing.T) {
	a, err := New(Config{Target: "reference", Secret: []byte("s"), Features: []string{"freshness"}})
	if err != nil {
		t.Fatal(err)
	}
	read := func() (int, map[string]any) {
		rep := a.freshSubjects(true)
		var doc map[string]any
		_ = json.Unmarshal(rep.body, &doc)
		return rep.status, doc
	}
	before := make(chan struct{})
	go func() { read(); close(before) }()
	time.Sleep(readDwell / 3) // the first read is under way
	a.mu.Lock()
	a.record("write")
	a.mu.Unlock()
	type answer struct {
		status int
		doc    map[string]any
	}
	after := make(chan answer)
	go func() { s, d := read(); after <- answer{s, d} }()
	<-before // the read from before the write has ended; the one after runs
	status, joined := read()
	got := <-after
	a.mu.Lock()
	reads := a.fresh.reads
	a.mu.Unlock()
	if got.status != 200 || got.doc["current"] != true || reads != 2 {
		t.Errorf("a read after the write answered %d %v after %d reads, want a read of its own, current", got.status, got.doc, reads)
	}
	if stale, _ := joined["stale"].(map[string]any); status != 200 || stale["reason"] != "cooldown" {
		t.Errorf("a read joining the one after the write answered %d %v, want it as cooldown", status, joined)
	}
}

// Inside the cooldown of a read that FAILED, a fresh read is answered as that
// failure was, not as "read moments ago": the copy `unreachable`, or the error
// when nothing is kept (never 429 and never `cooldown`), with no second attempt
// on the app. This is the TrueNAS add-on's rule (fresher.fallback), so the kit
// sees one add-on behave one way.
func TestAFreshReadInsideTheCooldownOfAFailedReadSaysUnreachable(t *testing.T) {
	var down atomic.Bool
	a, err := New(Config{Target: "reference", Secret: []byte("s"), Features: []string{"freshness"}, Down: down.Load})
	if err != nil {
		t.Fatal(err)
	}
	answer := func() (reply, map[string]any) {
		rep := a.freshSubjects(true)
		var doc map[string]any
		_ = json.Unmarshal(rep.body, &doc)
		return rep, doc
	}
	reads := func() int { a.mu.Lock(); defer a.mu.Unlock(); return a.fresh.reads }
	lapse := func() { a.mu.Lock(); a.fresh.began = time.Now().Add(-time.Hour); a.mu.Unlock() }

	// Nothing kept: the failed read, then another at once, are both 503.
	down.Store(true)
	for i := range 2 {
		if rep, doc := answer(); rep.status != 503 || doc["code"] != "TARGET_UNREACHABLE" {
			t.Fatalf("attempt %d with nothing kept answered %d %v, want 503 TARGET_UNREACHABLE", i+1, rep.status, doc["code"])
		}
	}
	if got := reads(); got != 1 {
		t.Fatalf("the cooldown let %d reads through, want 1", got)
	}

	// A copy kept, then a failed read, then a fresh read inside its cooldown.
	down.Store(false)
	lapse()
	_, live := answer()
	down.Store(true)
	lapse()
	if _, doc := answer(); doc["stale"].(map[string]any)["reason"] != "unreachable" {
		t.Fatalf("the failed read itself answered %v", doc)
	}
	before := reads()
	rep, doc := answer()
	stale, _ := doc["stale"].(map[string]any)
	if rep.status != 200 || doc["current"] != false || stale["reason"] != "unreachable" || stale["retry_after"] != nil || doc["taken_at"] != live["taken_at"] {
		t.Errorf("inside the cooldown of a failed read: %d %v, want 200, the last value (taken_at %v), stale unreachable with no retry_after", rep.status, doc, live["taken_at"])
	}
	if reads() != before {
		t.Errorf("the cooldown let a second attempt reach the app")
	}

	// A success clears it: the next read inside its cooldown is a plain cooldown again.
	down.Store(false)
	lapse()
	answer()
	if _, doc := answer(); doc["stale"].(map[string]any)["reason"] != "cooldown" {
		t.Errorf("after a successful read the cooldown answered %v, want cooldown", doc["stale"])
	}
}

// A read that joins a read of the app that FAILS is answered as that failure
// was: the mirror unreachable, or the error when nothing is kept. Never
// "cooldown" (that read did not succeed moments ago), and never a read of the
// app of its own: a plain read joining a failed one with nothing kept does
// not read the app again. The TrueNAS add-on's fresher answers a joiner the
// same (fresher.fallback), so the kit sees one add-on behave one way.
func TestAReadJoiningAFailedReadIsAnsweredAsItWas(t *testing.T) {
	var down atomic.Bool
	a, err := New(Config{Target: "reference", Secret: []byte("s"), Features: []string{"freshness"}, Down: down.Load})
	if err != nil {
		t.Fatal(err)
	}
	reads := func() int { a.mu.Lock(); defer a.mu.Unlock(); return a.fresh.reads }
	lapse := func() { a.mu.Lock(); a.fresh.began = time.Now().Add(-time.Hour); a.mu.Unlock() }
	// together sends a fresh read, then, while it reads the app, the joiner.
	together := func(joinerFresh bool) (reply, map[string]any) {
		lead := make(chan struct{})
		go func() { a.freshSubjects(true); close(lead) }()
		time.Sleep(readDwell / 3)
		rep := a.freshSubjects(joinerFresh)
		<-lead
		var doc map[string]any
		_ = json.Unmarshal(rep.body, &doc)
		return rep, doc
	}

	down.Store(true)
	for _, fresh := range []bool{false, true} {
		lapse()
		before := reads()
		if rep, doc := together(fresh); rep.status != 503 || doc["code"] != "TARGET_UNREACHABLE" {
			t.Errorf("fresh=%v joining a failed read, nothing kept: %d %v, want 503 TARGET_UNREACHABLE", fresh, rep.status, doc)
		}
		if got := reads() - before; got != 1 {
			t.Errorf("fresh=%v joining a failed read, nothing kept, read the app %d times, want 1", fresh, got)
		}
	}

	down.Store(false)
	lapse()
	a.freshSubjects(true) // a mirror kept
	down.Store(true)
	lapse()
	before := reads()
	rep, doc := together(true)
	stale, _ := doc["stale"].(map[string]any)
	if rep.status != 200 || doc["current"] != false || stale["reason"] != "unreachable" || stale["retry_after"] != nil {
		t.Errorf("a fresh read joining a failed read, a mirror kept: %d %v, want the mirror, unreachable", rep.status, doc)
	}
	if got := reads() - before; got != 1 {
		t.Errorf("joining a failed read read the app %d times, want 1", got)
	}
}

// A reader joining a read that fails is answered as that failure is, even when
// another reader starts a new read before the joiner wakes: it never joins the
// second read and answers its result as its own. The two reads are played by
// hand, as freshSubjects leaves them under a.mu, so the interleaving the
// scheduler rarely gives is the only one there is; synctest says when the
// joiner is parked on the read it joined.
func TestAJoinerOfAFailedReadNeverJoinsTheNextRead(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		a, err := New(Config{Target: "reference", Secret: []byte("s"), Features: []string{"freshness"}})
		if err != nil {
			t.Fatal(err)
		}
		f := &a.fresh
		begin := func() chan struct{} { // a reader starts a read of the app
			fl := make(chan struct{})
			f.flight, f.began = fl, time.Now()
			return fl
		}
		a.mu.Lock()
		first := begin()
		a.mu.Unlock()

		joined := make(chan reply, 1)
		go func() { joined <- a.freshSubjects(true) }()
		synctest.Wait() // the joiner waits on the first read

		a.mu.Lock()
		f.flight, f.failed = nil, true // the first read fails
		close(first)
		second := begin() // another reader begins the next, before the joiner wakes
		a.mu.Unlock()
		synctest.Wait()

		a.mu.Lock()
		f.flight, f.failed = nil, false // the next read succeeds
		a.keep([]subject{}, now(), f.gen)
		close(second)
		a.mu.Unlock()

		rep := <-joined
		var doc map[string]any
		_ = json.Unmarshal(rep.body, &doc)
		if rep.status != 503 || doc["code"] != "TARGET_UNREACHABLE" {
			t.Errorf("a joiner of a failed read answered %d %v, want 503 TARGET_UNREACHABLE: the read it joined, not the next one", rep.status, doc)
		}
	})
}

// A write forgets a failure with everything else: a read joining one begun
// after a failure, which a second write then forgets, decides again and reads
// the app, never answers the forgotten failure.
func TestAWriteForgetsTheFailureToo(t *testing.T) {
	var down atomic.Bool
	a, err := New(Config{Target: "reference", Secret: []byte("s"), Features: []string{"freshness"}, Down: down.Load})
	if err != nil {
		t.Fatal(err)
	}
	write := func() { a.mu.Lock(); a.record("write"); a.mu.Unlock() }
	down.Store(true)
	a.freshSubjects(true) // failed
	down.Store(false)
	write()
	lead := make(chan struct{})
	go func() { a.freshSubjects(true); close(lead) }()
	time.Sleep(readDwell / 3)
	joined := make(chan reply)
	go func() { joined <- a.freshSubjects(true) }()
	time.Sleep(readDwell / 3)
	write() // forgets the read both are in
	rep := <-joined
	<-lead
	var doc map[string]any
	_ = json.Unmarshal(rep.body, &doc)
	if rep.status != 200 || doc["current"] != true {
		t.Errorf("a read whose join a write forgot: %d %v, want a read of its own, current", rep.status, doc)
	}
}
