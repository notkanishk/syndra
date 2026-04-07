package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"mkauth/internal/cache"
	"mkauth/internal/db"
	"mkauth/internal/demo"
	"mkauth/internal/services"
)

type UpsertDirectGrantRequest struct {
	ProjectID    string `json:"project_id"`
	RoleKey      string `json:"role_key"`
	GrantedBy    string `json:"granted_by"`
	Reason       string `json:"reason"`
	DurationDays int    `json:"duration_days"`
}

type CreateAccessRequestRequest struct {
	RequesterID   string `json:"requester_id"`
	ProjectID     string `json:"project_id"`
	RoleKey       string `json:"role_key"`
	Justification string `json:"justification"`
	DurationDays  int    `json:"duration_days"`
}

type ResolveAccessRequestRequest struct {
	Status     string `json:"status"`
	ReviewerID string `json:"reviewer_id"`
	ReviewNote string `json:"review_note"`
}

func handleGetUserDirectGrants(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	grants, err := services.UserDirectGrants(r.Context(), userID)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, grants)
}

func handleUpsertUserDirectGrant(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	var req UpsertDirectGrantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErrorResponse(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid JSON")
		return
	}
	if req.ProjectID == "" || req.RoleKey == "" {
		jsonErrorResponse(w, http.StatusBadRequest, "VALIDATION_FAILED", "project_id and role_key are required")
		return
	}

	grantedBy := req.GrantedBy
	if grantedBy == "" {
		grantedBy = "system"
	}

	var expiresAt *time.Time
	if req.DurationDays > 0 {
		expiry := time.Now().UTC().Add(time.Duration(req.DurationDays) * 24 * time.Hour)
		expiresAt = &expiry
	}

	id, err := db.UpsertDirectGrant(r.Context(), userID, req.ProjectID, req.RoleKey, grantedBy, req.Reason, expiresAt)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	cache.RebuildUserCache(r.Context(), userID, allApplicationProjectIDs())
	_ = db.InsertAuditLog(r.Context(), grantedBy, userID, "direct_grant.upserted", id)
	jsonResponse(w, http.StatusOK, map[string]string{"id": id, "message": "Direct grant saved"})
}

func handleGetAccessRequests(w http.ResponseWriter, r *http.Request) {
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	requests, err := db.GetAccessRequests(r.Context(), status)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, requests)
}

func handleCreateAccessRequest(w http.ResponseWriter, r *http.Request) {
	var req CreateAccessRequestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErrorResponse(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid JSON")
		return
	}
	if req.RequesterID == "" || req.ProjectID == "" || req.RoleKey == "" || req.Justification == "" {
		jsonErrorResponse(w, http.StatusBadRequest, "VALIDATION_FAILED", "requester_id, project_id, role_key, and justification are required")
		return
	}

	var durationDays *int
	if req.DurationDays > 0 {
		durationDays = &req.DurationDays
	}
	id, err := db.CreateAccessRequest(r.Context(), req.RequesterID, req.ProjectID, req.RoleKey, req.Justification, durationDays)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	_ = db.InsertAuditLog(r.Context(), req.RequesterID, req.RequesterID, "access_request.created", id)
	jsonResponse(w, http.StatusCreated, map[string]string{"id": id})
}

func handleResolveAccessRequest(w http.ResponseWriter, r *http.Request) {
	requestID := r.PathValue("id")
	var req ResolveAccessRequestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErrorResponse(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid JSON")
		return
	}

	status := strings.ToLower(strings.TrimSpace(req.Status))
	if status != "approved" && status != "rejected" {
		jsonErrorResponse(w, http.StatusBadRequest, "VALIDATION_FAILED", "status must be approved or rejected")
		return
	}

	request, err := db.GetAccessRequestByID(r.Context(), requestID)
	if err != nil {
		jsonErrorResponse(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}

	if err := db.ResolveAccessRequest(r.Context(), requestID, status, req.ReviewerID, req.ReviewNote); err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	if status == "approved" {
		var expiresAt *time.Time
		if request.DurationDays != nil && *request.DurationDays > 0 {
			expiry := time.Now().UTC().Add(time.Duration(*request.DurationDays) * 24 * time.Hour)
			expiresAt = &expiry
		}
		if _, err := db.UpsertDirectGrant(r.Context(), request.RequesterID, request.ProjectID, request.RoleKey, req.ReviewerID, "Approved from access request", expiresAt); err != nil {
			jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
			return
		}
		cache.RebuildUserCache(r.Context(), request.RequesterID, allApplicationProjectIDs())
	}

	actor := req.ReviewerID
	if actor == "" {
		actor = "system"
	}
	_ = db.InsertAuditLog(r.Context(), actor, request.RequesterID, "access_request."+status, requestID)
	jsonResponse(w, http.StatusOK, map[string]string{"message": "Request resolved"})
}

func handleGetGovernanceSummary(w http.ResponseWriter, r *http.Request) {
	summary, err := services.Governance(r.Context())
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, summary)
}

func allApplicationProjectIDs() []string {
	projectIDs := make([]string, 0, len(demo.Applications()))
	for _, app := range demo.Applications() {
		projectIDs = append(projectIDs, app.ProjectID)
	}
	return projectIDs
}
