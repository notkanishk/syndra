package handlers

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"syndra/internal/claimserve"
	"syndra/internal/claimtel"
)

// ZITADEL Actions v2 signs every target request with HMAC-SHA256 over
// "<unix_timestamp_decimal>.<raw_body>". The signature is delivered via a
// single header whose value is a comma-separated list of key=value pairs:
//
//	ZITADEL-Signature: t=1700000000,v1=<hex>[,v1=<hex_for_rotated_key>...]
//
// The algorithm is re-implemented here in stdlib (matching
// github.com/zitadel/zitadel-go/v3/pkg/actions/signing.go) to avoid adding a
// dependency surface and to let the middleware share decoder + response helpers
// with the rest of the handlers package.
const (
	zitadelSignatureHeader    = "ZITADEL-Signature"
	zitadelSignatureTimestamp = "t"
	zitadelSignatureVersion   = "v1"
	zitadelSignatureSeparator = ","
	// zitadelSignatureTolerance matches Zitadel's DefaultTolerance (300s).
	// Requests older than this are rejected as replays.
	zitadelSignatureTolerance = 300 * time.Second
)

// withZitadelActionSignature is middleware that verifies the Actions v2 HMAC
// signature on the request body before invoking `next`.
//
// When the env var named by secretEnvVar is unset, the middleware logs a
// warning and passes through without verification — fall-through allowed only
// when ZITADEL_DOMAIN is also unset (dev mode). If ZITADEL_DOMAIN is set but
// the signing key is empty, the request is refused with 503 MISCONFIGURED so a
// production deployment never accepts unverified webhook/action payloads (May
// 2026 audit C1). Once the operator ships the Action target registration and
// captures the signing key, setting the env var enforces verification in
// production.
//
// The body is read once and rewound so the downstream handler's decoder still
// works. Requests with a missing, malformed, stale, or mismatched signature
// receive a 401 with code INVALID_SIGNATURE — no detail leakage.
func withZitadelActionSignature(secretEnvVar string, next http.HandlerFunc) http.HandlerFunc {
	return withSignatureKeys(secretEnvVar, func() []claimserve.SlotKey {
		if k := os.Getenv(secretEnvVar); k != "" {
			return []claimserve.SlotKey{{Slot: claimtel.KeySlotCurrent, Key: k}}
		}
		return nil
	}, nil, next)
}

// withClaimsSignature is the claims target's check (S5): the keyring Syndra
// read from Zitadel first (current, then the previous key for 15 minutes after
// a rotation), then the env key, which stays the bootstrap and the fallback:
// with the Zitadel read denied, the env key alone still verifies. A mismatch
// on every key asks the keyring to read Zitadel again at once.
func withClaimsSignature(next http.HandlerFunc) http.HandlerFunc {
	return withSignatureKeys(actionKeyEnv, actionSigningKeys, nudgeKeyring, next)
}

// actionKeyEnv is the claims target's env key.
const actionKeyEnv = "ZITADEL_ACTION_SIGNING_KEY"

// actionSigningKeys is every key a claims call may be signed with, keyring
// first. The env key counts under its own slot only when the keyring does not
// already hold it.
func actionSigningKeys() []claimserve.SlotKey {
	keys := claimPlane.SigningKeys()
	env := os.Getenv(actionKeyEnv)
	if env == "" {
		return keys
	}
	for _, k := range keys {
		if k.Key == env {
			return keys
		}
	}
	return append(keys, claimserve.SlotKey{Slot: claimtel.KeySlotEnv, Key: env})
}

// withSignatureKeys verifies against each of keys() in turn. With no key at
// all it passes through only in dev mode (ZITADEL_DOMAIN unset), and refuses
// otherwise (May 2026 audit C1). onMismatch, when set, hears every call whose
// signature matched none of the keys.
func withSignatureKeys(envName string, keys func() []claimserve.SlotKey, onMismatch func(), next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ks := keys()
		if len(ks) == 0 {
			if os.Getenv("ZITADEL_DOMAIN") == "" {
				// Dev mode: no Zitadel deployment, no signing key.
				log.Printf("[ACTION] %s unset and ZITADEL_DOMAIN unset — signature verification disabled (dev mode)", envName)
				next(w, r)
				return
			}
			// Production with empty signing key: startup gate should have caught
			// this. Refuse the request rather than fall through silently. No key
			// can match, so it counts as a mismatch.
			claimtel.From(r.Context()).Set(claimtel.SigMismatch)
			log.Printf("[ACTION] %s unset while ZITADEL_DOMAIN set — refusing request (misconfiguration)", envName)
			jsonErrorResponse(w, http.StatusServiceUnavailable, "MISCONFIGURED", "Signing key not configured for this endpoint")
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			claimtel.From(r.Context()).Set(claimtel.BadRequest)
			jsonErrorResponse(w, http.StatusBadRequest, "BAD_REQUEST", "Failed to read request body")
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))

		slot, err := verifyZitadelActionSignatureKeys(body, r.Header.Get(zitadelSignatureHeader), ks, zitadelSignatureTolerance, time.Now())
		if err != nil {
			o := signatureOutcome(err)
			claimtel.From(r.Context()).Set(o)
			if o == claimtel.SigMismatch && onMismatch != nil {
				onMismatch()
			}
			log.Printf("[ACTION] Signature verification failed: %v", err)
			jsonErrorResponse(w, http.StatusUnauthorized, "INVALID_SIGNATURE", "Invalid or missing Zitadel signature")
			return
		}
		if call := claimtel.From(r.Context()); call != nil {
			call.KeySlot = slot
		}

		next(w, r)
	}
}

