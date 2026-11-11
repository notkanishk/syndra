// Package ratelimit limits abuse of Syndra's public doors and never genuine
// traffic (openspec/changes/rate-limits). Two things are counted, nothing else:
//
//   - a call to a signed door (the Actions target, the event webhook) whose
//     signature failed, per source IP. A call that verifies is never counted,
//     so Zitadel's own calls, which arrive on every sign-in from the same few
//     addresses, can never be limited, and neither can Syndra's L1 and L2
//     probes, which sign theirs;
//   - a member's own password or storage write, per person, at limits no
//     person typing reaches: they guard against a stuck client.
//
// The buckets live in Redis so every process shares them. When Redis is down
// or slow the request is allowed, counted and logged: a limiter that blocks
// when its store fails is in the way of legitimate use.
package ratelimit

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

// Rule is a token bucket: N tokens, refilled at N per Per. "10/1m" is ten at
// once and one more every six seconds after.
type Rule struct {
	N   int
	Per time.Duration
}

func (r Rule) String() string { return fmt.Sprintf("%d/%s", r.N, r.Per) }

// The defaults. Env overrides them; SYNDRA_RATE_LIMITS=off turns every limit
// off.
var (
	// DefaultFailures is failed signature checks per source IP per endpoint.
	DefaultFailures = []Rule{{30, time.Minute}}
	// DefaultSelfService is one person's own password or storage writes per
	// endpoint: ten a minute and fifty an hour.
	DefaultSelfService = []Rule{{10, time.Minute}, {50, time.Hour}}
	// DefaultPoll is one person's reads of a page that polls, per endpoint:
	// sixty at once, then one a second. The UI's 5-second poll spends a fifth
	// of it, for as long as it runs; a stuck client in a loop meets it.
	DefaultPoll = []Rule{{60, time.Minute}}
)

// Env names.
const (
	EnvSwitch      = "SYNDRA_RATE_LIMITS"
	EnvFailures    = "SYNDRA_RATE_LIMIT_FAILURES"
	EnvSelfService = "SYNDRA_RATE_LIMIT_SELF_SERVICE"
)

// Message is what a limited request is told, in the copy guide's voice.
const Message = "Too many tries. Wait a minute and try again."

// Timeout bounds the one Redis call a counted request makes. Past it the
// request is allowed.
const Timeout = 50 * time.Millisecond

// Store takes one token from each key's bucket (rules[i] for keys[i]) when
// every bucket has one, and returns 0. Otherwise it takes none and returns
// how long until it could.
type Store func(ctx context.Context, keys []string, rules []Rule, now time.Time) (time.Duration, error)

// Limiter counts the two things above. A nil *Limiter allows everything: it
// is what the kill switch returns.
type Limiter struct {
	store       Store
	failures    []Rule
	selfService []Rule
	poll        []Rule
	now         func() time.Time

	limited  sync.Map // endpoint -> *atomic.Int64
	failOpen atomic.Int64
	down     atomic.Bool
}

// New is a limiter over store with the given rules, and DefaultPoll.
func New(store Store, failures, selfService []Rule) *Limiter {
	return &Limiter{store: store, failures: failures, selfService: selfService, poll: DefaultPoll, now: time.Now}
}

// FromEnv is the limiter a process runs: Redis-backed, the defaults unless
// env overrides them, and nil (everything allowed) when SYNDRA_RATE_LIMITS is
// off or there is no Redis. A value that does not parse keeps the default and
// says so.
func FromEnv(rdb redis.Scripter, getenv func(string) string) *Limiter {
	if strings.EqualFold(strings.TrimSpace(getenv(EnvSwitch)), "off") {
		log.Printf("[RATE LIMIT] %s=off: no request is limited", EnvSwitch)
		return nil
	}
	if rdb == nil {
		return nil
	}
	return New(Redis(rdb), rulesFromEnv(getenv, EnvFailures, DefaultFailures), rulesFromEnv(getenv, EnvSelfService, DefaultSelfService))
}

