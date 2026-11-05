package handlers

import (
	"net/http"
	"strings"
	"time"

	"syndra/internal/db"
)

// POST /me/first-sign-in (A-18 Q9, extended by A-20 P18 to record every
// sign-in). The UI's sign-in route calls it once per sign-in, with the person's
// own token. It records the sign-in and says two things about THIS one:
// first_visit is true for the session that wrote the first sign-in and false
// for every later one, and previous_sign_in_at is when they were last here
// before now (null on the first). The session carries both, so a refresh keeps
// the welcome and "since Tuesday", and a new sign-in moves them on.
func handleFirstSignIn(w http.ResponseWriter, r *http.Request) {
	subject := getAdminUserID(r.Context())
	if strings.TrimSpace(subject) == "" {
		jsonErrorResponse(w, http.StatusUnauthorized, "NO_SUBJECT", "This is a person's own.")
		return
	}
	in, err := db.RecordSignIn(r.Context(), subject)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", "Couldn't record the sign-in.")
		return
	}
	var previous any
	if in.Previous != nil {
		previous = in.Previous.UTC().Format(time.RFC3339)
	}
	jsonResponse(w, http.StatusOK, map[string]any{
		"first_sign_in_at":    in.FirstAt.UTC().Format(time.RFC3339),
		"first_visit":         in.First,
		"previous_sign_in_at": previous,
	})
}
