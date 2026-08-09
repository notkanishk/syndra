package services

import (
	"context"

	"syndra/internal/db"
	"syndra/internal/directory"
	"syndra/internal/models"
	"syndra/internal/zitadel"
)

// Injectable function vars — tests swap these to exercise services without a
// live database. Direct references to the db funcs; only vars that add logic
// carry a closure.
var (
	// Onboarding
	svcInsertOnboardingTrigger   = db.InsertOnboardingTrigger
	svcGetWelcomeBundle          = db.GetWelcomeBundle
	svcCascadeWelcomeBundle      = CascadeBundleAssignedToUser
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
	// Whole pending queue, unfiltered. Shared by the triage view and the
	// People index's "1 unexplained" column so the two can never disagree.
	svcGetPendingDriftItems = func(ctx context.Context) ([]models.DriftItem, error) {
		return db.GetDriftItems(ctx, db.DriftFilter{})
	}
	svcGetTopDrift = func(ctx context.Context, n int) ([]models.DriftItem, error) {
		items, err := db.GetDriftItems(ctx, db.DriftFilter{})
		if err != nil || len(items) <= n {
			return items, err
		}
		return items[:n], nil
	}
	svcGetBundlesForUser = db.GetBundlesForUser
	svcGetRolesForBundle = db.GetRolesForBundle
	// What a bundle grants TODAY: the latest published version, which is what a
	// new assignment pins to.
	svcLatestVersionRoles     = db.LatestVersionRoles
	svcVersionBelongsTo       = db.VersionBelongsTo
	svcGetDirectGrantsForUser = db.GetDirectGrantsForUser

	// Entitlement resolution (design §4, §6). Three seams, because the
	// resolver's whole content is how the three answers combine and a test has
	// to be able to move each one independently.
	svcEffectiveRoleRefs = effectiveRoleRefs
	dbMappingsForRoles   = db.MappingsForRoles
	dbAllowancesInForce  = db.AllowancesInForce
	// The lineage band reads the WHOLE history, not only what is in force: a
	// suspension that ended is part of the answer to what has been decided.
	svcAllowancesForSubject = db.AllowancesForSubject
	svcGetAllDirectGrants   = db.GetAllDirectGrants
	// Direct-grant removal: ledger delete + audit + the caller-computed
	// effective-access delta, in one transaction.
	svcDeleteDirectGrantAndEnqueue        = db.DeleteDirectGrantAndEnqueue
	svcDeleteExpiredDirectGrantAndEnqueue = db.DeleteExpiredDirectGrantAndEnqueue
	svcInTxLockingAccess                  = db.InTxLockingAccess
	svcGetActiveMappingRules              = db.GetActiveMappingRules
	// Queued revocations are decisions already taken; every effective-access
	// read subtracts them so a delta cannot be computed from a ledger row that
	// is on its way out.
	svcQueuedRevocations = db.QueuedRevocations

	// Role management
	svcDbCreateRole               = db.CreateRole
	svcDbGetRole                  = db.GetRole
	svcDbDeleteRole               = db.DeleteRole
	svcDbGetAllLocalRoles         = db.GetAllLocalRoles
	svcDbGetRoleUsageCounts       = db.GetRoleUsageCounts
	svcDbGetAssignedUserCounts    = db.GetAssignedUserCounts
	svcDbGetAllReferencedRoleKeys = db.GetAllReferencedRoleKeys

	// Claim shaping (token format + per-application overrides).
	svcGetClaimProfile        = db.GetClaimProfile
	svcListClaimProfiles      = db.ListClaimProfiles
	svcUpsertClaimProfile     = db.UpsertClaimProfile
	svcListAppClaimOverrides  = db.ListAppClaimOverrides
	svcUpsertAppClaimOverride = db.UpsertAppClaimOverride
	svcDeleteAppClaimOverride = db.DeleteAppClaimOverride

	// Directory lookup used by drift triage to tell a departed alumnus from an
	// active member and a person from a service account.
	directoryFindUser = func(ctx context.Context, id string) (models.UserProfile, bool, error) {
		return directory.Default.FindUser(ctx, id)
	}

	// Provisioning intents
	svcInsertProvisioningIntent = db.InsertProvisioningIntent

	// Shadow Password Vault
	svcUpsertShadowCredential      = db.UpsertShadowCredential
	svcDeleteShadowCredential      = db.DeleteShadowCredential
	svcHasShadowCredential         = db.HasShadowCredential
	svcInsertShadowCredentialAudit = db.InsertShadowCredentialAudit
)
