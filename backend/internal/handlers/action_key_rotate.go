package handlers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"syndra/internal/claimcheck"
	"syndra/internal/claimserve"
	"syndra/internal/claimtel"
	"syndra/internal/clock"
	"syndra/internal/db"
	"syndra/internal/extcall"
	"syndra/internal/zitadel"
)

// Syndra replaces the Zitadel Actions v2 signing keys itself
// (action-key-autorotation, ADR-005; owner, 5 Nov 2026: "Automatic +
// button"). A few days before the limit, in a quiet window, it asks Zitadel
// for new keys for its two targets, puts each into the keyring from Zitadel's
// own answer before anything else happens, runs the test sign-in, and records
// what came of it. "Replace now" does the same on demand.
//
// The order, and why:
//  1. The lock (a Postgres advisory lock): two processes or two clicks never
//     replace twice.
//  2. A refusal when the key was replaced in the last 24h, unless forced.
//  3. Zitadel answers the target read, or nothing is attempted.
//  4. The trace (action_key_rotations, in_flight) BEFORE the first write.
//  5. Each target: Zitadel's answer carries the new key; it is published to
//     the keyring at once (both processes), then recorded as replaced.
//  6. The test sign-in (L1, and L2 when set up) and the outcome. A failure
//     keeps both keys accepted, raises KEY_ROTATION and never retries on its
//     own: the next automatic attempt waits for the next due date.

const (
	envAutoRotate  = "SYNDRA_ACTION_KEY_AUTOROTATE" // "off" is the kill switch
	envLeadDays    = "SYNDRA_ACTION_KEY_LEAD_DAYS"
	envQuietWindow = "SYNDRA_ACTION_KEY_WINDOW" // "HH:MM-HH:MM", in the makerspace's zone (clock.Loc: the deployment's TZ)

	defaultLeadDays    = 5
	defaultQuietWindow = "03:00-05:00"

	// recentReplaceGuard refuses a second replacement within a day unless the
	// owner forces it from the button.
	recentReplaceGuard = 24 * time.Hour
	// autoAttemptGap: at most one automatic attempt per quiet window, whatever
	// its outcome, so a failure never becomes a loop.
	autoAttemptGap = 20 * time.Hour
	// probeFreshFor: the automatic attempt runs only when L1 passed this
	// recently, so a failed test sign-in after it is the replacement's doing.
	probeFreshFor = 5 * time.Minute
	// unverifiedHold keeps the previous key accepted after a replacement the
	// test sign-in did not confirm.
	unverifiedHold = 7 * 24 * time.Hour
	// rotationMarkFor bounds the in-flight mark the doors wait on.
	rotationMarkFor = 30 * time.Second
	// ActionKeyAutoEvery is how often the schedule looks.
	ActionKeyAutoEvery = 5 * time.Minute
)

// rotatedTargets are Syndra's two targets (targets.json; TestManifestMatchesTargetsJSON
// keeps the names equal), the claims one first.
var rotatedTargets = []string{claimcheck.ClaimTarget, claimcheck.EventTarget}

var (
	errRotationInFlight = errors.New("another replacement is running")
	errRecentlyReplaced = errors.New("the key was replaced less than a day ago")
	errZitadelDown      = errors.New("zitadel didn't answer, so nothing was changed")
	errNoClaimTarget    = errors.New("zitadel has no claims target to replace a key on")
)

// rotationResult is what one replacement came to. Outcome:
//
//	verified   : every key replaced, and the test sign-in passed
//	unverified : every key replaced, the test sign-in did not pass (or could not run)
//	partial    : one key replaced, the other not (refused, or its answer lost)
//	unknown    : nothing confirmed replaced; Zitadel's answer was lost and the
//	             read-back failed, so it may have been
//	failed     : Zitadel replaced nothing
//
// Reason is a plain sentence for the screen and the audit row: never an HTTP
// status or upstream text (it travels in a 200 body, past the error envelope).
type rotationResult struct {
	Outcome    string       `json:"outcome"`
	ReplacedAt *time.Time   `json:"replaced_at,omitempty"`
	Replaced   []string     `json:"replaced"`
	Probe      *probeResult `json:"probe,omitempty"`
	NextDueAt  *time.Time   `json:"next_due_at,omitempty"`
	Reason     string       `json:"reason,omitempty"`
}

