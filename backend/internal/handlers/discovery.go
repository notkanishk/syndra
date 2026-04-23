package handlers

import (
	"net/http"
	"strconv"

	"mkauth/internal/zitadel"
)

// --- Request types ---

type assignGrantRequest struct {
	ProjectID string   `json:"projectId"`
	RoleKeys  []string `json:"roleKeys"`
}

type updateGrantRequest struct {
	RoleKeys []string `json:"roleKeys"`
}

type createProjectRoleRequest struct {
	RoleKey     string `json:"roleKey"`
	DisplayName string `json:"displayName"`
	Group       string `json:"group"`
}

type updateProjectRoleRequest struct {
	DisplayName string `json:"displayName"`
	Group       string `json:"group"`
}

// paginatedResponse wraps a search result with explicit pagination metadata so
// the consumer always knows whether results are truncated.
type paginatedResponse struct {
	Items  any `json:"items"`
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// parseSearchParams extracts ?limit=N&offset=N from query string, applying defaults.
func parseSearchParams(r *http.Request) zitadel.SearchParams {
	p := zitadel.SearchParams{Limit: zitadel.DefaultSearchLimit}
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 1000 {
			p.Limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			p.Offset = n
		}
	}
	return p
}

// --- Users ---

func handleListZitadelUsers(w http.ResponseWriter, r *http.Request) {
	p := parseSearchParams(r)
	result, err := zitadelListUsers(r.Context(), p)
	if err != nil {
		jsonErrorResponse(w, http.StatusBadGateway, "ZITADEL_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, paginatedResponse{
		Items: result.Items, Total: result.Total, Limit: p.Limit, Offset: p.Offset,
	})
}

