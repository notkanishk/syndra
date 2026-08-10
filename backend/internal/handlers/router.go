package handlers

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"runtime/debug"
	"strings"
)

// NewRouter constructs the global multiplexer for API requests
func NewRouter() http.Handler {
	mux := http.NewServeMux()

	// Health check — no auth, no CORS
	mux.HandleFunc("GET /healthz", handleHealthCheck)

	// System mode diagnostic — auth-gated to avoid leaking deployment posture.
	// UI consumes this to render a "Live"/"Demo"/"Degraded" indicator.
	mux.HandleFunc("GET /api/v1/system/mode", withCORS(withUserAuth(handleSystemMode)))

	// Bundle Routes
	mux.HandleFunc("GET /api/v1/bundles", withCORS(withUserAuth(handleGetBundles)))
	mux.HandleFunc("POST /api/v1/bundles", withCORS(withOperatorAuth(handleCreateBundle)))
	// Renaming is not versioned and reaches nobody. Deleting revokes what only this bundle was
	// granting, per holder, in the transaction that removes it.
	mux.HandleFunc("PUT /api/v1/bundles/{id}", withCORS(withOperatorAuth(handleUpdateBundle)))
	mux.HandleFunc("DELETE /api/v1/bundles/{id}", withCORS(withOperatorAuth(handleDeleteBundle)))
	mux.HandleFunc("GET /api/v1/bundles/{id}/roles", withCORS(withUserAuth(handleGetBundleRoles)))

	// Bundle versioning. Editing a bundle changes its working copy and reaches
	// nobody; publishing a version is the rehearsed step that can move holders.
	mux.HandleFunc("GET /api/v1/bundles/{id}/versions", withCORS(withUserAuth(handleGetBundleVersions)))
	mux.HandleFunc("GET /api/v1/bundles/{id}/holders", withCORS(withOperatorAuth(handleGetBundleHolders)))
	mux.HandleFunc("GET /api/v1/bundles/{id}/draft", withCORS(withOperatorAuth(handleGetBundleDraft)))
	mux.HandleFunc("POST /api/v1/bundles/{id}/publish", withCORS(withOperatorAuth(handlePublishBundleVersion)))
	mux.HandleFunc("POST /api/v1/bundles/{id}/holders/move", withCORS(withOperatorAuth(handleMoveBundleHolders)))
	mux.HandleFunc("GET /api/v1/bundles/{id}/impact", withCORS(withUserAuth(handleGetBundleImpact)))
	mux.HandleFunc("POST /api/v1/bundles/{id}/roles", withCORS(withOperatorAuth(handleAddRoleToBundle)))
	// Welcome bundle toggle changes global onboarding policy (every newly-created
	// Zitadel user gets the flagged bundle), so it requires operator role —
	// withUserAuth alone would let any authenticated Zitadel user flip it.
	mux.HandleFunc("PUT /api/v1/bundles/{id}/welcome", withCORS(withOperatorAuth(handleSetWelcomeBundle)))

	// Explorer Views
	mux.HandleFunc("GET /api/v1/catalog", withCORS(withUserAuth(handleGetCatalog)))
	mux.HandleFunc("GET /api/v1/users", withCORS(withUserAuth(handleGetUsers)))
	// User-Bundle Assignments. Per-user reads are self-or-operator: members may
	// inspect their own access, never another user's. Granting is operator-only —
	// it feeds the claim-injection cache and (via ?apply=true) real Zitadel
	// mutations, so withUserAuth alone would let any member grant themselves
	// any role (July 2026 audit SC1/SC3).
	mux.HandleFunc("GET /api/v1/users/{id}/grants", withCORS(withSelfOrOperatorAuth(handleGetUserDirectGrants)))
	mux.HandleFunc("POST /api/v1/users/{id}/grants", withCORS(withOperatorAuth(handleUpsertUserDirectGrant)))
	// Removing a direct grant deletes the Syndra ledger row and queues the
	// Zitadel revoke in one transaction. NOT the same object as
	// DELETE /zitadel/users/{id}/grants/{grantId}, which removes the Zitadel-side
	// grant and would leave this row to restore the access on the next compile.
	mux.HandleFunc("DELETE /api/v1/users/{id}/grants/{grantId}", withCORS(withOperatorAuth(handleDeleteUserDirectGrant)))
	// Bulk access changes across a selection of people. Defaults to a rehearsal;
	// ?apply=true executes the plan it just computed.
	mux.HandleFunc("POST /api/v1/grants/bulk", withCORS(withOperatorAuth(handleBulkGrants)))
	mux.HandleFunc("GET /api/v1/users/{id}/bundles", withCORS(withSelfOrOperatorAuth(handleGetUserBundles)))
	mux.HandleFunc("POST /api/v1/users/{id}/bundles", withCORS(withOperatorAuth(handleAssignBundleToUser)))
	mux.HandleFunc("DELETE /api/v1/users/{id}/bundles/{bundleId}", withCORS(withOperatorAuth(handleRemoveBundleFromUser)))
	mux.HandleFunc("DELETE /api/v1/bundles/{id}/roles/{projectId}/{roleKey}", withCORS(withOperatorAuth(handleRemoveRoleFromBundle)))
	mux.HandleFunc("GET /api/v1/users/{id}/access", withCORS(withSelfOrOperatorAuth(handleGetUserAccess)))

	// Application Views
	mux.HandleFunc("GET /api/v1/applications", withCORS(withUserAuth(handleGetApplications)))
	mux.HandleFunc("GET /api/v1/applications/{id}/simulate", withCORS(withUserAuth(handleSimulateApplication)))

	// Claim shaping — what an application actually receives in its token.
	// Reads are operator-only: a claim template names the attributes the
	// organisation projects into tokens, which is not member-facing detail.
	mux.HandleFunc("GET /api/v1/claim-attributes", withCORS(withOperatorAuth(handleGetClaimAttributes)))
	mux.HandleFunc("GET /api/v1/projects/{id}/claim-shape", withCORS(withOperatorAuth(handleGetProjectClaimShape)))
	mux.HandleFunc("PUT /api/v1/projects/{id}/claim-profile", withCORS(withOperatorAuth(handleSetProjectClaimProfile)))
	mux.HandleFunc("PUT /api/v1/applications/{id}/claim-profile", withCORS(withOperatorAuth(handleSetApplicationClaimProfile)))
	mux.HandleFunc("DELETE /api/v1/applications/{id}/claim-profile", withCORS(withOperatorAuth(handleDeleteApplicationClaimProfile)))

	// Batch UID→name resolver. Powers <UserName/>/<ProjectName/>/<RoleName/>/<BundleName/>
	// components in the dashboard so raw UUIDs never reach the visible layer.
	mux.HandleFunc("POST /api/v1/lookup", withCORS(withUserAuth(handleLookup)))

	// Project Views
	mux.HandleFunc("GET /api/v1/projects", withCORS(withUserAuth(handleGetProjects)))
	mux.HandleFunc("GET /api/v1/topology", withCORS(withUserAuth(handleGetTopology)))

	// Rules Routes
	mux.HandleFunc("GET /api/v1/rules/mapping", withCORS(withUserAuth(handleGetMappingRules)))
	mux.HandleFunc("POST /api/v1/rules/mapping", withCORS(withOperatorAuth(handleCreateMappingRule)))
	mux.HandleFunc("PUT /api/v1/rules/mapping/{id}", withCORS(withOperatorAuth(handleUpdateMappingRule)))
	// Deleting a rule revokes what only that rule was granting, in the same transaction that
	// removes it — so a retired rule never leaves access behind that nothing explains.
	mux.HandleFunc("DELETE /api/v1/rules/mapping/{id}", withCORS(withOperatorAuth(handleDeleteMappingRule)))
	mux.HandleFunc("POST /api/v1/rules/mapping/validate", withCORS(withUserAuth(handleValidateMappingRule)))

	// Global confirmation-mode default (Task 22): every create-bundle/create-rule form reads this
	// to prefill its mode selector, so the GET is user-gated like the rest of those forms. Setting
	// it is global policy — operator-gated, same posture as the welcome-bundle toggle.
	mux.HandleFunc("GET /api/v1/config/confirmation-mode-default", withCORS(withUserAuth(handleGetGlobalConfirmationDefault)))
	mux.HandleFunc("PUT /api/v1/config/confirmation-mode-default", withCORS(withOperatorAuth(handleSetGlobalConfirmationDefault)))

	// Bulk confirmation-mode toggle (Task 22): flips confirmation_mode on a set of rules or
	// bundles in one statement. Operator-gated — a cross-cutting policy mutation.
	mux.HandleFunc("POST /api/v1/policies/confirmation-mode", withCORS(withOperatorAuth(handleBulkSetConfirmationMode)))

	// Audit log exposes every actor/target pair org-wide — operator-only (SC3).
	mux.HandleFunc("GET /api/v1/audit", withCORS(withOperatorAuth(handleGetAuditLogs)))
	// Members may list (own only — handler filters) and create (own only —
	// handler binds requester to the principal, SC8) requests; deciding one is
	// an operator action per the access-governance spec (SC1).
	mux.HandleFunc("GET /api/v1/requests", withCORS(withUserAuth(handleGetAccessRequests)))
	mux.HandleFunc("POST /api/v1/requests", withCORS(withUserAuth(handleCreateAccessRequest)))
	mux.HandleFunc("POST /api/v1/requests/{id}/decision", withCORS(withOperatorAuth(handleResolveAccessRequest)))
	// Withdrawing is the requester's own act, so it is user-gated and self-only (the handler
	// binds it to the row's requester). An operator taking back somebody else's ask would be a
	// rejection with no reviewer recorded — that is what the decision route is for.
	mux.HandleFunc("POST /api/v1/requests/{id}/withdraw", withCORS(withUserAuth(handleWithdrawAccessRequest)))
	// Bulk decisions. Rehearses by default, like every other bulk surface.
	mux.HandleFunc("POST /api/v1/requests/bulk-decision", withCORS(withOperatorAuth(handleBulkDecideRequests)))
	mux.HandleFunc("GET /api/v1/governance/summary", withCORS(withOperatorAuth(handleGetGovernanceSummary)))
	// Compact scalars for the sidebar badges. The rail polls this frequently;
	// it must never pull the full summary payload to render four numbers.
	mux.HandleFunc("GET /api/v1/governance/indicators", withCORS(withOperatorAuth(handleGetGovernanceIndicators)))

	// Review › Expiring access. Its own read with its own window — the screen is
	// time-boxed work with a deadline, not a slice of Today's queue.
	mux.HandleFunc("GET /api/v1/review/expiring-grants", withCORS(withOperatorAuth(handleGetExpiringGrants)))
	// "Seen it, letting it lapse." Records a decision and changes no access — the expiry sweep
	// still removes the grant on its date. The acknowledgement stops applying by itself if the
	// grant's expiry moves, so there is no invalidation path to call.
	mux.HandleFunc("POST /api/v1/review/expiring-grants/{grantId}/acknowledge", withCORS(withOperatorAuth(handleAcknowledgeGrantExpiry)))
	mux.HandleFunc("DELETE /api/v1/review/expiring-grants/{grantId}/acknowledge", withCORS(withOperatorAuth(handleClearGrantExpiryAcknowledgement)))

	// Role Management
	mux.HandleFunc("POST /api/v1/roles", withCORS(withUserAuth(handleCreateRole)))
	mux.HandleFunc("GET /api/v1/roles", withCORS(withUserAuth(handleGetGlobalRoleCatalog)))
	// Role → members: the reverse of "what can this person get into". Every row
	// carries its access sources so removal can be named after its own source.
	mux.HandleFunc("GET /api/v1/projects/{id}/roles/{key}/members", withCORS(withOperatorAuth(handleGetRoleMembers)))

	// Provisioning Intents (operator view)
	mux.HandleFunc("GET /api/v1/intents", withCORS(withUserAuth(handleGetProvisioningIntents)))

	// Provisioning Intents (sync service API — internal, API-key auth)
	mux.HandleFunc("POST /api/v1/intents/claim", withCORS(withAPIKeyAuth(handleClaimIntents)))
	mux.HandleFunc("POST /api/v1/intents/{id}/complete", withCORS(withAPIKeyAuth(handleCompleteIntent)))
	mux.HandleFunc("POST /api/v1/intents/{id}/fail", withCORS(withAPIKeyAuth(handleFailIntent)))

	// Shadow Password Vault (user-facing, self-only)
	mux.HandleFunc("PUT /api/v1/users/{uid}/shadow-credential", withCORS(withUserAuth(handleSetShadowCredential)))
	mux.HandleFunc("DELETE /api/v1/users/{uid}/shadow-credential", withCORS(withUserAuth(handleClearShadowCredential)))
	mux.HandleFunc("GET /api/v1/users/{uid}/shadow-credential/status", withCORS(withUserAuth(handleGetShadowCredentialStatus)))
	mux.HandleFunc("GET /api/v1/users/{uid}/shadow-credential/audit", withCORS(withUserAuth(handleGetShadowCredentialAudit)))

	// Shadow Password Vault (sync service — internal, API-key auth)
	mux.HandleFunc("GET /api/v1/shadow-credentials/{uid}/hash", withCORS(withAPIKeyAuth(handleGetShadowCredentialHash)))

	// User Profile (sync service — internal, API-key auth)
	mux.HandleFunc("GET /api/v1/users/{uid}/profile", withCORS(withAPIKeyAuth(handleGetUserProfile)))
	// Self-profile (UI — populates OIDC session cookie with Title/Team)
	mux.HandleFunc("GET /api/v1/me/profile", withCORS(withUserAuth(handleGetMyProfile)))

	// Operator: event and trigger logs
	mux.HandleFunc("GET /api/v1/onboarding/triggers", withCORS(withUserAuth(handleGetOnboardingTriggers)))
	mux.HandleFunc("GET /api/v1/webhook/events", withCORS(withUserAuth(handleGetWebhookEvents)))

	// Zitadel M2M health check — exercises the full service-account path
	// (key → token exchange → Management API call). Gated by withOperatorAuth
	// so it's reachable from the admin UI through the standard proxy. In dev
	// mode (no ZITADEL_DOMAIN), withUserAuth falls through to API-key auth,
	// preserving the cmdline smoke-test path.
	mux.HandleFunc("GET /api/v1/zitadel/health", withCORS(withOperatorAuth(handleZitadelHealth)))

	// Actions v2 signing-key rotation status — read-only observability.
	// Reports the age of the currently installed signing key against the
	// configured threshold (see rotation_status.go for semantics). Feeds the
	// Rotation Status panel on /zitadel in the UI.
	mux.HandleFunc("GET /api/v1/zitadel/action-rotation-status", withCORS(withOperatorAuth(HandleActionRotationStatus)))

	// Zitadel Discovery — live state introspection and cross-project role management.
	// Gated by withOperatorAuth: requires the admin project role (ZITADEL_ADMIN_ROLE_KEY).
	mux.HandleFunc("GET /api/v1/zitadel/users", withCORS(withOperatorAuth(handleListZitadelUsers)))
	mux.HandleFunc("GET /api/v1/zitadel/users/{id}", withCORS(withOperatorAuth(handleGetZitadelUser)))
	mux.HandleFunc("GET /api/v1/zitadel/projects", withCORS(withOperatorAuth(handleListZitadelProjects)))
	mux.HandleFunc("GET /api/v1/zitadel/projects/{id}/roles", withCORS(withOperatorAuth(handleListZitadelProjectRoles)))
	mux.HandleFunc("POST /api/v1/zitadel/projects/{id}/roles", withCORS(withOperatorAuth(handleCreateZitadelProjectRole)))
	mux.HandleFunc("PUT /api/v1/zitadel/projects/{id}/roles/{key}", withCORS(withOperatorAuth(handleUpdateZitadelProjectRole)))
	mux.HandleFunc("DELETE /api/v1/zitadel/projects/{id}/roles/{key}", withCORS(withOperatorAuth(handleDeleteZitadelProjectRole)))
	mux.HandleFunc("GET /api/v1/zitadel/grants", withCORS(withOperatorAuth(handleListAllZitadelGrants)))

	// Reconciliation: visibility-only diff between Syndra-direct grants and
	// Zitadel grants. Read-only — remediation is explicitly out of scope per
	// obsidian-clarity-redesign. Operator-gated because drift data exposes
	// the full grant inventory.
	mux.HandleFunc("GET /api/v1/reconciliation/grants", withCORS(withOperatorAuth(handleGetReconciliationDiff)))

	// Zitadel propagation outbox: operator drains the buffered Syndra-mediated
	// grant mutations explicitly (B4/D3). Operator-gated — draining issues real
	// Zitadel mutations and the pending list exposes the grant inventory.
	mux.HandleFunc("POST /api/v1/propagations/drain", withCORS(withOperatorAuth(handleDrainPropagations)))
	mux.HandleFunc("GET /api/v1/propagations", withCORS(withOperatorAuth(handleListPendingPropagations)))
	// Change history: the same rows grouped by the event that produced them,
	// including the ones still waiting. A half-applied cascade has to be
	// visible AS a half-applied cascade.
	mux.HandleFunc("GET /api/v1/propagations/cascade-groups", withCORS(withOperatorAuth(handleGetCascadeGroups)))
	mux.HandleFunc("GET /api/v1/zitadel/users/{id}/grants", withCORS(withOperatorAuth(handleListZitadelUserGrants)))
	mux.HandleFunc("POST /api/v1/zitadel/users/{id}/grants", withCORS(withOperatorAuth(handleAssignZitadelGrant)))
	mux.HandleFunc("PUT /api/v1/zitadel/users/{id}/grants/{grantId}", withCORS(withOperatorAuth(handleUpdateZitadelGrant)))
	mux.HandleFunc("DELETE /api/v1/zitadel/users/{id}/grants/{grantId}", withCORS(withOperatorAuth(handleRemoveZitadelGrant)))

	// Drift triage (B2): operator resolves out-of-band Zitadel grants surfaced by
	// the webhook listener or the reconciliation sweep. Operator-gated — same
	// posture as reconciliation/propagations, since drift exposes grant inventory
	// and every action mutates state (attribute/revoke/mark-external/reconcile).
	mux.HandleFunc("GET /api/v1/governance/drift", withCORS(withOperatorAuth(handleListDrift)))
	mux.HandleFunc("POST /api/v1/governance/drift/reconcile", withCORS(withOperatorAuth(handleReconcileDrift)))
	mux.HandleFunc("POST /api/v1/governance/drift/bulk-attribute", withCORS(withOperatorAuth(handleBulkAttributeDrift)))
	// The second and last bulk resolution. Bulk revoke is deliberately absent —
	// see handleBulkMarkDriftExternal.
	// Role-to-target mappings (change `addon-platform` group 7). What a Zitadel
	// role means on a target: a first-class versioned model with the same
	// history a bundle edit gets, because a mapping edit silently changes what
	// every holder of that role can reach.
	mux.HandleFunc("GET /api/v1/targets/mappings", withCORS(withOperatorAuth(handleListRoleMappings)))
	mux.HandleFunc("POST /api/v1/targets/mappings", withCORS(withOperatorAuth(handleCreateRoleMapping)))
	mux.HandleFunc("PATCH /api/v1/targets/mappings/{id}", withCORS(withOperatorAuth(handleUpdateRoleMapping)))
	mux.HandleFunc("DELETE /api/v1/targets/mappings/{id}", withCORS(withOperatorAuth(handleDeleteRoleMapping)))
	// The blast radius, before anything lands.
	mux.HandleFunc("GET /api/v1/targets/mappings/{id}/holders", withCORS(withOperatorAuth(handleMappingHolders)))
	// And the approval for it. Two surfaces rather than one flag: an edit and a
	// delete reach the same cohort and do opposite things to it, so a citation
	// must not be able to cross between them.
	mux.HandleFunc("POST /api/v1/targets/mappings/{id}/rehearse-edit", withCORS(withOperatorAuth(handleRehearseMappingEdit)))
	mux.HandleFunc("POST /api/v1/targets/mappings/{id}/rehearse-delete", withCORS(withOperatorAuth(handleRehearseMappingDelete)))
	mux.HandleFunc("POST /api/v1/targets/mappings/versions", withCORS(withOperatorAuth(handlePublishMappingVersion)))
	mux.HandleFunc("POST /api/v1/targets/{target}/mappings/versions/{version}/rollback", withCORS(withOperatorAuth(handleRollbackMappingVersion)))

	// The unmanaged inventory: what lives on a target that Syndra never put
	// there. Reported, never triaged — and adoption is the one way an account
	// moves out of it, because that decision hands somebody else's home
	// directory to a member if it is wrong.
	mux.HandleFunc("GET /api/v1/targets/{target}/inventory", withCORS(withOperatorAuth(handleTargetInventory)))
	mux.HandleFunc("POST /api/v1/targets/{target}/inventory/{username}/adopt", withCORS(withOperatorAuth(handleAdoptAccount)))

	// Access withdrawn that has not gone away. Beside drift triage, never inside
	// it: drift is access that appeared without an explanation, and this is
	// access somebody took away that is still there.
	mux.HandleFunc("GET /api/v1/governance/unconfirmed-revocations", withCORS(withOperatorAuth(handleUnconfirmedRevocations)))

	// Stopping an add-on writing, without a redeploy. A maintenance mode you
	// have to restart into is a maintenance mode nobody uses.
	mux.HandleFunc("POST /api/v1/targets/{target}/lifecycle", withCORS(withOperatorAuth(handleSetTargetLifecycle)))

	// Revocation as a composition (design §10). This target cannot end a
	// session, so "revoke" is a disabling allowance plus a credential rotation,
	// and the response says what neither half does.
	mux.HandleFunc("POST /api/v1/targets/{target}/users/{id}/revoke-access", withCORS(withOperatorAuth(handleRevokeTargetAccess)))

	// Entitlement convergence on an add-on target (group 9). Plan-then-apply,
	// like every other target-affecting operator action: the rehearsal computes
	// the diff through the add-on and records it, and the apply cites the id it
	// was given rather than resubmitting the request.
	mux.HandleFunc("POST /api/v1/targets/{target}/entitlements/rehearse", withCORS(withOperatorAuth(handleRehearseEntitlements)))
	mux.HandleFunc("POST /api/v1/targets/{target}/entitlements/apply", withCORS(withOperatorAuth(handleApplyEntitlements)))

	// Allowances (group 8): the per-user overlay beside role-derived access.
	// Nothing here deletes — an allowance is lifted, and the row survives,
	// because the reason and the actor stay attached to the person.
	mux.HandleFunc("POST /api/v1/allowances", withCORS(withOperatorAuth(handleCreateAllowance)))
	mux.HandleFunc("POST /api/v1/allowances/{id}/lift", withCORS(withOperatorAuth(handleLiftAllowance)))
	mux.HandleFunc("GET /api/v1/users/{id}/allowances", withCORS(withOperatorAuth(handleSubjectAllowances)))
	mux.HandleFunc("GET /api/v1/governance/allowances/review-due", withCORS(withOperatorAuth(handleAllowancesDueForReview)))

	mux.HandleFunc("POST /api/v1/governance/drift/bulk-mark-external", withCORS(withOperatorAuth(handleBulkMarkDriftExternal)))
	mux.HandleFunc("POST /api/v1/governance/drift/{id}/attribute", withCORS(withOperatorAuth(handleAttributeDrift)))
	mux.HandleFunc("POST /api/v1/governance/drift/{id}/revoke", withCORS(withOperatorAuth(handleRevokeDrift)))
	mux.HandleFunc("POST /api/v1/governance/drift/{id}/mark-external", withCORS(withOperatorAuth(handleMarkDriftExternal)))

	// Data Plane routes — verified by their own mechanisms (HMAC / Redis)
	mux.HandleFunc("POST /api/webhooks/zitadel",
		withCORS(withZitadelActionSignature("ZITADEL_EVENT_SIGNING_KEY", HandleZitadelWebhook)))
	mux.HandleFunc("POST /api/action/inject",
		withCORS(withZitadelActionSignature("ZITADEL_ACTION_SIGNING_KEY", HandleActionInject)))

	return withPanicGuard(withMaxBody(withSecurityHeaders(mux)))
}

