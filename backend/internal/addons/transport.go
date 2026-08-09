package addons

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Outcome is what the backend knows about a dispatched call. Four states, not
// two, because "did it work" has three honest answers and the third is the one
// that matters.
//
// The mapping from wire event to outcome is the whole safety property of this
// layer: an outcome is chosen by what the ADD-ON may have done, never by what
// is convenient to report. When the evidence is ambiguous the pessimistic
// reading wins, because being wrong towards Indeterminate costs an operator a
// glance and being wrong towards Unreached costs a duplicated mutation.
type Outcome string

const (
	// OutcomeSucceeded — the add-on applied it and said so.
	OutcomeSucceeded Outcome = "succeeded"
	// OutcomeRejected — the add-on received the call, refused it, and did not
	// act. Deterministic: an identical retry fails identically, so retrying is
	// pointless rather than dangerous.
	OutcomeRejected Outcome = "rejected"
	// OutcomeUnreached — the call never arrived. Nothing happened on the
	// target, so the intent is still exactly true and the row stays queued.
	OutcomeUnreached Outcome = "unreached"
	// OutcomeIndeterminate — the call was sent and the answer was lost. The
	// add-on may have acted. This is the state that must never be auto-retried
	// for a secret-bearing operation and must never be counted as either
	// success or failure.
	OutcomeIndeterminate Outcome = "indeterminate"
)

// Terminal reports whether the outcome settles the call. Indeterminate is not
// terminal: it is the unresolved surface, awaiting a human.
func (o Outcome) Terminal() bool { return o == OutcomeSucceeded || o == OutcomeRejected }

// Retryable reports whether the call may be safely dispatched again. Only when
// the backend knows the add-on never saw it.
func (o Outcome) Retryable() bool { return o == OutcomeUnreached }

// ErrCircuitOpen is returned while a target's breaker is open. It is an
// Unreached outcome, not a failure: nothing was dispatched, so the intent is
// intact and the row stays queued.
var ErrCircuitOpen = errors.New("addon: circuit is open for this target")

// ErrNoCallRecord is the refusal to dispatch without an operation record id.
// The record exists so a crash between dispatch and response leaves evidence;
// dispatching without one would leave a mutation nothing in the database knows
// was attempted.
var ErrNoCallRecord = errors.New("addon: refusing to dispatch without an operation record id")

// SignatureHeader carries the request signature in signed mode.
const SignatureHeader = "X-Syndra-Addon-Signature"

// maxResponseBytes bounds what the backend will read back. The add-on is the
// least trusted component; an unbounded read from it is a denial of service
// against the backend that governs every other target.
const maxResponseBytes = 1 << 20

// CallRequest is one dispatch to an add-on.
//
// It does NOT carry the secret parameter list. That comes from the effective
// operation — policy ∩ manifest — resolved inside Call, so a caller cannot
// widen what is loggable by omitting a name.
type CallRequest struct {
	Target    string
	Operation string
	// CallID is the addon_operations record id, committed before this call is
	// made. It doubles as the add-on's idempotency key.
	CallID  string
	Subject string
	// PlanID and Fingerprint bind an entitlement apply to what was reviewed.
	// Empty for one-shot operations, which are approved by confirmation at the
	// moment of invocation rather than rehearsed against a state read.
	PlanID      string
	Fingerprint string
	Params      map[string]any
}

// String redacts. Both this and GoString exist so that the two verbs a future
// caller reaches for without thinking — %v and %#v — cannot print a password.
// A redaction that depends on every caller remembering to redact is not one.
func (r CallRequest) String() string {
	return fmt.Sprintf("addon call target=%s operation=%s call_id=%s subject=%s plan=%s params=%v",
		r.Target, r.Operation, r.CallID, r.Subject, r.PlanID,
		RedactedParams(r.Target, r.Operation, r.Params))
}

// GoString mirrors String so %#v is no more revealing than %v.
func (r CallRequest) GoString() string { return r.String() }

// CallResponse is deliberately the only return value of Call: there is no error
// to check, because "err == nil" is a wrong proxy for success here and a wrong
// proxy that reads like the right one is worse than no proxy at all. Callers
// switch on Outcome. Err explains it.
type CallResponse struct {
	Outcome Outcome
	Status  int
	Body    []byte
	Err     error
}

