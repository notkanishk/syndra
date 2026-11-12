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
	"net/url"
	"strconv"
	"sync"
	"time"

	"syndra/internal/extcall"
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

// AllOutcomes is every outcome Call can return. It exists so that anything
// mapping outcomes onto another vocabulary — a persisted status, a summary
// column — can be checked for completeness by a test rather than by reading.
// A fifth outcome added without a mapping fails closed somewhere; the point is
// that it fails in CI instead.
var AllOutcomes = []Outcome{
	OutcomeSucceeded, OutcomeRejected, OutcomeUnreached, OutcomeIndeterminate,
}

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

// SignatureHeader carries the request signature in signed mode.
const SignatureHeader = "X-Syndra-Addon-Signature"

// maxResponseBytes bounds what the backend will read back. The add-on is the
// least trusted component; an unbounded read from it is a denial of service
// against the backend that governs every other target.
const maxResponseBytes = 1 << 20

// CallRequest is one dispatch to an add-on.
//
// It does NOT carry the secret parameter list. That comes from the effective
// operation — the union of policy's secret names and the manifest's, which is
// the more protective reading of both — resolved inside Call, so a caller cannot
// widen what is loggable by omitting a name.
type CallRequest struct {
	Target    string
	Operation string
	// Record is the verified durable row authorising this dispatch, obtained
	// from OperationRecord. It is a token rather than an id because a
	// non-empty string is not evidence that anything was committed: with a
	// string, "the record exists before the call" holds only for callers who
	// remember to make it hold.
	Record  DispatchRecord
	Subject string
	// Actor is who decided this. The add-on's mutation log promises who did what
	// to whom, and it knows only the whom — a record naming the subject alone
	// answers two thirds of the question the log exists to answer. Carried for
	// the same reason the apply carries it, and it is not a secret: it is the
	// operator id already on the `addon_operations` row.
	Actor string
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
	return fmt.Sprintf("addon call target=%s operation=%s call_id=%s subject=%s actor=%s plan=%s params=%v",
		r.Target, r.Operation, r.Record.callID, r.Subject, r.Actor, r.PlanID,
		RedactedParams(r.Target, r.Operation, r.Params))
}

// GoString mirrors String so %#v is no more revealing than %v.
func (r CallRequest) GoString() string { return r.String() }

// MarshalJSON is the third verb, and the one the other two miss. `Params` is
// exported and untagged, so `json.Marshal(req)` — a structured log line, an
// error payload, a debug dump — emitted the secret in full while %v and %#v
// were carefully redacting it.
func (r CallRequest) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Target      string         `json:"target"`
		Operation   string         `json:"operation"`
		CallID      string         `json:"call_id"`
		Subject     string         `json:"subject,omitempty"`
		Actor       string         `json:"actor,omitempty"`
		PlanID      string         `json:"plan_id,omitempty"`
		Fingerprint string         `json:"fingerprint,omitempty"`
		Params      map[string]any `json:"params,omitempty"`
	}{
		Target: r.Target, Operation: r.Operation, CallID: r.Record.callID,
		Subject: r.Subject, Actor: r.Actor, PlanID: r.PlanID, Fingerprint: r.Fingerprint,
		Params: RedactedParams(r.Target, r.Operation, r.Params),
	})
}

