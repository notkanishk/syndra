package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"syndra/internal/cache"
	"syndra/internal/claimcheck"
	"syndra/internal/claimserve"
	"syndra/internal/claimserve/claimservetest"
	"syndra/internal/claimtel"
	"syndra/internal/db"
	"syndra/internal/models"
	"syndra/internal/zitadel"
)

// fakeActions is Zitadel's Actions API: a claims target with a key, or an error.
type fakeActions struct {
	mu    sync.Mutex
	key   string
	err   error
	reads int
}

func (f *fakeActions) set(key string, err error) { f.mu.Lock(); f.key, f.err = key, err; f.mu.Unlock() }

func (f *fakeActions) ListActionTargets(context.Context) ([]zitadel.ActionTarget, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reads++
	if f.err != nil {
		return nil, f.err
	}
	return []zitadel.ActionTarget{{ID: "t1", Name: claimcheck.ClaimTarget, SigningKey: f.key, Endpoint: "https://syndra.example.edu/api/action/inject",
		Timeout: "3s", RestCall: &zitadel.RestCall{}}}, nil
}
func (f *fakeActions) ListActionExecutions(context.Context) ([]zitadel.ActionExecution, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return nil, f.err
}
func (f *fakeActions) TokenFlowActions(context.Context) ([]zitadel.TokenAction, error) {
	return nil, nil
}

// findingsSink records what the detectors would write.
type findingsSink struct {
	mu  sync.Mutex
	ups []claimtel.FindingUpdate
}

func (s *findingsSink) kinds() map[string]int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := map[string]int64{}
	for _, u := range s.ups {
		out[u.Kind+" "+u.Subject] += u.Fails
	}
	return out
}

func stubClaimFindings(t *testing.T) *findingsSink {
	t.Helper()
	a, c, st, open := dbApplyClaimFindings, dbClaimFindings, dbSettleUnseenClaimFindings, claimFindingsOpen.Load()
	t.Cleanup(func() {
		dbApplyClaimFindings, dbClaimFindings, dbSettleUnseenClaimFindings = a, c, st
		claimFindingsOpen.Store(open)
	})
	s := &findingsSink{}
	dbApplyClaimFindings = func(_ context.Context, ups []claimtel.FindingUpdate, _ int) error {
		s.mu.Lock()
		s.ups = append(s.ups, ups...)
		s.mu.Unlock()
		return nil
	}
	dbClaimFindings = func(context.Context, time.Time) ([]db.ClaimFinding, error) { return nil, nil }
	dbSettleUnseenClaimFindings = func(context.Context, string, []string, []string) error { return nil }
	return s
}

// keyringPlane is a data plane over a fake Redis the keyring publishes into.
func keyringPlane(t *testing.T, reader *fakeActions) *claimservetest.Redis {
	t.Helper()
	r := builtinPlane(t)
	za, cr := zitadelActions, claimConfigRedis
	t.Cleanup(func() { zitadelActions, claimConfigRedis = za, cr })
	zitadelActions = func() zitadel.ActionsReader { return reader }
	claimConfigRedis = func() claimserve.Redis { return r }
	t.Setenv("ZITADEL_DOMAIN", "zitadel.example.edu")
	return r
}

func signedWith(key string) http.Header {
	b := body(pre, `[{"project_id":"p1"}]`, "")
	at := time.Now()
	return http.Header{"Zitadel-Signature": {fmt.Sprintf("t=%d,v1=%x", at.Unix(), computeZitadelActionSignature(at, []byte(b), key))}}
}

func injectSigned(t *testing.T, log *callLog, key string) claimtel.Call {
	t.Helper()
	postInject(t, body(pre, `[{"project_id":"p1"}]`, ""), signedWith(key))
	return log.last(t)
}

