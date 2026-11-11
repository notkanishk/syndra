package ratelimit

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func req(remote string, xff ...string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/api/action/inject", nil)
	r.RemoteAddr = remote
	for _, v := range xff {
		r.Header.Add("X-Forwarded-For", v)
	}
	return r
}

// clocked is a limiter on Memory whose clock the test moves.
func clocked(failures, self []Rule) (*Limiter, *time.Time) {
	now := time.Unix(1_800_000_000, 0)
	l := New(Memory(), failures, self)
	l.now = func() time.Time { return now }
	return l, &now
}

func TestFailuresLimitPerSourceAndRefill(t *testing.T) {
	l, now := clocked([]Rule{{3, time.Minute}}, nil)
	a := req("203.0.113.7:4000")
	for i := 0; i < 3; i++ {
		if w := l.Failed(a); w != 0 {
			t.Fatalf("failure %d limited (%s) under the limit", i+1, w)
		}
	}
	w := l.Failed(a)
	if w <= 0 || w > 20*time.Second {
		t.Fatalf("4th failure: wait %s, want (0, 20s]", w)
	}
	// Another source is its own bucket.
	if w := l.Failed(req("203.0.113.8:4000")); w != 0 {
		t.Fatalf("another source limited: %s", w)
	}
	// One token refills in Per/N.
	*now = now.Add(20 * time.Second)
	if w := l.Failed(a); w != 0 {
		t.Fatalf("after a refill: wait %s", w)
	}
	if got := l.Stats().Limited["POST /api/action/inject"]; got != 1 {
		t.Fatalf("limited count = %d, want 1", got)
	}
}

func TestPersonHasBothWindows(t *testing.T) {
	l, now := clocked(nil, DefaultSelfService)
	r := req("203.0.113.2:1")
	n := 0
	// A write every 6s never meets the minute limit, so only the hour can stop it.
	for ; n < 100 && l.Person(r, "u1") == 0; n++ {
		*now = now.Add(6 * time.Second)
	}
	// 50 to start, plus what the hour bucket refilled meanwhile (50/h, one per 72s).
	if n < 50 || n > 60 {
		t.Fatalf("a steady writer got %d writes before the hour limit, want 50-60", n)
	}
	// Ten at once pass, the eleventh does not, for a fresh person.
	for i := 0; i < 10; i++ {
		if w := l.Person(r, "u2"); w != 0 {
			t.Fatalf("write %d limited", i+1)
		}
	}
	if w := l.Person(r, "u2"); w == 0 {
		t.Fatal("the 11th write in a minute passed")
	}
	if w := l.Person(r, ""); w != 0 {
		t.Fatal("no subject was counted")
	}
}

// A 5-second poll runs past the in-doubt hour unlimited; sixty at once pass
// and the sixty-first does not.
func TestPollAllowsTheUIsPollAndStopsALoop(t *testing.T) {
	l, now := clocked(nil, nil)
	r := req("203.0.113.2:1")
	for i := 0; i < 2*720; i++ {
		if w := l.Poll(r, "u1"); w != 0 {
			t.Fatalf("poll %d at 5s limited", i+1)
		}
		*now = now.Add(5 * time.Second)
	}
	for i := 0; i < 60; i++ {
		if w := l.Poll(r, "u2"); w != 0 {
			t.Fatalf("burst poll %d limited", i+1)
		}
	}
	if l.Poll(r, "u2") == 0 {
		t.Fatal("the 61st poll in a second passed")
	}
	if l.Poll(r, "") != 0 || (*Limiter)(nil).Poll(r, "u3") != 0 {
		t.Fatal("no subject, or no limiter, was counted")
	}
}

func TestFailsOpen(t *testing.T) {
	down := New(func(context.Context, []string, []Rule, time.Time) (time.Duration, error) {
		return 0, errors.New("connection refused")
	}, []Rule{{1, time.Minute}}, nil)
	slow := New(func(ctx context.Context, _ []string, _ []Rule, _ time.Time) (time.Duration, error) {
		<-ctx.Done() // a hung Redis: the deadline ends it
		return 0, ctx.Err()
	}, []Rule{{1, time.Minute}}, nil)
	for _, l := range []*Limiter{down, slow} {
		for i := 0; i < 5; i++ {
			start := time.Now()
			if w := l.Failed(req("203.0.113.7:1")); w != 0 {
				t.Fatalf("limited with Redis unavailable: %s", w)
			}
			if el := time.Since(start); el > 5*Timeout {
				t.Fatalf("a slow Redis held the request %s", el)
			}
		}
		if s := l.Stats(); s.FailOpen != 5 || len(s.Limited) != 0 {
			t.Fatalf("stats = %+v, want 5 fail-open and none limited", s)
		}
	}
}