// CallResponse is deliberately the only return value of Call: there is no error
// to check, because "err == nil" is a wrong proxy for success here and a wrong
// proxy that reads like the right one is worse than no proxy at all. Callers
// switch on Outcome. Err explains it.
type CallResponse struct {
	Outcome Outcome
	Status  int
	Body    []byte
	Err     error
	// LifecycleRefusal is the add-on saying it is draining or read-only and
	// declined to act — not that it is unwell (design §18).
	//
	// Carried apart from the outcome because the two answer different
	// questions. The outcome says what may have happened to the work: nothing,
	// so the row stays queued. This says whether the target is healthy: it is,
	// and tripping the breaker on a maintenance window would report an add-on
	// that is answering as unreachable, hiding the state an operator set on
	// purpose behind one they did not.
	LifecycleRefusal bool
	// Code is the add-on's own code for a refusal: the problem's `code`
	// (contract 2), or a contract-1 add-on's `error`, read for classification
	// only (design §11). Empty on a 2xx, or on a refusal that named none.
	Code string
	// ShapeRefusal is the add-on refusing the request's shape, not its
	// content: CONTRACT_VERSION_MISMATCH, UNKNOWN_REQUEST_FIELD, or a 404 on a
	// route the backend called under a feature. Nothing was done, so it is
	// unreached, and the manifest is read again: the add-on is not the one the
	// backend thinks it is (a rollback, an add-on that dropped a feature).
	ShapeRefusal bool
	// StatsRefusal is a coded answer to /stats (NO_CURRENT_READ while the
	// add-on has not read its app yet, a window it cannot serve): the add-on
	// answering on purpose about its own data, never a breaker failure.
	StatsRefusal bool
	// RetryAfter is the reply's Retry-After in seconds (RFC 9110 §10.2.3,
	// delay-seconds only, as registry/errors.json has it): 0 when absent or
	// not an integer. A cooldown's countdown (FRESH_READ_COOLDOWN, 429).
	RetryAfter int
	// Refusal is what a refusal says beyond its code, for the person who
	// asked: which parameter, which limit. Zero on a 2xx.
	Refusal Refusal
	// NoChange is a 2xx whose outcome is no_change (feature calls): the
	// add-on answered that the state was already as asked and wrote nothing.
	// Outcome stays succeeded, the transport fact (it was answered, and the
	// state is as asked); the record says no_change (StatusOf).
	NoChange bool
	// Clock is the response's own date and when it was asked for: what dates a
	// kept copy by its age (ahead).
	Clock Clock
}

// Clock is what lets a copy be dated by its age instead of by the add-on's
// clock, as an HTTP cache does (RFC 9111 §4.2.3, Calculating Age). Date is the
// response's Date header (RFC 9110 §6.6.1), the add-on's clock at the moment
// it answered; Sent is this backend's clock when the request went out (the
// RFC's request_time). The zero Clock (no Date, or one that does not parse)
// dates nothing, and a taken_at is then held to this clock as sent.
type Clock struct {
	Date, Sent time.Time
}

// age is age_value = Date - taken_at: both are the add-on's own clock, so a
// skew between the clocks cancels out. Never negative: Date has one-second
// resolution, so a taken_at within a second after it is age 0 (ahead ignores a
// Date further before it).
func (c Clock) age(takenAt time.Time) time.Duration {
	return max(0, c.Date.Sub(takenAt))
}

// parseDate reads a Date header (RFC 9110 §5.6.7, http.ParseTime); the zero
// time when absent or not a date, never an invented one.
func parseDate(h http.Header) time.Time {
	t, err := http.ParseTime(h.Get("Date"))
	if err != nil {
		return time.Time{}
	}
	return t
}

// Refusal is a refusal's extension members (RFC 9457 §3.2) that name what was
// refused, never a value: Param is the parameter (UNKNOWN_RESOURCE,
// NOT_ENTITLED, RESOURCE_TAKEN), Limit, Unit and Count the limit reached
// (LIMIT_REACHED).
type Refusal struct {
	Param string `json:"param,omitempty"`
	Limit *int   `json:"limit,omitempty"`
	Unit  string `json:"unit,omitempty"`
	Count *int   `json:"count,omitempty"`
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
	Actor           string         `json:"actor,omitempty"`
	PlanID          string         `json:"plan_id,omitempty"`
	Fingerprint     string         `json:"fingerprint,omitempty"`
	Params          map[string]any `json:"params,omitempty"`
}

