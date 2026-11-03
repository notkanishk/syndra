package claimserve

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"

	"syndra/internal/claimtel"
)

// The relay (claims-delivery-integrity S7, P4). The claims process has no
// Postgres: what it learns goes to Redis, and the backend takes it from there.
//
//   - Counters: every few seconds the claims process's recorder flushes into
//     claims:tel:{minute} (HINCRBY, TTL 2h). The backend folds each closed
//     minute into Postgres exactly once (RENAME to a fold key, write, DEL).
//   - Recent outcomes and the last served/called times, for the backend's
//     /healthz/claims: claims:tel:recent:{minute} (TTL 15m), claims:tel:last.
//   - Calls, for the sign-in correlator: claims:calls:{person}␟{client}, the
//     last relayCallsKept calls of each pair, TTL 20m.
//   - Signals the backend acts on: a person to recompile, the keyring to read
//     again, a debug capture. claims:signals, capped at relaySignalsKept.
//
// The backend's own fallback door (:8080) records in process as before; the
// backend reads both and merges them.

// RelayRedis is what the relay needs beyond the data plane's reads.
type RelayRedis interface {
	Redis
	HIncrBy(ctx context.Context, key, field string, incr int64) *redis.IntCmd
	HSet(ctx context.Context, key string, values ...any) *redis.IntCmd
	HGetAll(ctx context.Context, key string) *redis.MapStringStringCmd
	Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd
	Rename(ctx context.Context, key, newkey string) *redis.StatusCmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd
	SRem(ctx context.Context, key string, members ...any) *redis.IntCmd
	RPush(ctx context.Context, key string, values ...any) *redis.IntCmd
	LTrim(ctx context.Context, key string, start, stop int64) *redis.StatusCmd
	LRange(ctx context.Context, key string, start, stop int64) *redis.StringSliceCmd
	LPopCount(ctx context.Context, key string, count int) *redis.StringSliceCmd
	Ping(ctx context.Context) *redis.StatusCmd
}

const (
	telMinutesKey = "claims:tel:minutes"
	telLastKey    = "claims:tel:last"
	signalsKey    = "claims:signals"
	// DebugKey holds debug mode's watched (client → person, end) set, which
	// the backend writes and the snapshot carries to the claims process.
	DebugKey = "claims:debug"

	telKeep          = 2 * time.Hour
	recentKeep       = 15 * time.Minute
	relayCallsKept   = 64
	relayCallsKeep   = 20 * time.Minute
	relaySignalsKept = 1000
	relayBacklogCap  = 20000 // calls held across failed flushes
	// RelayEvery is how often the claims process flushes to Redis.
	RelayEvery = 2 * time.Second

	sep = "\x1f" // unit separator: in no id, function or outcome
)

func telKey(m int64) string       { return "claims:tel:" + strconv.FormatInt(m, 10) }
func foldKey(m int64) string      { return "claims:tel:fold:" + strconv.FormatInt(m, 10) }
func recentKey(m int64) string    { return "claims:tel:recent:" + strconv.FormatInt(m, 10) }
func callsKey(u, c string) string { return "claims:calls:" + u + sep + c }

// Signal is one thing the claims process asks the backend to do.
type Signal struct {
	Kind    string        `json:"k"` // repair | nudge | debug
	UserID  string        `json:"u,omitempty"`
	Capture *DebugCapture `json:"d,omitempty"`
}

// DebugWatch is one watched sign-in for debug mode, by client id.
type DebugWatch struct {
	UserID string `json:"user_id"`
	Ends   int64  `json:"ends"` // unix seconds
}

// PublishDebug writes debug mode's watch set through and moves claims:gen, so
// the claims process captures the same sign-ins the backend would.
func PublishDebug(ctx context.Context, rdb Redis, watch map[string]DebugWatch) error {
	v, err := encode(watch)
	if err != nil {
		return err
	}
	if err := rdb.MSet(ctx, DebugKey, v).Err(); err != nil {
		return fmt.Errorf("publish debug watch: %w", err)
	}
	return rdb.Incr(ctx, GenKey).Err()
}

