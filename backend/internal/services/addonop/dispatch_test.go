package addonop

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"testing"
	"time"

	"syndra/internal/addons"
	"syndra/internal/db"
)

const theSecret = "correct-horse-battery-staple"

// harness records what each seam was asked and, crucially, the ORDER. The whole
// content of this protocol is an ordering, and an ordering is the one thing a
// per-call assertion cannot check.
type harness struct {
	mu    sync.Mutex
	steps []string

	op         addons.EffectiveOperation
	resolveErr error

	beginID  string
	beginErr error
	begun    []db.AddonOperationParams

	// recentOps is what the rate limiter sees. Zero by default: every test in
	// this file is about something else, and a limiter that fired would make
	// them all pass for the wrong reason.
	recentOps    int
	recentOpsErr error
	rateQueries  []string

	verified []string

	resp      addons.CallResponse
	calls     []addons.CallRequest
	settleErr error
	settled   []string
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	h := &harness{
		// Mirrors the real policy entry for password.set, schema included. A
		// default operation with no declared parameters would make every test
		// here fail validation, which is the validator working — but it would
		// also mean nothing else in this file ever ran.
		op: addons.EffectiveOperation{
			ID: "password.set", Scope: addons.ScopeMember, Available: true,
			SecretParams: []string{"password"},
			Params:       []addons.ParamSpec{{Name: "password", Type: "string", Required: true, Secret: true}},
		},
		recentOps: 0,
		beginID:   "rec-0001",
		resp:      addons.CallResponse{Outcome: addons.OutcomeSucceeded, Status: 200},
	}

	sr, sv, sn, sb, sc, ss := resolveOperation, validateParams, operationRecord, beginOperation, callAddon, settleOperation
	scr := countRecentOperations
	countRecentOperations = func(_ context.Context, subject, operation string, _ time.Duration) (int, error) {
		h.record("rate")
		h.mu.Lock()
		h.rateQueries = append(h.rateQueries, subject+"|"+operation)
		h.mu.Unlock()
		return h.recentOps, h.recentOpsErr
	}
	resolveOperation = func(target, id string) (addons.EffectiveOperation, error) {
		h.record("resolve")
		return h.op, h.resolveErr
	}
	validateParams = func(op addons.EffectiveOperation, params map[string]any) error {
		h.record("validate")
		return addons.ValidateParams(op, params)
	}
	// Returns the zero token. A test in this package cannot mint a real one —
	// DispatchRecord's fields are unexported, which is exactly the property
	// under test — so what is asserted here is that the token is minted for the
	// right call, in the right place. That it is then claimed at the moment of
	// dispatch, once, is asserted where it can be: in the transport's package.
	operationRecord = func(id, target, operation, subject string) addons.DispatchRecord {
		h.record("mint")
		h.mu.Lock()
		h.verified = append(h.verified, strings.Join([]string{id, target, operation, subject}, "|"))
		h.mu.Unlock()
		return addons.DispatchRecord{}
	}
	beginOperation = func(_ context.Context, p db.AddonOperationParams) (string, error) {
		h.record("begin")
		h.mu.Lock()
		h.begun = append(h.begun, p)
		h.mu.Unlock()
		return h.beginID, h.beginErr
	}
	callAddon = func(_ context.Context, r addons.CallRequest) addons.CallResponse {
		h.record("call")
		h.mu.Lock()
		h.calls = append(h.calls, r)
		h.mu.Unlock()
		return h.resp
	}
	settleOperation = func(_ context.Context, id, status string) error {
		h.record("settle:" + status)
		h.mu.Lock()
		h.settled = append(h.settled, id+"="+status)
		h.mu.Unlock()
		return h.settleErr
	}
	t.Cleanup(func() {
		resolveOperation, validateParams, operationRecord, beginOperation, callAddon, settleOperation = sr, sv, sn, sb, sc, ss
		countRecentOperations = scr
	})
	return h
}

func (h *harness) record(step string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.steps = append(h.steps, step)
}

func (h *harness) order() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return strings.Join(h.steps, " -> ")
}

