package addons

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"time"
)

// Why a reply is not a reading (design §13.1): which side is down, and why.
// Reason is empty for a reply that was read.
//
// Two outages, never one word. addon_unreachable is Syndra holding no usable
// answer from the add-on (only Syndra can know it, so Cause says what the pipe
// saw); app_unreachable is the add-on answering that it cannot read its app
// (only the add-on can know it, so Detail is its own sentence). The other three
// are not outages: read_refused (the add-on answered and did not serve the read,
// something to fix rather than wait out), cooldown (a fresh read refused inside
// the add-on's cooldown) and addon_outdated (the add-on does not speak what was
// asked).
type Why struct {
	Reason string `json:"reason"`
	Cause  string `json:"cause,omitempty"`
	// Detail is the add-on's own sentence, shown first where it has one.
	Detail string `json:"detail,omitempty"`
	// RetryAfter is seconds: the breaker's time left, the add-on's Retry-After.
	RetryAfter int `json:"retry_after,omitempty"`
}

// Reasons (design §13.1, §13.3).
const (
	ReasonAddonUnreachable = "addon_unreachable"
	ReasonAppUnreachable   = "app_unreachable"
	ReasonReadRefused      = "read_refused"
	ReasonCooldown         = "cooldown"
	ReasonAddonOutdated    = "addon_outdated"
)

// Causes of addon_unreachable: what the pipe saw (design §13.1's table).
const (
	// CauseNoAnswer: refused, no route, name not found, dial or TLS handshake
	// timeout. Nothing reached the add-on.
	CauseNoAnswer = "no_answer"
	// CauseWrongHost: something answered at the address and is not the add-on
	// registered here (the pinned key, a certificate error).
	CauseWrongHost = "wrong_host"
	// CauseNoKey: Syndra cannot load the add-on's shared secret, so nothing was
	// sent. Syndra's fault, not the add-on's: its next step is Syndra's secret
	// file (added 21 Nov 2026 from the 8b review).
	CauseNoKey = "no_key"
	// CauseTimeout: the request went out and the answer stopped (a timeout, a
	// reset, a reply cut off).
	CauseTimeout = "timeout"
	// CauseFault: the add-on answered with an error it did not explain (a 5xx
	// with no registered code, a redirect, a body that does not read).
	CauseFault = "fault"
)

// Unreachable says the reply is one of the two outages: the reason a queued
// row spends no attempt on (design §13.4), and the monitor's miss.
func (w Why) Unreachable() bool {
	return w.Reason == ReasonAddonUnreachable || w.Reason == ReasonAppUnreachable
}

// Add-on wire codes that say the app is down (registry/errors.json).
const (
	codeNoCurrentRead    = "NO_CURRENT_READ"
	codeTargetUnreadable = "TARGET_UNREADABLE"
)

// errNoTransport is Syndra holding no usable key for the add-on (its secret
// did not load): no_key, whose next step is Syndra's secret file.
var errNoTransport = errors.New("addon: no transport material")

// errNotPrepared is Syndra failing to make the call before sending it (its
// call record did not write, its body did not build): Syndra's own refusal,
// read_refused, never an outage.
var errNotPrepared = errors.New("addon: the call could not be prepared")

// CircuitOpenError is a call the backend's breaker refused (design §13.2): what
// opened it and the time left. errors.Is(err, ErrCircuitOpen) holds.
type CircuitOpenError struct {
	Target     string
	Opener     Why
	RetryAfter time.Duration
}

func (e *CircuitOpenError) Error() string {
	return fmt.Sprintf("%v: %s (%s %s, %s left)", ErrCircuitOpen, e.Target, e.Opener.Reason, e.Opener.Cause, e.RetryAfter.Round(time.Second))
}

func (e *CircuitOpenError) Is(target error) bool { return target == ErrCircuitOpen }

