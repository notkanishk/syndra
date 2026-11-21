package conformance

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
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
