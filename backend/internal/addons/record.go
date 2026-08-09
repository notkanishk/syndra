package addons

import (
	"context"
	"errors"
	"fmt"
)

// DispatchRecord is proof that a durable record exists for the call about to be
// made, and that it names this exact call.
//
// It replaces what used to be a string. "Commit the record before the call" was
// enforced by Call refusing an empty CallID, which is not the same property: any
// caller could satisfy it with a generated UUID and mutate a target with nothing
// in the database aware the attempt existed. The rule was structural in the one
// path that happened to follow it and conventional everywhere else, which is the
// definition of a rule that will be broken by the next path.
//
// The field is unexported, so this type cannot be constructed outside this
// package. The only way to obtain one is a constructor that reads the record
// back and checks it, which means "a call is authorised by a durable record"
// stops being a thing callers must remember and becomes a thing they cannot
// avoid.
type DispatchRecord struct {
	// callID is what travels to the add-on as the deduplication key.
	callID string
}

// CallID is the deduplication key the add-on will see. Exposed for logging and
// for callers that need to name the record afterwards; it confers nothing,
// because a string cannot be turned back into a DispatchRecord.
func (d DispatchRecord) CallID() string { return d.callID }

func (d DispatchRecord) valid() bool { return d.callID != "" }

// ErrNoCallRecord is the refusal to dispatch without a verified durable record.
var ErrNoCallRecord = errors.New("addon: refusing to dispatch without a durable operation record")

// ErrRecordMismatch means a record exists but does not describe this call.
var ErrRecordMismatch = errors.New("addon: the operation record does not match this call")

// OperationRecord verifies that an open addon_operations row exists for id and
// that it names this target, operation, and subject, and returns the token that
// authorises the dispatch.
//
// Matching all four fields, not merely existence. A caller holding any real
// record id could otherwise use it to authorise a different call entirely —
// dispatching a password.set under the record of a health.get, against a
// subject the record never mentioned, leaving an audit trail that describes
// something that did not happen. An audit trail that can be pointed at the
// wrong event is worse than none, because it will be believed.
//
// It also requires the row to be non-terminal, which makes a second dispatch
// under a settled record impossible: the settle is what closes the window, and
// the window is what a replay would need.
func OperationRecord(ctx context.Context, id, target, operation, subject string) (DispatchRecord, error) {
	row, err := dbOpenAddonOperation(ctx, id)
	if err != nil {
		return DispatchRecord{}, fmt.Errorf("%w: %s: %w", ErrNoCallRecord, id, err)
	}
	if row.Target != target || row.Operation != operation || row.SubjectID != subject {
		// The mismatch is not itemised. Which field disagreed is information
		// about a record the caller has just demonstrated they should not be
		// using.
		return DispatchRecord{}, fmt.Errorf("%w: %s", ErrRecordMismatch, id)
	}
	return DispatchRecord{callID: id}, nil
}
