package handlers

import (
	"net/http"
	"strconv"
)

// handleGetProvisioningIntents returns all provisioning intents for operator visibility.
// Supports optional ?status= query parameter.
func handleGetProvisioningIntents(w http.ResponseWriter, r *http.Request) {
	statusFilter := r.URL.Query().Get("status")

	intents, err := dbGetProvisioningIntents(r.Context(), statusFilter)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	if intents == nil {
		jsonResponse(w, http.StatusOK, []any{})
		return
	}
	jsonResponse(w, http.StatusOK, intents)
}

// handleClaimIntents atomically claims pending intents for the sync service.
// Uses FOR UPDATE SKIP LOCKED so concurrent workers never claim the same intents.
// Returns the claimed intents already transitioned to 'acknowledged' status.
// Accepts optional ?limit= parameter (default 50, max 200).
func handleClaimIntents(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		parsed, err := strconv.Atoi(l)
		if err != nil || parsed < 1 {
			jsonValidationErrorResponse(w, "Invalid limit", map[string]string{"limit": "must be a positive integer"})
			return
		}
		if parsed > 200 {
			parsed = 200
		}
		limit = parsed
	}

	intents, err := dbClaimPendingIntents(r.Context(), limit)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	if intents == nil {
		jsonResponse(w, http.StatusOK, []any{})
		return
	}
	jsonResponse(w, http.StatusOK, intents)
}

// handleCompleteIntent transitions an intent from acknowledged to completed.
func handleCompleteIntent(w http.ResponseWriter, r *http.Request) {
	intentID := r.PathValue("id")
	if intentID == "" {
		jsonValidationErrorResponse(w, "Missing intent ID", map[string]string{"id": "required"})
		return
	}

	if err := dbCompleteIntent(r.Context(), intentID); err != nil {
		jsonErrorResponse(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, map[string]string{"message": "Intent completed"})
}

// handleFailIntent transitions an intent to failed with an error message.
func handleFailIntent(w http.ResponseWriter, r *http.Request) {
	intentID := r.PathValue("id")
	if intentID == "" {
		jsonValidationErrorResponse(w, "Missing intent ID", map[string]string{"id": "required"})
		return
	}

	var body struct {
		ErrorMessage string `json:"error_message"`
	}
	if err := decodeJSONStrict(r.Body, &body); err != nil {
		jsonValidationErrorResponse(w, "Invalid request body", map[string]string{"body": err.Error()})
		return
	}
	if body.ErrorMessage == "" {
		jsonValidationErrorResponse(w, "error_message is required", map[string]string{"error_message": "required"})
		return
	}

	if err := dbFailIntent(r.Context(), intentID, body.ErrorMessage); err != nil {
		jsonErrorResponse(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, map[string]string{"message": "Intent marked as failed"})
}
