package cache

import (
	"context"
	"log"
	"time"

	"mkauth/internal/db"
	"mkauth/internal/models"
)

var (
	dbGetDirectGrantsForUser = db.GetDirectGrantsForUser
	dbGetActiveMappingRules  = db.GetActiveMappingRules
	dbGetBundlesForUser      = db.GetBundlesForUser
	dbGetRolesForBundle      = db.GetRolesForBundle

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

// grantsToBundleRoles is a helper to look up roles for multiple bundles.
func grantsToBundleRoles(ctx context.Context, bundles []models.Bundle) []models.BundleRole {
	var roles []models.BundleRole
	for _, b := range bundles {
		r, err := dbGetRolesForBundle(ctx, b.ID)
		if err != nil {
			log.Printf("[CACHE WARN] Failed to fetch roles for bundle %s: %v", b.ID, err)
			continue
		}
		roles = append(roles, r...)
	}
	return roles
}
