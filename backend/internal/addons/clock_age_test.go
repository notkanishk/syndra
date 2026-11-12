package addons

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// A kept copy is dated by its age on the add-on's own clock (RFC 9111 §4.2.3),
// so the skew between the add-on's clock and this one cancels out.
func TestACopyIsDatedByItsAgeNotByTheAddonsClock(t *testing.T) {
	now := time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)
	withClock(t, now)
	t.Cleanup(func() { aheadLogged.Delete("truenas") })

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
	// A Date before taken_at is a bogus add-on: age 0, never in the future.
	srvDate := now.Add(10 * time.Minute).Add(-time.Hour).Format(http.TimeFormat)
	if got := age(serve(10*time.Minute, 3*time.Minute, srvDate)); got != 0 {
		t.Errorf("Date before taken_at: copy aged %v, want 0", got)
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