func rulesFromEnv(getenv func(string) string, name string, def []Rule) []Rule {
	v := strings.TrimSpace(getenv(name))
	if v == "" {
		return def
	}
	rules, err := ParseRules(v)
	if err != nil {
		log.Printf("[RATE LIMIT] %s=%q does not parse (%v); keeping %v", name, v, err, def)
		return def
	}
	return rules
}

// ParseRules reads "10/1m,50/1h".
func ParseRules(s string) ([]Rule, error) {
	var rules []Rule
	for _, part := range strings.Split(s, ",") {
		n, per, ok := strings.Cut(strings.TrimSpace(part), "/")
		if !ok {
			return nil, fmt.Errorf("%q is not N/duration", part)
		}
		count, err := strconv.Atoi(n)
		if err != nil || count < 1 {
			return nil, fmt.Errorf("%q: N must be a whole number above 0", part)
		}
		d, err := time.ParseDuration(per)
		if err != nil || d < time.Second {
			return nil, fmt.Errorf("%q: the duration must be at least 1s", part)
		}
		rules = append(rules, Rule{count, d})
	}
	return rules, nil
}

// Failed counts one failed signature check from r's source and returns how
// long that source must wait, or 0 when it is under the limit. Call it only
// after the check has failed: a call that verifies is never counted.
func (l *Limiter) Failed(r *http.Request) time.Duration {
	if l == nil {
		return 0
	}
	return l.take(r, "ip:"+ClientIP(r), l.failures)
}

// Person counts one self-service write by who and returns how long they must
// wait, or 0. A request with no subject is not counted: there is nobody to
// bind the bucket to, and the handler refuses it anyway.
func (l *Limiter) Person(r *http.Request, who string) time.Duration {
	if l == nil || who == "" {
		return 0
	}
	return l.take(r, "person:"+who, l.selfService)
}

// Poll counts one polled read by who (DefaultPoll), as Person counts a write.
func (l *Limiter) Poll(r *http.Request, who string) time.Duration {
	if l == nil || who == "" {
		return 0
	}
	return l.take(r, "person:"+who, l.poll)
}

func (l *Limiter) take(r *http.Request, who string, rules []Rule) time.Duration {
	if len(rules) == 0 {
		return 0
	}
	endpoint := Endpoint(r)
	keys := make([]string, len(rules))
	for i, rule := range rules {
		keys[i] = "ratelimit:" + endpoint + ":" + who + ":" + rule.Per.String()
	}
	ctx, cancel := context.WithTimeout(r.Context(), Timeout)
	defer cancel()
	wait, err := l.store(ctx, keys, rules, l.now())
	if err != nil {
		// Fail open: counted, logged once per outage, never blocking.
		l.failOpen.Add(1)
		if l.down.CompareAndSwap(false, true) {
			log.Printf("[RATE LIMIT] Redis did not answer; allowing every request until it does: %v", err)
		}
		return 0
	}
	if l.down.CompareAndSwap(true, false) {
		log.Printf("[RATE LIMIT] Redis answers again; limits apply")
	}
	if wait > 0 {
		c, _ := l.limited.LoadOrStore(endpoint, new(atomic.Int64))
		c.(*atomic.Int64).Add(1)
	}
	return wait
}

// Stats is what health shows: limited requests per endpoint, and how many
// were allowed because Redis did not answer.
type Stats struct {
	Off      bool             `json:"off,omitempty"`
	Limited  map[string]int64 `json:"limited"`
	FailOpen int64            `json:"fail_open"`
}

// Stats since the process started.
func (l *Limiter) Stats() Stats {
	if l == nil {
		return Stats{Off: true, Limited: map[string]int64{}}
	}
	s := Stats{Limited: map[string]int64{}, FailOpen: l.failOpen.Load()}
	l.limited.Range(func(k, v any) bool {
		s.Limited[k.(string)] = v.(*atomic.Int64).Load()
		return true
	})
	return s
}

// Refuse answers 429 with Retry-After in whole seconds, at least one.
func Refuse(w http.ResponseWriter, wait time.Duration) {
	w.Header().Set("Retry-After", strconv.Itoa(int(math.Max(1, math.Ceil(wait.Seconds())))))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusTooManyRequests)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "RATE_LIMITED", "message": Message})
}

