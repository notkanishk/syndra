package addons

import (
	"context"
	"errors"
	"sync"
	"time"
)

// offlineThreshold is how many consecutive failed health reads it takes to call
// a target offline. Three, at the probe's 30s interval: one slow answer is
// ordinary, and a page that said "offline" on it would tell a member their files
// are out of reach when they are not.
const offlineThreshold = 3

// HealthStatus is what Syndra can honestly say about a target's reachability.
type HealthStatus string

const (
	HealthOnline  HealthStatus = "online"
	HealthOffline HealthStatus = "offline"
	// HealthUnknown is "can't tell": no answer seen yet, or fewer failures than
	// it takes to be sure. Pages show nothing rather than guess.
	HealthUnknown HealthStatus = "unknown"
)

// HealthState is the monitor's account of one target.
type HealthState struct {
	Status HealthStatus `json:"status"`
	// CheckedAt is the latest health read, zero when none has happened.
	CheckedAt time.Time `json:"checked_at"`
	// ReachableSince is when the current run of failures began: the time the
	// target stopped answering. Set only while it is offline.
	ReachableSince *time.Time `json:"reachable_since,omitempty"`
	// Lifecycle is the add-on's own lifecycle from its last answer, empty when it
	// has not answered.
	Lifecycle string `json:"lifecycle,omitempty"`
}

type healthRecord struct {
	checkedAt     time.Time
	failures      int
	firstFailedAt time.Time
	everSucceeded bool
	lastLifecycle string
}

// healthMonitor remembers health reads so "offline" means confirmed.
//
// ponytail: in memory. A restart forgets an outage in progress, and the pages
// say "can't tell" until the next probes (≤ 90s) rather than guess. A table is
// the upgrade if restarts during outages turn out to matter.
type healthMonitor struct {
	mu      sync.Mutex
	records map[string]*healthRecord
}

func newHealthMonitor() *healthMonitor {
	return &healthMonitor{records: map[string]*healthRecord{}}
}

// monitor is fed by every Health read, wherever it was made from, so the
// member's page, the operator's page and the periodic probe share one count.
var monitor = newHealthMonitor()

func (m *healthMonitor) observe(target string, reachable bool, lifecycle string, at time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.records[target]
	if r == nil {
		r = &healthRecord{}
		m.records[target] = r
	}
	r.checkedAt = at
	if reachable {
		r.failures, r.firstFailedAt, r.everSucceeded = 0, time.Time{}, true
		r.lastLifecycle = lifecycle
		return
	}
	if r.failures == 0 {
		r.firstFailedAt = at
	}
	r.failures++
}

func (m *healthMonitor) state(target string) HealthState {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.records[target]
	if r == nil {
		return HealthState{Status: HealthUnknown}
	}
	st := HealthState{CheckedAt: r.checkedAt, Lifecycle: r.lastLifecycle}
	switch {
	case r.failures >= offlineThreshold:
		since := r.firstFailedAt
		st.Status, st.ReachableSince = HealthOffline, &since
	case r.everSucceeded:
		st.Status = HealthOnline
	default:
		st.Status = HealthUnknown
	}
	return st
}

// HealthOf is what Syndra can say about a target's reachability right now.
func HealthOf(target string) HealthState { return monitor.state(target) }

// observeHealth records one Health read. A read the backend's own breaker
// refused says nothing about the target, so it is not counted either way.
func observeHealth(target string, h TargetHealth) {
	if errors.Is(h.Err, ErrCircuitOpen) {
		return
	}
	monitor.observe(target, h.Outcome == OutcomeSucceeded && h.Reachable, h.Lifecycle, timeNow().UTC())
}

// ProbeAll reads every registered target's health once; the periodic runner
// calls it so the count keeps moving when nobody has a page open.
func ProbeAll(ctx context.Context) error {
	for _, reg := range Registered() {
		Health(ctx, reg.Target)
	}
	return nil
}
