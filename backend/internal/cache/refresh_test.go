package cache

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"syndra/internal/claimserve"
	"syndra/internal/db"
	"syndra/internal/models"
)

// factsStore is an in-memory Redis for the cache's seams: the facts hashes,
// the old per-project keys, and a count of every command.
type factsStore struct {
	mu     sync.Mutex
	hashes map[string]map[string]string
	kv     map[string]string
	ops    map[string]int
}

func (s *factsStore) count(op string) { s.mu.Lock(); s.ops[op]++; s.mu.Unlock() }

// fakeRedis swaps every Redis seam for an in-memory store.
func fakeRedis(t *testing.T) *factsStore {
	t.Helper()
	oKeys, oHGet, oWrite, oGet, oDel, oScan := redisHKeys, redisHGet, redisWriteFacts, redisGet, redisDel, redisScanKeys
	t.Cleanup(func() {
		redisHKeys, redisHGet, redisWriteFacts, redisGet, redisDel, redisScanKeys = oKeys, oHGet, oWrite, oGet, oDel, oScan
	})
	s := &factsStore{hashes: map[string]map[string]string{}, kv: map[string]string{}, ops: map[string]int{}}
	redisHKeys = func(_ context.Context, key string) ([]string, error) {
		s.count("HKEYS")
		s.mu.Lock()
		defer s.mu.Unlock()
		var out []string
		for f := range s.hashes[key] {
			out = append(out, f)
		}
		return out, nil
	}
	redisHGet = func(_ context.Context, key, field string) (string, error) {
		s.count("HGET")
		s.mu.Lock()
		defer s.mu.Unlock()
		v, ok := s.hashes[key][field]
		if !ok {
			return "", redis.Nil
		}
		return v, nil
	}
	redisWriteFacts = func(_ context.Context, key string, set map[string]string, drop, legacy []string, _ time.Duration) error {
		s.count("MULTI")
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.hashes[key] == nil {
			s.hashes[key] = map[string]string{}
		}
		for f, v := range set {
			s.hashes[key][f] = v
		}
		for _, f := range drop {
			delete(s.hashes[key], f)
		}
		for _, k := range legacy {
			delete(s.kv, k)
		}
		return nil
	}
	redisGet = func(_ context.Context, key string) (string, error) {
		s.count("GET")
		s.mu.Lock()
		defer s.mu.Unlock()
		v, ok := s.kv[key]
		if !ok {
			return "", redis.Nil
		}
		return v, nil
	}
	redisDel = func(_ context.Context, keys ...string) error {
		s.count("DEL")
		s.mu.Lock()
		defer s.mu.Unlock()
		for _, k := range keys {
			delete(s.kv, k)
			delete(s.hashes, k)
		}
		return nil
	}
	redisScanKeys = func(_ context.Context, pattern string) ([]string, error) {
		s.count("SCAN")
		s.mu.Lock()
		defer s.mu.Unlock()
		prefix := strings.TrimSuffix(pattern, "*")
		var keys []string
		for k := range s.kv {
			if strings.HasPrefix(k, prefix) {
				keys = append(keys, k)
			}
		}
		return keys, nil
	}
	return s
}

// roles is what the data plane would read for (user, project), nil when none.
func (s *factsStore) roles(t *testing.T, user, project string) []string {
	t.Helper()
	raw, ok := s.hashes[claimserve.FactsKey(user)][project]
	if !ok {
		return nil
	}
	var f struct{ Roles []string }
	if err := json.Unmarshal([]byte(raw), &f); err != nil {
		t.Fatalf("parse %s/%s: %v", user, project, err)
	}
	return f.Roles
}

func (s *factsStore) has(user, project string) bool {
	_, ok := s.hashes[claimserve.FactsKey(user)][project]
	return ok
}

func holding(grants ...models.DirectGrant) {
	dbGetDirectGrantsForUser = func(context.Context, string, bool) ([]models.DirectGrant, error) { return grants, nil }
	dbGetActiveMappingRules = func(context.Context) ([]models.MappingRule, error) { return nil, nil }
}

