package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"time"

	"mkauth/internal/db"
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
	grants, err := db.GetDirectGrantsForUser(ctx, userID, false)
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

// CompileUserCache builds a flat JSON claims map for a given user+project
// by evaluating all mapping rules against the user's current roles.
func CompileUserCache(ctx context.Context, userID, projectID string) error {
	// 1. Fetch user's base roles from Zitadel (mocked)
	baseGrants, err := fetchBaseGrants(ctx, userID)
	if err != nil {
		return fmt.Errorf("cache compile: failed to fetch base grants: %w", err)
	}

	// 2. Load all active mapping rules
	allRules, err := db.GetActiveMappingRules(ctx)
	if err != nil {
		return fmt.Errorf("cache compile: failed to load rules: %w", err)
	}

	activeRoles := make(map[string]map[string]bool) // projectID -> map[roleKey]exists
	for _, g := range baseGrants {
		if activeRoles[g.ProjectID] == nil {
			activeRoles[g.ProjectID] = make(map[string]bool)
		}
		activeRoles[g.ProjectID][g.RoleKey] = true
	}

	// Fetch user's assigned bundles from MkAuth DB
	userBundles, err := db.GetBundlesForUser(ctx, userID)
	if err != nil {
		log.Printf("[CACHE WARN] Failed to fetch bundles for %s: %v", userID, err)
	}
	for _, b := range userBundles {
		roles, err := db.GetRolesForBundle(ctx, b.ID)
		if err != nil {
			log.Printf("[CACHE WARN] Failed to fetch roles for bundle %s: %v", b.ID, err)
			continue
		}
		for _, r := range roles {
			if activeRoles[r.ProjectID] == nil {
				activeRoles[r.ProjectID] = make(map[string]bool)
			}
			activeRoles[r.ProjectID][r.RoleKey] = true
		}
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

	// 4. Extract derived roles for the specifically requested project
	derivedRoles := []string{}
	if activeRoles[projectID] != nil {
		for role := range activeRoles[projectID] {
			derivedRoles = append(derivedRoles, role)
		}
	}
	sort.Strings(derivedRoles)

	// 5. Construct the claims payload
	claims := map[string]interface{}{
		"roles":       derivedRoles,
		"user_id":     userID,
		"project_id":  projectID,
		"compiled_at": time.Now().UTC().Format(time.RFC3339),
		"source":      "mkauth_cache_compiler_v1",
	}

	data, err := json.Marshal(claims)
	if err != nil {
		return fmt.Errorf("cache compile: marshal failed: %w", err)
	}

	// 6. Write to Redis
	cacheKey := fmt.Sprintf("mapping:%s:%s", userID, projectID)
	err = db.Redis.Set(ctx, cacheKey, string(data), cacheTTL).Err()
	if err != nil {
		return fmt.Errorf("cache compile: redis write failed: %w", err)
	}

	log.Printf("[CACHE] Successfully compiled %d roles for %s in %s", len(derivedRoles), userID, projectID)
	return nil
}

// InvalidateUser removes all cached entries for a user across all projects.
func InvalidateUser(ctx context.Context, userID string) error {
	pattern := fmt.Sprintf("mapping:%s:*", userID)
	iter := db.Redis.Scan(ctx, 0, pattern, 100).Iterator()

	count := 0
	for iter.Next(ctx) {
		if err := db.Redis.Del(ctx, iter.Val()).Err(); err != nil {
			log.Printf("[CACHE] Failed to delete key %s: %v", iter.Val(), err)
		}
		count++
	}

	if err := iter.Err(); err != nil {
		return fmt.Errorf("cache invalidate: scan error: %w", err)
	}

	log.Printf("[CACHE] Invalidated %d cached entries for user %s", count, userID)
	return nil
}

// RebuildUserCache invalidates + recompiles cache for a user.
func RebuildUserCache(ctx context.Context, userID string, projectIDs []string) {
	_ = InvalidateUser(ctx, userID)
	for _, pid := range projectIDs {
		if err := CompileUserCache(ctx, userID, pid); err != nil {
			log.Printf("[CACHE ERROR] Rebuild failed for %s/%s: %v", userID, pid, err)
		}
	}
}
