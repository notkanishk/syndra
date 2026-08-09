// Package addonop runs the one-shot half of the add-on contract: operations
// that carry a secret, happen once, and are never queued.
//
// The entitlement plane and this one differ in exactly one property, and every
// other difference follows from it. An entitlement is desired state, so it can
// be re-derived and re-applied forever; an operation is an event carrying a
// value the backend refuses to keep, so it cannot be re-attempted at all. That
// is why there is no outbox row here, no retry, and no queue — and why there is
// a record committed before the call, because when a dispatch cannot be retried
// the only thing that can survive it is evidence that it happened.
package addonop

import (
	"context"
	"errors"
	"fmt"

	"syndra/internal/addons"
	"syndra/internal/db"
)

// ErrConfirmationRequired is the refusal for an operation backend policy marks
// as needing confirmation, invoked without one.
//
// Enforced here rather than left to the interface. A confirmation dialog that
// only the frontend knows about is a suggestion; account.purge is irreversible,
// and policy.go states plainly that the backend refuses the call without a
// confirmation, so the backend refuses the call.
var ErrConfirmationRequired = errors.New("addonop: this operation requires an explicit confirmation")

// Request is one operation invocation.
//
// Params carries the secret and nothing else does. It reaches the add-on and is
// discarded; no field of it is persisted, and the record type this package
// writes has nowhere to put it even by accident.
type Request struct {
	Target    string
	Operation string
	ActorID   string
	SubjectID string
	Confirmed bool
	Params    map[string]any
}

// Result is what the backend knows afterwards.
//
// Read Outcome, not the returned error. Dispatch's error reports that the
// PROTOCOL broke — the operation was not callable, the record could not be
// committed, the outcome could not be written — and a nil error says only that
// the protocol ran, never that the target did what was asked.
type Result struct {
	// OperationID is the record id, which is also what the add-on deduplicates
	// on. Empty only when nothing was recorded, which means nothing was sent.
	OperationID string
	Status      string
	Outcome     addons.Outcome
	// Err is the add-on's own answer, when it was not success. Held here rather
	// than returned so that reading it is a choice and not an accident.
	Err error
}

// statusFor maps a dispatch outcome onto the persisted status. Total over
// addons.AllOutcomes, checked by a test — an outcome with no mapping would
// otherwise be written as the empty string and rejected by the CHECK, turning a
// missing case into a constraint violation at the worst moment.
var statusFor = map[addons.Outcome]string{
	addons.OutcomeSucceeded:     db.AddonOpSucceeded,
	addons.OutcomeRejected:      db.AddonOpRejected,
	addons.OutcomeUnreached:     db.AddonOpUnreached,
	addons.OutcomeIndeterminate: db.AddonOpIndeterminate,
}

// Dispatch performs one secret-bearing operation end to end.
//
// The order is the contract:
//
//  1. Resolve callability. An operation the effective set does not offer leaves
//     no record, because it was never a legitimate attempt.
//  2. Commit the record, non-terminal, with its audit row. Before the call, so
//     that a crash during the call still leaves the question visible.
//  3. Call, sending the record id as the deduplication key.
//  4. Write the terminal status.
//
// It is called once. There is no retry anywhere in this function, and that is
// deliberate rather than unfinished: a retry needs the parameters, the
// parameters are the secret, and keeping the secret to enable the retry is the
// vault this design exists to avoid. A member whose attempt did not land
// resubmits, which is a new record and a new secret.
func Dispatch(ctx context.Context, req Request) (Result, error) {
	op, err := resolveOperation(req.Target, req.Operation)
	if err != nil {
		return Result{}, err
	}
	if op.Confirm && !req.Confirmed {
		return Result{}, fmt.Errorf("%w: %s/%s", ErrConfirmationRequired, req.Target, req.Operation)
	}
	// Before the record, not after it. A request that does not satisfy backend
	// policy's parameter schema is not an attempt at anything — recording it
	// would put a row on the operator's surface for a call that was never
	// legitimate and never left the process.
	if err := validateParams(op, req.Params); err != nil {
		return Result{}, err
	}

	id, err := beginOperation(ctx, db.AddonOperationParams{
		Target:    req.Target,
		Operation: req.Operation,
		ActorID:   req.ActorID,
		SubjectID: req.SubjectID,
	})
	if err != nil {
		// Nothing was sent. The record is the precondition for the call, not a
		// log of it, so failing to write it means the call does not happen.
		return Result{}, fmt.Errorf("addonop: no record committed, nothing dispatched: %w", err)
	}

	// Read the record back and check it names this exact call. It was just
	// written, so this is not doubt about the write — it is what mints the
	// token the transport requires, and the token is what makes "a dispatch is
	// authorised by a durable record" a property of the type system rather than
	// of every caller's memory.
	record, err := operationRecord(ctx, id, req.Target, req.Operation, req.SubjectID)
	if err != nil {
		// The record exists and cannot be verified, so nothing is dispatched
		// and the row stays non-terminal — which is exactly what it is: an
		// operation nobody can say happened, because it did not.
		return Result{OperationID: id, Status: db.AddonOpDispatching}, fmt.Errorf("addonop: %w", err)
	}

	resp := callAddon(ctx, addons.CallRequest{
		Target:    req.Target,
		Operation: req.Operation,
		Record:    record,
		Subject:   req.SubjectID,
		Params:    req.Params,
	})

	status, ok := statusFor[resp.Outcome]
	if !ok {
		// An outcome this package has never heard of is not evidence of
		// success. Recording it as indeterminate is the safe reading and puts
		// the row on the surface a human already watches.
		status = db.AddonOpIndeterminate
	}

	result := Result{OperationID: id, Status: status, Outcome: resp.Outcome, Err: resp.Err}
	if err := settleOperation(ctx, id, status); err != nil {
		// The call already happened; failing to record its outcome does not
		// un-happen it. The row stays `dispatching`, which is the honest state
		// for "we dispatched and cannot say what came back", and the unresolved
		// surface will show it. Reporting the outcome as settled here would be
		// the one lie this protocol must not tell.
		result.Status = db.AddonOpDispatching
		return result, fmt.Errorf("addonop: operation %s was dispatched but its outcome was not recorded: %w", id, err)
	}
	return result, nil
}