// The token path serves only what is stored, so a change has to leave the new
// claims in place, not a missing key. A grant in one project must also leave
// the user's other projects alone, and a revoke must take the role out of the
// stored claims while the rest stay.
func TestRefreshUser_RecompilesInsteadOfInvalidating(t *testing.T) {
	resetCacheDeps(t)
	s := fakeRedis(t)
	ctx := context.Background()

	holding(
		models.DirectGrant{ProjectID: "pA", RoleKey: "viewer"},
		models.DirectGrant{ProjectID: "pB", RoleKey: "editor"},
		models.DirectGrant{ProjectID: "pB", RoleKey: "admin"},
	)
	if err := refreshPerson(ctx, "u1"); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if got := s.roles(t, "u1", "pA"); len(got) != 1 {
		t.Fatalf("a held project must be compiled without being named: %v", got)
	}

	// A new grant in pC: naming only pC must not disturb pA or pB.
	holding(
		models.DirectGrant{ProjectID: "pA", RoleKey: "viewer"},
		models.DirectGrant{ProjectID: "pB", RoleKey: "editor"},
		models.DirectGrant{ProjectID: "pB", RoleKey: "admin"},
		models.DirectGrant{ProjectID: "pC", RoleKey: "member"},
	)
	if err := refreshPerson(ctx, "u1", "pC"); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	for p, want := range map[string]int{"pA": 1, "pB": 2, "pC": 1} {
		if got := s.roles(t, "u1", p); len(got) != want {
			t.Errorf("%s after a grant in pC: want %d roles, got %v", p, want, got)
		}
	}

	// Revoke admin in pB and the only role in pA.
	holding(
		models.DirectGrant{ProjectID: "pB", RoleKey: "editor"},
		models.DirectGrant{ProjectID: "pC", RoleKey: "member"},
	)
	if err := refreshPerson(ctx, "u1", "pB"); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if got := s.roles(t, "u1", "pB"); len(got) != 1 || got[0] != "editor" {
		t.Errorf("the revoked role must be gone and the other kept: %v", got)
	}
	if s.has("u1", "pA") {
		t.Errorf("a project the user holds nothing in must not keep serving its old roles")
	}
	if got := s.roles(t, "u1", "pC"); len(got) != 1 {
		t.Errorf("an untouched project must survive: %v", got)
	}
	if s.ops["SCAN"] != 1 {
		t.Errorf("a person on the hash layout must never SCAN: %d scans over three refreshes, want the one migration", s.ops["SCAN"])
	}
}

// A person still on the old layout moves in one refresh: their old keys are
// read as the projects they had, and deleted by the same write that creates
// the hash, so the data plane's fallback read can never find a project the
// hash has since dropped. Later refreshes never SCAN.
func TestRefreshUser_MovesTheOldLayoutOnce(t *testing.T) {
	resetCacheDeps(t)
	s := fakeRedis(t)
	ctx := context.Background()
	s.kv["mapping:u1:pOld"] = `{"roles":["revoked"]}`
	s.kv["mapping:u1:pA"] = `{"roles":["viewer"]}`
	s.kv["mapping:u2:pA"] = `{"roles":["admin"]}`
	holding(models.DirectGrant{ProjectID: "pA", RoleKey: "viewer"})

	if err := refreshPerson(ctx, "u1"); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.kv["mapping:u1:pOld"]; ok {
		t.Error("an old key survived the move: the fallback would serve its revoked role")
	}
	if _, ok := s.kv["mapping:u1:pA"]; ok {
		t.Error("an old key survived the move")
	}
	if _, ok := s.kv["mapping:u2:pA"]; !ok {
		t.Error("another person's old key was touched")
	}
	if s.has("u1", "pOld") || len(s.roles(t, "u1", "pA")) != 1 {
		t.Errorf("hash after the move: %v", s.hashes[claimserve.FactsKey("u1")])
	}
	if _, ok := s.hashes[claimserve.FactsKey("u1")][claimserve.MetaField]; !ok {
		t.Error("the hash does not say it is on the new layout")
	}
	f, ok, err := StoredFacts(ctx, "u2", "pA")
	if err != nil || !ok || len(f.Roles) != 1 {
		t.Errorf("an unmoved person reads from the old key until they move: %+v %v %v", f, ok, err)
	}

	_ = refreshPerson(ctx, "u1")
	if s.ops["SCAN"] != 1 {
		t.Errorf("SCANs = %d, want one, for the move only", s.ops["SCAN"])
	}
}