// withUserAuth is the primary authorization middleware for all admin API routes.
//
// Production mode (ZITADEL_DOMAIN set): requires a Zitadel-issued RS256 JWT in
// the Authorization header. Validates signature, issuer, audience, and expiry.
// Stores the extracted admin user ID in the request context for audit attribution.
//
// Local-dev mode (ZITADEL_DOMAIN unset): falls back to shared API key
// (SYNDRA_API_KEY) so existing tooling continues to work without a live Zitadel
// instance. The shared key is never sufficient in production.
func withUserAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := os.Getenv("ZITADEL_DOMAIN")

		if domain == "" {
			// Local-dev fallback: shared API key
			withAPIKeyAuth(next)(w, r)
			return
		}

		// Production: Zitadel-issued user access token required
		audience := os.Getenv("ZITADEL_AUDIENCE")
		if audience == "" {
			log.Printf("[AUTH] ZITADEL_AUDIENCE is not set; rejecting request")
			jsonErrorResponse(w, http.StatusInternalServerError, "SERVER_ERROR", "Server missing auth configuration")
			return
		}

		rawToken := extractBearerToken(r)
		if rawToken == "" {
			log.Printf("[AUTH] Missing bearer token from %s %s", r.Method, r.URL.Path)
			jsonErrorResponse(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or invalid authorization token")
			return
		}

		principal, err := jwtValidate(r.Context(), rawToken, domain, audience)
		if err != nil {
			log.Printf("[AUTH] Token validation failed for %s %s: %v", r.Method, r.URL.Path, err)
			jsonErrorResponse(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid or expired token")
			return
		}

		log.Printf("[AUTH] Authorized admin=%s for %s %s", principal.Subject, r.Method, r.URL.Path)
		next(w, r.WithContext(withPrincipal(r.Context(), principal)))
	}
}

