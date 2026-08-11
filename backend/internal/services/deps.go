package services

import (
	"context"

	"syndra/internal/addons"
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
	svcEffectiveRoleRefs  = effectiveRoleRefs
	dbMappingsForRoles    = db.MappingsForRoles
	dbAllowancesInForce   = db.AllowancesInForce
	dbAllowancesOnTargets = db.AllowancesInForceOnTargets
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

	// Shadow Password Vault
	svcRecordCredentialSet         = db.RecordCredentialSet
	svcDeleteShadowCredential      = db.DeleteShadowCredential
	svcHasShadowCredential         = db.HasShadowCredential
	svcInsertShadowCredentialAudit = db.InsertShadowCredentialAudit
)

// The add-on read leg the entitlement rehearsal asks its question through.
//
// A seam because every test of that rehearsal is a test about what the backend
// does with an answer — provisional when the read was the mirror, blocked when
// a subject went unanswered — and none of them should need an add-on process to
// produce one.
var addonsPlan = addons.Plan

// The lifecycle trigger's two reads and its one write (7.9).
//
// Seams because the trigger's whole content is a decision — does this role reach
// a target, and what should the subject hold there — and a test of that decision
// must not need a database or an add-on to make it.
var (
	dbTargetsMappedToRole     = db.TargetsMappedToRole
	dbRecordSystemConvergence = db.RecordSystemConvergence
	svcResolveEntitlements    = ResolveEntitlements
)

// The unconfirmed-revocation count behind the governance badge (2.51, 9.16).
var svcCountUnconfirmedRevocations = db.CountUnconfirmedRevocations

// Holds past their review date. Counted for the badge beside expiring access,
// and deliberately not merged with it — see Indicators.HoldsDue.
var svcAllowancesDueForReview = db.AllowancesDueForReview

// The dormant listing's four reads (change `addon-platform` 9.11; design §29).
//
// Separate seams rather than one, because each answers a different question and
// a test of this surface has to be able to make one of them lie: an account on
// the target, a binding for it, what the subject resolves to, and whether the
// subject is still a member at all. The last one is what separates housekeeping
// from a lockout, and it is the only field the surface refuses to act on.
var (
	dormantSubjects = addons.Subjects
	dormantBindings = db.ListTargetBindings
	dormantResolve  = ResolveEntitlements

	// dormantSubjectStatus is the display name and whether they are still a
	// member. A miss is NOT an error: somebody who has left the makerspace
	// resolves to nothing, and that is the answer rather than a failure.
	dormantSubjectStatus = func(ctx context.Context, subjectID string) (name string, stillMember bool) {
		profile, found, err := directory.Default.FindUser(ctx, subjectID)
		if err != nil || !found {
			return "", false
		}
		return profile.Name, true
	}

	// dormantSubjectRoles is what they hold anywhere, to tell "no roles at all"
	// from "roles that no longer reach here".
	dormantSubjectRoles = func(ctx context.Context, subjectID string) ([]models.DirectGrant, error) {
		return db.GetDirectGrantsForUser(ctx, subjectID, false)
	}

	// dormantTargetMapped reports whether anything maps to this target at all.
	dormantTargetMapped = func(ctx context.Context, target string) (bool, error) {
		mappings, err := db.ListRoleMappings(ctx, target)
		return len(mappings) > 0, err
	}
)
