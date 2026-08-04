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
	"syndra/internal/cache"
	"syndra/internal/db"
	"syndra/internal/services"
	"syndra/internal/zitadel"
)

// Injectable dependencies. Mirrors the save-swap-restore pattern used across
// the backend (see services/deps.go, cache/deps.go, zitadel/deps.go). Tests
// exercise sweep logic without a live DB/Redis/Zitadel by swapping these.
var (
	svcGetExpiredDirectGrants         = db.GetExpiredDirectGrants
	svcDeleteExpiredDirectGrantsByIDs = db.DeleteExpiredDirectGrantsByIDs
	svcEmitIntentFromScheduler        = services.EmitProvisioningIntentFromScheduler
	svcInsertAuditLog                 = db.InsertAuditLog
	cacheInvalidateUser               = cache.InvalidateUser
	zitadelRevokeMappingRules         = zitadel.RevokeMappingRules
)
