package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"mkauth/internal/db"
)

const cacheTTL = 24 * time.Hour

// CompileUserCache builds a flat JSON claims map for a given user+project
// and stores it in Redis for sub-millisecond retrieval by the Data Plane.
//
// The key format is: mapping:<userID>:<projectID>
// The value is a JSON object of claim keys → role arrays.
func CompileUserCache(ctx context.Context, userID, projectID string) error {
	// 1. Query all active mapping rules that target this project
	rules, err := db.GetActiveMappingRules(ctx)
	if err != nil {
		return fmt.Errorf("cache compile: failed to load rules: %w", err)
	}

	// 2. Build the derived roles for this project
	derivedRoles := []string{}
	for _, rule := range rules {
		if rule.TargetProject == projectID {
			derivedRoles = append(derivedRoles, rule.TargetRole)
		}
	}

	// 3. Construct the claims payload
	claims := map[string]interface{}{
		"derived_roles": derivedRoles,
		"compiled_at":   time.Now().UTC().Format(time.RFC3339),
		"source":        "mkauth_cache_compiler",
	}

	data, err := json.Marshal(claims)
	if err != nil {
		return fmt.Errorf("cache compile: marshal failed: %w", err)
	}

	// 4. Write to Redis with TTL
	cacheKey := fmt.Sprintf("mapping:%s:%s", userID, projectID)
	err = db.Redis.Set(ctx, cacheKey, string(data), cacheTTL).Err()
	if err != nil {
		return fmt.Errorf("cache compile: redis write failed: %w", err)
	}

	log.Printf("[CACHE] Compiled %d derived roles for %s → %s", len(derivedRoles), userID, projectID)
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

// RebuildUserCache invalidates + recompiles cache for a user triggered by webhooks.
func RebuildUserCache(ctx context.Context, userID string, projectIDs []string) {
	_ = InvalidateUser(ctx, userID)
	for _, pid := range projectIDs {
		if err := CompileUserCache(ctx, userID, pid); err != nil {
			log.Printf("[CACHE ERROR] Rebuild failed for %s/%s: %v", userID, pid, err)
		}
	}
}
