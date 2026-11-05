package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"syndra/internal/claimcheck"
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
	// LastRotatedSource says where LastRotatedAt was observed: "syndra"
	// (Syndra's own record of replacing it), "keyring" (Syndra saw Zitadel's
	// key change, e.g. a manual rotate.sh) or "env" (the .env bootstrap date,
	// only until either of the others has one).
	LastRotatedSource string `json:"last_rotated_source,omitempty"`
	AgeDays           *int   `json:"age_days,omitempty"`
	ThresholdDays     int    `json:"threshold_days"`
	Status            string `json:"status"`
	RotateCommand     string `json:"rotate_command"`
	// RegisterCommand sets a key up when none is (the guide's setup path).
	RegisterCommand string     `json:"register_command"`
	DueAt           *time.Time `json:"due_at,omitempty"`
	// Auto is the schedule (action-key-autorotation).
	Auto autoRotateStatus `json:"auto"`
	// Rotating is whether a replacement is running now.
	Rotating    bool                   `json:"rotating"`
	LastOutcome *rotationOutcomeStatus `json:"last_outcome,omitempty"`
}

type autoRotateStatus struct {
	On            bool       `json:"on"`
	OffReason     string     `json:"off_reason,omitempty"`
	LeadDays      int        `json:"lead_days"`
	Window        string     `json:"window"`
	NextAttemptAt *time.Time `json:"next_attempt_at,omitempty"`
	// WaitingFor is what keeps the schedule from replacing the key even when
	// it is due (W5): "test_sign_in_setup" (no test sign-in can run, because
	// SYNDRA_EXTERNAL_URL is unset) or "test_sign_in" (none has passed in the
	// last probeFreshFor). Empty when the schedule would act at its time.
	WaitingFor string `json:"waiting_for,omitempty"`
}

// scheduleWaitingFor is what the schedule waits on before it may replace the
// key, the same test RunActionKeyAutoRotate applies; "" when nothing.
func scheduleWaitingFor(now time.Time) string {
	switch {
	case probeRecentlyPassed(now):
		return ""
	case !ProbeL1Enabled():
		return "test_sign_in_setup"
	}
	return "test_sign_in"
}

// rotationOutcomeStatus is Syndra's latest attempt, without its id or actor.
type rotationOutcomeStatus struct {
	Trigger    string          `json:"trigger"`
	StartedAt  time.Time       `json:"started_at"`
	FinishedAt *time.Time      `json:"finished_at,omitempty"`
	Outcome    string          `json:"outcome"`
	Replaced   []string        `json:"replaced"`
	Probe      json.RawMessage `json:"probe,omitempty"`
	// Reason is the attempt's plain sentence (rotationResult.Reason).
	Reason string `json:"reason,omitempty"`
}

// HandleActionRotationStatus reports the Action signing key: whether one is
// installed, how old it is (observed: observedLastReplaced), when it is due,
// the schedule that replaces it (action-key-autorotation) and Syndra's last
// attempt. Read-only. Gated by withOperatorAuth at the router.
//
// Zitadel does not expire the signing key on its own; the threshold is
// Syndra's policy, and since 5 Nov 2026 Syndra keeps it (ADR-005).
func HandleActionRotationStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonErrorResponse(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET is supported")
		return
	}

	// The by-hand guide watches for the new key: ?watch=1 asks Zitadel for it
	// now (RunKeyring keeps that to one read per 10s, whoever asks).
	if r.URL.Query().Get("watch") == "1" {
		nudgeKeyring()
	}
	keyInstalled := len(actionSigningKeys()) > 0
	c := autoConfig()
	resp := ActionRotationStatus{
		KeyInstalled:    keyInstalled,
		ThresholdDays:   rotationThresholdDays(),
		RotateCommand:   serverCommand("make zitadel-actions-rotate-key"),
		RegisterCommand: serverCommand("make zitadel-actions-register"),
		Auto:            autoRotateStatus{On: c.On, OffReason: c.OffReason, LeadDays: c.LeadDays, Window: strings.Replace(c.Window, "-", "–", 1)},
	}

	// Precedence 1: no signing key installed -> signature verification is off.
	// A rotation timestamp under these conditions is not actionable and must
	// not be allowed to render as "ok" — that would mask the misconfiguration.
	if !keyInstalled {
		resp.Status = "disabled"
		jsonResponse(w, http.StatusOK, resp)
		return
	}

	st, err := dbActionKeyRotationNow(r.Context(), claimcheck.ClaimTarget)
	if err != nil {
		log.Printf("[ACTION] could not read Syndra's key replacement record: %v", err)
	}
	if l := st.Last; l != nil {
		resp.Rotating = l.Outcome == "in_flight"
		resp.LastOutcome = &rotationOutcomeStatus{Trigger: l.Trigger, StartedAt: l.StartedAt, FinishedAt: l.FinishedAt,
			Outcome: l.Outcome, Replaced: l.Replaced, Probe: l.Probe, Reason: l.Error}
	}

	now := nowFn()
	t, source := observedLastReplaced(st)
	if c.On {
		next := nextAutoAttempt(now, t, st.LastAutoAt, c)
		resp.Auto.NextAttemptAt = &next
		resp.Auto.WaitingFor = scheduleWaitingFor(now)
	}
	// Precedence 2: no observed date (none recorded, the .env one unset,
	// malformed or in the future: a typo in the year must never hide an old
	// key) is "unknown", which the schedule treats as due.
	if t == nil {
		if raw := os.Getenv(envActionKeyRotatedAt); raw != "" {
			log.Printf("[ACTION] %s=%q is not a past RFC3339 time; reporting unknown", envActionKeyRotatedAt, raw)
		}
		resp.Status = "unknown"
		jsonResponse(w, http.StatusOK, resp)
		return
	}

	age := ageInDays(now, *t)
	due := t.Add(time.Duration(resp.ThresholdDays) * 24 * time.Hour)
	resp.LastRotatedAt, resp.LastRotatedSource, resp.AgeDays, resp.DueAt = t, source, &age, &due
	resp.Status = classifyRotationStatus(age, resp.ThresholdDays)

	jsonResponse(w, http.StatusOK, resp)
}

// envServerDir is the Syndra checkout on the server (where make runs). The
// backend runs in a container and cannot see it, so compose passes it in.
const envServerDir = "SYNDRA_SERVER_DIR"

// serverCommand is cmd as the operator pastes it on the server: prefixed
// with the checkout when it is known, so the guide never types a path.
func serverCommand(cmd string) string {
	dir := strings.TrimSpace(os.Getenv(envServerDir))
	if dir == "" || !strings.HasPrefix(dir, "/") || strings.ContainsAny(dir, " '\"`$;&|\n") {
		return cmd
	}
	return "cd " + dir + " && " + cmd
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
