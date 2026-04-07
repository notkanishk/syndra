package handlers

import (
	"encoding/json"
	"net/http"

	"mkauth/internal/db"
)

type AddRoleToBundleRequest struct {
	ProjectID string `json:"project_id"`
	RoleKey   string `json:"role_key"`
}

type AssignBundleRequest struct {
	BundleID string `json:"bundle_id"`
}

func handleGetBundles(w http.ResponseWriter, r *http.Request) {
	bundles, err := db.GetAllBundles(r.Context())
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
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErrorResponse(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid JSON")
		return
	}

	id, err := db.CreateBundle(r.Context(), req.Name, req.Description)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	_ = db.InsertAuditLog(r.Context(), "system", "-", "bundle.created", id)
	jsonResponse(w, http.StatusCreated, map[string]string{"id": id})
}

func handleGetBundleRoles(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	roles, err := db.GetRolesForBundle(r.Context(), id)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, roles)
}

func handleAddRoleToBundle(w http.ResponseWriter, r *http.Request) {
	bundleID := r.PathValue("id")
	var req AddRoleToBundleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErrorResponse(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid JSON")
		return
	}

	if err := db.AddRoleToBundle(r.Context(), bundleID, req.ProjectID, req.RoleKey); err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	_ = db.InsertAuditLog(r.Context(), "system", "-", "bundle.role_added", bundleID+":"+req.RoleKey)
	jsonResponse(w, http.StatusOK, map[string]string{"message": "Role added to bundle"})
}

func handleGetUserBundles(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	bundles, err := db.GetBundlesForUser(r.Context(), userID)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, bundles)
}

func handleAssignBundleToUser(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	var req AssignBundleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErrorResponse(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid JSON")
		return
	}

	if err := db.AssignBundleToUser(r.Context(), userID, req.BundleID); err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	_ = db.InsertAuditLog(r.Context(), "system", userID, "bundle.assigned", req.BundleID)
	jsonResponse(w, http.StatusOK, map[string]string{"message": "Bundle assigned to user"})
}
