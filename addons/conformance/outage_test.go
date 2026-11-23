package conformance

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"syndra/addons/reference"
)

// The outage checks, watched both ways: the reference passes them with its
// app switched off and on, and each check fails on a reply the reference
// never gives, written in by the test while the app is down.

// edit rewrites a JSON reply.
func edit(reply []byte, change func(m map[string]any)) []byte {
	var m map[string]any
	if json.Unmarshal(reply, &m) != nil {
		return reply
	}
	change(m)
	out, _ := json.Marshal(m)
	return out
}

func TestEachOutageCheckFailsOnABadReply(t *testing.T) {
	type rewrite = func(r *http.Request, reply []byte, down bool) []byte
	cases := []struct {
		name     string
		features []string
		mutate   bool
		rewrite  rewrite
		header   func(r *http.Request, h http.Header, down bool)
		require  bool // no hooks, -require-outage
		want     []string
	}{
		{name: "health says reachable while the app is down", want: []string{"outage.health"},
			rewrite: func(r *http.Request, reply []byte, down bool) []byte {
				if down && r.URL.Path == "/health" {
					return edit(reply, func(m map[string]any) { m["reachable"] = true })
				}
				return reply
			}},
		{name: "last_read_at moves while the app is down", features: []string{"freshness"}, want: []string{"outage.health"},
			rewrite: func(r *http.Request, reply []byte, down bool) []byte {
				if down && r.URL.Path == "/health" {
					return edit(reply, func(m map[string]any) { m["last_read_at"] = time.Now().UTC().Format(time.RFC3339Nano) })
				}
				return reply
			}},
		{name: "the copy is dated anew", features: []string{"freshness"}, want: []string{"outage.kept", "freshness.unreachable"},
			rewrite: func(r *http.Request, reply []byte, down bool) []byte {
				if down && r.URL.Path == "/subjects" {
					return edit(reply, func(m map[string]any) { m["taken_at"] = time.Now().UTC().Format(time.RFC3339Nano) })
				}
				return reply
			}},
		{name: "a fresh read with a copy says cooldown, not unreachable", features: []string{"freshness"}, want: []string{"outage.kept", "freshness.unreachable"},
			rewrite: func(r *http.Request, reply []byte, down bool) []byte {
				if down && r.URL.Query().Get("fresh") == "true" {
					return edit(reply, func(m map[string]any) { m["stale"] = map[string]any{"reason": "cooldown", "retry_after": 1} })
				}
				return reply
			}},
		{name: "no copy, and an empty list beside the problem", want: []string{"outage.none", "no-false-zero"},
			rewrite: func(r *http.Request, reply []byte, down bool) []byte {
				if down && r.URL.Path == "/subjects" {
					return edit(reply, func(m map[string]any) { m["subjects"] = []any{} })
				}
				return reply
			}},
		// A stopped app is down, never busy (8g fix round 3).
		{name: "a stopped app with no copy answered busy", want: []string{"outage.none"},
			rewrite: func(r *http.Request, reply []byte, down bool) []byte {
				if down && r.URL.Path == "/subjects" {
					return edit(reply, func(m map[string]any) {
						if _, problem := m["code"]; problem {
							m["code"], m["title"], m["type"] = "TARGET_BUSY", "Target busy", "urn:syndra:addon:error:TARGET_BUSY"
						}
					})
				}
				return reply
			}},
		// The NAS down, health.get withdrawn (design §13.1, the withdrawn row).
		{name: "a read withdrawn while the app is down", want: []string{"outage.health"},
			rewrite: func(r *http.Request, reply []byte, down bool) []byte {
				if down && r.URL.Path == "/capabilities" {
					return withdrawHealth(reply)
				}
				return reply
			}},
		{name: "an apply refused without Retry-After", mutate: true, want: []string{"outage.write"},
			header: func(r *http.Request, h http.Header, down bool) {
				if down && r.URL.Path == "/apply" {
					h.Del("Retry-After")
				}
			}},
		{name: "a zero taken_at", want: []string{"no-false-zero"},
			rewrite: func(r *http.Request, reply []byte, _ bool) []byte {
				if r.URL.Path == "/subjects" {
					return edit(reply, func(m map[string]any) {
						if _, ok := m["taken_at"]; ok {
							m["taken_at"] = "0001-01-01T00:00:00Z"
						}
					})
				}
				return reply
			}},
		{name: "current false without stale", features: []string{"freshness"}, want: []string{"no-false-zero"},
			rewrite: func(r *http.Request, reply []byte, _ bool) []byte {
				if r.URL.Path == "/subjects" {
					return edit(reply, func(m map[string]any) { delete(m, "stale") })
				}
				return reply
			}},
		{name: "no app hooks under -require-outage", require: true, want: []string{"outage.health", "outage.kept", "outage.none"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var down atomic.Bool
			var rw func(*http.Request, []byte) []byte
			if tc.rewrite != nil {
				rw = func(r *http.Request, reply []byte) []byte { return tc.rewrite(r, reply, down.Load()) }
			}
			var hd func(*http.Request, http.Header)
			if tc.header != nil {
				hd = func(r *http.Request, h http.Header) { tc.header(r, h, down.Load()) }
			}
			url, _ := serveAddon(t, reference.Config{Target: "reference", Secret: []byte(testSecret), Features: tc.features, Down: down.Load}, nil, rw, hd)
			cfg := withApp(config(url, tc.mutate), &down)
			if tc.require {
				cfg.StopApp, cfg.StartApp, cfg.RequireOutage = nil, nil, true
			}
			r, err := Run(context.Background(), cfg)
			if err != nil && !errors.Is(err, ErrReadsFailed) {
				t.Fatal(err)
			}
			for _, id := range tc.want {
				if got := r.Status(id); got != "fail" {
					t.Errorf("%s says %s, want fail", id, got)
				}
			}
			if !r.Failed() {
				t.Error("the run would exit 0")
			}
			if down.Load() {
				t.Error("the run left the app down")
			}
		})
	}
}

