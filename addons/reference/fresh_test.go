package reference

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
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
