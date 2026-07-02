package handlers

import (
	"context"
	"fmt"
	"time"

	"mkauth/internal/auth"
	"mkauth/internal/cache"
	"mkauth/internal/db"
	"mkauth/internal/services"
	"mkauth/internal/services/propagation"
	"mkauth/internal/zitadel"
)

var (
	// jwtValidate is the test-injectable parse-and-validate entrypoint used by
	// withUserAuth. Tests substitute a counting wrapper to assert the C4
	// contract (parsed exactly once per request — no re-parse in
	// withOperatorAuth).
	jwtValidate = auth.Validate

	dbUpsertDirectGrant = db.UpsertDirectGrant
	dbGetAccessRequests = db.GetAccessRequests

	// Outbox: every MkAuth-mediated Zitadel grant mutation flows through the
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
	dbResolveAccessRequest = db.ResolveAccessRequest
	dbInsertAuditLog       = db.InsertAuditLog

	// Bundle handler injectable vars.
	dbCreateBundle       = db.CreateBundle
	dbGetAllBundles      = db.GetAllBundles
	dbGetRolesForBundle  = db.GetRolesForBundle
	dbAddRoleToBundle    = db.AddRoleToBundle
	dbGetBundlesForUser  = db.GetBundlesForUser
	dbAssignBundleToUser = db.AssignBundleToUser
	dbSetWelcomeBundle   = db.SetWelcomeBundle

	// Lookup handler injectable var (single-role accessor, used for UID→name resolution).
	dbGetRole = db.GetRole

	// Rules handler injectable vars.
	dbGetActiveMappingRules = db.GetActiveMappingRules
	dbCreateMappingRule     = db.CreateMappingRule
	dbDetectCycleOnInsert   = db.DetectCycleOnInsert

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

	// Role management injectable vars.
	svcCreateRole        = services.CreateRole
	svcGlobalRoleCatalog = services.GlobalRoleCatalog

	// Reconciliation injectable vars — let tests exercise drift computation
	// without a database or live Zitadel connection. The Zitadel side reuses
	// zitadelListAllGrants below so a mocked MgmtClient flows through here too.
	svcAllDirectGrants = services.AllDirectGrants

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
)