// withdrawHealth is a /capabilities reply with health.get withdrawn, as an
// add-on that had read no version of its app withdrew it.
func withdrawHealth(reply []byte) []byte {
	return edit(reply, func(m map[string]any) {
		for _, op := range m["operations"].([]any) {
			if op := op.(map[string]any); op["id"] == "health.get" {
				op["available"], op["unavailable_reason"] = false, "the target version has not been read yet"
			}
		}
	})
}

// The add-on is restarted while its app is down, as on staging: it has read
// nothing from the app, and its reads stay declared even so. One that
// withdraws a read only then fails outage.health.
func TestOutageHealthFailsAReadWithdrawnByARestartWithTheAppDown(t *testing.T) {
	var down, restarted atomic.Bool
	withdraw := func(r *http.Request, reply []byte) []byte {
		if down.Load() && restarted.Load() && r.URL.Path == "/capabilities" {
			return withdrawHealth(reply)
		}
		return reply
	}
	url, _ := serveAddon(t, reference.Config{Target: "reference", Secret: []byte(testSecret), Down: down.Load}, nil, withdraw, nil)
	cfg := withApp(config(url, false), &down)
	cfg.StopAddon = func(context.Context) error { return nil }
	cfg.StartAddon = func(context.Context) error { restarted.Store(true); return nil }
	r, _ := Run(context.Background(), cfg)
	if got := r.Status("outage.health"); got != "fail" {
		t.Errorf("outage.health says %s for a read withdrawn after a restart with the app down, want fail", got)
	}
}

// serveRestartable serves the reference with its app switch, and an add-on
// that can be restarted: keeping what it held (its disk kept) or not.
func serveRestartable(t *testing.T, down *atomic.Bool, features ...string) (url string, restart func(keep bool)) {
	t.Helper()
	rc := reference.Config{Target: "reference", Secret: []byte(testSecret), Features: features, Down: down.Load}
	first, err := reference.New(rc)
	if err != nil {
		t.Fatal(err)
	}
	var h atomic.Pointer[http.Handler]
	handler := first.Handler()
	h.Store(&handler)
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { (*h.Load()).ServeHTTP(w, r) }))
	srv.TLS = &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{first.Certificate()}}
	srv.StartTLS()
	t.Cleanup(srv.Close)
	return srv.URL, func(keep bool) {
		if keep {
			return
		}
		again, err := reference.New(rc) // the same key, nothing remembered
		if err != nil {
			t.Error(err)
			return
		}
		fresh := again.Handler()
		h.Store(&fresh)
	}
}

