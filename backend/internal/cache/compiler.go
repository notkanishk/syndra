package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"

	"syndra/internal/claims"
	"syndra/internal/claimserve"
	"syndra/internal/models"
	"syndra/internal/zitadel"
)

// cacheTTL is a backstop, not the freshness mechanism. Events, the expiry
// sweep and the 12h warm keep facts current; expiry is what happens when all of
// them stop, and an expired entry is a token without claims. At 24h, twice the
// warm interval, two skipped warms in a row (a directory outage) expired
// everyone's facts at once (claims-delivery-integrity F-6). A week leaves
// fourteen warms of room; staleness becomes something to measure (compiled_at
// is in every entry), not an outage.
const cacheTTL = 7 * 24 * time.Hour

// warmPace spreads a warm over time instead of one burst: at 10ms a person,
// 10,000 people take under two minutes of mostly-idle pass beside live
// traffic, never a wall of queries in front of the data plane's Redis or the
// directory (assessment P10).
var warmPace = 10 * time.Millisecond

// Facts layout (assessment SC1). One Redis hash per person, facts:{u}, with a
// field per project and claimserve.MetaField. A refresh is one HKEYS and one
// MULTI, O(the person's own projects); the data plane reads one HGET. The old
// layout, a key per (person, project), needed a SCAN of the whole keyspace per
// refresh, so a warm was O(people x keys). A person without MetaField is still
// on the old layout: their first refresh SCANs once for their old keys and
// deletes them in the same MULTI that writes the hash.

// UserGrant represents a simple role held by a user in Zitadel
type UserGrant struct {
	ProjectID string
	RoleKey   string
	ExpiresAt *time.Time
}

// fetchBaseGrants reads the user's direct grants from Syndra's ledger.
func fetchBaseGrants(ctx context.Context, userID string) ([]UserGrant, error) {
	grants, err := dbGetDirectGrantsForUser(ctx, userID, false)
	if err != nil {
		return nil, err
	}
	result := make([]UserGrant, 0, len(grants))
	for _, grant := range grants {
		result = append(result, UserGrant{
			ProjectID: grant.ProjectID,
			RoleKey:   grant.RoleKey,
			ExpiresAt: grant.ExpiresAt,
		})
	}
	return result, nil
}

// pass is what a compile reads once however many people it compiles (SC2,
// SC3): the mapping rules, bundle names and service-account marks, and, for a
// warm, one directory snapshot. Per person only their grants and bundle
// assignments are read. Bundle names and marks are read on first need, so a
// one-person refresh of someone with no bundle never lists bundles. Not safe
// for concurrent use: a warm is sequential, a refresh has its own.
type pass struct {
	rules []models.MappingRule
	// users is the warm's directory snapshot by id; nil means ask the
	// directory per person (a change-driven refresh of one person).
	users map[string]models.UserProfile
	// legacy is every old per-project key by person, from one sweep at the
	// start of a warm; nil means SCAN for the one person being refreshed.
	legacy map[string][]string

	bundles    map[string]string // bundle id -> name
	bundlesErr error
	bundlesRd  bool
	service    map[string]bool // user ids an operator marked
	serviceErr error
	serviceRd  bool
}

func newPass(ctx context.Context, snapshot []models.UserProfile) (*pass, error) {
	rules, err := dbGetActiveMappingRules(ctx)
	if err != nil {
		return nil, fmt.Errorf("cache compile: failed to load rules: %w", err)
	}
	p := &pass{rules: rules}
	if snapshot != nil {
		p.users = make(map[string]models.UserProfile, len(snapshot))
		for _, u := range snapshot {
			p.users[u.ID] = u
		}
	}
	return p, nil
}

// errNotInDirectory is a person the warm's directory snapshot does not list:
// deleted in Zitadel, still holding records here.
var errNotInDirectory = errors.New("not in the directory")

func (p *pass) profile(ctx context.Context, userID string) (models.UserProfile, bool, error) {
	if p.users == nil {
		return cacheFindUser(ctx, userID)
	}
	u, ok := p.users[userID]
	if !ok {
		return models.UserProfile{}, false, errNotInDirectory
	}
	return u, true, nil
}

