package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"syndra/internal/claimcheck"
	"syndra/internal/claimserve"
	"syndra/internal/claimtel"
	"syndra/internal/db"
	"syndra/internal/services/periodic"
)

// The detectors (claims-delivery-integrity S3, S4, S5): the sign-in
// correlator, the configuration reconciler, the signing keyring, and the
// dead-man's switch. Everything here reads; the only writes are Syndra's own
// findings rows and the keyring it publishes for the data plane.

// claimCorrelator matches sign-ins against calls. Built at process start:
// sign-ins from before then are unchecked ("checking since …").
var claimCorrelator = func() *claimtel.Correlator {
	c := claimtel.NewCorrelator(time.Now)
	// A no_grants call is only a failure if Zitadel, as Syndra last observed
	// it, gives the person a role in that app's project: Syndra's own facts
	// lack roles given in Zitadel directly, and a role Syndra has queued is
	// not one Zitadel could have listed.
	c.Holds = func(userID, clientID string) (bool, bool) {
		return observedHolds(context.Background(), userID, clientID)
	}
	return c
}()

// observedHolds is whether Zitadel, as last observed, gives the person a role
// in the project of the client's app. known is false when the client maps to
// no app, the read fails, or no complete current sweep can vouch for "none".
func observedHolds(ctx context.Context, userID, clientID string) (holds, known bool) {
	cfg := claimPlane.Config()
	if cfg == nil || cfg.ClientProject[clientID] == "" {
		return false, false
	}
	roles, absent, err := svcObservedRoles(ctx, userID, cfg.ClientProject[clientID])
	if err != nil {
		return false, false
	}
	return len(roles) > 0, len(roles) > 0 || absent
}

// ---------------------------------------------------------------------------
// Findings: applied by each detector, cached in memory for /healthz/claims
// (which is served on the data plane's door and never reads Postgres).
// ---------------------------------------------------------------------------

var claimFindingsOpen atomic.Pointer[[]db.ClaimFinding]

// applyClaimFindings folds observations into claim_findings and refreshes
// the open set /healthz/claims reads.
func applyClaimFindings(ctx context.Context, obs []claimtel.Observation, resolveAfter int) error {
	if err := dbApplyClaimFindings(ctx, claimtel.Fold(obs), resolveAfter); err != nil {
		return err
	}
	return refreshClaimFindings(ctx)
}

func refreshClaimFindings(ctx context.Context) error {
	all, err := dbClaimFindings(ctx, time.Now())
	if err != nil {
		return err
	}
	open := all[:0:0]
	for _, f := range all {
		if f.ResolvedAt == nil {
			open = append(open, f)
		}
	}
	claimFindingsOpen.Store(&open)
	return nil
}

func openClaimFindings() []db.ClaimFinding {
	if p := claimFindingsOpen.Load(); p != nil {
		return *p
	}
	return nil
}

// ---------------------------------------------------------------------------
// S3 · the correlator, once a minute.
// ---------------------------------------------------------------------------

// claimAppJWT is each OIDC client's access-token type, from the reconciler's
// last read of the apps' settings: what a sign-in is expected to call.
var claimAppJWT atomic.Pointer[map[string]bool]

func appSettingsForCorrelator(clientID string) (jwt, known bool) {
	m := claimAppJWT.Load()
	if m == nil {
		return false, false
	}
	jwt, known = (*m)[clientID]
	return jwt, known
}

// RunClaimCorrelation checks every sign-in old enough to check, and the
// inverse (calls with no sign-in events: the detector is blind).
func RunClaimCorrelation(ctx context.Context) error {
	checked := claimCorrelator.Check(claimCalls{}, appSettingsForCorrelator)
	obs := claimtel.Observe(checked)
	now := time.Now()
	if lastCalled := claimLastCalled(ctx); claimCorrelator.EventsMissing(lastCalled) {
		obs = append(obs, claimtel.Observation{Kind: claimtel.KindEventsMissing, Subject: claimtel.None, Fail: true, At: now,
			Sample: map[string]any{"last_sign_in_at": nullTime(claimCorrelator.LastSignIn()), "last_call_at": nullTime(lastCalled)}})
	} else if len(checked) > 0 {
		obs = append(obs, claimtel.Observation{Kind: claimtel.KindEventsMissing, Subject: claimtel.None, At: now})
	}
	if len(obs) == 0 {
		return refreshClaimFindings(ctx)
	}
	return applyClaimFindings(ctx, obs, claimtel.ResolveAfterSignIns)
}

func nullTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

// ---------------------------------------------------------------------------
// S4 · the configuration reconciler, every 10 minutes and at start.
// ---------------------------------------------------------------------------

type configRun struct {
	At   time.Time        `json:"checked_at"`
	Took float64          `json:"took_ms"`
	Rows []claimcheck.Row `json:"rows"`
}

var lastConfigRun atomic.Pointer[configRun]

