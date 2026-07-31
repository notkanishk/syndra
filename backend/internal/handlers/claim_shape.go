package handlers

import (
	"context"
	"log"
	"net/http"
	"strings"

	"mkauth/internal/claims"
	"mkauth/internal/directory"
	"mkauth/internal/models"
	"mkauth/internal/services"
)

// Claim shaping endpoints — the operator's control over what an application
// receives in its token.
//
//	GET    /api/v1/projects/{id}/claim-shape       read the project's shape
//	PUT    /api/v1/projects/{id}/claim-profile     edit the project default
//	PUT    /api/v1/applications/{id}/claim-profile edit one app's override
//	DELETE /api/v1/applications/{id}/claim-profile drop the override
//	GET    /api/v1/claim-attributes                selectable attribute sources
//
// Every write invalidates the data plane's cached shape for the affected
// project, so the next token issued carries the new shape. A "save" that takes
// effect at some unpredictable point within a TTL would make the editor
// untrustworthy, which is the exact failure this whole change exists to fix.

// ClaimProfileRequest is the wire shape for a profile edit. Deliberately not
// the claims.Profile struct: project_id and application_id are derived from
// the path and the directory, never accepted from the client.
type ClaimProfileRequest struct {
	ClaimName       string                 `json:"claim_name"`
	FormatType      string                 `json:"format_type"`
	AttributeClaims map[string]string      `json:"attribute_claims"`
	StaticClaims    map[string]interface{} `json:"static_claims"`
}

func (r ClaimProfileRequest) toProfile() claims.Profile {
	return claims.Profile{
		ClaimName:       strings.TrimSpace(r.ClaimName),
		FormatType:      strings.TrimSpace(r.FormatType),
		AttributeClaims: r.AttributeClaims,
		StaticClaims:    r.StaticClaims,
	}
}

func handleGetProjectClaimShape(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	if !trimmedNonEmpty(projectID) {
		jsonValidationErrorResponse(w, "id path parameter is required", map[string]string{"id": "required"})
		return
	}

	shape, err := svcProjectClaimShape(r.Context(), projectID)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "CLAIM_SHAPE_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, shape)
}

func handleSetProjectClaimProfile(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	if !trimmedNonEmpty(projectID) {
		jsonValidationErrorResponse(w, "id path parameter is required", map[string]string{"id": "required"})
		return
	}

	var req ClaimProfileRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}

	if err := svcSaveProjectClaimProfile(r.Context(), projectID, req.toProfile()); err != nil {
		jsonValidationErrorResponse(w, err.Error(), map[string]string{"claim_name": "invalid"})
		return
	}

	invalidateClaimShape(r.Context(), projectID)
	auditClaimShape(r, "claim_profile.updated", projectID)

	shape, err := svcProjectClaimShape(r.Context(), projectID)
	if err != nil {
		jsonResponse(w, http.StatusOK, map[string]string{"status": "saved"})
		return
	}
	jsonResponse(w, http.StatusOK, shape)
}

func handleSetApplicationClaimProfile(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("id")
	if !trimmedNonEmpty(appID) {
		jsonValidationErrorResponse(w, "id path parameter is required", map[string]string{"id": "required"})
		return
	}

	var req ClaimProfileRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}

	if err := svcSaveAppClaimOverride(r.Context(), appID, req.toProfile()); err != nil {
		jsonValidationErrorResponse(w, err.Error(), map[string]string{"claim_name": "invalid"})
		return
	}

	projectID := invalidateClaimShapeForApp(r.Context(), appID)
	auditClaimShape(r, "app_claim_override.updated", appID)

	if projectID == "" {
		jsonResponse(w, http.StatusOK, map[string]string{"status": "saved"})
		return
	}
	shape, err := svcProjectClaimShape(r.Context(), projectID)
	if err != nil {
		jsonResponse(w, http.StatusOK, map[string]string{"status": "saved"})
		return
	}
	jsonResponse(w, http.StatusOK, shape)
}

func handleDeleteApplicationClaimProfile(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("id")
	if !trimmedNonEmpty(appID) {
		jsonValidationErrorResponse(w, "id path parameter is required", map[string]string{"id": "required"})
		return
	}

	// Resolve the project BEFORE deleting: afterwards the override row that
	// carried the project id is gone.
	projectID := claimShapeProjectForApp(r.Context(), appID)

	if err := svcDeleteAppClaimOverride(r.Context(), appID); err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	if projectID != "" {
		invalidateClaimShape(r.Context(), projectID)
	}
	auditClaimShape(r, "app_claim_override.deleted", appID)
	jsonResponse(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// handleGetClaimAttributes lists the attribute sources a profile may project.
// The UI populates its per-claim source picker from this rather than hard-
// coding a list that would drift from the shaper.
func handleGetClaimAttributes(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"attributes": claims.Attributes(),
		"formats":    []string{claims.FormatArray, claims.FormatCSV, claims.FormatSpaceDelimited},
	})
}

// invalidateClaimShape drops the data plane's cached profile set for a project.
func invalidateClaimShape(ctx context.Context, projectID string) {
	if err := redisDelKeys(ctx, claimShapeCacheKey(projectID)); err != nil {
		log.Printf("[CLAIM-SHAPE] cache invalidation failed for project=%s: %v (edit lands within TTL)", projectID, err)
	}
}

func invalidateClaimShapeForApp(ctx context.Context, appID string) string {
	projectID := claimShapeProjectForApp(ctx, appID)
	if projectID != "" {
		invalidateClaimShape(ctx, projectID)
	}
	return projectID
}

func claimShapeProjectForApp(ctx context.Context, appID string) string {
	app, ok, err := svcFindApplication(ctx, appID)
	if err != nil || !ok {
		return ""
	}
	return app.ProjectID
}

func auditClaimShape(r *http.Request, action, resourceID string) {
	actor := resolveActor(r, "")
	if err := dbInsertAuditLog(r.Context(), actor, resourceID, action, resourceID); err != nil {
		log.Printf("[CLAIM-SHAPE] audit write failed for %s %s: %v", action, resourceID, err)
	}
}

var (
	svcProjectClaimShape       = services.ProjectClaimShape
	svcSaveProjectClaimProfile = services.SaveProjectClaimProfile
	svcSaveAppClaimOverride    = services.SaveAppClaimOverride
	svcDeleteAppClaimOverride  = services.DeleteAppClaimOverride
	svcFindApplication         = func(ctx context.Context, appID string) (models.ApplicationCatalog, bool, error) {
		return directory.Default.FindApplication(ctx, appID)
	}
)