func passwordSet() Request {
	return Request{
		Target:    "truenas",
		Operation: "password.set",
		ActorID:   "user-42",
		SubjectID: "user-42",
		Params:    map[string]any{"password": theSecret},
	}
}

// 2.13 — the record is committed before the call, not after it. A record
// written afterwards is missing for exactly the dispatches that need it: the
// ones where the backend died mid-call. The parameters are never retained, so a
// missing record cannot be reconstructed from anything.
func TestTheRecordIsCommittedBeforeTheCall(t *testing.T) {
	h := newHarness(t)

	res, err := Dispatch(context.Background(), passwordSet())
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if got := h.order(); got != "resolve -> validate -> rate -> begin -> mint -> call -> settle:succeeded" {
		t.Fatalf("protocol order = %q", got)
	}
	if res.OperationID != "rec-0001" {
		t.Fatalf("result lost the record id: %+v", res)
	}
	if len(h.verified) != 1 || h.verified[0] != "rec-0001|truenas|password.set|user-42" {
		t.Fatalf("the record must be read back and checked against this exact call, got %v", h.verified)
	}
}

// 2.13 — the record is a precondition for the call, not a log of it. If it
// cannot be committed, nothing is dispatched.
func TestNoRecordMeansNothingIsDispatched(t *testing.T) {
	h := newHarness(t)
	h.beginErr = errors.New("database is down")

	res, err := Dispatch(context.Background(), passwordSet())
	if err == nil {
		t.Fatal("a failed record write must fail the dispatch")
	}
	if len(h.calls) != 0 {
		t.Fatalf("the add-on was called with no record committed: %+v", h.calls)
	}
	if res.OperationID != "" {
		t.Fatalf("a result naming a record that does not exist: %+v", res)
	}
}

// 2.13 — every outcome the transport can produce has a persisted status, and
// every one of those is a status the column accepts. A missing case would be
// written as the empty string and rejected by the CHECK, turning an unhandled
// outcome into a constraint violation on a dispatch that already happened.
func TestEveryDispatchOutcomeHasAnAcceptedStatus(t *testing.T) {
	accepted := map[string]bool{}
	for _, s := range db.AddonOperationStatuses {
		accepted[s] = true
	}
	for _, o := range addons.AllOutcomes {
		status, ok := statusFor[o]
		if !ok {
			t.Errorf("outcome %q has no persisted status", o)
			continue
		}
		if !accepted[status] {
			t.Errorf("outcome %q maps to %q, which the status CHECK does not accept", o, status)
		}
		if status == db.AddonOpDispatching {
			t.Errorf("outcome %q maps to the non-terminal status; an answered call must settle", o)
		}
	}
	if len(statusFor) != len(addons.AllOutcomes) {
		t.Errorf("statusFor has %d entries for %d outcomes — one of them maps something that cannot occur",
			len(statusFor), len(addons.AllOutcomes))
	}
}

// 2.13, 2.14 — each outcome is recorded as itself, and the add-on is called
// exactly once whatever comes back. No retry: a retry needs the parameters, the
// parameters are the secret, and keeping the secret to enable the retry is the
// vault this design exists to avoid.
func TestOutcomeIsRecordedAndTheCallIsNeverRepeated(t *testing.T) {
	// The wanted status is spelled out here rather than read from statusFor.
	// Taking it from the map under test would compare the implementation to
	// itself and pass for any mapping at all, including one that records an
	// indeterminate dispatch as a success.
	cases := []struct {
		outcome addons.Outcome
		want    string
	}{
		{addons.OutcomeSucceeded, "succeeded"},
		{addons.OutcomeRejected, "rejected"},
		{addons.OutcomeUnreached, "unreached"},
		{addons.OutcomeIndeterminate, "indeterminate"},
	}
	if len(cases) != len(addons.AllOutcomes) {
		t.Fatalf("this table covers %d of %d outcomes; a new one needs a row and a decision, not a default",
			len(cases), len(addons.AllOutcomes))
	}

	for _, tc := range cases {
		t.Run(string(tc.outcome), func(t *testing.T) {
			h := newHarness(t)
			h.resp = addons.CallResponse{Outcome: tc.outcome, Err: fmt.Errorf("as it happened")}
			if tc.outcome == addons.OutcomeSucceeded {
				h.resp.Err = nil
			}

			res, err := Dispatch(context.Background(), passwordSet())
			if err != nil {
				t.Fatalf("Dispatch: %v", err)
			}
			if len(h.calls) != 1 {
				t.Fatalf("the add-on was called %d times for outcome %q", len(h.calls), tc.outcome)
			}
			if res.Status != tc.want || h.settled[0] != "rec-0001="+tc.want {
				t.Fatalf("outcome %q recorded as %q / %v, want %q", tc.outcome, res.Status, h.settled, tc.want)
			}

			// The contract that makes all of this readable: a nil error does
			// not mean the target did what was asked. The answer is the
			// outcome, and nothing else may stand in for it.
			if tc.outcome != addons.OutcomeSucceeded && res.Err == nil {
				t.Fatal("a non-success outcome carried no explanation")
			}
		})
	}
}

