package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// 4.3, 4.7, 4.10 — the session guard and the per-operation capability probe.

// dialCounter counts logins, which is the thing the breaker exists to bound:
// TrueNAS locks an account out for ten minutes after 20 failed authentications
// in 60 seconds, and every operation this add-on performs shares one session.
type dialCounter struct {
	dials int
	err   error
	rpc   *fakeRPC
}

func (d *dialCounter) dial() (rpc, error) {
	d.dials++
	if d.err != nil {
		return nil, d.err
	}
	return d.rpc, nil
}

// 4.3 — one session, reused. A login per call would reach the rate limit on the
// twentieth read of an ordinary sweep.
func TestTheSessionIsReusedAcrossCalls(t *testing.T) {
	d := &dialCounter{rpc: &fakeRPC{users: `[]`, groups: `[]`, version: "25.04.2"}}
	n := newNAS(d.dial, []string{"25.04"})

	for range 5 {
		if _, err := n.SystemVersion(); err != nil {
			t.Fatal(err)
		}
	}
	if d.dials != 1 {
		t.Fatalf("want one login across five calls, got %d", d.dials)
	}
}

// A burst of failures must cost a bounded number of logins, not one each.
func TestTheReconnectCooldownBoundsLoginAttempts(t *testing.T) {
	d := &dialCounter{err: errors.New("connection refused")}
	n := newNAS(d.dial, []string{"25.04"})
	n.now = func() time.Time { return time.Unix(1_700_000_000, 0) }

	for range 20 {
		_, _ = n.SystemVersion()
	}
	if d.dials != 1 {
		t.Fatalf("twenty calls during an outage must cost one login attempt, got %d", d.dials)
	}
}

// 4.3 — a rate-limit response opens the circuit instead of retrying. Retrying
// into it is how a transient problem becomes a ten-minute lockout for everybody.
func TestARateLimitOpensTheCircuitImmediately(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	d := &dialCounter{err: errors.New("rate limit exceeded")}
	n := newNAS(d.dial, []string{"25.04"})
	n.now = func() time.Time { return now }

	if _, err := n.SystemVersion(); err == nil {
		t.Fatal("the call must fail")
	}
	if !n.CircuitOpen() {
		t.Fatal("a rate limit must open the circuit at once — the next attempt is the one that locks the account")
	}

	// And it stays shut past the cooldown the target imposes, so a breaker that
	// opened because of a lockout has certainly cleared it before allowing again.
	before := d.dials
	now = now.Add(breakerCooldown - time.Minute)
	if _, err := n.SystemVersion(); err == nil {
		t.Fatal("the call must still fail")
	}
	if d.dials != before {
		t.Fatalf("nothing may be dialled while the circuit is open, got %d more", d.dials-before)
	}

	// After it, one attempt is allowed again.
	now = now.Add(2 * time.Minute)
	d.err = nil
	d.rpc = &fakeRPC{version: "25.04.2"}
	if _, err := n.SystemVersion(); err != nil {
		t.Fatalf("the circuit must allow again after the cooldown: %v", err)
	}
	if n.CircuitOpen() {
		t.Fatal("and a success must close it")
	}

	// Closing it means the COUNT is cleared, not merely the deadline passed. A
	// breaker that kept its failures would re-open on the next single failure
	// forever, so one bad minute would leave the target permanently one hiccup
	// from a ten-minute outage.
	n.drop()
	d.err = errors.New("connection refused")
	now = now.Add(reconnectCooldown + time.Second)
	if _, err := n.SystemVersion(); err == nil {
		t.Fatal("the call must fail")
	}
	if n.CircuitOpen() {
		t.Fatal("one failure after a success must not re-open the circuit — the count must have been cleared")
	}
}

// Ordinary failures open it too, just not on the first one.
func TestRepeatedFailuresOpenTheCircuit(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	d := &dialCounter{err: errors.New("connection refused")}
	n := newNAS(d.dial, []string{"25.04"})
	n.now = func() time.Time { return now }

	for range breakerThreshold {
		_, _ = n.SystemVersion()
		// Past the reconnect cooldown, so each call is a real attempt.
		now = now.Add(reconnectCooldown + time.Second)
	}
	if !n.CircuitOpen() {
		t.Fatalf("want the circuit open after %d failures", breakerThreshold)
	}
}

// An operator seeing only "unreachable" looks at the network, when what
// happened is that the add-on backed off to avoid a lockout.
func TestHealthDistinguishesAnOpenCircuitFromAnOutage(t *testing.T) {
	s, _ := applyServer(t, `[]`)
	s.nas.openUntil = time.Now().Add(time.Minute)

	rr := httptest.NewRecorder()
	s.handleHealth(rr, httptest.NewRequest(http.MethodGet, "/health", nil), nil)

	var h Health
	if err := json.Unmarshal(rr.Body.Bytes(), &h); err != nil {
		t.Fatal(err)
	}
	if !h.CircuitOpen {
		t.Fatal("health must say the add-on is refusing its own calls")
	}
}

// 4.7/4.10 — an operation whose method the target lacks is reported unavailable
// with a reason, and refused rather than attempted.
func TestAnOperationWhoseMethodIsAbsentIsUnavailableWithAReason(t *testing.T) {
	s, _ := applyServer(t, `[]`)
	// A release without the audit API.
	s.nas.methods = map[string]bool{
		"user.update": true, "user.delete": true,
		"system.info": true, "alert.list": true, "pool.query": true, "service.query": true,
	}

	byID := map[string]Operation{}
	for _, op := range operationSet(s) {
		byID[op.ID] = op
	}
	if byID["activity.get"].Available {
		t.Fatal("an operation whose method is absent must be unavailable")
	}
	if !strings.Contains(byID["activity.get"].UnavailableReason, "audit.query") {
		t.Errorf("the reason must name the method: %q", byID["activity.get"].UnavailableReason)
	}
	// Omitting it would leave an operator wondering whether the feature exists.
	if _, present := byID["activity.get"]; !present {
		t.Fatal("an unavailable operation must still be declared")
	}
	// The rest are unaffected: this is per operation, not per target version.
	if !byID["password.set"].Available {
		t.Errorf("an unrelated operation must stay available: %q", byID["password.set"].UnavailableReason)
	}
}

// A target that will not enumerate its methods is not a target with none.
// Declaring everything unavailable on one failed read would withdraw the whole
// surface during an outage.
func TestAnUnreadMethodListDoesNotWithdrawTheSurface(t *testing.T) {
	s, _ := applyServer(t, `[]`)
	s.nas.methods = nil

	for _, op := range operationSet(s) {
		if !op.Available {
			t.Errorf("%s must stay available when the method list is unknown: %q", op.ID, op.UnavailableReason)
		}
	}
}

// The two reasons read differently to an operator — "we will not" and "it
// cannot" — and the version check comes first, because on an untested major
// nothing is offered whatever the method list says.
func TestAnUntestedMajorOutranksTheMethodProbe(t *testing.T) {
	s, _ := applyServer(t, `[]`)
	s.nas.version = "27.10.0"
	s.nas.methods = map[string]bool{}

	for _, op := range operationSet(s) {
		if op.Available {
			t.Errorf("%s must be unavailable on an untested major", op.ID)
		}
		if !strings.Contains(op.UnavailableReason, "27.10") {
			t.Errorf("%s: the version reason must win, got %q", op.ID, op.UnavailableReason)
		}
	}
}
