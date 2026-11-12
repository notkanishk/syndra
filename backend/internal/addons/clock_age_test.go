package addons

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// A kept copy is dated by its age on the add-on's own clock (RFC 9111 §4.2.3),
// so the skew between the add-on's clock and this one cancels out.
func TestACopyIsDatedByItsAgeNotByTheAddonsClock(t *testing.T) {
	now := time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)
	withClock(t, now)
	forgetLogged(t)

	// serve answers /reach, a copy taken copyAge before the add-on's own clock,
	// which runs skew ahead of (or, negative, behind) this backend's. date, when
	// set, replaces the Date header ("-" sends none).
	serve := func(skew, copyAge time.Duration, date string) Reach {
		t.Helper()
		addonClock := now.Add(skew)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header()["Date"] = []string{addonClock.Format(http.TimeFormat)}
			switch date {
			case "":
			case "-":
				w.Header()["Date"] = nil
			default:
				w.Header()["Date"] = []string{date}
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"taken_at":%q,"current":false,"stale":{"reason":"cooldown"},"groups":{},"everyone":[]}`,
				addonClock.Add(-copyAge).Format(time.RFC3339))
		}))
		t.Cleanup(srv.Close)
		signedAddon(t, srv.URL, "a-test-secret")
		return ReadReach(context.Background(), "truenas", false)
	}
	age := func(r Reach) time.Duration {
		t.Helper()
		if !r.Read {
			t.Fatalf("not read: %+v", r)
		}
		return now.Sub(r.ReadAt)
	}

	// The add-on's clock 10 min ahead, the copy 3 min old by it: was "just now".
	if got := age(serve(10*time.Minute, 3*time.Minute, "")); got != 3*time.Minute {
		t.Errorf("clock 10m ahead: copy aged %v, want 3m0s", got)
	}
	// The add-on's clock behind: the same age (was 7m old, as sent).
	if got := age(serve(-4*time.Minute, 3*time.Minute, "")); got != 3*time.Minute {
		t.Errorf("clock 4m behind: copy aged %v, want 3m0s", got)
	}
	// No Date: taken_at as sent, the clamp as before (10 min ahead: now).
	if got := age(serve(10*time.Minute, 3*time.Minute, "-")); got != 0 {
		t.Errorf("no Date: copy aged %v, want the clamp's 0", got)
	}
	// ... and a skew inside clockSkew is as sent.
	if got := age(serve(time.Minute, 3*time.Minute, "-")); got != 2*time.Minute {
		t.Errorf("no Date, 1m ahead: copy aged %v, want 2m0s as sent", got)
	}
	// A Date that is not a date says nothing: as sent.
	if got := age(serve(-4*time.Minute, 3*time.Minute, "yesterday")); got != 7*time.Minute {
		t.Errorf("unparseable Date: copy aged %v, want 7m0s as sent", got)
	}
	// A Date more than a second before taken_at is another clock's (a proxy, a
	// bogus add-on): ignored, taken_at as sent, said once.
	before := now.Add(-3 * time.Minute).Add(-time.Hour).Format(http.TimeFormat)
	if got := age(serve(0, 3*time.Minute, before)); got != 3*time.Minute {
		t.Errorf("Date an hour before taken_at: copy aged %v, want 3m0s as sent", got)
	}
	// ... and within a second (Date has one-second resolution) it is age 0.
	if got := age(serve(0, 0, now.Add(-500*time.Millisecond).Format(http.TimeFormat))); got != 0 {
		t.Errorf("Date within a second of taken_at: copy aged %v, want 0", got)
	}
}

// A live read is unaffected: taken_at is its Date, age 0, so it is dated when
// this backend asked, on a clock 10 min ahead as much as on a true one.
func TestALiveReadIsDatedWhenItWasAsked(t *testing.T) {
	now := time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)
	withClock(t, now)
	for _, skew := range []time.Duration{0, 10 * time.Minute, -10 * time.Minute} {
		addonClock := now.Add(skew)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header()["Date"] = []string{addonClock.Format(http.TimeFormat)}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"taken_at":%q,"current":true,"groups":{},"everyone":[]}`, addonClock.Format(time.RFC3339))
		}))
		t.Cleanup(srv.Close)
		signedAddon(t, srv.URL, "a-test-secret")
		if r := ReadReach(context.Background(), "truenas", false); !r.Read || !r.ReadAt.Equal(now) {
			t.Errorf("skew %v: a live read dated %v, want now %v", skew, r.ReadAt, now)
		}
	}
}

// Every result shape is dated the same way, through ReadReply.decodeShape.
func TestAResultShapeIsDatedByItsAgeToo(t *testing.T) {
	now := time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)
	withClock(t, now)
	r := ReadReply{Target: "shapes", Schema: "common/records.json", Results: true, Outcome: OutcomeSucceeded,
		Clock: Clock{Date: now.Add(10 * time.Minute), Sent: now},
		Body:  []byte(`{"result":{"records":[],"taken_at":"2026-11-10T09:07:00Z","current":true}}`)}
	v := r.View()
	if !v.Readable {
		t.Fatalf("not readable: %+v", v)
	}
	if got := v.Result.(observed).observation().TakenAt; !got.Equal(now.Add(-3 * time.Minute)) {
		t.Errorf("records copy dated %v, want 3 min before now", got)
	}
}