// Against the reference serving freshness and calls, every outage check
// passes with all four hooks; an add-on that forgets its calls on a restart
// fails outage.restart.
func TestTheOutageChecksPassAndARestartThatForgetsFails(t *testing.T) {
	for _, keep := range []bool{true, false} {
		t.Run(map[bool]string{true: "disk kept", false: "disk lost"}[keep], func(t *testing.T) {
			t.Parallel()
			var down atomic.Bool
			url, restart := serveRestartable(t, &down, "freshness", "calls")
			cfg := withApp(config(url, true), &down)
			cfg.StopAddon = func(context.Context) error { return nil }
			cfg.StartAddon = func(context.Context) error { restart(keep); return nil }
			r, err := Run(context.Background(), cfg)
			if !keep {
				// The rest of the run meets an add-on with no account of the kit's.
				if got := r.Status("outage.restart"); got != "fail" {
					t.Errorf("outage.restart says %s after a restart that forgot every call, want fail", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			for _, f := range failures(r) {
				t.Errorf("%s failed: %v", f.ID, f.Detail)
			}
			for _, id := range []string{"outage.health", "outage.kept", "outage.none", "outage.write", "outage.restart", "freshness.unreachable", "no-false-zero"} {
				// outage.none: with freshness the read just before the stop is
				// always kept, so no read is without a copy.
				want := map[bool]string{true: "skip", false: "pass"}[id == "outage.none"]
				if got := r.Status(id); got != want {
					t.Errorf("%s says %s, want %s", id, got, want)
				}
			}
		})
	}
}

// A -start-addon that returns before the add-on listens (docker start) is
// waited out: the kit polls /health until it answers, so the reads after the
// restart meet the add-on, not a closed port.
func TestAStartAddonThatReturnsBeforeListeningIsWaitedOut(t *testing.T) {
	var down atomic.Bool
	var listensAt atomic.Int64 // unix nanos; 0 is listening
	url, _ := serveAddon(t, reference.Config{Target: "reference", Secret: []byte(testSecret), Features: []string{"calls"}, Down: down.Load},
		func(*http.Request, []byte, bool) {
			if at := listensAt.Load(); at != 0 && time.Now().UnixNano() < at {
				panic(http.ErrAbortHandler) // no reply at all, as a port not yet bound
			}
		}, nil, nil)
	cfg := withApp(config(url, false), &down)
	cfg.StopAddon = func(context.Context) error { listensAt.Store(time.Now().Add(time.Hour).UnixNano()); return nil }
	cfg.StartAddon = func(context.Context) error {
		listensAt.Store(time.Now().Add(600 * time.Millisecond).UnixNano())
		return nil
	}
	r, _ := Run(context.Background(), cfg)
	for _, id := range []string{"outage.health", "outage.restart"} {
		if got := r.Status(id); got != "pass" {
			t.Errorf("%s says %s %v after a start that returned before the add-on listened, want pass", id, got, detailOf(r, id))
		}
	}
}

func detailOf(r *Report, id string) []string {
	for _, res := range r.Results {
		if res.ID == id {
			return res.Detail
		}
	}
	return nil
}

// Without hooks every outage check is "not checked", a skip, never a pass.
func TestWithoutHooksTheOutageChecksAreNotChecked(t *testing.T) {
	url, _ := serve(t, testSecret, nil)
	r := run(t, config(url, false))
	for _, res := range r.Results {
		if strings.HasPrefix(res.ID, "outage.") && res.ID != "outage.write" {
			if res.Status != "skip" || !slices.ContainsFunc(res.Detail, func(d string) bool { return strings.HasPrefix(d, "not checked") }) {
				t.Errorf("%s says %s %v without hooks, want skip: not checked", res.ID, res.Status, res.Detail)
			}
		}
	}
}

// With the app hooks but not the add-on's, outage.health says its restart
// step was not checked: never a silent pass over a step that did not run.
func TestOutageHealthSaysTheRestartWasNotChecked(t *testing.T) {
	var down atomic.Bool
	url, _ := serveAddon(t, reference.Config{Target: "reference", Secret: []byte(testSecret), Down: down.Load}, nil, nil, nil)
	r := run(t, withApp(config(url, false), &down))
	if got, detail := r.Status("outage.health"), detailOf(r, "outage.health"); got != "pass" ||
		!slices.ContainsFunc(detail, func(d string) bool { return strings.HasPrefix(d, "not checked: -stop-addon and -start-addon") }) {
		t.Errorf("outage.health says %s %v without the add-on hooks, want pass with its restart step not checked", got, detail)
	}
}

// Once a stop is tried the matching start runs, whatever the stop did: a hook
// that failed or hung half-way may have stopped the app, or the add-on,
// anyway. The app is the reference's, off while its file exists, as the
// binary's -stop-app "touch …; exit 1" leaves it.
func TestAFailedStopStillStarts(t *testing.T) {
	failed := func(context.Context) error { return errors.New("exit status 1") }
	hung := func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, 50*time.Millisecond) // the binary's hook timeout, shortened
		defer cancel()
		<-ctx.Done()
		return ctx.Err()
	}
	for name, fails := range map[string]func(context.Context) error{"failed": failed, "hung": hung} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			file := filepath.Join(t.TempDir(), "app-down")
			isDown := func() bool { _, err := os.Stat(file); return err == nil }
			url, _ := serveAddon(t, reference.Config{Target: "reference", Secret: []byte(testSecret), Down: isDown}, nil, nil, nil)
			cfg := config(url, false)
			addonStarted := false
			cfg.StopApp = func(ctx context.Context) error { _ = os.WriteFile(file, nil, 0o600); return fails(ctx) }
			cfg.StartApp = func(context.Context) error { return os.Remove(file) }
			cfg.StopAddon = fails
			cfg.StartAddon = func(context.Context) error { addonStarted = true; return nil }
			r, _ := Run(context.Background(), cfg)
			if isDown() {
				t.Error("the app-down file is still there: -start-app did not run after -stop-app failed")
			}
			if !addonStarted {
				t.Error("-start-addon did not run after -stop-addon failed")
			}
			for _, id := range []string{"outage.health", "outage.restart"} {
				if got := r.Status(id); got != "fail" {
					t.Errorf("%s says %s after its stop failed, want fail", id, got)
				}
			}
		})
	}
}

