package addons

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func observeSeq(m *healthMonitor, target string, start time.Time, ok ...bool) {
	for i, reachable := range ok {
		m.observe(target, reachable, "active", start.Add(time.Duration(i)*30*time.Second))
	}
}

func TestOneSlowAnswerStaysOnline(t *testing.T) {
	m := newHealthMonitor()
	t0 := time.Date(2026, 10, 14, 9, 40, 0, 0, time.UTC)
	observeSeq(m, "truenas", t0, true, false, true)
	st := m.state("truenas")
	if st.Status != HealthOnline || st.ReachableSince != nil {
		t.Fatalf("a recovered blip must leave no trace, got %+v", st)
	}
	observeSeq(m, "truenas", t0.Add(2*time.Minute), false, false)
	if st := m.state("truenas"); st.Status != HealthOnline {
		t.Fatalf("two misses are not an outage, got %+v", st)
	}
}

func TestThreeFailuresAreOfflineSinceTheFirst(t *testing.T) {
	m := newHealthMonitor()
	t0 := time.Date(2026, 10, 14, 9, 40, 0, 0, time.UTC)
	observeSeq(m, "truenas", t0, true, false, false, false)
	st := m.state("truenas")
	if st.Status != HealthOffline {
		t.Fatalf("want offline, got %+v", st)
	}
	if st.ReachableSince == nil || !st.ReachableSince.Equal(t0.Add(30*time.Second)) {
		t.Fatalf("offline since the FIRST failure in the run, got %v", st.ReachableSince)
	}
	if !st.CheckedAt.Equal(t0.Add(90 * time.Second)) {
		t.Fatalf("checked_at is the latest probe, got %v", st.CheckedAt)
	}
	m.observe("truenas", true, "active", t0.Add(2*time.Minute))
	if st := m.state("truenas"); st.Status != HealthOnline || st.ReachableSince != nil {
		t.Fatalf("recovery clears the run, got %+v", st)
	}
}

func TestBeforeAnyAnswerSyndraCannotTell(t *testing.T) {
	m := newHealthMonitor()
	if st := m.state("truenas"); st.Status != HealthUnknown {
		t.Fatalf("never probed is unknown, got %+v", st)
	}
	observeSeq(m, "truenas", time.Now(), false, false)
	if st := m.state("truenas"); st.Status != HealthUnknown {
		t.Fatalf("two failures with no success yet is still unknown, got %+v", st)
	}
	m.observe("truenas", false, "", time.Now())
	if st := m.state("truenas"); st.Status != HealthOffline {
		t.Fatalf("the third confirms it even with no success seen, got %+v", st)
	}
}

// A-01: the operator sees the last three checks and when the run of misses
// began, before it is an outage.
func TestTheMonitorRemembersTheLastThreeChecks(t *testing.T) {
	m := newHealthMonitor()
	t0 := time.Date(2026, 10, 14, 9, 39, 0, 0, time.UTC)
	observeSeq(m, "truenas", t0, true, true, true, false)
	st := m.state("truenas")
	if got := strings.Join(st.Recent, ","); got != "answered,answered,missed" {
		t.Fatalf("want the last three, oldest first, got %s", got)
	}
	if st.Status != HealthOnline || st.MissedSince == nil || !st.MissedSince.Equal(t0.Add(90*time.Second)) {
		t.Fatalf("one miss stays online and carries when it began, got %+v", st)
	}
	m.observe("truenas", true, "active", t0.Add(2*time.Minute))
	if st := m.state("truenas"); st.MissedSince != nil {
		t.Fatalf("an answer ends the run, got %+v", st)
	}
	if st := newHealthMonitor().state("x"); st.StartedAt.IsZero() || st.Recent == nil {
		t.Fatalf("never checked still says when Syndra started, with an empty list: %+v", st)
	}
}

// "Last answered" is the time of the last SUCCESSFUL answer: a later miss moves
// checked_at but not it, and a target that never answered has none (null).
func TestLastAnsweredAtStaysPutAcrossAMiss(t *testing.T) {
	m := newHealthMonitor()
	t0 := time.Date(2026, 10, 14, 9, 40, 0, 0, time.UTC)
	if st := m.state("truenas"); st.LastAnsweredAt != nil {
		t.Fatalf("never probed: no last answer, got %v", st.LastAnsweredAt)
	}
	observeSeq(m, "truenas", t0, false)
	if st := m.state("truenas"); st.LastAnsweredAt != nil {
		t.Fatalf("a miss is not an answer, got %v", st.LastAnsweredAt)
	}
	observeSeq(m, "truenas", t0.Add(time.Minute), true, false, false)
	st := m.state("truenas")
	if st.LastAnsweredAt == nil || !st.LastAnsweredAt.Equal(t0.Add(time.Minute)) {
		t.Fatalf("last_answered_at is the answered check, got %v", st.LastAnsweredAt)
	}
	if !st.CheckedAt.Equal(t0.Add(2 * time.Minute)) {
		t.Fatalf("checked_at still moves with each check, got %v", st.CheckedAt)
	}
	b, _ := json.Marshal(m.state("never"))
	if !strings.Contains(string(b), `"last_answered_at":null`) {
		t.Fatalf("null on the wire when never answered, got %s", b)
	}
}

// A zero time on the wire is a real-looking year-1 moment to a screen. Absent
// is how the page learns nothing was checked.
func TestZeroTimesAreOmittedFromTheWire(t *testing.T) {
	b, _ := json.Marshal(HealthState{Status: HealthUnknown})
	if strings.Contains(string(b), "checked_at") || strings.Contains(string(b), "started_at") {
		t.Fatalf("zero times must be omitted, got %s", b)
	}
	b, _ = json.Marshal(HealthState{Status: HealthOnline, CheckedAt: time.Now(), StartedAt: time.Now()})
	if !strings.Contains(string(b), `"checked_at"`) || !strings.Contains(string(b), `"started_at"`) {
		t.Fatalf("set times stay on the wire, got %s", b)
	}
}
