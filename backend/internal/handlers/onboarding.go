package handlers

import (
	"net/http"

	"syndra/internal/db"
)

// handleGetOnboardingTriggers returns the full onboarding trigger log for operator
// visibility into welcome-bundle assignment events, including failed and pending entries.
func handleGetOnboardingTriggers(w http.ResponseWriter, r *http.Request) {
	triggers, err := db.GetOnboardingTriggers(r.Context())
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	if triggers == nil {
		jsonResponse(w, http.StatusOK, []interface{}{})
		return
	}

	jsonResponse(w, http.StatusOK, triggers)
}
