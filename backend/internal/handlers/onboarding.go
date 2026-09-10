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

// handleGetMissedOnboarding answers "who joined and got no welcome bundle" by
// reading current state (services.FindMissedOnboarding), not the trigger log
// — see that function's comment for why the two answer different questions.
func handleGetMissedOnboarding(w http.ResponseWriter, r *http.Request) {
	report, err := svcFindMissedOnboarding(r.Context())
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, report)
}