// rotationProbe is the test sign-in after a replacement: L1 at once, and L2
// when the canary is set up (the one that proves Zitadel signs with the key
// Syndra holds). Each records itself as its schedule would.
var rotationProbe = func(ctx context.Context) probeResult {
	if !ProbeL1Enabled() {
		return probeResult{At: time.Now(), Class: "not_set_up", Detail: "SYNDRA_EXTERNAL_URL is unset, so no test sign-in can run"}
	}
	_ = RunProbeL1(ctx)
	res := lastProbeL1.Load()
	if res == nil || !res.Pass || !ProbeL2Enabled() {
		if res == nil {
			return probeResult{At: time.Now(), Class: "unreachable", Detail: "the test sign-in did not run"}
		}
		return *res
	}
	_ = RunProbeL2(ctx)
	if l2 := lastProbeL2.Load(); l2 != nil {
		return *l2
	}
	return *res
}

// rotateActionKeys replaces every target's key once. A refusal (in flight,
// recent, Zitadel down, no target) changes nothing and writes nothing.
func rotateActionKeys(ctx context.Context, trigger, actor string, force bool) (rotationResult, error) {
	rotator, reader := zitadelKeyRotator(), zitadelActions()
	if rotator == nil || reader == nil {
		return rotationResult{}, errNoClient
	}
	release, ok, err := dbTryActionKeyRotationLock(ctx)
	if err != nil {
		return rotationResult{}, err
	}
	if !ok {
		return rotationResult{}, errRotationInFlight
	}
	defer release()
	// Holding the lock, no attempt is running: a leftover in_flight row is a
	// crashed one.
	if err := dbAbandonActionKeyRotations(ctx); err != nil {
		return rotationResult{}, err
	}
	st, err := dbActionKeyRotationNow(ctx, claimcheck.ClaimTarget)
	if err != nil {
		return rotationResult{}, err
	}
	if !force && st.LastReplacedAt != nil && nowFn().Sub(*st.LastReplacedAt) < recentReplaceGuard {
		return rotationResult{}, errRecentlyReplaced
	}

	lctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	targets, err := reader.ListActionTargets(lctx)
	cancel()
	if err != nil {
		return rotationResult{}, fmt.Errorf("%w (%v)", errZitadelDown, err)
	}
	ids, before := map[string]string{}, map[string]string{}
	for _, t := range targets {
		ids[t.Name], before[t.Name] = t.ID, t.SigningKey
	}
	if ids[claimcheck.ClaimTarget] == "" {
		return rotationResult{}, errNoClaimTarget
	}
	var plan []string
	for _, name := range rotatedTargets {
		if ids[name] != "" {
			plan = append(plan, name)
		}
	}

	// The trace, before the first write. No trace, no write.
	id, err := dbStartActionKeyRotation(ctx, trigger, actor, plan)
	if err != nil {
		return rotationResult{}, err
	}
	rdb := claimConfigRedis()
	if rdb != nil {
		_ = claimserve.MarkRotation(ctx, rdb, time.Now().Add(rotationMarkFor))
	}
	res := rotationResult{Replaced: []string{}}
	newKeys := map[string]string{}
	// cause is why the attempt stopped short, in a sentence; lost is whether
	// nobody knows if Zitadel replaced the key it stopped on.
	var cause string
	var lost bool
	for _, name := range plan {
		rctx, cancel := context.WithTimeout(ctx, 15*time.Second)
		key, err := rotator.RotateTargetSigningKey(rctx, ids[name])
		cancel()
		if err != nil {
			log.Printf("[ACTION] replacing %s: %v", name, err)
			if !keyAnswerLost(err) {
				cause = keyRefusal(err)
				break
			}
			// Zitadel may have replaced it and the answer was lost: read the key
			// back now rather than guess.
			key, err = readBackKey(ctx, name, before[name])
			if err != nil {
				log.Printf("[ACTION] reading %s back: %v", name, err)
				cause, lost = "Zitadel didn't answer, and Syndra couldn't read the key back, so it can't tell whether Zitadel replaced it. Syndra reads the keys from Zitadel again every minute and follows whatever it finds.", true
				break
			}
			if key == "" {
				cause = "Zitadel didn't answer. Syndra read the key again: it didn't change."
				break
			}
			log.Printf("[ACTION] %s: the answer was lost, but the read-back shows a new key", name)
		}
		// The key first: every door verifies with it before anything else. A
		// publish that fails is caught by the check below (Syndra not yet
		// verifying with the new key), and the keyring reads it within a minute.
		if err := adoptKey(ctx, name, key, time.Now()); err != nil {
			log.Printf("[ACTION] could not publish the new key for %s: %v", name, err)
		}
		newKeys[name] = key
		res.Replaced = append(res.Replaced, name)
		if err := dbMarkActionKeyReplaced(ctx, id, name); err != nil {
			log.Printf("[ACTION] could not record that %s was replaced (attempt %d): %v", name, id, err)
		}
	}
	if rdb != nil {
		_ = claimserve.MarkRotation(ctx, rdb, time.Time{})
	}
	now := time.Now()
	if len(res.Replaced) > 0 {
		res.ReplacedAt = &now
	}

	switch {
	case len(res.Replaced) == 0 && lost:
		res.Outcome, res.Reason = "unknown", cause
	case len(res.Replaced) == 0:
		res.Outcome, res.Reason = "failed", cause
	case cause != "":
		res.Outcome = "partial"
		res.Reason = fmt.Sprintf("Zitadel replaced %s, but not %s. %s", keyWords(res.Replaced), keyWords(slices.DeleteFunc(slices.Clone(plan),
			func(n string) bool { return slices.Contains(res.Replaced, n) })), cause)
	}
	if len(res.Replaced) > 0 {
		probe := verifyRotation(ctx, newKeys)
		res.Probe = &probe
		switch {
		case res.Outcome != "":
		case probe.Pass:
			res.Outcome = "verified"
		case probe.Class == "not_set_up":
			res.Outcome, res.Reason = "unverified", "Replaced. No test sign-in could run, because SYNDRA_EXTERNAL_URL is not set."
		default:
			res.Outcome, res.Reason = "unverified", "The test sign-in after the replacement did not get its claims: "+probe.Detail
		}
		if !probe.Pass {
			// Both keys stay accepted while somebody looks.
			for name := range newKeys {
				if err := moveKeyring(ctx, name, func(k claimserve.Keyring) claimserve.Keyring { return k.Hold(now.Add(unverifiedHold)) }); err != nil {
					log.Printf("[ACTION] could not keep the previous key for %s: %v", name, err)
				}
			}
		}
	}
	if err := dbFinishActionKeyRotation(ctx, id, res.Outcome, res.Probe, res.Reason); err != nil {
		log.Printf("[ACTION] could not record the outcome of attempt %d: %v", id, err)
	}
	recordRotationFinding(ctx, res.Outcome, res.Reason, now)
	details := map[string]any{"trigger": trigger, "outcome": res.Outcome, "replaced": res.Replaced, "targets": plan}
	if res.Probe != nil {
		details["probe"] = map[string]any{"pass": res.Probe.Pass, "class": res.Probe.Class}
	}
	if res.Reason != "" {
		details["reason"] = res.Reason
	}
	auditAction := "action_key.replaced" // verified, unverified
	switch res.Outcome {
	case "partial":
		auditAction = "action_key.partly_replaced"
	case "unknown":
		auditAction = "action_key.replace_unknown"
	case "failed":
		auditAction = "action_key.not_replaced"
	}
	if err := dbInsertAuditLogDetails(ctx, actor, "", auditAction, strconv.FormatInt(id, 10), details); err != nil {
		log.Printf("[ACTION] could not audit attempt %d: %v", id, err)
	}
	if res.ReplacedAt != nil && newKeys[claimcheck.ClaimTarget] != "" {
		due := res.ReplacedAt.Add(time.Duration(rotationThresholdDays()) * 24 * time.Hour)
		res.NextDueAt = &due
	}
	log.Printf("[ACTION] %s replacement (attempt %d): %s; replaced %v", trigger, id, res.Outcome, res.Replaced)
	return res, nil
}

