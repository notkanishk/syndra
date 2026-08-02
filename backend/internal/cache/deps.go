package cache

import (
	"context"
	"log"
	"time"

	"mkauth/internal/db"
	"mkauth/internal/directory"
	"mkauth/internal/models"
)

var (
	dbGetDirectGrantsForUser = db.GetDirectGrantsForUser
	dbGetActiveMappingRules  = db.GetActiveMappingRules
	// Version-aware. The cache compiles the claims a token is issued from, so
	// resolving bundles through the mutable working copy meant an unpublished
	// edit reached real tokens on the next rebuild — before anybody published
	// it, and without appearing in any plan.
	dbGetUserBundleRoles = db.GetUserBundleRolesGrouped

	// cacheFindUser supplies the profile attributes (email, name, title,
	// team) a claim profile may project into a token. Injectable so compiler
	// tests don't need a live directory.
	cacheFindUser = func(ctx context.Context, userID string) (models.UserProfile, bool, error) {
		return directory.Default.FindUser(ctx, userID)
	}

	redisSet = func(ctx context.Context, key string, value string, ttl time.Duration) error {
		return db.Redis.Set(ctx, key, value, ttl).Err()
	}
	redisDel = func(ctx context.Context, keys ...string) error {
		return db.Redis.Del(ctx, keys...).Err()
	}
	redisScanKeys = func(ctx context.Context, pattern string) ([]string, error) {
		var keys []string
		iter := db.Redis.Scan(ctx, 0, pattern, 100).Iterator()
		for iter.Next(ctx) {
			keys = append(keys, iter.Val())
		}
		return keys, iter.Err()
	}
)

// bundleRolesFor returns everything this user gets from their bundles, each
// resolved through the version THEY are pinned to.
func bundleRolesFor(ctx context.Context, userID string) []models.BundleRole {
	byBundle, err := dbGetUserBundleRoles(ctx, userID)
	if err != nil {
		log.Printf("[CACHE WARN] Failed to fetch bundle roles for %s: %v", userID, err)
		return nil
	}
	var roles []models.BundleRole
	for _, r := range byBundle {
		roles = append(roles, r...)
	}
	return roles
}
