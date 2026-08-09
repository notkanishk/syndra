package addons

import (
	"context"
	"errors"
	"fmt"
)

// DispatchRecord is proof that a durable record exists for the call about to be
// made, that it names that exact call, and that no other call may use it.
//
// It replaces what used to be a string. "Commit the record before the call" was
// enforced by Call refusing an empty CallID, which is not the same property: any
// caller could satisfy it with a generated UUID and mutate a target with nothing
// in the database aware the attempt existed.
//
// It carries the binding rather than only the id, and that is the second half.
// A token that remembered only which record it came from would have its
// verification evaporate at the moment of minting: a caller could mint against
// one record and then send an entirely different call under it, and the check
// that ran would have checked the call that was not made.
//
// The fields are unexported, so this type cannot be constructed or edited
// outside this package. The only way to obtain one is a constructor that claims
// the record, which means "a call is authorised by a durable record describing
// it" stops being a thing callers must remember and becomes a thing they cannot
// avoid.
type DispatchRecord struct {
	// callID is what travels to the add-on as the deduplication key.
	callID string
	// The call this record authorises, and no other.
	target    string
	operation string
	subject   string
}

// CallID is the deduplication key the add-on will see. Exposed for logging and
// for callers that need to name the record afterwards; it confers nothing,
// because a string cannot be turned back into a DispatchRecord.
func (d DispatchRecord) CallID() string { return d.callID }

func (d DispatchRecord) valid() bool { return d.callID != "" }

// authorises reports whether this token was minted for exactly this call.
func (d DispatchRecord) authorises(target, operation, subject string) bool {
	return d.target == target && d.operation == operation && d.subject == subject
}

// ErrNoCallRecord is the refusal to dispatch without a claimed durable record.
var ErrNoCallRecord = errors.New("addon: refusing to dispatch without a durable operation record")

// ErrRecordMismatch means the token authorises a different call from the one
// being made.
var ErrRecordMismatch = errors.New("addon: the operation record does not authorise this call")

// OperationRecord claims the addon_operations row for this call and returns the
// token authorising the dispatch.
//
// A claim, not a lookup. A lookup can be repeated: two callers could obtain the
// same record concurrently, and a caller could re-read a settled record and
// dispatch under it a second time. The claim is a single conditional UPDATE, so
// a record authorises exactly one call, once, and the window closes the instant
// it is taken rather than when the outcome is finally written.
//
// The record must also name this target, operation, and subject. Existence
// alone would let a caller holding any real record id authorise a different call
// with it, leaving an audit trail that describes something that did not happen —
// worse than no trail, because it will be believed. The comparison lives in the
// claim's own predicate, so an attempt that does not match consumes nothing and
// the legitimate dispatch behind it is unharmed.
func OperationRecord(ctx context.Context, id, target, operation, subject string) (DispatchRecord, error) {
	row, err := dbClaimAddonOperation(ctx, id, target, operation, subject)
	if err != nil {
		return DispatchRecord{}, fmt.Errorf("%w: %s: %w", ErrNoCallRecord, id, err)
	}
	// Built from the row the database returned, not from the arguments. The
	// token's binding is then a fact about a committed record rather than an
	// echo of what the caller asked for.
	return DispatchRecord{
		callID:    row.ID,
		target:    row.Target,
		operation: row.Operation,
		subject:   row.SubjectID,
	}, nil
}