func (p *pass) bundleNames(ctx context.Context) (map[string]string, error) {
	if !p.bundlesRd {
		p.bundlesRd = true
		all, err := dbGetAllBundles(ctx)
		p.bundles, p.bundlesErr = make(map[string]string, len(all)), err
		for _, b := range all {
			p.bundles[b.ID] = b.Name
		}
	}
	return p.bundles, p.bundlesErr
}

// isServiceAccount is the one rule (services.IsServiceAccount): a Zitadel
// machine user, or one an operator marked. Restated here because services
// imports this package.
func (p *pass) isServiceAccount(ctx context.Context, u models.UserProfile) (bool, error) {
	if u.IsMachine() {
		return true, nil
	}
	if !p.serviceRd {
		p.serviceRd = true
		rows, err := dbListServiceAccounts(ctx)
		p.service, p.serviceErr = make(map[string]bool, len(rows)), err
		for _, r := range rows {
			p.service[r.UserID] = true
		}
	}
	return p.service[u.ID], p.serviceErr
}

// userRoles is everything one user holds across every project, resolved once:
// their direct grants and bundles, then whatever the mapping rules derive from
// those. A refresh covers many projects from this single pass.
type userRoles struct {
	userID string
	active map[string]map[string]bool // projectID -> roleKey -> held
	// ends is the date each role ends, kept only when every source of it
	// ends: a role also held through a bundle, a rule or an open grant has none.
	ends map[string]map[string]time.Time
	// bundles names the bundles that give a role in each project.
	bundles map[string][]string
	profile models.UserProfile
	hasProf bool
	service bool
	// profileErr is a profile read that failed. Not fatal to a compile (a
	// token without an email still opens doors), but a warm must not store it.
	profileErr error
}

