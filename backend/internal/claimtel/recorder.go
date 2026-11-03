package claimtel

import (
	"context"
	"encoding/json"
	"hash/maphash"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// BucketWidth is the stored row's width. At ~60 apps that is at most a few
	// thousand rows a day (assessment SC7).
	BucketWidth = 5 * time.Minute

	shards           = 16
	maxPairsPerShard = 4096 // 65,536 (person, client) pairs in all, under ~8MB
	pairKeep         = 20 * time.Minute
	maxRetained      = 20000 // counter rows kept across failed flushes
	maxFingerprints  = 256   // per process; a payload that varies more is not a version
	exampleCap       = 64 << 10
	recentMinutes    = 10
)

// boundsMS are the latency histogram's upper bounds; a 12th bucket is +Inf.
var boundsMS = [...]float64{1, 2, 5, 10, 25, 50, 100, 250, 500, 1000, 3000}

// HistBuckets is the histogram's length, the column's cardinality.
const HistBuckets = len(boundsMS) + 1

func histIndex(d time.Duration) int {
	ms := float64(d) / float64(time.Millisecond)
	for i, b := range boundsMS {
		if ms <= b {
			return i
		}
	}
	return len(boundsMS)
}

// UpperBoundMS is the histogram bucket bound at or under which q of the calls
// fell. Nil when there are no calls or the quantile is in the +Inf bucket.
func UpperBoundMS(hist []int64, q float64) *float64 {
	var total int64
	for _, n := range hist {
		total += n
	}
	if total == 0 {
		return nil
	}
	need := int64(q*float64(total) + 0.999999)
	var seen int64
	for i, n := range hist {
		seen += n
		if seen >= need {
			if i >= len(boundsMS) {
				return nil
			}
			b := boundsMS[i]
			return &b
		}
	}
	return nil
}

// Key is one counter row: a 5-minute bucket and the call's dimensions.
type Key struct {
	Bucket    time.Time
	ProjectID string
	ClientID  string
	Function  string
	KeySlot   string
	Outcome   Outcome
}

// Count is a row's calls and their latency histogram.
type Count struct {
	N    int64
	Hist [HistBuckets]int64
}

func (c *Count) add(o Count) {
	c.N += o.N
	for i := range c.Hist {
		c.Hist[i] += o.Hist[i]
	}
}

// ClientKey and ClientLast are claim_delivery_last: when an app last reached
// Syndra through each function, and when it was last served.
type ClientKey struct{ ClientID, Function string }

type ClientLast struct {
	At          time.Time
	Outcome     Outcome
	ServedAt    time.Time // zero: not served since the last flush
	ClientKnown bool
}

func mergeClient(m map[ClientKey]ClientLast, k ClientKey, v ClientLast) {
	cur, ok := m[k]
	if !ok {
		m[k] = v
		return
	}
	if v.At.After(cur.At) {
		cur.At, cur.Outcome, cur.ClientKnown = v.At, v.Outcome, v.ClientKnown
	}
	if v.ServedAt.After(cur.ServedAt) {
		cur.ServedAt = v.ServedAt
	}
	m[k] = cur
}

// LastCall is the newest call for one (person, client): what the sign-in
// correlator (S3) matches a sign-in event against.
type LastCall struct {
	At       time.Time
	Function string
	Outcome  Outcome
	MS       float64
	Gen      int64 // the snapshot that answered: each answer is reproducible
}

type pairKey struct{ user, client string }

// The two Actions v2 functions, as Zitadel names them on the wire.
const (
	FnPreAccessToken = "function/preaccesstoken"
	FnPreUserinfo    = "function/preuserinfo"
)

// span is a run of calls through one function for one (person, client), at
// most spanMax from first to last, keeping the worst outcome inside it: what
// the sign-in correlator matches an event against. Several calls per sign-in
// (an access token, the ID token, a userinfo read) fold into one span.
type span struct {
	from, to int64 // unix ms
	fn       uint8 // fnCode
	worst    uint8 // index into Outcomes
}

