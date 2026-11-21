package addons

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"syndra/internal/db"
)

// One reply for every read (design §13.3). A read is answered now, from a
// copy the add-on kept, or not at all, and ReadState says which; ReadOf is
// the one function that writes it. The add-on is the one keeper of copies:
// Syndra keeps none, so with the add-on down nobody gets one.

// The three states of a read.
const (
	ReadNow  = "now"
	ReadKept = "kept"
	ReadNone = "none"
)

// ReadState is design §13.3's `read`: {state, taken_at, reason, cause,
// detail, retry_after}. taken_at is present exactly when there is data to
// date; a reading is dated by the add-on's taken_at through ahead and by
// nothing else.
type ReadState struct {
	State      string     `json:"state"`
	TakenAt    *time.Time `json:"taken_at,omitempty"`
	Reason     string     `json:"reason,omitempty"`
	Cause      string     `json:"cause,omitempty"`
	Detail     string     `json:"detail,omitempty"`
	RetryAfter int        `json:"retry_after,omitempty"`
}

// None says there is no data: nothing to show, count or date.
func (s ReadState) None() bool { return s.State != ReadNow && s.State != ReadKept }

// Why is the state's account of why it is not a reading now.
func (s ReadState) Why() Why {
	return Why{Reason: s.Reason, Cause: s.Cause, Detail: s.Detail, RetryAfter: s.RetryAfter}
}

// Kept-copy reasons beyond Unread's (design §13.3): the add-on answered from
// its mirror without being asked for a fresh read.
const ReasonCached = "cached"

// ReadOf is the one writer of `read`. A reply with no observation, or one
// dated the zero time, is not a reading (design §13.3 "one date"): none, with
// why, or a fault when the caller has no better word. A current observation is
// now; any other is the add-on's kept copy, with the stale reason in Syndra's
// words (common/stale.json: unreachable is the app's outage).
func ReadOf(o *Observation, why Why) ReadState {
	if o == nil || o.TakenAt.IsZero() {
		if why.Reason == "" {
			why = Why{Reason: ReasonAddonUnreachable, Cause: CauseFault}
		}
		return ReadState{State: ReadNone, Reason: why.Reason, Cause: why.Cause, Detail: why.Detail, RetryAfter: why.RetryAfter}
	}
	at := o.TakenAt
	if o.Current {
		return ReadState{State: ReadNow, TakenAt: &at}
	}
	kept := ReadState{State: ReadKept, TakenAt: &at, Reason: ReasonCached}
	if s := o.Stale; s != nil {
		switch s.Reason {
		case StaleCooldown:
			kept.Reason = ReasonCooldown
		case staleUnreachable:
			kept.Reason = ReasonAppUnreachable
		}
		kept.Detail = s.Note
		if s.RetryAfter != nil {
			kept.RetryAfter = *s.RetryAfter
		}
	}
	return kept
}

// staleUnreachable is common/stale.json's "the app could not be read now".
const staleUnreachable = "unreachable"

// answeredNow is an observation for a reply whose schema carries no taken_at
// (values, settings, health.get): a live answer the add-on keeps no copy of,
// dated when Syndra asked (Clock.Sent), the older bound (RFC 9111 §4.2.3).
func answeredNow(c Clock) *Observation {
	at := c.Sent
	if at.IsZero() {
		at = timeNow().UTC()
	}
	return &Observation{TakenAt: at.UTC(), Current: true}
}

// unreadable is a 2xx whose body does not read: answered, and not a reading
// (Unread's fault).
func unreadable(resp CallResponse, err error) Why {
	resp.Outcome, resp.Err = OutcomeIndeterminate, fmt.Errorf("%w: %w", ErrUnreadableReply, err)
	return Unread(resp)
}

// Down is the monitor's account of a target that is down (design §13.2), ok
// false while it answers or has not been probed. A page that may not dial (a
// member view with no manifest) and the usage cards' "not counted since" take
// their why from here.
func (st HealthState) Down() (Why, bool) {
	switch st.State {
	case StateAddonUnreachable:
		return Why{Reason: ReasonAddonUnreachable, Cause: st.Cause, RetryAfter: st.RetryAfter}, true
	case StateAppUnreachable:
		return Why{Reason: ReasonAppUnreachable, RetryAfter: st.RetryAfter}, true
	}
	return Why{}, false
}