// RunClaimConfigCheck reads Zitadel's targets, executions, v1 flow and every
// app's token settings, and Syndra's snapshot and env, then raises
// CONFIG_DRIFT per check that differs and CANNOT_CHECK per read that failed.
func RunClaimConfigCheck(ctx context.Context) error {
	start := time.Now()
	in := gatherConfigInputs(ctx)
	rep := claimcheck.Evaluate(in)
	jwt := rep.JWT
	claimAppJWT.Store(&jwt)
	lastConfigRun.Store(&configRun{At: start, Took: float64(time.Since(start).Microseconds()) / 1000, Rows: rep.Rows})

	drift, cannot := rep.Observations(start)
	err := applyClaimFindings(ctx, append(drift, cannot...), claimtel.ResolveAfterReads)
	return errors.Join(err, dbSettleUnseenClaimFindings(ctx, claimtel.KindConfigDrift, rep.Ran(), rep.Seen()), refreshClaimFindings(ctx))
}

func gatherConfigInputs(ctx context.Context) claimcheck.Inputs {
	// The L1 canary's project is no app Zitadel knows: left out, or R6 and R7
	// would report it.
	in := claimcheck.Inputs{ExternalURL: os.Getenv("SYNDRA_EXTERNAL_URL"), Snapshot: claimserve.WithoutCanary(claimPlane.Config())}
	reader := zitadelActions()
	if reader == nil {
		in.TargetsErr, in.ExecutionsErr, in.V1Err = errNoClient, errNoClient, errNoClient
	} else {
		// Follow the key first, so a rotation since the last keyring read is
		// not reported as a key the data plane lacks.
		_ = followKeyring(ctx)
		in.Targets, in.TargetsErr = reader.ListActionTargets(ctx)
		in.Executions, in.ExecutionsErr = reader.ListActionExecutions(ctx)
		in.V1, in.V1Err = reader.TokenFlowActions(ctx)
	}
	for _, k := range actionSigningKeys() {
		in.HeldKeys = append(in.HeldKeys, claimcheck.KeyCheck(k.Key))
	}
	in.EnvKey = claimcheck.KeyCheck(os.Getenv(actionKeyEnv))
	if u, err := url.Parse(in.ExternalURL); err != nil || u.Hostname() == "" {
		in.ResolveErr = fmt.Errorf("SYNDRA_EXTERNAL_URL has no host")
	} else {
		rctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		in.ResolveErr = resolveHost(rctx, u.Hostname())
		cancel()
	}

	apps, err := directoryApplications(ctx)
	in.AppsErr = err
	for _, a := range apps {
		if a.ClientID == "" {
			continue
		}
		app := claimcheck.App{ID: a.ID, Name: a.Name, ClientID: a.ClientID, ProjectID: a.ProjectID}
		app.Settings, app.SettingsErr = zitadelTokenSettings(ctx, a.ProjectID, a.ID)
		app.OIDC = app.Settings != nil && app.Settings.AccessTokenType != ""
		if app.SettingsErr != nil {
			app.OIDC = true // unknown: R5 says it could not check
		}
		in.Apps = append(in.Apps, app)
	}

	if in.Snapshot != nil {
		in.ProjectRoles = map[string][]string{}
		for pid := range in.Snapshot.Projects {
			keys, err := dirRoleKeys(ctx, pid)
			if err != nil {
				in.RolesErr = err
				break
			}
			in.ProjectRoles[pid] = keys
		}
	}
	return in
}

// ---------------------------------------------------------------------------
// S5 · the signing keyring. Zitadel signs with one key and swaps it with no
// grace period, so Syndra reads the claims target's key every minute, and at
// once (at most every 10s) after a call matches no key it holds. The data
// plane verifies with {current, previous for 15 min}, then the env key.
// ---------------------------------------------------------------------------

const (
	keyringEvery    = time.Minute
	keyringMinGap   = 10 * time.Second
	keyringReadTime = 5 * time.Second
)

type keyringStatus struct {
	ReadAt    *time.Time     `json:"read_at"`
	ChangedAt *time.Time     `json:"changed_at"`
	Error     map[string]any `json:"error,omitempty"`
	Reads     int64          `json:"reads"`
}

var keyring = struct {
	nudge  chan struct{}
	mu     sync.Mutex
	status keyringStatus
	last   time.Time
}{nudge: make(chan struct{}, 1)}

// nudgeKeyring asks for a read now. Never blocks: a nudge already waiting is
// the same read.
func nudgeKeyring() {
	select {
	case keyring.nudge <- struct{}{}:
	default:
	}
}

// RunKeyring follows Zitadel's key until ctx ends.
func RunKeyring(ctx context.Context) {
	tick := time.NewTicker(keyringEvery)
	defer tick.Stop()
	for {
		readKeyringOnce(ctx)
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		case <-keyring.nudge:
			// Single flight, at most once per keyringMinGap: a burst of
			// mismatches is one read.
			if wait := keyringMinGap - time.Since(keyring.last); wait > 0 {
				select {
				case <-ctx.Done():
					return
				case <-time.After(wait):
				}
			}
		}
	}
}

func readKeyringOnce(ctx context.Context) {
	defer periodic.Recover("CLAIM-KEYRING")
	keyring.last = time.Now()
	if err := followKeyring(ctx); err != nil {
		log.Printf("[DATA PLANE] keyring: could not read the claims target's key from Zitadel; the env key and the last keyring keep verifying: %v", err)
	}
}

