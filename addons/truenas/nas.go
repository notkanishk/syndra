package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"

	truenas "github.com/truenas/api_client_golang/truenas_api"
)

// The TrueNAS session (design §10).
//
// One persistent WebSocket, reconnected on loss, guarded by a breaker. That is
// not tidiness: TrueNAS rate-limits authentication to 20 attempts per 60
// seconds with a ten-minute lockout, so a reconnect loop is a way to lock the
// backend out of its own NAS for ten minutes — and every operation this add-on
// performs shares the one session.

const (
	callTimeoutSeconds = 30
	// reconnectCooldown is the floor between login attempts. Well under the
	// rate limit even if every attempt failed, which is the point: the limit
	// must be unreachable by this code, not merely usually avoided.
	reconnectCooldown = 15 * time.Second
	// probeCooldown is the floor between version probes on a LIVE session.
	//
	// A failed probe leaves the version empty, which is what the gate needs, and
	// it must be retried — a target that answers `system.version` a minute later
	// should not need a reconnect to be usable. But retried on every call it
	// doubles the traffic on the one rate-limited session this whole design is
	// built around, against a target that answers everything else and refuses or
	// lacks that one method. So the probe gets the floor the dial already has.
	probeCooldown = 30 * time.Second

	// The breaker. It exists for one specific failure: TrueNAS locks an account
	// out for ten minutes after 20 failed authentications in 60 seconds, and
	// every operation this add-on performs shares one session. Retrying into
	// that is how a transient problem becomes a ten-minute outage for everybody.
	//
	// The cooldown is longer than the lockout, so a breaker that opened because
	// of one has certainly cleared it before allowing again.
	breakerThreshold = 5
	breakerCooldown  = 11 * time.Minute
)

var (
	// ErrTargetUnreachable means nothing was asked. Distinct from a refusal,
	// because only this one is safe to try again.
	ErrTargetUnreachable = errors.New("the target is not reachable")
	// ErrTargetRefused is the NAS answering no. Deterministic; retrying changes
	// nothing.
	ErrTargetRefused = errors.New("the target refused the call")
	// ErrRateLimited is the NAS pushing back. Its own error because the right
	// response is to stop, not to retry harder into a lockout.
	ErrRateLimited = errors.New("the target is rate-limiting")
)

// rpc is the narrow slice of the TrueNAS client this add-on uses.
//
// An interface because every behaviour worth testing here — reconnect, the
// breaker, version gating, the absence of hashes from a response — is about
// what this code does with an answer, not about the wire. A test that needed a
// NAS would be a test nobody runs.
type rpc interface {
	Call(method string, timeoutSeconds int64, params any) (json.RawMessage, error)
	Ping() (string, error)
	Close() error
}

// dialer opens an authenticated session.
type dialer func() (rpc, error)

// NAS holds the session and everything that decides whether to use it.
type NAS struct {
	mu      sync.Mutex
	dial    dialer
	client  rpc
	lastTry time.Time

	// callMu serialises the wire. The client writes its request frame with
	// `conn.WriteJSON` outside its own mutex, and gorilla panics outright on a
	// concurrent write — so two requests arriving together take down the
	// process rather than interleaving. One session was always the design; this
	// is the part that makes it one at a time.
	callMu sync.Mutex

	// version is read once per successful connect. Majors break, and an
	// untested one is refused rather than attempted (§Risks).
	version       string
	supportedMajs map[string]bool
	// selfName and selfUID are the target account this add-on's API key belongs
	// to, read from `auth.me`. Held so the add-on can refuse to hand its own
	// credential's account to an operator as something to adopt or delete.
	selfName string
	selfUID  int64
	// probed says the version and method list have been read for the CURRENT
	// connection. Reset by every dial, because a reconnect may land on an
	// upgraded target — and because a version read once at startup leaves an
	// add-on that started before its NAS refusing every mutation forever.
	probed bool
	// lastProbe bounds how often a FAILED probe is retried on a live session.
	lastProbe time.Time

	lastRead time.Time
	now      func() time.Time

	// failures and openUntil are the breaker. Held here rather than in their
	// own type because they guard exactly one thing — this session — and a
	// breaker that could be shared would be a breaker somebody shares.
	failures  int
	openUntil time.Time

	// methods is what the target exposes, or nil for "not read". Nil reads as
	// everything-present, so a failed enumeration never withdraws the surface.
	methods map[string]bool
}