// The three ways a signature fails, as telemetry tells them apart: no header
// (not Zitadel), a timestamp outside tolerance (clock skew, or a replay), and
// everything else (a wrong key: every token loses its claims).
var (
	errSignatureMissing  = errors.New("signature missing")
	errSignatureStale    = errors.New("signature stale")
	errSignatureMismatch = errors.New("signature mismatch")
)

func signatureOutcome(err error) claimtel.Outcome {
	switch {
	case errors.Is(err, errSignatureMissing):
		return claimtel.SigMissing
	case errors.Is(err, errSignatureStale):
		return claimtel.SigStale
	}
	return claimtel.SigMismatch
}

// verifyZitadelActionSignature parses the ZITADEL-Signature header value,
// checks freshness against `now` using the given tolerance, and confirms at
// least one v1= signature matches HMAC-SHA256(<ts_decimal>.<body>, secret).
// Returns a specific error for each failure class; callers SHOULD log but
// MUST NOT propagate the detail to the response (to avoid oracle leaks).
func verifyZitadelActionSignature(body []byte, header, secret string, tolerance time.Duration, now time.Time) error {
	_, err := verifyZitadelActionSignatureKeys(body, header, []claimserve.SlotKey{{Slot: claimtel.KeySlotCurrent, Key: secret}}, tolerance, now)
	return err
}

// verifyZitadelActionSignatureKeys is verifyZitadelActionSignature over a
// keyring: the slot of the first key with a matching v1, in order.
func verifyZitadelActionSignatureKeys(body []byte, header string, keys []claimserve.SlotKey, tolerance time.Duration, now time.Time) (string, error) {
	if header == "" {
		return "", fmt.Errorf("%w: missing %s header", errSignatureMissing, zitadelSignatureHeader)
	}

	var (
		ts         time.Time
		haveTS     bool
		signatures [][]byte
	)
	for _, pair := range strings.Split(header, zitadelSignatureSeparator) {
		key, value, ok := strings.Cut(pair, "=")
		if !ok {
			return "", fmt.Errorf("%w: malformed signature pair: %q", errSignatureMismatch, pair)
		}
		switch strings.TrimSpace(key) {
		case zitadelSignatureTimestamp:
			n, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
			if err != nil {
				return "", fmt.Errorf("%w: malformed timestamp: %v", errSignatureMismatch, err)
			}
			ts = time.Unix(n, 0)
			haveTS = true
		case zitadelSignatureVersion:
			sig, err := hex.DecodeString(strings.TrimSpace(value))
			if err != nil {
				// One bad entry does not invalidate the header when a sibling v1 is valid.
				continue
			}
			signatures = append(signatures, sig)
		}
	}

	if !haveTS {
		return "", fmt.Errorf("%w: signature header missing timestamp", errSignatureMismatch)
	}
	if len(signatures) == 0 {
		return "", fmt.Errorf("%w: signature header missing v1 value", errSignatureMismatch)
	}
	if now.Sub(ts) > tolerance || ts.Sub(now) > tolerance {
		return "", fmt.Errorf("%w: signature timestamp outside tolerance (skew %s)", errSignatureStale, now.Sub(ts).Round(time.Second))
	}

	for _, k := range keys {
		expected := computeZitadelActionSignature(ts, body, k.Key)
		for _, sig := range signatures {
			if hmac.Equal(expected, sig) {
				return k.Slot, nil
			}
		}
	}
	return "", fmt.Errorf("%w: no matching signature", errSignatureMismatch)
}

// computeZitadelActionSignature returns the HMAC-SHA256 of
// "<unix_timestamp_decimal>.<body>" under secret, matching the algorithm in
// github.com/zitadel/zitadel-go/v3/pkg/actions.ComputeSignatureHeader.
func computeZitadelActionSignature(ts time.Time, body []byte, secret string) []byte {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = fmt.Fprintf(mac, "%d", ts.Unix())
	mac.Write([]byte("."))
	mac.Write(body)
	return mac.Sum(nil)
}
