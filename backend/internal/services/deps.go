package services

import (
	"context"

	"mkauth/internal/db"
	"mkauth/internal/models"
	"mkauth/internal/zitadel"
)

// Injectable function vars — tests swap these to exercise services without a
// live database. Direct references to the db funcs; only vars that add logic
// carry a closure.
var (
	// Onboarding
	svcInsertOnboardingTrigger   = db.InsertOnboardingTrigger
	svcGetWelcomeBundle          = db.GetWelcomeBundle
	svcAssignBundleToUser        = db.AssignBundleToUser
	svcInsertAuditLog            = db.InsertAuditLog
	svcCompleteOnboardingTrigger = db.CompleteOnboardingTrigger
	svcFailOnboardingTrigger     = db.FailOnboardingTrigger

	// Governance and lineage (Governance, ExplainUserAccess, BundleImpact,
	// collectUserRoles)
	svcGetAccessRequests       = db.GetAccessRequests
	svcGetExpiringDirectGrants = db.GetExpiringDirectGrants
	svcGetAllBundles           = db.GetAllBundles

	// Pending-propagation summary. Reachability is intentionally the cheap
	// "client configured" signal (MgmtClient != nil) rather than a live ping —
	// the governance summary is fetched on every dashboard load, so a per-load
	// round-trip to Zitadel would be wasteful. In local-policy-only mode the
	// client is nil → reachable=false, which correctly disables "Resume now".
	svcCountPendingPropagations = db.CountPendingPropagations
	svcZitadelReachable         = func(ctx context.Context) bool {
		return zitadel.MgmtClient != nil
	}

	// Drift summary (B2): pending-triage count + a top-N preview for the
	// dashboard callout. svcGetTopDrift reuses GetDriftItems' default
	// pending_triage filter rather than a dedicated top-N query — the operator's
	// drift queue is expected to stay small enough that fetching-then-slicing is
	// cheap; revisit if the pending queue routinely grows large.
	svcCountPendingDrift = db.CountPendingDrift
	svcGetTopDrift       = func(ctx context.Context, n int) ([]models.DriftItem, error) {
		items, err := db.GetDriftItems(ctx, db.DriftFilter{})
		if err != nil || len(items) <= n {
			return items, err
		}
		return items[:n], nil
	}
	svcGetBundlesForUser      = db.GetBundlesForUser
	svcGetRolesForBundle      = db.GetRolesForBundle
	svcGetDirectGrantsForUser = db.GetDirectGrantsForUser
	svcGetAllDirectGrants     = db.GetAllDirectGrants
	svcGetActiveMappingRules  = db.GetActiveMappingRules

	// Role management
	svcDbCreateRole               = db.CreateRole
	svcDbGetRole                  = db.GetRole
	svcDbDeleteRole               = db.DeleteRole
	svcDbGetAllLocalRoles         = db.GetAllLocalRoles
	svcDbGetRoleUsageCounts       = db.GetRoleUsageCounts
	svcDbGetAssignedUserCounts    = db.GetAssignedUserCounts
	svcDbGetAllReferencedRoleKeys = db.GetAllReferencedRoleKeys

	// Provisioning intents
	svcInsertProvisioningIntent = db.InsertProvisioningIntent

	// Shadow Password Vault
	svcUpsertShadowCredential      = db.UpsertShadowCredential
	svcDeleteShadowCredential      = db.DeleteShadowCredential
	svcHasShadowCredential         = db.HasShadowCredential
	svcInsertShadowCredentialAudit = db.InsertShadowCredentialAudit
)