// Unread is the one mapping from a reply to why it is not a reading (design
// §13.1); the zero Why for a reply that was read. Every page, the reconcile
// pass (SubjectsResult.Halt), the health monitor and the drain read it.
func Unread(r CallResponse) Why {
	if r.Outcome == OutcomeSucceeded {
		return Why{}
	}
	var open *CircuitOpenError
	if errors.As(r.Err, &open) {
		why := open.Opener
		why.RetryAfter = seconds(open.RetryAfter)
		return why
	}
	switch {
	case r.ShapeRefusal:
		return Why{Reason: ReasonAddonOutdated}
	case r.Code == CodeStoreUnreadable, r.Code == codeLifecycleRefusal, r.Code == CodeTargetChanged:
		// TARGET_CHANGED: the app changed while the add-on read it. Answered,
		// so not an outage: a queued row spends an attempt and backs off.
		return Why{Reason: ReasonReadRefused, Detail: problemDetail(r.Body), RetryAfter: r.RetryAfter}
	case r.Code == codeTargetUnreachable, r.Code == codeNoCurrentRead, r.Code == codeTargetUnreadable:
		// Before 429: TARGET_UNREADABLE may come as one, and it is the app.
		return Why{Reason: ReasonAppUnreachable, Detail: problemDetail(r.Body), RetryAfter: r.RetryAfter}
	case r.Code == codeFreshReadCooldown, r.Status == 429:
		return Why{Reason: ReasonCooldown, RetryAfter: max(1, r.RetryAfter)}
	case r.Code != "":
		// Any other registered code is the add-on explaining itself, whatever
		// its status: only a 5xx with no code is a fault.
		return Why{Reason: ReasonReadRefused, Detail: problemDetail(r.Body)}
	case r.Status == 0:
		return unsent(r)
	case r.Status >= 200 && r.Status < 300:
		// Answered and not read whole: cut off is the answer stopping, the
		// rest is the add-on saying something that does not read.
		if cutOff(r.Err) {
			return Why{Reason: ReasonAddonUnreachable, Cause: CauseTimeout}
		}
		return Why{Reason: ReasonAddonUnreachable, Cause: CauseFault}
	case r.Status >= 500, r.Status >= 300 && r.Status < 400:
		return Why{Reason: ReasonAddonUnreachable, Cause: CauseFault}
	}
	// Any other refusal is the add-on answering about the request.
	return Why{Reason: ReasonReadRefused, Detail: problemDetail(r.Body)}
}

// unsent is a reply with no status: nothing came back.
func unsent(r CallResponse) Why {
	var held *HeldError
	switch {
	case errors.As(r.Err, &held):
		return held.Why
	case errors.Is(r.Err, ErrFeatureNotDeclared):
		return Why{Reason: ReasonAddonOutdated}
	case errors.Is(r.Err, ErrNoCallRecord), errors.Is(r.Err, ErrNoFingerprint), errors.Is(r.Err, ErrFreshOutOfScope),
		errors.Is(r.Err, errNotPrepared):
		// Syndra's own refusal before anything left: not an outage.
		return Why{Reason: ReasonReadRefused}
	case errors.Is(r.Err, errNoTransport):
		return Why{Reason: ReasonAddonUnreachable, Cause: CauseNoKey}
	case wrongHost(r.Err):
		return Why{Reason: ReasonAddonUnreachable, Cause: CauseWrongHost}
	case r.Outcome == OutcomeIndeterminate:
		return Why{Reason: ReasonAddonUnreachable, Cause: CauseTimeout}
	}
	return Why{Reason: ReasonAddonUnreachable, Cause: CauseNoAnswer}
}

// cutOff says a 2xx's body stopped arriving: a deadline, a reset, an early EOF.
func cutOff(err error) bool {
	var ne net.Error
	return errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, context.DeadlineExceeded) || errors.As(err, &ne)
}

func seconds(d time.Duration) int { return int(math.Ceil(d.Seconds())) }

// HeldError is a write Syndra refused undialed because the monitor holds its
// system addon_unreachable (design §13.4): nothing went out. Why is the
// monitor's, so the page names the same cause the roster does.
type HeldError struct {
	Target string
	Why    Why
}

func (e *HeldError) Error() string {
	return fmt.Sprintf("addon %s: held %s (%s), not dialed", e.Target, e.Why.Reason, e.Why.Cause)
}

// Held is Down filtered to addon_unreachable: a read or an interactive write
// to target is refused at once, undialed (design §13.2, §13.4 "Halts"). App
// down is not held: the add-on answers, and its own refusal says so. The
// drain does not ask: it already claims no row of an unreachable system.
func Held(target string) (Why, bool) {
	why, down := HealthOf(target).Down()
	if !down || why.Reason != ReasonAddonUnreachable {
		return Why{}, false
	}
	return why, true
}

// HeldTargets is every registered target Held right now: the systems whose
// queued revocations wait without escalating (db.CountUnconfirmedRevocations,
// owner 28.3: only a revocation that can't reach its system is exempt).
func HeldTargets() []string {
	var out []string
	for _, reg := range Registered() {
		if _, held := Held(reg.Target); held {
			out = append(out, reg.Target)
		}
	}
	return out
}

// NotLandedError is a write that did not land: unreached (nothing went out) or
// indeterminate (sent, answer lost), and why. What writeOutcome answers from.
type NotLandedError struct {
	Target  string
	Outcome Outcome
	Why     Why
	Err     error
}

func (e *NotLandedError) Error() string {
	return fmt.Sprintf("addon %s: %s (%s %s): %v", e.Target, e.Outcome, e.Why.Reason, e.Why.Cause, e.Err)
}

func (e *NotLandedError) Unwrap() error { return e.Err }