// isOperator reports whether the request carries operator-level (admin)
// access: the principal stashed by withUserAuth has the admin project role
// (ZITADEL_ADMIN_ROLE_KEY, default "admin"). In dev mode (no ZITADEL_DOMAIN)
// it is always true — auth fell through to the shared API key, which is
// operator tooling by definition.
//
// Reads the parsed principal from context; never re-extracts or re-parses
// the bearer token (audit ref C4). Must only be called behind withUserAuth.
func isOperator(r *http.Request) bool {
	if os.Getenv("ZITADEL_DOMAIN") == "" {
		return true
	}
	adminRoleKey := os.Getenv("ZITADEL_ADMIN_ROLE_KEY")
	if adminRoleKey == "" {
		adminRoleKey = "admin"
	}
	return principalFromContext(r.Context()).HasProjectRole(adminRoleKey)
}

// withOperatorAuth gates endpoints that require operator-level (admin) access.
func withOperatorAuth(next http.HandlerFunc) http.HandlerFunc {
	return withUserAuth(func(w http.ResponseWriter, r *http.Request) {
		if !isOperator(r) {
			log.Printf("[AUTH] Operator access denied for user=%s on %s %s",
				getAdminUserID(r.Context()), r.Method, r.URL.Path)
			jsonErrorResponse(w, http.StatusForbidden, "FORBIDDEN", "Operator-level access required")
			return
		}
		next(w, r)
	})
}

