package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"syndra/internal/claimcheck"
	"syndra/internal/claimserve"
	"syndra/internal/claimserve/claimservetest"
	"syndra/internal/claimtel"
	"syndra/internal/clock"
	"syndra/internal/db"
	"syndra/internal/zitadel"
)

// ---------------------------------------------------------------------------
// Fakes: Zitadel's two targets and Syndra's record, sharing one event log so
// a test can say what happened in which order.
// ---------------------------------------------------------------------------

const (
	oldClaimsKey = "claims-old-0123456789abcdef0123456789"
	oldEventsKey = "events-old-0123456789abcdef0123456789"
)

type events struct {
	mu  sync.Mutex
	all []string
}

func (e *events) add(s string) { e.mu.Lock(); e.all = append(e.all, s); e.mu.Unlock() }
func (e *events) list() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return slices.Clone(e.all)
}
func (e *events) count(prefix string) int {
	n := 0
	for _, s := range e.list() {
		if strings.HasPrefix(s, prefix) {
			n++
		}
	}
	return n
}

// fakeKeys is Zitadel holding Syndra's two targets.
type fakeKeys struct {
	mu        sync.Mutex
	ev        *events
	keys      map[string]string
	listErr   error
	rotateErr map[string]error
	gate      chan struct{} // when set, a rotation waits on it
	n         int
}

func (f *fakeKeys) ListActionTargets(context.Context) ([]zitadel.ActionTarget, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.listErr != nil {
		return nil, f.listErr
	}
	var out []zitadel.ActionTarget
	for name, k := range f.keys {
		out = append(out, zitadel.ActionTarget{ID: "id-" + name, Name: name, SigningKey: k})
	}
	return out, nil
}
func (f *fakeKeys) ListActionExecutions(context.Context) ([]zitadel.ActionExecution, error) {
	return nil, nil
}
func (f *fakeKeys) TokenFlowActions(context.Context) ([]zitadel.TokenAction, error) { return nil, nil }