// 2.13 — an outcome this package has never heard of is not evidence of success.
// The fallback exists for a transport that grows a fifth state before this
// package learns about it, and the only safe reading is the one that puts the
// row in front of a human.
func TestAnUnrecognisedOutcomeIsRecordedAsUnresolved(t *testing.T) {
	h := newHarness(t)
	h.resp = addons.CallResponse{Outcome: addons.Outcome("something-new")}

	res, err := Dispatch(context.Background(), passwordSet())
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if res.Status != db.AddonOpIndeterminate {
		t.Fatalf("an unrecognised outcome was recorded as %q; the safe reading is %q",
			res.Status, db.AddonOpIndeterminate)
	}
	if !(db.AddonOperation{Status: res.Status}).Unresolved() {
		t.Fatal("the fallback status must land on the unresolved surface")
	}
}

// 2.14 — a crash between the dispatch and the terminal write leaves the row
// non-terminal, and the protocol says so rather than reporting an outcome it
// did not manage to persist. The call already happened; failing to record it
// does not un-happen it.
func TestAFailedTerminalWriteLeavesTheRowUnresolved(t *testing.T) {
	h := newHarness(t)
	h.resp = addons.CallResponse{Outcome: addons.OutcomeSucceeded, Status: 200}
	h.settleErr = errors.New("connection reset")

	res, err := Dispatch(context.Background(), passwordSet())
	if err == nil {
		t.Fatal("an unrecorded outcome must be reported, not swallowed")
	}
	if res.Status != db.AddonOpDispatching {
		t.Fatalf("status = %q; the row is still non-terminal and the result must say so", res.Status)
	}
	if !strings.Contains(err.Error(), "rec-0001") {
		t.Errorf("the error must name the record a human now has to resolve: %v", err)
	}
	if len(h.calls) != 1 {
		t.Fatalf("the call was repeated after a failed settle (%d calls)", len(h.calls))
	}
	if strings.Count(h.order(), "settle") != 1 {
		t.Fatalf("the settle was retried: %q", h.order())
	}
}

// 2.14 — the secret reaches the add-on and nothing else. Not by convention: the
// record type has no field to put it in, and this asserts the consequence.
func TestNoSecretReachesTheRecordOrTheLog(t *testing.T) {
	var buf bytes.Buffer
	saved := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(saved) })

	h := newHarness(t)
	h.resp = addons.CallResponse{Outcome: addons.OutcomeRejected, Status: 400, Err: errors.New("addon returned 400")}

	res, err := Dispatch(context.Background(), passwordSet())
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	for _, p := range h.begun {
		if strings.Contains(fmt.Sprintf("%+v", p), theSecret) {
			t.Fatalf("the committed record carried the secret: %+v", p)
		}
	}
	if strings.Contains(buf.String(), theSecret) {
		t.Fatalf("the secret was logged: %s", buf.String())
	}
	if strings.Contains(fmt.Sprintf("%+v", res), theSecret) {
		t.Fatal("the result echoed the secret back to its caller")
	}

	// It did reach the add-on, or the operation would not have happened at all.
	if got, _ := h.calls[0].Params["password"].(string); got != theSecret {
		t.Fatal("the secret must still be delivered to the add-on")
	}
}