// A rotation: Zitadel swaps the key with no grace period. The first call
// signed with the new key fails and asks for a read; after the read, calls
// signed with it verify (slot current), calls Zitadel signed just before the
// swap still verify for 15 minutes (slot previous), and the env key still
// verifies as the fallback.
func TestKeyring_RotationFollowsWithoutAnOutage(t *testing.T) {
	stubClaimFindings(t)
	log := captureCalls(t)
	reader := &fakeActions{key: "key-one-0123456789abcdef0123456789"}
	keyringPlane(t, reader)
	t.Setenv(actionKeyEnv, "key-env-0123456789abcdef0123456789ab")

	if err := followKeyring(context.Background()); err != nil {
		t.Fatal(err)
	}
	if c := injectSigned(t, log, "key-one-0123456789abcdef0123456789"); c.Outcome == claimtel.SigMismatch || c.KeySlot != claimtel.KeySlotCurrent {
		t.Fatalf("after the first read: %+v", c)
	}
	if c := injectSigned(t, log, "key-env-0123456789abcdef0123456789ab"); c.Outcome == claimtel.SigMismatch || c.KeySlot != claimtel.KeySlotEnv {
		t.Fatalf("the env key is the fallback: %+v", c)
	}

	// Zitadel rotates.
	reader.set("key-two-0123456789abcdef0123456789", nil)
	select {
	case <-keyring.nudge:
	default:
	}
	if c := injectSigned(t, log, "key-two-0123456789abcdef0123456789"); c.Outcome != claimtel.SigMismatch {
		t.Fatalf("before the re-read the new key is unknown: %+v", c)
	}
	select {
	case <-keyring.nudge:
	default:
		t.Fatal("a mismatch did not ask the keyring to read again")
	}
	if err := followKeyring(context.Background()); err != nil {
		t.Fatal(err)
	}
	mismatches := 0
	for i := 0; i < 20; i++ {
		if c := injectSigned(t, log, "key-two-0123456789abcdef0123456789"); c.Outcome == claimtel.SigMismatch || c.KeySlot != claimtel.KeySlotCurrent {
			mismatches++
		}
	}
	if mismatches != 0 {
		t.Fatalf("%d of 20 calls failed after the re-read", mismatches)
	}
	if c := injectSigned(t, log, "key-one-0123456789abcdef0123456789"); c.KeySlot != claimtel.KeySlotPrevious {
		t.Fatalf("a call signed just before the swap: %+v", c)
	}
	if st := keyringSnapshot(); st.ChangedAt == nil || st.Error != nil {
		t.Errorf("status = %+v", st)
	}
	// The key is never logged or shown: the status carries no key.
	if b, _ := json.Marshal(keyringSnapshot()); strings.Contains(string(b), "key-two") {
		t.Errorf("keyring status leaked the key: %s", b)
	}
}

// Without the read permission the data plane keeps verifying with the env key
// alone, and the failed read is CANNOT_CHECK, never silent.
func TestKeyring_DeniedReadKeepsTheEnvKeyAndSaysSo(t *testing.T) {
	sink := stubClaimFindings(t)
	log := captureCalls(t)
	reader := &fakeActions{err: &zitadel.StatusError{Code: 403, Message: "No matching permissions found"}}
	keyringPlane(t, reader)
	t.Setenv(actionKeyEnv, "key-env-0123456789abcdef0123456789ab")

	if err := followKeyring(context.Background()); err == nil {
		t.Fatal("a denied read is an error")
	}
	if c := injectSigned(t, log, "key-env-0123456789abcdef0123456789ab"); c.Outcome == claimtel.SigMismatch || c.KeySlot != claimtel.KeySlotEnv {
		t.Fatalf("the env key alone must verify: %+v", c)
	}
	if sink.kinds()[claimtel.KindCannotCheck+" R1"] == 0 {
		t.Fatalf("findings = %v", sink.kinds())
	}
	if st := keyringSnapshot(); st.Error["status"] != 403 {
		t.Errorf("status = %+v", st)
	}

	// The reconciler says the same, for every check that reads Actions.
	stubConfigReads(t)
	if err := RunClaimConfigCheck(context.Background()); err != nil {
		t.Fatal(err)
	}
	k := sink.kinds()
	for _, c := range []string{"R1", "R2", "R3"} {
		if k[claimtel.KindCannotCheck+" "+c] == 0 {
			t.Errorf("no CANNOT_CHECK for %s: %v", c, k)
		}
	}
	if run := lastConfigRun.Load(); run == nil || len(run.Rows) == 0 {
		t.Error("the configuration part has nothing to show")
	}
}

