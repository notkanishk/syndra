package addons

import (
	"context"
	"errors"
	"fmt"
)

// DispatchRecord authorises exactly one dispatch, and spends its authority at
// the moment the request goes out rather than at the moment it was obtained.
//
// The distinction is the whole design, and it took three attempts to get right:
//
//   - A plain string was not evidence of anything. Any caller could satisfy
//     "the record exists" with a generated UUID.
//   - A token verified only when minted verified a call that might not be the
//     one sent: claim against a health check, dispatch a password.
//   - A token whose durable state was claimed when minted still authorised any
//     number of dispatches, because a Go value is copyable and comparing its
//     stored binding is not the same as spending it. One legitimate mint could
//     be replayed through Call, concurrently or after settlement.
//
// So the token holds no claimed state. It holds the binding and a one-shot
// consume, and the transport calls consume immediately before the request. The
// authority lives in the database row, where copying a Go value cannot reach
// it: the second attempt finds `claimed_at` already set and is refused.
type DispatchRecord struct {
	// callID is what travels to the add-on as the deduplication key.
	callID string
	// The call this record authorises, and no other.
	target    string
	operation string
	subject   string
	// consume takes the durable claim. Exactly one caller can succeed, ever.
	consume func(context.Context) error
}

// CallID is the deduplication key the add-on will see. Exposed for logging and
// for callers that need to name the record afterwards; it confers nothing,
// because a string cannot be turned back into a DispatchRecord.
func (d DispatchRecord) CallID() string { return d.callID }

func (d DispatchRecord) valid() bool { return d.callID != "" && d.consume != nil }

// authorises reports whether this token was minted for exactly this call.
func (d DispatchRecord) authorises(target, operation, subject string) bool {
	return d.target == target && d.operation == operation && d.subject == subject
}

// ErrNoCallRecord is the refusal to dispatch without a durable record that can
// be claimed for this call.
var ErrNoCallRecord = errors.New("addon: refusing to dispatch without a claimable operation record")

// ErrRecordMismatch means the token authorises a different call from the one
// being made.
var ErrRecordMismatch = errors.New("addon: the operation record does not authorise this call")

// OperationRecord names the addon_operations row that will authorise a
// dispatch. It touches nothing: the row is claimed by the transport, at the
// moment of the call.
//
// Deliberately not "read it now and claim it now". A capability claimed when it
// is obtained is still spendable more than once, because what the caller then
// holds is an ordinary Go value that copies freely — and a check that compares
// the copy's contents is a check the copy always passes. Deferring the claim to
// the point of use puts the one-shot where the shot is.
//
// The claim carries the call's identity in its own predicate, so a row that
// does not describe this call is not claimed at all: a mismatched attempt
// consumes nothing and the legitimate dispatch behind it is unharmed.
func OperationRecord(id, target, operation, subject string) DispatchRecord {
	return DispatchRecord{
		callID:    id,
		target:    target,
		operation: operation,
		subject:   subject,
		consume: func(ctx context.Context) error {
			if _, err := dbClaimAddonOperation(ctx, id, target, operation, subject); err != nil {
				return fmt.Errorf("%w: %s: %w", ErrNoCallRecord, id, err)
			}
			return nil
		},
	}
}