// withSelfOrOperatorAuth gates per-user reads: the authenticated subject must
// match the {id} path parameter, or carry the operator role. Prevents a
// member from reading another user's grants/bundles/access (SC3). Dev mode
// passes via isOperator (API-key auth carries no subject to compare).
func withSelfOrOperatorAuth(next http.HandlerFunc) http.HandlerFunc {
	return withUserAuth(func(w http.ResponseWriter, r *http.Request) {
		if !isOperator(r) && getAdminUserID(r.Context()) != r.PathValue("id") {
			jsonErrorResponse(w, http.StatusForbidden, "FORBIDDEN", "You can only view your own access")
			return
		}
		next(w, r)
	})
}

// withAPIKeyAuth verifies the SYNDRA_API_KEY shared secret.
// Used as the auth mechanism in local-dev mode and as defense-in-depth for
// data-plane routes that have their own verification (HMAC, Redis).
func withAPIKeyAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		expectedKey := os.Getenv("SYNDRA_API_KEY")
		if expectedKey == "" {
			jsonErrorResponse(w, http.StatusInternalServerError, "SERVER_ERROR", "Server missing auth configuration")
			return
		}
		if subtle.ConstantTimeCompare([]byte(extractBearerToken(r)), []byte(expectedKey)) != 1 {
			jsonErrorResponse(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or invalid authorization token")
			return
		}
		next(w, r)
	}
}

