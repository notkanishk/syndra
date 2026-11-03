package claimserve

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"

	"syndra/internal/claims"
	"syndra/internal/claimtel"
)

// Redis is the part of go-redis this package uses; *redis.Client is one.
type Redis interface {
	Get(ctx context.Context, key string) *redis.StringCmd
	HGet(ctx context.Context, key, field string) *redis.StringCmd
	SMembers(ctx context.Context, key string) *redis.StringSliceCmd
	MGet(ctx context.Context, keys ...string) *redis.SliceCmd
	MSet(ctx context.Context, values ...any) *redis.StatusCmd
	SAdd(ctx context.Context, key string, members ...any) *redis.IntCmd
	Incr(ctx context.Context, key string) *redis.IntCmd
}

// Budgets (assessment §4e).
const (
	// FactsTimeout bounds the one Redis read a call makes. Real: the client
	// honours context deadlines (db.NewRedisClient, F-1).
	FactsTimeout = 50 * time.Millisecond
	// PollEvery is how often the loader asks whether config moved.
	PollEvery   = time.Second
	pollTimeout = 200 * time.Millisecond
	// ReloadTimeout bounds one full config reload; on breach the previous
	// snapshot keeps answering.
	ReloadTimeout = 2 * time.Second
	// ReloadAtLeast reloads a snapshot this old even when claims:gen has not
	// moved: a write whose INCR failed (or a hand edit in Redis) still lands
	// within a minute. A reload is about 200KB and a millisecond.
	ReloadAtLeast = time.Minute
	// LKGCap bounds the last-known-good copy: everyone active in about a day.
	LKGCap = 20000
)

// Plane answers calls. Safe for concurrent use.
type Plane struct {
	rdb Redis
	cfg atomic.Pointer[Config]
	lkg *lru
	now func() time.Time
	// Shaper is claims.ShapeAt, the only shaper; a field so a test can make it
	// panic and prove the route survives.
	Shaper func(claims.Profile, claims.Facts, time.Time) *claims.Object

	lastCheck atomic.Int64 // unix nanos of the last poll Redis answered
	failures  atomic.Int64 // polls or reloads that failed since the last success
	reloads   atomic.Int64
	lastErr   atomic.Pointer[string]
}

// New is a plane with no config yet: every call answers degraded until the
// first Poll succeeds.
func New(rdb Redis, now func() time.Time) *Plane {
	if now == nil {
		now = time.Now
	}
	return &Plane{rdb: rdb, lkg: newLRU(LKGCap), now: now, Shaper: claims.ShapeAt}
}

// Config is the snapshot answering now, nil before the first load.
func (p *Plane) Config() *Config { return p.cfg.Load() }

// Result is one call's answer.
type Result struct {
	Claims      []Claim
	Outcome     claimtel.Outcome
	ClientKnown bool
	// Miss is Redis saying this person has no facts for the project: a repair
	// signal. Redis erroring is not one (a compile cannot help it).
	Miss bool
	Gen  int64
}

// Answer is the claims for (user, project) as the client's shape says, and
// what the call came to. Every branch is a counted outcome; none blocks past
// FactsTimeout or ctx.
func (p *Plane) Answer(ctx context.Context, userID, projectID, clientID, function string) Result {
	cfg := p.cfg.Load()
	if cfg == nil {
		return Result{Claims: []Claim{}, Outcome: claimtel.DegradedShapeUnreadable}
	}
	proj, ok := cfg.Projects[projectID]
	if !ok {
		// Never published: a guessed shape could show a role the app's own
		// shape hides, so the answer is fail closed. The write side publishes
		// every project it knows each minute.
		return Result{Claims: []Claim{}, Outcome: claimtel.DegradedShapeUnreadable, Gen: cfg.Gen}
	}
	shape, known := proj.Shapes.For(clientID)
	res := Result{ClientKnown: known, Gen: cfg.Gen}
	if !shape.Profile.LandsIn(function) {
		res.Claims, res.Outcome = []Claim{}, claimtel.NotLanding
		return res
	}

	facts, outcome, miss := p.facts(ctx, userID, projectID, proj.Mode.LKGMaxAge())
	res.Miss = miss
	switch outcome {
	case claimtel.Served, claimtel.ServedLKG:
		res.Claims, res.Outcome = Envelope(p.Shaper(shape.Profile, facts, p.now())), outcome
		if outcome == claimtel.Served && function != claims.FunctionPreAccessToken && function != claims.FunctionPreUserinfo {
			res.Outcome = claimtel.UnknownFunction // answered as both (F-5), and counted
		}
	default:
		res.Claims, res.Outcome = proj.Mode.Claims(), outcome
		if ctx.Err() != nil {
			res.Outcome = claimtel.DeadlineExceeded
		}
	}
	return res
}