func (f *fakeKeys) RotateTargetSigningKey(_ context.Context, id string) (string, error) {
	name := strings.TrimPrefix(id, "id-")
	f.ev.add("zitadel:rotate:" + name)
	if f.gate != nil {
		<-f.gate
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.rotateErr[name]; err != nil {
		return "", err
	}
	f.n++
	k := fmt.Sprintf("%s-new%d-0123456789abcdef0123456789", name, f.n)
	f.keys[name] = k
	return k, nil
}

func (f *fakeKeys) key(name string) string { f.mu.Lock(); defer f.mu.Unlock(); return f.keys[name] }

// fakeRecord is action_key_rotations and its advisory lock.
type fakeRecord struct {
	mu       sync.Mutex
	ev       *events
	rows     []db.ActionKeyRotation
	held     bool
	startErr error
}

func (r *fakeRecord) lock(context.Context) (func(), bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.held {
		return nil, false, nil
	}
	r.held = true
	return func() { r.mu.Lock(); r.held = false; r.mu.Unlock() }, true, nil
}

func (r *fakeRecord) state(_ context.Context, claimTarget string) (db.ActionKeyRotationState, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var st db.ActionKeyRotationState
	for i := range r.rows {
		row := r.rows[i]
		st.Last = &row
		if slices.Contains(row.Replaced, claimTarget) {
			t := row.StartedAt
			if row.FinishedAt != nil {
				t = *row.FinishedAt
			}
			st.LastReplacedAt = &t
		}
		if row.Trigger == "auto" {
			t := row.StartedAt
			st.LastAutoAt = &t
		}
	}
	return st, nil
}

func (r *fakeRecord) install(t *testing.T) {
	t.Helper()
	l, ab, s, m, f, n, a := dbTryActionKeyRotationLock, dbAbandonActionKeyRotations, dbStartActionKeyRotation,
		dbMarkActionKeyReplaced, dbFinishActionKeyRotation, dbActionKeyRotationNow, dbInsertAuditLogDetails
	t.Cleanup(func() {
		dbTryActionKeyRotationLock, dbAbandonActionKeyRotations, dbStartActionKeyRotation = l, ab, s
		dbMarkActionKeyReplaced, dbFinishActionKeyRotation, dbActionKeyRotationNow, dbInsertAuditLogDetails = m, f, n, a
	})
	dbTryActionKeyRotationLock = r.lock
	dbAbandonActionKeyRotations = func(context.Context) error {
		r.mu.Lock()
		defer r.mu.Unlock()
		for i := range r.rows {
			if r.rows[i].Outcome == "in_flight" {
				now := time.Now()
				r.rows[i].Outcome, r.rows[i].FinishedAt = "abandoned", &now
			}
		}
		return nil
	}
	dbStartActionKeyRotation = func(_ context.Context, trigger, actor string, targets []string) (int64, error) {
		r.ev.add("record:start:" + trigger)
		if r.startErr != nil {
			return 0, r.startErr
		}
		r.mu.Lock()
		defer r.mu.Unlock()
		r.rows = append(r.rows, db.ActionKeyRotation{ID: int64(len(r.rows) + 1), Trigger: trigger, Actor: actor, Targets: targets,
			Replaced: []string{}, StartedAt: time.Now(), Outcome: "in_flight"})
		return int64(len(r.rows)), nil
	}
	dbMarkActionKeyReplaced = func(_ context.Context, id int64, target string) error {
		r.ev.add("record:replaced:" + target)
		r.mu.Lock()
		defer r.mu.Unlock()
		r.rows[id-1].Replaced = append(r.rows[id-1].Replaced, target)
		return nil
	}
	dbFinishActionKeyRotation = func(_ context.Context, id int64, outcome string, _ any, errText string) error {
		r.ev.add("record:finish:" + outcome)
		r.mu.Lock()
		defer r.mu.Unlock()
		now := time.Now()
		r.rows[id-1].Outcome, r.rows[id-1].FinishedAt, r.rows[id-1].Error = outcome, &now, errText
		return nil
	}
	dbActionKeyRotationNow = r.state
	dbInsertAuditLogDetails = func(_ context.Context, _, _, action, _ string, _ any) error {
		r.ev.add("audit:" + action)
		return nil
	}
}

type rotationRig struct {
	ev     *events
	zit    *fakeKeys
	rec    *fakeRecord
	rdb    *claimservetest.Redis
	sink   *findingsSink
	probes int
	probe  probeResult
}

// newRotationRig is a backend over fake Zitadel, a fake record and a fake
// Redis, with the keyring already following the old keys.
func newRotationRig(t *testing.T) *rotationRig {
	t.Helper()
	ev := &events{}
	g := &rotationRig{ev: ev, rec: &fakeRecord{ev: ev}, probe: probeResult{Pass: true},
		zit: &fakeKeys{ev: ev, keys: map[string]string{claimcheck.ClaimTarget: oldClaimsKey, claimcheck.EventTarget: oldEventsKey}}}
	g.sink = stubClaimFindings(t)
	g.rdb = builtinPlane(t)
	g.rec.install(t)
	za, zr, cr, rp, ek := zitadelActions, zitadelKeyRotator, claimConfigRedis, rotationProbe, eventKeyring.Load()
	t.Cleanup(func() {
		zitadelActions, zitadelKeyRotator, claimConfigRedis, rotationProbe = za, zr, cr, rp
		eventKeyring.Store(ek)
	})
	eventKeyring.Store(nil)
	zitadelActions = func() zitadel.ActionsReader { return g.zit }
	zitadelKeyRotator = func() zitadel.ActionKeyRotator { return g.zit }
	claimConfigRedis = func() claimserve.Redis { return g.rdb }
	rotationProbe = func(context.Context) probeResult {
		g.ev.add("probe")
		g.probes++
		p := g.probe
		p.At = time.Now()
		return p
	}
	t.Setenv("ZITADEL_DOMAIN", "zitadel.example.org")
	t.Setenv(actionKeyEnv, oldClaimsKey)
	t.Setenv(eventKeyEnv, oldEventsKey)
	if err := followKeyring(context.Background()); err != nil {
		t.Fatal(err)
	}
	return g
}

// eventSigned posts an event signed with key through the listener's check.
func eventSigned(t *testing.T, key string) int {
	t.Helper()
	b := []byte(`{"event_type":"user.human.added"}`)
	at := time.Now()
	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/zitadel", strings.NewReader(string(b)))
	req.Header.Set(claimserve.SignatureHeader, claimserve.SignatureHeaderValue(at, b, key))
	rr := httptest.NewRecorder()
	withZitadelActionSignature(eventKeyEnv, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })(rr, req)
	return rr.Code
}