const (
	spansPerPair = 8
	spanMax      = 2 * time.Minute
)

func fnCode(f string) uint8 {
	switch f {
	case FnPreAccessToken:
		return 1
	case FnPreUserinfo:
		return 2
	}
	return 0
}

// pairState is one (person, client): its newest call, and a ring of spans.
// About 220 bytes, so the 65,536-pair cap is under 15MB at worst.
type pairState struct {
	last  LastCall
	spans [spansPerPair]span
	n     uint8 // spans in use
	head  uint8 // the newest span
}

func (p *pairState) add(at time.Time, fn string, o Outcome) {
	ms, code, idx := at.UnixMilli(), fnCode(fn), uint8(outcomeIndex[o])
	// The newest span of this function takes the call while it is young enough.
	for i := uint8(0); i < p.n; i++ {
		s := &p.spans[(p.head+spansPerPair-i)%spansPerPair]
		if s.fn != code {
			continue
		}
		if ms-s.from <= spanMax.Milliseconds() && ms >= s.from {
			if ms > s.to {
				s.to = ms
			}
			if Worse(Outcomes[idx], Outcomes[s.worst]) {
				s.worst = idx
			}
			return
		}
		break
	}
	if p.n > 0 {
		p.head = (p.head + 1) % spansPerPair
	}
	if p.n < spansPerPair {
		p.n++
	}
	p.spans[p.head] = span{from: ms, to: ms, fn: code, worst: idx}
}

// Rank orders outcomes from best to worst for one sign-in: served, then a
// shape that by design does not land in this function, then answers that
// still carried the person's claims, then no claims for a reason that is the
// app's setting, then the rest by severity.
func Rank(o Outcome) int {
	switch o {
	case Served:
		return 0
	case NotLanding:
		return 1
	case ServedLKG, ServedWelcome, UnknownFunction:
		return 2
	case NoGrants:
		return 3
	}
	return 4 + int(o.Severity())
}

// Worse reports whether a is a worse outcome than b.
func Worse(a, b Outcome) bool { return Rank(a) > Rank(b) }

// Window is what one (person, client) called between from and to: the worst
// outcome per function among the spans that overlap it. Overflowed is true
// when the ring is full and its oldest span starts after from, so a call
// inside the window may have been pushed out: the caller cannot say "not
// called".
func (r *Recorder) Window(userID, clientID string, from, to time.Time) (worst map[string]Outcome, overflowed bool) {
	s := &r.shards[maphash.String(r.seed, userID+"\x00"+clientID)%shards]
	s.mu.Lock()
	defer s.mu.Unlock()
	ps := s.pairs[pairKey{userID, orNone(clientID)}]
	if ps == nil {
		return nil, false
	}
	return ps.window(from, to)
}

func (ps *pairState) window(from, to time.Time) (worst map[string]Outcome, overflowed bool) {
	f, t := from.UnixMilli(), to.UnixMilli()
	names := [...]string{"", FnPreAccessToken, FnPreUserinfo}
	worst = map[string]Outcome{}
	oldest := int64(0)
	for i := uint8(0); i < ps.n; i++ {
		sp := ps.spans[(ps.head+spansPerPair-i)%spansPerPair]
		oldest = sp.from
		if sp.to < f || sp.from > t {
			continue
		}
		fn, o := names[sp.fn], Outcomes[sp.worst]
		if cur, ok := worst[fn]; !ok || Worse(o, cur) {
			worst[fn] = o
		}
	}
	return worst, ps.n == spansPerPair && oldest > f
}

// CallRecord is one call as the claims process relays it to the backend's
// correlator (S7): through which function, when, and what it came to.
type CallRecord struct {
	At       time.Time
	Function string
	Outcome  Outcome
}