// 2.4, 2.13 — an operation the effective set does not offer leaves no record. It
// was never a legitimate attempt, and a record for it would put a row on the
// unresolved surface for something that never reached the network.
func TestAnUncallableOperationLeavesNoRecord(t *testing.T) {
	h := newHarness(t)
	h.resolveErr = addons.ErrUnknownOperation

	if _, err := Dispatch(context.Background(), passwordSet()); !errors.Is(err, addons.ErrUnknownOperation) {
		t.Fatalf("err = %v, want ErrUnknownOperation", err)
	}
	if len(h.begun) != 0 || len(h.calls) != 0 {
		t.Fatalf("an uncallable operation wrote a record or dispatched: begun=%d calls=%d", len(h.begun), len(h.calls))
	}
}

// 2.32 — backend policy says the backend refuses a confirmation-required
// operation without one. A confirmation only the frontend enforces is a
// suggestion, and account.purge is irreversible.
func TestAConfirmationRequiredOperationIsRefusedWithoutOne(t *testing.T) {
	h := newHarness(t)
	h.op = addons.EffectiveOperation{ID: "account.purge", Scope: addons.ScopeAdmin, Confirm: true, Available: true}

	req := passwordSet()
	req.Operation = "account.purge"
	req.Params = nil

	if _, err := Dispatch(context.Background(), req); !errors.Is(err, ErrConfirmationRequired) {
		t.Fatalf("err = %v, want ErrConfirmationRequired", err)
	}
	if len(h.begun) != 0 || len(h.calls) != 0 {
		t.Fatal("an unconfirmed irreversible operation wrote a record or dispatched")
	}

	req.Confirmed = true
	if _, err := Dispatch(context.Background(), req); err != nil {
		t.Fatalf("a confirmed operation must proceed: %v", err)
	}
	if len(h.calls) != 1 {
		t.Fatalf("the confirmed operation was not dispatched (%d calls)", len(h.calls))
	}
}

// 2.16 — the vocabulary an unresolved row is read back through. `dispatching`
// and `indeterminate` are unresolved; the other three are not, and a row that
// answered must never be presented as an open question.
func TestUnresolvedIsExactlyTheTwoStatesWithNoAnswer(t *testing.T) {
	unresolved := map[string]bool{}
	for _, s := range db.AddonUnresolvedStatuses {
		unresolved[s] = true
	}
	for _, s := range db.AddonOperationStatuses {
		row := db.AddonOperation{Status: s}
		if row.Unresolved() != unresolved[s] {
			t.Errorf("status %q: Unresolved() = %t, but AddonUnresolvedStatuses says %t",
				s, row.Unresolved(), unresolved[s])
		}
	}
	if len(db.AddonUnresolvedStatuses) != 2 {
		t.Fatalf("unresolved must be exactly the two states carrying no answer, got %v", db.AddonUnresolvedStatuses)
	}
}

// 2.32, P1 — backend policy's parameter schema is enforced, and enforced before
// the record is written. An unknown key, a wrong type, or a missing required
// value is not an attempt at anything: recording it would put a row on the
// operator's surface for a call that never left the process, and sending it
// would let an add-on-specific input reach the target without passing the
// boundary that is supposed to bound it.
func TestParametersAreValidatedBeforeAnythingIsRecorded(t *testing.T) {
	cases := []struct {
		name   string
		params map[string]any
	}{
		{"unknown key", map[string]any{"password": theSecret, "shell": "/bin/sh"}},
		{"missing required", map[string]any{}},
		{"wrong type", map[string]any{"password": 5}},
		{"present but empty", map[string]any{"password": "   "}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.op = addons.EffectiveOperation{
				ID: "password.set", Scope: addons.ScopeMember, Available: true,
				SecretParams: []string{"password"},
				Params:       []addons.ParamSpec{{Name: "password", Type: "string", Required: true, Secret: true}},
			}
			req := passwordSet()
			req.Params = tc.params

			_, err := Dispatch(context.Background(), req)
			if !errors.Is(err, addons.ErrInvalidParams) {
				t.Fatalf("err = %v, want ErrInvalidParams", err)
			}
			if len(h.begun) != 0 || len(h.calls) != 0 {
				t.Fatalf("an invalid request wrote a record or dispatched: begun=%d calls=%d",
					len(h.begun), len(h.calls))
			}
			if strings.Contains(err.Error(), theSecret) || strings.Contains(err.Error(), "/bin/sh") {
				t.Fatalf("the refusal echoed a submitted value: %v", err)
			}
			if got := h.order(); strings.Contains(got, "begin") {
				t.Fatalf("validation must run before the record: %q", got)
			}
		})
	}
}

