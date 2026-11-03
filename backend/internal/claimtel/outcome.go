// Package claimtel counts every Actions v2 call by what Syndra answered.
//
// Zitadel calls the claims target with interrupt-on-error off: whatever Syndra
// answers, or fails to answer, the token is issued. So an answer without
// Syndra's claims is silent unless something here counts it. Every call gets
// exactly one Outcome, set by a wrapper around the whole route (signature check
// included), and lands in fixed-size in-memory counters that a background flush
// folds into 5-minute rows. Never one row per call (claims-delivery-integrity
// assessment §4a).
package claimtel

import (
	"context"
	"time"
)

// Outcome is what one call came to. The classes are exhaustive and mutually
// exclusive: a call that sets none is BadRequest.
type Outcome string

const (
	Served                  Outcome = "served"
	ServedLKG               Outcome = "served_lkg" // a last-known-good answer; set from S2 on
	DegradedNoFacts         Outcome = "degraded_no_facts"
	DegradedShapeUnreadable Outcome = "degraded_shape_unreadable"
	DegradedMalformed       Outcome = "degraded_malformed"
	NoGrants                Outcome = "no_grants"
	ContractFault           Outcome = "contract_fault"
	RefusedCrossProject     Outcome = "refused_cross_project"
	NotLanding              Outcome = "not_landing"
	UnknownFunction         Outcome = "unknown_function"
	SigMissing              Outcome = "sig_missing"
	SigStale                Outcome = "sig_stale"
	SigMismatch             Outcome = "sig_mismatch"
	BadRequest              Outcome = "bad_request"
	DeadlineExceeded        Outcome = "deadline_exceeded"
	Panic                   Outcome = "panic"
)

// Outcomes lists every class once, in a fixed order.
var Outcomes = [...]Outcome{
	Served, ServedLKG, DegradedNoFacts, DegradedShapeUnreadable, DegradedMalformed,
	NoGrants, ContractFault, RefusedCrossProject, NotLanding, UnknownFunction,
	SigMissing, SigStale, SigMismatch, BadRequest, DeadlineExceeded, Panic,
}

var outcomeIndex = func() map[Outcome]int {
	m := make(map[Outcome]int, len(Outcomes))
	for i, o := range Outcomes {
		m[o] = i
	}
	return m
}()

// Valid reports whether o is one of the enumerated classes.
func Valid(o Outcome) bool { _, ok := outcomeIndex[o]; return ok }

// Severity is how /healthz/claims reads a class seen recently: Hard means every
// token on that path is losing its claims for a reason no retry fixes; Soft
// means some are, on a degraded path; anything else is expected traffic
// (no_grants and not_landing are an app's settings, not a fault).
//
// sig_missing is deliberately not Hard: Zitadel always signs, so an unsigned
// request did not come from Zitadel. The deploy check sends one on every
// deploy, and anyone on the internet can.
type Severity int

const (
	Informational Severity = iota
	Soft
	Hard
)

func (o Outcome) Severity() Severity {
	switch o {
	case SigStale, SigMismatch, ContractFault, BadRequest, Panic:
		return Hard
	case ServedLKG, DegradedNoFacts, DegradedShapeUnreadable, DegradedMalformed, DeadlineExceeded, UnknownFunction:
		return Soft
	}
	return Informational
}

// KeySlotCurrent names the signing key that verified a call. Only one key
// exists until the keyring (S5) adds a next one.
const KeySlotCurrent = "current"

// None stands in for an absent dimension (no project, no client id).
const None = "-"

// Call is one v2 call, filled in as the route runs. The wrapper owns it; the
// signature check and the handler only set fields on it.
type Call struct {
	Outcome     Outcome
	ProjectID   string
	ClientID    string
	Function    string
	KeySlot     string
	UserID      string
	ClientKnown bool   // the client has a shape of its own
	Body        []byte // the verified body, for the payload fingerprint
	At          time.Time
	Elapsed     time.Duration
}

// Set records the outcome. Safe on a nil Call, so shared middleware can call
// it on routes that carry none.
func (c *Call) Set(o Outcome) {
	if c != nil {
		c.Outcome = o
	}
}

type ctxKey struct{}

// With carries c through a request.
func With(ctx context.Context, c *Call) context.Context { return context.WithValue(ctx, ctxKey{}, c) }

// From is the request's Call, or nil off the data plane.
func From(ctx context.Context) *Call {
	c, _ := ctx.Value(ctxKey{}).(*Call)
	return c
}
