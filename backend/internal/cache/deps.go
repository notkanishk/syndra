package cache

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/redis/go-redis/v9"

	"syndra/internal/db"
	"syndra/internal/directory"
	"syndra/internal/models"
)

var (
	dbGetDirectGrantsForUser = db.GetDirectGrantsForUser
	dbGetActiveMappingRules  = db.GetActiveMappingRules
	dbGetAllKnownUserIDs     = db.GetAllKnownUserIDs
	// Version-aware. The cache compiles the claims a token is issued from, so
	// resolving bundles through the mutable working copy meant an unpublished
	// edit reached real tokens on the next rebuild — before anybody published
	// it, and without appearing in any plan.
	dbGetUserBundleRoles = db.GetUserBundleRolesGrouped

	// cacheFindUser supplies the profile attributes (email, name, title,
	// team) a claim profile may project into a token, for a one-person
	// refresh. A warm reads its one directory snapshot instead.
	cacheFindUser = func(ctx context.Context, userID string) (models.UserProfile, bool, error) {
		return directory.Default.FindUser(ctx, userID)
	}

	// dbGetAllBundles names bundles: a token carries bundle names, not ids, so
	// a rename reaches the app. Read once per pass.
	dbGetAllBundles = db.GetAllBundles
	// dbListServiceAccounts is the operator's service-account marks. Read once
	// per pass.
	dbListServiceAccounts = db.ListServiceAccounts

	// directorySnapshot is the warm's one read of the directory, and whether
	// it is current (a read served from its last good copy is not).
	directorySnapshot = func(ctx context.Context) ([]models.UserProfile, bool) {
		users, err := directory.Default.Users(ctx)
		if err != nil {
			return nil, false
		}
		_, current := directory.FreshnessOf(directory.Default)
		return users, current
	}

	// The facts hash (claimserve.FactsKey): one HKEYS and one MULTI per
	// refresh, one HGET per token.
	redisHKeys = func(ctx context.Context, key string) ([]string, error) {
		return db.Redis.HKeys(ctx, key).Result()
	}
	redisHGet = func(ctx context.Context, key, field string) (string, error) {
		return db.Redis.HGet(ctx, key, field).Result()
	}
	// redisWriteFacts sets and drops fields of one person's hash, renews its
	// TTL and deletes their old per-project keys, atomically.
	redisWriteFacts = func(ctx context.Context, key string, set map[string]string, drop, legacy []string, ttl time.Duration) error {
		_, err := db.Redis.TxPipelined(ctx, func(p redis.Pipeliner) error {
			p.HSet(ctx, key, set)
			if len(drop) > 0 {
				p.HDel(ctx, key, drop...)
			}
			p.Expire(ctx, key, ttl)
			if len(legacy) > 0 {
				p.Del(ctx, legacy...)
			}
			return nil
		})
		return err
	}

	// The old layout, read and removed during the move to the hash.
	redisGet = func(ctx context.Context, key string) (string, error) {
		return db.Redis.Get(ctx, key).Result()
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
// A read error is returned, never logged away: the caller persists what this
// returns as the facts a token is built from, and an empty answer is
// indistinguishable from "belongs to no bundle". Swallowing it here mints a
// token missing every bundle role the person holds, for as long as that cache
// entry lives.
func bundleRolesFor(ctx context.Context, userID string) ([]models.BundleRole, error) {
	byBundle, err := dbGetUserBundleRoles(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("bundle roles for %s: %w", userID, err)
	}
	var roles []models.BundleRole
	for _, r := range byBundle {
		roles = append(roles, r...)
	}
	return roles, nil
}

// bundleNamesByProject turns each project's bundle ids into sorted bundle
// names. Only read when the person holds a bundle at all. A bundle missing
// from the list (retired) is left out rather than sent by its id.
func bundleNamesByProject(ctx context.Context, ps *pass, byProject map[string]map[string]bool) (map[string][]string, error) {
	if len(byProject) == 0 {
		return nil, nil
	}
	names, err := ps.bundleNames(ctx)
	if err != nil {
		return nil, fmt.Errorf("bundle names: %w", err)
	}
	out := make(map[string][]string, len(byProject))
	for pid, ids := range byProject {
		list := []string{}
		for id := range ids {
			if n := names[id]; n != "" {
				list = append(list, n)
			}
		}
		sort.Strings(list)
		out[pid] = list
	}
	return out, nil
}