func newNAS(dial dialer, supportedMajors []string) *NAS {
	m := make(map[string]bool, len(supportedMajors))
	for _, v := range supportedMajors {
		m[v] = true
	}
	return &NAS{dial: dial, supportedMajs: m, now: time.Now}
}

// session returns a live client, connecting if needed.
//
// The cooldown is checked before dialling rather than after failing, so a burst
// of calls during an outage costs one login attempt rather than one each. This
// is the whole defence against the ten-minute lockout.
func (n *NAS) session() (rpc, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.client != nil {
		return n.client, nil
	}
	if n.now().Before(n.openUntil) {
		// Open. Refusing here rather than at the call site is deliberate: the
		// thing being protected is the login, and a breaker checked after the
		// session is established protects nothing.
		return nil, fmt.Errorf("%w: the circuit is open until %s",
			ErrTargetUnreachable, n.openUntil.UTC().Format(time.RFC3339))
	}
	if since := n.now().Sub(n.lastTry); since < reconnectCooldown {
		return nil, fmt.Errorf("%w: waiting out the reconnect cooldown", ErrTargetUnreachable)
	}
	n.lastTry = n.now()

	c, err := n.dial()
	if err != nil {
		n.recordFailureLocked(err)
		return nil, fmt.Errorf("%w: %v", ErrTargetUnreachable, err)
	}
	// A successful login clears the count outright rather than decrementing it.
	// The failure this guards is a burst, so one working connection means the
	// burst is over.
	n.failures, n.openUntil = 0, time.Time{}
	// A new connection probes immediately: the cooldown exists to bound retries
	// on a live session, not to delay the first read on a fresh one.
	n.client, n.probed, n.lastProbe = c, false, time.Time{}
	return c, nil
}

// ensureProbed reads the version and method list once per connection.
//
// Per connection rather than once per process. The version gate refuses every
// mutation until a version has been read, so an add-on that came up while its
// NAS was down — the ordinary case on a host reboot — would refuse every write
// until somebody restarted the container, against a target that was answering
// perfectly.
//
// Best-effort in both directions: a failed probe leaves the version empty,
// which the gate reads as "not tested" and refuses mutations on, and leaves
// `probed` false so the next call tries again rather than waiting for a
// reconnect that may never be needed.
func (n *NAS) ensureProbed() {
	n.mu.Lock()
	if n.probed || n.client == nil || n.now().Sub(n.lastProbe) < probeCooldown {
		n.mu.Unlock()
		return
	}
	// Both set before probing, not after. The flag because the probe calls back
	// into `call` and one set afterwards would recurse until the stack ran out;
	// the timestamp because a probe that FAILS clears the flag again, and
	// without a time beside it every subsequent call would re-probe — doubling
	// the traffic on the one session the rate limit is about.
	n.probed, n.lastProbe = true, n.now()
	n.mu.Unlock()

	if _, err := n.SystemVersion(); err != nil {
		n.mu.Lock()
		n.version, n.probed = "", false
		n.mu.Unlock()
		return
	}
	n.readSelf()
	n.loadMethods()
}

// readSelf learns which target account this add-on's own credential belongs to.
//
// Without it, the add-on's TrueNAS service account is just another row in
// `user.query`: it shows up in the unmanaged inventory, offering an operator
// the choice to adopt it — and, one confirmation later, to PURGE it. Deleting
// that account does not break a member's access, it deletes the credential
// Syndra authenticates with, and no operation afterwards can put it back.
//
// A read of the target's own opinion rather than a configured name, because a
// name in `.env` would be a second definition of an identity the API key
// already carries, and the two would disagree the first time a key was reissued
// against a different user.
//
// Non-fatal by design. Not knowing means the guards below refuse to vouch for
// anything, which is the safe direction: they only ever REMOVE an account from
// what an operator may act on.
func (n *NAS) readSelf() {
	var me struct {
		Name string `json:"pw_name"`
		UID  int64  `json:"pw_uid"`
	}
	if err := n.call("auth.me", []any{}, &me); err != nil {
		log.Printf("[NAS] could not read this add-on's own account (auth.me): %v; "+
			"its TrueNAS account will appear as an ordinary unmanaged one", err)
		return
	}
	n.mu.Lock()
	n.selfName, n.selfUID = me.Name, me.UID
	n.mu.Unlock()
}