// ---------------------------------------------------------------------------
// The claims process's side.
// ---------------------------------------------------------------------------

type pair struct{ user, client string }

// Outbox holds what the claims process has not relayed yet. Safe for
// concurrent use; adding never blocks and never touches Redis.
type Outbox struct {
	mu      sync.Mutex
	calls   map[pair][]string
	n       int
	signals []string
	dropped atomic.Int64
	failed  atomic.Int64
}

// NewOutbox is an empty outbox.
func NewOutbox() *Outbox { return &Outbox{calls: map[pair][]string{}} }

// Call queues one finished call for the correlator. Only a call that named a
// person counts there, and never the probe's.
func (o *Outbox) Call(c claimtel.Call) {
	if c.UserID == "" || c.Probe {
		return
	}
	client := c.ClientID
	if client == "" {
		client = claimtel.None
	}
	rec := strconv.FormatInt(c.At.UnixMilli(), 10) + sep + c.Function + sep + string(c.Outcome)
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.n >= relayBacklogCap {
		// ponytail: drops the newest past the cap (Redis out for ~10 min at
		// the design peak); a sign-in in that gap may read not-called. A
		// "relay truncated since" marker if that is ever seen.
		o.dropped.Add(1)
		return
	}
	k := pair{c.UserID, client}
	o.calls[k] = append(o.calls[k], rec)
	o.n++
}