// WindowOf is Window over relayed calls, oldest first, folded into spans by
// the same rules the recorder uses. truncated says older calls were dropped
// from the relay, so a window reaching before the first one cannot say "not
// called".
func WindowOf(calls []CallRecord, truncated bool, from, to time.Time) (map[string]Outcome, bool) {
	if len(calls) == 0 {
		return nil, truncated
	}
	var ps pairState
	for _, c := range calls {
		ps.add(c.At, c.Function, c.Outcome)
	}
	worst, overflowed := ps.window(from, to)
	return worst, overflowed || (truncated && calls[0].At.After(from))
}

// FingerprintSeen is a payload shape this process had not seen.
type FingerprintSeen struct {
	FP       string
	Function string
	Paths    []string
	Example  json.RawMessage // redacted; nil for a body over 64KiB
	At       time.Time
}

// Batch is what one flush writes.
type Batch struct {
	Counts       map[Key]Count
	Clients      map[ClientKey]ClientLast
	Fingerprints []FingerprintSeen
}

func (b Batch) empty() bool {
	return len(b.Counts) == 0 && len(b.Clients) == 0 && len(b.Fingerprints) == 0
}

// FlushStatus is the flush's health: a failing flush keeps its counters and
// says so here, which /healthz/claims reads.
type FlushStatus struct {
	LastAt   time.Time `json:"last_at"`
	LastErr  string    `json:"last_error,omitempty"`
	Failures int       `json:"consecutive_failures"`
	Retained int       `json:"retained_rows"`
	Dropped  int64     `json:"dropped_rows"`
}

type shard struct {
	mu      sync.Mutex
	counts  map[Key]*Count
	clients map[ClientKey]ClientLast
	pairs   map[pairKey]*pairState
}

type minuteTally struct {
	minute atomic.Int64
	n      [len(Outcomes)]atomic.Int64
}

// Recorder holds the counters. A call costs one map write under one of 16
// mutexes, chosen by person and client, so concurrent sign-ins rarely meet.
type Recorder struct {
	now    func() time.Time
	seed   maphash.Seed
	shards [shards]shard

	recent     [recentMinutes]minuteTally
	probes     [len(Outcomes)]atomic.Int64
	lastServed atomic.Int64 // unix nanoseconds
	lastCalled atomic.Int64 // unix nanoseconds of the last call that named a person

	fps     sync.Map // fingerprint -> struct{}
	fpCount atomic.Int64
	fpMu    sync.Mutex
	newFPs  []FingerprintSeen

	flushMu         sync.Mutex // one flush at a time; guards the fields below
	retained        map[Key]Count
	retainedClients map[ClientKey]ClientLast
	status          FlushStatus
	statusMu        sync.Mutex
}

// New is an empty recorder.
func New(now func() time.Time) *Recorder {
	r := &Recorder{now: now, seed: maphash.MakeSeed()}
	for i := range r.shards {
		r.shards[i] = shard{counts: map[Key]*Count{}, clients: map[ClientKey]ClientLast{}, pairs: map[pairKey]*pairState{}}
	}
	return r
}

func orNone(s string) string {
	if s == "" {
		return None
	}
	return s
}

