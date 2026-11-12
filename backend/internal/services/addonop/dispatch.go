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

// ErrReadNotDispatched is the refusal for an effect: read operation: a read
// goes through addons.Read, which writes no row, and never through here, so no
// caller can log one (design §4: one read path, no log).
var ErrReadNotDispatched = errors.New("addonop: a read is never dispatched; it goes through the one read path")

// ErrPhysicalNotResent refuses a resend of a physical operation. Nobody sends
// one again, ever (design §4 physical rule 2): a lost answer is settled by
// asking the add-on (GET /calls), never by acting a second time.
var ErrPhysicalNotResent = errors.New("addonop: a physical operation is never sent again")

// ErrCallUnsettled refuses a physical operation while this subject's earlier
// one is open, sent or about to be, and unsettled (CALL_UNSETTLED), checked
// in the record's insert (db.OpenCallError). The add-on refuses it too; this
// is the backend not leaving "never twice" to the least trusted component.
var ErrCallUnsettled = errors.New("addonop: an earlier call of this operation for this subject is still unsettled")

// UnsettledError is ErrCallUnsettled naming the earlier call, so the page can
// show that one as still being checked.
type UnsettledError struct {
	OperationID string
	SentAt      time.Time
}

func (e *UnsettledError) Error() string {
	return fmt.Sprintf("%v: %s", ErrCallUnsettled, e.OperationID)
}

func (e *UnsettledError) Unwrap() error { return ErrCallUnsettled }

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
	// About is the non-secret thing this change is about, for the record's
	// `about` column: the account for adopt, release and purge, the value for
	// setting.put. Dropped for every other operation (aboutOperations), so a
	// password operation's record can never carry one.
	About string
	// RetryOf names the first send when this is the member page's one
	// automatic resend of the same value after a lost reply (A-13 §4). The
	// record links to it, and the resend does not spend the rate limit: the
	// limiter must not be what answers it.
	RetryOf string
}

// aboutOperations are the only operations whose record may carry `about`:
// the item a checkout names is what the log says it was about (design §10).
var aboutOperations = map[string]bool{
	"account.adopt": true, "account.release": true, "account.purge": true, "setting.put": true,
	"item.checkout": true,
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
	// AccountID is the identity (contract 2's account_id) of the account an
	// `account.adopt` bound, as the add-on read it from the target.
	//
	// The only field of an add-on's answer this layer parses, and it is parsed
	// because the backend writes its own copy of the binding: a copy keyed only
	// on the name it was given disagrees with the add-on's the moment somebody
	// renames the account, and a binding that cannot recognise its own account
	// hands it back to the unmanaged inventory for anybody to claim.
	//
	// Empty when the add-on did not send one. Deliberately not an error: an
	// operation that succeeded must not be reported as failed because its
	// answer lacked an optional field, and the caller decides what an empty
	// one means for the record it is about to write.
	AccountID string
	// NoChange is the add-on answering that nothing needed doing (Status
	// no_change): Outcome is still succeeded, the state is as asked.
	NoChange bool
	// Code and Refusal are a refusal's code and what it names, for the page's
	// words (UNKNOWN_RESOURCE and its param, LIMIT_REACHED and its limit).
	Code    string
	Refusal addons.Refusal
	// View is a physical operation's reply for the generic renderer (the
	// record made), through the read path's decoder; empty for a secret one.
	View *addons.ReadView
}

// operationAnswer is the add-on's response envelope, narrowed to what this
// layer reads. Everything else in it belongs to the caller that asked for it.
type operationAnswer struct {
	AccountID string `json:"account_id"`
}