// compileUser evaluates all mapping rules against the user's current roles.
func compileUser(ctx context.Context, userID string, ps *pass) (userRoles, error) {
	baseGrants, err := fetchBaseGrants(ctx, userID)
	if err != nil {
		return userRoles{}, fmt.Errorf("cache compile: failed to fetch base grants: %w", err)
	}

	activeRoles := make(map[string]map[string]bool) // projectID -> map[roleKey]exists
	ends := map[string]map[string]time.Time{}
	endless := map[string]bool{} // projectID+"\x00"+roleKey held with no end
	for _, g := range baseGrants {
		if activeRoles[g.ProjectID] == nil {
			activeRoles[g.ProjectID] = make(map[string]bool)
		}
		activeRoles[g.ProjectID][g.RoleKey] = true
		if g.ExpiresAt == nil {
			endless[g.ProjectID+"\x00"+g.RoleKey] = true
			continue
		}
		if ends[g.ProjectID] == nil {
			ends[g.ProjectID] = map[string]time.Time{}
		}
		if g.ExpiresAt.After(ends[g.ProjectID][g.RoleKey]) {
			ends[g.ProjectID][g.RoleKey] = *g.ExpiresAt
		}
	}

	// What their bundles give them, through each assignment's pinned version.
	bundleRoles, err := bundleRolesFor(ctx, userID)
	if err != nil {
		return userRoles{}, fmt.Errorf("cache compile: %w", err)
	}
	bundleIDs := map[string]map[string]bool{} // projectID -> bundle ids
	for _, r := range bundleRoles {
		if activeRoles[r.ProjectID] == nil {
			activeRoles[r.ProjectID] = make(map[string]bool)
		}
		activeRoles[r.ProjectID][r.RoleKey] = true
		endless[r.ProjectID+"\x00"+r.RoleKey] = true
		if bundleIDs[r.ProjectID] == nil {
			bundleIDs[r.ProjectID] = map[string]bool{}
		}
		bundleIDs[r.ProjectID][r.BundleID] = true
	}
	bundles, err := bundleNamesByProject(ctx, ps, bundleIDs)
	if err != nil {
		return userRoles{}, fmt.Errorf("cache compile: %w", err)
	}

	// What the rules derive is models.RuleClosure's answer, the same function
	// the cascades use to decide what Zitadel is sent: the token and Zitadel
	// follow one chain, not two copies of it.
	base := map[models.HeldRole]bool{}
	for pid, byRole := range activeRoles {
		for role := range byRole {
			base[models.HeldRole{ProjectID: pid, RoleKey: role}] = true
		}
	}
	for k := range models.RuleClosure(base, ps.rules, models.NewHeldRole) {
		if base[k] {
			continue
		}
		if activeRoles[k.ProjectID] == nil {
			activeRoles[k.ProjectID] = make(map[string]bool)
		}
		activeRoles[k.ProjectID][k.RoleKey] = true
		// ponytail: a role a rule gives has no end, even when its source
		// does; carry the source's end once an app needs it (NEXT.md).
		endless[k.ProjectID+"\x00"+k.RoleKey] = true
	}

	for pid, byRole := range ends {
		for role := range byRole {
			if endless[pid+"\x00"+role] {
				delete(byRole, role)
			}
		}
	}

	u := userRoles{userID: userID, active: activeRoles, ends: ends, bundles: bundles}
	// Profile attributes (email, team, ...) are captured now because a
	// directory read is affordable during compile and is not affordable inside
	// the Actions v2 latency budget.
	if profile, ok, err := ps.profile(ctx, userID); err != nil {
		// Non-fatal: roles are the load-bearing part of the token. A missing
		// email claim is a degraded token; a missing roles claim is a locked door.
		if !errors.Is(err, errNotInDirectory) {
			log.Printf("[CACHE] profile attributes unavailable for %s: %v", userID, err)
		}
		u.profileErr = err
	} else if ok {
		u.profile, u.hasProf = profile, true
		// The token is the only place an app learns this, so a failed read of
		// the marks counts as a failed profile read: never guess "a person".
		if service, serr := ps.isServiceAccount(ctx, profile); serr != nil {
			log.Printf("[CACHE] service-account mark unavailable for %s: %v", userID, serr)
			u.profileErr = serr
		} else {
			u.service = service
		}
	}
	return u, nil
}

// facts is the user's compiled FACTS for one project, not a finished claim map.
//
// The token's shape is an operator-editable profile resolved at read time
// (internal/claims). Baking the shape in here would mean every claim-name
// or format edit silently applied only to users whose cache happened to be
// recompiled afterwards — an edit that takes effect per-user at random is
// worse than no edit at all.
func (u userRoles) facts(projectID string) claims.Facts {
	derivedRoles := []string{}
	for role := range u.active[projectID] {
		derivedRoles = append(derivedRoles, role)
	}
	sort.Strings(derivedRoles)

	facts := claims.Facts{
		Roles:      derivedRoles,
		UserID:     u.userID,
		ProjectID:  projectID,
		CompiledAt: time.Now().UTC().Format(time.RFC3339),
	}
	if u.hasProf {
		facts.Email, facts.Name, facts.Title, facts.Team = u.profile.Email, u.profile.Name, u.profile.Title, u.profile.Team
		facts.GivenName, facts.FamilyName, facts.Username = u.profile.GivenName, u.profile.FamilyName, u.profile.Username
		facts.IsServiceAccount = u.service
	}
	facts.Bundles = u.bundles[projectID]
	for role, end := range u.ends[projectID] {
		if facts.RoleEnds == nil {
			facts.RoleEnds = map[string]string{}
		}
		facts.RoleEnds[role] = end.UTC().Format(time.DateOnly)
	}
	return facts
}

// CompileFacts computes a user's facts for a project without touching Redis.
// The simulator reads this: a preview computes, the token path serves what the
// write paths stored. A preview that stored its own answer would quietly
// repair the one project being looked at and hide a cache that had gone stale.
func CompileFacts(ctx context.Context, userID, projectID string) (claims.Facts, error) {
	ps, err := newPass(ctx, nil)
	if err != nil {
		return claims.Facts{}, err
	}
	u, err := compileUser(ctx, userID, ps)
	if err != nil {
		return claims.Facts{}, err
	}
	return u.facts(projectID), nil
}

