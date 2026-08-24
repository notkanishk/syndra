package handlers

import (
	"context"
	"fmt"
	"time"

	"syndra/internal/addons"
	"syndra/internal/auth"
	"syndra/internal/cache"
	"syndra/internal/db"
	"syndra/internal/services"
	"syndra/internal/services/addonop"
	"syndra/internal/services/drift"
	"syndra/internal/services/planapply"
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
	svcDrainAddon                   = propagation.DrainAddon

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
	svcBundleDraft        = services.BundleDraft
	svcListBundleVersions = db.ListBundleVersions
	svcGetRolesForVersion = db.GetRolesForVersion
	svcBundleHolders      = db.GetBundleHoldersByVersion
	// Decoration happens here rather than inside the rehearsal: the apply path
	// runs the same rehearsal under the access lock, and looking a name up in
	// the directory there would hold that lock across a call to Zitadel.
	svcRehearsePublish = func(ctx context.Context, req services.PublishRequest) (services.BulkPlan, services.DraftDiff, error) {
		plan, draft, err := services.RehearseBundlePublish(ctx, req)
		services.DecoratePlan(ctx, &plan)
		return plan, draft, err
	}
	svcPublishBundleVersion = services.PublishBundleVersion
	svcRehearseMoveHolders  = func(ctx context.Context, req services.MoveHoldersRequest) (services.BulkPlan, error) {
		plan, err := services.RehearseMoveHolders(ctx, req)
		services.DecoratePlan(ctx, &plan)
		return plan, err
	}
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
	dbHasExclusion                = func(ctx context.Context, target, u, p, r string) (bool, error) {
		ex, err := db.GetExclusions(ctx, target)
		if err != nil {
			return false, err
		}
		return services.IsExcluded(ex, target, u, p, r), nil
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

	// Shadow Password Vault injectable vars.
	svcRecordCredentialSet     = services.RecordCredentialSet
	svcClearShadowPassword     = services.ClearShadowPassword
	dbHasShadowCredential      = db.HasShadowCredential
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
	svcDriftTriageRows      = services.DriftTriageRows

	// Confirmation-mode surfaces (Task 22): global default read/write, bulk toggle, and
	// Change history.
	dbGetConfigSetting          = db.GetConfigSetting
	dbSetConfigSetting          = db.SetConfigSetting
	dbSetRuleConfirmationMode   = db.SetRuleConfirmationMode
	dbSetBundleConfirmationMode = db.SetBundleConfirmationMode
	dbGetCascadeGroups          = db.GetCascadeGroups

	// Bulk access changes. Rehearsal is its own injectable so the handler's
	// central contract — apply never acts on a diff nobody approved — is
	// assertable without a database.
	svcRehearseBulk     = services.RehearseBulk
	svcUserDirectGrants = services.UserDirectGrants

	// Plan-then-apply. The rehearsal persists what it showed; the apply cites
	// it. Two seams rather than one because they are two halves of the
	// guarantee and a test has to be able to fail either: a rehearsal that
	// records nothing, and an apply that claims without verifying.
	dbCreatePlan        = db.CreatePlan
	dbClaimPlanVerified = db.ClaimPlanVerified

	// Role-to-target mappings (group 7). Validation is split — Syndra checks
	// structure, the add-on checks reference — so the two halves are two seams
	// and a test can fail either.
	dbListRoleMappings       = db.ListRoleMappings
	dbGetRoleMapping         = db.GetRoleMapping
	dbCreateRoleMapping      = db.CreateRoleMapping
	dbUpdateRoleMappingValue = db.UpdateRoleMappingValue
	dbDeleteRoleMapping      = db.DeleteRoleMapping
	dbMappingHolders         = db.MappingHolders
	dbPublishMappingVersion  = db.PublishMappingVersion
	dbRollbackMappingVersion = db.RollbackMappingVersion
	addonsEntitlementSchema  = addons.EntitlementSchema

	// addonsResolvesValue is the add-on's half of mapping validation: whether
	// `lab_makers` names anything on its target. Syndra cannot answer it — it
	// does not know what the value means — so the add-on is asked, through the
	// `/values/{field}` read it now serves.
	//
	// It fails open on everything except a definite "no". A target that could
	// not be read is the absence of an answer, and refusing a mapping edit
	// because a NAS was rebooting would make an outage look like a validation
	// failure. Structure is Syndra's own and is enforced regardless.
	addonsResolvesValue = addons.ResolvesValue

	// Allowances (group 8).
	dbCreateAllowance        = db.CreateAllowance
	dbLiftAllowance          = db.LiftAllowance
	dbAllowancesForSubject   = db.AllowancesForSubject
	dbAllowancesDueForReview = db.AllowancesDueForReview

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

// The entitlement convergence surface (change `addon-platform` groups 7 and 9).
//
// Two seams, not one, because they answer different questions and a test has to
// be able to fail either: what the rehearsal computed, and what the gate did
// with the approval it was cited under.
var (
	svcRehearseEntitlements = services.RehearseEntitlements
	svcApplyEntitlements    = planapply.Apply
)

// The mapping-edit plan path (7.11). `svcInTxLockingAccess` is here rather than
// called directly so a test can assert the edit and its convergences share one
// transaction — the property, not the call.
var (
	svcInTxLockingAccess      = db.InTxLockingAccess
	dbRecordSystemConvergence = db.RecordSystemConvergence
)

// The unmanaged inventory and its one action (1.18/1.19, 6.8).
var (
	svcTargetInventory = drift.Inventory
	// On-demand add-on reconciliation. The scheduler has driven this since it
	// was written; an operator had no way to ask. [Reconcile now] existed for
	// Zitadel and for nothing else, so the answer to "is this target in step?"
	// was "wait up to six hours and read a log line".
	svcReconcileAddon = drift.ReconcileAddon
	// A member's own account state and usage. A read, not an operation: it runs
	// on an ordinary page load, and a durable row per page view would fill the
	// operation log with events that changed nothing.
	addonsMyStorage = addons.MyStorage
	// What the TARGET's own audit log says about a subject. Also a read: an
	// operator opening somebody's record must not append to the operation log
	// for having looked.
	addonsActivity        = addons.Activity
	addonsSystemHealth    = addons.SystemHealth
	svcDispatchOperation  = addonop.Dispatch
	dbRecordTargetBinding = db.RecordTargetBinding
)

// The unconfirmed-revocation surface (2.51, 9.9).
var (
	dbListUnconfirmedRevocations  = db.ListUnconfirmedRevocations
	dbCountUnconfirmedRevocations = db.CountUnconfirmedRevocations
)

// The add-on's runtime lifecycle setter (§18, 15.6).
var addonsSetLifecycle = addons.SetLifecycle

// The target roster (9.13). Seams because the roster is deployment
// configuration and a test of the surface must be able to state a deployment.
var (
	addonsRegistered = addons.Registered
	// Whether each registered target's transport secret still loads, read at
	// request time rather than trusted from start-up.
	addonsTransportCredentials = addons.TransportCredentials
	addonsGet                  = addons.Get
	addonsHealth               = addons.Health
	// How a member reaches a target, for the instructions on their own page.
	addonsConnection = addons.ConnectionFor
	// The backend's memory of an add-on's mutation log, read beside that add-on's
	// own account of itself. Two authorities on purpose.
	svcDormantAccounts    = services.DormantAccounts
	dbListMappingHistory  = db.ListMappingHistory
	dbForgetTargetBinding = db.ForgetTargetBinding
	// The merge base goes with the binding, always. One left behind is a claim
	// about an account nobody manages any more — and if that subject is later
	// bound to a DIFFERENT account, it would be compared against a person it
	// was never about.
	dbForgetMergeBase    = db.ForgetMergeBase
	dbForgetPropagations = db.ForgetPropagations

	// The findings a reconciliation could not resolve, and the operator's answer
	// to one. Separate seams because the assertion that matters spans them: the
	// resolution must be WRITTEN before the finding is closed, and a test proving
	// that has to be able to fail the first and watch the second not happen.
	dbStandingMergeFindings   = db.StandingMergeFindings
	dbGetStandingMergeFinding = db.GetStandingMergeFinding
	dbResolveMergeFinding     = db.ResolveMergeFinding
	dbRecordMergeDecision     = db.RecordMergeDecision
	dbReleaseMergeDecision    = db.ReleaseMergeDecision
	// The roles a subject actually holds, for scoping a policy hint to the
	// mappings that produce THEIR value rather than every mapping on the field.
	svcHeldRoles               = services.HeldRoles
	dbCountMergeFindings       = db.CountStandingMergeFindings
	dbGetLogAnchor             = db.GetLogAnchor
	dbStandingBindingConflicts = db.StandingBindingConflicts
	dbResolveBindingConflict   = db.ResolveBindingConflict
	dbResolveLogViolation      = db.ResolveLogViolation
	// When a member's access to a target was written down, for the one sentence
	// on their own page that would otherwise be a promise about a person.
	dbEntitlementRecordedAt = db.EntitlementRecordedAt
)

// A member's own view of a target (group 10).
var (
	svcResolveEntitlementSet = services.ResolveEntitlements
	dbGetTargetBinding       = db.GetTargetBinding

	// addonsCallable is "has this add-on published a manifest we understand" —
	// registration is a deployment fact and callability is a runtime one, and a
	// member's credential form must be gated on the second.
	addonsCallable = func(target string) bool {
		a, err := addons.Get(target)
		if err != nil {
			return false
		}
		return !a.FetchedAt().IsZero() && !a.CircuitOpen()
	}
)