// Endpoint is the route's pattern ("POST /api/action/inject"), so a path with
// an id in it is one endpoint, not one per id. Outside a mux, the path.
func Endpoint(r *http.Request) string {
	if r.Pattern != "" {
		return r.Pattern
	}
	return r.Method + " " + r.URL.Path
}

// ClientIP is the address a request came from. Behind Caddy, or Docker's port
// mapping, the peer is a private address and the client is the last entry of
// X-Forwarded-For, the one the nearest proxy wrote. From any other peer the
// header is ignored.
//
// ponytail: a private peer is trusted to forward. One that lies can only move
// failed calls between buckets, since a verified call is never counted; name
// the proxy's address in env if that ever matters.
func ClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if ip := net.ParseIP(host); ip != nil && (ip.IsPrivate() || ip.IsLoopback()) {
		if xff := r.Header.Values("X-Forwarded-For"); len(xff) > 0 {
			parts := strings.Split(xff[len(xff)-1], ",")
			if last := strings.TrimSpace(parts[len(parts)-1]); net.ParseIP(last) != nil {
				return last
			}
		}
	}
	return host
}

// bucketScript is the token bucket, every rule at once, atomically. ARGV[1]
// is now in ms, then N and Per (ms) per key. State is the hash {t: tokens, ts:
// last refill}; a bucket untouched for Per is full, so it expires then.
var bucketScript = redis.NewScript(`
local now = tonumber(ARGV[1])
local wait = 0
local tokens, at = {}, {}
for i = 1, #KEYS do
  local n, per = tonumber(ARGV[2*i]), tonumber(ARGV[2*i+1])
  local s = redis.call('HMGET', KEYS[i], 't', 'ts')
  local t, ts = tonumber(s[1]), tonumber(s[2])
  if t == nil or ts == nil then t, ts = n, now end
  local a = math.max(now, ts)
  t = math.min(n, t + (a - ts) * n / per)
  if t < 1 then wait = math.max(wait, math.ceil((1 - t) * per / n)) end
  tokens[i], at[i] = t, a
end
for i = 1, #KEYS do
  local t = tokens[i]
  if wait == 0 then t = t - 1 end
  redis.call('HSET', KEYS[i], 't', t, 'ts', at[i])
  redis.call('PEXPIRE', KEYS[i], tonumber(ARGV[2*i+1]))
end
return wait
`)

// Redis is the shared store.
func Redis(rdb redis.Scripter) Store {
	return func(ctx context.Context, keys []string, rules []Rule, now time.Time) (time.Duration, error) {
		args := []any{now.UnixMilli()}
		for _, r := range rules {
			args = append(args, r.N, r.Per.Milliseconds())
		}
		ms, err := bucketScript.Run(ctx, rdb, keys, args...).Int64()
		return time.Duration(ms) * time.Millisecond, err
	}
}

// Memory is the same bucket in this process: the model bucketScript is held
// to (TestRedisMatchesMemory), and the store unit tests run on.
func Memory() Store {
	type state struct{ t, ts float64 }
	var mu sync.Mutex
	m := map[string]state{}
	return func(_ context.Context, keys []string, rules []Rule, now time.Time) (time.Duration, error) {
		mu.Lock()
		defer mu.Unlock()
		ms := float64(now.UnixMilli())
		var wait float64
		next := make([]state, len(keys))
		for i, k := range keys {
			n, per := float64(rules[i].N), float64(rules[i].Per.Milliseconds())
			s, ok := m[k]
			if !ok {
				s = state{n, ms}
			}
			a := math.Max(ms, s.ts)
			t := math.Min(n, s.t+(a-s.ts)*n/per)
			if t < 1 {
				wait = math.Max(wait, math.Ceil((1-t)*per/n))
			}
			next[i] = state{t, a}
		}
		for i, k := range keys {
			if wait == 0 {
				next[i].t--
			}
			m[k] = next[i]
		}
		return time.Duration(wait) * time.Millisecond, nil
	}
}