// ---------------------------------------------------------------------------

// The trace is written before Zitadel is asked to change anything, and
// without a trace nothing is asked.
func TestRotate_TraceBeforeTheCall(t *testing.T) {
	g := newRotationRig(t)
	res, err := rotateActionKeys(context.Background(), "button", "admin-1", false)
	if err != nil || res.Outcome != "verified" {
		t.Fatalf("got %+v, %v", res, err)
	}
	ev := g.ev.list()
	start := slices.Index(ev, "record:start:button")
	first := slices.IndexFunc(ev, func(s string) bool { return strings.HasPrefix(s, "zitadel:rotate:") })
	if start < 0 || first < 0 || start > first {
		t.Fatalf("the trace must come before the first Zitadel write: %v", ev)
	}
	want := []string{"record:start:button", "zitadel:rotate:" + claimcheck.ClaimTarget, "record:replaced:" + claimcheck.ClaimTarget,
		"zitadel:rotate:" + claimcheck.EventTarget, "record:replaced:" + claimcheck.EventTarget, "probe", "record:finish:verified", "audit:action_key.replaced"}
	if !slices.Equal(ev, want) {
		t.Errorf("events\n got %v\nwant %v", ev, want)
	}
	if res.NextDueAt == nil || res.NextDueAt.Sub(*res.ReplacedAt) != 90*24*time.Hour {
		t.Errorf("next due %v after %v", res.NextDueAt, res.ReplacedAt)
	}

	// No trace, no write.
	g2 := newRotationRig(t)
	g2.rec.startErr = errors.New("db down")
	if _, err := rotateActionKeys(context.Background(), "button", "admin-1", true); err == nil {
		t.Fatal("a failed trace must fail the replacement")
	}
	if n := g2.ev.count("zitadel:rotate:"); n != 0 {
		t.Fatalf("Zitadel was asked %d times with no trace", n)
	}
}

// Both processes, the backend's door and the claims container's, verify with
// the new key on the very first call after the replacement, with no restart
// and no wait for a poll; the old key keeps verifying through the overlap;
// the event listener follows too.
func TestRotate_NoUnverifiableWindowInEitherProcess(t *testing.T) {
	g := newRotationRig(t)
	log := captureCalls(t)
	// The claims container: its own plane over the same Redis, loaded before.
	proc := claimserve.NewProcess(g.rdb, oldClaimsKey, false)
	if err := proc.Plane.Poll(context.Background()); err != nil {
		t.Fatal(err)
	}
	container := proc.Router()
	callContainer := func(key string) int {
		req := httptest.NewRequest(http.MethodPost, "/api/action/inject", strings.NewReader(body(pre, `[{"project_id":"p1"}]`, "")))
		for k, v := range signedWith(key) {
			req.Header[k] = v
		}
		rr := httptest.NewRecorder()
		container.ServeHTTP(rr, req)
		return rr.Code
	}

	if _, err := rotateActionKeys(context.Background(), "auto", "system", false); err != nil {
		t.Fatal(err)
	}
	newClaims, newEvents := g.zit.key(claimcheck.ClaimTarget), g.zit.key(claimcheck.EventTarget)
	if newClaims == oldClaimsKey || newEvents == oldEventsKey {
		t.Fatal("the fake did not rotate")
	}
	if c := injectSigned(t, log, newClaims); c.Outcome == claimtel.SigMismatch || c.KeySlot != claimtel.KeySlotCurrent {
		t.Fatalf("backend, new key: %+v", c)
	}
	if code := callContainer(newClaims); code == http.StatusUnauthorized {
		t.Fatal("claims container refused the new key on its first call")
	}
	if c := injectSigned(t, log, oldClaimsKey); c.Outcome == claimtel.SigMismatch || c.KeySlot != claimtel.KeySlotPrevious {
		t.Fatalf("backend, old key during the overlap: %+v", c)
	}
	if code := callContainer(oldClaimsKey); code == http.StatusUnauthorized {
		t.Fatal("claims container refused the old key during the overlap")
	}
	if code := eventSigned(t, newEvents); code != http.StatusNoContent {
		t.Fatalf("event listener, new key: HTTP %d", code)
	}
	if code := eventSigned(t, oldEventsKey); code != http.StatusNoContent {
		t.Fatalf("event listener, old key during the overlap: HTTP %d", code)
	}
	if code := eventSigned(t, "forged-0123456789abcdef0123456789abcd"); code != http.StatusUnauthorized {
		t.Fatalf("a forged event: HTTP %d", code)
	}
	// The keys persist in Redis: a restart reads them back.
	if k, err := claimserve.ReadKeyring(context.Background(), g.rdb, claimserve.EventsKeyringKey); err != nil || k.Current != newEvents || k.Previous != oldEventsKey {
		t.Fatalf("events:keyring = %+v, %v", k, err)
	}
	// A projection read that lags the write (Zitadel answering the old key)
	// never swaps the keyring back.
	g.zit.mu.Lock()
	g.zit.keys[claimcheck.ClaimTarget] = oldClaimsKey
	g.zit.mu.Unlock()
	_ = followKeyring(context.Background())
	if c := injectSigned(t, log, newClaims); c.KeySlot != claimtel.KeySlotCurrent {
		t.Fatalf("a stale read demoted the new key: %+v", c)
	}
}