// Record counts one finished call. An outcome outside the classes counts as
// bad_request, so the classes stay exhaustive whatever a caller sets.
func (r *Recorder) Record(c Call) {
	if !Valid(c.Outcome) {
		c.Outcome = BadRequest
	}
	if c.At.IsZero() {
		c.At = r.now()
	}
	if c.Probe {
		r.probes[outcomeIndex[c.Outcome]].Add(1)
		return
	}
	key := Key{
		Bucket: c.At.UTC().Truncate(BucketWidth), ProjectID: orNone(c.ProjectID), ClientID: orNone(c.ClientID),
		Function: orNone(c.Function), KeySlot: orNone(c.KeySlot), Outcome: c.Outcome,
	}
	ms := float64(c.Elapsed.Microseconds()) / 1000

	s := &r.shards[maphash.String(r.seed, c.UserID+"\x00"+c.ClientID)%shards]
	s.mu.Lock()
	cnt := s.counts[key]
	if cnt == nil {
		cnt = &Count{}
		s.counts[key] = cnt
	}
	cnt.N++
	cnt.Hist[histIndex(c.Elapsed)]++
	// Only a call the handler read (it set the function) names an app; a
	// refused signature carries nothing anyone vouched for.
	if c.Function != "" {
		cl := ClientLast{At: c.At, Outcome: c.Outcome, ClientKnown: c.ClientKnown}
		if c.Outcome == Served {
			cl.ServedAt = c.At
		}
		mergeClient(s.clients, ClientKey{key.ClientID, key.Function}, cl)
	}
	if c.UserID != "" {
		pk := pairKey{c.UserID, key.ClientID}
		ps := s.pairs[pk]
		if ps == nil {
			if len(s.pairs) >= maxPairsPerShard {
				// ponytail: evicts an arbitrary pair when full; an LRU if the
				// correlator ever reports pairs missing under a real burst.
				for k := range s.pairs {
					delete(s.pairs, k)
					break
				}
			}
			ps = &pairState{}
			s.pairs[pk] = ps
		}
		ps.last = LastCall{At: c.At, Function: key.Function, Outcome: c.Outcome, MS: ms, Gen: c.Gen}
		ps.add(c.At, c.Function, c.Outcome)
	}
	s.mu.Unlock()
	if c.UserID != "" {
		r.lastCalled.Store(c.At.UnixNano())
	}

	r.tally(c.At, c.Outcome)
	if c.Outcome == Served {
		r.lastServed.Store(c.At.UnixNano())
	}
	if len(c.Body) > 0 {
		r.fingerprint(c)
	}
}

// tally counts the outcome in its minute of the recent ring. A writer racing a
// minute's rollover can lose its increment; this ring only feeds the health
// summary, where one call in a minute is not the difference.
func (r *Recorder) tally(at time.Time, o Outcome) {
	m := at.Unix() / 60
	t := &r.recent[m%recentMinutes]
	if old := t.minute.Load(); old != m && t.minute.CompareAndSwap(old, m) {
		for i := range t.n {
			t.n[i].Store(0)
		}
	}
	t.n[outcomeIndex[o]].Add(1)
}

func (r *Recorder) fingerprint(c Call) {
	paths := Paths(c.Body)
	if paths == nil {
		return
	}
	fp := Fingerprint(paths)
	if _, seen := r.fps.Load(fp); seen || r.fpCount.Load() >= maxFingerprints {
		return
	}
	if _, loaded := r.fps.LoadOrStore(fp, struct{}{}); loaded {
		return
	}
	r.fpCount.Add(1)
	seen := FingerprintSeen{FP: fp, Function: orNone(c.Function), Paths: paths, At: c.At}
	if len(c.Body) <= exampleCap {
		seen.Example = Redacted(c.Body)
	}
	log.Printf("[DATA PLANE] payload shape %s seen for the first time by this process (%d key paths)", fp, len(paths))
	r.fpMu.Lock()
	r.newFPs = append(r.newFPs, seen)
	r.fpMu.Unlock()
}

// Recent is the calls of the last `minutes` minutes (at most 10), by outcome.
func (r *Recorder) Recent(minutes int) map[Outcome]int64 {
	if minutes > recentMinutes {
		minutes = recentMinutes
	}
	now := r.now().Unix() / 60
	out := map[Outcome]int64{}
	for i := range r.recent {
		t := &r.recent[i]
		if m := t.minute.Load(); m > now-int64(minutes) && m <= now {
			for j, o := range Outcomes {
				if n := t.n[j].Load(); n > 0 {
					out[o] += n
				}
			}
		}
	}
	return out
}

// Probes is the L1 canary's calls by outcome since this process started:
// labelled probe, kept out of every SLI.
func (r *Recorder) Probes() map[Outcome]int64 {
	out := map[Outcome]int64{}
	for i, o := range Outcomes {
		if n := r.probes[i].Load(); n > 0 {
			out[o] = n
		}
	}
	return out
}

// LastServed is when a call was last served, or zero.
func (r *Recorder) LastServed() time.Time {
	if n := r.lastServed.Load(); n != 0 {
		return time.Unix(0, n)
	}
	return time.Time{}
}

