package handlers

import (
	"net/http"
	"strconv"
	"time"

	"mkauth/internal/models"
)

// reviewWindowDefaultDays matches the copy on Review › Expiring access:
// "direct grants inside the next 30 days". Today deliberately uses a tighter
// window — its job is a queue short enough to finish in one sitting.
const reviewWindowDefaultDays = 30

// reviewWindowMaxDays bounds the query. Past a year "expiring" stops meaning
// anything and the screen becomes the grant ledger it deliberately is not.
const reviewWindowMaxDays = 365

// handleGetExpiringGrants serves Review › Expiring access directly, rather than
// making the screen read a slice off the governance summary and inherit Today's
// window. The two answer different questions and are allowed different horizons.
//
// GET /api/v1/review/expiring-grants?within_days=30
func handleGetExpiringGrants(w http.ResponseWriter, r *http.Request) {
	days := reviewWindowDefaultDays
	if raw := r.URL.Query().Get("within_days"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 || parsed > reviewWindowMaxDays {
			jsonValidationErrorResponse(w, "within_days must be between 1 and 365",
				map[string]string{"within_days": "out of range"})
			return
		}
		days = parsed
	}

	grants, err := dbGetExpiringDirectGrants(r.Context(), time.Duration(days)*24*time.Hour)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	if grants == nil {
		grants = []models.DirectGrant{}
	}
	jsonResponse(w, http.StatusOK, map[string]any{
		"within_days": days,
		"grants":      grants,
	})
}