// verifyRotation is the check after a replacement: this process verifies
// with each new key (it was published), then the test sign-in.
func verifyRotation(ctx context.Context, newKeys map[string]string) probeResult {
	if k := newKeys[claimcheck.ClaimTarget]; k != "" && firstKey(actionSigningKeys()) != k {
		return probeResult{At: time.Now(), Class: "keyring", Detail: "Syndra is not verifying with the new claims key yet"}
	}
	if k := newKeys[claimcheck.EventTarget]; k != "" && firstKey(eventSigningKeys(eventKeyEnv)) != k {
		return probeResult{At: time.Now(), Class: "keyring", Detail: "Syndra is not verifying events with the new key yet"}
	}
	return rotationProbe(ctx)
}

const eventKeyEnv = "ZITADEL_EVENT_SIGNING_KEY"

func firstKey(ks []claimserve.SlotKey) string {
	if len(ks) == 0 {
		return ""
	}
	return ks[0].Key
}

// recordRotationFinding raises KEY_ROTATION for an attempt that did not end
// verified, and settles it when one did.
func recordRotationFinding(ctx context.Context, outcome, detail string, at time.Time) {
	var err error
	if outcome == "verified" {
		err = dbSettleUnseenClaimFindings(ctx, claimtel.KindKeyRotation, []string{"unverified", "partial", "unknown", "failed"}, []string{""})
		err = errors.Join(err, refreshClaimFindings(ctx))
	} else {
		err = applyClaimFindings(ctx, []claimtel.Observation{{Kind: claimtel.KindKeyRotation, Subject: outcome, Fail: true, At: at,
			Sample: map[string]any{"detail": detail}}}, claimtel.ResolveAfterReads)
	}
	if err != nil {
		log.Printf("[ACTION] could not record the finding: %v", err)
	}
}