// P1 — a record that cannot be claimed surfaces as an unreached dispatch and
// settles as one, because that is what it is: nothing was sent. Minting itself
// can no longer fail, since it touches nothing — the claim lives at the call.
func TestAnUnclaimableRecordSettlesAsUnreached(t *testing.T) {
	h := newHarness(t)
	h.resp = addons.CallResponse{Outcome: addons.OutcomeUnreached, Err: addons.ErrNoCallRecord}

	res, err := Dispatch(context.Background(), passwordSet())
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if res.Status != db.AddonOpUnreached {
		t.Fatalf("status = %q, want %q", res.Status, db.AddonOpUnreached)
	}
	if h.settled[0] != "rec-0001="+db.AddonOpUnreached {
		t.Fatalf("settled as %v", h.settled)
	}
}

// 2.43/2.44 — scope decides who may invoke an operation, and it must also
// decide on whom.
//
// Without this, "scoped to member" means only "a member may call this", and
// `password.set` with somebody else's subject id resets their storage
// credential. The check is here rather than in the manifest (the least trusted
// input in the system) or the policy table (which describes operations, not
// requests): "who is this call about" is a property of the request and exists
// nowhere else.
func TestAMemberScopedOperationActsOnlyOnTheActor(t *testing.T) {
	// Each case asserts the REASON as well as the refusal. A blank identifier
	// that falls through to the mismatch comparison is refused by accident —
	// correct today, and silently wrong the moment somebody makes the
	// comparison tolerant of an empty value. The distinct reasons are also what
	// an operator reading the log needs: "nobody was authenticated" and "you
	// named somebody else" are different incidents.
	for _, tc := range []struct {
		name           string
		actor, subject string
		reason         string
		why            string
	}{
		{"another member's subject", "user-42", "user-99", "may only act on the person invoking it",
			"this is the whole attack: one member resetting another's credential"},
		{"no authenticated actor", "", "user-99", "no authenticated actor",
			"an unauthenticated caller must not be able to name anybody"},
		{"no subject", "user-42", "", "no subject",
			"two absences matching is not a match"},
		{"neither", "", "", "no authenticated actor",
			"and least of all when both are absent"},
		{"an actor that is only whitespace", "   ", "user-99", "no authenticated actor",
			"a blank is a blank however it is spelled"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			req := passwordSet()
			req.ActorID, req.SubjectID = tc.actor, tc.subject

			res, err := Dispatch(context.Background(), req)
			if !errors.Is(err, ErrSubjectNotActor) {
				t.Fatalf("want ErrSubjectNotActor, got %v — %s", err, tc.why)
			}
			if !strings.Contains(err.Error(), tc.reason) {
				t.Errorf("want the refusal to say %q, got %q", tc.reason, err.Error())
			}
			// Refused before the record and therefore before the network: an
			// illegitimate call must leave no row on an operator's surface and
			// must certainly not reach the add-on.
			if h.order() != "resolve" {
				t.Errorf("nothing may happen after the refusal, got %q", h.order())
			}
			if res.OperationID != "" || len(h.calls) != 0 || len(h.begun) != 0 {
				t.Error("a refused call must record nothing and send nothing")
			}
			// Neither identifier is echoed: a refusal is not a lookup service
			// for which subject ids exist.
			for _, id := range []string{tc.actor, tc.subject} {
				if id != "" && strings.Contains(err.Error(), id) {
					t.Errorf("the refusal must not name %q", id)
				}
			}
		})
	}
}

