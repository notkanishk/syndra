package handlers

import (
	"errors"
	"net/http"

	"syndra/internal/services"
)

// svcAccessHistory is the read behind the person page's access over time.
var svcAccessHistory = services.AccessHistory

// handleGetUserAccessHistory is GET /api/v1/users/{id}/access-history?since=,
// operators only (A-24 §9, access over time; Advanced): each role's runs since
// a makerspace day at most a year back, and whether it is held today.
func handleGetUserAccessHistory(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	if !trimmedNonEmpty(userID) {
		jsonValidationErrorResponse(w, "id path parameter is required", map[string]string{"id": "required"})
		return
	}
	history, err := svcAccessHistory(r.Context(), userID, r.URL.Query().Get("since"))
	if errors.Is(err, services.ErrInvalidHistorySince) {
		jsonValidationErrorResponse(w, err.Error(), map[string]string{"since": "invalid"})
		return
	}
	if err != nil {
		viewError(w, err, http.StatusInternalServerError, "VIEW_ERROR")
		return
	}
	jsonResponse(w, http.StatusOK, history)
}