// A compile that fails must not leave the old claims in place: they may hold
// the role that was just revoked. The data plane's degraded path is the safe
// answer.
func TestRefreshUser_ACompileFailureDropsWhatItCouldNotRefresh(t *testing.T) {
	resetCacheDeps(t)
	s := fakeRedis(t)
	s.hashes[claimserve.FactsKey("u1")] = map[string]string{"pA": `{"roles":["admin"]}`, claimserve.MetaField: "x"}
	s.hashes[claimserve.FactsKey("u2")] = map[string]string{"pA": `{"roles":["admin"]}`, claimserve.MetaField: "x"}
	dbGetDirectGrantsForUser = func(context.Context, string, bool) ([]models.DirectGrant, error) {
		return nil, errors.New("db down")
	}

	if err := refreshPerson(context.Background(), "u1", "pA"); err == nil {
		t.Fatal("a failed compile must be reported")
	}
	if s.has("u1", "pA") {
		t.Error("the unrefreshable facts kept serving")
	}
	if !s.has("u2", "pA") {
		t.Error("another user's facts were touched")
	}
}

// The simulator must compute without writing: a preview that stored its own
// answer would repair the one project being looked at and hide a stale cache.
func TestCompileFacts_WritesNothing(t *testing.T) {
	resetCacheDeps(t)
	s := fakeRedis(t)
	holding(models.DirectGrant{ProjectID: "pA", RoleKey: "viewer"})

	facts, err := CompileFacts(context.Background(), "u1", "pA")
	if err != nil || len(facts.Roles) != 1 {
		t.Fatalf("facts=%+v err=%v", facts, err)
	}
	if len(s.ops) != 0 {
		t.Errorf("a preview touched the token cache: %v", s.ops)
	}
}

// warmDirectory makes the warm's one directory snapshot list these people.
func warmDirectory(t *testing.T, ids ...string) *int {
	t.Helper()
	orig := directorySnapshot
	t.Cleanup(func() { directorySnapshot = orig })
	reads := 0
	directorySnapshot = func(context.Context) ([]models.UserProfile, bool) {
		reads++
		out := make([]models.UserProfile, len(ids))
		for i, id := range ids {
			out[i] = models.UserProfile{ID: id}
		}
		return out, true
	}
	return &reads
}

// noPace runs warms without spreading them, for tests.
func noPace(t *testing.T) {
	orig := warmPace
	warmPace = 0
	t.Cleanup(func() { warmPace = orig })
}

// Warm covers everyone Syndra knows, so a restart does not leave the token
// path degraded for people whose access never changed.
func TestWarm_CompilesEveryKnownUser(t *testing.T) {
	resetCacheDeps(t)
	noPace(t)
	s := fakeRedis(t)
	orig := dbGetAllKnownUserIDs
	t.Cleanup(func() { dbGetAllKnownUserIDs = orig })
	dbGetAllKnownUserIDs = func(context.Context) ([]string, error) { return []string{"u1", "u2"}, nil }
	warmDirectory(t, "u1", "u2")
	holding(models.DirectGrant{ProjectID: "pA", RoleKey: "viewer"})

	if err := Warm(context.Background()); err != nil {
		t.Fatalf("warm: %v", err)
	}
	if len(s.roles(t, "u1", "pA")) != 1 || len(s.roles(t, "u2", "pA")) != 1 {
		t.Errorf("not everyone was warmed: %v", s.hashes)
	}
}