// StoredFacts is what the token path serves right now: the facts the write
// paths stored for (user, project), and false when there is no entry. The
// explain step reads this so its "claims are ready" check is the live answer,
// not a fresh compile that would hide a stale or missing entry. Same read as
// the data plane: the hash field, then the old key while it may still exist.
func StoredFacts(ctx context.Context, userID, projectID string) (claims.Facts, bool, error) {
	raw, err := redisHGet(ctx, claimserve.FactsKey(userID), projectID)
	if errors.Is(err, redis.Nil) {
		raw, err = redisGet(ctx, claimserve.LegacyFactsKey(userID, projectID))
	}
	if errors.Is(err, redis.Nil) {
		return claims.Facts{}, false, nil
	}
	if err != nil {
		return claims.Facts{}, false, err
	}
	var f claims.Facts
	if err := json.Unmarshal([]byte(raw), &f); err != nil {
		return claims.Facts{}, false, fmt.Errorf("stored facts for %s in %s: %w", userID, projectID, err)
	}
	return f, true, nil
}

// RefreshUser recompiles a user's claims after something changed what they
// hold. The token path serves ONLY what is stored here, so deleting a key and
// waiting is an outage of the claims, not a refresh: nothing else rebuilds it.
//
// It refreshes every project the user already has facts for (a revoke must
// reach all of them, including one they no longer hold anything in), every
// project they now hold roles in, and any project the caller names. A project
// is dropped only when it must not be served: it now yields no roles (and
// nobody asked for it), or the compile failed, where stale facts could still
// carry a revoked role and the data plane's degraded path is safer.
func RefreshUser(ctx context.Context, userID string, projectIDs ...string) error {
	ps, err := newPass(ctx, nil)
	if err != nil {
		return err
	}
	return refreshUser(ctx, userID, false, projectIDs, ps)
}

// userLocks serialize refreshes per user, so a slower compile can't store over
// a newer one (a warm racing a revoke would restore the revoked role for a
// day). ponytail: in-process only, the backend is one process; several
// replicas need a Redis lock. One mutex per user ever refreshed, small next to
// the user table.
var userLocks sync.Map