// serveAs installs the truenas add-on answering body with a Date from
// addonClock; onRequest runs first (a test moves the faked clock there).
func serveAs(t *testing.T, addonClock func() time.Time, body string, onRequest func()) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if onRequest != nil {
			onRequest()
		}
		w.Header()["Date"] = []string{addonClock().UTC().Format(http.TimeFormat)}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, body)
	}))
	t.Cleanup(srv.Close)
	signedAddon(t, srv.URL, "a-test-secret")
}

// The copy dates from when the request was sent (RFC 9111's request_time), not
// from when it was read: the backend's clock moves while the add-on answers.
func TestACopyDatesFromWhenTheRequestWasSent(t *testing.T) {
	start := time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)
	c := withTestClock(t, start)
	addonClock := func() time.Time { return start.Add(10 * time.Minute) }
	serveAs(t, addonClock, `{"taken_at":"`+start.Add(7*time.Minute).Format(time.RFC3339)+
		`","current":false,"stale":{"reason":"cooldown"},"groups":{},"everyone":[]}`,
		func() { c.advance(5 * time.Second) })
	r := ReadReach(context.Background(), "truenas", false)
	if want := start.Add(-3 * time.Minute); !r.Read || !r.ReadAt.Equal(want) {
		t.Errorf("copy dated %v, want %v (sent %v less its 3 min age)", r.ReadAt, want, start)
	}
}

// A physical operation's recorded reply (GET /calls) is dated by the /calls
// answer's Date, and ViewOf carries the clock into the renderer.
func TestARecordedReplyIsDatedByTheCallsAnswer(t *testing.T) {
	now := time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)
	withClock(t, now)
	const id = "0b7d4c1e-6a5f-4e2b-9c3d-8f1a2e4b6c70"
	reply := `{"operation":"item.checkout","outcome":"succeeded","result":{"records":[],"taken_at":"` +
		now.Add(7*time.Minute).Format(time.RFC3339) + `","current":true}}`
	inventoryAddon(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header()["Date"] = []string{now.Add(10 * time.Minute).Format(http.TimeFormat)}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"call_id":%q,"state":"recorded","reply":%s}`, id, reply)
	})
	s, err := CallAnswer(context.Background(), "inventory", "item.checkout", id)
	if err != nil || s.Clock.Date.IsZero() {
		t.Fatalf("%+v %v: the /calls answer's clock was dropped", s, err)
	}
	v := ViewOf("inventory", "item.checkout", s.Reply, s.Clock)
	if !v.Readable {
		t.Fatalf("not readable: %+v", v)
	}
	if got := v.Result.(observed).observation().TakenAt; !got.Equal(s.Clock.Sent.Add(-3 * time.Minute)) {
		t.Errorf("recorded reply dated %v, want 3 min before the request", got)
	}
}

// /stats and the health timestamps are dated through the same function.
func TestStatsAndHealthAreDatedByAge(t *testing.T) {
	now := time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)
	withClock(t, now)
	declare := func(feature string) { t.Helper(); declare(t, feature) }

	var stats statsWire
	if err := json.Unmarshal(contractFixture(t, "stats.json"), &stats); err != nil {
		t.Fatal(err)
	}
	serveAs(t, func() time.Time { return stats.TakenAt.Add(3 * time.Minute) }, string(contractFixture(t, "stats.json")), nil)
	declare("stats")
	page, err := ReadStats(context.Background(), "truenas", "laser.job_seconds", now, "")
	if err != nil {
		t.Fatal(err)
	}
	if want := now.Add(-3 * time.Minute); !page.TakenAt.Equal(want) {
		t.Errorf("stats taken_at %v, want %v (3 min old by the add-on's clock)", page.TakenAt, want)
	}

	// Health: the mirror's date and the last read are the add-on's clock too.
	serveAs(t, func() time.Time { return time.Date(2026, 11, 10, 9, 29, 41, 0, time.UTC).Add(10 * time.Minute) },
		string(contractFixture(t, "health.json")), nil)
	h := readHealth(context.Background(), "truenas")
	// last_read_at 09:29:41, Date 09:39:41: ten minutes old.
	if want := now.Add(-10 * time.Minute).Format(time.RFC3339); h.LastReadAt != want {
		t.Errorf("last_read_at %q, want %q", h.LastReadAt, want)
	}
	if want := now.Add(-14*time.Minute - 41*time.Second).Format(time.RFC3339); h.SnapshotTakenAt != want {
		t.Errorf("snapshot_taken_at %q, want %q", h.SnapshotTakenAt, want)
	}
}

// A clock far from this one, and a Date that is not taken_at's clock, are said
// once per target, so "check the add-on host's clock" survives the correction.
func TestAClockOffAndABogusDateAreSaidOnce(t *testing.T) {
	now := time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)
	var logs bytes.Buffer
	w := log.Writer()
	log.SetOutput(&logs)
	forgetLogged(t)
	t.Cleanup(func() { log.SetOutput(w) })
	c := Clock{Date: now.Add(10 * time.Minute), Sent: now}
	for range 3 {
		ahead("skewy", now.Add(7*time.Minute), c)
	}
	if n := strings.Count(logs.String(), "check the add-on host's clock"); n != 1 || !strings.Contains(logs.String(), "10m0s") {
		t.Errorf("a clock 10 min off was said %d times:\n%s", n, logs.String())
	}
	logs.Reset()
	for range 3 {
		ahead("skewy", now, Clock{Date: now.Add(-time.Hour), Sent: now})
	}
	if n := strings.Count(logs.String(), "the Date is ignored"); n != 1 {
		t.Errorf("a bogus Date was said %d times:\n%s", n, logs.String())
	}
}
