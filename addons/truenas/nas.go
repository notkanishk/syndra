package main

import (
	"encoding/json"
	"errors"
	"fmt"
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

	// version is read once per successful connect. Majors break, and an
	// untested one is refused rather than attempted (§Risks).
	version       string
	supportedMajs map[string]bool

	lastRead time.Time
	now      func() time.Time
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
	if since := n.now().Sub(n.lastTry); since < reconnectCooldown {
		return nil, fmt.Errorf("%w: waiting out the reconnect cooldown", ErrTargetUnreachable)
	}
	n.lastTry = n.now()

	c, err := n.dial()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrTargetUnreachable, err)
	}
	n.client = c
	return c, nil
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
	raw, err := c.Call(method, callTimeoutSeconds, params)
	if err != nil {
		if classifyNASError(err) == ErrTargetUnreachable {
			// A dead socket must not be reused for the next call, or every
			// subsequent one fails on a connection nothing will reopen.
			n.drop()
		}
		return fmt.Errorf("%s: %w", method, classifyNASError(err))
	}
	n.mu.Lock()
	n.lastRead = n.now()
	n.mu.Unlock()

	if out == nil {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("%s: decode response: %w", method, err)
	}
	return nil
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

// majorOf takes the leading `YY.MM` of a TrueNAS version string, which is what
// its release line is named by.
func majorOf(version string) string {
	parts := strings.SplitN(strings.TrimSpace(version), ".", 3)
	if len(parts) < 2 {
		return strings.TrimSpace(version)
	}
	return parts[0] + "." + parts[1]
}

// dialTrueNAS is the real dialer: one WebSocket, authenticated by API key.
//
// `auth.login_with_api_key` under the hood — a user-linked key whose privileges
// come from the linked user's group. The add-on's identity is a dedicated local
// user granted `ACCOUNT_WRITE` and `SYSTEM_AUDIT_READ` and nothing more; in
// particular it cannot delete an account, which is why purge runs on a separate
// credential the backend injects into that one call.
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
