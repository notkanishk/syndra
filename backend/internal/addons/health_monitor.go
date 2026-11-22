package addons

import (
	"context"
	"log"
	"math"
	"sync"
	"time"
)

// The health monitor is the one owner of "reachable" (design §13.2): the drain,
// the member view, the roster and the pages all read its State and nothing
// else computes it.
//
// failureThreshold is a Kubernetes probe's default (about 90 s at the 30 s
// probe); its successThreshold default, 1, is one answer clearing an outage.
// From checking, one answer decides, but an outage takes the same three misses
// as from answering: a restart that probes an add-on not yet listening, or a
// NAS busy for one probe, is not an outage, and checking is the honest word
// until it is one (task #89).
const failureThreshold = 3

// ReachState is what the monitor can say of one target.
type ReachState string

const (
	// StateChecking: no probe answered yet, and fewer than failureThreshold
	// missed in a row. The first ProbeAll runs before the listener serves, so
	// an add-on that answers it is never seen here.
	StateChecking ReachState = "checking"
	// StateAnswering: the add-on answered its last probe with reachable true
	// (or has missed fewer than failureThreshold in a row since).
	StateAnswering ReachState = "answering"
	// StateAddonUnreachable: Syndra holds no usable answer from the add-on.
	StateAddonUnreachable ReachState = ReasonAddonUnreachable
	// StateAppUnreachable: the add-on answers and says it cannot read its app.
	StateAppUnreachable ReachState = ReasonAppUnreachable
)

// HealthState is the monitor's account of one target: design §13.2's
// {state, cause, since, checked_at, retry_after}, and A-01's check pips.
type HealthState struct {
	State ReachState `json:"state"`
	// Cause is addon_unreachable's (Unread): no_answer, wrong_host, timeout,
	// fault. Empty otherwise.
	Cause string `json:"cause,omitempty"`
	// Since is when the current state began, absent when it was already so at
	// the first probe: not observed, so not stated.
	Since *time.Time `json:"since,omitempty"`
	// CheckedAt is the latest probe, zero when none has run.
	CheckedAt time.Time `json:"checked_at,omitzero"`
	// RetryAfter is seconds until Syndra tries again, while unreachable: the
	// breaker's time left or the next probe, whichever is later.
	RetryAfter int `json:"retry_after,omitempty"`
	// LastAnsweredAt is the last probe answered with reachable true; null when
	// none has been since Syndra started.
	LastAnsweredAt *time.Time `json:"last_answered_at"`
	// Lifecycle is the add-on's own lifecycle from its last answer.
	Lifecycle string `json:"lifecycle,omitempty"`
	// Recent is the last three checks, oldest first: "answered" or "missed".
	Recent []string `json:"recent"`
	// MissedSince is the first miss of the current run, before it is an outage
	// ("Missed 1 check, 09:40").
	MissedSince *time.Time `json:"missed_since,omitempty"`
	// StartedAt is when this Syndra process started.
	StartedAt time.Time `json:"started_at,omitzero"`
	// LastMiss is the class of the last probe when it missed (missOf), empty
	// when none has. It is never sent: the drain's halt reason reads it.
	LastMiss ReachState `json:"-"`
}

// Answering says the target is reachable: the one test every reader makes.
func (s HealthState) Answering() bool { return s.State == StateAnswering }

type healthRecord struct {
	state       ReachState
	cause       string
	since       time.Time // zero: not observed
	checkedAt   time.Time
	retryAt     time.Time
	answeredAt  time.Time
	misses      int
	firstMissAt time.Time
	lifecycle   string
	recent      []string
	last        TargetHealth // the last probe's answer (LastHealth)
}

// healthMonitor holds every target's state in memory. Nothing is persisted: a
// restart probes once before it serves (ProbeAll in main), so it does not
// forget for longer than one probe.
type healthMonitor struct {
	mu        sync.Mutex
	records   map[string]*healthRecord
	startedAt time.Time
}

func newHealthMonitor() *healthMonitor {
	return &healthMonitor{records: map[string]*healthRecord{}, startedAt: time.Now().UTC()}
}

// monitor is fed by the probe (probe, from ProbeAll) and by one write:
// SetLifecycle's answered switch (setLifecycle), so the switch is in force at
// once rather than after the next probe.
var monitor = newHealthMonitor()