// A failed -stop-addon never hides a failed -start-addon: both are said.
func TestAFailedStopAndStartAreBothSaid(t *testing.T) {
	url, _ := serveAddon(t, reference.Config{Target: "reference", Secret: []byte(testSecret)}, nil, nil, nil)
	cfg := config(url, false)
	cfg.StopAddon = func(context.Context) error { return errors.New("stop broke") }
	cfg.StartAddon = func(context.Context) error { return errors.New("start broke") }
	r, _ := Run(context.Background(), cfg)
	for _, res := range r.Results {
		if res.ID != "outage.restart" {
			continue
		}
		detail := strings.Join(res.Detail, "\n")
		if !strings.Contains(detail, "stop broke") || !strings.Contains(detail, "start broke") {
			t.Errorf("outage.restart said %q; want both the stop's and the start's failure", detail)
		}
		return
	}
	t.Fatal("no outage.restart result")
}

// A cooldown longer than the kit waits skips freshness.unreachable, and only
// it: the other outage checks run on plain reads, or fail under
// -require-outage without the app hooks.
func TestNoFreshReadSkipsOnlyFreshnessUnreachable(t *testing.T) {
	for _, require := range []bool{false, true} {
		t.Run(map[bool]string{false: "with hooks", true: "no hooks, -require-outage"}[require], func(t *testing.T) {
			t.Parallel()
			var down atomic.Bool
			url, _ := serveAddon(t, reference.Config{Target: "reference", Secret: []byte(testSecret), Features: []string{"freshness"}, Down: down.Load}, nil, nil, nil)
			cfg := withApp(config(url, false), &down)
			cfg.FreshWait = time.Nanosecond
			if require {
				cfg.StopApp, cfg.StartApp, cfg.RequireOutage = nil, nil, true
			}
			r, _ := Run(context.Background(), cfg)
			if got := r.Status("freshness.unreachable"); got != "skip" {
				t.Errorf("freshness.unreachable says %s, want skip", got)
			}
			want := map[bool]string{false: "pass", true: "fail"}[require]
			for _, id := range []string{"outage.health", "outage.kept"} {
				if got := r.Status(id); got != want {
					t.Errorf("%s says %s, want %s", id, got, want)
				}
			}
		})
	}
}

