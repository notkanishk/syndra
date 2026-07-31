package handlers

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

// Default threshold for the signing-key rotation warning. 90 days matches the
// cadence common compliance frameworks (SOC 2, ISO 27001) assume for shared
// credentials. Operators override via ZITADEL_ACTION_SIGNING_KEY_ROTATION_THRESHOLD_DAYS.
const defaultRotationThresholdDays = 90

// Env vars that feed the rotation-status endpoint.
//
//   - envActionKey           : the signing key itself. When unset the
//     backend is in dev-mode signature pass-through
//     and the rotation panel MUST reflect that —
//     otherwise a stale/lying "ok" would mask a
//     production misconfiguration.
//   - envActionKeyRotatedAt  : RFC3339 UTC timestamp emitted by rotate.sh.
//   - envActionKeyThresholdDays : integer; warn threshold.
const (
	envActionKey              = "ZITADEL_ACTION_SIGNING_KEY"
	envActionKeyRotatedAt     = "ZITADEL_ACTION_SIGNING_KEY_ROTATED_AT"
	envActionKeyThresholdDays = "ZITADEL_ACTION_SIGNING_KEY_ROTATION_THRESHOLD_DAYS"
)

// ActionRotationStatus mirrors the JSON response of
// GET /api/v1/zitadel/action-rotation-status.
//
// Status ladder (highest precedence first):
//   - disabled : ZITADEL_ACTION_SIGNING_KEY is unset. Signature verification
//     is passing every Action request through unchecked; no
//     rotation metric is meaningful until a real key is installed.
//   - unknown  : key is installed, but rotation timestamp is unset, not
//     RFC3339-parseable, or in the future (clock skew / typo).
//   - ok       : key installed, age < threshold
//   - warn     : key installed, threshold <= age < 2*threshold
//   - stale    : key installed, age >= 2*threshold
//
// KeyInstalled is reported independently of status so the UI can explain
// the "disabled" state to operators without having to special-case the
// string value. `rotate_command` is the verbatim shell command an operator
// would paste — surfaced in the response so the UI never hardcodes it.
type ActionRotationStatus struct {
	KeyInstalled  bool       `json:"key_installed"`
	LastRotatedAt *time.Time `json:"last_rotated_at,omitempty"`
	AgeDays       *int       `json:"age_days,omitempty"`
	ThresholdDays int        `json:"threshold_days"`
	Status        string     `json:"status"`
	RotateCommand string     `json:"rotate_command"`
}

// HandleActionRotationStatus reports the state of the currently installed
// Action signing key: whether one is installed at all, and if so how old it
// is relative to the configured rotation threshold. Read-only; no side
// effects. Gated by withOperatorAuth at the router so only admins hit it.
//
// Zitadel does not expire the signing key on its own (see application-claims
// spec § "Signing Key Rotation is Policy-Driven, Not Scheduled"), so this
// endpoint is how operators get observability without scheduling rotation.
func HandleActionRotationStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonErrorResponse(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET is supported")
		return
	}

	keyInstalled := os.Getenv(envActionKey) != ""
	resp := ActionRotationStatus{
		KeyInstalled:  keyInstalled,
		ThresholdDays: rotationThresholdDays(),
		RotateCommand: "make zitadel-actions-rotate-key",
	}

	// Precedence 1: no signing key installed -> signature verification is off.
	// A rotation timestamp under these conditions is not actionable and must
	// not be allowed to render as "ok" — that would mask the misconfiguration.
	if !keyInstalled {
		resp.Status = "disabled"
		jsonResponse(w, http.StatusOK, resp)
		return
	}

	raw := os.Getenv(envActionKeyRotatedAt)
	if raw == "" {
		resp.Status = "unknown"
		jsonResponse(w, http.StatusOK, resp)
		return
	}

	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		log.Printf("[ACTION] %s is set but not RFC3339: %v", envActionKeyRotatedAt, err)
		resp.Status = "unknown"
		jsonResponse(w, http.StatusOK, resp)
		return
	}

	now := nowFn()
	// Precedence 2: future-dated timestamps are a config error, not a fresh
	// rotation. Treating them as "ok, age 0" would suppress the warn/stale
	// ladder indefinitely (a typo in the year, or host clock skew, could hide
	// an old key forever).
	if t.After(now) {
		log.Printf("[ACTION] %s=%q is in the future (%.0fh ahead); reporting unknown",
			envActionKeyRotatedAt, raw, t.Sub(now).Hours())
		resp.Status = "unknown"
		jsonResponse(w, http.StatusOK, resp)
		return
	}

	age := ageInDays(now, t)
	resp.LastRotatedAt = &t
	resp.AgeDays = &age
	resp.Status = classifyRotationStatus(age, resp.ThresholdDays)

	jsonResponse(w, http.StatusOK, resp)
}

// rotationThresholdDays reads the operator-configured threshold or falls back
// to the default. Non-positive values fall back too — a zero threshold would
// flag every install as warn the moment it lands, which is never what the
// operator meant.
func rotationThresholdDays() int {
	v := os.Getenv(envActionKeyThresholdDays)
	if v == "" {
		return defaultRotationThresholdDays
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		log.Printf("[ACTION] %s=%q invalid (want positive int); using default %d", envActionKeyThresholdDays, v, defaultRotationThresholdDays)
		return defaultRotationThresholdDays
	}
	return n
}

// classifyRotationStatus returns "ok", "warn", or "stale" from an age and
// threshold. Broken out so tests can assert the boundary conditions directly.
// Callers MUST have already filtered out future-dated timestamps; this helper
// assumes a non-negative age.
func classifyRotationStatus(ageDays, thresholdDays int) string {
	switch {
	case ageDays >= 2*thresholdDays:
		return "stale"
	case ageDays >= thresholdDays:
		return "warn"
	default:
		return "ok"
	}
}

// ageInDays returns the floor number of 24-hour periods between then and now.
// Negative results (timestamps in the future) are returned as-is — the
// handler explicitly filters those out above and treats them as "unknown".
// Keeping this function pure makes the boundary behaviour obvious at the
// call site instead of hidden in a silent clamp.
func ageInDays(now, then time.Time) int {
	return int(now.Sub(then) / (24 * time.Hour))
}

// nowFn is overridable in tests.
var nowFn = time.Now