// A warm is background work: when it can't read what it needs it must leave
// the stored claims alone, not drop them (the token path would answer degraded
// for people whose access never changed).
func TestWarm_NeverDegradesGoodEntries(t *testing.T) {
	resetCacheDeps(t)
	noPace(t)
	s := fakeRedis(t)
	origIDs := dbGetAllKnownUserIDs
	t.Cleanup(func() { dbGetAllKnownUserIDs = origIDs })
	dbGetAllKnownUserIDs = func(context.Context) ([]string, error) { return []string{"u1"}, nil }
	holding(models.DirectGrant{ProjectID: "pA", RoleKey: "viewer"})
	const good = `{"roles":["admin"]}`
	key := claimserve.FactsKey("u1")
	s.hashes[key] = map[string]string{"pA": good, claimserve.MetaField: "x"}

	// The marks can't be read: skip the user, keep the facts.
	warmDirectory(t, "u1")
	dbListServiceAccounts = func(context.Context) ([]db.ServiceAccountRow, error) { return nil, errors.New("db down") }
	_ = Warm(context.Background())
	if s.hashes[key]["pA"] != good {
		t.Errorf("a failed profile read rewrote or dropped the entry: %q", s.hashes[key]["pA"])
	}

	// Compile fails: keep the facts (a change-driven refresh would drop them).
	dbListServiceAccounts = func(context.Context) ([]db.ServiceAccountRow, error) { return nil, nil }
	dbGetDirectGrantsForUser = func(context.Context, string, bool) ([]models.DirectGrant, error) {
		return nil, errors.New("db down")
	}
	_ = Warm(context.Background())
	if s.hashes[key]["pA"] != good {
		t.Errorf("a failed compile dropped the entry: %q", s.hashes[key]["pA"])
	}

	// Directory not answering: the pass is skipped outright.
	holding(models.DirectGrant{ProjectID: "pA", RoleKey: "viewer"})
	directorySnapshot = func(context.Context) ([]models.UserProfile, bool) { return nil, false }
	_ = Warm(context.Background())
	if s.hashes[key]["pA"] != good {
		t.Errorf("a pass with the directory down still wrote: %q", s.hashes[key]["pA"])
	}
}

// Two refreshes of one user must run one after the other: the one that
// started later has the newer facts and must store last. Here the older
// compile (still holding admin) is stuck mid-flight while a revoke's refresh
// arrives; unserialized, the old compile stores admin back over the revoke.
func TestRefreshUser_SerializedPerUser(t *testing.T) {
	resetCacheDeps(t)
	s := fakeRedis(t)
	entered, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	dbGetDirectGrantsForUser = func(context.Context, string, bool) ([]models.DirectGrant, error) {
		if calls.Add(1) == 1 {
			close(entered)
			<-release
			return []models.DirectGrant{{ProjectID: "pA", RoleKey: "admin"}}, nil
		}
		return nil, nil // the revoke has landed
	}
	dbGetActiveMappingRules = func(context.Context) ([]models.MappingRule, error) { return nil, nil }

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _ = refreshPerson(context.Background(), "u1") }()
	<-entered
	go func() { defer wg.Done(); _ = refreshPerson(context.Background(), "u1") }()
	time.Sleep(50 * time.Millisecond) // let the second reach the lock
	close(release)
	wg.Wait()

	if s.has("u1", "pA") {
		t.Errorf("the older compile overwrote the newer one: %v", s.hashes[claimserve.FactsKey("u1")])
	}
}

// A user id is not a pattern: * or [ in one must not reach other users' keys.
func TestRefreshUser_EscapesGlobCharsInTheScan(t *testing.T) {
	resetCacheDeps(t)
	fakeRedis(t)
	var pattern string
	redisScanKeys = func(_ context.Context, p string) ([]string, error) { pattern = p; return nil, nil }
	holding()

	_ = refreshPerson(context.Background(), `a*b[c]?\d`)
	if want := `mapping:a\*b\[c\]\?\\d:*`; pattern != want {
		t.Errorf("scan pattern = %q, want %q", pattern, want)
	}
}