// globEscape makes s match itself literally in a SCAN MATCH pattern: a user id
// carrying * or [ would otherwise refresh (or drop) other users' keys.
var globEscape = strings.NewReplacer(`\`, `\\`, `*`, `\*`, `?`, `\?`, `[`, `\[`, `]`, `\]`).Replace

// refreshUser is RefreshUser; warm marks a background pass, which must never
// make a good entry worse: a failed compile there keeps the facts, since
// nothing changed and the stored claims are as right as they were. A
// change-driven refresh drops them instead, where stale facts could carry a
// revoked role.
func refreshUser(ctx context.Context, userID string, warm bool, projectIDs []string, ps *pass) error {
	if userID == claimserve.CanaryUserID {
		return WriteCanary(ctx)
	}
	mu, _ := userLocks.LoadOrStore(userID, &sync.Mutex{})
	mu.(*sync.Mutex).Lock()
	defer mu.(*sync.Mutex).Unlock()

	key := claimserve.FactsKey(userID)
	fields, err := redisHKeys(ctx, key)
	if err != nil {
		return fmt.Errorf("cache refresh: read %s: %w", key, err)
	}
	existing := map[string]bool{}
	migrated := false
	for _, f := range fields {
		if f == claimserve.MetaField {
			migrated = true
			continue
		}
		existing[f] = true
	}
	// Still on the old layout: find their old keys once, so the write below
	// deletes them with the same MULTI and the data plane's fallback read
	// never finds a project the hash has since dropped.
	var legacy []string
	if !migrated {
		prefix := claimserve.LegacyFactsKey(userID, "")
		if ps.legacy != nil {
			legacy = ps.legacy[userID]
		} else if legacy, err = redisScanKeys(ctx, globEscape(prefix)+"*"); err != nil {
			return fmt.Errorf("cache refresh: scan error: %w", err)
		}
		for _, k := range legacy {
			existing[strings.TrimPrefix(k, prefix)] = true
		}
	}

	u, err := compileUser(ctx, userID, ps)
	if err != nil {
		if len(existing) > 0 && !warm {
			if derr := redisDel(ctx, append(legacy, key)...); derr != nil {
				log.Printf("[CACHE] could not drop %d stale entries for %s: %v", len(existing), userID, derr)
			}
		}
		return err
	}
	if warm && u.profileErr != nil {
		if isGone(u.profileErr) {
			return errGone
		}
		return fmt.Errorf("cache warm: profile unavailable, keeping entries: %w", u.profileErr)
	}

	asked := map[string]bool{}
	targets := existing
	for _, p := range projectIDs {
		asked[p], targets[p] = true, true
	}
	for p := range u.active {
		targets[p] = true
	}

	set := map[string]string{claimserve.MetaField: time.Now().UTC().Format(time.RFC3339)}
	var drop []string
	for p := range targets {
		facts := u.facts(p)
		if len(facts.Roles) == 0 && !asked[p] {
			drop = append(drop, p)
			continue
		}
		data, err := json.Marshal(facts)
		if err != nil {
			return fmt.Errorf("cache compile: marshal failed: %w", err)
		}
		set[p] = string(data)
	}
	if err := redisWriteFacts(ctx, key, set, drop, legacy, cacheTTL); err != nil {
		// Nothing of the MULTI applied: the OLD facts stand, and may hold a
		// revoked role.
		if !warm {
			_ = redisDel(ctx, append(legacy, key)...)
		}
		return fmt.Errorf("cache compile: redis write failed: %w", err)
	}
	return nil
}

// WriteCanary writes the L1 canary's facts (claims-delivery-integrity S6)
// through the same write every person's facts take, so the probe exercises
// compile → Redis → snapshot. The canary is in no directory: its facts are
// fixed (claimserve.CanaryFacts). Each warm writes it first, and a repair
// after a canary miss (Redis lost) writes it again.
func WriteCanary(ctx context.Context) error {
	data, err := json.Marshal(claimserve.CanaryFacts())
	if err != nil {
		return err
	}
	set := map[string]string{claimserve.MetaField: time.Now().UTC().Format(time.RFC3339), claimserve.CanaryProjectID: string(data)}
	if err := redisWriteFacts(ctx, claimserve.FactsKey(claimserve.CanaryUserID), set, nil, nil, cacheTTL); err != nil {
		return fmt.Errorf("cache: write the canary's facts: %w", err)
	}
	return nil
}

// RebuildUserCache refreshes a user's cache, logging rather than returning a
// failure, for callers with nothing to do about one.
func RebuildUserCache(ctx context.Context, userID string, projectIDs []string) {
	if err := RefreshUser(ctx, userID, projectIDs...); err != nil {
		log.Printf("[CACHE] rebuild failed for %s: %v", userID, err)
	}
}

// errGone is a person Zitadel no longer has: deleted there, still holding
// records here. Not a failure to retry on every pass (assessment D1).
var errGone = errors.New("gone from Zitadel")

// isGone is a person the warm's directory snapshot does not list, or a
// Zitadel 404 for them.
func isGone(err error) bool {
	if errors.Is(err, errNotInDirectory) {
		return true
	}
	var se *zitadel.StatusError
	return errors.As(err, &se) && se.Code == 404
}

// legacyCleared is set once a warm's sweep found no old per-project key left:
// from then on this process never sweeps again.
var legacyCleared atomic.Bool

// legacyKeys is every old per-project key, by person, from one SCAN sweep of
// the keyspace: once per warm, never once per person, and not at all once a
// sweep has found none. Project and user ids are Zitadel's numeric ids, so the
// person is everything between the prefix and the last colon.
func legacyKeys(ctx context.Context) (map[string][]string, error) {
	out := map[string][]string{}
	if legacyCleared.Load() {
		return out, nil
	}
	keys, err := redisScanKeys(ctx, claimserve.LegacyFactsPrefix+"*")
	if err != nil {
		return nil, fmt.Errorf("sweep old keys: %w", err)
	}
	for _, k := range keys {
		rest := strings.TrimPrefix(k, claimserve.LegacyFactsPrefix)
		if i := strings.LastIndex(rest, ":"); i > 0 {
			out[rest[:i]] = append(out[rest[:i]], k)
		}
	}
	if len(keys) == 0 {
		legacyCleared.Store(true)
	}
	return out, nil
}

// WarmReport is how the last warm went. Refreshed and Failed count only people
// who still exist in Zitadel; Gone is the rest.
type WarmReport struct {
	At        time.Time `json:"at"`
	Skipped   bool      `json:"skipped"`
	Refreshed int       `json:"refreshed"`
	Of        int       `json:"of"`
	Failed    int       `json:"failed"`
	Gone      int       `json:"gone"`
}

var lastWarm atomic.Pointer[WarmReport]

// LastWarm is the last finished warm, false before the first.
func LastWarm() (WarmReport, bool) {
	if r := lastWarm.Load(); r != nil {
		return *r, true
	}
	return WarmReport{}, false
}

// Warm refreshes every known user's claims once. Entries come only from change
// events and expire after cacheTTL, so without this a restart (or a quiet day)
// leaves the token path answering degraded for people whose access never
// changed. Sequential and paced on purpose: it runs beside live traffic.
//
// One pass reads the global tables once and the directory once (one snapshot:
// ListUsers pages, no per-person call), then per person only their own grants
// and their own facts hash. A person the snapshot does not list is gone:
// counted apart, their facts kept, and nothing asked of Zitadel about them.
//
// A pass that skips or fails anyone says so as an ERROR with its counts: a warm
// that quietly stops keeping facts fresh is how expiry became the outage.
func Warm(ctx context.Context) error {
	if err := WriteCanary(ctx); err != nil {
		log.Printf("[CACHE] ERROR %v: the L1 probe will fail until it is written", err)
	}
	ids, err := dbGetAllKnownUserIDs(ctx)
	if err != nil {
		return fmt.Errorf("cache warm: %w", err)
	}
	// A directory that isn't answering fails every user's profile read; say so
	// once rather than once per user.
	users, current := directorySnapshot(ctx)
	if !current {
		lastWarm.Store(&WarmReport{At: time.Now(), Skipped: true, Of: len(ids)})
		log.Printf("[CACHE] ERROR warm skipped: the directory is not answering; 0 of %d users refreshed (stored facts last %s from their compile)", len(ids), cacheTTL)
		return nil
	}
	if users == nil {
		users = []models.UserProfile{}
	}
	ps, err := newPass(ctx, users)
	if err != nil {
		return fmt.Errorf("cache warm: %w", err)
	}
	if ps.legacy, err = legacyKeys(ctx); err != nil {
		return fmt.Errorf("cache warm: %w", err)
	}
	rep := WarmReport{Of: len(ids)}
	for i, id := range ids {
		if i > 0 && warmPace > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(warmPace):
			}
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		switch err := refreshUser(ctx, id, true, nil, ps); {
		case err == nil:
			rep.Refreshed++
		case errors.Is(err, errGone):
			rep.Gone++
		default:
			rep.Failed++
			log.Printf("[CACHE] warm failed for %s: %v", id, err)
		}
	}
	rep.Of -= rep.Gone
	rep.At = time.Now()
	lastWarm.Store(&rep)
	if rep.Failed > 0 {
		log.Printf("[CACHE] ERROR warm: refreshed %d of %d users, %d failed (%d gone from Zitadel, not counted)", rep.Refreshed, rep.Of, rep.Failed, rep.Gone)
		return nil
	}
	log.Printf("[CACHE] Warmed claims for %d of %d users (%d gone from Zitadel, not counted)", rep.Refreshed, rep.Of, rep.Gone)
	return nil
}