// Call dispatches one operation to its add-on.
//
// The order is not incidental. Callability is resolved first, so an operation
// the effective set does not offer never reaches the network; the parameters are
// checked against backend policy before anything leaves; the durable record is
// verified next, so a dispatch can never outrun its own audit trail; the breaker
// is consulted before the credential is loaded, so an add-on that is known down
// costs nothing.
func Call(ctx context.Context, req CallRequest) CallResponse {
	op, err := ResolveOperation(req.Target, req.Operation)
	if err != nil {
		return CallResponse{Outcome: OutcomeUnreached, Err: err}
	}
	// A read goes through Read alone, the one read path, as Dispatch refuses it.
	if op.Effect == EffectRead {
		return CallResponse{Outcome: OutcomeUnreached, Err: fmt.Errorf("%w: %s/%s", ErrReadNotCalled, req.Target, req.Operation)}
	}
	// Re-checked here as well as at the caller. The caller validates so that an
	// invalid request never becomes a durable record; this validates so that no
	// path to an add-on exists that skipped the schema, whoever adds it later.
	if err := ValidateParams(op, req.Params); err != nil {
		return CallResponse{Outcome: OutcomeUnreached, Err: err}
	}
	if !req.Record.valid() {
		return CallResponse{Outcome: OutcomeUnreached, Err: ErrNoCallRecord}
	}
	// The token was minted against a specific record; this is where that stops
	// being history. Without it the verification would apply to whatever call
	// was made at mint time and not to the one actually being sent, so a caller
	// could claim a record for a health check and dispatch a password under it.
	if !req.Record.authorises(req.Target, req.Operation, req.Subject) {
		return CallResponse{Outcome: OutcomeUnreached,
			Err: fmt.Errorf("%w: %s", ErrRecordMismatch, req.Record.callID)}
	}
	a, err := Get(req.Target)
	if err != nil {
		return CallResponse{Outcome: OutcomeUnreached, Err: err}
	}

	body, err := json.Marshal(callEnvelope{
		ContractVersion: ContractVersion,
		CallID:          req.Record.callID,
		Operation:       req.Operation,
		Subject:         req.Subject,
		Actor:           req.Actor,
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

	// The pipe takes the claim, immediately before the request and after every
	// other refusal. Here rather than when the token was obtained, because a
	// token is a Go value and a Go value copies: the durable row is the only
	// thing a copy cannot duplicate. Not earlier, because a call refused by the
	// breaker or by a missing credential never left, and a record it consumed
	// would say otherwise.
	resp, err := a.send(ctx, sendSpec{
		Method: http.MethodPost, Path: "/operations/" + req.Operation, Body: body,
		Write: true, Claim: req.Record.consume,
	})
	if err != nil {
		return resp
	}

	if resp.Outcome == OutcomeSucceeded {
		resp = readOutcome(resp, a.declares("calls") == nil)
	}
	if resp.Outcome != OutcomeSucceeded {
		// Neither the request body nor the response body is logged. The request
		// carries the secret; the response carries whatever the least trusted
		// component chose to echo back, which may be the same secret.
		log.Printf("[ADDON] %s/%s call_id=%s outcome=%s status=%d err=%v secret_params=%d",
			req.Target, req.Operation, req.Record.callID, resp.Outcome, resp.Status, resp.Err, len(op.SecretParams))
	}
	return resp
}

// ErrUnknownOutcome is a 2xx operation reply whose outcome is missing or not
// one this add-on may say: answered, and saying nothing Syndra can stand
// behind, so the call may have acted (absence is never a value).
var ErrUnknownOutcome = errors.New("addon: the reply's outcome is missing or not one the add-on may say")

// readOutcome reads a 2xx operation reply's own outcome (6.3a): succeeded as
// it was; no_change, from an add-on declaring calls, as answered and nothing
// done; anything else, or none, as indeterminate. Status alone said only that
// it answered, and a no-op recorded as done is the lie this reading prevents.
func readOutcome(resp CallResponse, calls bool) CallResponse {
	var reply struct {
		Outcome string `json:"outcome"`
	}
	_ = json.Unmarshal(resp.Body, &reply)
	switch {
	case reply.Outcome == string(OutcomeSucceeded):
	case reply.Outcome == "no_change" && calls:
		resp.NoChange = true
	default:
		resp.Outcome, resp.Err = OutcomeIndeterminate, fmt.Errorf("%w: %q", ErrUnknownOutcome, reply.Outcome)
	}
	return resp
}

// addonAttempts is how many times one add-on call is sent. 1, as before W1d:
// the transport never retried (a 5xx may have acted; the pipe's callers and the
// member's page own any resend). It is an explicit extcall option, so raising
// it is one visible change, after which extcall's rule applies (reads retry on
// 408/429/5xx, writes only on 429).
var (
	addonAttempts = 1
	addonBackoff  = time.Second // first wait between tries; a var so tests do not wait it out
)

// addonPolicy is the extcall Policy of an add-on call: the caller's
// per-attempt timeout and addonAttempts tries.
func addonPolicy(timeout time.Duration) extcall.Policy {
	return extcall.Policy{Attempts: addonAttempts, Base: addonBackoff, Max: 10 * time.Second, Timeout: timeout}
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
	// Timeout, retry spacing and "which statuses may be tried again" are
	// extcall's (W1d), not decided here. The request is rebuilt and re-signed
	// per attempt, so a retry never reuses a spent timestamp or body reader.
	call := extcall.Call{
		Name:       method + " " + url,
		Idempotent: method == http.MethodGet,
		Subsystem:  "ADDON",
	}
	var sent time.Time
	httpResp, err := extcall.Do(ctx, addonPolicy(timeout), call, func(ctx context.Context) (*http.Response, error) {
		sent = timeNow() // the attempt that answered, not the first
		var rdr io.Reader
		if body != nil {
			rdr = bytes.NewReader(body)
		}
		req, err := http.NewRequestWithContext(ctx, method, url, rdr)
		if err != nil {
			return nil, err
		}
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		// Every request is signed. There is no mode in which it is not: the
		// signature is what authenticates the caller now that no client certificate
		// exists, so a conditional here would be a path on which the add-on is
		// called anonymously.
		//
		// The target as it goes on the wire — the escaped path and the raw
		// query (contract 2, decision 24) — from req.URL rather than the string
		// this was built from: it is what the add-on reads off the request line.
		req.Header.Set(SignatureHeader, ComputeSignature(timeNow(), method, signedTarget(req.URL), body, cred.signingKey))
		return cred.client.Do(req)
	})
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

	// One byte past the bound, because io.LimitReader signals its limit with EOF
	// and not with an error: reading exactly maxResponseBytes cannot tell a body
	// that ended from one that was cut off, and would report an oversized
	// response as a clean success.
	respBody, readErr := io.ReadAll(io.LimitReader(httpResp.Body, maxResponseBytes+1))
	oversized := len(respBody) > maxResponseBytes
	if oversized {
		respBody = respBody[:maxResponseBytes]
	}

	out := CallResponse{Status: httpResp.StatusCode, Body: respBody, Clock: Clock{Date: parseDate(httpResp.Header), Sent: sent}}
	if out.Status < 200 || out.Status >= 300 {
		out.Code, out.Refusal = refusalOf(respBody)
	}
	if secs, err := strconv.Atoi(httpResp.Header.Get("Retry-After")); err == nil && secs > 0 {
		out.RetryAfter = secs
	}
	out.Outcome, out.LifecycleRefusal, out.ShapeRefusal = classifyStatus(out.Status, out.Code)
	switch {
	case readErr != nil && out.Outcome == OutcomeSucceeded:
		// A 2xx whose body was cut off says the add-on acted but not what it
		// did. Not a success the backend can record.
		out.Outcome, out.Err = OutcomeIndeterminate, readErr
	case oversized && out.Outcome == OutcomeSucceeded:
		out.Outcome = OutcomeIndeterminate
		out.Err = fmt.Errorf("response exceeds %d bytes and was not read whole", maxResponseBytes)
	case oversized:
		// A refusal is decided by its status, so an oversized body does not
		// make it ambiguous — reclassifying it would turn a deterministic
		// refusal into a row that never settles. The truncation is still
		// recorded, because the body is the diagnostic and it is incomplete.
		out.Err = fmt.Errorf("addon returned %d with a body exceeding %d bytes", httpResp.StatusCode, maxResponseBytes)
	}
	if out.Outcome != OutcomeSucceeded && out.Err == nil {
		if httpResp.StatusCode >= 300 && httpResp.StatusCode < 400 {
			// The Location is deliberately not echoed: it is attacker-chosen
			// content, and a log line is the wrong place to reproduce one. The
			// status alone tells an operator what to fix, and the add-on's own
			// log holds where it tried to send them.
			out.Err = fmt.Errorf("addon returned redirect %d; add-on responses are not followed", httpResp.StatusCode)
		} else {
			out.Err = fmt.Errorf("addon returned %d", httpResp.StatusCode)
		}
	}
	return out
}

// Codes the classification reads (registry/errors.json). Status stays primary;
// a code refines it only where the status alone says the wrong thing.
const (
	codeLifecycleRefusal        = "LIFECYCLE_REFUSAL"
	codeContractVersionMismatch = "CONTRACT_VERSION_MISMATCH"
	codeUnknownRequestField     = "UNKNOWN_REQUEST_FIELD"
	codeCallInProgress          = "CALL_IN_PROGRESS"
	codeFreshReadCooldown       = "FRESH_READ_COOLDOWN"
	codeTargetUnreachable       = "TARGET_UNREACHABLE"
	codeUnknownRoute            = "UNKNOWN_ROUTE"
)

// refusalOf is a refusal's code and what it names, in one decode: the
// problem's `code` (contract 2), else a contract-1 add-on's `error`. The
// fallback is for classification only and only until the next major: a
// contract-1 add-on writes {"error": CODE}, and without it, during a rollback,
// its 400 would fall to rejected and its 503 lifecycle refusal to
// indeterminate (design §11). An extension member of the wrong type is not
// read (a refusal is still classified by its code).
func refusalOf(body []byte) (string, Refusal) {
	var p struct {
		Code  any             `json:"code"`
		Error any             `json:"error"`
		Param json.RawMessage `json:"param"`
		Limit json.RawMessage `json:"limit"`
		Unit  json.RawMessage `json:"unit"`
		Count json.RawMessage `json:"count"`
	}
	if json.Unmarshal(body, &p) != nil {
		return "", Refusal{}
	}
	var r Refusal
	_ = json.Unmarshal(p.Param, &r.Param)
	_ = json.Unmarshal(p.Unit, &r.Unit)
	var n int
	if json.Unmarshal(p.Limit, &n) == nil {
		r.Limit = &n
	}
	var c int
	if json.Unmarshal(p.Count, &c) == nil {
		r.Count = &c
	}
	if code, ok := p.Code.(string); ok && code != "" {
		return code, r
	}
	code, _ := p.Error.(string)
	return code, r
}

// classifyStatus maps a status and the add-on's code to what the add-on may
// have done.
//
//   - 2xx: it acted and said so.
//   - LIFECYCLE_REFUSAL: draining or read-only, declined before doing anything
//     (design §18). Detected by the code alone: "503 with Retry-After" is also
//     what a target the add-on cannot reach answers (TARGET_UNREACHABLE), and
//     only one of the two is a maintenance window.
//   - CONTRACT_VERSION_MISMATCH, UNKNOWN_REQUEST_FIELD: the request's shape
//     was refused before anything was done, so unreached, and a shape
//     refusal: never the target refusing the change, so never target_refused
//     (design §11). A plain BAD_REQUEST — a missing call_id or subject — stays
//     rejected, or a malformed row would requeue for ever.
//   - CALL_IN_PROGRESS: the same call is executing now; this copy did nothing.
//   - TARGET_UNREACHABLE, 429: it did not act and expects to be asked again.
//   - 408, 5xx: it may have acted. A 500 raised after a partial mutation looks
//     exactly like one raised before validation.
//   - 3xx: a contract violation, surfaced rather than followed (see
//     newAddonClient). Deterministic.
//   - other 4xx: it validated the call and refused. Deterministic.
func classifyStatus(status int, code string) (outcome Outcome, lifecycle, shape bool) {
	switch {
	case status >= 200 && status < 300:
		return OutcomeSucceeded, false, false
	case code == codeLifecycleRefusal:
		return OutcomeUnreached, true, false
	case code == codeContractVersionMismatch || code == codeUnknownRequestField:
		return OutcomeUnreached, false, true
	case code == codeCallInProgress || code == codeTargetUnreachable || status == http.StatusTooManyRequests:
		return OutcomeUnreached, false, false
	case status == http.StatusRequestTimeout || status >= 500:
		return OutcomeIndeterminate, false, false
	default:
		return OutcomeRejected, false, false
	}
}

// signedTarget is what the MAC covers of the URL (contract 2, decision 24):
// the path escaped exactly as sent, plus "?" and the raw query when there is
// one — RFC 9421's @query as sent, never sorted or re-encoded. An empty query
// adds nothing, so a query-less MAC is contract 1's whenever the path needs no
// escaping.
func signedTarget(u *url.URL) string {
	if u.RawQuery == "" {
		return u.EscapedPath()
	}
	return u.EscapedPath() + "?" + u.RawQuery
}

// ComputeSignature returns the full SignatureHeader value: HMAC-SHA256 over
// "<unix_seconds>.<method>.<target>.<body>" under key, formatted
// "t=<ts>,v1=<hex>", target being signedTarget.
//
// The timestamp is inside the MAC input, not merely beside it, so it cannot be
// edited to extend a captured signature's life. The body is inside it, so the
// signature authenticates what was asked and not only who asked.
//
// The METHOD and PATH are inside it because the operation name is in the URL
// and nothing else carried it: `GET /capabilities` has an empty body, so its
// MAC was a function of the timestamp alone and verified for any zero-body
// request inside the two-minute window — including a mutating one. Signing the
// request line is what SigV4 does, for this reason.
//
// The target is the path and query only, never the full URL: the host is
// already pinned by the registration and by TLS, and including it would make a
// deployment that reaches the same add-on by two names fail to authenticate
// against itself.
//
// A bare shared secret in a header is deliberately not offered as an
// alternative: it identifies the caller and binds nothing, so an intercepted
// call replays verbatim, forever.
func ComputeSignature(ts time.Time, method, target string, body, key []byte) string {
	mac := hmac.New(sha256.New, key)
	fmt.Fprintf(mac, "%d", ts.Unix())
	mac.Write([]byte("."))
	mac.Write([]byte(method))
	mac.Write([]byte("."))
	mac.Write([]byte(target))
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
func (b *breaker) record(now time.Time, resp CallResponse) {
	b.mu.Lock()
	defer b.mu.Unlock()
	// A coded /stats answer neither opens nor clears the breaker: counted, an
	// hourly collection of an unread metric would take the target away from
	// everybody; clearing, a broken add-on answering /stats with a code would
	// hide its failures elsewhere. A transport failure, a timeout, an uncoded
	// 5xx on /stats still counts, and a 2xx still clears. /stats only: a coded
	// refusal on any other route is recorded as ever.
	if resp.StatsRefusal {
		return
	}
	// A 2xx and a 4xx are the add-on demonstrably working: it read the request
	// and decided. Those clear the count.
	if resp.Outcome == OutcomeSucceeded || resp.Outcome == OutcomeRejected {
		b.failures, b.openUntil = 0, time.Time{}
		return
	}
	// A lifecycle refusal is not counted — a maintenance window must not take a
	// target offline for everybody — but it does not CLEAR the count either,
	// and the difference is the whole difference between a breaker and a
	// suggestion. `LifecycleRefusal` is derived entirely from add-on-controlled
	// output (the code LIFECYCLE_REFUSAL), so a clearing one let a broken
	// add-on alternate 5xx with lifecycle refusals and keep the breaker shut for
	// ever. Skipped, not reset: the window pauses the count, it does not
	// forgive it.
	//
	// A fresh read refused inside the add-on's cooldown (FRESH_READ_COOLDOWN,
	// 429) is the same: the add-on answering on purpose. Counted, five presses
	// of "Read again" would open the breaker and take the target away from
	// everybody for its cooldown.
	if resp.LifecycleRefusal || resp.Code == codeFreshReadCooldown {
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