// Zitadel signs with the new key from the moment it commits it, a moment
// before its answer reaches Syndra. A call signed in that gap waits for the
// keyring instead of being refused.
func TestRotate_ACallInTheGapWaitsForTheKey(t *testing.T) {
	r := builtinPlane(t)
	proc := claimserve.NewProcess(r, oldClaimsKey, false)
	_ = proc.Plane.Poll(context.Background())
	if err := claimserve.MarkRotation(context.Background(), r, time.Now().Add(10*time.Second)); err != nil {
		t.Fatal(err)
	}
	newKey := "claims-gap-0123456789abcdef0123456789"
	go func() {
		time.Sleep(200 * time.Millisecond)
		_ = claimserve.PublishKeyring(context.Background(), r, claimserve.Keyring{}.Next(oldClaimsKey, time.Now()).Next(newKey, time.Now()))
	}()
	req := httptest.NewRequest(http.MethodPost, "/api/action/inject", strings.NewReader(body(pre, `[{"project_id":"p1"}]`, "")))
	for k, v := range signedWith(newKey) {
		req.Header[k] = v
	}
	rr := httptest.NewRecorder()
	proc.Router().ServeHTTP(rr, req)
	if rr.Code == http.StatusUnauthorized {
		t.Fatal("a call signed in the gap was refused")
	}

	// Without a replacement in flight a forged call is refused at once.
	_ = claimserve.MarkRotation(context.Background(), r, time.Time{})
	req = httptest.NewRequest(http.MethodPost, "/api/action/inject", strings.NewReader(body(pre, `[{"project_id":"p1"}]`, "")))
	for k, v := range signedWith("forged-0123456789abcdef0123456789abcd") {
		req.Header[k] = v
	}
	rr = httptest.NewRecorder()
	start := time.Now()
	proc.Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized || time.Since(start) > 500*time.Millisecond {
		t.Fatalf("forged call: HTTP %d after %s", rr.Code, time.Since(start))
	}
}

// Two clicks, or a click and the schedule, never replace twice.
func TestRotate_SingleFlight(t *testing.T) {
	g := newRotationRig(t)
	g.zit.gate = make(chan struct{})
	done := make(chan error, 1)
	go func() {
		_, err := rotateActionKeys(context.Background(), "button", "admin-1", true)
		done <- err
	}()
	deadline := time.Now().Add(2 * time.Second)
	for g.ev.count("zitadel:rotate:") == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if _, err := rotateActionKeys(context.Background(), "button", "admin-2", true); !errors.Is(err, errRotationInFlight) {
		t.Fatalf("second click: %v", err)
	}
	close(g.zit.gate)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if n := g.ev.count("zitadel:rotate:" + claimcheck.ClaimTarget); n != 1 {
		t.Fatalf("the claims key was replaced %d times", n)
	}

	// The HTTP answer for the same case.
	g.rec.held = true
	rr := postRotate(t, `{"force":true}`)
	if rr.Code != http.StatusConflict || !strings.Contains(rr.Body.String(), "ROTATION_IN_FLIGHT") {
		t.Fatalf("HTTP %d %s", rr.Code, rr.Body.String())
	}
}

