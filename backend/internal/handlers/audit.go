package handlers

import (
	"net/http"
	"strconv"
	"time"

	"syndra/internal/db"
)

// maxAuditPage bounds one page, not the readable history. Before keyset paging
// it was both, so anything older than the most recent 200 mutations org-wide
// was simply unreachable unless you happened to know whose ?user_id= to look
// under.
const maxAuditPage = 200

func handleGetAuditLogs(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 && parsed <= maxAuditPage {
			limit = parsed
		}
	}

	// `user_id` narrows the tail to one person's involvement — actor or target.
	// Without it a person's Activity tab would have to client-filter the global
	// tail, which silently truncates: an account whose last action fell outside
	// the most recent 200 rows would render as "nothing ever happened".
	userID := r.URL.Query().Get("user_id")

	// The cursor is the (created_at, id) of the last row the client holds, and
	// both halves are required: `created_at` is the transaction timestamp, so a
	// cascade writing several audit rows writes them all at the identical
	// instant, and a timestamp-only cursor would skip the rest of that batch.
	// A malformed or half-supplied cursor is ignored rather than rejected —
	// the caller gets the newest page, which is a recoverable answer.
	var after *db.AuditCursor
	beforeAt := r.URL.Query().Get("before_at")
	beforeID := r.URL.Query().Get("before_id")
	if beforeAt != "" && beforeID != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, beforeAt); err == nil {
			after = &db.AuditCursor{CreatedAt: parsed, ID: beforeID}
		}
	}

	logs, err := db.GetAuditLogsForUser(r.Context(), userID, limit, after)
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