// rotationReason is a Zitadel error in a few plain words: never a key, a
// token, an HTTP status or Zitadel's own text (those go to the log).
func rotationReason(err error) string {
	var se *zitadel.StatusError
	switch {
	case keyAnswerLost(err):
		return "Zitadel didn't answer"
	case errors.As(err, &se) && se.Code == http.StatusForbidden:
		return "Zitadel doesn't allow Syndra's service account to do this"
	case errors.As(err, &se) && se.Code == http.StatusNotFound:
		return "Zitadel couldn't find the target"
	}
	return "Zitadel refused"
}

// keyRefusal is why Zitadel's final answer to a key replacement was no, and
// what to do about it.
func keyRefusal(err error) string {
	var se *zitadel.StatusError
	if errors.As(err, &se) && se.Code == http.StatusForbidden {
		return "Zitadel refused because Syndra's service account is not allowed to change Actions targets. Give it action.target.write, then Replace now."
	}
	return rotationReason(err) + "."
}

// keyAnswerLost is whether err leaves the key's state unknown: no answer, or
// a status that means "try again" (408, 429, 5xx).
func keyAnswerLost(err error) bool {
	var se *zitadel.StatusError
	return !errors.As(err, &se) || extcall.RetryableStatus(se.Code)
}

// readBackKey is name's key as Zitadel holds it now when it differs from
// was, "" when it is unchanged, or the read's error.
func readBackKey(ctx context.Context, name, was string) (string, error) {
	rctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	keys, err := readTargetKeys(rctx)
	if err != nil {
		return "", err
	}
	if k := keys[name]; k != "" && k != was {
		return k, nil
	}
	return "", nil
}

