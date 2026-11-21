package addons

import (
	"context"
	"errors"
	"testing"
	"time"
)

// While the monitor holds the add-on unreachable, every read answers no copy
// with the monitor's why, and nothing is dialed (design §13.2); the probe
// alone still dials, so it is what finds the add-on back.
func TestAHeldAddonIsAnsweredWithoutDialing(t *testing.T) {
	m := monitor
	monitor = newHealthMonitor()
	t.Cleanup(func() { monitor = m })
	got := addonAnswering(t, "reach.json")

	t0 := time.Now().UTC()
	for i := range failureThreshold {
		monitor.observe("truenas", TargetHealth{Outcome: OutcomeUnreached, Why: Why{Reason: ReasonAddonUnreachable, Cause: CauseTimeout}}, t0.Add(time.Duration(i)*time.Second))
	}
	r := ReadReach(context.Background(), "truenas", false)
	if got.path != "" {
		t.Fatalf("a held add-on was dialed: %s %s", got.method, got.path)
	}
	if r.Read.State != ReadNone || r.Read.Reason != ReasonAddonUnreachable || r.Read.Cause != CauseTimeout || r.Groups != nil {
		t.Fatalf("want no copy with the monitor's why, got %+v", r)
	}
	if s, read := ReadSettings(context.Background(), "truenas"); s != nil || !read.None() || got.path != "" {
		t.Fatalf("settings too: %v %+v, dialed %q", s, read, got.path)
	}

	probe(context.Background(), "truenas")
	if got.path != "/health" {
		t.Fatalf("the probe must still dial, got %q", got.path)
	}
}

// ReadOf is the one writer of `read`: a zero or missing taken_at is no copy,
// current is now, and a stale copy is kept with Syndra's word for its reason.
func TestReadOfDatesOnlyAReading(t *testing.T) {
	at := time.Date(2026, 11, 21, 9, 0, 0, 0, time.UTC)
	retry := 9
	for _, tc := range []struct {
		name       string
		o          *Observation
		state, why string
		retryAfter int
		dated      bool
	}{
		{"nothing", nil, ReadNone, ReasonAddonUnreachable, 0, false},
		{"zero date", &Observation{Current: true}, ReadNone, ReasonAddonUnreachable, 0, false},
		{"now", &Observation{TakenAt: at, Current: true}, ReadNow, "", 0, true},
		{"app down", &Observation{TakenAt: at, Stale: &Stale{Reason: "unreachable"}}, ReadKept, ReasonAppUnreachable, 0, true},
		{"cooldown", &Observation{TakenAt: at, Stale: &Stale{Reason: StaleCooldown, RetryAfter: &retry}}, ReadKept, ReasonCooldown, 9, true},
		{"cached", &Observation{TakenAt: at, Stale: &Stale{Reason: "cached"}}, ReadKept, ReasonCached, 0, true},
	} {
		s := ReadOf(tc.o, Why{})
		if s.State != tc.state || s.Reason != tc.why || s.RetryAfter != tc.retryAfter || (s.TakenAt != nil) != tc.dated {
			t.Errorf("%s: %+v", tc.name, s)
		}
	}
}

// A value the add-on could not be asked about is refused, never saved
// unchecked (design §13.3, owner Q2): ResolvesValue fails closed with the why.
func TestAValueThatCannotBeCheckedIsRefused(t *testing.T) {
	addonServingStatus(t, 503, []byte(`{"type":"urn:syndra:addon:error:TARGET_UNREADABLE","title":"x","status":503,"code":"TARGET_UNREADABLE","retry":"backoff"}`))
	_, err := ResolvesValue(context.Background(), "truenas", "group", "lab_makers")
	var nr NotRead
	if !errors.As(err, &nr) || nr.Why.Reason != ReasonAppUnreachable {
		t.Fatalf("want NotRead app_unreachable, got %v", err)
	}
}
