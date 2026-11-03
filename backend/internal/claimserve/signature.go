package claimserve

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"syndra/internal/claimtel"
)

// ZITADEL Actions v2 signs every target request with HMAC-SHA256 over
// "<unix_timestamp_decimal>.<raw_body>". The signature is delivered via a
// single header whose value is a comma-separated list of key=value pairs:
//
//	ZITADEL-Signature: t=1700000000,v1=<hex>[,v1=<hex_for_rotated_key>...]
//
// Re-implemented in stdlib (matching
// github.com/zitadel/zitadel-go/v3/pkg/actions/signing.go) to avoid a
// dependency on the data plane's narrow image.
const (
	SignatureHeader = "ZITADEL-Signature"
	// SignatureTolerance matches Zitadel's DefaultTolerance (300s). Requests
	// older than this are rejected as replays.
	SignatureTolerance = 300 * time.Second
)

// The three ways a signature fails, as telemetry tells them apart: no header
// (not Zitadel), a timestamp outside tolerance (clock skew, or a replay), and
// everything else (a wrong key: every token loses its claims).
var (
	ErrSignatureMissing  = errors.New("signature missing")
	ErrSignatureStale    = errors.New("signature stale")
	ErrSignatureMismatch = errors.New("signature mismatch")
)

// SignatureOutcome is the outcome class of a failed verification.
func SignatureOutcome(err error) claimtel.Outcome {
	switch {
	case errors.Is(err, ErrSignatureMissing):
		return claimtel.SigMissing
	case errors.Is(err, ErrSignatureStale):
		return claimtel.SigStale
	}
	return claimtel.SigMismatch
}

// VerifySignature parses the ZITADEL-Signature header, checks freshness
// against now, and returns the slot of the first key, in order, with a
// matching v1. Each failure class is its own error; callers log it but never
// return its detail (no oracle).
func VerifySignature(body []byte, header string, keys []SlotKey, tolerance time.Duration, now time.Time) (string, error) {
	if header == "" {
		return "", fmt.Errorf("%w: missing %s header", ErrSignatureMissing, SignatureHeader)
	}
	var (
		ts         time.Time
		haveTS     bool
		signatures [][]byte
	)
	for _, pair := range strings.Split(header, ",") {
		key, value, ok := strings.Cut(pair, "=")
		if !ok {
			return "", fmt.Errorf("%w: malformed signature pair: %q", ErrSignatureMismatch, pair)
		}
		switch strings.TrimSpace(key) {
		case "t":
			n, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
			if err != nil {
				return "", fmt.Errorf("%w: malformed timestamp: %v", ErrSignatureMismatch, err)
			}
			ts, haveTS = time.Unix(n, 0), true
		case "v1":
			sig, err := hex.DecodeString(strings.TrimSpace(value))
			if err != nil {
				continue // one bad entry does not invalidate a valid sibling
			}
			signatures = append(signatures, sig)
		}
	}
	if !haveTS {
		return "", fmt.Errorf("%w: signature header missing timestamp", ErrSignatureMismatch)
	}
	if len(signatures) == 0 {
		return "", fmt.Errorf("%w: signature header missing v1 value", ErrSignatureMismatch)
	}
	if now.Sub(ts) > tolerance || ts.Sub(now) > tolerance {
		return "", fmt.Errorf("%w: signature timestamp outside tolerance (skew %s)", ErrSignatureStale, now.Sub(ts).Round(time.Second))
	}
	for _, k := range keys {
		expected := ComputeSignature(ts, body, k.Key)
		for _, sig := range signatures {
			if hmac.Equal(expected, sig) {
				return k.Slot, nil
			}
		}
	}
	return "", fmt.Errorf("%w: no matching signature", ErrSignatureMismatch)
}

// ComputeSignature is HMAC-SHA256("<unix_ts>.<body>") under secret.
func ComputeSignature(ts time.Time, body []byte, secret string) []byte {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = fmt.Fprintf(mac, "%d", ts.Unix())
	mac.Write([]byte("."))
	mac.Write(body)
	return mac.Sum(nil)
}

// MinKeyLen is the shortest signing key Syndra accepts in production. Zitadel
// generates its target keys as 32 letters and digits.
const MinKeyLen = 32

// KeyProblem says what is wrong with a signing key, or "" when nothing is: a
// key shorter than MinKeyLen, or with a byte outside printable ASCII (a pasted
// newline or space is the usual one), is refused at start (§4f).
func KeyProblem(key string) string {
	if len(key) < MinKeyLen {
		return fmt.Sprintf("is %d bytes, under %d", len(key), MinKeyLen)
	}
	for i := 0; i < len(key); i++ {
		if key[i] <= ' ' || key[i] > '~' {
			return fmt.Sprintf("has a non-printable or space byte at %d", i)
		}
	}
	return ""
}

// SignatureHeaderValue is the header Zitadel would send for body at ts: what
// the L1 probe signs its canary call with.
func SignatureHeaderValue(ts time.Time, body []byte, secret string) string {
	return fmt.Sprintf("t=%d,v1=%s", ts.Unix(), hex.EncodeToString(ComputeSignature(ts, body, secret)))
}
