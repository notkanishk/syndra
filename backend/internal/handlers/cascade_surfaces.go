package handlers

import (
	"net/http"
	"strings"

	"syndra/internal/models"
	"syndra/internal/services"
)

// cascadeGroupsLimit caps Change history — a glance list, not a paginated worklist.
const cascadeGroupsLimit = 50

// handleGetCascadeGroups is Change history: every cascade-originated write,
// grouped by the event that produced it. It does NOT filter to applied — a
// cascade with writes still waiting is exactly the entry worth seeing, and
// "8 applied · 2 waiting" is the whole vocabulary of this screen.
//
// `?cascade=<id>` narrows to one, which is where an audit row's trace link
// lands. Answered here rather than by filtering the glance list in the console,
// because the audit tail reaches further back than the 50 most recent cascades.
// GET /api/v1/propagations/cascade-groups
func handleGetCascadeGroups(w http.ResponseWriter, r *http.Request) {
	cascadeID := strings.TrimSpace(r.URL.Query().Get("cascade"))
	groups, err := dbGetCascadeGroups(r.Context(), cascadeGroupsLimit, cascadeID)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	if groups == nil {
		groups = []models.CascadeGroup{}
	}
	jsonResponse(w, http.StatusOK, map[string]any{"cascades": groups})
}

// handleRemoveBundleFromUser is the revoke-side counterpart to handleAssignBundleToUser: the
// planner decides the Change (bundlePlanned) and the write carries the plan the sheet was shown
// (?fingerprint=&cancel_ids=).
// DELETE /api/v1/users/{id}/bundles/{bundleId}
func handleRemoveBundleFromUser(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	bundleID := r.PathValue("bundleId")
	if !trimmedNonEmpty(userID) || !trimmedNonEmpty(bundleID) {
		jsonValidationErrorResponse(w, "id and bundleId are required", map[string]string{
			"id":       "required",
			"bundleId": "required",
		})
		return
	}

	bundlePlanned(w, r, services.BulkOpRemoveBundle, userID, bundleID, seenFromQuery(r))
}

// handleRemoveRoleFromBundle is the revoke-side counterpart to handleAddRoleToBundle. The cascade
// OWNS the mutation (delete bundle_role + per-member revoke outbox rows commit in one tx via
// db.RemoveRoleFromBundleAndEnqueue), then (auto mode) drains those rows.
// DELETE /api/v1/bundles/{id}/roles/{projectId}/{roleKey}
func handleRemoveRoleFromBundle(w http.ResponseWriter, r *http.Request) {
	bundleID := r.PathValue("id")
	projectID := r.PathValue("projectId")
	roleKey := r.PathValue("roleKey")
	if !trimmedNonEmpty(bundleID) || !trimmedNonEmpty(projectID) || !trimmedNonEmpty(roleKey) {
		jsonValidationErrorResponse(w, "id, projectId, and roleKey are required", map[string]string{
			"id":        "required",
			"projectId": "required",
			"roleKey":   "required",
		})
		return
	}

	actor := getAdminUserID(r.Context())
	if actor == "" {
		actor = "system"
	}
	if err := svcEditBundleWorkingCopy(r.Context(), actor, bundleID, projectID, roleKey, false); err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	draft, err := svcBundleDraft(r.Context(), bundleID)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{
		"message": "Role removed from the bundle's working copy. Publish a version to apply it.",
		"draft":   draft,
	})
}