// callEnvelope is the wire body. Everything that binds the call to an approval
// travels inside it rather than in headers, so in signed mode the signature
// covers them: a call intercepted and replayed against a different plan or a
// different subject fails verification rather than succeeding under someone
// else's approval.
type callEnvelope struct {
	ContractVersion int            `json:"contract_version"`
	CallID          string         `json:"call_id"`
	Operation       string         `json:"operation"`
	Subject         string         `json:"subject,omitempty"`
	PlanID          string         `json:"plan_id,omitempty"`
	Fingerprint     string         `json:"fingerprint,omitempty"`
	Params          map[string]any `json:"params,omitempty"`
}

// Call dispatches one operation to its add-on.
//
// The order is not incidental. Callability is resolved first, so an operation
// the effective set does not offer never reaches the network; the record id is
// checked next, so a dispatch can never outrun its own audit trail; the breaker
// is consulted before the credential is loaded, so an add-on that is known down
// costs nothing.
func Call(ctx context.Context, req CallRequest) CallResponse {
	op, err := ResolveOperation(req.Target, req.Operation)
	if err != nil {
		return CallResponse{Outcome: OutcomeUnreached, Err: err}
	}
	if strings.TrimSpace(req.CallID) == "" {
		return CallResponse{Outcome: OutcomeUnreached, Err: ErrNoCallRecord}
	}
	a, err := Get(req.Target)
	if err != nil {
		return CallResponse{Outcome: OutcomeUnreached, Err: err}
	}
	if !a.br.allow(timeNow()) {
		return CallResponse{Outcome: OutcomeUnreached, Err: fmt.Errorf("%w: %s", ErrCircuitOpen, req.Target)}
	}
	cred, err := credentialFor(a.Registration)
	if err != nil {
		return CallResponse{Outcome: OutcomeUnreached, Err: fmt.Errorf("addon %s: %w", req.Target, err)}
	}

	body, err := json.Marshal(callEnvelope{
		ContractVersion: ContractVersion,
		CallID:          req.CallID,
		Operation:       req.Operation,
		Subject:         req.Subject,
		PlanID:          req.PlanID,
		Fingerprint:     req.Fingerprint,
		Params:          req.Params,
	})
	if err != nil {
		// Cannot happen with the params the policy layer permits, and is still
		// not swallowed: an unmarshalable parameter is a bug, not a target
		// outage, and reporting it as one would send an operator to the NAS.
		return CallResponse{Outcome: OutcomeUnreached, Err: fmt.Errorf("addon %s: encode call: %w", req.Target, err)}
	}

	url := a.Registration.BaseURL + "/operations/" + req.Operation
	resp := doAuthenticated(ctx, cred, http.MethodPost, url, body, callTimeout)
	a.br.record(timeNow(), resp.Outcome)

	if resp.Outcome != OutcomeSucceeded {
		// Neither the request body nor the response body is logged. The request
		// carries the secret; the response carries whatever the least trusted
		// component chose to echo back, which may be the same secret.
		log.Printf("[ADDON] %s/%s call_id=%s outcome=%s status=%d err=%v secret_params=%d",
			req.Target, req.Operation, req.CallID, resp.Outcome, resp.Status, resp.Err, len(op.SecretParams))
	}
	return resp
}

