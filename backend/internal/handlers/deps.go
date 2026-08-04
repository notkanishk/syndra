package handlers

import (
	"context"
	"fmt"
	"time"

	"syndra/internal/auth"
	"syndra/internal/cache"
	"syndra/internal/db"
	"syndra/internal/services"
	"syndra/internal/services/drift"
	"syndra/internal/services/propagation"
	"syndra/internal/zitadel"
)

var (
	// jwtValidate is the test-injectable parse-and-validate entrypoint used by
	// withUserAuth. Tests substitute a counting wrapper to assert the C4
	// contract (parsed exactly once per request — no re-parse in
	// withOperatorAuth).
	jwtValidate = auth.Validate

	dbUpsertDirectGrant = db.UpsertDirectGrant
	dbGetAccessRequests = db.GetAccessRequests

	// Outbox: every Syndra-mediated Zitadel grant mutation flows through the
	// transactional enqueue (ledger+audit+outbox), drained explicitly by the
	// operator. The handlers no longer call Zitadel grant APIs directly (B4/D3).
	dbEnqueueDirectGrantPropagation = db.EnqueueDirectGrantPropagation
	dbApproveRequestAndEnqueue      = db.ApproveRequestAndEnqueue
	dbGetPendingPropagations        = db.GetPendingPropagations
	dbGetPropagationStatus          = db.GetPropagationStatus
	svcDrainPropagations            = propagation.Drain
	svcDrainPropagationRow          = propagation.DrainOne

	dbCreateAccessRequest  = db.CreateAccessRequest
	dbGetAccessRequestByID = db.GetAccessRequestByID
	// Taking back your own ask. Scoped to the requester inside the statement, not just here.
	dbWithdrawAccessRequest = db.WithdrawAccessRequest
	dbResolveAccessRequest  = db.ResolveAccessRequest
	dbInsertAuditLog        = db.InsertAuditLog

	// Bundle handler injectable vars. dbAddRoleToBundle/dbAssignBundleToUser were removed
	// (sub-phase 3, Task 20): the cascade OWNS the source mutation now — see
	// svcCascadeRoleAdded/svcCascadeBundleAssigned below, which call the atomic
	// db.AddRoleToBundleAndEnqueue/db.AssignBundleAndEnqueue instead.
	dbCreateBundle          = db.CreateBundle
	dbUpdateBundle          = db.UpdateBundle
	dbGetBundleByID         = db.GetBundleByID
	dbGetAllBundles         = db.GetAllBundles
	dbGetRolesForBundle     = db.GetRolesForBundle
	dbGetBundlesForUser     = db.GetBundlesForUser
	dbSetWelcomeBundle      = db.SetWelcomeBundle
	dbGetBundleHolderCounts = db.GetBundleHolderCounts
	dbGetAssignedUserCounts = db.GetAssignedUserCounts

	// Lookup handler injectable var (single-role accessor, used for UID→name resolution).
	dbGetRole = db.GetRole

	// Rules handler injectable vars. dbCreateMappingRule was removed (Task 20): the cascade
	// creates the rule via the atomic db.CreateMappingRuleAndEnqueue — see svcCascadeRuleCreated.
	dbGetActiveMappingRules = db.GetActiveMappingRules
	dbDetectCycleOnInsert   = db.DetectCycleOnInsert

	// Add-side cascade injectables (sub-phase 3, Task 20): the cascade OWNS the source
	// mutation (assign/add-role/create happen inside the atomic *AndEnqueue tx).
	svcCascadeBundleAssigned = services.CascadeBundleAssignedToUser // (ctx, actor, userID, bundleID)
	// Working-copy edits. These reach nobody — adding or removing a role
	// changes what the bundle WILL grant when its next version is published,
	// which is the whole point of versioning.
	svcEditBundleWorkingCopy = services.EditBundleWorkingCopy // (ctx, actor, bundleID, projectID, roleKey, add)
	svcCascadeRuleCreated    = services.CascadeRuleCreated    // (ctx, actor, sp, sr, tp, tr, mode) → (ruleID, res, err)

	// Revoke-side + rule-update cascade injectables (sub-phase 3, Task 21): same ownership —
	// remove/update happen inside the atomic *AndEnqueue tx, no separate dbRemove*/dbUpdate*
	// injectable in the handler layer.
	svcCascadeBundleRemoved = services.CascadeBundleRemovedFromUser // (ctx, actor, userID, bundleID)
	// Retiring the bundle itself: the same revoke, computed once per holder.
	svcCascadeBundleDeleted = services.CascadeBundleDeleted // (ctx, actor, bundleID)
	svcCascadeRuleUpdated   = services.CascadeRuleUpdated   // (ctx, actor, old, sp, sr, tp, tr)
	// Retiring a rule is the revoke half of a retarget with no new edge: same closure diff,
	// same one transaction, so the rule and the access it was granting leave together.
	svcCascadeRuleDeleted = services.CascadeRuleDeleted // (ctx, actor, old)

	// Rules handler injectables for the 6th trigger (Task 21f): read the pre-update rule, then
	// validate the retarget against the graph WITHOUT the rule's own old edge.
	dbGetMappingRuleByID  = db.GetMappingRuleByID
	dbDetectCycleOnUpdate = db.DetectCycleOnUpdate

	// Bundle versioning: the draft diff, the version list, and the two rehearsed
	// applies (publish, move holders).
	svcBundleDraft          = services.BundleDraft
	svcListBundleVersions   = db.ListBundleVersions
	svcGetRolesForVersion   = db.GetRolesForVersion
	svcBundleHolders        = db.GetBundleHoldersByVersion
	svcRehearsePublish      = services.RehearseBundlePublish
	svcPublishBundleVersion = services.PublishBundleVersion
	svcRehearseMoveHolders  = services.RehearseMoveHolders
	svcMoveHolders          = services.MoveHolders
	dbGetStaleHolderCounts  = db.GetStaleHolderCounts
	dbGetUserBundleVersions = db.GetUserBundleVersions

	cacheRebuildUser    = cache.RebuildUserCache
	cacheInvalidateUser = cache.InvalidateUser

	// Webhook handler injectable vars.
	webhookEnforceMappingRules             = zitadel.EnforceMappingRules
	webhookRevokeMappingRules              = zitadel.RevokeMappingRules
	webhookTriggerOnboarding               = services.TriggerOnboarding
	dbInsertWebhookEvent                   = db.InsertWebhookEvent
	dbCompleteWebhookEvent                 = db.CompleteWebhookEvent
	dbFailWebhookEvent                     = db.FailWebhookEvent
	dbGetWebhookEvents                     = db.GetWebhookEvents
	dbDropWebhookEventEnrichmentIncomplete = db.DropWebhookEventEnrichmentIncomplete

	// Zitadel grants index (event-listener enrichment cache).
	dbUpsertGrantIndex   = db.UpsertGrantIndex
	dbGetGrantIndex      = db.GetGrantIndex
	dbDeleteGrantIndex   = db.DeleteGrantIndex
	dbListUserGrantsLive = listUserGrantsViaZitadel

	// Real-time webhook drift detection (C6): a surviving grant_added event
	// (already past the self-mutation guard) that Syndra neither expects nor
	// has excluded is out-of-band drift. See detectWebhookDrift in webhook.go.
	dbUpsertDriftItemWithEvidence = db.UpsertDriftItemWithEvidence
	dbHasExclusion                = func(ctx context.Context, u, p, r string) (bool, error) {
		ex, err := db.GetExclusions(ctx)
		if err != nil {
			return false, err
		}
		return services.IsExcluded(ex, u, p, r), nil
	}
	// svcUserExpectsRole reports whether Syndra already expects (project,role)
	// for the user — via direct grant, bundle, or mapping rule. Reuses the
	// existing per-user resolver so the webhook's "is this explained?" check is
	// one function, not a re-implementation.
	svcUserExpectsRole = services.UserExpectsRole

	// Role management injectable vars.
	svcCreateRole        = services.CreateRole
	svcGlobalRoleCatalog = services.GlobalRoleCatalog

	// Reconciliation injectable vars — let tests exercise drift computation
	// without a database or live Zitadel connection. The Zitadel side reuses
	// zitadelListAllGrants below so a mocked MgmtClient flows through here too.
	svcAllDirectGrants = services.AllDirectGrants
	// Rule/exclusion lookups for reconciliation's expected-set filtering (B2).
	// Errors from these MUST propagate as 500s, not degrade to an empty set —
	// an empty set would misclassify rule-derived/excluded grants as drift.
	svcGetActiveMappingRulesRecon = db.GetActiveMappingRules
	svcGetExclusions              = db.GetExclusions

	// Provisioning intent injectable vars.
	webhookEmitProvisioningIntent = services.EmitProvisioningIntent
	dbGetProvisioningIntents      = db.GetProvisioningIntents
	dbClaimPendingIntents         = db.ClaimPendingIntents
	dbCompleteIntent              = db.CompleteIntent
	dbFailIntent                  = db.FailIntent

	// Shadow Password Vault injectable vars.
	svcSetShadowPassword       = services.SetShadowPassword
	svcClearShadowPassword     = services.ClearShadowPassword
	dbHasShadowCredential      = db.HasShadowCredential
	dbGetShadowCredential      = db.GetShadowCredential
	dbGetShadowCredentialAudit = db.GetShadowCredentialAudit

	// Zitadel discovery injectable vars.
	// Each closure checks MgmtClient at call time (not definition time) so the
	// nil guard in discovery handlers and these closures are defense-in-depth.
	errNoClient = fmt.Errorf("zitadel client not initialized")

	zitadelListUsers = func(ctx context.Context, p zitadel.SearchParams) (*zitadel.SearchResult[zitadel.ZitadelUser], error) {
		if zitadel.MgmtClient == nil {
			return nil, errNoClient
		}
		return zitadel.MgmtClient.ListUsers(ctx, p)
	}
	zitadelGetUser = func(ctx context.Context, userID string) (*zitadel.ZitadelUser, error) {
		if zitadel.MgmtClient == nil {
			return nil, errNoClient
		}
		return zitadel.MgmtClient.GetUser(ctx, userID)
	}
	zitadelListProjects = func(ctx context.Context, p zitadel.SearchParams) (*zitadel.SearchResult[zitadel.ZitadelProject], error) {
		if zitadel.MgmtClient == nil {
			return nil, errNoClient
		}
		return zitadel.MgmtClient.ListProjects(ctx, p)
	}
	zitadelListProjectRoles = func(ctx context.Context, projectID string, p zitadel.SearchParams) (*zitadel.SearchResult[zitadel.ProjectRoleResult], error) {
		if zitadel.MgmtClient == nil {
			return nil, errNoClient
		}
		return zitadel.MgmtClient.ListProjectRoles(ctx, projectID, p)
	}
	zitadelAddProjectRole = func(ctx context.Context, projectID, roleKey, displayName, group string) error {
		if zitadel.MgmtClient == nil {
			return errNoClient
		}
		return zitadel.MgmtClient.AddProjectRole(ctx, projectID, roleKey, displayName, group)
	}
	zitadelUpdateProjectRole = func(ctx context.Context, projectID, roleKey, displayName, group string) error {
		if zitadel.MgmtClient == nil {
			return errNoClient
		}
		return zitadel.MgmtClient.UpdateProjectRole(ctx, projectID, roleKey, displayName, group)
	}
	zitadelDeleteProjectRole = func(ctx context.Context, projectID, roleKey string) error {
		if zitadel.MgmtClient == nil {
			return errNoClient
		}
		return zitadel.MgmtClient.DeleteProjectRole(ctx, projectID, roleKey)
	}
	zitadelListAllGrants = func(ctx context.Context, p zitadel.SearchParams) (*zitadel.SearchResult[zitadel.UserGrant], error) {
		if zitadel.MgmtClient == nil {
			return nil, errNoClient
		}
		return zitadel.MgmtClient.ListAllGrants(ctx, p)
	}
	zitadelListUserGrants = func(ctx context.Context, userID string, p zitadel.SearchParams) (*zitadel.SearchResult[zitadel.UserGrant], error) {
		if zitadel.MgmtClient == nil {
			return nil, errNoClient
		}
		return zitadel.MgmtClient.ListUserGrants(ctx, userID, p)
	}
	// Grant CRUD no longer calls Zitadel directly from the handler layer — all
	// add/replace/revoke mutations flow through dbEnqueueDirectGrantPropagation
	// and the operator-triggered drain (services/propagation), which owns the
	// Zitadel grant-API closures. Removing the handler-side closures makes the
	// B4/D3 single-mutation-authority boundary structural, not just conventional.

	// Data-plane injectable vars — used by HandleActionInject and degradedResponse.
	// Separate from the control-plane vars above so tests can exercise the degraded
	// paths without a live Redis instance or database connection.
	redisGetClaims = func(ctx context.Context, key string) (string, error) {
		return db.Redis.Get(ctx, key).Result()
	}
	dbGetClaimFailureMode = db.GetClaimFailureMode

	// Read-through cache for the per-project claim_failure_mode (audit ref C5).
	// Layered in front of dbGetClaimFailureMode so a transient DB outage cannot
	// collapse degraded-mode behaviour into fail_closed for projects whose
	// operator configured minimal_safe.
	redisGetClaimMode = func(ctx context.Context, projectID string) (string, error) {
		return db.Redis.Get(ctx, "claim_mode:"+projectID).Result()
	}
	redisSetClaimMode = func(ctx context.Context, projectID, value string, ttlSeconds int) error {
		return db.Redis.SetEx(ctx, "claim_mode:"+projectID, value, time.Duration(ttlSeconds)*time.Second).Err()
	}

	// Claim shaping. dbResolveClaimProfiles is the data plane's read of the
	// operator-configured token shape; the Redis pair in front of it keeps that
	// read off the token hot path, and redisDelKeys drops the cached shape the
	// moment an operator saves so an edit is never one TTL late.
	dbResolveClaimProfiles = services.ResolveClaimProfiles
	redisGetKey            = func(ctx context.Context, key string) (string, error) {
		return db.Redis.Get(ctx, key).Result()
	}
	redisSetKey = func(ctx context.Context, key, value string, ttlSeconds int) error {
		return db.Redis.SetEx(ctx, key, value, time.Duration(ttlSeconds)*time.Second).Err()
	}
	redisDelKeys = func(ctx context.Context, keys ...string) error {
		return db.Redis.Del(ctx, keys...).Err()
	}

	// Drift triage injectable vars (B2). The three action helpers are atomic
	// claim+side-effect transactions (db.*AndEnqueue / db.MarkDriftExternalTx) —
	// the drift handlers never resolve a drift row outside that transaction.
	dbGetDriftItems         = db.GetDriftItems
	dbGetDriftItem          = db.GetDriftItem
	dbAttributeDriftTx      = db.AttributeDriftTx
	dbRevokeDriftAndEnqueue = db.RevokeDriftAndEnqueue
	dbMarkDriftExternalTx   = db.MarkDriftExternalTx
	svcDriftSweep           = drift.Sweep
	svcDrainOne             = propagation.DrainOne
	svcDriftTriageQueue     = services.DriftTriageQueue

	// Confirmation-mode surfaces (Task 22): global default read/write, bulk toggle, and
	// Change history.
	dbGetConfigSetting          = db.GetConfigSetting
	dbSetConfigSetting          = db.SetConfigSetting
	dbSetRuleConfirmationMode   = db.SetRuleConfirmationMode
	dbSetBundleConfirmationMode = db.SetBundleConfirmationMode
	dbGetCascadeGroups          = db.GetCascadeGroups

	// Bulk access changes. Rehearsal is its own injectable so the handler's
	// central contract — apply NEVER trusts a client-supplied plan, it
	// re-rehearses server-side first — is assertable without a database.
	svcRehearseBulk     = services.RehearseBulk
	svcUserDirectGrants = services.UserDirectGrants

	// Review › Expiring access reads its own window rather than a slice of the
	// governance summary, so a 30-day review and Today's 14-day queue can
	// differ without either lying about the other.
	dbGetExpiringDirectGrants = db.GetExpiringDirectGrants

	// The same window, plus the acknowledgement that currently applies to each row. Separate from
	// the read above because that one serves four callers with no use for one.
	dbGetExpiringWithAcks             = db.GetExpiringDirectGrantsWithAcknowledgements
	dbAcknowledgeGrantExpiry          = db.AcknowledgeGrantExpiry
	dbClearGrantExpiryAcknowledgement = db.ClearGrantExpiryAcknowledgement
)