// Signal queues a signal for the backend.
func (o *Outbox) Signal(s Signal) {
	b, err := json.Marshal(s)
	if err != nil {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if len(o.signals) >= relaySignalsKept {
		o.dropped.Add(1)
		return
	}
	o.signals = append(o.signals, string(b))
}

// Dropped is how many calls and signals were dropped at the cap.
func (o *Outbox) Dropped() int64 { return o.dropped.Load() }

// Failures is how many flushes failed since the last one that worked.
func (o *Outbox) Failures() int64 { return o.failed.Load() }

// Flush relays everything queued, and the recorder's counters. What fails to
// write is kept for the next flush (the recorder keeps its own).
func (o *Outbox) Flush(ctx context.Context, rdb RelayRedis, rec *claimtel.Recorder, now time.Time) error {
	o.mu.Lock()
	calls, signals := o.calls, o.signals
	o.calls, o.signals, o.n = map[pair][]string{}, nil, 0
	o.mu.Unlock()

	var errs []error
	for k, recs := range calls {
		if err := writeCalls(ctx, rdb, k, recs); err != nil {
			errs = append(errs, err)
			o.requeue(k, recs)
		}
	}
	if len(signals) > 0 {
		if err := writeList(ctx, rdb, signalsKey, signals, relaySignalsKept, time.Hour); err != nil {
			errs = append(errs, err)
			o.mu.Lock()
			o.signals = append(signals, o.signals...)
			if len(o.signals) > relaySignalsKept {
				o.signals = o.signals[len(o.signals)-relaySignalsKept:]
			}
			o.mu.Unlock()
		}
	}
	if rec != nil {
		errs = append(errs, rec.Flush(ctx, func(ctx context.Context, b claimtel.Batch) error {
			return writeTelemetry(ctx, rdb, b, now)
		}))
		if err := writeLast(ctx, rdb, rec); err != nil {
			errs = append(errs, err)
		}
	}
	err := errors.Join(errs...)
	if err != nil {
		o.failed.Add(1)
	} else {
		o.failed.Store(0)
	}
	return err
}

func (o *Outbox) requeue(k pair, recs []string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.n+len(recs) > relayBacklogCap {
		o.dropped.Add(int64(len(recs)))
		return
	}
	o.calls[k] = append(recs, o.calls[k]...)
	o.n += len(recs)
}

func writeCalls(ctx context.Context, rdb RelayRedis, k pair, recs []string) error {
	return writeList(ctx, rdb, callsKey(k.user, k.client), recs, relayCallsKept, relayCallsKeep)
}

func writeList(ctx context.Context, rdb RelayRedis, key string, vals []string, keep int64, ttl time.Duration) error {
	args := make([]any, len(vals))
	for i, v := range vals {
		args[i] = v
	}
	if err := rdb.RPush(ctx, key, args...).Err(); err != nil {
		return fmt.Errorf("relay %s: %w", key, err)
	}
	if err := rdb.LTrim(ctx, key, -keep, -1).Err(); err != nil {
		return fmt.Errorf("relay %s: %w", key, err)
	}
	return rdb.Expire(ctx, key, ttl).Err()
}

func countField(k claimtel.Key) string {
	return strings.Join([]string{strconv.FormatInt(k.Bucket.Unix(), 10), k.ProjectID, k.ClientID, k.Function, k.KeySlot, string(k.Outcome)}, sep)
}

// writeTelemetry adds one recorder batch into this minute's hash.
func writeTelemetry(ctx context.Context, rdb RelayRedis, b claimtel.Batch, now time.Time) error {
	m := now.Unix() / 60
	key, rkey := telKey(m), recentKey(m)
	recent := map[claimtel.Outcome]int64{}
	for k, c := range b.Counts {
		f := countField(k)
		if err := rdb.HIncrBy(ctx, key, "c"+sep+f, c.N).Err(); err != nil {
			return fmt.Errorf("relay telemetry: %w", err)
		}
		for i, n := range c.Hist {
			if n > 0 {
				if err := rdb.HIncrBy(ctx, key, "h"+strconv.Itoa(i)+sep+f, n).Err(); err != nil {
					return fmt.Errorf("relay telemetry: %w", err)
				}
			}
		}
		recent[k.Outcome] += c.N
	}
	var sets []any
	for k, v := range b.Clients {
		known := "0"
		if v.ClientKnown {
			known = "1"
		}
		sets = append(sets, "l"+sep+k.ClientID+sep+k.Function, strconv.FormatInt(v.At.UnixMilli(), 10)+sep+string(v.Outcome)+sep+known)
		if !v.ServedAt.IsZero() {
			sets = append(sets, "s"+sep+k.ClientID+sep+k.Function, strconv.FormatInt(v.ServedAt.UnixMilli(), 10))
		}
	}
	for _, fp := range b.Fingerprints {
		j, err := json.Marshal(fp)
		if err == nil {
			sets = append(sets, "f"+sep+fp.FP, string(j))
		}
	}
	if len(sets) > 0 {
		if err := rdb.HSet(ctx, key, sets...).Err(); err != nil {
			return fmt.Errorf("relay telemetry: %w", err)
		}
	}
	if err := rdb.Expire(ctx, key, telKeep).Err(); err != nil {
		return fmt.Errorf("relay telemetry: %w", err)
	}
	if err := rdb.SAdd(ctx, telMinutesKey, m).Err(); err != nil {
		return fmt.Errorf("relay telemetry: %w", err)
	}
	for o, n := range recent {
		if err := rdb.HIncrBy(ctx, rkey, string(o), n).Err(); err != nil {
			return fmt.Errorf("relay telemetry: %w", err)
		}
	}
	if len(recent) > 0 {
		return rdb.Expire(ctx, rkey, recentKeep).Err()
	}
	return nil
}

func writeLast(ctx context.Context, rdb RelayRedis, rec *claimtel.Recorder) error {
	var sets []any
	if t := rec.LastServed(); !t.IsZero() {
		sets = append(sets, "served", strconv.FormatInt(t.UnixNano(), 10))
	}
	if t := rec.LastCalled(); !t.IsZero() {
		sets = append(sets, "called", strconv.FormatInt(t.UnixNano(), 10))
	}
	if len(sets) == 0 {
		return nil
	}
	return rdb.HSet(ctx, telLastKey, sets...).Err()
}

// ---------------------------------------------------------------------------
// The backend's side.
// ---------------------------------------------------------------------------

// FoldTelemetry writes every closed minute the claims process relayed into
// Postgres through write, once: a minute is renamed to its fold key before it
// is read, so the claims process can never add to a minute being folded, and
// deleted only after write succeeds. A failed write leaves the fold key for
// the next pass (until its 2h TTL). Returns how many minutes were folded.
func FoldTelemetry(ctx context.Context, rdb RelayRedis, now time.Time, write func(context.Context, claimtel.Batch) error) (int, error) {
	members, err := rdb.SMembers(ctx, telMinutesKey).Result()
	if err != nil {
		return 0, fmt.Errorf("fold telemetry: %w", err)
	}
	closed := now.Unix()/60 - 1 // the current and previous minute may still be written
	folded := 0
	var errs []error
	for _, s := range members {
		m, perr := strconv.ParseInt(s, 10, 64)
		if perr != nil || m >= closed {
			if perr != nil {
				_ = rdb.SRem(ctx, telMinutesKey, s).Err()
			}
			continue
		}
		if err := rdb.Rename(ctx, telKey(m), foldKey(m)).Err(); err != nil && !isNoSuchKey(err) {
			errs = append(errs, err)
			continue
		}
		fields, err := rdb.HGetAll(ctx, foldKey(m)).Result()
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if len(fields) > 0 {
			if err := write(ctx, DecodeTelemetry(fields)); err != nil {
				errs = append(errs, err)
				continue
			}
			folded++
		}
		if err := rdb.Del(ctx, foldKey(m)).Err(); err != nil {
			errs = append(errs, err)
			continue
		}
		_ = rdb.SRem(ctx, telMinutesKey, s).Err()
	}
	return folded, errors.Join(errs...)
}

func isNoSuchKey(err error) bool { return err != nil && strings.Contains(err.Error(), "no such key") }

// DecodeTelemetry is a relayed minute as a recorder batch.
func DecodeTelemetry(fields map[string]string) claimtel.Batch {
	b := claimtel.Batch{Counts: map[claimtel.Key]claimtel.Count{}, Clients: map[claimtel.ClientKey]claimtel.ClientLast{}}
	parseKey := func(parts []string) (claimtel.Key, bool) {
		if len(parts) != 6 {
			return claimtel.Key{}, false
		}
		sec, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return claimtel.Key{}, false
		}
		return claimtel.Key{Bucket: time.Unix(sec, 0).UTC(), ProjectID: parts[1], ClientID: parts[2], Function: parts[3], KeySlot: parts[4], Outcome: claimtel.Outcome(parts[5])}, true
	}
	served := map[claimtel.ClientKey]time.Time{}
	for f, v := range fields {
		kind, rest, _ := strings.Cut(f, sep)
		n, _ := strconv.ParseInt(v, 10, 64)
		switch {
		case kind == "c":
			if k, ok := parseKey(strings.Split(rest, sep)); ok {
				c := b.Counts[k]
				c.N += n
				b.Counts[k] = c
			}
		case strings.HasPrefix(kind, "h"):
			i, err := strconv.Atoi(kind[1:])
			if k, ok := parseKey(strings.Split(rest, sep)); ok && err == nil && i >= 0 && i < claimtel.HistBuckets {
				c := b.Counts[k]
				c.Hist[i] += n
				b.Counts[k] = c
			}
		case kind == "l":
			client, fn, _ := strings.Cut(rest, sep)
			p := strings.Split(v, sep)
			if len(p) == 3 {
				ms, _ := strconv.ParseInt(p[0], 10, 64)
				cl := b.Clients[claimtel.ClientKey{ClientID: client, Function: fn}]
				cl.At, cl.Outcome, cl.ClientKnown = time.UnixMilli(ms), claimtel.Outcome(p[1]), p[2] == "1"
				b.Clients[claimtel.ClientKey{ClientID: client, Function: fn}] = cl
			}
		case kind == "s":
			client, fn, _ := strings.Cut(rest, sep)
			served[claimtel.ClientKey{ClientID: client, Function: fn}] = time.UnixMilli(n)
		case kind == "f":
			var fp claimtel.FingerprintSeen
			if json.Unmarshal([]byte(v), &fp) == nil {
				b.Fingerprints = append(b.Fingerprints, fp)
			}
		}
	}
	for k, t := range served {
		cl := b.Clients[k]
		cl.ServedAt = t
		b.Clients[k] = cl
	}
	return b
}