func stubConfigReads(t *testing.T) {
	t.Helper()
	da, ts, rk, rh := directoryApplications, zitadelTokenSettings, dirRoleKeys, resolveHost
	t.Cleanup(func() { directoryApplications, zitadelTokenSettings, dirRoleKeys, resolveHost = da, ts, rk, rh })
	directoryApplications = func(context.Context) ([]models.ApplicationCatalog, error) {
		return []models.ApplicationCatalog{{ID: "a1", Name: "NetBird", ProjectID: "p1", ClientID: "c1"}}, nil
	}
	zitadelTokenSettings = func(context.Context, string, string) (*zitadel.TokenSettings, error) {
		return &zitadel.TokenSettings{AccessTokenType: "JWT"}, nil
	}
	dirRoleKeys = func(context.Context, string) ([]string, error) { return []string{"staff"}, nil }
	resolveHost = func(context.Context, string) error { return nil }
	t.Setenv("SYNDRA_EXTERNAL_URL", "https://syndra.example.edu")
}

// The reconciler's app read feeds the correlator: a JWT app's sign-in
// expects preaccesstoken.
func TestConfigCheck_FeedsTheCorrelatorTokenTypes(t *testing.T) {
	stubClaimFindings(t)
	keyringPlane(t, &fakeActions{key: "k"})
	stubConfigReads(t)
	orig := claimAppJWT.Load()
	t.Cleanup(func() { claimAppJWT.Store(orig) })
	_ = RunClaimConfigCheck(context.Background())
	if jwt, known := appSettingsForCorrelator("c1"); !jwt || !known {
		t.Errorf("c1 jwt=%v known=%v", jwt, known)
	}
	if _, known := appSettingsForCorrelator("unknown"); known {
		t.Error("an app never read is unknown")
	}
}