// When the plan read just before the stop names an account the kit does not
// own, the run halts there: the app is never stopped.
func TestAHaltBeforeTheStopLeavesTheAppUp(t *testing.T) {
	plans := func(seen []string) int {
		return len(slices.DeleteFunc(seen, func(s string) bool { return s != "POST /plan" }))
	}
	// The first run finds which /plan is the one just before the stop.
	var down atomic.Bool
	url, seen := serveAddon(t, reference.Config{Target: "reference", Secret: []byte(testSecret), Down: down.Load}, nil, nil, nil)
	cfg := withApp(config(url, true), &down)
	stopAt := 0
	cfg.StopApp = func(context.Context) error { stopAt = plans(seen()); down.Store(true); return nil }
	run(t, cfg)
	if stopAt == 0 {
		t.Fatal("the first run never stopped the app")
	}

	var n atomic.Int32
	foreign := func(r *http.Request, reply []byte) []byte {
		if r.URL.Path != "/plan" || int(n.Add(1)) != stopAt {
			return reply
		}
		return edit(reply, func(m map[string]any) {
			if outs, _ := m["outcomes"].([]any); len(outs) > 0 {
				outs[0].(map[string]any)["account_id"] = "9999"
			}
		})
	}
	url, _ = serveAddon(t, reference.Config{Target: "reference", Secret: []byte(testSecret), Down: down.Load}, nil, foreign, nil)
	cfg = withApp(config(url, true), &down)
	stopped := false
	cfg.StopApp = func(context.Context) error { stopped = true; return nil }
	r, _ := Run(context.Background(), cfg)
	if stopped {
		t.Error("the app was stopped after the plan before the stop named an account the kit does not own")
	}
	if got := r.Status("bound-to-created"); got != "fail" {
		t.Errorf("bound-to-created says %s, want fail", got)
	}
}

// An add-on that dates its copy anew on a restart fails outage.restart.
func TestARestartThatRedatesTheCopyFails(t *testing.T) {
	var down, restarted atomic.Bool
	redate := func(r *http.Request, reply []byte) []byte {
		if !restarted.Load() || r.URL.Path != "/subjects" {
			return reply
		}
		return edit(reply, func(m map[string]any) {
			if m["current"] == false {
				m["taken_at"] = time.Now().Add(time.Hour).UTC().Format(time.RFC3339Nano)
			}
		})
	}
	url, _ := serveAddon(t, reference.Config{Target: "reference", Secret: []byte(testSecret), Features: []string{"freshness"}, Down: down.Load}, nil, redate, nil)
	cfg := withApp(config(url, false), &down)
	cfg.StopAddon = func(context.Context) error { return nil }
	cfg.StartAddon = func(context.Context) error { restarted.Store(true); return nil }
	r, _ := Run(context.Background(), cfg)
	if got := r.Status("outage.restart"); got != "fail" {
		t.Errorf("outage.restart says %s after a restart that dated the copy anew, want fail", got)
	}
}

