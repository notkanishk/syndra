package handlers

import (
	"net/http"
	"strconv"

	"mkauth/internal/db"
)

func handleGetAuditLogs(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 && parsed <= 200 {
			limit = parsed
		}
	}

	// `user_id` narrows the tail to one person's involvement — actor or target.
	// Without it a person's Activity tab would have to client-filter the global
	// tail, which silently truncates: an account whose last action fell outside
	// the most recent 200 rows would render as "nothing ever happened".
	userID := r.URL.Query().Get("user_id")

	logs, err := db.GetAuditLogsForUser(r.Context(), userID, limit)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	if logs == nil {
		jsonResponse(w, http.StatusOK, []interface{}{})
		return
	}

	jsonResponse(w, http.StatusOK, logs)
}