// Self is the target account this add-on authenticates as, and whether it is
// known at all.
func (n *NAS) Self() (string, int64, bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.selfName, n.selfUID, n.selfName != ""
}

// Probe forces a connection and reads what the gates depend on.
//
// Used at startup so `/capabilities` can answer with a version straight away.
// Not required for correctness — every call probes what it needs — which is why
// its failure is not fatal.
func (n *NAS) Probe() error {
	if _, err := n.session(); err != nil {
		return err
	}
	n.ensureProbed()
	if n.Version() == "" {
		return fmt.Errorf("%w: the target did not answer system.version", ErrTargetUnreachable)
	}
	return nil
}

// recordFailureLocked counts a failed connection and opens the circuit at the
// threshold. Caller holds the lock.
//
// Rate limiting counts double — it is the target saying, in as many words, that
// the next attempt is the one that locks the account — so it opens the circuit
// immediately rather than after five more tries.
func (n *NAS) recordFailureLocked(err error) {
	if errors.Is(classifyNASError(err), ErrRateLimited) {
		n.failures = breakerThreshold
	} else {
		n.failures++
	}
	if n.failures >= breakerThreshold {
		n.openUntil = n.now().Add(breakerCooldown)
	}
}

// CircuitOpen reports whether the breaker is holding, for `/health`. An add-on
// that is refusing its own calls must say so, or an operator sees an unreachable
// target and looks at the network.
func (n *NAS) CircuitOpen() bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.now().Before(n.openUntil)
}

// drop closes and forgets the session so the next call reconnects.
func (n *NAS) drop() {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.client != nil {
		_ = n.client.Close()
		n.client = nil
	}
}

// call issues one JSON-RPC method and classifies the failure.
//
// The classification matters more than it looks: the backend's accounting
// distinguishes "nothing happened" from "it refused" from "we do not know", and
// collapsing them here would make that distinction a fiction one layer up.
func (n *NAS) call(method string, params any, out any) error {
	c, err := n.session()
	if err != nil {
		return err
	}
	n.ensureProbed()

	raw, err := n.invoke(c, method, params)
	if err != nil {
		return fmt.Errorf("%s: %w", method, n.classify(err))
	}
	n.mu.Lock()
	n.lastRead = n.now()
	n.mu.Unlock()

	result, refusal, err := splitEnvelope(raw)
	if err != nil {
		return fmt.Errorf("%s: %w", method, err)
	}
	if refusal != nil {
		// Checked before `out`, and independently of it. Every mutation on this
		// add-on passes out == nil, so a refusal noticed only when a result was
		// wanted would be a refusal never noticed at all — and `user.update`
		// answering "no" would be reported to the backend as applied.
		return fmt.Errorf("%s: %w (target error code %d)", method, n.classifyRefusal(refusal), refusal.Code)
	}
	if out == nil {
		return nil
	}
	if len(result) == 0 {
		return fmt.Errorf("%s: the target answered with neither a result nor an error", method)
	}
	if err := json.Unmarshal(result, out); err != nil {
		return fmt.Errorf("%s: decode result: %w", method, err)
	}
	return nil
}

// invoke is the one place a request frame is written.
//
// Serialised, because the client writes outside its own lock and gorilla panics
// on a concurrent write. The session lock is deliberately not reused for this:
// that one is held while dialling, and a 30-second call must not block the
// breaker check every other request makes.
func (n *NAS) invoke(c rpc, method string, params any) (json.RawMessage, error) {
	n.callMu.Lock()
	defer n.callMu.Unlock()
	return c.Call(method, callTimeoutSeconds, params)
}

