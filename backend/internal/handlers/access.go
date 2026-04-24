package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"mkauth/internal/directory"
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
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}
	if !trimmedNonEmpty(userID) {
		jsonValidationErrorResponse(w, "id path parameter is required", map[string]string{"id": "required"})
		return
	}
	if !trimmedNonEmpty(req.ProjectID) || !trimmedNonEmpty(req.RoleKey) {
		jsonValidationErrorResponse(w, "project_id and role_key are required", map[string]string{
			"project_id": "required",
			"role_key":   "required",
		})
		return
	}
	if req.DurationDays < 0 {
		jsonValidationErrorResponse(w, "duration_days must be zero or greater", map[string]string{"duration_days": "min=0"})
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

	id, err := dbUpsertDirectGrant(r.Context(), userID, req.ProjectID, req.RoleKey, grantedBy, req.Reason, expiresAt)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	rebuildUserCacheOrSkip(r.Context(), userID)
	_ = dbInsertAuditLog(r.Context(), grantedBy, userID, "direct_grant.upserted", id)
	jsonResponse(w, http.StatusOK, map[string]string{"id": id, "message": "Direct grant saved"})
}

func handleGetAccessRequests(w http.ResponseWriter, r *http.Request) {
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	if status != "" && status != "pending" && status != "approved" && status != "rejected" {
		jsonValidationErrorResponse(w, "status must be one of pending, approved, rejected", map[string]string{"status": "enum"})
		return
	}
	requests, err := dbGetAccessRequests(r.Context(), status)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, requests)
}

func handleCreateAccessRequest(w http.ResponseWriter, r *http.Request) {
	var req CreateAccessRequestRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}
	if !trimmedNonEmpty(req.RequesterID) || !trimmedNonEmpty(req.ProjectID) || !trimmedNonEmpty(req.RoleKey) || !trimmedNonEmpty(req.Justification) {
		jsonValidationErrorResponse(w, "requester_id, project_id, role_key, and justification are required", map[string]string{
			"requester_id":   "required",
			"project_id":     "required",
			"role_key":       "required",
			"justification":  "required",
		})
		return
	}
	if req.DurationDays < 0 {
		jsonValidationErrorResponse(w, "duration_days must be zero or greater", map[string]string{"duration_days": "min=0"})
		return
	}

	var durationDays *int
	if req.DurationDays > 0 {
		durationDays = &req.DurationDays
	}
	id, err := dbCreateAccessRequest(r.Context(), req.RequesterID, req.ProjectID, req.RoleKey, req.Justification, durationDays)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	_ = dbInsertAuditLog(r.Context(), req.RequesterID, req.RequesterID, "access_request.created", id)
	jsonResponse(w, http.StatusCreated, map[string]string{"id": id})
}

func handleResolveAccessRequest(w http.ResponseWriter, r *http.Request) {
	requestID := r.PathValue("id")
	if !trimmedNonEmpty(requestID) {
		jsonValidationErrorResponse(w, "id path parameter is required", map[string]string{"id": "required"})
		return
	}

	var req ResolveAccessRequestRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}

	status := strings.ToLower(strings.TrimSpace(req.Status))
	if status != "approved" && status != "rejected" {
		jsonValidationErrorResponse(w, "status must be approved or rejected", map[string]string{"status": "enum"})
		return
	}
	if status == "approved" && !trimmedNonEmpty(req.ReviewerID) {
		jsonValidationErrorResponse(w, "reviewer_id is required when approving a request", map[string]string{"reviewer_id": "required_when=status:approved"})
		return
	}

	request, err := dbGetAccessRequestByID(r.Context(), requestID)
	if err != nil {
		jsonErrorResponse(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}

	if request.Status == "approved" || request.Status == "rejected" {
		jsonErrorResponse(w, http.StatusConflict, "ALREADY_RESOLVED",
			fmt.Sprintf("access request is already %s", request.Status))
		return
	}

	if err := dbResolveAccessRequest(r.Context(), requestID, status, req.ReviewerID, req.ReviewNote); err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	if status == "approved" {
		var expiresAt *time.Time
		if request.DurationDays != nil && *request.DurationDays > 0 {
			expiry := time.Now().UTC().Add(time.Duration(*request.DurationDays) * 24 * time.Hour)
			expiresAt = &expiry
		}
		if _, err := dbUpsertDirectGrant(r.Context(), request.RequesterID, request.ProjectID, request.RoleKey, req.ReviewerID, "Approved from access request", expiresAt); err != nil {
			jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
			return
		}
		rebuildUserCacheOrSkip(r.Context(), request.RequesterID)
	}

	actor := req.ReviewerID
	if actor == "" {
		actor = "system"
	}
	_ = dbInsertAuditLog(r.Context(), actor, request.RequesterID, "access_request."+status, requestID)
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

// rebuildUserCacheOrSkip pulls the project scope from the directory and
// rebuilds the user's compiled claims. On directory failure it logs and
// skips the rebuild, leaving previously compiled claims in place — see
// allApplicationProjectIDs for the rationale.
func rebuildUserCacheOrSkip(ctx context.Context, userID string) {
	projectIDs, err := allApplicationProjectIDs(ctx)
	if err != nil {
		log.Printf("[ACCESS] Skipping cache rebuild for user %s: directory lookup failed: %v", userID, err)
		return
	}
	cacheRebuildUser(ctx, userID, projectIDs)
}

// allApplicationProjectIDs returns the set of project IDs the cache compiler
// should rebuild for. Pulled from the directory (live Zitadel or demo fallback).
//
// Returns an error on directory failure so the caller can skip the rebuild
// entirely. cache.RebuildUserCache starts by wiping every mapping:<user>:*
// key; calling it with an empty slice would leave the Actions v2 path serving
// degraded (fail_closed or minimal_safe) output for this user until the next
// rebuild triggers. Preserving the last-known-good compiled claims is
// strictly safer than nuking them on a transient Zitadel blip.
func allApplicationProjectIDs(ctx context.Context) ([]string, error) {
	apps, err := directory.Default.Applications(ctx)
	if err != nil {
		return nil, err
	}
	projectIDs := make([]string, 0, len(apps))
	for _, app := range apps {
		projectIDs = append(projectIDs, app.ProjectID)
	}
	return projectIDs, nil
}
