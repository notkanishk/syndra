package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// The server side of the backend-to-add-on contract (design §16).
//
// This add-on is the least trusted component in the system: it holds the
// TrueNAS API key and talks to a third-party service. What that buys the
// deployment is isolation, and isolation is only worth anything if the channel
// into it is authenticated — otherwise anything on the Compose network can
// order a credential reset.
//
// Two modes, matching what the backend can offer: mutual TLS verified against
// the deployment's own private CA, or a signed request carrying a timestamp and
// a hash of the body. A bare shared secret is deliberately not one of them: it
// identifies the caller and binds nothing, so an intercepted call replays
// verbatim, forever.

// SignatureHeader must match the backend's constant exactly. Written out rather
// than imported because the two are separately deployed binaries — a shared
// constant would be a shared module, and the version skew that module would
// hide is precisely what the contract version exists to surface.
const SignatureHeader = "X-Syndra-Addon-Signature"

// ContractVersion is what this add-on speaks. The backend refuses to register a
// version it does not support, loudly and at startup, rather than discovering
// the mismatch later as a field that is silently absent.
const ContractVersion = 2

// signatureTolerance bounds how old a signed request may be.
//
// It is the replay window for signed mode, and it is short on purpose: the
// operation id deduplicates a replay that reaches the store, but a call
// replayed before its first attempt was recorded would not be caught by it.
// Two minutes absorbs ordinary clock skew between two containers on one host
// and nothing else.
const signatureTolerance = 2 * time.Minute

// maxRequestBytes bounds a body before it is read, let alone parsed. An add-on
// with one caller has no reason to accept a large one, and "read it all and
// then check" is how a small service becomes a memory exhaustion target.
const maxRequestBytes = 1 << 20

var (
	errNoSignature    = errors.New("request carries no signature")
	errBadSignature   = errors.New("signature does not match the body and timestamp")
	errStaleSignature = errors.New("signature timestamp is outside the accepted window")
	errNoSigningKey   = errors.New("the add-on has no signing key and cannot authenticate anything")
)

// authenticator decides whether a request came from the backend.
//
// The signature is the whole authentication, so it is verified over the body
// that will actually be parsed rather than over a re-read of it.
//
// There was a second mode — mutual TLS against a private CA — and it is gone
// with the CA. The signing key is now derived from the deployment secret rather
// than configured beside a certificate, so there is nothing left to choose
// between and no "configure exactly one of" to refuse. What the TLS layer still
// carries is confidentiality of a body holding declared secret_params, and the
// backend's assurance that it is talking to this add-on is its pin on the key
// derived from that same secret.
type authenticator struct {
	signingKey []byte
	now        func() time.Time
}

// verify reads the body, authenticates the request, and returns the bytes the
// handler should parse.
//
// Returning the body is not a convenience. Verifying a signature over one read
// and parsing a second is the classic way to authenticate something other than
// what was executed, and an http.Request body can only be read once — so the
// function that checks the signature is the function that hands over the bytes.
func (a *authenticator) verify(r *http.Request) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxRequestBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if len(body) > maxRequestBytes {
		return nil, fmt.Errorf("body exceeds %d bytes", maxRequestBytes)
	}

	if len(a.signingKey) == 0 {
		// Unreachable through loadConfig, which refuses to start without a
		// secret. Kept as a refusal rather than an assumption: a future caller
		// constructing an authenticator directly must not get an open door,
		// and "the config validates it" is the kind of reasoning that stops
		// holding one refactor later.
		return nil, errNoSigningKey
	}
	if err := a.verifySignature(r.Header.Get(SignatureHeader), r.Method, signedTarget(r.URL), body); err != nil {
		return nil, err
	}
	return body, nil
}

// signedTarget is what the MAC covers of the URL (contract 2, decision 24): the
// path ESCAPED, exactly as sent, plus "?" and the raw query when there is one —
// as RFC 9421's @query has it, never sorted or re-encoded. An empty query adds
// nothing, so a query-less MAC is byte-identical to contract 1's whenever the
// path needs no escaping. Contract 1 signed the decoded path and no query, which
// left a query (a cursor, a since) outside the signature.
func signedTarget(u *url.URL) string {
	if u.RawQuery == "" {
		return u.EscapedPath()
	}
	return u.EscapedPath() + "?" + u.RawQuery
}

// verifySignature checks "t=<unix>,v1=<hex>" over
// "<unix>.<method>.<target>.<body>", target being signedTarget.
//
// The timestamp is inside the MAC input rather than merely beside it, so it
// cannot be edited to extend a captured signature's life; the body is inside
// it, so the signature authenticates what was asked and not only who asked.
//
// The method and path are inside it because the OPERATION NAME is in the path
// and in nothing else. `GET /capabilities` carries an empty body, so its MAC
// was a function of the timestamp alone — valid for any zero-body request
// inside the two-minute window, whichever path it was replayed at. The call-id
// dedup happened to block the sequential version of that, which is a mitigation
// standing where a check should be.
func (a *authenticator) verifySignature(header, method, target string, body []byte) error {
	if strings.TrimSpace(header) == "" {
		return errNoSignature
	}
	var ts, mac string
	for _, part := range strings.Split(header, ",") {
		k, v, found := strings.Cut(strings.TrimSpace(part), "=")
		if !found {
			continue
		}
		switch k {
		case "t":
			ts = v
		case "v1":
			mac = v
		}
	}
	if ts == "" || mac == "" {
		return errNoSignature
	}

	seconds, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return errNoSignature
	}
	// Checked before the MAC, because an expired signature is refused whether
	// or not it verifies and computing the MAC first would only tell an
	// attacker how long the comparison took.
	age := a.now().Sub(time.Unix(seconds, 0))
	if age > signatureTolerance || age < -signatureTolerance {
		return errStaleSignature
	}

	want := computeMAC(seconds, method, target, body, a.signingKey)
	got, err := hex.DecodeString(mac)
	if err != nil {
		return errBadSignature
	}
	// Constant time, and length-checked by ConstantTimeCompare returning 0 on a
	// mismatch rather than panicking.
	if subtle.ConstantTimeCompare(want, got) != 1 {
		return errBadSignature
	}
	return nil
}

// computeMAC must stay byte-identical to the backend's ComputeSignature. The
// two are separately deployed binaries and the constant is written out on both
// sides on purpose — a shared module would hide the version skew the contract
// version exists to surface — so a change to one is a change to both.
func computeMAC(unix int64, method, target string, body, key []byte) []byte {
	m := hmac.New(sha256.New, key)
	fmt.Fprintf(m, "%d", unix)
	m.Write([]byte("."))
	m.Write([]byte(method))
	m.Write([]byte("."))
	m.Write([]byte(target))
	m.Write([]byte("."))
	m.Write(body)
	return m.Sum(nil)
}

// authenticated wraps a handler so no route can be reached unauthenticated.
//
// Applied to the whole mux rather than per route. One handler wrapped and the
// next one not is the arrangement that fails on the endpoint somebody adds in a
// hurry, and on this service that endpoint would be reachable by anything on
// the internal network.
func (a *authenticator) authenticated(next func(http.ResponseWriter, *http.Request, []byte)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := a.verify(r)
		if err != nil {
			// The reason is logged, never returned. A caller that cannot
			// authenticate has no business learning which half of the check it
			// failed, and the operator debugging a real misconfiguration reads
			// the add-on's log.
			logRefusal(r, err)
			writeError(w, http.StatusUnauthorized, CodeUnauthenticated, "The request is not signed by this add-on's caller.", nil)
			return
		}
		next(w, r, body)
	}
}
