// Package expiry contains the backend-side scheduler that sweeps expired
// direct role grants and performs the end-to-end cleanup side effects
// (LLDAP provisioning intent, hard-delete, cache invalidation, audit, and
// best-effort Zitadel derived-grant cascade).
//
// Effective access for expired grants is already correct at query time via
// GetDirectGrantsForUser(..., includeExpired=false). This package closes the
// *cleanup* gap: without it, expired rows linger in the DB, LLDAP groups keep
// stale members, and Zitadel derived grants remain authoritative-looking.
package expiry

import (
	"context"
	"time"

	"mkauth/internal/cache"
	"mkauth/internal/db"
	"mkauth/internal/models"
	"mkauth/internal/services"
	"mkauth/internal/zitadel"
)

// Injectable dependencies. Mirrors the save-swap-restore pattern used across
// the backend (see services/deps.go, cache/deps.go, zitadel/deps.go). Tests
// exercise sweep logic without a live DB/Redis/Zitadel by swapping these.
var (
	svcGetExpiredDirectGrants = func(ctx context.Context, limit int) ([]models.DirectGrant, error) {
		return db.GetExpiredDirectGrants(ctx, limit)
	}
	svcDeleteExpiredDirectGrantsByIDs = func(ctx context.Context, userID string, ids []string) ([]models.DirectGrant, error) {
		return db.DeleteExpiredDirectGrantsByIDs(ctx, userID, ids)
	}
	svcEmitIntentFromScheduler = func(ctx context.Context, targetUID, action, projectID, roleKey, grantID string) error {
		return services.EmitProvisioningIntentFromScheduler(ctx, targetUID, action, projectID, roleKey, grantID)
	}
	svcInsertAuditLog = func(ctx context.Context, actorID, targetID, action, resourceID string) error {
		return db.InsertAuditLog(ctx, actorID, targetID, action, resourceID)
	}
	cacheInvalidateUser = func(ctx context.Context, userID string) error {
		return cache.InvalidateUser(ctx, userID)
	}
	zitadelRevokeMappingRules = func(ctx context.Context, userID, projectID, roleKey string) error {
		return zitadel.RevokeMappingRules(ctx, userID, projectID, roleKey)
	}

	timeNow = time.Now
)
