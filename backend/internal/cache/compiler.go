package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"syndra/internal/claims"
	"syndra/internal/models"
)

const cacheTTL = 24 * time.Hour

// UserGrant represents a simple role held by a user in Zitadel
type UserGrant struct {
	ProjectID string
	RoleKey   string
}

// fetchBaseGrants simulates fetching the user's primary roles from Zitadel.
// In Phase 2, this will call the real Zitadel Management API using MgmtClient.
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
		})
	}
	return result, nil
}

// userRoles is everything one user holds across every project, resolved once:
// their direct grants and bundles, then whatever the mapping rules derive from
// those. A refresh covers many projects from this single pass.
type userRoles struct {
	userID  string
	active  map[string]map[string]bool // projectID -> roleKey -> held
	profile models.UserProfile
	hasProf bool
}

// compileUser evaluates all mapping rules against the user's current roles.
func compileUser(ctx context.Context, userID string) (userRoles, error) {
	// 1. Fetch user's base roles from Zitadel (mocked)
	baseGrants, err := fetchBaseGrants(ctx, userID)
	if err != nil {
		return userRoles{}, fmt.Errorf("cache compile: failed to fetch base grants: %w", err)
	}

	// 2. Load all active mapping rules
	allRules, err := dbGetActiveMappingRules(ctx)
	if err != nil {
		return userRoles{}, fmt.Errorf("cache compile: failed to load rules: %w", err)
	}

	activeRoles := make(map[string]map[string]bool) // projectID -> map[roleKey]exists
	for _, g := range baseGrants {
		if activeRoles[g.ProjectID] == nil {
			activeRoles[g.ProjectID] = make(map[string]bool)
		}
		activeRoles[g.ProjectID][g.RoleKey] = true
	}

	// What their bundles give them, through each assignment's pinned version.
	bundleRoles, err := bundleRolesFor(ctx, userID)
	if err != nil {
		return userRoles{}, fmt.Errorf("cache compile: %w", err)
	}
	for _, r := range bundleRoles {
		if activeRoles[r.ProjectID] == nil {
			activeRoles[r.ProjectID] = make(map[string]bool)
		}
		activeRoles[r.ProjectID][r.RoleKey] = true
	}

	// 3. Iterative Role Resolution (Forward Pass)
	// We start with the user's base roles and see which rules they activate.

	// Simple fixed-point iteration (max passes = number of rules)
	changed := true
	for i := 0; i < len(allRules) && changed; i++ {
		changed = false
		for _, rule := range allRules {
			// If user has the source role...
			if activeRoles[rule.SourceProject] != nil && activeRoles[rule.SourceProject][rule.SourceRole] {
				// ...and doesn't yet have the target role
				if activeRoles[rule.TargetProject] == nil {
					activeRoles[rule.TargetProject] = make(map[string]bool)
				}
				if !activeRoles[rule.TargetProject][rule.TargetRole] {
					activeRoles[rule.TargetProject][rule.TargetRole] = true
					changed = true
				}
			}
		}
	}

	u := userRoles{userID: userID, active: activeRoles}
	// Profile attributes (email, team, ...) are captured now because a
	// directory call is affordable during compile and is not affordable inside
	// the Actions v2 latency budget.
	if profile, ok, err := cacheFindUser(ctx, userID); err != nil {
		// Non-fatal: roles are the load-bearing part of the token. A missing
		// email claim is a degraded token; a missing roles claim is a locked door.
		log.Printf("[CACHE] profile attributes unavailable for %s: %v", userID, err)
	} else if ok {
		u.profile, u.hasProf = profile, true
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
	}
	return facts
}

// CompileFacts computes a user's facts for a project without touching Redis.
// The simulator reads this: a preview computes, the token path serves what the
// write paths stored. A preview that stored its own answer would quietly
// repair the one project being looked at and hide a cache that had gone stale.
func CompileFacts(ctx context.Context, userID, projectID string) (claims.Facts, error) {
	u, err := compileUser(ctx, userID)
	if err != nil {
		return claims.Facts{}, err
	}
	return u.facts(projectID), nil
}