// doAuthenticated performs one authenticated request and classifies it. Shared
// by Call and the manifest read so both legs are authenticated by construction
// — a manifest fetched over an unauthenticated channel is a capability set an
// on-path attacker can edit, and capability is what the backend decides against.
func doAuthenticated(ctx context.Context, cred *credential, method, url string, body []byte, timeout time.Duration) CallResponse {
	if err := ctx.Err(); err != nil {
		// Already cancelled: nothing is sent, so nothing happened.
		return CallResponse{Outcome: OutcomeUnreached, Err: err}
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, rdr)
	if err != nil {
		return CallResponse{Outcome: OutcomeUnreached, Err: err}
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if cred.mode == "signed" {
		req.Header.Set(SignatureHeader, ComputeSignature(timeNow(), body, cred.signingKey))
	}

	httpResp, err := cred.client.Do(req)
	if err != nil {
		if dialFailed(err) {
			return CallResponse{Outcome: OutcomeUnreached, Err: err}
		}
		// Everything else — a timeout, a reset, a truncated response — happened
		// at or after the point the request could have been delivered. The
		// pessimistic reading is the only safe one.
		return CallResponse{Outcome: OutcomeIndeterminate, Err: err}
	}
	defer httpResp.Body.Close()

	respBody, readErr := io.ReadAll(io.LimitReader(httpResp.Body, maxResponseBytes))
	out := CallResponse{Status: httpResp.StatusCode, Body: respBody, Outcome: classifyStatus(httpResp.StatusCode)}
	if readErr != nil && out.Outcome == OutcomeSucceeded {
		// A 2xx whose body was cut off says the add-on acted but not what it
		// did. Not a success the backend can record.
		out.Outcome, out.Err = OutcomeIndeterminate, readErr
	}
	if out.Outcome != OutcomeSucceeded && out.Err == nil {
		out.Err = fmt.Errorf("addon returned %d", httpResp.StatusCode)
	}
	return out
}

// classifyStatus maps a status code to what the add-on may have done.
//
//   - 2xx: it acted and said so.
//   - 408, 5xx: it may have acted. A 500 raised after a partial mutation looks
//     exactly like one raised before validation, and the backend cannot tell
//     them apart from here.
//   - 429: backpressure. It explicitly did not act and expects to be asked
//     again, which makes this unreached rather than rejected.
//   - other 4xx: it validated the call and refused. Deterministic.
func classifyStatus(code int) Outcome {
	switch {
	case code >= 200 && code < 300:
		return OutcomeSucceeded
	case code == http.StatusTooManyRequests:
		return OutcomeUnreached
	case code == http.StatusRequestTimeout:
		return OutcomeIndeterminate
	case code >= 500:
		return OutcomeIndeterminate
	default:
		return OutcomeRejected
	}
}

// ComputeSignature returns the full SignatureHeader value: HMAC-SHA256 over
// "<unix_seconds>.<body>" under key, formatted "t=<ts>,v1=<hex>".
//
// The timestamp is inside the MAC input, not merely beside it, so it cannot be
// edited to extend a captured signature's life; the body is inside it, so the
// signature authenticates what was asked and not only who asked. This is the
// same construction the Zitadel Actions leg already verifies — one algorithm in
// the system, not two.
//
// A bare shared secret in a header is deliberately not offered as an
// alternative: it identifies the caller and binds nothing, so an intercepted
// call replays verbatim, forever.
func ComputeSignature(ts time.Time, body, key []byte) string {
	mac := hmac.New(sha256.New, key)
	fmt.Fprintf(mac, "%d", ts.Unix())
	mac.Write([]byte("."))
	mac.Write(body)
	return fmt.Sprintf("t=%d,v1=%x", ts.Unix(), mac.Sum(nil))
}

// breaker stops the backend from spending a request timeout per row against a
// target that is down. One per add-on.
type breaker struct {
	mu        sync.Mutex
	failures  int
	openUntil time.Time
}

// allow reports whether a call may be dispatched.
//
// When the cooldown expires the breaker simply allows again while keeping its
// failure count at the threshold, so the next failure re-opens immediately and
// a single success clears it. That is half-open behaviour without a half-open
// state: several concurrent calls may probe at once, which costs one round of
// timeouts and buys no extra state to get wrong.
func (b *breaker) allow(now time.Time) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.openUntil.IsZero() || !now.Before(b.openUntil)
}

// record folds one outcome into the breaker.
//
// A rejection does not count. A 4xx is a healthy add-on refusing a specific
// request, and letting one badly-formed operation open the breaker would turn a
// single operator's mistake into an outage for the whole target.
func (b *breaker) record(now time.Time, o Outcome) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if o == OutcomeSucceeded || o == OutcomeRejected {
		b.failures, b.openUntil = 0, time.Time{}
		return
	}
	b.failures++
	if b.failures >= breakerThreshold {
		b.openUntil = now.Add(breakerCooldown)
	}
}

// CircuitOpen reports whether this target's breaker is currently open, for the
// health surface. Registration and reachability stay separate states; this is
// the third one, and an operator seeing a target listed but not answering
// should be able to see why without reading a log.
func (a *Addon) CircuitOpen() bool { return !a.br.allow(timeNow()) }