// A replacement within a day is refused unless forced; the refusal touches
// nothing.
func TestRotate_RecentlyReplacedRefusedUnlessForced(t *testing.T) {
	g := newRotationRig(t)
	if _, err := rotateActionKeys(context.Background(), "button", "admin-1", false); err != nil {
		t.Fatal(err)
	}
	before := g.ev.count("zitadel:rotate:")
	if _, err := rotateActionKeys(context.Background(), "auto", "system", false); !errors.Is(err, errRecentlyReplaced) {
		t.Fatalf("got %v", err)
	}
	if g.ev.count("zitadel:rotate:") != before || g.ev.count("record:start:auto") != 0 {
		t.Fatal("a refused replacement touched Zitadel or the record")
	}
	if rr := postRotate(t, `{"force":false}`); rr.Code != http.StatusConflict || !strings.Contains(rr.Body.String(), "RECENTLY_REPLACED") {
		t.Fatalf("HTTP %d %s", rr.Code, rr.Body.String())
	}
	if rr := postRotate(t, `{"force":true}`); rr.Code != http.StatusOK {
		t.Fatalf("forced: HTTP %d %s", rr.Code, rr.Body.String())
	}
}

// Zitadel not answering refuses before any trace or write.
func TestRotate_RefusedWhileZitadelIsDown(t *testing.T) {
	g := newRotationRig(t)
	g.zit.listErr = &zitadel.StatusError{Code: 503, Message: "unavailable"}
	if _, err := rotateActionKeys(context.Background(), "auto", "system", true); !errors.Is(err, errZitadelDown) {
		t.Fatalf("got %v", err)
	}
	if g.ev.count("record:") != 0 || g.ev.count("zitadel:rotate:") != 0 {
		t.Fatalf("events %v", g.ev.list())
	}
	rr := postRotate(t, `{"force":true}`)
	if rr.Code != http.StatusServiceUnavailable || !strings.Contains(rr.Body.String(), "ZITADEL_UNAVAILABLE") {
		t.Fatalf("HTTP %d %s", rr.Code, rr.Body.String())
	}
}

// Zitadel refusing the write: failed, the old key still verifies, a finding
// is raised, and the outcome is recorded.
func TestRotate_RefusedWriteIsAnHonestFailure(t *testing.T) {
	g := newRotationRig(t)
	log := captureCalls(t)
	g.zit.rotateErr = map[string]error{claimcheck.ClaimTarget: &zitadel.StatusError{Code: 403, Message: "no permission"}}
	res, err := rotateActionKeys(context.Background(), "button", "admin-1", true)
	if err != nil || res.Outcome != "failed" || !strings.Contains(res.Error, "not allowed") {
		t.Fatalf("got %+v, %v", res, err)
	}
	if g.ev.count("zitadel:rotate:"+claimcheck.EventTarget) != 0 {
		t.Error("it went on to the next target after a refusal")
	}
	if c := injectSigned(t, log, oldClaimsKey); c.KeySlot != claimtel.KeySlotCurrent {
		t.Fatalf("the old key stopped verifying: %+v", c)
	}
	if g.sink.kinds()[claimtel.KindKeyRotation+" failed"] == 0 || g.ev.count("audit:action_key.not_replaced") != 1 {
		t.Fatalf("findings %v events %v", g.sink.kinds(), g.ev.list())
	}
}

