package addons

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"syndra/internal/db"
)

// The add-on pipe: the one place a request leaves for a target.
//
// Every call used to hand-copy `allow → credentialFor → [claim] → doAuthenticated
// → record`, about twelve times, so a new endpoint that forgot a step never
// tripped the breaker or skipped its trace. Now there is one copy, and a guard
// test (pipe_guard_test.go) fails if another file in this package reaches the
// transport or the breaker directly.
//
// What the pipe guarantees:
//
//   - the breaker is consulted before the credential is loaded, and every answer
//     is recorded against it (SkipBreaker is the one declared exception);
//   - a Write is refused unless it carries a trace made BEFORE the call: a
//     claimable addon_operations row (Claim) or an outbox row's call id (TraceID);
//   - every Write carries an idempotency key, sent as the Idempotency-Key header;
//   - the claim is taken after every other refusal and immediately before the
//     request, so a call that never left never spends its record.
//
// operate adds the record itself (row and audit line, then settle) for the
// mutations that have no row of their own: PutSetting and SetLifecycle.

// idempotencyHeader names the key a Write carries. It is a header, not a body
// field: the add-ons decode strictly, so a new body field would be refused by an
// add-on that has not learned it, whereas an unknown header is ignored.
const idempotencyHeader = "Idempotency-Key"

type sendSpec struct {
	Method, Path string
	Body         []byte
	// Write is anything that changes the target. A read, including a POST read
	// such as /plan, leaves no row and needs no trace.
	Write bool
	// Claim takes the one-shot claim on the operation record.
	Claim func(context.Context) error
	// TraceID is the call id of a durable row made elsewhere (the outbox row for
	// an entitlement apply). A Write needs this or Claim.
	TraceID string
	// IdempotencyKey defaults to TraceID. Set it when the key is the record's id.
	IdempotencyKey string
	// SkipBreaker leaves the breaker neither consulted nor fed. Only lifecycle
	// uses it: it is the call an operator makes to stop the add-on serving work,
	// and a refusal there must not count as the add-on failing to answer.
	SkipBreaker bool
}

// send is the one request path. A non-nil error means nothing left the process;
// the returned response is then an unreached outcome carrying the same error.
func (a *Addon) send(ctx context.Context, s sendSpec) (CallResponse, error) {
	refuse := func(err error) (CallResponse, error) {
		return CallResponse{Outcome: OutcomeUnreached, Err: err}, err
	}
	if s.Write && s.Claim == nil && s.TraceID == "" {
		return refuse(ErrNoCallRecord)
	}
	if !s.SkipBreaker && !a.br.allow(timeNow()) {
		return refuse(fmt.Errorf("%w: %s", ErrCircuitOpen, a.Registration.Target))
	}
	cred, err := credentialFor(a.Registration)
	if err != nil {
		return refuse(fmt.Errorf("addon %s: %w", a.Registration.Target, err))
	}
	if s.Claim != nil {
		if err := s.Claim(ctx); err != nil {
			return refuse(err)
		}
	}
	var hdr http.Header
	if key := firstNonEmpty(s.IdempotencyKey, s.TraceID); s.Write && key != "" {
		hdr = http.Header{}
		hdr.Set(idempotencyHeader, key)
	}
	resp := doAuthenticatedWith(ctx, cred, s.Method, a.Registration.BaseURL+s.Path, s.Body, callTimeout, hdr)
	if !s.SkipBreaker {
		a.br.record(timeNow(), resp)
	}
	return resp, nil
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// operate is a Write that owns its record: begin the row and its audit line
// (committed before anything is sent, and no row means no call), send with the
// row's id as claim and idempotency key, then settle how it ended. A refusal
// settles as unreached, which is what an unclaimed row may settle as.
func (a *Addon) operate(ctx context.Context, p db.AddonOperationParams, s sendSpec) (CallResponse, error) {
	id, err := dbBeginAddonOperation(ctx, p)
	if err != nil {
		// No record, no call: the record is the precondition, not a log of it.
		err = fmt.Errorf("addon %s: no record committed, nothing sent: %w", p.Target, err)
		return CallResponse{Outcome: OutcomeUnreached, Err: err}, err
	}
	s.Write = true
	s.Claim = OperationRecord(id, p.Target, p.Operation, p.SubjectID).consume
	s.IdempotencyKey = id
	resp, sendErr := a.send(ctx, s)
	if err := dbSettleAddonOperation(ctx, id, StatusFor(resp.Outcome)); err != nil {
		// The row stays `dispatching`, which is the honest state for "sent, and we
		// cannot say what came back"; the unresolved surface shows it.
		log.Printf("[ADDON] %s/%s call_id=%s dispatched but its outcome was not recorded: %v", p.Target, p.Operation, id, err)
	}
	return resp, sendErr
}

// StatusFor maps an outcome onto the persisted addon_operations status. Total
// over AllOutcomes; an outcome nobody has heard of is indeterminate, never
// success, so it lands on the surface a human already watches.
func StatusFor(o Outcome) string {
	switch o {
	case OutcomeSucceeded:
		return db.AddonOpSucceeded
	case OutcomeRejected:
		return db.AddonOpRejected
	case OutcomeUnreached:
		return db.AddonOpUnreached
	}
	return db.AddonOpIndeterminate
}
