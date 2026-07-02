package services

import (
	"context"
	"time"

	"mkauth/internal/db"
	"mkauth/internal/models"
	"mkauth/internal/zitadel"
)

// Onboarding DB injectable vars — allows tests to exercise TriggerOnboarding
// without a live database connection.
var (
	svcInsertOnboardingTrigger = func(ctx context.Context, userID, source, key string) (string, bool, error) {
		return db.InsertOnboardingTrigger(ctx, userID, source, key)
	}
	svcGetWelcomeBundle = func(ctx context.Context) (string, error) {
		return db.GetWelcomeBundle(ctx)
	}
	svcAssignBundleToUser = func(ctx context.Context, userID, bundleID string) error {
		return db.AssignBundleToUser(ctx, userID, bundleID)
	}
	svcInsertAuditLog = func(ctx context.Context, actorID, targetID, action, resourceID string) error {
		return db.InsertAuditLog(ctx, actorID, targetID, action, resourceID)
	}
	svcCompleteOnboardingTrigger = func(ctx context.Context, triggerID, bundleID string) error {
		return db.CompleteOnboardingTrigger(ctx, triggerID, bundleID)
	}
	svcFailOnboardingTrigger = func(ctx context.Context, triggerID, errMsg string) error {
		return db.FailOnboardingTrigger(ctx, triggerID, errMsg)
	}

	// Governance and lineage injectable vars — allows tests to exercise
	// Governance, ExplainUserAccess, BundleImpact, and collectUserRoles
	// without a live database connection.
	svcGetAccessRequests = func(ctx context.Context, status string) ([]models.AccessRequest, error) {
		return db.GetAccessRequests(ctx, status)
	}
	svcGetExpiringDirectGrants = func(ctx context.Context, within time.Duration) ([]models.DirectGrant, error) {
		return db.GetExpiringDirectGrants(ctx, within)
	}
	svcGetAllBundles = func(ctx context.Context) ([]models.Bundle, error) {
		return db.GetAllBundles(ctx)
	}

	// Pending-propagation summary injectables. Reachability is intentionally the
	// cheap "client configured" signal (MgmtClient != nil) rather than a live
	// ping — the governance summary is fetched on every dashboard load, so a
	// per-load round-trip to Zitadel would be wasteful. In local-policy-only mode
	// the client is nil → reachable=false, which correctly disables "Resume now".
	svcCountPendingPropagations = func(ctx context.Context) (int, error) {
		return db.CountPendingPropagations(ctx)
	}
	svcZitadelReachable = func(ctx context.Context) bool {
		return zitadel.MgmtClient != nil
	}
	svcGetBundlesForUser = func(ctx context.Context, userID string) ([]models.Bundle, error) {
		return db.GetBundlesForUser(ctx, userID)
	}
	svcGetRolesForBundle = func(ctx context.Context, bundleID string) ([]models.BundleRole, error) {
		return db.GetRolesForBundle(ctx, bundleID)
	}
	svcGetDirectGrantsForUser = func(ctx context.Context, userID string, includeExpired bool) ([]models.DirectGrant, error) {
		return db.GetDirectGrantsForUser(ctx, userID, includeExpired)
	}
	svcGetAllDirectGrants = func(ctx context.Context, includeExpired bool) ([]models.DirectGrant, error) {
		return db.GetAllDirectGrants(ctx, includeExpired)
	}
	svcGetActiveMappingRules = func(ctx context.Context) ([]models.MappingRule, error) {
		return db.GetActiveMappingRules(ctx)
	}

	// Role management injectable vars.
	svcDbCreateRole = func(ctx context.Context, projectID, roleKey, displayName, description, group, createdBy string,
		clonedFromProject, clonedFromRole *string) (string, error) {
		return db.CreateRole(ctx, projectID, roleKey, displayName, description, group, createdBy, clonedFromProject, clonedFromRole)
	}
	svcDbGetRole = func(ctx context.Context, projectID, roleKey string) (models.Role, error) {
		return db.GetRole(ctx, projectID, roleKey)
	}
	svcDbDeleteRole = func(ctx context.Context, id string) error {
		return db.DeleteRole(ctx, id)
	}
	svcDbGetAllLocalRoles = func(ctx context.Context) ([]models.Role, error) {
		return db.GetAllLocalRoles(ctx)
	}
	svcDbGetRoleUsageCounts = func(ctx context.Context) (map[string]db.RoleUsage, error) {
		return db.GetRoleUsageCounts(ctx)
	}
	svcDbGetAssignedUserCounts = func(ctx context.Context) (map[string]int, error) {
		return db.GetAssignedUserCounts(ctx)
	}
	svcDbGetAllReferencedRoleKeys = func(ctx context.Context) ([][2]string, error) {
		return db.GetAllReferencedRoleKeys(ctx)
	}

	// Provisioning intent injectable vars.
	svcInsertProvisioningIntent = func(ctx context.Context, targetUID, action, lldapGroup,
		sourceProject, sourceRole, webhookEventID, idempotencyKey string) (string, bool, error) {
		return db.InsertProvisioningIntent(ctx, targetUID, action, lldapGroup,
			sourceProject, sourceRole, webhookEventID, idempotencyKey)
	}

	// Shadow Password Vault injectable vars.
	svcUpsertShadowCredential = func(ctx context.Context, userID, hash, algorithm, saltParams string) (string, error) {
		return db.UpsertShadowCredential(ctx, userID, hash, algorithm, saltParams)
	}
	svcDeleteShadowCredential = func(ctx context.Context, userID string) error {
		return db.DeleteShadowCredential(ctx, userID)
	}
	svcHasShadowCredential = func(ctx context.Context, userID string) (models.ShadowCredentialStatus, error) {
		return db.HasShadowCredential(ctx, userID)
	}
	svcInsertShadowCredentialAudit = func(ctx context.Context, userID, action, actorID, ipAddress string) error {
		return db.InsertShadowCredentialAudit(ctx, userID, action, actorID, ipAddress)
	}
)
