// Package drift contains the backend-side scheduler that periodically
// reconciles Zitadel grants against Syndra's expected set (direct grants +
// rule-derived expectations + operator exclusions), flagging unexplained
// Zitadel grants as drift and replaying missed direct-grant propagations.
package drift

import (
	"context"
	"encoding/json"

	"syndra/internal/addons"

	"syndra/internal/db"
	"syndra/internal/models"
	"syndra/internal/services"
	"syndra/internal/zitadel"
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
	svcGetExclusions         = func(ctx context.Context, target string) ([]models.ExternalGrantExclusion, error) {
		return db.GetExclusions(ctx, target)
	}
	upsertDriftItem        = db.UpsertDriftItem          // (ctx,target,user,project,roleKeys,grantID,source,type) (id,inserted,err)
	pendingOutboxAddExists = db.PendingOutboxAddExists   // (ctx,target,user,project,role) (bool,err) — dedupes syndra_only replay
	insertPending          = db.InsertPendingPropagation // re-enqueue path (syndra_only) — Zitadel-shaped by construction

	// Reachability + paginated grant listing. A nil MgmtClient means offline.
	zitadelReachable     = func(ctx context.Context) bool { return zitadel.MgmtClient != nil }
	zitadelListAllGrants = func(ctx context.Context, p zitadel.SearchParams) (*zitadel.SearchResult[zitadel.UserGrant], error) {
		return zitadel.MgmtClient.ListAllGrants(ctx, p)
	}

	// idempotency-key minting for re-enqueued rows: reuse the outbox's crypto/rand
	// helper (the repo has NO uuid module). Returns (string, error); the sweep
	// handles the error.
	newIdempotencyKey = db.NewOutboxIdempotencyKey // () (string, error)

	// How current Syndra's picture of the target is. Written by the sweep
	// itself because the sweep is the only thing that knows whether the read it
	// consumed was one it can stand behind.
	markUnreconciled = db.MarkTargetUnreconciled // (ctx, target, reason)
	markReconciled   = db.MarkTargetReconciled   // (ctx, target)
)

// classification helpers are pure and shared with the reconciliation endpoint.
var (
	buildHolderSet  = services.BuildHolderSet
	expectedViaRule = services.ExpectedViaRule
	isExcluded      = services.IsExcluded
)

// The add-on reconciler's seams (1.18, 1.22).
//
// Separate from the Zitadel sweep's on purpose: they are a different reader with
// different failure modes, and a test has to be able to fail either — a target
// that answered from its mirror and was diffed anyway, an unmanaged account
// entered into triage, a convergence queued for a blocked row.
var (
	addonSubjects = addons.Subjects
	addonPlan     = addons.Plan
	listBindings  = db.ListTargetBindings

	recordConvergence = db.RecordSystemConvergence

	// The mutation-log anchor (2.28). Two seams rather than one, because the
	// read and the comparison fail differently: a health read that did not
	// happen is no evidence, and a comparison that says the chain was trimmed is
	// the strongest evidence this system produces.
	addonHealth   = addons.Health
	anchorLogHead = db.RecordLogHead

	resolveIntent = func(ctx context.Context, subjectID, target string) (map[string]json.RawMessage, error) {
		set, err := services.ResolveEntitlements(ctx, subjectID, target)
		if err != nil {
			return nil, err
		}
		return set.Desired(), nil
	}
)