// The test sign-in failing after a replacement: unverified, both keys stay
// accepted for days, a finding is raised, and the schedule does not try
// again.
func TestRotate_ProbeFailureHoldsBothKeysAndDoesNotLoop(t *testing.T) {
	g := newRotationRig(t)
	log := captureCalls(t)
	g.probe = probeResult{Pass: false, Class: "claims", Detail: "append_claims []"}
	withOldKeyDue(t)
	res, err := rotateActionKeys(context.Background(), "auto", "system", false)
	if err != nil || res.Outcome != "unverified" {
		t.Fatalf("got %+v, %v", res, err)
	}
	if g.sink.kinds()[claimtel.KindKeyRotation+" unverified"] == 0 {
		t.Fatalf("no finding: %v", g.sink.kinds())
	}
	if cfg := claimPlane.Config(); cfg == nil || time.Until(time.Unix(cfg.Keys.PreviousUntil, 0)) < 6*24*time.Hour {
		t.Fatalf("the previous claims key is not held: %+v", cfg)
	}
	if k := eventKeyring.Load(); k == nil || time.Until(time.Unix(k.PreviousUntil, 0)) < 6*24*time.Hour {
		t.Fatalf("the previous event key is not held: %+v", k)
	}
	if c := injectSigned(t, log, oldClaimsKey); c.KeySlot != claimtel.KeySlotPrevious {
		t.Fatalf("old key: %+v", c)
	}
	// The schedule looks again in the window: no second replacement.
	freshProbe(t, true)
	for i := 0; i < 3; i++ {
		if err := RunActionKeyAutoRotate(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if n := g.ev.count("zitadel:rotate:" + claimcheck.ClaimTarget); n != 1 {
		t.Fatalf("replaced %d times after an unverified one", n)
	}
}

// An unverified replacement is confirmed later by observation alone.
func TestConfirmUnverified_ByTheProbesNotAnotherReplacement(t *testing.T) {
	g := newRotationRig(t)
	g.probe = probeResult{Pass: false, Class: "timeout"}
	if _, err := rotateActionKeys(context.Background(), "button", "admin-1", true); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	_ = followKeyring(context.Background()) // the keyring read since
	freshProbe(t, true)
	t.Setenv(envAutoRotate, "off")
	if err := RunActionKeyAutoRotate(context.Background()); err != nil {
		t.Fatal(err)
	}
	st, _ := g.rec.state(context.Background(), claimcheck.ClaimTarget)
	if st.Last.Outcome != "verified" || g.ev.count("zitadel:rotate:") != 2 {
		t.Fatalf("outcome %s, events %v", st.Last.Outcome, g.ev.list())
	}
}

// ---------------------------------------------------------------------------
// The schedule.
// ---------------------------------------------------------------------------

func localAt(s string) time.Time {
	t, err := time.ParseInLocation("2006-01-02 15:04", s, clock.Loc)
	if err != nil {
		panic(err)
	}
	return t
}

func TestNextAutoAttempt_LeadAndWindow(t *testing.T) {
	c := autoRotateConfig{Threshold: 90 * 24 * time.Hour, Lead: 5 * 24 * time.Hour}
	c.start, c.end, _ = parseQuietWindow(defaultQuietWindow)
	ptr := func(t time.Time) *time.Time { return &t }
	cases := []struct {
		name           string
		now            time.Time
		last, lastAuto *time.Time
		want           time.Time
	}{
		{"92 days old, afternoon: the next window", localAt("2026-11-05 14:00"), ptr(localAt("2026-08-05 10:00")), nil, localAt("2026-11-06 03:00")},
		{"92 days old, inside the window: now", localAt("2026-11-06 03:20"), ptr(localAt("2026-08-05 10:00")), nil, localAt("2026-11-06 03:20")},
		{"just before the window ends", localAt("2026-11-06 04:59"), ptr(localAt("2026-08-05 10:00")), nil, localAt("2026-11-06 04:59")},
		{"window just ended", localAt("2026-11-06 05:00"), ptr(localAt("2026-08-05 10:00")), nil, localAt("2026-11-07 03:00")},
		{"fresh key: lead days before the limit", localAt("2026-11-06 12:00"), ptr(localAt("2026-11-06 03:10")), nil, localAt("2027-01-30 03:10")},
		{"due at noon: that night", localAt("2026-11-06 12:00"), ptr(localAt("2026-08-13 12:00")), nil, localAt("2026-11-07 03:00")},
		{"unknown age: due", localAt("2026-11-05 14:00"), nil, nil, localAt("2026-11-06 03:00")},
		{"tried this window: tomorrow's", localAt("2026-11-06 03:40"), ptr(localAt("2026-08-05 10:00")), ptr(localAt("2026-11-06 03:05")), localAt("2026-11-07 03:00")},
	}
	for _, tc := range cases {
		if got := nextAutoAttempt(tc.now, tc.last, tc.lastAuto, c); !got.Equal(tc.want) {
			t.Errorf("%s: got %s, want %s", tc.name, got.Format(time.DateTime), tc.want.Format(time.DateTime))
		}
	}
	// A window across midnight.
	c.start, c.end, _ = parseQuietWindow("23:30-01:00")
	if got := nextAutoAttempt(localAt("2026-11-06 00:30"), nil, nil, c); !got.Equal(localAt("2026-11-06 00:30")) {
		t.Errorf("across midnight, inside: %s", got)
	}
	if got := nextAutoAttempt(localAt("2026-11-06 12:00"), nil, nil, c); !got.Equal(localAt("2026-11-06 23:30")) {
		t.Errorf("across midnight, outside: %s", got)
	}
	for _, bad := range []string{"", "3-5", "03:00", "05:00-05:00", "25:00-26:00"} {
		if _, _, ok := parseQuietWindow(bad); ok {
			t.Errorf("window %q parsed", bad)
		}
	}
}

func withOldKeyDue(t *testing.T) {
	t.Helper()
	t.Setenv(envActionKeyRotatedAt, time.Now().Add(-92*24*time.Hour).UTC().Format(time.RFC3339))
	// The window is the whole day but this minute's neighbour, so the test
	// never depends on the hour it runs at.
	now := time.Now()
	t.Setenv(envQuietWindow, fmt.Sprintf("%s-%s", now.Add(-time.Minute).Format("15:04"), now.Add(2*time.Hour).Format("15:04")))
}

func freshProbe(t *testing.T, pass bool) {
	t.Helper()
	orig := lastProbeL1.Load()
	t.Cleanup(func() { lastProbeL1.Store(orig) })
	lastProbeL1.Store(&probeResult{At: time.Now(), Pass: pass})
}

// The first production run: a 92-day-old key, in the window, with L1 passing
// lately, is replaced and verified; not before L1 has passed; not outside the
// window; never with the kill switch.
func TestRunAuto_FirstRunOnAnOverdueKey(t *testing.T) {
	g := newRotationRig(t)
	withOldKeyDue(t)

	freshProbe(t, false)
	_ = RunActionKeyAutoRotate(context.Background())
	if g.ev.count("zitadel:rotate:") != 0 {
		t.Fatal("rotated while the test sign-in was failing")
	}

	t.Setenv(envAutoRotate, "off")
	freshProbe(t, true)
	_ = RunActionKeyAutoRotate(context.Background())
	if g.ev.count("zitadel:rotate:") != 0 {
		t.Fatal("rotated with the kill switch off")
	}

	t.Setenv(envAutoRotate, "")
	now := time.Now()
	t.Setenv(envQuietWindow, fmt.Sprintf("%s-%s", now.Add(time.Hour).Format("15:04"), now.Add(2*time.Hour).Format("15:04")))
	_ = RunActionKeyAutoRotate(context.Background())
	if g.ev.count("zitadel:rotate:") != 0 {
		t.Fatal("rotated outside the quiet window")
	}

	withOldKeyDue(t)
	if err := RunActionKeyAutoRotate(context.Background()); err != nil {
		t.Fatal(err)
	}
	st, _ := g.rec.state(context.Background(), claimcheck.ClaimTarget)
	if st.Last == nil || st.Last.Trigger != "auto" || st.Last.Outcome != "verified" || g.ev.count("zitadel:rotate:") != 2 {
		t.Fatalf("last %+v events %v", st.Last, g.ev.list())
	}
	// The next look finds nothing due.
	_ = RunActionKeyAutoRotate(context.Background())
	if g.ev.count("zitadel:rotate:") != 2 {
		t.Fatal("rotated again")
	}

	// The status now reads Syndra's own record, not .env.
	rr := httptest.NewRecorder()
	HandleActionRotationStatus(rr, httptest.NewRequest(http.MethodGet, "/api/v1/zitadel/action-rotation-status", nil))
	got := decodeRotationStatus(t, rr)
	if got.LastRotatedSource != "syndra" || got.AgeDays == nil || *got.AgeDays != 0 || got.Status != "ok" ||
		!got.Auto.On || got.Auto.NextAttemptAt == nil || got.LastOutcome == nil || got.LastOutcome.Outcome != "verified" || got.DueAt == nil {
		t.Fatalf("status %+v", got)
	}
}

// Staging has no Zitadel: the schedule is off and says why.
func TestAutoConfig_OffWithoutZitadel(t *testing.T) {
	t.Setenv("ZITADEL_DOMAIN", "")
	if c := autoConfig(); c.On || c.OffReason != "no_zitadel" {
		t.Fatalf("%+v", c)
	}
}

// ---------------------------------------------------------------------------
// The endpoint.
// ---------------------------------------------------------------------------

func postRotate(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	rr := httptest.NewRecorder()
	HandleRotateActionKey(rr, httptest.NewRequest(http.MethodPost, "/api/v1/zitadel/action-key/rotate", strings.NewReader(body)))
	return rr
}

func TestHandleRotateActionKey_StrictJSONAndResult(t *testing.T) {
	g := newRotationRig(t)
	for _, bad := range []string{``, `{"force":true,"extra":1}`, `{"force":"yes"}`, `[]`} {
		if rr := postRotate(t, bad); rr.Code != http.StatusBadRequest {
			t.Errorf("body %q: HTTP %d", bad, rr.Code)
		}
	}
	if g.ev.count("record:") != 0 {
		t.Fatal("a bad body reached the rotator")
	}
	rr := postRotate(t, `{"force":true}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("HTTP %d %s", rr.Code, rr.Body.String())
	}
	var res rotationResult
	_ = json.Unmarshal(rr.Body.Bytes(), &res)
	if res.Outcome != "verified" || res.Probe == nil || !res.Probe.Pass || res.NextDueAt == nil || len(res.Replaced) != 2 {
		t.Fatalf("%+v", res)
	}
	if strings.Contains(rr.Body.String(), "-new") {
		t.Fatal("the answer carried a key")
	}
	if g.ev.count("audit:action_key.replaced") != 1 {
		t.Fatalf("not audited: %v", g.ev.list())
	}
}

// Admin only, at the router.
func TestActionKeyRoutes_AreOperatorOnly(t *testing.T) {
	t.Setenv("ZITADEL_DOMAIN", "zitadel.example.org")
	t.Setenv("ZITADEL_AUDIENCE", "syndra")
	for _, path := range []string{"/api/v1/zitadel/action-key/rotate", "/api/v1/zitadel/action-key/verify"} {
		rr := httptest.NewRecorder()
		NewRouter().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"force":true}`)))
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("%s unauthenticated: HTTP %d", path, rr.Code)
		}
	}
}

