package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
)

type AddRoleToBundleRequest struct {
	ProjectID string `json:"project_id"`
	RoleKey   string `json:"role_key"`
}

type AssignBundleRequest struct {
	BundleID string `json:"bundle_id"`
}

type CreateBundleRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func handleGetBundles(w http.ResponseWriter, r *http.Request) {
	bundles, err := dbGetAllBundles(r.Context())
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	if bundles == nil {
		jsonResponse(w, http.StatusOK, []interface{}{})
		return
	}
	jsonResponse(w, http.StatusOK, bundles)
}

func handleCreateBundle(w http.ResponseWriter, r *http.Request) {
	var req CreateBundleRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		jsonValidationErrorResponse(w, "name is required", map[string]string{"name": "required"})
		return
	}

	id, err := dbCreateBundle(r.Context(), req.Name, req.Description)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	actor := getAdminUserID(r.Context())
	if actor == "" {
		actor = "system"
	}
	_ = dbInsertAuditLog(r.Context(), actor, "-", "bundle.created", id)
	jsonResponse(w, http.StatusCreated, map[string]string{"id": id})
}

func handleGetBundleRoles(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	roles, err := dbGetRolesForBundle(r.Context(), id)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, roles)
}

func handleAddRoleToBundle(w http.ResponseWriter, r *http.Request) {
	bundleID := r.PathValue("id")
	if strings.TrimSpace(bundleID) == "" {
		jsonValidationErrorResponse(w, "id path parameter is required", map[string]string{"id": "required"})
		return
	}

	var req AddRoleToBundleRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}
	if !trimmedNonEmpty(req.ProjectID) || !trimmedNonEmpty(req.RoleKey) {
		jsonValidationErrorResponse(w, "project_id and role_key are required", map[string]string{
			"project_id": "required",
			"role_key":   "required",
		})
		return
	}

	if err := dbAddRoleToBundle(r.Context(), bundleID, req.ProjectID, req.RoleKey); err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	actor := getAdminUserID(r.Context())
	if actor == "" {
		actor = "system"
	}
	_ = dbInsertAuditLog(r.Context(), actor, "-", "bundle.role_added", bundleID+":"+req.RoleKey)
	jsonResponse(w, http.StatusOK, map[string]string{"message": "Role added to bundle"})
}

func handleGetUserBundles(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	bundles, err := dbGetBundlesForUser(r.Context(), userID)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, bundles)
}

func handleAssignBundleToUser(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	if strings.TrimSpace(userID) == "" {
		jsonValidationErrorResponse(w, "id path parameter is required", map[string]string{"id": "required"})
		return
	}
	var req AssignBundleRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}
	if !trimmedNonEmpty(req.BundleID) {
		jsonValidationErrorResponse(w, "bundle_id is required", map[string]string{"bundle_id": "required"})
		return
	}

	if err := dbAssignBundleToUser(r.Context(), userID, req.BundleID); err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	actor := getAdminUserID(r.Context())
	if actor == "" {
		actor = "system"
	}
	_ = dbInsertAuditLog(r.Context(), actor, userID, "bundle.assigned", req.BundleID)
	jsonResponse(w, http.StatusOK, map[string]string{"message": "Bundle assigned to user"})
}

// handleSetWelcomeBundle marks a bundle as the welcome bundle. Clears any
// previous welcome flag in the same transaction (see db.SetWelcomeBundle).
// PUT /api/v1/bundles/{id}/welcome
func handleSetWelcomeBundle(w http.ResponseWriter, r *http.Request) {
	bundleID := r.PathValue("id")
	if strings.TrimSpace(bundleID) == "" {
		jsonValidationErrorResponse(w, "id path parameter is required", map[string]string{"id": "required"})
		return
	}

	if err := dbSetWelcomeBundle(r.Context(), bundleID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			jsonErrorResponse(w, http.StatusNotFound, "NOT_FOUND", "Bundle not found")
			return
		}
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	actor := getAdminUserID(r.Context())
	if actor == "" {
		actor = "system"
	}
	_ = dbInsertAuditLog(r.Context(), actor, "-", "bundle.welcome_set", bundleID)
	jsonResponse(w, http.StatusOK, map[string]string{"message": "Welcome bundle set"})
}
