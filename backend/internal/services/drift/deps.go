// Package drift contains the backend-side scheduler that periodically
// reconciles Zitadel grants against MkAuth's expected set (direct grants +
// rule-derived expectations + operator exclusions), flagging unexplained
// Zitadel grants as drift and replaying missed direct-grant propagations.
package drift

import (
	"context"

	"mkauth/internal/db"
	"mkauth/internal/models"
	"mkauth/internal/services"
	"mkauth/internal/zitadel"
)

// driftSafetyCap is the same right-sized cap as the on-demand reconciliation
// endpoint (B2): the sweep pages Zitadel grants and stops here.
const driftSafetyCap = 2_000
const zitadelPageSize = 500

// Injectable dependencies. Mirrors the save-swap-restore pattern used across
// the backend (see services/expiry/deps.go). Tests exercise sweep logic
// without a live DB/Zitadel by swapping these.
var (
	svcAllDirectGrants = func(ctx context.Context) ([]models.DirectGrant, error) {
		return db.GetAllDirectGrants(ctx, false) // active grants only — expired grants are not expected in Zitadel
	}
	svcGetActiveMappingRules = db.GetActiveMappingRules
	svcGetExclusions = func(ctx context.Context) ([]models.ExternalGrantExclusion, error) {
		return db.GetExclusions(ctx)
	}
	upsertDriftItem        = db.UpsertDriftItem          // (ctx,user,project,roleKeys,grantID,source,type) (id,inserted,err)
	pendingOutboxAddExists = db.PendingOutboxAddExists   // (ctx,user,project,role) (bool,err) — dedupes mkauth_only replay
	insertPending          = db.InsertPendingPropagation // re-enqueue path (mkauth_only)

	// Reachability + paginated grant listing. A nil MgmtClient means offline.
	zitadelReachable     = func(ctx context.Context) bool { return zitadel.MgmtClient != nil }
	zitadelListAllGrants = func(ctx context.Context, p zitadel.SearchParams) (*zitadel.SearchResult[zitadel.UserGrant], error) {
		return zitadel.MgmtClient.ListAllGrants(ctx, p)
	}

	// idempotency-key minting for re-enqueued rows: reuse the outbox's crypto/rand
	// helper (the repo has NO uuid module). Returns (string, error); the sweep
	// handles the error.
	newIdempotencyKey = db.NewOutboxIdempotencyKey // () (string, error)
)

// classification helpers are pure and shared with the reconciliation endpoint.
var (
	buildHolderSet  = services.BuildHolderSet
	expectedViaRule = services.ExpectedViaRule
	isExcluded      = services.IsExcluded
)