func TestKillSwitchAndEnv(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	defer rdb.Close()
	env := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

	if l := FromEnv(rdb, env(map[string]string{EnvSwitch: "OFF"})); l != nil {
		t.Fatal("SYNDRA_RATE_LIMITS=off still built a limiter")
	}
	var off *Limiter
	r := req("203.0.113.7:1")
	for i := 0; i < 1000; i++ {
		if off.Failed(r) != 0 || off.Person(r, "u") != 0 {
			t.Fatal("the off limiter limited")
		}
	}
	if !off.Stats().Off {
		t.Fatal("health does not say the limits are off")
	}

	l := FromEnv(rdb, env(map[string]string{EnvFailures: "5/10s", EnvSelfService: "nonsense"}))
	if len(l.failures) != 1 || l.failures[0] != (Rule{5, 10 * time.Second}) {
		t.Fatalf("failures = %v", l.failures)
	}
	if len(l.selfService) != 2 || l.selfService[1] != DefaultSelfService[1] {
		t.Fatalf("an unparsable value did not keep the default: %v", l.selfService)
	}
}

func TestParseRules(t *testing.T) {
	got, err := ParseRules(" 10/1m, 50/1h ")
	if err != nil || len(got) != 2 || got[0] != (Rule{10, time.Minute}) || got[1] != (Rule{50, time.Hour}) {
		t.Fatalf("got %v, %v", got, err)
	}
	for _, bad := range []string{"", "10", "0/1m", "-1/1m", "x/1m", "10/1ms", "10/soon"} {
		if _, err := ParseRules(bad); err == nil {
			t.Errorf("%q parsed", bad)
		}
	}
}

func TestClientIP(t *testing.T) {
	for _, c := range []struct {
		r    *http.Request
		want string
	}{
		{req("203.0.113.7:1"), "203.0.113.7"},
		// A public peer cannot name somebody else.
		{req("203.0.113.7:1", "198.51.100.1"), "203.0.113.7"},
		// Behind Caddy: the last entry, the one the proxy wrote.
		{req("[fd00::5]:1", "198.51.100.9, 198.51.100.1"), "198.51.100.1"},
		{req("[fd00::18]:1", "198.51.100.1", "2001:db8::1"), "2001:db8::1"},
		{req("127.0.0.1:1", "not-an-ip"), "127.0.0.1"},
		{req("[fd00::3]:1"), "fd00::3"},
	} {
		if got := ClientIP(c.r); got != c.want {
			t.Errorf("%s %v: got %s, want %s", c.r.RemoteAddr, c.r.Header.Values("X-Forwarded-For"), got, c.want)
		}
	}
}

func TestRefuse(t *testing.T) {
	for wait, want := range map[time.Duration]string{0: "1", 300 * time.Millisecond: "1", 6 * time.Second: "6", 6001 * time.Millisecond: "7"} {
		rr := httptest.NewRecorder()
		Refuse(rr, wait)
		var body map[string]string
		_ = json.Unmarshal(rr.Body.Bytes(), &body)
		if rr.Code != http.StatusTooManyRequests || rr.Header().Get("Retry-After") != want || body["message"] != Message || body["error"] != "RATE_LIMITED" {
			t.Errorf("wait %s: %d Retry-After=%q %v", wait, rr.Code, rr.Header().Get("Retry-After"), body)
		}
	}
}

// TestRedisMatchesMemory runs the Lua bucket against a real Redis and holds it
// to Memory, step for step. Skipped unless SYNDRA_TEST_REDIS_URL names one
// (host:port; DB 15 is used and only ratelimit:test:* keys, which expire).
func TestRedisMatchesMemory(t *testing.T) {
	addr := os.Getenv("SYNDRA_TEST_REDIS_URL")
	if addr == "" {
		t.Skip("SYNDRA_TEST_REDIS_URL unset")
	}
	rdb := redis.NewClient(&redis.Options{Addr: addr, DB: 15})
	defer rdb.Close()
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	prefix := "ratelimit:test:" + hex.EncodeToString(b)
	rules := []Rule{{3, time.Minute}, {5, time.Hour}}
	keys := []string{prefix + ":m", prefix + ":h"}
	red, mem := Redis(rdb), Memory()
	now := time.Now()
	for i, step := range []time.Duration{0, 0, 0, 0, 10 * time.Second, 10 * time.Second, 0, time.Minute, 0, 0, 0, 0} {
		now = now.Add(step)
		ctx := context.Background()
		got, err := red(ctx, keys, rules, now)
		if err != nil {
			t.Fatal(err)
		}
		want, _ := mem(ctx, keys, rules, now)
		if got != want {
			t.Fatalf("step %d: Redis wait %s, Memory %s", i, got, want)
		}
	}
	if ttl := rdb.PTTL(context.Background(), keys[0]).Val(); ttl <= 0 || ttl > time.Minute {
		t.Fatalf("bucket TTL %s, want (0, 1m]", ttl)
	}
}
