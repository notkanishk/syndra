package addons

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Probe results, as observe sees them.
var (
	probeOK     = TargetHealth{Outcome: OutcomeSucceeded, Reachable: true, Lifecycle: "active"}
	probeAppOff = TargetHealth{Outcome: OutcomeSucceeded, Reachable: false}
	probeNoAns  = TargetHealth{Outcome: OutcomeUnreached, Why: Why{Reason: ReasonAddonUnreachable, Cause: CauseNoAnswer}}
)

func observeSeq(m *healthMonitor, target string, start time.Time, probes ...TargetHealth) {
	for i, h := range probes {
		m.observe(target, h, start.Add(time.Duration(i)*30*time.Second))
	}
}

var t0 = time.Date(2026, 10, 14, 9, 40, 0, 0, time.UTC)

// Design §13.2, decision 28.5: three missed probes make it unreachable (about
// 90 s), one success recovers it.
func TestThreeMissesAreUnreachableOneSuccessRecovers(t *testing.T) {
	m := newHealthMonitor()
	observeSeq(m, "truenas", t0, probeOK, probeNoAns, probeNoAns)
	if st := m.state("truenas", t0); st.State != StateAnswering {
		t.Fatalf("two misses are not an outage, got %+v", st)
	}
	m.observe("truenas", probeNoAns, t0.Add(90*time.Second))
	st := m.state("truenas", t0.Add(90*time.Second))
	if st.State != StateAddonUnreachable || st.Cause != CauseNoAnswer {
		t.Fatalf("the third miss is addon_unreachable no_answer, got %+v", st)
	}
	if st.Since == nil || !st.Since.Equal(t0.Add(30*time.Second)) {
		t.Fatalf("unreachable since the FIRST miss of the run, got %v", st.Since)
	}
	if st.RetryAfter != 30 {
		t.Fatalf("retry_after is the next probe, got %d", st.RetryAfter)
	}
	m.observe("truenas", probeOK, t0.Add(2*time.Minute))
	st = m.state("truenas", t0.Add(2*time.Minute))
	if st.State != StateAnswering || st.Cause != "" || st.RetryAfter != 0 {
		t.Fatalf("one success recovers, got %+v", st)
	}
	if st.Since == nil || !st.Since.Equal(t0.Add(2*time.Minute)) {
		t.Fatalf("answering since the recovery, got %v", st.Since)
	}
}

// From checking the first probe decides either way, and since is absent: the
// state was already so when Syndra first looked, so when it began is unknown.
func TestTheFirstProbeDecidesFromChecking(t *testing.T) {
	m := newHealthMonitor()
	if st := m.state("truenas", t0); st.State != StateChecking {
		t.Fatalf("never probed is checking, got %+v", st)
	}
	m.observe("truenas", probeNoAns, t0)
	if st := m.state("truenas", t0); st.State != StateAddonUnreachable || st.Since != nil {
		t.Fatalf("one miss from checking is unreachable, since not observed: %+v", st)
	}
	m2 := newHealthMonitor()
	m2.observe("truenas", probeOK, t0)
	if st := m2.state("truenas", t0); st.State != StateAnswering || st.Since != nil {
		t.Fatalf("one answer from checking is answering, since not observed: %+v", st)
	}
}

// The two outages are named apart: the add-on answering reachable false is
// app_unreachable, after the same three misses.
func TestAnAnswerSayingTheAppIsDownIsAppUnreachable(t *testing.T) {
	m := newHealthMonitor()
	observeSeq(m, "truenas", t0, probeOK, probeAppOff, probeAppOff, probeAppOff)
	st := m.state("truenas", t0)
	if st.State != StateAppUnreachable || st.Cause != "" {
		t.Fatalf("want app_unreachable with no cause, got %+v", st)
	}
	// The add-on going too while the app is down switches the class at once:
	// it is already an outage, and the operator's next step changed.
	m.observe("truenas", probeNoAns, t0.Add(2*time.Minute))
	if st := m.state("truenas", t0); st.State != StateAddonUnreachable || st.Cause != CauseNoAnswer {
		t.Fatalf("want addon_unreachable no_answer, got %+v", st)
	}
}

// An open breaker is reported as the cause that opened it, with its time left.
func TestAProbeTheBreakerRefusedCarriesTheOpeningCause(t *testing.T) {
	m := newHealthMonitor()
	open := CallResponse{Outcome: OutcomeUnreached,
		Err: &CircuitOpenError{Target: "truenas", Opener: Why{Reason: ReasonAddonUnreachable, Cause: CauseWrongHost}, RetryAfter: 50 * time.Second}}
	h := TargetHealth{Outcome: OutcomeUnreached, Err: open.Err, Why: Unread(open)}
	m.observe("truenas", h, t0)
	st := m.state("truenas", t0)
	if st.State != StateAddonUnreachable || st.Cause != CauseWrongHost || st.RetryAfter != 50 {
		t.Fatalf("want wrong_host, retry in 50 s, got %+v", st)
	}
}