// An admin-scoped operation is about somebody else by definition — that is what
// admin scope means — so the binding must not reach it.
func TestAnAdminScopedOperationMayNameAnotherSubject(t *testing.T) {
	h := newHarness(t)
	h.op.Scope = addons.ScopeAdmin
	req := passwordSet()
	req.SubjectID = "user-99"

	if _, err := Dispatch(context.Background(), req); err != nil {
		t.Fatalf("an admin-scoped operation must be able to act on another subject: %v", err)
	}
	if len(h.calls) != 1 || h.calls[0].Subject != "user-99" {
		t.Fatalf("the named subject must reach the add-on: %+v", h.calls)
	}
}

// A manifest cannot defeat the check by declaring no subject constraint, and it
// cannot reach it by declaring itself member-scoped either: the effective scope
// is policy ∩ manifest with policy winning, so a manifest can only narrow.
func TestAManifestCannotWidenOrEvadeTheSubjectBinding(t *testing.T) {
	// Policy says admin; a manifest claiming member resolves to admin, which is
	// asserted in the addons package. What is asserted here is that the binding
	// reads the RESOLVED scope rather than anything a manifest supplies.
	resolved := addons.ResolveOperation
	_ = resolved

	h := newHarness(t)
	h.op.Scope = addons.ScopeMember
	req := passwordSet()
	req.SubjectID = "user-99"
	if _, err := Dispatch(context.Background(), req); !errors.Is(err, ErrSubjectNotActor) {
		t.Fatalf("the binding must apply wherever the resolved scope is member: %v", err)
	}
}

// 2.49/2.50 — a member can drive the credential path at will, and it terminates
// in a single rate-limited session the add-on shares with every other
// operation. Repeated resets are a cheap way to wedge the target for everybody.
func TestAMemberIsRateLimitedPerSubjectBeforeAnythingIsSent(t *testing.T) {
	t.Setenv("ADDON_MEMBER_OP_LIMIT", "3")
	h := newHarness(t)
	h.recentOps = 3

	res, err := Dispatch(context.Background(), passwordSet())
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("want ErrRateLimited, got %v", err)
	}
	if res.OperationID != "" || len(h.begun) != 0 || len(h.calls) != 0 {
		t.Fatal("the refusal must come before the record and therefore before the network")
	}
	// Per subject and per operation. A global counter would let one member's
	// retries lock out everybody else's first attempt.
	if len(h.rateQueries) != 1 || h.rateQueries[0] != "user-42|password.set" {
		t.Errorf("the count must be scoped to this subject and this operation: %v", h.rateQueries)
	}
}

func TestOrdinaryUseDoesNotReachTheLimit(t *testing.T) {
	t.Setenv("ADDON_MEMBER_OP_LIMIT", "3")
	h := newHarness(t)
	h.recentOps = 2

	if _, err := Dispatch(context.Background(), passwordSet()); err != nil {
		t.Fatalf("below the limit must dispatch: %v", err)
	}
	if len(h.calls) != 1 {
		t.Fatal("the call must be sent")
	}
}

// Fail closed. The limit exists because the path terminates in a shared,
// rate-limited session on the target; letting it through because the counter
// could not be read spends exactly the resource the limit protects.
func TestAnUnreadableCounterRefusesRatherThanWavesThrough(t *testing.T) {
	h := newHarness(t)
	h.recentOpsErr = errors.New("db down")

	if _, err := Dispatch(context.Background(), passwordSet()); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("want ErrRateLimited, got %v", err)
	}
	if len(h.calls) != 0 {
		t.Fatal("nothing may be sent when the budget is unknown")
	}
}

// An operator path is behind operator authentication and is rate-limited by
// there being an operator on the other end. Counting it would spend a database
// read per admin operation to enforce a limit nobody can reach.
func TestAnAdminScopedOperationIsNotCounted(t *testing.T) {
	h := newHarness(t)
	h.op.Scope = addons.ScopeAdmin
	h.recentOps = 9999

	if _, err := Dispatch(context.Background(), passwordSet()); err != nil {
		t.Fatalf("an admin operation must not be rate-limited per subject: %v", err)
	}
	if len(h.rateQueries) != 0 {
		t.Errorf("and must not even ask: %v", h.rateQueries)
	}
}