// facts reads (user, project): served from Redis, else the last-known-good
// copy within maxAge when Redis is erroring, else a degraded outcome.
func (p *Plane) facts(ctx context.Context, userID, projectID string, maxAge time.Duration) (claims.Facts, claimtel.Outcome, bool) {
	rctx, cancel := context.WithTimeout(ctx, FactsTimeout)
	defer cancel()
	raw, err := p.rdb.HGet(rctx, FactsKey(userID), projectID).Result()
	if errors.Is(err, redis.Nil) {
		raw, err = p.rdb.Get(rctx, LegacyFactsKey(userID, projectID)).Result()
	}
	switch {
	case err == nil:
		var f claims.Facts
		if json.Unmarshal([]byte(raw), &f) != nil {
			return claims.Facts{}, claimtel.DegradedMalformed, false
		}
		if f.ProjectID == "" {
			f.ProjectID = projectID
		}
		if f.UserID == "" {
			f.UserID = userID
		}
		p.lkg.put(userID+"\x00"+projectID, f, p.now())
		return f, claimtel.Served, false
	case errors.Is(err, redis.Nil):
		// Redis says there are none: a copy from before would be a revoked
		// role served from memory.
		p.lkg.drop(userID + "\x00" + projectID)
		return claims.Facts{}, claimtel.DegradedNoFacts, true
	default:
		if f, at, ok := p.lkg.get(userID + "\x00" + projectID); ok && p.now().Sub(at) <= maxAge {
			return f, claimtel.ServedLKG, false
		}
		return claims.Facts{}, claimtel.DegradedNoFacts, false
	}
}

