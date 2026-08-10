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
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

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

// ErrSubjectNotActor is the refusal for a `member`-scoped operation aimed at
// somebody other than the person invoking it.
//
// Scope decides who may invoke an operation. It must also decide ON WHOM.
// Without this, "scoped to member" means only "a member may call this", and
// `password.set` with another person's subject id resets their storage
// credential (design §14).
var ErrSubjectNotActor = errors.New("addonop: a member-scoped operation may only act on the person invoking it")

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

// String, GoString and MarshalJSON all redact, because this type carries a
// member's password in `Params` and the three verbs a caller reaches for
// without thinking are %v, %#v and json.Marshal. A redaction that depends on
// every caller remembering to redact is not one — and this type had none at
// all, while the transport request one layer down had two of the three.
//
// The secret set comes from the effective operation, which fails closed: a
// target that cannot be resolved redacts everything rather than nothing.
func (r Request) String() string {
	return fmt.Sprintf("addon operation target=%s operation=%s actor=%s subject=%s confirmed=%t params=%v",
		r.Target, r.Operation, r.ActorID, r.SubjectID, r.Confirmed,
		addons.RedactedParams(r.Target, r.Operation, r.Params))
}

func (r Request) GoString() string { return r.String() }

func (r Request) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Target    string         `json:"target"`
		Operation string         `json:"operation"`
		ActorID   string         `json:"actor_id"`
		SubjectID string         `json:"subject_id"`
		Confirmed bool           `json:"confirmed"`
		Params    map[string]any `json:"params,omitempty"`
	}{
		Target: r.Target, Operation: r.Operation, ActorID: r.ActorID,
		SubjectID: r.SubjectID, Confirmed: r.Confirmed,
		Params: addons.RedactedParams(r.Target, r.Operation, r.Params),
	})
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

// ErrRateLimited refuses a member driving a secret-bearing path faster than any
// honest member would.
var ErrRateLimited = errors.New("addonop: too many recent attempts for this subject; try again shortly")

// memberOpWindow and memberOpLimit bound how often one subject may drive a
// member-scoped operation.
//
// A credential set is a write path a member controls at will, and it terminates
// in a single rate-limited WebSocket session shared with every other operation
// the add-on performs — so repeated resets are a cheap way to wedge the target
// for everybody, deliberately or by a retry loop in a browser tab. The limit is
// per subject and generous: an honest member sets a password once and resets it
// when they forget it.
//
// Scoped to member operations because those are the ones a member can drive.
// An operator path is behind operator authentication and is rate-limited by
// there being an operator on the other end of it.
const (
	memberOpWindow      = time.Hour
	defaultMemberOpRate = 10
)

func memberOpLimit() int {
	if v := os.Getenv("ADDON_MEMBER_OP_LIMIT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return defaultMemberOpRate
}

// withinSubjectRate refuses before the record and therefore before the network.
//
// A limit enforced after the call is not a limit on the target; it is a limit
// on the reporting.
func withinSubjectRate(ctx context.Context, op addons.EffectiveOperation, req Request) error {
	if op.Scope != addons.ScopeMember {
		return nil
	}
	// Per TARGET as well as per subject and operation. Without the target, one
	// add-on's retries consumed every other add-on's budget for the same
	// person — so a member locked out of the NAS by a loop in a browser tab
	// was also locked out of the door controller, which is a different system
	// with a different session and a different reason for the limit.
	n, err := countRecentOperations(ctx, req.Target, req.SubjectID, req.Operation, memberOpWindow)
	if err != nil {
		// Fail closed. The reason this limit exists is that the path terminates
		// in a shared, rate-limited session on the target; letting it through
		// because the counter could not be read spends exactly the resource the
		// limit protects, at the moment the backend is least able to see it.
		return fmt.Errorf("%w: the recent-attempt count could not be read: %v", ErrRateLimited, err)
	}
	if n >= memberOpLimit() {
		return fmt.Errorf("%w (%d in the last %s)", ErrRateLimited, n, memberOpWindow)
	}
	return nil
}

// bindSubjectToActor enforces that a `member`-scoped operation acts only on the
// authenticated actor.
//
// It reads the EFFECTIVE scope, which is the more restrictive of policy and
// manifest with policy winning,
// so a manifest cannot reach this check by declaring itself member-scoped: it
// can only ever narrow. The rule is still enforced here rather than left to
// either of them, because the manifest is the least trusted input in the system
// and the policy table describes operations rather than requests — "who is this
// call about" is a property of the request and exists nowhere else.
//
// Blank identifiers are refused rather than compared. An unauthenticated caller
// and an unaddressed operation would otherwise both be the empty string, and
// the check would pass on the strength of two absences matching.
func bindSubjectToActor(scope addons.Scope, actor, subject string) error {
	if scope != addons.ScopeMember {
		return nil
	}
	switch {
	case strings.TrimSpace(actor) == "":
		return fmt.Errorf("%w: no authenticated actor", ErrSubjectNotActor)
	case strings.TrimSpace(subject) == "":
		return fmt.Errorf("%w: no subject", ErrSubjectNotActor)
	case actor != subject:
		// Neither identifier is echoed. They are not secrets, but a refusal is
		// not a confirmation service for which subject ids exist either.
		return ErrSubjectNotActor
	}
	return nil
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
	if err := bindSubjectToActor(op.Scope, req.ActorID, req.SubjectID); err != nil {
		return Result{}, fmt.Errorf("%w (%s/%s)", err, req.Target, req.Operation)
	}
	// Before the record, not after it. A request that does not satisfy backend
	// policy's parameter schema is not an attempt at anything — recording it
	// would put a row on the operator's surface for a call that was never
	// legitimate and never left the process.
	if err := validateParams(op, req.Params); err != nil {
		return Result{}, err
	}
	// After validation and before the record. Validation is local and free, so
	// a malformed request costs no database read — and it must not spend a
	// member's budget either, since it never reached the target and never cost
	// it anything.
	if err := withinSubjectRate(ctx, op, req); err != nil {
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

	// The token naming the record that will authorise this dispatch. It claims
	// nothing here: the transport takes the claim at the moment it sends, so
	// the one-shot is where the shot is. A record that cannot be claimed then
	// comes back as an unreached outcome and settles as one, which is the
	// truth — nothing was sent.
	record := operationRecord(id, req.Target, req.Operation, req.SubjectID)

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
