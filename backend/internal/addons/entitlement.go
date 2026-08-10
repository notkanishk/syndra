package addons

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
)

// The entitlement leg of the transport (design §4).
//
// Separate from `Call` on purpose. An operation is an event carrying a secret,
// so that path is built around a durable record minted before the dispatch and
// a one-shot token that cannot authorise a second one. An entitlement apply is
// level-triggered desired state: it carries no secret, it is queued in the
// outbox, and re-sending it converges rather than duplicating. Forcing both
// through one function would mean either the apply borrowed a record it does
// not need, or the operation lost the record it cannot do without.
//
// What they DO share is the transport: the same mutual TLS or signed request,
// the same breaker, the same four outcomes, the same refusal to follow a
// redirect. That is `doAuthenticated`, and both legs go through it, so no
// second authentication story can grow beside the first.

// ErrNoFingerprint is the refusal to dispatch an apply that verifies nothing.
// An absent fingerprint passes the add-on's check vacuously, which turns "the
// diff you approved is the diff that lands" into a property callers opt into.
var ErrNoFingerprint = errors.New("addon: refusing to dispatch an apply with no fingerprint to verify")

// ApplyRequest is one subject's resolved desired state, on its way to a target.
type ApplyRequest struct {
	Target  string
	Subject string
	Email   string
	// Fingerprint is the target state the plan was computed against. The add-on
	// re-verifies it immediately before writing and refuses if the subject
	// moved — verification at the backend would be a statement about a moment
	// that has already passed by the time the drain runs.
	Fingerprint string
	// CallID deduplicates. Unlike an operation's record it is not a durable row
	// here: the outbox row IS the durable intent, and minting a second record
	// beside it would be two accounts of one decision, free to disagree.
	CallID string
	// PlanID is the approval this apply executes. Inside the signed body like
	// everything else that binds the call: a call replayed against another plan
	// must fail verification rather than succeed under somebody else's approval.
	PlanID string
	// Actor is who decided this, carried through to the add-on's mutation log.
	// That log promises who did what to whom and the add-on knows only the whom.
	Actor string
	// Desired is the resolved set, by field. Encoded as raw JSON so this layer
	// stays ignorant of what any field means — Syndra decides who and what, the
	// add-on decides how.
	Desired map[string]json.RawMessage
}

// ApplyResponse is what the target did.
type ApplyResponse struct {
	Outcome Outcome
	Status  int
	Err     error
	// Effect and Username come back from the add-on. `Username` is why account
	// creation needs no separate operation sequenced before the apply: the
	// derived name is reported by the convergence that created it.
	Effect   string `json:"effect"`
	Username string `json:"username"`
	// UID is the target's stable identity for the account. Recorded beside the
	// name because the name can move out of band and this cannot.
	UID    int64  `json:"uid"`
	Detail string `json:"detail"`
	// Fingerprint is the subject's state afterwards, so the next plan starts
	// from something current rather than from a read the backend has to make.
	Fingerprint string `json:"fingerprint"`
	// Code is the add-on's own error code on a refusal, from a closed set it
	// declares. Read for one reason: `PLAN_STALE` is the refusal that means the
	// subject moved since the diff was approved, and the operator action for it
	// is a re-plan somebody reads — not "fix it and retry", which is what every
	// other refusal means.
	Code string `json:"error"`
	// LifecycleRefusal says the add-on is draining or read-only. Carried apart
	// from the outcome for the reason `CallResponse` carries it apart: one says
	// what happened to the work, the other says whether the target is healthy.
	LifecycleRefusal bool
}

// CodePlanStale is the add-on refusing because live state no longer matches the
// fingerprint the plan was computed against.
const CodePlanStale = "PLAN_STALE"

// applyEnvelope is the wire body. Everything binding the call to an approval
// travels inside it rather than in headers, so in signed mode the signature
// covers them — an intercepted call replayed against a different subject fails
// verification rather than succeeding under somebody else's approval.
type applyEnvelope struct {
	ContractVersion int                        `json:"contract_version"`
	CallID          string                     `json:"call_id"`
	Subject         string                     `json:"subject"`
	Email           string                     `json:"email,omitempty"`
	Fingerprint     string                     `json:"fingerprint,omitempty"`
	PlanID          string                     `json:"plan_id,omitempty"`
	Actor           string                     `json:"actor,omitempty"`
	Desired         map[string]json.RawMessage `json:"desired"`
}

