package addons

import (
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