// Run polls until ctx ends.
func (p *Plane) Run(ctx context.Context) {
	t := time.NewTicker(PollEvery)
	defer t.Stop()
	for {
		_ = p.Poll(ctx)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// Poll reloads the snapshot when claims:gen has moved (or there is none). On
// any failure the previous snapshot keeps answering.
func (p *Plane) Poll(ctx context.Context) error {
	gctx, cancel := context.WithTimeout(ctx, pollTimeout)
	gen, err := p.rdb.Get(gctx, GenKey).Int64()
	cancel()
	if errors.Is(err, redis.Nil) {
		gen, err = 0, nil
	}
	if err != nil {
		return p.fail(fmt.Errorf("read %s: %w", GenKey, err))
	}
	if cur := p.cfg.Load(); cur != nil && cur.Gen == gen && p.now().Sub(cur.BuiltAt) < ReloadAtLeast {
		p.ok()
		return nil
	}
	lctx, cancel := context.WithTimeout(ctx, ReloadTimeout)
	defer cancel()
	cfg, err := p.load(lctx, gen)
	if err != nil {
		return p.fail(err)
	}
	p.cfg.Store(cfg)
	p.reloads.Add(1)
	p.ok()
	return nil
}

func (p *Plane) ok() {
	p.lastCheck.Store(p.now().UnixNano())
	p.failures.Store(0)
}

func (p *Plane) fail(err error) error {
	if p.failures.Add(1) == 1 {
		log.Printf("[DATA PLANE] config snapshot not refreshed, the previous one keeps answering: %v", err)
	}
	s := err.Error()
	p.lastErr.Store(&s)
	return err
}

// load reads every published project. gen is read before the load, so a
// write that lands during it moves gen again and the next poll reloads.
// A project whose entry can't be read is left out (it answers fail closed)
// rather than failing the whole snapshot.
func (p *Plane) load(ctx context.Context, gen int64) (*Config, error) {
	ids, err := p.rdb.SMembers(ctx, ProjectsKey).Result()
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", ProjectsKey, err)
	}
	cfg := &Config{Gen: gen, BuiltAt: p.now(), Projects: make(map[string]Project, len(ids)), ClientProject: map[string]string{}}
	kr, err := p.rdb.Get(ctx, KeyringKey).Result()
	switch {
	case errors.Is(err, redis.Nil):
	case err != nil:
		return nil, fmt.Errorf("read %s: %w", KeyringKey, err)
	case json.Unmarshal([]byte(kr), &cfg.Keys) != nil:
		// The env key still verifies; the write side rewrites this within a minute.
		cfg.Keys = Keyring{}
		log.Printf("[DATA PLANE] the signing keyring in Redis is unreadable; verifying with the env key alone")
	}
	if len(ids) == 0 {
		return cfg, nil
	}
	keys := make([]string, 0, 2*len(ids))
	for _, id := range ids {
		keys = append(keys, ShapesKey(id), ModeKey(id))
	}
	vals, err := p.rdb.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	for i, id := range ids {
		var proj Project
		s, _ := vals[2*i].(string)
		if s == "" || json.Unmarshal([]byte(s), &proj.Shapes) != nil || proj.Shapes.Default.Lands == "" {
			log.Printf("[DATA PLANE] project %s: shapes unreadable in Redis, answering fail closed until the next write", id)
			continue
		}
		if m, _ := vals[2*i+1].(string); m != "" {
			if json.Unmarshal([]byte(m), &proj.Mode) != nil {
				proj.Mode = Mode{} // fail closed, never a guessed minimal set
			}
		}
		cfg.Projects[id] = proj
		for client := range proj.Shapes.ByClient {
			cfg.ClientProject[client] = id
		}
	}
	return cfg, nil
}

// SigningKeys is the keyring's live keys, current first; none before the
// first load or before the write side has read Zitadel.
func (p *Plane) SigningKeys() []SlotKey {
	cfg := p.cfg.Load()
	if cfg == nil {
		return nil
	}
	return cfg.Keys.Keys(p.now())
}

// PublishKeyring writes the keyring through and moves claims:gen, so every
// data plane picks it up within a poll.
func PublishKeyring(ctx context.Context, rdb Redis, k Keyring) error {
	v, err := encode(k)
	if err != nil {
		return err
	}
	if err := rdb.MSet(ctx, KeyringKey, v).Err(); err != nil {
		return fmt.Errorf("publish keyring: %w", err)
	}
	if err := rdb.Incr(ctx, GenKey).Err(); err != nil {
		return fmt.Errorf("publish keyring: gen: %w", err)
	}
	return nil
}

// Status is the loader's health, for /healthz/claims.
type Status struct {
	Loaded   bool   `json:"loaded"`
	Gen      int64  `json:"gen"`
	Projects int    `json:"projects"`
	BuiltAgo *int64 `json:"built_age_s"`
	// CheckedAgo is how long since Redis last confirmed the snapshot is
	// current: the staleness that matters.
	CheckedAgo *int64 `json:"checked_age_s"`
	Failures   int64  `json:"failures"`
	LastError  string `json:"last_error,omitempty"`
	Reloads    int64  `json:"reloads"`
	LKG        int    `json:"lkg_entries"`
}

// Status reports the snapshot and the loader.
func (p *Plane) Status() Status {
	now := p.now()
	st := Status{Failures: p.failures.Load(), Reloads: p.reloads.Load(), LKG: p.lkg.len()}
	if cfg := p.cfg.Load(); cfg != nil {
		st.Loaded, st.Gen, st.Projects = true, cfg.Gen, len(cfg.Projects)
		s := int64(now.Sub(cfg.BuiltAt).Seconds())
		st.BuiltAgo = &s
	}
	if n := p.lastCheck.Load(); n != 0 {
		s := int64(now.Sub(time.Unix(0, n)).Seconds())
		st.CheckedAgo = &s
	}
	if e := p.lastErr.Load(); e != nil && st.Failures > 0 {
		st.LastError = *e
	}
	return st
}

// Publish writes projects' config through and moves claims:gen, but only for
// what changed: a pass that republishes everything as it already is leaves the
// data plane's snapshot alone. Values never expire. The gen moves last, so a
// reader that sees the new gen sees the new values.
//
// Every shape save, restore, seed and failure-mode write goes through here.
func Publish(ctx context.Context, rdb Redis, projects map[string]Project) (changed int, err error) {
	if len(projects) == 0 {
		return 0, nil
	}
	ids := make([]string, 0, len(projects))
	keys := make([]string, 0, 2*len(projects))
	for id := range projects {
		ids = append(ids, id)
		keys = append(keys, ShapesKey(id), ModeKey(id))
	}
	cur, err := rdb.MGet(ctx, keys...).Result()
	if err != nil {
		return 0, fmt.Errorf("publish claims config: read: %w", err)
	}
	var sets []any
	var members []any
	for i, id := range ids {
		shapes, err := encode(projects[id].Shapes)
		if err != nil {
			return 0, err
		}
		mode, err := encode(projects[id].Mode)
		if err != nil {
			return 0, err
		}
		s, _ := cur[2*i].(string)
		m, _ := cur[2*i+1].(string)
		if s == shapes && m == mode {
			continue
		}
		sets = append(sets, ShapesKey(id), shapes, ModeKey(id), mode)
		members = append(members, id)
	}
	if len(members) == 0 {
		return 0, nil
	}
	// MSET drops any TTL the old read-through cache left on these keys.
	if err := rdb.MSet(ctx, sets...).Err(); err != nil {
		return 0, fmt.Errorf("publish claims config: write: %w", err)
	}
	if err := rdb.SAdd(ctx, ProjectsKey, members...).Err(); err != nil {
		return 0, fmt.Errorf("publish claims config: index: %w", err)
	}
	if err := rdb.Incr(ctx, GenKey).Err(); err != nil {
		return 0, fmt.Errorf("publish claims config: gen: %w", err)
	}
	return len(members), nil
}