// classify maps a failure onto the three outcomes that differ and does the
// bookkeeping each one implies.
func (n *NAS) classify(err error) error {
	classified := classifyNASError(err)
	switch classified {
	case ErrTargetUnreachable:
		// A dead socket must not be reused for the next call, or every
		// subsequent one fails on a connection nothing will reopen.
		n.drop()
	case ErrRateLimited:
		// Backpressure on a live session still counts: the next reconnect
		// is what would trip the lockout, and the breaker has to be holding
		// before that reconnect is attempted.
		n.mu.Lock()
		n.recordFailureLocked(err)
		n.mu.Unlock()
		n.drop()
	}
	return classified
}

// classifyRefusal narrows an answered refusal to the two outcomes it can be.
//
// Never unreachable, however the message reads: the target demonstrably
// received the call, and the entire worth of that outcome is that it means
// nothing happened. Rate limiting is the exception that still has to reach the
// breaker — it is the target saying, in as many words, that the next reconnect
// is the one that locks the account out.
func (n *NAS) classifyRefusal(e *rpcError) error {
	if errors.Is(classifyNASError(e), ErrRateLimited) {
		n.mu.Lock()
		n.recordFailureLocked(e)
		n.mu.Unlock()
		n.drop()
		return ErrRateLimited
	}
	return ErrTargetRefused
}

// rpcError is a JSON-RPC error member.
//
// Its message is deliberately never propagated. `user.update({password})` puts
// a member's credential in the call's own parameters, and the middleware's
// error text is the likeliest place for one to be echoed back — so what leaves
// this file is a classification and a numeric code, which is the guarantee
// every caller's "the target's own text never reaches here" comment rests on.
type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	// Data carries the middleware's structured complaint. Only the FIELD PATHS
	// inside it are ever read — see validationFields.
	Data struct {
		// Extra is `[["user_create.password", "Password is required", 22], ...]`
		// on a validation refusal, and something else entirely on anything else,
		// which is why it is decoded loosely and read defensively.
		Extra []json.RawMessage `json:"extra"`
	} `json:"data"`
}

// validationFields returns the field PATHS a refusal named, and nothing else.
//
// The paths, never the messages. A message is free text the middleware built,
// and `user.update({password})` puts a member's credential in the parameters of
// the very call most likely to produce one — which is why this file's rule is
// that the target's own text does not leave it. A path like
// `user_create.password` is structural: it names a field of the REQUEST SCHEMA,
// it cannot contain a value, and it is the entire difference between "the
// target refused the call (target error code -32602)" and knowing that the
// payload is missing a password.
//
// That difference was an afternoon: account creation had never worked against
// any release, and the add-on reported only the numeric code while the target
// had been saying `user_create.password: Password is required` all along.
func (e *rpcError) validationFields() []string {
	var out []string
	for _, raw := range e.Data.Extra {
		// Each entry is a positional array whose first element is the path.
		var entry []json.RawMessage
		if err := json.Unmarshal(raw, &entry); err != nil || len(entry) == 0 {
			continue
		}
		var field string
		if err := json.Unmarshal(entry[0], &field); err != nil {
			continue
		}
		if field = strings.TrimSpace(field); field == "" {
			continue
		}
		// Belt and braces: a path is an identifier chain. Anything carrying a
		// space, a quote or a non-ASCII byte is not one, and is dropped rather
		// than forwarded on the assumption that it must be.
		if !fieldPath.MatchString(field) {
			continue
		}
		out = append(out, field)
	}
	return out
}

// fieldPath is what a schema path looks like and nothing wider.
var fieldPath = regexp.MustCompile(`^[A-Za-z0-9_.\[\]-]{1,120}$`)

func (e *rpcError) Error() string { return e.Message }

// jsonrpcEnvelope is what the client's `Call` actually returns.
//
// Not the result — the WHOLE message. `Call` forwards the raw frame the
// listener read and reports err == nil for a frame carrying an `error` member,
// so decoding its return straight into a caller's type reads the envelope's
// keys as the result's, and a refusal reads as a success.
type jsonrpcEnvelope struct {
	Result json.RawMessage `json:"result"`
	Error  *rpcError       `json:"error"`
}

