package handlers

import (
	"context"

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

	// Data-plane injectable vars — used by HandleActionInject and degradedResponse.
	// Separate from the control-plane vars above so tests can exercise the degraded
	// paths without a live Redis instance or database connection.
	redisGetClaims       = func(ctx context.Context, key string) (string, error) {
		return db.Redis.Get(ctx, key).Result()
	}
	dbGetClaimFailureMode = db.GetClaimFailureMode
)