func cacheKey(userID, projectID string) string {
	return fmt.Sprintf("mapping:%s:%s", userID, projectID)
}

func store(ctx context.Context, facts claims.Facts) error {
	data, err := json.Marshal(facts)
	if err != nil {
		return fmt.Errorf("cache compile: marshal failed: %w", err)
	}
	if err := redisSet(ctx, cacheKey(facts.UserID, facts.ProjectID), string(data), cacheTTL); err != nil {
		return fmt.Errorf("cache compile: redis write failed: %w", err)
	}
	log.Printf("[CACHE] Successfully compiled %d roles for %s in %s", len(facts.Roles), facts.UserID, facts.ProjectID)
	return nil
}

// CompileUserCache compiles and stores a user's claims for one project.
func CompileUserCache(ctx context.Context, userID, projectID string) error {
	facts, err := CompileFacts(ctx, userID, projectID)
	if err != nil {
		return err
	}
	return store(ctx, facts)
}

// RefreshUser recompiles a user's claims after something changed what they
// hold. The token path serves ONLY what is stored here, so deleting a key and
// waiting is an outage of the claims, not a refresh: nothing else rebuilds it.
//
// It refreshes every project the user already has a key for (a revoke must
// reach all of them, including one they no longer hold anything in), every
// project they now hold roles in, and any project the caller names. A key is
// dropped only when it must not be served: its project now yields no roles
// (and nobody asked for it), or the compile failed, where a stale key could
// still carry a revoked role and the data plane's degraded path is safer.
func RefreshUser(ctx context.Context, userID string, projectIDs ...string) error {
	prefix := cacheKey(userID, "")
	keys, err := redisScanKeys(ctx, prefix+"*")
	if err != nil {
		return fmt.Errorf("cache refresh: scan error: %w", err)
	}

	u, err := compileUser(ctx, userID)
	if err != nil {
		if len(keys) > 0 {
			if derr := redisDel(ctx, keys...); derr != nil {
				log.Printf("[CACHE] could not drop %d stale entries for %s: %v", len(keys), userID, derr)
			}
		}
		return err
	}

	asked := map[string]bool{}
	targets := map[string]bool{}
	for _, p := range projectIDs {
		asked[p], targets[p] = true, true
	}
	for _, k := range keys {
		targets[strings.TrimPrefix(k, prefix)] = true
	}
	for p := range u.active {
		targets[p] = true
	}

	var errs []error
	for p := range targets {
		facts := u.facts(p)
		if len(facts.Roles) == 0 && !asked[p] {
			if err := redisDel(ctx, cacheKey(userID, p)); err != nil {
				errs = append(errs, fmt.Errorf("cache refresh: drop %s: %w", p, err))
			}
			continue
		}
		if err := store(ctx, facts); err != nil {
			// A failed write leaves the OLD value, which may hold a revoked role.
			_ = redisDel(ctx, cacheKey(userID, p))
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// RebuildUserCache refreshes a user's cache, logging rather than returning a
// failure, for callers with nothing to do about one.
func RebuildUserCache(ctx context.Context, userID string, projectIDs []string) {
	if err := RefreshUser(ctx, userID, projectIDs...); err != nil {
		log.Printf("[CACHE] rebuild failed for %s: %v", userID, err)
	}
}

// Warm refreshes every known user's claims once. Entries come only from change
// events and expire after cacheTTL, so without this a restart (or a quiet day)
// leaves the token path answering degraded for people whose access never
// changed. Sequential on purpose: it runs beside live traffic and a slow pass
// costs nothing.
func Warm(ctx context.Context) error {
	ids, err := dbGetAllKnownUserIDs(ctx)
	if err != nil {
		return fmt.Errorf("cache warm: %w", err)
	}
	warmed := 0
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := RefreshUser(ctx, id); err != nil {
			log.Printf("[CACHE] warm failed for %s: %v", id, err)
			continue
		}
		warmed++
	}
	log.Printf("[CACHE] Warmed claims for %d of %d users", warmed, len(ids))
	return nil
}