// outage.hang, watched both ways: the reference answers /health and its
// read operations within their bounds while its app hangs, and passes; a
// /health that waits on the hanging app past healthWithin, a read operation
// that waits past readWithin, or a /health that says reachable while it
// hangs, fails; one that
// counts an answer from within recentReadWindow passes, because the kit
// waits past it (hangSettle); without -hang-app under -require-outage it
// fails as not checked.
func TestOutageHangFailsAHealthThatWaitsOnTheApp(t *testing.T) {
	// Short, so the read that waits past it costs the run 2 s, not 25.
	was := readWithin
	readWithin = 2 * time.Second
	t.Cleanup(func() { readWithin = was }) // after the parallel subtests, unlike a defer
	// hungFor is how long the app has been hanging, zero while it is not.
	slow := func(r *http.Request, reply []byte, hungFor time.Duration) []byte {
		if hungFor > 0 && r.URL.Path == "/health" {
			time.Sleep(healthWithin)
		}
		return reply
	}
	// A read operation that waits on the hanging app past readWithin: what
	// health.get did on the 8g staging rerun, so Syndra gave up on the add-on.
	slowRead := func(r *http.Request, reply []byte, hungFor time.Duration) []byte {
		if hungFor > 0 && r.URL.Path == "/operations/health.get" {
			time.Sleep(readWithin)
		}
		return reply
	}
	says := func(r *http.Request, reply []byte, hungFor time.Duration) []byte {
		if hungFor > 0 && r.URL.Path == "/health" {
			return edit(reply, func(m map[string]any) { m["reachable"] = true })
		}
		return reply
	}
	// Counts its last answer, from just before the hang, while it is recent
	// (§13.2), as the TrueNAS add-on does while its session is busy.
	recent := func(r *http.Request, reply []byte, hungFor time.Duration) []byte {
		if hungFor > 0 && r.URL.Path == "/health" {
			return edit(reply, func(m map[string]any) { m["reachable"] = hungFor < recentReadWindow })
		}
		return reply
	}
	// A read with no copy answered busy: not an outage, and a no-copy code
	// (TARGET_BUSY, 8g fix round 2, 22 Nov 2026).
	busy := func(r *http.Request, reply []byte, hungFor time.Duration) []byte {
		if hungFor > 0 && strings.HasPrefix(r.URL.Path, "/operations/") {
			return edit(reply, func(m map[string]any) {
				if _, problem := m["code"]; problem {
					m["code"], m["title"], m["type"] = "TARGET_BUSY", "Target busy", "urn:syndra:addon:error:TARGET_BUSY"
				}
			})
		}
		return reply
	}
	// /health answering busy: a read or operation answer only (8g fix round 3).
	busyHealth := func(r *http.Request, reply []byte, hungFor time.Duration) []byte {
		if hungFor > 0 && r.URL.Path == "/health" {
			return edit(reply, func(m map[string]any) { m["code"] = "TARGET_BUSY" })
		}
		return reply
	}
	for name, tc := range map[string]struct {
		rewrite func(*http.Request, []byte, time.Duration) []byte
		noHook  bool
		want    string
	}{
		"the reference":                     {want: "pass"},
		"a health that waits on the app":    {rewrite: slow, want: "fail"},
		"a read that waits on the app":      {rewrite: slowRead, want: "fail"},
		"reachable while the app hangs":     {rewrite: says, want: "fail"},
		"reachable on a recent answer":      {rewrite: recent, want: "pass"},
		"a read with no copy answered busy": {rewrite: busy, want: "pass"},
		"a health that answered busy":       {rewrite: busyHealth, want: "fail"},
		"no -hang-app under require-outage": {noHook: true, want: "fail"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var down, hang atomic.Bool
			var hungAt atomic.Int64
			var rw func(*http.Request, []byte) []byte
			if tc.rewrite != nil {
				rw = func(r *http.Request, reply []byte) []byte {
					var hungFor time.Duration
					if hang.Load() {
						hungFor = time.Since(time.Unix(0, hungAt.Load()))
					}
					return tc.rewrite(r, reply, hungFor)
				}
			}
			url, _ := serveAddon(t, reference.Config{Target: "reference", Secret: []byte(testSecret), Down: down.Load, Hang: hang.Load}, nil, rw, nil)
			cfg := withHang(config(url, false), &down, &hang)
			cfg.RequireOutage = true
			if tc.noHook {
				cfg.HangApp = nil
			} else {
				hangApp := cfg.HangApp
				cfg.HangApp = func(ctx context.Context) error { hungAt.Store(time.Now().UnixNano()); return hangApp(ctx) }
			}
			r, _ := Run(context.Background(), cfg)
			if got := r.Status("outage.hang"); got != tc.want {
				t.Errorf("outage.hang says %s, want %s", got, tc.want)
			}
			if tc.want == "pass" {
				for _, f := range failures(r) {
					t.Errorf("%s failed: %v", f.ID, f.Detail)
				}
			}
			if hang.Load() || down.Load() {
				t.Error("the run left the app hanging")
			}
		})
	}
}
