package handlers

import (
	"context"
	"net/http"
	"os"
	"sync/atomic"
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
// request body before invoking `next`: the event listener's keyring Syndra
// read from Zitadel first (action-key-autorotation), then the key in
// secretEnvVar, which is only the bootstrap. A mismatch asks the keyring to
// read Zitadel again, as the claims door does, so no key change ever needs a
// restart.
//
// With no key at all it passes through only when ZITADEL_DOMAIN is also unset
// (dev mode); otherwise the request is refused with 503 MISCONFIGURED, so a
// production deployment never accepts unverified payloads (May 2026 audit
// C1). A missing, malformed, stale or mismatched signature is a 401
// INVALID_SIGNATURE with no detail.
func withZitadelActionSignature(secretEnvVar string, next http.HandlerFunc) http.HandlerFunc {
	keys := func() []claimserve.SlotKey { return eventSigningKeys(secretEnvVar) }
	refresh := func(ctx context.Context) []claimserve.SlotKey {
		rdb := claimConfigRedis()
		if rdb == nil {
			return keys()
		}
		return claimserve.AwaitNewKeys(ctx, rdb, &eventKeysRefreshed, keys, func() []claimserve.SlotKey {
			loadEventKeyring(ctx, rdb)
			return keys()
		})
	}
	return claimserve.SignatureCheck(secretEnvVar, keys, refresh, func() { nudgeKeyring() }, devModeNoZitadel, next)
}

// eventKeyring is the event listener target's keyring in this process: loaded
// from Redis (events:keyring), moved by the keyring follower and the rotator.
var (
	eventKeyring       atomic.Pointer[claimserve.Keyring]
	eventKeysRefreshed atomic.Int64
)

// eventSigningKeys is every key an event may be signed with: the keyring,
// then the env key.
func eventSigningKeys(envVar string) []claimserve.SlotKey {
	var ks []claimserve.SlotKey
	if k := eventKeyring.Load(); k != nil {
		ks = k.Keys(time.Now())
	}
	return claimserve.KeysWithEnv(ks, os.Getenv(envVar))
}

// loadEventKeyring reads events:keyring into this process; a failed or
// unreadable read keeps what it had.
func loadEventKeyring(ctx context.Context, rdb claimserve.Redis) {
	rctx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()
	if k, err := claimserve.ReadKeyring(rctx, rdb, claimserve.EventsKeyringKey); err == nil {
		eventKeyring.Store(&k)
	}
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
