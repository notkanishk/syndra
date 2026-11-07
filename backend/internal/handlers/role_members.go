package handlers

import (
	"net/http"

	"syndra/internal/services"
)

// handleGetRoleMembers answers "who can currently use this?" for one
// (project, role) pair, with every row's access sources attached so the UI can
// name each removal after the thing being removed.
//
//	GET /api/v1/projects/{id}/roles/{key}/members
func handleGetRoleMembers(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	roleKey := r.PathValue("key")
	if !trimmedNonEmpty(projectID) || !trimmedNonEmpty(roleKey) {
		jsonValidationErrorResponse(w, "id and key path parameters are required", map[string]string{
			"id":  "required",
			"key": "required",
		})
		return
	}

	view, err := svcRoleMembers(r.Context(), projectID, roleKey)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "ROLE_MEMBERS_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, view)
}

// handleGetGovernanceIndicators returns the four badge scalars the sidebar
// polls. Separate from /governance/summary on purpose: the rail refreshes
// often and must not drag every pending request object across the wire to
// render a number.
//
//	GET /api/v1/governance/indicators
func handleGetGovernanceIndicators(w http.ResponseWriter, r *http.Request) {
	indicators, err := svcGovernanceIndicators(r.Context(), resolveActor(r, ""))
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "INDICATORS_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, indicators)
}

// handleDeleteUserDirectGrant removes one Syndra direct grant: the ledger row
// goes away and a revoke is queued for Zitadel, in one transaction.
//
//	DELETE /api/v1/users/{id}/grants/{grantId}
func handleDeleteUserDirectGrant(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	grantID := r.PathValue("grantId")
	if !trimmedNonEmpty(userID) || !trimmedNonEmpty(grantID) {
		jsonValidationErrorResponse(w, "id and grantId path parameters are required", map[string]string{
			"id":      "required",
			"grantId": "required",
		})
		return
	}

	removeGrantPlanned(w, r, userID, grantID)
}

var (
	// Injectable so the delete handler's contract — the cache is rebuilt before
	// the response — is assertable without a live Redis.
	rebuildUserCacheDetachedFn = rebuildUserCacheDetached

	svcRoleMembers          = services.RoleMembers
	svcGovernanceIndicators = services.GovernanceIndicators
	svcDeleteDirectGrant    = services.DeleteDirectGrant
	// svcPreviewDirectGrantRemoval is the removal's answer without its write.
	svcPreviewDirectGrantRemoval = services.PreviewDirectGrantRemoval
)
