package handlers

import (
	"errors"
	"net/http"

	"mkauth/internal/db"
	"mkauth/internal/services"
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
	indicators, err := svcGovernanceIndicators(r.Context())
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "INDICATORS_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, indicators)
}

// handleDeleteUserDirectGrant removes one MkAuth direct grant: the ledger row
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

	res, err := svcDeleteDirectGrant(r.Context(), userID, grantID, resolveActor(r, ""))
	if errors.Is(err, db.ErrGrantNotFound) {
		jsonErrorResponse(w, http.StatusNotFound, "GRANT_NOT_FOUND",
			"No direct grant with that id belongs to this user. It may already have been removed or expired.")
		return
	}
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	// Recompile before returning, on a context detached from the request, so
	// the access is gone from the token path even if the caller disconnects.
	rebuildUserCacheDetachedFn(r.Context(), userID)

	// Same inline-apply opt-in as the grant path: ?apply=true drains these rows
	// now instead of leaving them for the operator to resume. There may be none
	// — every role the grant carried is still covered by another source — and
	// that is the point: nothing is queued, so nothing is drained.
	if r.URL.Query().Get("apply") == "true" {
		for _, id := range res.OutboxIDs {
			if _, derr := svcDrainPropagationRow(r.Context(), id); derr != nil {
				continue
			}
			if st, serr := dbGetPropagationStatus(r.Context(), id); serr == nil && st != "" {
				res.Status = st
			}
		}
	}

	jsonResponse(w, http.StatusAccepted, res)
}

var (
	// Injectable so the delete handler's contract — the cache is rebuilt before
	// the response — is assertable without a live Redis.
	rebuildUserCacheDetachedFn = rebuildUserCacheDetached

	svcRoleMembers          = services.RoleMembers
	svcGovernanceIndicators = services.GovernanceIndicators
	svcDeleteDirectGrant    = services.DeleteDirectGrant
)
