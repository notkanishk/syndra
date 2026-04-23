package handlers

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
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
// warning and passes through without verification — matching the dev-mode
// fall-through already established by withUserAuth (no ZITADEL_DOMAIN set).
// Once the operator ships the Action target registration and captures the
// signing key, setting the env var enforces verification in production.
//
// The body is read once and rewound so the downstream handler's decoder still
// works. Requests with a missing, malformed, stale, or mismatched signature
// receive a 401 with code INVALID_SIGNATURE — no detail leakage.
func withZitadelActionSignature(secretEnvVar string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		secret := os.Getenv(secretEnvVar)
		if secret == "" {
			log.Printf("[ACTION] %s unset — signature verification disabled (dev mode)", secretEnvVar)
			next(w, r)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			jsonErrorResponse(w, http.StatusBadRequest, "BAD_REQUEST", "Failed to read request body")
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))

		sigHeader := r.Header.Get(zitadelSignatureHeader)
		if err := verifyZitadelActionSignature(body, sigHeader, secret, zitadelSignatureTolerance, time.Now()); err != nil {
			log.Printf("[ACTION] Signature verification failed: %v", err)
			jsonErrorResponse(w, http.StatusUnauthorized, "INVALID_SIGNATURE", "Invalid or missing Zitadel signature")
			return
		}

		next(w, r)
	}
}

// verifyZitadelActionSignature parses the ZITADEL-Signature header value,
// checks freshness against `now` using the given tolerance, and confirms at
// least one v1= signature matches HMAC-SHA256(<ts_decimal>.<body>, secret).
// Returns a specific error for each failure class; callers SHOULD log but
// MUST NOT propagate the detail to the response (to avoid oracle leaks).
func verifyZitadelActionSignature(body []byte, header, secret string, tolerance time.Duration, now time.Time) error {
	if header == "" {
		return fmt.Errorf("missing %s header", zitadelSignatureHeader)
	}

	var (
		ts         time.Time
		haveTS     bool
		signatures [][]byte
	)
	for _, pair := range strings.Split(header, zitadelSignatureSeparator) {
		key, value, ok := strings.Cut(pair, "=")
		if !ok {
			return fmt.Errorf("malformed signature pair: %q", pair)
		}
		switch strings.TrimSpace(key) {
		case zitadelSignatureTimestamp:
			n, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
			if err != nil {
				return fmt.Errorf("malformed timestamp: %w", err)
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
		return fmt.Errorf("signature header missing timestamp")
	}
	if len(signatures) == 0 {
		return fmt.Errorf("signature header missing v1 value")
	}
	if now.Sub(ts) > tolerance || ts.Sub(now) > tolerance {
		return fmt.Errorf("signature timestamp outside tolerance")
	}

	expected := computeZitadelActionSignature(ts, body, secret)
	for _, sig := range signatures {
		if hmac.Equal(expected, sig) {
			return nil
		}
	}
	return fmt.Errorf("no matching signature")
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
