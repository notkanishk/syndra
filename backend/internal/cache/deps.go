package cache

import (
	"context"
	"fmt"
	"sort"
	"time"

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
	// team) a claim profile may project into a token. Injectable so compiler
	// tests don't need a live directory.
	cacheFindUser = func(ctx context.Context, userID string) (models.UserProfile, bool, error) {
		return directory.Default.FindUser(ctx, userID)
	}

	// dbGetAllBundles names bundles: a token carries bundle names, not ids, so
	// a rename reaches the app.
	dbGetAllBundles = db.GetAllBundles

	// cacheIsServiceAccount is the one rule (services.IsServiceAccount): a
	// Zitadel machine user, or one an operator marked. Restated here because
	// services imports this package.
	cacheIsServiceAccount = func(ctx context.Context, u models.UserProfile) (bool, error) {
		if u.IsMachine() {
			return true, nil
		}
		rows, err := db.ListServiceAccounts(ctx)
		if err != nil {
			return false, err
		}
		for _, r := range rows {
			if r.UserID == u.ID {
				return true, nil
			}
		}
		return false, nil
	}

	redisGet = func(ctx context.Context, key string) (string, error) {
		return db.Redis.Get(ctx, key).Result()
	}

	// directoryAnswers is whether the directory is answering now: one real
	// read, then its own freshness (a read served from its last good copy
	// leaves it not current).
	directoryAnswers = func(ctx context.Context) bool {
		if _, err := directory.Default.Users(ctx); err != nil {
			return false
		}
		_, current := directory.FreshnessOf(directory.Default)
		return current
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

// bundleNames turns each project's bundle ids into sorted bundle names. Only
// read when the person holds a bundle at all. A bundle missing from the list
// (retired) is left out rather than sent by its id.
func bundleNames(ctx context.Context, byProject map[string]map[string]bool) (map[string][]string, error) {
	if len(byProject) == 0 {
		return nil, nil
	}
	all, err := dbGetAllBundles(ctx)
	if err != nil {
		return nil, fmt.Errorf("bundle names: %w", err)
	}
	names := make(map[string]string, len(all))
	for _, b := range all {
		names[b.ID] = b.Name
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