// keyWords names keys the way the screen does: "the claims action's key and
// the event listener's key".
func keyWords(targets []string) string {
	words := make([]string, 0, len(targets))
	for _, t := range targets {
		switch t {
		case claimcheck.ClaimTarget:
			words = append(words, "the claims action's key")
		case claimcheck.EventTarget:
			words = append(words, "the event listener's key")
		default:
			words = append(words, "the key for "+t)
		}
	}
	return strings.Join(words, " and ")
}

// ---------------------------------------------------------------------------
// The schedule.
// ---------------------------------------------------------------------------

// autoRotateConfig is the schedule as configured.
type autoRotateConfig struct {
	On        bool
	OffReason string // switched_off | no_zitadel | stand_in
	Threshold time.Duration
	Lead      time.Duration
	LeadDays  int
	Window    string
	start     int // minutes after local midnight
	end       int
}

func autoConfig() autoRotateConfig {
	threshold := rotationThresholdDays()
	c := autoRotateConfig{On: true, Threshold: time.Duration(threshold) * 24 * time.Hour, LeadDays: defaultLeadDays}
	if v := os.Getenv(envLeadDays); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 && n < threshold {
			c.LeadDays = n
		} else {
			log.Printf("[ACTION] %s=%q invalid (want 0 to %d days); using %d", envLeadDays, v, threshold-1, defaultLeadDays)
		}
	}
	if c.LeadDays >= threshold {
		c.LeadDays = threshold - 1
	}
	c.Lead = time.Duration(c.LeadDays) * 24 * time.Hour
	c.Window = os.Getenv(envQuietWindow)
	var ok bool
	if c.start, c.end, ok = parseQuietWindow(c.Window); !ok {
		if c.Window != "" {
			log.Printf("[ACTION] %s=%q invalid (want HH:MM-HH:MM); using %s", envQuietWindow, c.Window, defaultQuietWindow)
		}
		c.Window = defaultQuietWindow
		c.start, c.end, _ = parseQuietWindow(c.Window)
	}
	switch strings.ToLower(strings.TrimSpace(os.Getenv(envAutoRotate))) {
	case "off", "false", "0", "no":
		c.On, c.OffReason = false, "switched_off"
	}
	// Staging's stand-in has no actions and so no key: nothing to rotate.
	if zitadel.IsStandIn() {
		c.On, c.OffReason = false, "stand_in"
	} else if c.On && (devModeNoZitadel() || zitadelKeyRotator() == nil) {
		c.On, c.OffReason = false, "no_zitadel"
	}
	return c
}

// parseQuietWindow reads "HH:MM-HH:MM" (may cross midnight) as minutes.
func parseQuietWindow(s string) (start, end int, ok bool) {
	a, b, found := strings.Cut(strings.TrimSpace(s), "-")
	if !found {
		return 0, 0, false
	}
	clock := func(v string) (int, bool) {
		t, err := time.Parse("15:04", strings.TrimSpace(v))
		return t.Hour()*60 + t.Minute(), err == nil
	}
	start, ok1 := clock(a)
	end, ok2 := clock(b)
	return start, end, ok1 && ok2 && start != end
}

// inWindow is whether t's clock in the makerspace's zone falls in the window.
func (c autoRotateConfig) inWindow(t time.Time) bool {
	t = t.In(clock.Loc)
	m := t.Hour()*60 + t.Minute()
	if c.start < c.end {
		return m >= c.start && m < c.end
	}
	return m >= c.start || m < c.end
}

// windowFrom is t when it is in the window, else the window's next start.
func (c autoRotateConfig) windowFrom(t time.Time) time.Time {
	if c.inWindow(t) {
		return t
	}
	t = t.In(clock.Loc)
	start := time.Date(t.Year(), t.Month(), t.Day(), c.start/60, c.start%60, 0, 0, clock.Loc)
	if start.After(t) {
		return start
	}
	return start.AddDate(0, 0, 1)
}