// WhyNoManifest is why a system that may not be asked (no accepted manifest,
// or its breaker open) has no read: the monitor's why while it holds the
// system down, else what opened the breaker, else the add-on unreachable with
// whatever cause the monitor last saw. The one fallback for every page that
// may not ask.
func WhyNoManifest(target string) Why {
	st := HealthOf(target)
	if why, down := st.Down(); down {
		return why
	}
	if a, err := Get(target); err == nil {
		if opener, left, open := a.br.opened(timeNow()); open {
			opener.RetryAfter = seconds(left)
			return opener
		}
	}
	return Why{Reason: ReasonAddonUnreachable, Cause: st.Cause}
}

// heldError is a read not sent because the monitor holds the add-on
// unreachable (design §13.2): no copy, answered without dialing.
type heldError struct{ why Why }

func (e *heldError) Error() string {
	return fmt.Sprintf("addon: held %s (%s); not dialed", e.why.Reason, e.why.Cause)
}

// read is send for a read: while the monitor holds the add-on unreachable,
// nothing is dialed and the reply is the monitor's why. The probe sends
// through send itself, so it is the one read that still finds the add-on back.
func (a *Addon) read(ctx context.Context, s sendSpec) (CallResponse, error) {
	if why, down := HealthOf(a.Registration.Target).Down(); down && why.Reason == ReasonAddonUnreachable {
		err := &heldError{why: why}
		return CallResponse{Outcome: OutcomeUnreached, Err: err}, err
	}
	return a.send(ctx, s)
}

// NotRead is a read that has no copy to give, as an error: what a service
// returns for the handler to answer 503 TARGET_NOT_READ with (design §13.3),
// and what ResolvesValue fails closed with (owner Q2).
type NotRead struct{ Why Why }

func (e NotRead) Error() string {
	return fmt.Sprintf("not read: %s %s", e.Why.Reason, e.Why.Cause)
}

// whyOfUnreconciled is a reconciliation row's word (Halt, Attribution) as a
// Why: the same words as Unread's, but a cooldown, which the row calls
// read_stale.
func whyOfUnreconciled(reason string) Why {
	if reason == db.UnreconciledStaleRead {
		return Why{Reason: ReasonCooldown}
	}
	return Why{Reason: reason}
}

// notRead is why a read reply is not a reading: Unread of what came back, or
// a fault when it answered and did not decode (err).
func (r ReadReply) notRead(err error) Why {
	resp := CallResponse{Outcome: r.Outcome, Status: r.Status, Err: r.Err, Body: r.Body, Code: r.Code, RetryAfter: r.RetryAfter}
	if r.Outcome == OutcomeSucceeded {
		return unreadable(resp, err)
	}
	return Unread(resp)
}

// observed is when a decoded result was read: its own Observation, or for a
// result whose schema dates nothing, when Syndra asked (answeredNow).
func (r ReadReply) observed(decoded any) *Observation {
	if o, ok := decoded.(interface{ observation() *Observation }); ok && o.observation() != nil {
		return o.observation()
	}
	return answeredNow(r.Clock)
}

// observedRaw is observed for an id's own result, shown raw: dated by its
// taken_at when it carries one (through ahead), else when Syndra asked.
func (r ReadReply) observedRaw(raw json.RawMessage) *Observation {
	var w struct {
		TakenAt *string `json:"taken_at"`
		Current *bool   `json:"current"`
		Stale   *Stale  `json:"stale"`
	}
	if DecodeReply(raw, &w) != nil || w.TakenAt == nil {
		return answeredNow(r.Clock)
	}
	at, ok := takenAt(r.Target, w.TakenAt, r.Clock)
	if !ok {
		return nil
	}
	return &Observation{TakenAt: at, Current: w.Current == nil || *w.Current, Stale: w.Stale}
}
