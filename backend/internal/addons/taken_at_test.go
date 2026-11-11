package addons

import (
	"bytes"
	"context"
	"log"
	"strings"
	"testing"
	"time"
)

// A taken_at later than now, beyond the clock skew allowed, is read as now
// and said once per target; one within the skew, or long past, is as sent.
func TestATakenAtFromTheFutureIsReadAsNow(t *testing.T) {
	now := time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)
	saved := timeNow
	timeNow = func() time.Time { return now }
	t.Cleanup(func() { timeNow = saved })
	var logs bytes.Buffer
	w := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(w) })
	t.Cleanup(func() { aheadLogged.Delete("ahead-a"); aheadLogged.Delete("ahead-b") })

	read := func(target, raw string) time.Time {
		t.Helper()
		at, ok := takenAt(target, &raw)
		if !ok {
			t.Fatalf("%s: not read", raw)
		}
		return at
	}
	for raw, want := range map[string]time.Time{
		"2026-11-10T10:00:00Z":      now,                                         // an hour ahead
		"2027-01-01T00:00:00Z":      now,                                         // months ahead
		"2026-11-10T09:01:59Z":      now.Add(119 * time.Second),                  // inside the skew: as sent
		"2026-11-10T09:00:00+01:00": now.Add(-time.Hour),                         // an offset, read in UTC
		"2001-01-01T00:00:00Z":      time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC), // far past: as sent
	} {
		if got := read("ahead-a", raw); !got.Equal(want) {
			t.Errorf("%s read %v, want %v", raw, got, want)
		}
	}
	read("ahead-b", "2026-11-10T09:30:00Z")
	read("ahead-b", "2026-11-10T09:30:00Z")
	if n := strings.Count(logs.String(), "ahead of this clock"); n != 2 {
		t.Errorf("logged %d times for two targets, each ahead more than once, want once per target:\n%s", n, logs.String())
	}
	if !strings.Contains(logs.String(), "ahead-b dates a read 30m0s ahead") {
		t.Errorf("the log does not say whose clock or by how much:\n%s", logs.String())
	}
}

// Every read that dates itself reads its taken_at through takenAt: /reach and
// /homes as much as /subjects.
func TestEveryDatedReadIsClampedAlike(t *testing.T) {
	now := time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)
	saved := timeNow
	timeNow = func() time.Time { return now }
	t.Cleanup(func() { timeNow = saved; aheadLogged.Delete("truenas") })
	addonServing(t, []byte(`{"taken_at":"2099-01-01T00:00:00Z","current":true,"groups":{},"everyone":[]}`))
	if r := ReadReach(context.Background(), "truenas", false); !r.Read || !r.ReadAt.Equal(now) {
		t.Errorf("reach dated 2099 read %+v, want read at now", r)
	}
	addonServing(t, []byte(`{"taken_at":"2099-01-01T00:00:00Z","current":true,"homes":[],"quota_read":true}`))
	if h := ReadHomes(context.Background(), "truenas", false); !h.Read || !h.Observation.TakenAt.Equal(now) {
		t.Errorf("homes dated 2099 read %+v, want taken at now", h)
	}
}