func handleGetZitadelUser(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	if !trimmedNonEmpty(userID) {
		jsonValidationErrorResponse(w, "user id is required", map[string]string{"id": "required"})
		return
	}

	user, err := zitadelGetUser(r.Context(), userID)
	if err != nil {
		jsonErrorResponse(w, http.StatusBadGateway, "ZITADEL_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, user)
}

// --- Projects ---

func handleListZitadelProjects(w http.ResponseWriter, r *http.Request) {
	p := parseSearchParams(r)
	result, err := zitadelListProjects(r.Context(), p)
	if err != nil {
		jsonErrorResponse(w, http.StatusBadGateway, "ZITADEL_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, paginatedResponse{
		Items: result.Items, Total: result.Total, Limit: p.Limit, Offset: p.Offset,
	})
}

// --- Project Roles ---

func handleListZitadelProjectRoles(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	if !trimmedNonEmpty(projectID) {
		jsonValidationErrorResponse(w, "project id is required", map[string]string{"id": "required"})
		return
	}

	p := parseSearchParams(r)
	result, err := zitadelListProjectRoles(r.Context(), projectID, p)
	if err != nil {
		jsonErrorResponse(w, http.StatusBadGateway, "ZITADEL_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, paginatedResponse{
		Items: result.Items, Total: result.Total, Limit: p.Limit, Offset: p.Offset,
	})
}

func handleCreateZitadelProjectRole(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	if !trimmedNonEmpty(projectID) {
		jsonValidationErrorResponse(w, "project id is required", map[string]string{"id": "required"})
		return
	}

	var req createProjectRoleRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "invalid request body", map[string]string{"body": err.Error()})
		return
	}
	if !trimmedNonEmpty(req.RoleKey) {
		jsonValidationErrorResponse(w, "roleKey is required", map[string]string{"roleKey": "required"})
		return
	}

	if err := zitadelAddProjectRole(r.Context(), projectID, req.RoleKey, req.DisplayName, req.Group); err != nil {
		jsonErrorResponse(w, http.StatusBadGateway, "ZITADEL_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusCreated, map[string]string{"status": "created"})
}

func handleUpdateZitadelProjectRole(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	roleKey := r.PathValue("key")
	if !trimmedNonEmpty(projectID) || !trimmedNonEmpty(roleKey) {
		jsonValidationErrorResponse(w, "project id and role key are required", map[string]string{"id": "required", "key": "required"})
		return
	}

	var req updateProjectRoleRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "invalid request body", map[string]string{"body": err.Error()})
		return
	}

	if err := zitadelUpdateProjectRole(r.Context(), projectID, roleKey, req.DisplayName, req.Group); err != nil {
		jsonErrorResponse(w, http.StatusBadGateway, "ZITADEL_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, map[string]string{"status": "updated"})
}

func handleDeleteZitadelProjectRole(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	roleKey := r.PathValue("key")
	if !trimmedNonEmpty(projectID) || !trimmedNonEmpty(roleKey) {
		jsonValidationErrorResponse(w, "project id and role key are required", map[string]string{"id": "required", "key": "required"})
		return
	}

	if err := zitadelDeleteProjectRole(r.Context(), projectID, roleKey); err != nil {
		jsonErrorResponse(w, http.StatusBadGateway, "ZITADEL_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// --- Grants ---

func handleListAllZitadelGrants(w http.ResponseWriter, r *http.Request) {
	p := parseSearchParams(r)
	result, err := zitadelListAllGrants(r.Context(), p)
	if err != nil {
		jsonErrorResponse(w, http.StatusBadGateway, "ZITADEL_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, paginatedResponse{
		Items: result.Items, Total: result.Total, Limit: p.Limit, Offset: p.Offset,
	})
}

func handleListZitadelUserGrants(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	if !trimmedNonEmpty(userID) {
		jsonValidationErrorResponse(w, "user id is required", map[string]string{"id": "required"})
		return
	}

	p := parseSearchParams(r)
	result, err := zitadelListUserGrants(r.Context(), userID, p)
	if err != nil {
		jsonErrorResponse(w, http.StatusBadGateway, "ZITADEL_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, paginatedResponse{
		Items: result.Items, Total: result.Total, Limit: p.Limit, Offset: p.Offset,
	})
}

func handleAssignZitadelGrant(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	if !trimmedNonEmpty(userID) {
		jsonValidationErrorResponse(w, "user id is required", map[string]string{"id": "required"})
		return
	}

	var req assignGrantRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "invalid request body", map[string]string{"body": err.Error()})
		return
	}
	if !trimmedNonEmpty(req.ProjectID) || len(req.RoleKeys) == 0 {
		jsonValidationErrorResponse(w, "projectId and roleKeys are required", map[string]string{
			"projectId": "required",
			"roleKeys":  "at least one role key required",
		})
		return
	}

	if err := zitadelAddUserGrant(r.Context(), userID, req.ProjectID, req.RoleKeys); err != nil {
		jsonErrorResponse(w, http.StatusBadGateway, "ZITADEL_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusCreated, map[string]string{"status": "granted"})
}

func handleUpdateZitadelGrant(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	grantID := r.PathValue("grantId")
	if !trimmedNonEmpty(userID) || !trimmedNonEmpty(grantID) {
		jsonValidationErrorResponse(w, "user id and grant id are required", map[string]string{"id": "required", "grantId": "required"})
		return
	}

	var req updateGrantRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "invalid request body", map[string]string{"body": err.Error()})
		return
	}
	if len(req.RoleKeys) == 0 {
		jsonValidationErrorResponse(w, "roleKeys is required", map[string]string{"roleKeys": "at least one role key required"})
		return
	}

	if err := zitadelUpdateUserGrant(r.Context(), userID, grantID, req.RoleKeys); err != nil {
		jsonErrorResponse(w, http.StatusBadGateway, "ZITADEL_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, map[string]string{"status": "updated"})
}

func handleRemoveZitadelGrant(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	grantID := r.PathValue("grantId")
	if !trimmedNonEmpty(userID) || !trimmedNonEmpty(grantID) {
		jsonValidationErrorResponse(w, "user id and grant id are required", map[string]string{"id": "required", "grantId": "required"})
		return
	}

	if err := zitadelRemoveUserGrant(r.Context(), userID, grantID); err != nil {
		jsonErrorResponse(w, http.StatusBadGateway, "ZITADEL_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, map[string]string{"status": "removed"})
}