// The by-hand guide's verify: reads Zitadel's key, runs the test sign-in,
// changes nothing.
func TestHandleVerifyActionKey(t *testing.T) {
	g := newRotationRig(t)
	log := captureCalls(t)
	// A manual rotate.sh: Zitadel has a new key Syndra has not read yet.
	g.zit.mu.Lock()
	g.zit.keys[claimcheck.ClaimTarget] = "claims-manual-0123456789abcdef01234567"
	g.zit.mu.Unlock()
	rr := httptest.NewRecorder()
	HandleVerifyActionKey(rr, httptest.NewRequest(http.MethodPost, "/api/v1/zitadel/action-key/verify", strings.NewReader(`{}`)))
	if rr.Code != http.StatusOK {
		t.Fatalf("HTTP %d %s", rr.Code, rr.Body.String())
	}
	var res verifyResult
	_ = json.Unmarshal(rr.Body.Bytes(), &res)
	if !res.Probe.Pass || res.LastRotatedAt == nil || res.NextDueAt == nil {
		t.Fatalf("%+v", res)
	}
	if c := injectSigned(t, log, "claims-manual-0123456789abcdef01234567"); c.KeySlot != claimtel.KeySlotCurrent {
		t.Fatalf("the manual key: %+v", c)
	}
	if g.ev.count("zitadel:rotate:") != 0 || g.ev.count("record:") != 0 {
		t.Fatalf("verify changed something: %v", g.ev.list())
	}
	rr = httptest.NewRecorder()
	HandleVerifyActionKey(rr, httptest.NewRequest(http.MethodPost, "/api/v1/zitadel/action-key/verify", strings.NewReader(`{"x":1}`)))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("strict JSON: HTTP %d", rr.Code)
	}
}

func TestServerCommand(t *testing.T) {
	t.Setenv(envServerDir, "/srv/syndra")
	if got := serverCommand("make zitadel-actions-rotate-key"); got != "cd /srv/syndra && make zitadel-actions-rotate-key" {
		t.Errorf("got %q", got)
	}
	for _, bad := range []string{"", "relative", "/srv/a b", "/srv/x;rm -rf /", "/srv/$(id)"} {
		t.Setenv(envServerDir, bad)
		if got := serverCommand("make x"); got != "make x" {
			t.Errorf("dir %q: %q", bad, got)
		}
	}
}