// A sign-in event from Zitadel reaches the correlator with its client and
// scopes. A minute's check with no call raises CLAIMS_NOT_CALLED; a sign-in
// from before the process started raises nothing.
func TestSignInEvent_ReachesTheCorrelator(t *testing.T) {
	sink := stubClaimFindings(t)
	freshRecorder(t)
	origC, origJWT := claimCorrelator, claimAppJWT.Load()
	t.Cleanup(func() { claimCorrelator = origC; claimAppJWT.Store(origJWT) })
	claimAppJWT.Store(nil)
	ru, da := dbRecordUsage, directoryApplications
	t.Cleanup(func() { dbRecordUsage, directoryApplications = ru, da })
	dbRecordUsage = func(context.Context, string, string, string, string, time.Time) error { return nil }
	directoryApplications = func(context.Context) ([]models.ApplicationCatalog, error) { return nil, nil }
	clockAt := time.Now().Add(-10 * time.Minute)
	claimCorrelator = claimtel.NewCorrelator(func() time.Time { return clockAt }) // started ten minutes ago
	clockAt = time.Now()

	ev := zitadelEventPayload{EventType: "oidc_session.added", AggregateID: "sess-1",
		EventPayload: json.RawMessage(`{"userID":"u1","clientID":"c1@proj","scope":["openid","profile"],"audience":["c1@proj","other"]}`)}
	p := translateEventName(ev)
	if p.SignInClientID != "c1@proj" || strings.Join(p.SignInScopes, " ") != "openid profile" {
		t.Fatalf("payload = %+v", p)
	}
	before := clockAt.Add(-11 * time.Minute)
	p.EventCreatedAt = &before
	recordSignIn(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", nil), p) // from before the start
	recent := clockAt.Add(-5 * time.Minute)
	p.EventCreatedAt = &recent
	recordSignIn(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", nil), p)

	if err := RunClaimCorrelation(context.Background()); err != nil {
		t.Fatal(err)
	}
	if n := sink.kinds()[claimtel.KindNotCalled+" c1@proj"]; n != 1 {
		t.Fatalf("findings = %v, want one CLAIMS_NOT_CALLED for the recent sign-in", sink.kinds())
	}
}

// /healthz/claims names open findings without ids, and says when no
// external alert is configured, as a note that changes no status.
func TestClaimsHealth_FindingsAndTheAlertNote(t *testing.T) {
	freshRecorder(t)
	stubHealthDeps(t, nil, &cache.WarmReport{At: time.Now().Add(-time.Hour), Refreshed: 1, Of: 1}, 0)
	stubClaimFindings(t)
	t.Setenv(deadmanEnv, "")

	if _, h, _ := getHealth(t); h.Status != "ok" || strings.Join(h.Notes, ",") != "NO_EXTERNAL_ALERT" {
		t.Fatalf("no alert configured is a note, not a failure: %+v", h)
	}
	open := []db.ClaimFinding{
		{Kind: claimtel.KindConfigDrift, Subject: "R2:preaccesstoken"},
		{Kind: claimtel.KindConfigDrift, Subject: "R2:preuserinfo"},
		{Kind: claimtel.KindNotServed, Subject: "client-secret", Sample: map[string]any{"outcome": "degraded_no_facts"}},
	}
	claimFindingsOpen.Store(&open)
	code, h, raw := getHealth(t)
	if code != 503 || h.Status != "failing" || strings.Join(h.Reasons, ",") != "CONFIG_DRIFT:R2,CLAIMS_NOT_SERVED" {
		t.Fatalf("HTTP %d %+v", code, h)
	}
	if strings.Contains(raw, "client-secret") || strings.Contains(raw, "preaccesstoken") {
		t.Fatalf("health leaked a subject: %s", raw)
	}
}

// The dead-man ping goes out only while claim delivery is ok, with the
// reasons in its body; failing means no ping, which is the alert.
func TestPingDeadman_OnlyWhileOK(t *testing.T) {
	freshRecorder(t)
	stubHealthDeps(t, nil, &cache.WarmReport{At: time.Now().Add(-time.Hour), Refreshed: 1, Of: 1}, 0)
	stubClaimFindings(t)
	t.Setenv(deadmanEnv, "https://hc.example.edu/ping/abc")
	var mu sync.Mutex
	var pings []string
	orig := deadmanPost
	t.Cleanup(func() { deadmanPost = orig })
	deadmanPost = func(_ context.Context, target string, body []byte) error {
		mu.Lock()
		pings = append(pings, string(body))
		mu.Unlock()
		return nil
	}

	claimFindingsOpen.Store(nil)
	_ = PingDeadman(context.Background())
	if len(pings) != 0 {
		t.Fatal("pinged before the findings were read this process")
	}
	none := []db.ClaimFinding{}
	claimFindingsOpen.Store(&none)
	if err := PingDeadman(context.Background()); err != nil || len(pings) != 1 || !strings.Contains(pings[0], `"status":"ok"`) {
		t.Fatalf("pings = %v, err %v", pings, err)
	}
	drift := []db.ClaimFinding{{Kind: claimtel.KindNotCalled, Subject: "c1"}}
	claimFindingsOpen.Store(&drift)
	_ = PingDeadman(context.Background())
	if len(pings) != 1 {
		t.Fatalf("pinged while failing: %v", pings)
	}

	claimFindingsOpen.Store(&none)
	deadmanPost = func(context.Context, string, []byte) error { return errors.New("dial tcp: no route") }
	if err := PingDeadman(context.Background()); err == nil || strings.Contains(err.Error(), "hc.example.edu") {
		t.Fatalf("err = %v: a failed ping is reported, without the URL", err)
	}
	if _, h, _ := getHealth(t); strings.Join(h.Notes, ",") != "EXTERNAL_ALERT_UNREACHABLE" || h.Status != "ok" {
		t.Errorf("health = %+v", h)
	}
}

// Home's line reads the same answer the dead-man acts on.
func TestClaimsStatus(t *testing.T) {
	freshRecorder(t)
	stubHealthDeps(t, nil, &cache.WarmReport{At: time.Now().Add(-time.Hour), Refreshed: 1, Of: 1}, 0)
	stubClaimFindings(t)
	drift := []db.ClaimFinding{{Kind: claimtel.KindConfigDrift, Subject: "R1:endpoint"}}
	claimFindingsOpen.Store(&drift)
	rr := httptest.NewRecorder()
	handleClaimsStatus(rr, httptest.NewRequest(http.MethodGet, "/api/v1/claims/status", nil))
	var got struct {
		Status   string           `json:"status"`
		Problems int              `json:"problems"`
		SignIns  map[string]int64 `json:"sign_ins_24h"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &got)
	if rr.Code != 200 || got.Status != "failing" || got.Problems != 1 || len(got.SignIns) != len(claimtel.Verdicts) {
		t.Fatalf("HTTP %d %+v", rr.Code, got)
	}
}
