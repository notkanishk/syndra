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
	Name             string `json:"name"`
	Description      string `json:"description"`
	ConfirmationMode string `json:"confirmation_mode,omitempty"`
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

	// Creating a bundle triggers no cascade (no members/roles yet) — mode just seeds the row for
	// future add-role/assign cascades. Inherits the global default unless overridden.
	mode, err := resolveConfirmationMode(r.Context(), req.ConfirmationMode)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	id, err := dbCreateBundle(r.Context(), req.Name, req.Description, mode)
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

	actor := getAdminUserID(r.Context())
	if actor == "" {
		actor = "system"
	}
	// The cascade owns the mutation now (add-role + per-member outbox rows commit in one tx),
	// then (auto mode) drains those rows. Enqueue failure rolls back the mutation → 500; a
	// drain failure rides in cascade.drain and is not fatal.
	cascade, err := svcCascadeRoleAdded(r.Context(), actor, bundleID, req.ProjectID, req.RoleKey)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "CASCADE_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{
		"message": "Role added to bundle",
		"cascade": cascade,
	})
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

	actor := getAdminUserID(r.Context())
	if actor == "" {
		actor = "system"
	}
	// The cascade owns the mutation now (assign + per-role outbox rows commit in one tx), then
	// (auto mode) drains those rows. Enqueue failure rolls back the mutation → 500; a drain
	// failure rides in cascade.drain and is not fatal.
	cascade, err := svcCascadeBundleAssigned(r.Context(), actor, userID, req.BundleID)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "CASCADE_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{
		"message": "Bundle assigned to user",
		"cascade": cascade,
	})
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
