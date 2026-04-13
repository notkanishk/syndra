package handlers

import (
	"context"

	"mkauth/internal/cache"
	"mkauth/internal/db"
)

var (
	dbUpsertDirectGrant    = db.UpsertDirectGrant
	dbGetAccessRequests    = db.GetAccessRequests
	dbCreateAccessRequest  = db.CreateAccessRequest
	dbGetAccessRequestByID = db.GetAccessRequestByID
	dbResolveAccessRequest = db.ResolveAccessRequest
	dbInsertAuditLog       = db.InsertAuditLog

	cacheRebuildUser = cache.RebuildUserCache

	// Data-plane injectable vars — used by HandleActionInject and degradedResponse.
	// Separate from the control-plane vars above so tests can exercise the degraded
	// paths without a live Redis instance or database connection.
	redisGetClaims       = func(ctx context.Context, key string) (string, error) {
		return db.Redis.Get(ctx, key).Result()
	}
	dbGetClaimFailureMode = db.GetClaimFailureMode
)
