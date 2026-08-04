package handlers

import (
	"net/http"
	"strings"

	"syndra/internal/models"
	"syndra/internal/services"
)

// Bundle version surfaces.
//
// Publishing and moving holders both follow the rehearsal contract the rest of
// the product uses: POST returns a plan, POST?apply=true returns the same plan
// with the writes done. There is no `apply` body field — applying is a property
// of the request, not of the payload, so a plan cannot be replayed into an
// accidental write.

// GET /api/v1/bundles/{id}/versions
func handleGetBundleVersions(w http.ResponseWriter, r *http.Request) {
	bundleID := r.PathValue("id")
	if !trimmedNonEmpty(bundleID) {
		jsonValidationErrorResponse(w, "id path parameter is required", map[string]string{"id": "required"})
		return
	}

	versions, err := svcListBundleVersions(r.Context(), bundleID)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	// Contents come with the list. A version is only meaningful as what it
	// contained, and a UI that had to fetch each one to say so would either
	// fan out N requests or show version numbers with nothing behind them.
	for i := range versions {
		roles, err := svcGetRolesForVersion(r.Context(), versions[i].ID)
		if err != nil {
			jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
			return
		}
		if roles == nil {
			roles = []models.BundleRole{}
		}
		versions[i].Roles = roles
	}
	if versions == nil {
		versions = []models.BundleVersion{}
	}
	jsonResponse(w, http.StatusOK, versions)
}

// GET /api/v1/bundles/{id}/holders — who is on which version.
func handleGetBundleHolders(w http.ResponseWriter, r *http.Request) {
	bundleID := r.PathValue("id")
	if !trimmedNonEmpty(bundleID) {
		jsonValidationErrorResponse(w, "id path parameter is required", map[string]string{"id": "required"})
		return
	}
	holders, err := svcBundleHolders(r.Context(), bundleID)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	if holders == nil {
		holders = []models.BundleHolder{}
	}
	jsonResponse(w, http.StatusOK, holders)
}

// GET /api/v1/bundles/{id}/draft — the unpublished difference.
func handleGetBundleDraft(w http.ResponseWriter, r *http.Request) {
	bundleID := r.PathValue("id")
	if !trimmedNonEmpty(bundleID) {
		jsonValidationErrorResponse(w, "id path parameter is required", map[string]string{"id": "required"})
		return
	}
	draft, err := svcBundleDraft(r.Context(), bundleID)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, draft)
}

// PublishBundleVersionRequest is the body of a publish.
type PublishBundleVersionRequest struct {
	Note string `json:"note"`
	// Migrate moves everyone currently holding the bundle onto the new version.
	// Absent means false: leaving people where they are is the answer that
	// changes nothing, and it is the safe default for a request that forgot to
	// say.
	Migrate bool `json:"migrate"`
}

// POST /api/v1/bundles/{id}/publish[?apply=true]
func handlePublishBundleVersion(w http.ResponseWriter, r *http.Request) {
	bundleID := r.PathValue("id")
	if !trimmedNonEmpty(bundleID) {
		jsonValidationErrorResponse(w, "id path parameter is required", map[string]string{"id": "required"})
		return
	}

	var req PublishBundleVersionRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}

	call := services.PublishRequest{
		BundleID: bundleID,
		Note:     strings.TrimSpace(req.Note),
		Migrate:  req.Migrate,
	}

	if r.URL.Query().Get("apply") != "true" {
		plan, draft, err := svcRehearsePublish(r.Context(), call)
		if err != nil {
			jsonErrorResponse(w, http.StatusInternalServerError, "PLAN_ERROR", err.Error())
			return
		}
		jsonResponse(w, http.StatusOK, map[string]any{"plan": plan, "draft": draft})
		return
	}

	actor := getAdminUserID(r.Context())
	if actor == "" {
		actor = "system"
	}
	plan, version, err := svcPublishBundleVersion(r.Context(), actor, call)
	if err != nil {
		// "nothing to publish" is a conflict with the world, not a server
		// fault: somebody else published while this dialog was open, or the
		// draft was discarded.
		if strings.HasPrefix(err.Error(), "nothing to publish") {
			jsonErrorResponse(w, http.StatusConflict, "NOTHING_TO_PUBLISH", err.Error())
			return
		}
		jsonErrorResponse(w, http.StatusInternalServerError, "APPLY_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{"plan": plan, "version": version})
}

// MoveHoldersRequest repins named holders onto one version.
type MoveHoldersRequest struct {
	VersionID string   `json:"version_id"`
	UserIDs   []string `json:"user_ids"`
}

// POST /api/v1/bundles/{id}/holders/move[?apply=true]
func handleMoveBundleHolders(w http.ResponseWriter, r *http.Request) {
	bundleID := r.PathValue("id")
	if !trimmedNonEmpty(bundleID) {
		jsonValidationErrorResponse(w, "id path parameter is required", map[string]string{"id": "required"})
		return
	}

	var req MoveHoldersRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}
	problems := map[string]string{}
	if !trimmedNonEmpty(req.VersionID) {
		problems["version_id"] = "required"
	}
	if len(req.UserIDs) == 0 {
		problems["user_ids"] = "at least one"
	}
	if len(problems) > 0 {
		jsonValidationErrorResponse(w, "version_id and user_ids are required", problems)
		return
	}

	call := services.MoveHoldersRequest{
		BundleID:  bundleID,
		VersionID: req.VersionID,
		UserIDs:   req.UserIDs,
	}

	if r.URL.Query().Get("apply") != "true" {
		plan, err := svcRehearseMoveHolders(r.Context(), call)
		if err != nil {
			jsonErrorResponse(w, http.StatusInternalServerError, "PLAN_ERROR", err.Error())
			return
		}
		jsonResponse(w, http.StatusOK, plan)
		return
	}

	actor := getAdminUserID(r.Context())
	if actor == "" {
		actor = "system"
	}
	plan, err := svcMoveHolders(r.Context(), actor, call)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "APPLY_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, plan)
}