// Apply dispatches one subject's desired state.
//
// It returns no `error` beside the outcome, deliberately, for the reason `Call`
// does not: `err == nil` is a wrong proxy for success here — an add-on can
// answer, refuse, and be entirely healthy — and a wrong proxy that reads like
// the right one is worse than none.
func Apply(ctx context.Context, req ApplyRequest) ApplyResponse {
	a, err := Get(req.Target)
	if err != nil {
		return ApplyResponse{Outcome: OutcomeUnreached, Err: err}
	}
	if req.CallID == "" {
		// The add-on deduplicates on this and the design leans on it: §16
		// declines a nonce store because the call id already prevents replay,
		// and that argument only holds if nothing dispatches without one.
		return ApplyResponse{Outcome: OutcomeUnreached, Err: ErrNoCallRecord}
	}
	if req.Fingerprint == "" {
		// The add-on refuses this too, and both ends check on purpose: the
		// add-on's refusal protects the target, and this one keeps a row that
		// could never verify from spending a dispatch and a retry to learn it.
		return ApplyResponse{Outcome: OutcomeUnreached, Err: ErrNoFingerprint}
	}
	if !a.br.allow(timeNow()) {
		return ApplyResponse{Outcome: OutcomeUnreached, Err: fmt.Errorf("%w: %s", ErrCircuitOpen, req.Target)}
	}
	cred, err := credentialFor(a.Registration)
	if err != nil {
		return ApplyResponse{Outcome: OutcomeUnreached, Err: fmt.Errorf("addon %s: %w", req.Target, err)}
	}

	body, err := json.Marshal(applyEnvelope{
		ContractVersion: ContractVersion,
		CallID:          req.CallID,
		Subject:         req.Subject,
		Email:           req.Email,
		Fingerprint:     req.Fingerprint,
		PlanID:          req.PlanID,
		Actor:           req.Actor,
		Desired:         req.Desired,
	})
	if err != nil {
		return ApplyResponse{Outcome: OutcomeUnreached, Err: fmt.Errorf("addon %s: encode apply: %w", req.Target, err)}
	}

	resp := doAuthenticated(ctx, cred, http.MethodPost, a.Registration.BaseURL+"/apply", body, callTimeout)
	a.br.record(timeNow(), resp)

	out := ApplyResponse{
		Outcome: resp.Outcome, Status: resp.Status, Err: resp.Err,
		LifecycleRefusal: resp.LifecycleRefusal,
	}
	if resp.Outcome == OutcomeSucceeded {
		// A 2xx whose body will not decode is not a success the backend can
		// record: the add-on acted and did not say what it did, which is the
		// definition of indeterminate.
		if err := json.Unmarshal(resp.Body, &out); err != nil {
			out.Outcome, out.Err = OutcomeIndeterminate, fmt.Errorf("addon %s: decode apply outcome: %w", req.Target, err)
		}
	}
	if resp.Outcome == OutcomeRejected {
		// Only the code and the detail, and only into fields that already
		// exist. A refusal body is whatever the least trusted component chose
		// to send; a failure to decode one leaves the refusal exactly as
		// classified rather than reclassifying it.
		var refusal struct {
			Code   string `json:"error"`
			Detail string `json:"detail"`
		}
		if err := json.Unmarshal(resp.Body, &refusal); err == nil {
			out.Code, out.Detail = refusal.Code, refusal.Detail
		}
	}
	if out.Outcome != OutcomeSucceeded {
		// Neither body is logged. The request carries a resolved entitlement
		// set — not a secret, but a person's access — and the response carries
		// whatever the least trusted component chose to send back.
		log.Printf("[ADDON] %s/apply subject=%s call_id=%s outcome=%s status=%d err=%v",
			req.Target, req.Subject, req.CallID, out.Outcome, out.Status, out.Err)
	}
	return out
}