// splitEnvelope separates the result from the refusal.
func splitEnvelope(raw json.RawMessage) (json.RawMessage, *rpcError, error) {
	var env jsonrpcEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, nil, fmt.Errorf("decode response envelope: %w", err)
	}
	if env.Error != nil {
		return nil, env.Error, nil
	}
	return env.Result, nil, nil
}

// callOnce issues one call on a session that is not the shared one.
//
// The purge runs on a credential injected for that single call, so it has no
// NAS to route through — and it is the one operation that cannot be undone,
// which makes reading its refusal the least optional of all of them.
func callOnce(c rpc, method string, params any, out any) error {
	raw, err := c.Call(method, callTimeoutSeconds, params)
	if err != nil {
		return fmt.Errorf("%s: %w", method, classifyNASError(err))
	}
	result, refusal, err := splitEnvelope(raw)
	if err != nil {
		return fmt.Errorf("%s: %w", method, err)
	}
	if refusal != nil {
		if fields := refusal.validationFields(); len(fields) > 0 {
			return fmt.Errorf("%s: %w (target error code %d, rejected: %s)",
				method, ErrTargetRefused, refusal.Code, strings.Join(fields, ", "))
		}
		return fmt.Errorf("%s: %w (target error code %d)", method, ErrTargetRefused, refusal.Code)
	}
	if out == nil {
		return nil
	}
	if len(result) == 0 {
		return fmt.Errorf("%s: the target answered with neither a result nor an error", method)
	}
	return json.Unmarshal(result, out)
}

// classifyNASError maps a client error onto the three outcomes that differ.
//
// String matching, and stated as the compromise it is: the client returns
// errors built from the middleware's own text and carries no code. What is
// matched on is the rate-limit and connection vocabulary, and anything
// unrecognised falls to `refused` — the conservative direction, because
// treating an unknown failure as retryable is how a lockout happens.
func classifyNASError(err error) error {
	if err == nil {
		return nil
	}
	text := strings.ToLower(err.Error())
	switch {
	case strings.Contains(text, "rate limit"), strings.Contains(text, "too many"):
		return ErrRateLimited
	case strings.Contains(text, "connection"), strings.Contains(text, "closed"),
		strings.Contains(text, "eof"), strings.Contains(text, "timeout"),
		strings.Contains(text, "websocket"), strings.Contains(text, "not connected"):
		return ErrTargetUnreachable
	default:
		return ErrTargetRefused
	}
}

// SystemVersion reads and caches the target's version.
func (n *NAS) SystemVersion() (string, error) {
	var v string
	if err := n.call("system.version", []any{}, &v); err != nil {
		return "", err
	}
	n.mu.Lock()
	n.version = v
	n.mu.Unlock()
	return v, nil
}

// Version returns the last observed version, or "" if never read.
func (n *NAS) Version() string {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.version
}

// LastRead is when the NAS last answered. It is what `/health` reports and what
// tells an operator "not seen since Tuesday" from "seen a minute ago".
func (n *NAS) LastRead() time.Time {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.lastRead
}

// MethodPresent reports whether the target exposes a method.
//
// Read once per connection from `core.get_methods` and cached, because it is a
// fact about a release rather than a moment — and because asking per call would
// spend a round trip through a rate-limited session to learn something that
// cannot change until the NAS is upgraded.
//
// Unknown is treated as PRESENT. A target that will not enumerate its methods
// is not a target with none, and declaring every operation unavailable on the
// strength of one failed read would withdraw the whole surface during an
// outage — the same mistake as concluding an absence from a read that could not
// happen.
func (n *NAS) MethodPresent(method string) bool {
	n.mu.Lock()
	known := n.methods
	n.mu.Unlock()
	if known == nil {
		return true
	}
	return known[method]
}