// observe folds one probe into the target's state. A probe answered with
// reachable true is a success; answered with false is app_unreachable; any
// other is addon_unreachable with Unread's cause (an open breaker's is the
// cause that opened it).
func (m *healthMonitor) observe(target string, h TargetHealth, at time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.records[target]
	if r == nil {
		r = &healthRecord{state: StateChecking}
		m.records[target] = r
	}
	r.checkedAt, r.last = at, h
	ok := h.Outcome == OutcomeSucceeded && h.Reachable
	result := "missed"
	if ok {
		result = "answered"
	}
	r.recent = append(r.recent, result)
	if len(r.recent) > failureThreshold {
		r.recent = r.recent[len(r.recent)-failureThreshold:]
	}

	next, cause := StateAnswering, ""
	if ok {
		r.misses, r.firstMissAt, r.answeredAt, r.lifecycle = 0, time.Time{}, at, h.Lifecycle
	} else {
		if r.misses == 0 {
			r.firstMissAt = at
		}
		r.misses++
		next, cause = missOf(h)
		r.retryAt = at.Add(max(probeInterval, time.Duration(h.Why.RetryAfter)*time.Second))
		if (r.state == StateAnswering || r.state == StateChecking) && r.misses < failureThreshold {
			next, cause = r.state, ""
		}
	}
	if next == r.state && cause == r.cause {
		return
	}
	switch {
	case r.state == StateChecking:
		r.since = time.Time{}
	case r.state == StateAnswering:
		r.since = r.firstMissAt
	case next == StateAnswering, next != r.state:
		r.since = at
	}
	errorType := cause
	if errorType == "" && next != StateAnswering {
		errorType = string(next)
	}
	// One line per change, OpenTelemetry's error.type the cause.
	log.Printf("[ADDON-HEALTH] %s: %s -> %s error.type=%s", target, r.state, next, errorType)
	r.state, r.cause = next, cause
}

// setLifecycle records a lifecycle the add-on has just accepted. Without it a
// Send right after "Make active" still halted on the old state, for up to one
// probe interval. Reach is the probe's alone, so only the lifecycle moves, and
// a target never probed has nothing to correct.
func (m *healthMonitor) setLifecycle(target, state string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r := m.records[target]; r != nil {
		r.lifecycle, r.last.Lifecycle = state, state
	}
}

// missOf is a failed probe's state and cause.
func missOf(h TargetHealth) (ReachState, string) {
	switch {
	case h.Outcome == OutcomeSucceeded, h.Why.Reason == ReasonAppUnreachable:
		return StateAppUnreachable, ""
	case h.Why.Cause != "":
		return StateAddonUnreachable, h.Why.Cause
	}
	return StateAddonUnreachable, CauseFault
}

func (m *healthMonitor) state(target string, now time.Time) HealthState {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.records[target]
	if r == nil {
		return HealthState{State: StateChecking, Recent: []string{}, StartedAt: m.startedAt}
	}
	st := HealthState{State: r.state, Cause: r.cause, CheckedAt: r.checkedAt, Lifecycle: r.lifecycle,
		StartedAt: m.startedAt, Recent: append([]string{}, r.recent...)}
	if !r.since.IsZero() {
		since := r.since
		st.Since = &since
	}
	if !r.answeredAt.IsZero() {
		answered := r.answeredAt
		st.LastAnsweredAt = &answered
	}
	if r.misses > 0 {
		missed := r.firstMissAt
		st.MissedSince = &missed
		st.LastMiss, _ = missOf(r.last)
	}
	if !st.Answering() && r.retryAt.After(now) {
		st.RetryAfter = int(math.Ceil(r.retryAt.Sub(now).Seconds()))
	}
	return st
}

func (m *healthMonitor) last(target string) TargetHealth {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r := m.records[target]; r != nil {
		return r.last
	}
	return TargetHealth{Outcome: OutcomeUnreached}
}

// HealthOf is the monitor's State of a target (design §13.2).
func HealthOf(target string) HealthState { return monitor.state(target, timeNow().UTC()) }

// ProbeAll reads every registered target's health once, in parallel, each
// bounded by probeTimeout: one dead add-on costs its own 5 s, not everyone's.
// main runs it once before the listener serves, then every probeInterval.
func ProbeAll(ctx context.Context) error {
	var wg sync.WaitGroup
	for _, reg := range Registered() {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(ctx, probeTimeout)
			defer cancel()
			probe(ctx, reg.Target)
		})
	}
	wg.Wait()
	return nil
}

// ProbeInterval is the monitor's period, for main's runner.
func ProbeInterval() time.Duration { return probeInterval }
