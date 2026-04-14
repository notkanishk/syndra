package handlers

import "net/http"

// handleGetWebhookEvents returns persisted webhook events for operator visibility.
// Supports an optional ?status= query parameter to filter by status.
func handleGetWebhookEvents(w http.ResponseWriter, r *http.Request) {
	statusFilter := r.URL.Query().Get("status")

	events, err := dbGetWebhookEvents(r.Context(), statusFilter)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	if events == nil {
		jsonResponse(w, http.StatusOK, []any{})
		return
	}
	jsonResponse(w, http.StatusOK, events)
}
