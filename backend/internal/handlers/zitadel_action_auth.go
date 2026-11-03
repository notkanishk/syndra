package handlers

import (
	"net/http"
	"os"
	"time"

	"syndra/internal/claimserve"
	"syndra/internal/claimtel"
)

// ZITADEL Actions v2 signs every target request with HMAC-SHA256 over
// "<unix_timestamp_decimal>.<raw_body>" in the ZITADEL-Signature header
// (claimserve.VerifySignature). The check is shared with the claims
// container, so both doors refuse exactly the same requests.
const (
	zitadelSignatureHeader    = claimserve.SignatureHeader
	zitadelSignatureTolerance = claimserve.SignatureTolerance
)

func devModeNoZitadel() bool { return os.Getenv("ZITADEL_DOMAIN") == "" }

// withZitadelActionSignature verifies the Actions v2 HMAC signature on the
// request body before invoking `next`, with the key in secretEnvVar.
//
// With the key unset it passes through only when ZITADEL_DOMAIN is also unset
// (dev mode); otherwise the request is refused with 503 MISCONFIGURED, so a
// production deployment never accepts unverified payloads (May 2026 audit
// C1). A missing, malformed, stale or mismatched signature is a 401
// INVALID_SIGNATURE with no detail.
func withZitadelActionSignature(secretEnvVar string, next http.HandlerFunc) http.HandlerFunc {
	return claimserve.SignatureCheck(secretEnvVar, func() []claimserve.SlotKey {
		if k := os.Getenv(secretEnvVar); k != "" {
			return []claimserve.SlotKey{{Slot: claimtel.KeySlotCurrent, Key: k}}
		}
		return nil
	}, nil, devModeNoZitadel, next)
}

// withClaimsSignature is the claims target's check (S5): the keyring Syndra
// read from Zitadel first (current, then the previous key for 15 minutes after
// a rotation), then the env key, which stays the bootstrap and the fallback.
// A mismatch on every key asks the keyring to read Zitadel again at once.
func withClaimsSignature(next http.HandlerFunc) http.HandlerFunc { return claimDoor.Signature(next) }

// withClaimDelivery wraps the WHOLE data-plane route, signature check
// included, so every call is counted exactly once (claimserve.Door.Delivery).
func withClaimDelivery(next http.HandlerFunc) http.HandlerFunc { return claimDoor.Delivery(next) }

// actionKeyEnv is the claims target's env key.
const actionKeyEnv = "ZITADEL_ACTION_SIGNING_KEY"

// actionSigningKeys is every key a claims call may be signed with, keyring
// first, then the env key.
func actionSigningKeys() []claimserve.SlotKey {
	return claimserve.KeysWithEnv(claimPlane.SigningKeys(), os.Getenv(actionKeyEnv))
}

// verifyZitadelActionSignature checks one key; see claimserve.VerifySignature.
func verifyZitadelActionSignature(body []byte, header, secret string, tolerance time.Duration, now time.Time) error {
	_, err := claimserve.VerifySignature(body, header, []claimserve.SlotKey{{Slot: claimtel.KeySlotCurrent, Key: secret}}, tolerance, now)
	return err
}

var (
	verifyZitadelActionSignatureKeys = claimserve.VerifySignature
	computeZitadelActionSignature    = claimserve.ComputeSignature
)