// followKeyring reads the key once and publishes it when it moved. A failed
// read leaves the keyring as it was (the env key still verifies) and raises
// CANNOT_CHECK on R1, the check that reads the same target.
func followKeyring(ctx context.Context) error {
	now := time.Now()
	rctx, cancel := context.WithTimeout(ctx, keyringReadTime)
	defer cancel()
	key, err := readClaimTargetKey(rctx)
	keyring.mu.Lock()
	keyring.status.Reads++
	if err != nil {
		keyring.status.Error = claimcheck.Reason(err)
	} else {
		keyring.status.ReadAt, keyring.status.Error = &now, nil
	}
	keyring.mu.Unlock()
	if err != nil {
		_ = applyClaimFindings(ctx, []claimtel.Observation{{Kind: claimtel.KindCannotCheck, Subject: "R1", Fail: true, At: now,
			Sample: map[string]any{"read": "keyring", "reason": claimcheck.Reason(err)}}}, claimtel.ResolveAfterReads)
		return err
	}
	var cur claimserve.Keyring
	if cfg := claimPlane.Config(); cfg != nil {
		cur = cfg.Keys
	}
	next := cur.Next(key, now)
	if next == cur {
		return nil
	}
	rdb := claimConfigRedis()
	if rdb == nil {
		return errors.New("redis is not connected")
	}
	if err := claimserve.PublishKeyring(ctx, rdb, next); err != nil {
		return err
	}
	_ = claimPlane.Poll(ctx) // this process's data plane follows at once
	keyring.mu.Lock()
	keyring.status.ChangedAt = &now
	keyring.mu.Unlock()
	log.Printf("[DATA PLANE] keyring: Zitadel's signing key for the claims target changed; the data plane verifies with it now (the previous one for %s)", claimserve.PreviousKeyKeep)
	return nil
}

func readClaimTargetKey(ctx context.Context) (string, error) {
	reader := zitadelActions()
	if reader == nil {
		return "", errNoClient
	}
	targets, err := reader.ListActionTargets(ctx)
	if err != nil {
		return "", err
	}
	for _, t := range targets {
		if t.Name == claimcheck.ClaimTarget {
			if t.SigningKey == "" {
				return "", errors.New("zitadel returned the claims target without its signing key")
			}
			return t.SigningKey, nil
		}
	}
	return "", fmt.Errorf("zitadel has no target named %s", claimcheck.ClaimTarget)
}

func keyringSnapshot() keyringStatus {
	keyring.mu.Lock()
	defer keyring.mu.Unlock()
	return keyring.status
}

// ---------------------------------------------------------------------------
// The dead-man's switch (S3). Syndra pings SYNDRA_DEADMAN_URL every 5 minutes
// only while /healthz/claims is ok; the service alerts the owner when a ping
// does not come. That covers what nothing inside the LXC can report: the LXC
// down, the backend crash-looping, Syndra's own alerting broken.
// ---------------------------------------------------------------------------

const (
	deadmanEnv   = "SYNDRA_DEADMAN_URL"
	DeadmanEvery = 5 * time.Minute
)

// deadmanState is the last ping: what /healthz/claims says about the alert.
var deadmanState struct {
	mu     sync.Mutex
	failed bool
}

// DeadmanConfigured is whether an external alert is set up.
func DeadmanConfigured() bool { return os.Getenv(deadmanEnv) != "" }

// PingDeadman pings when claim delivery is ok, with the reasons (none, or the
// notes) in the body. Not ok: no ping, which is the alert.
func PingDeadman(ctx context.Context) error {
	target := os.Getenv(deadmanEnv)
	if target == "" || claimFindingsOpen.Load() == nil {
		return nil // off, or the findings have not been read yet this process
	}
	h, _ := claimsHealthNow(ctx)
	if h.Status != "ok" {
		log.Printf("[DATA PLANE] dead-man: claim delivery is %s (%v): not pinging, so the external alert fires", h.Status, h.Reasons)
		return nil
	}
	body, _ := json.Marshal(map[string]any{"status": h.Status, "reasons": h.Reasons, "notes": h.Notes})
	pctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	err := deadmanPost(pctx, target, body)
	deadmanState.mu.Lock()
	deadmanState.failed = err != nil
	deadmanState.mu.Unlock()
	if err != nil {
		// Never log the URL: it is the check's secret.
		return fmt.Errorf("dead-man ping failed: %w", err)
	}
	return nil
}

func postDeadman(ctx context.Context, target string, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		return errors.New("SYNDRA_DEADMAN_URL is not a URL")
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		var ue *url.Error
		if errors.As(err, &ue) {
			return ue.Err // without the URL
		}
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}

func deadmanNotes() []string {
	if !DeadmanConfigured() {
		return []string{"NO_EXTERNAL_ALERT"}
	}
	deadmanState.mu.Lock()
	defer deadmanState.mu.Unlock()
	if deadmanState.failed {
		return []string{"EXTERNAL_ALERT_UNREACHABLE"}
	}
	return []string{}
}