// RemoteRecent is the claims process's calls of the last minutes, by outcome.
func RemoteRecent(ctx context.Context, rdb RelayRedis, now time.Time, minutes int) (map[claimtel.Outcome]int64, error) {
	out := map[claimtel.Outcome]int64{}
	cur := now.Unix() / 60
	for m := cur - int64(minutes) + 1; m <= cur; m++ {
		fields, err := rdb.HGetAll(ctx, recentKey(m)).Result()
		if err != nil {
			return out, err
		}
		for o, v := range fields {
			n, _ := strconv.ParseInt(v, 10, 64)
			out[claimtel.Outcome(o)] += n
		}
	}
	return out, nil
}

// RemoteLast is when the claims process last served a call and last had one
// that named a person; zero when it has not.
func RemoteLast(ctx context.Context, rdb RelayRedis) (served, called time.Time, err error) {
	fields, err := rdb.HGetAll(ctx, telLastKey).Result()
	if err != nil {
		return
	}
	at := func(s string) time.Time {
		if n, err := strconv.ParseInt(s, 10, 64); err == nil && n > 0 {
			return time.Unix(0, n)
		}
		return time.Time{}
	}
	return at(fields["served"]), at(fields["called"]), nil
}

// RemoteWindow is claimtel.Recorder.Window over the calls the claims process
// relayed. A read that fails says overflowed, so the correlator leaves the
// sign-in unchecked rather than calling it not called.
// RemoteServed says whether the claims container recorded a served call for
// (person, client) through fn between from and to. It reads the raw records,
// not merged spans: a nearby call with a worse outcome (Syndra's own Zitadel
// login under the same account, seconds earlier) must not hide it.
func RemoteServed(ctx context.Context, rdb RelayRedis, userID, clientID, fn string, from, to time.Time) bool {
	if clientID == "" {
		clientID = claimtel.None
	}
	vals, err := rdb.LRange(ctx, callsKey(userID, clientID), 0, -1).Result()
	if err != nil {
		return false
	}
	for _, v := range vals {
		p := strings.Split(v, sep)
		if len(p) != 3 || p[1] != fn {
			continue
		}
		ms, err := strconv.ParseInt(p[0], 10, 64)
		if err != nil {
			continue
		}
		at := time.UnixMilli(ms)
		o := claimtel.Outcome(p[2])
		if !at.Before(from) && !at.After(to) && (o == claimtel.Served || o == claimtel.ServedLKG) {
			return true
		}
	}
	return false
}