// A-01: the last three checks, and when the run of misses began.
func TestTheMonitorRemembersTheLastThreeChecks(t *testing.T) {
	m := newHealthMonitor()
	observeSeq(m, "truenas", t0, probeOK, probeOK, probeOK, probeNoAns)
	st := m.state("truenas", t0)
	if got := strings.Join(st.Recent, ","); got != "answered,answered,missed" {
		t.Fatalf("want the last three, oldest first, got %s", got)
	}
	if st.MissedSince == nil || !st.MissedSince.Equal(t0.Add(90*time.Second)) {
		t.Fatalf("one miss carries when it began, got %+v", st)
	}
	m.observe("truenas", probeOK, t0.Add(2*time.Minute))
	if st := m.state("truenas", t0); st.MissedSince != nil {
		t.Fatalf("an answer ends the run, got %+v", st)
	}
	if st := newHealthMonitor().state("x", t0); st.StartedAt.IsZero() || st.Recent == nil {
		t.Fatalf("never checked still says when Syndra started, with an empty list: %+v", st)
	}
}

// "Last answered" is the last SUCCESSFUL probe; null when none.
func TestLastAnsweredAtStaysPutAcrossAMiss(t *testing.T) {
	m := newHealthMonitor()
	observeSeq(m, "truenas", t0, probeNoAns)
	if st := m.state("truenas", t0); st.LastAnsweredAt != nil {
		t.Fatalf("a miss is not an answer, got %v", st.LastAnsweredAt)
	}
	observeSeq(m, "truenas", t0.Add(time.Minute), probeOK, probeNoAns, probeNoAns)
	st := m.state("truenas", t0)
	if st.LastAnsweredAt == nil || !st.LastAnsweredAt.Equal(t0.Add(time.Minute)) {
		t.Fatalf("last_answered_at is the answered check, got %v", st.LastAnsweredAt)
	}
	if !st.CheckedAt.Equal(t0.Add(2 * time.Minute)) {
		t.Fatalf("checked_at moves with each check, got %v", st.CheckedAt)
	}
	b, _ := json.Marshal(m.state("never", t0))
	if !strings.Contains(string(b), `"last_answered_at":null`) || !strings.Contains(string(b), `"state":"checking"`) {
		t.Fatalf("null on the wire when never answered, got %s", b)
	}
}

// A zero time on the wire is a real-looking year-1 moment to a screen.
func TestZeroTimesAreOmittedFromTheWire(t *testing.T) {
	b, _ := json.Marshal(HealthState{State: StateChecking})
	for _, k := range []string{"checked_at", "started_at", "since", "retry_after", "cause"} {
		if strings.Contains(string(b), k) {
			t.Fatalf("%s must be omitted when unset, got %s", k, b)
		}
	}
}

// Only the monitor's own probe counts (Kubernetes: only the kubelet's periodic
// probe counts toward failureThreshold). Pages and "Check now" read Health, and
// however many of them there are, the monitor does not move; one probe does.
func TestOnlyTheProbeMovesTheMonitor(t *testing.T) {
	m := monitor
	monitor = newHealthMonitor()
	t.Cleanup(func() { monitor = m })
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close() // a closed port
	signedAddon(t, url, "a-poll-secret")
	for range failureThreshold + 1 {
		Health(context.Background(), "truenas")
	}
	if st := HealthOf("truenas"); st.State != StateChecking || len(st.Recent) != 0 {
		t.Fatalf("live reads moved the monitor: %+v", st)
	}
	_ = ProbeAll(context.Background())
	if st := HealthOf("truenas"); st.State != StateAddonUnreachable || LastHealth("truenas").Why.Cause != CauseNoAnswer {
		t.Fatalf("the probe decides from checking and keeps its answer: %+v %+v", st, LastHealth("truenas"))
	}
}

// A target not registered is never probed, so its last health says why, as a
// live read does, rather than an unreached answer with no detail.
func TestLastHealthOfAnUnregisteredTargetSaysItIsNotRegistered(t *testing.T) {
	if h := LastHealth("no-such-target"); h.Outcome != OutcomeUnreached || !errors.Is(h.Err, ErrNotRegistered) {
		t.Fatalf("LastHealth = %+v, want unreached with ErrNotRegistered", h)
	}
}
