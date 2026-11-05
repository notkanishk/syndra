package handlers

import (
	"net/http"
	"strings"
	"time"

	"syndra/internal/db"
)

// POST /me/first-sign-in (A-18 Q9). The UI's sign-in route calls it once per
// sign-in, with the person's own token. It records the first sign-in if there
// is none, and says whether THIS call wrote it: first_visit is true for the
// session that recorded it and false for every later one. The session carries
// the answer, so a refresh keeps the welcome and a new sign-in never has it.
func handleFirstSignIn(w http.ResponseWriter, r *http.Request) {
	subject := getAdminUserID(r.Context())
	if strings.TrimSpace(subject) == "" {
		jsonErrorResponse(w, http.StatusUnauthorized, "NO_SUBJECT", "This is a person's own.")
		return
	}
	at, first, err := db.RecordFirstSignIn(r.Context(), subject)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", "Couldn't record the sign-in.")
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{"first_sign_in_at": at.UTC().Format(time.RFC3339), "first_visit": first})
}