// AccountIDOf is an operation reply's account_id, empty when it has none or
// does not parse: the one reading of it, for a live reply (Dispatch) and for
// the reply an add-on recorded and GET /calls hands back (settleByCall).
func AccountIDOf(reply []byte) string {
	var answer operationAnswer
	if len(reply) == 0 || json.Unmarshal(reply, &answer) != nil {
		return ""
	}
	return answer.AccountID
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

// The outcome-to-status mapping is addons.StatusFor, the one the pipe settles
// PutSetting and SetLifecycle with too.

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
	if op.Effect == addons.EffectRead {
		return Result{}, fmt.Errorf("%w: %s/%s", ErrReadNotDispatched, req.Target, req.Operation)
	}
	if op.Confirm && !req.Confirmed {
		return Result{}, fmt.Errorf("%w: %s/%s", ErrConfirmationRequired, req.Target, req.Operation)
	}
	if op.Effect == addons.EffectPhysical && req.RetryOf != "" {
		return Result{}, fmt.Errorf("%w: %s/%s", ErrPhysicalNotResent, req.Target, req.Operation)
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
	//
	// The automatic resend is the exception: it is the same change sent again
	// once, and the insert below refuses a second resend or one naming a
	// different call, so skipping the count here cannot be spent twice.
	if req.RetryOf == "" {
		if err := withinSubjectRate(ctx, op, req); err != nil {
			return Result{}, err
		}
	}
	about := req.About
	if !aboutOperations[req.Operation] {
		about = ""
	}

	id, err := beginOperation(ctx, db.AddonOperationParams{
		Target:    req.Target,
		Operation: req.Operation,
		ActorID:   req.ActorID,
		SubjectID: req.SubjectID,
		About:     about,
		RetryOf:   req.RetryOf,
		// The backend's half of CALL_UNSETTLED: one check, in the insert, so
		// a refused confirm leaves no record and sends nothing. The add-on
		// refuses it too; this is not leaving "never twice" to the least
		// trusted component.
		Physical: op.Effect == addons.EffectPhysical,
	})
	var open *db.OpenCallError
	switch {
	case errors.As(err, &open):
		return Result{}, &UnsettledError{OperationID: open.OperationID, SentAt: open.SentAt}
	case err != nil && op.Effect == addons.EffectPhysical:
		// Fail closed: not knowing whether one is open is not knowing that
		// sending again cannot act twice.
		return Result{}, fmt.Errorf("%w: it could not be checked, nothing dispatched: %v", ErrCallUnsettled, err)
	case err != nil:
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
		// The add-on's mutation log is an independent forensic record, and it
		// knows only the subject. Without this it would answer "who did what to
		// whom" with the whom alone — on the one path that carries a credential.
		Actor:  req.ActorID,
		Params: req.Params,
	})

	// An outcome nobody has heard of is indeterminate, never success; a
	// no_change reply is recorded as no_change, never as done.
	status := addons.StatusOf(resp)

	result := Result{OperationID: id, Status: status, Outcome: resp.Outcome, Err: resp.Err,
		NoChange: status == db.AddonOpNoChange, Code: resp.Code, Refusal: resp.Refusal}
	if op.Effect == addons.EffectPhysical && resp.Outcome == addons.OutcomeSucceeded {
		v := addons.ViewOf(req.Target, req.Operation, resp.Body)
		result.View = &v
	}
	// Only on success, and never fatally. A body that will not parse is not a
	// reason to un-succeed a call the target confirmed; it is a reason for the
	// caller to see a zero and decide.
	if resp.Outcome == addons.OutcomeSucceeded {
		result.AccountID = AccountIDOf(resp.Body)
	}
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

// Preview asks the add-on what a physical operation would do (design §4
// physical rule 1), under the same checks as the confirm that may follow:
// resolvable, physical, its subject bound to its actor, its parameters
// valid. Nothing is recorded, counted or claimed, and nothing carries a
// call_id: a preview is not an attempt at anything, so it spends no rate
// limit and leaves no row (addons.Preview).
func Preview(ctx context.Context, req Request) (addons.ReadReply, error) {
	op, err := resolveOperation(req.Target, req.Operation)
	if err != nil {
		return addons.ReadReply{}, err
	}
	if op.Effect != addons.EffectPhysical {
		return addons.ReadReply{}, fmt.Errorf("%w: %s/%s", addons.ErrNotPhysical, req.Target, req.Operation)
	}
	if err := bindSubjectToActor(op.Scope, req.ActorID, req.SubjectID); err != nil {
		return addons.ReadReply{}, fmt.Errorf("%w (%s/%s)", err, req.Target, req.Operation)
	}
	if err := validateParams(op, req.Params); err != nil {
		return addons.ReadReply{}, err
	}
	return previewAddon(ctx, req.Target, req.Operation, req.SubjectID, req.ActorID, req.Params), nil
}