// loadMethods reads the target's method list. Best-effort by construction: a
// failure leaves the cache nil, which reads as "everything present".
func (n *NAS) loadMethods() {
	var listing map[string]json.RawMessage
	if err := n.call("core.get_methods", []any{}, &listing); err != nil {
		return
	}
	known := make(map[string]bool, len(listing))
	for name := range listing {
		known[name] = true
	}
	n.mu.Lock()
	n.methods = known
	n.mu.Unlock()
}

// MajorSupported reports whether the observed version is one this add-on has
// been tested against.
//
// An untested major refuses MUTATIONS and keeps serving reads. Refusing reads
// too would make the backend record the target as unreconciled, which says
// "we cannot see it" when the truth is "we can see it and will not write to
// it" — different problems with different fixes.
func (n *NAS) MajorSupported() (bool, string) {
	v := n.Version()
	if v == "" {
		return false, "the target version has not been read yet"
	}
	major := majorOf(v)
	if !n.supportedMajs[major] {
		return false, fmt.Sprintf("TrueNAS %s is outside the tested range for this add-on", major)
	}
	return true, ""
}

// majorOf takes the `YY.MM` of a TrueNAS version string, which is what its
// release line is named by.
//
// It searches for the release number rather than assuming the string starts
// with it, because TrueNAS does not: a real 25.10 answers `system.version` with
// **`TrueNAS-25.10.5`**, and older SCALE releases answered `TrueNAS-SCALE-24.04.0`.
// Splitting on "." from the left yielded `TrueNAS-25.10`, which matches no entry
// in TRUENAS_SUPPORTED_MAJORS — so the add-on refused every mutation against a
// version it explicitly supports, and reported the reason as "outside the tested
// range" while naming a major nobody had ever written down.
//
// Found on the first connection to a real NAS. Nothing could have found it
// earlier: the recorded fixture carried a bare `25.04.2.1`, so the parser and
// the fake agreed with each other and the target was the only thing that
// disagreed.
func majorOf(version string) string {
	if m := releaseNumber.FindString(version); m != "" {
		return m
	}
	// No YY.MM anywhere: hand back what was read, so the refusal quotes the
	// target's own answer rather than an empty string.
	return strings.TrimSpace(version)
}

// releaseNumber is the first `N.N` in a version string. Anchored on digits
// rather than on any prefix, because the prefixes are the part that varies
// between products and releases.
//
// Not `\d{2}\.\d{2}`: that reads SCALE's YY.MM and silently fails to match
// CORE's `TrueNAS-13.0-U6`, which would hand the whole string back to the
// supported-majors gate and have it refuse "TrueNAS-13.0-U6" instead of "13.0".
// CORE is out of scope for this add-on either way — but a refusal that names the
// release is a refusal an operator can act on, and one that quotes a blob is not.
var releaseNumber = regexp.MustCompile(`\d+\.\d+`)

// dialTrueNAS is the real dialer: one WebSocket, authenticated by API key.
//
// `auth.login_with_api_key` under the hood — a user-linked key whose privileges
// come from the linked user's group. The add-on's identity is a dedicated local
// user granted `ACCOUNT_WRITE` and `SYSTEM_AUDIT_READ` and nothing more.
//
// That is not a capability separation for deletion, and this comment claimed it
// was. TrueNAS requires `ACCOUNT_WRITE` for `user.delete` and has no narrower
// role, so the standing key CAN delete an account; what the separate injected
// credential buys is that no delete travels on the long-lived session and each
// one is attributable to a key issued for that single call. The mechanism is
// unchanged — the reason given for it was wrong, which is the more expensive
// kind of wrong, because a reason is what stops the next person checking.
func dialTrueNAS(url, apiKey string, verifyTLS bool) dialer {
	return func() (rpc, error) {
		c, err := truenas.NewClient(url, verifyTLS)
		if err != nil {
			return nil, err
		}
		// Username and password empty: the three-argument Login uses the key
		// when one is given, and sending a blank password beside it would be a
		// credential-shaped value in a call that does not want one.
		if err := c.Login("", "", apiKey); err != nil {
			_ = c.Close()
			return nil, err
		}
		return c, nil
	}
}