// nextAutoAttempt is the first moment the schedule would replace the key:
// threshold minus lead after it was last replaced (now when nobody knows),
// no sooner than autoAttemptGap after the last automatic attempt, inside the
// quiet window.
func nextAutoAttempt(now time.Time, lastReplaced, lastAuto *time.Time, c autoRotateConfig) time.Time {
	earliest := now
	if lastReplaced != nil {
		if due := lastReplaced.Add(c.Threshold - c.Lead); due.After(earliest) {
			earliest = due
		}
	}
	if lastAuto != nil {
		if gap := lastAuto.Add(autoAttemptGap); gap.After(earliest) {
			earliest = gap
		}
	}
	return c.windowFrom(earliest)
}

// probeRecentlyPassed is whether L1's last run passed within probeFreshFor.
func probeRecentlyPassed(now time.Time) bool {
	r := lastProbeL1.Load()
	return r != nil && r.Pass && now.Sub(r.At) < probeFreshFor
}

// observedLastReplaced is when the claims key was last replaced, as observed:
// Syndra's own record, or the keyring noticing Zitadel's key change (a
// manual rotate.sh) when that is later; the .env timestamp only when neither
// has seen one (the bootstrap). A future .env date is no date.
func observedLastReplaced(st db.ActionKeyRotationState) (*time.Time, string) {
	rec := st.LastReplacedAt
	var kr *time.Time
	if cfg := claimPlane.Config(); cfg != nil && cfg.Keys.ChangedAt > 0 {
		t := time.Unix(cfg.Keys.ChangedAt, 0)
		kr = &t
	}
	switch {
	case rec != nil && (kr == nil || !kr.After(rec.Add(time.Minute))):
		return rec, "syndra"
	case kr != nil:
		return kr, "keyring"
	}
	raw := os.Getenv(envActionKeyRotatedAt)
	if raw == "" {
		return nil, ""
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil || t.After(nowFn()) {
		return nil, ""
	}
	return &t, "env"
}

// RunActionKeyAutoRotate is the schedule's look, every ActionKeyAutoEvery.
func RunActionKeyAutoRotate(ctx context.Context) error {
	c := autoConfig()
	if c.OffReason == "no_zitadel" || c.OffReason == "stand_in" {
		return nil
	}
	st, err := dbActionKeyRotationNow(ctx, claimcheck.ClaimTarget)
	if err != nil {
		return err
	}
	now := nowFn()
	confirmUnverified(ctx, st, now)
	if !c.On {
		return nil
	}
	last, _ := observedLastReplaced(st)
	if nextAutoAttempt(now, last, st.LastAutoAt, c).After(now) {
		return nil
	}
	// The same test the status reports as auto.waiting_for.
	if w := scheduleWaitingFor(now); w != "" {
		log.Printf("[ACTION] the key is due, but the test sign-in has not passed in the last %s (%s); waiting", probeFreshFor, w)
		return nil
	}
	if _, err := rotateActionKeys(ctx, "auto", "system", false); err != nil && !errors.Is(err, errRotationInFlight) && !errors.Is(err, errRecentlyReplaced) {
		return fmt.Errorf("automatic key replacement: %w", err)
	}
	return nil
}

// confirmUnverified settles an unverified replacement once the regular
// probes have passed since it, with the keyring read from Zitadel since:
// observation, never another replacement.
func confirmUnverified(ctx context.Context, st db.ActionKeyRotationState, now time.Time) {
	l := st.Last
	if l == nil || l.Outcome != "unverified" || l.FinishedAt == nil {
		return
	}
	after := *l.FinishedAt
	l1 := lastProbeL1.Load()
	if l1 == nil || !l1.Pass || !l1.At.After(after) {
		return
	}
	if ProbeL2Enabled() {
		if l2 := lastProbeL2.Load(); l2 == nil || !l2.Pass || !l2.At.After(after) {
			return
		}
	}
	if ks := keyringSnapshot(); ks.Error != nil || ks.ReadAt == nil || !ks.ReadAt.After(after) {
		return
	}
	if err := dbFinishActionKeyRotation(ctx, l.ID, "verified", l1, ""); err != nil {
		log.Printf("[ACTION] could not record attempt %d as verified: %v", l.ID, err)
		return
	}
	recordRotationFinding(ctx, "verified", "", now)
	_ = dbInsertAuditLogDetails(ctx, "system", "", "action_key.verified_later", strconv.FormatInt(l.ID, 10), map[string]any{"probe": "passed after"})
}

// ---------------------------------------------------------------------------
// POST /api/v1/zitadel/action-key/verify (admin): the by-hand guide's last
// step. Once the guide has seen a new key, Syndra reads Zitadel's keys again
// and runs the test sign-in. It changes no key and writes no Zitadel state;
// the probe records itself as its schedule would. Body {}.
// ---------------------------------------------------------------------------

type verifyResult struct {
	Probe         probeResult `json:"probe"`
	LastRotatedAt *time.Time  `json:"last_rotated_at,omitempty"`
	NextDueAt     *time.Time  `json:"next_due_at,omitempty"`
}

// HandleVerifyActionKey runs the test sign-in against the keys Syndra holds.
func HandleVerifyActionKey(w http.ResponseWriter, r *http.Request) {
	var req struct{}
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonErrorResponse(w, http.StatusBadRequest, "INVALID_REQUEST", "The request body must be {}.")
		return
	}
	if zitadelActions() == nil {
		jsonErrorResponse(w, http.StatusServiceUnavailable, "NOT_CONFIGURED", "Syndra has no connection to Zitadel, so it can't read the key.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), time.Minute)
	defer cancel()
	res := verifyResult{}
	if err := followKeyring(ctx); err != nil {
		res.Probe = probeResult{At: time.Now(), Class: "keyring", Detail: "Syndra couldn't read the key from Zitadel: " + rotationReason(err)}
	} else {
		res.Probe = rotationProbe(ctx)
	}
	st, err := dbActionKeyRotationNow(ctx, claimcheck.ClaimTarget)
	if err != nil {
		log.Printf("[ACTION] verify: could not read the record: %v", err)
	}
	if t, _ := observedLastReplaced(st); t != nil {
		due := t.Add(time.Duration(rotationThresholdDays()) * 24 * time.Hour)
		res.LastRotatedAt, res.NextDueAt = t, &due
	}
	jsonResponse(w, http.StatusOK, res)
}

// ---------------------------------------------------------------------------
// POST /api/v1/zitadel/action-key/rotate (admin): "Replace now".
// ---------------------------------------------------------------------------

// HandleRotateActionKey replaces the keys now. Body {"force": bool}: force
// replaces even when the key was replaced in the last day (the button sends
// it; its Modal is the confirmation).
func HandleRotateActionKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Force bool `json:"force"`
	}
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonErrorResponse(w, http.StatusBadRequest, "INVALID_REQUEST", "The request body must be {\"force\": true|false}.")
		return
	}
	// The replacement outlives a closed tab: a Zitadel write cut off halfway
	// is the one outcome nobody can read back.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 2*time.Minute)
	defer cancel()
	res, err := rotateActionKeys(ctx, "button", resolveActor(r, ""), req.Force)
	switch {
	case err == nil:
		jsonResponse(w, http.StatusOK, res)
	case errors.Is(err, errNoClient):
		jsonErrorResponse(w, http.StatusServiceUnavailable, "NOT_CONFIGURED", "Syndra has no connection to Zitadel, so it can't replace the key.")
	case errors.Is(err, errRotationInFlight):
		jsonErrorResponse(w, http.StatusConflict, "ROTATION_IN_FLIGHT", "The key is being replaced right now.")
	case errors.Is(err, errRecentlyReplaced):
		jsonErrorResponse(w, http.StatusConflict, "RECENTLY_REPLACED", "The key was replaced less than a day ago.")
	case errors.Is(err, errZitadelDown):
		jsonErrorResponse(w, http.StatusServiceUnavailable, "ZITADEL_UNAVAILABLE", "Zitadel didn't answer, so nothing was changed.")
	case errors.Is(err, errNoClaimTarget):
		jsonErrorResponse(w, http.StatusConflict, "NOT_REGISTERED", "Zitadel has no claims action to replace a key on. Register it first.")
	default:
		log.Printf("[ACTION] replace now: %v", err)
		jsonErrorResponse(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Couldn't replace the key.")
	}
}
