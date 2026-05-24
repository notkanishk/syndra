package handlers

import (
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"

	"mkauth/internal/services"
)

// enforceSelfOnly verifies that the acting user matches the target {uid}.
//
// Production mode (JWT actor present): the actor MUST equal {uid}; otherwise
// 403. The actor is used for audit attribution.
//
// Dev mode (API-key auth, no JWT actor): if requireActor is true (mutations),
// the caller MUST provide ?actor=<id> to attribute the action — without it,
// the audit log would record the target user as the actor (May 2026 audit C3).
// If requireActor is false (reads), the actor falls back to {uid} silently.
//
// Returns false and writes the appropriate JSON error response if the check
// fails. The returned actorID is what should appear in audit_logs.
func enforceSelfOnly(w http.ResponseWriter, r *http.Request, requireActor bool) (uid, actorID string, ok bool) {
	uid = r.PathValue("uid")
	if uid == "" {
		jsonValidationErrorResponse(w, "Missing user ID", map[string]string{"uid": "required"})
		return "", "", false
	}
	actorID = getAdminUserID(r.Context())
	if actorID != "" && actorID != uid {
		jsonErrorResponse(w, http.StatusForbidden, "FORBIDDEN", "You can only manage your own shadow credential")
		return "", "", false
	}
	if actorID == "" {
		// Dev mode: no JWT actor.
		if requireActor {
			actorID = strings.TrimSpace(r.URL.Query().Get("actor"))
			if actorID == "" {
				jsonErrorResponse(w, http.StatusBadRequest, "MISSING_ACTOR", "Dev-mode mutations require ?actor=<id> for audit attribution")
				return "", "", false
			}
			log.Printf("[VAULT] dev-mode actor=%s for %s %s", actorID, r.Method, r.URL.Path)
		} else {
			actorID = uid
		}
	}
	return uid, actorID, true
}

// handleSetShadowCredential sets or rotates a user's shadow password.
// PUT /api/v1/users/{uid}/shadow-credential
func handleSetShadowCredential(w http.ResponseWriter, r *http.Request) {
	uid, actorID, ok := enforceSelfOnly(w, r, true)
	if !ok {
		return
	}

	var body struct {
		Password string `json:"password"`
	}
	if err := decodeJSONStrict(r.Body, &body); err != nil {
		jsonValidationErrorResponse(w, "Invalid request body", map[string]string{"body": err.Error()})
		return
	}
	if !trimmedNonEmpty(body.Password) {
		jsonValidationErrorResponse(w, "password is required", map[string]string{"password": "required"})
		return
	}

	if err := svcSetShadowPassword(r.Context(), uid, actorID, body.Password, r.RemoteAddr); err != nil {
		if errors.Is(err, services.ErrComplexity) {
			jsonValidationErrorResponse(w, err.Error(), map[string]string{"password": "complexity"})
			return
		}
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, map[string]string{"message": "Shadow credential set"})
}

// handleClearShadowCredential removes a user's shadow password.
// DELETE /api/v1/users/{uid}/shadow-credential
func handleClearShadowCredential(w http.ResponseWriter, r *http.Request) {
	uid, actorID, ok := enforceSelfOnly(w, r, true)
	if !ok {
		return
	}

	if err := svcClearShadowPassword(r.Context(), uid, actorID, r.RemoteAddr); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			jsonErrorResponse(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		} else {
			jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		}
		return
	}
	jsonResponse(w, http.StatusOK, map[string]string{"message": "Shadow credential cleared"})
}

// handleGetShadowCredentialStatus checks if a user has a shadow credential.
// GET /api/v1/users/{uid}/shadow-credential/status
func handleGetShadowCredentialStatus(w http.ResponseWriter, r *http.Request) {
	uid, _, ok := enforceSelfOnly(w, r, false)
	if !ok {
		return
	}

	status, err := dbHasShadowCredential(r.Context(), uid)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, status)
}

// handleGetShadowCredentialAudit returns the audit trail for a user's shadow credential.
// GET /api/v1/users/{uid}/shadow-credential/audit
func handleGetShadowCredentialAudit(w http.ResponseWriter, r *http.Request) {
	uid, _, ok := enforceSelfOnly(w, r, false)
	if !ok {
		return
	}

	entries, err := dbGetShadowCredentialAudit(r.Context(), uid)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	if entries == nil {
		jsonResponse(w, http.StatusOK, []any{})
		return
	}
	jsonResponse(w, http.StatusOK, entries)
}

// handleGetShadowCredentialHash returns the full credential hash for the sync service.
// GET /api/v1/shadow-credentials/{uid}/hash
// Auth: API key only (sync service internal).
func handleGetShadowCredentialHash(w http.ResponseWriter, r *http.Request) {
	uid := r.PathValue("uid")
	if uid == "" {
		jsonValidationErrorResponse(w, "Missing user ID", map[string]string{"uid": "required"})
		return
	}

	cred, err := dbGetShadowCredential(r.Context(), uid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			jsonErrorResponse(w, http.StatusNotFound, "NOT_FOUND", "No shadow credential for this user")
		} else {
			jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		}
		return
	}
	jsonResponse(w, http.StatusOK, map[string]string{
		"credential_hash": cred.CredentialHash,
		"algorithm":       cred.Algorithm,
	})
}