func RemoteWindow(ctx context.Context, rdb RelayRedis, userID, clientID string, from, to time.Time) (map[string]claimtel.Outcome, bool) {
	if clientID == "" {
		clientID = claimtel.None
	}
	vals, err := rdb.LRange(ctx, callsKey(userID, clientID), 0, -1).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, true
	}
	calls := make([]claimtel.CallRecord, 0, len(vals))
	for _, v := range vals {
		p := strings.Split(v, sep)
		if len(p) != 3 {
			continue
		}
		ms, err := strconv.ParseInt(p[0], 10, 64)
		if err != nil {
			continue
		}
		calls = append(calls, claimtel.CallRecord{At: time.UnixMilli(ms), Function: p[1], Outcome: claimtel.Outcome(p[2])})
	}
	return claimtel.WindowOf(calls, len(vals) >= relayCallsKept, from, to)
}

// DrainSignals takes up to max queued signals.
func DrainSignals(ctx context.Context, rdb RelayRedis, max int) ([]Signal, error) {
	vals, err := rdb.LPopCount(ctx, signalsKey, max).Result()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := make([]Signal, 0, len(vals))
	for _, v := range vals {
		var s Signal
		if json.Unmarshal([]byte(v), &s) == nil {
			out = append(out, s)
		}
	}
	return out, nil
}