// extractBearerToken parses the Authorization header and returns the token string,
// or empty string if the header is absent or not in Bearer format.
func extractBearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(h, "Bearer ")
}

// withCORS sets CORS headers using the CORS_ORIGIN env var (default http://localhost:3000).
func withCORS(next http.HandlerFunc) http.HandlerFunc {
	origin := os.Getenv("CORS_ORIGIN")
	if origin == "" {
		origin = "http://localhost:3000"
	}
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next(w, r)
	}
}

// withPanicGuard turns a panicking handler into a 500 that says nothing about
// the request that caused it.
//
// Two reasons, and the second is the one this change owes. A panic in a handler
// otherwise closes the connection with no response, which a client cannot tell
// from a network fault. And the standard library's own recovery prints the
// request that was being served — a panic capture is one of the paths a
// declared secret escapes through in practice, alongside request-logging
// middleware and error responses that echo the offending payload (design §17).
//
// So the log line here is deliberately narrow: method, path, the panic value
// and the stack. Never the body, never the headers, never the parameters. The
// body is where a member's credential is, and a diagnostic that has to be
// redacted before it is safe is a diagnostic somebody will forget to redact.
//
// The client is told nothing beyond "this failed", for the same reason: an
// error response that echoes the offending request is the second path on that
// list.
func withPanicGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			// The documented way for a handler to abandon a response, and not
			// a bug. Asserted rather than compared with errors.Is, which would
			// itself panic on a non-error value — inside a recovery, that turns
			// one handler's bug into a crash.
			if err, ok := rec.(error); ok && errors.Is(err, http.ErrAbortHandler) {
				panic(rec)
			}
			log.Printf("[PANIC] %s %s: %v\n%s", r.Method, r.URL.Path, rec, debug.Stack())
			jsonErrorResponse(w, http.StatusInternalServerError, "INTERNAL_ERROR",
				"Something went wrong handling that request.")
		}()
		next.ServeHTTP(w, r)
	})
}

// withSecurityHeaders adds standard security response headers to all responses.
func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}

// withMaxBody limits request body size to 1 MB to prevent abuse.
func withMaxBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		next.ServeHTTP(w, r)
	})
}

// jsonResponse simplifies writing standard struct definitions to HTTP streams
func jsonResponse(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("Failed to encode json: %v", err)
	}
}

// jsonError defines standard API error shape
type ErrorResponse struct {
	Error   string            `json:"error"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
}

func jsonErrorResponse(w http.ResponseWriter, status int, errStr, msg string) {
	jsonResponse(w, status, ErrorResponse{Error: errStr, Message: msg})
}

func jsonValidationErrorResponse(w http.ResponseWriter, msg string, details map[string]string) {
	jsonResponse(w, http.StatusBadRequest, ErrorResponse{
		Error:   "VALIDATION_FAILED",
		Message: msg,
		Details: details,
	})
}
