package handlers

import (
	"errors"
	"net/http"
	"strings"

	"syndra/internal/db"
	"syndra/internal/services"
)

func handleCreateRole(w http.ResponseWriter, r *http.Request) {
	var req services.CreateRoleRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}

	req.ProjectID = strings.TrimSpace(req.ProjectID)
	req.RoleKey = strings.TrimSpace(req.RoleKey)
	req.DisplayName = strings.TrimSpace(req.DisplayName)

	if req.ProjectID == "" {
		jsonValidationErrorResponse(w, "project_id is required", map[string]string{"project_id": "required"})
		return
	}
	if req.RoleKey == "" {
		jsonValidationErrorResponse(w, "role_key is required", map[string]string{"role_key": "required"})
		return
	}
	if req.DisplayName == "" && req.CloneFrom == nil {
		jsonValidationErrorResponse(w, "display_name is required (or provide clone_from)", map[string]string{"display_name": "required"})
		return
	}
	if req.CloneFrom != nil {
		req.CloneFrom.ProjectID = strings.TrimSpace(req.CloneFrom.ProjectID)
		req.CloneFrom.RoleKey = strings.TrimSpace(req.CloneFrom.RoleKey)
		if req.CloneFrom.ProjectID == "" || req.CloneFrom.RoleKey == "" {
			jsonValidationErrorResponse(w, "clone_from requires both project_id and role_key", map[string]string{
				"clone_from.project_id": "required",
				"clone_from.role_key":   "required",
			})
			return
		}
	}

	actor := getAdminUserID(r.Context())
	if actor == "" {
		actor = "system"
	}

	role, err := svcCreateRole(r.Context(), req, actor)
	if err != nil {
		if errors.Is(err, db.ErrDuplicateRole) {
			jsonErrorResponse(w, http.StatusConflict, "CONFLICT", "A role with this project_id and role_key already exists")
			return
		}
		if errors.Is(err, services.ErrCloneSourceNotFound) {
			jsonErrorResponse(w, http.StatusNotFound, "NOT_FOUND", "Clone source role not found")
			return
		}
		jsonErrorResponse(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	jsonResponse(w, http.StatusCreated, role)
}

func handleGetGlobalRoleCatalog(w http.ResponseWriter, r *http.Request) {
	catalog, err := svcGlobalRoleCatalog(r.Context())
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	// Optional project_id filter.
	if projectFilter := r.URL.Query().Get("project_id"); projectFilter != "" {
		filtered := catalog[:0]
		for _, cr := range catalog {
			if cr.ProjectID == projectFilter {
				filtered = append(filtered, cr)
			}
		}
		catalog = filtered
	}

	if catalog == nil {
		jsonResponse(w, http.StatusOK, []interface{}{})
		return
	}
	jsonResponse(w, http.StatusOK, catalog)
}