// ActivePairs is how many (person, client) pairs called in the last 20 minutes,
// give or take the minute since the last prune.
func (r *Recorder) ActivePairs() int {
	n := 0
	for i := range r.shards {
		s := &r.shards[i]
		s.mu.Lock()
		n += len(s.pairs)
		s.mu.Unlock()
	}
	return n
}

// LastCall is the newest call for one (person, client).
func (r *Recorder) LastCall(userID, clientID string) (LastCall, bool) {
	s := &r.shards[maphash.String(r.seed, userID+"\x00"+clientID)%shards]
	s.mu.Lock()
	defer s.mu.Unlock()
	ps, ok := s.pairs[pairKey{userID, orNone(clientID)}]
	if !ok {
		return LastCall{}, false
	}
	return ps.last, true
}

// LastCalled is when a call that named a person last arrived, or zero.
func (r *Recorder) LastCalled() time.Time {
	if n := r.lastCalled.Load(); n != 0 {
		return time.Unix(0, n)
	}
	return time.Time{}
}

// Status is the flush's health.
func (r *Recorder) Status() FlushStatus {
	r.statusMu.Lock()
	defer r.statusMu.Unlock()
	return r.status
}

// Flush drains the counters into write. A failed write keeps them, merged
// into the next flush and capped at maxRetained rows, so a Postgres outage
// costs detail, never memory without bound. It also prunes pairs older than
// 20 minutes.
func (r *Recorder) Flush(ctx context.Context, write func(context.Context, Batch) error) error {
	r.flushMu.Lock()
	defer r.flushMu.Unlock()

	b := Batch{Counts: r.retained, Clients: r.retainedClients}
	if b.Counts == nil {
		b.Counts = map[Key]Count{}
	}
	if b.Clients == nil {
		b.Clients = map[ClientKey]ClientLast{}
	}
	cut := r.now().Add(-pairKeep)
	for i := range r.shards {
		s := &r.shards[i]
		s.mu.Lock()
		for k, c := range s.counts {
			cur := b.Counts[k]
			cur.add(*c)
			b.Counts[k] = cur
		}
		for k, v := range s.clients {
			mergeClient(b.Clients, k, v)
		}
		s.counts, s.clients = map[Key]*Count{}, map[ClientKey]ClientLast{}
		for k, ps := range s.pairs {
			if ps.last.At.Before(cut) {
				delete(s.pairs, k)
			}
		}
		s.mu.Unlock()
	}
	r.fpMu.Lock()
	b.Fingerprints, r.newFPs = r.newFPs, nil
	r.fpMu.Unlock()

	if b.empty() {
		r.retained, r.retainedClients = nil, nil
		r.setStatus(func(s *FlushStatus) { s.LastAt, s.LastErr, s.Failures, s.Retained = r.now(), "", 0, 0 })
		return nil
	}
	if err := write(ctx, b); err != nil {
		var dropped int64
		for k := range b.Counts {
			if len(b.Counts) <= maxRetained {
				break
			}
			// ponytail: drops arbitrary rows past the cap; oldest-first only if
			// an outage that long is ever seen.
			dropped += b.Counts[k].N
			delete(b.Counts, k)
		}
		r.retained, r.retainedClients = b.Counts, b.Clients
		r.fpMu.Lock()
		r.newFPs = append(b.Fingerprints, r.newFPs...)
		r.fpMu.Unlock()
		r.setStatus(func(s *FlushStatus) {
			s.LastErr, s.Retained = err.Error(), len(b.Counts)
			s.Failures++
			s.Dropped += dropped
		})
		return err
	}
	r.retained, r.retainedClients = nil, nil
	r.setStatus(func(s *FlushStatus) { s.LastAt, s.LastErr, s.Failures, s.Retained = r.now(), "", 0, 0 })
	return nil
}

func (r *Recorder) setStatus(f func(*FlushStatus)) {
	r.statusMu.Lock()
	f(&r.status)
	r.statusMu.Unlock()
}
