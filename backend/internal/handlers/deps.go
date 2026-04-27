package handlers

import (
	"context"
	"fmt"

	"mkauth/internal/cache"
	"mkauth/internal/db"
	"mkauth/internal/services"
	"mkauth/internal/zitadel"
)

var (
	dbUpsertDirectGrant    = db.UpsertDirectGrant
	dbGetAccessRequests    = db.GetAccessRequests
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

	// Lookup handler injectable var (single-role accessor, used for UID→name resolution).
	dbGetRole = db.GetRole

	// Rules handler injectable vars.
	dbGetActiveMappingRules = db.GetActiveMappingRules
	dbCreateMappingRule     = db.CreateMappingRule
	dbUpdateMappingRule     = db.UpdateMappingRule
	dbDetectCycleOnInsert   = db.DetectCycleOnInsert

	cacheRebuildUser    = cache.RebuildUserCache
	cacheInvalidateUser = cache.InvalidateUser

	// Webhook handler injectable vars.
	webhookEnforceMappingRules = zitadel.EnforceMappingRules
	webhookRevokeMappingRules  = zitadel.RevokeMappingRules
	webhookTriggerOnboarding   = services.TriggerOnboarding
	dbInsertWebhookEvent       = db.InsertWebhookEvent
	dbCompleteWebhookEvent     = db.CompleteWebhookEvent
	dbFailWebhookEvent         = db.FailWebhookEvent
	dbGetWebhookEvents         = db.GetWebhookEvents

	// Role management injectable vars.
	svcCreateRole        = services.CreateRole
	svcGlobalRoleCatalog = services.GlobalRoleCatalog

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
		if zitadel.MgmtClient == nil { return nil, errNoClient }
		return zitadel.MgmtClient.ListUsers(ctx, p)
	}
	zitadelGetUser = func(ctx context.Context, userID string) (*zitadel.ZitadelUser, error) {
		if zitadel.MgmtClient == nil { return nil, errNoClient }
		return zitadel.MgmtClient.GetUser(ctx, userID)
	}
	zitadelListProjects = func(ctx context.Context, p zitadel.SearchParams) (*zitadel.SearchResult[zitadel.ZitadelProject], error) {
		if zitadel.MgmtClient == nil { return nil, errNoClient }
		return zitadel.MgmtClient.ListProjects(ctx, p)
	}
	zitadelListProjectRoles = func(ctx context.Context, projectID string, p zitadel.SearchParams) (*zitadel.SearchResult[zitadel.ProjectRoleResult], error) {
		if zitadel.MgmtClient == nil { return nil, errNoClient }
		return zitadel.MgmtClient.ListProjectRoles(ctx, projectID, p)
	}
	zitadelAddProjectRole = func(ctx context.Context, projectID, roleKey, displayName, group string) error {
		if zitadel.MgmtClient == nil { return errNoClient }
		return zitadel.MgmtClient.AddProjectRole(ctx, projectID, roleKey, displayName, group)
	}
	zitadelUpdateProjectRole = func(ctx context.Context, projectID, roleKey, displayName, group string) error {
		if zitadel.MgmtClient == nil { return errNoClient }
		return zitadel.MgmtClient.UpdateProjectRole(ctx, projectID, roleKey, displayName, group)
	}
	zitadelDeleteProjectRole = func(ctx context.Context, projectID, roleKey string) error {
		if zitadel.MgmtClient == nil { return errNoClient }
		return zitadel.MgmtClient.DeleteProjectRole(ctx, projectID, roleKey)
	}
	zitadelListAllGrants = func(ctx context.Context, p zitadel.SearchParams) (*zitadel.SearchResult[zitadel.UserGrant], error) {
		if zitadel.MgmtClient == nil { return nil, errNoClient }
		return zitadel.MgmtClient.ListAllGrants(ctx, p)
	}
	zitadelListUserGrants = func(ctx context.Context, userID string, p zitadel.SearchParams) (*zitadel.SearchResult[zitadel.UserGrant], error) {
		if zitadel.MgmtClient == nil { return nil, errNoClient }
		return zitadel.MgmtClient.ListUserGrants(ctx, userID, p)
	}
	zitadelAddUserGrant = func(ctx context.Context, userID, projectID string, roleKeys []string) error {
		if zitadel.MgmtClient == nil { return errNoClient }
		return zitadel.MgmtClient.AddUserGrant(ctx, userID, projectID, roleKeys)
	}
	zitadelUpdateUserGrant = func(ctx context.Context, userID, grantID string, roleKeys []string) error {
		if zitadel.MgmtClient == nil { return errNoClient }
		return zitadel.MgmtClient.UpdateUserGrant(ctx, userID, grantID, roleKeys)
	}
	zitadelRemoveUserGrant = func(ctx context.Context, userID, grantID string) error {
		if zitadel.MgmtClient == nil { return errNoClient }
		return zitadel.MgmtClient.RemoveUserGrant(ctx, userID, grantID)
	}

	// Data-plane injectable vars — used by HandleActionInject and degradedResponse.
	// Separate from the control-plane vars above so tests can exercise the degraded
	// paths without a live Redis instance or database connection.
	redisGetClaims       = func(ctx context.Context, key string) (string, error) {
		return db.Redis.Get(ctx, key).Result()
	}
	dbGetClaimFailureMode = db.GetClaimFailureMode
)
