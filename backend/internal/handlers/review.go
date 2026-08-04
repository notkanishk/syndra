package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"syndra/internal/db"
	"syndra/internal/models"
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

	grants, err := dbGetExpiringWithAcks(r.Context(), time.Duration(days)*24*time.Hour)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	if grants == nil {
		grants = []models.ExpiringGrant{}
	}
	jsonResponse(w, http.StatusOK, map[string]any{
		"within_days": days,
		"grants":      grants,
	})
}

type acknowledgeExpiryRequest struct {
	// The expiry the operator was looking at. Required, and checked against the row rather than
	// trusted: an acknowledgement is of a specific date, so one made against a date the grant no
	// longer carries would be stored and then never apply.
	ExpiresAt time.Time `json:"expires_at"`
	Note      string    `json:"note"`
}

// handleAcknowledgeGrantExpiry records "seen, and letting it lapse" against one expiring grant.
// Operator-gated. It changes NOTHING about the access — the expiry sweep still removes the grant on
// its date — so it is not a destructive route and there is nothing to undo upstream.
//
// POST /api/v1/review/expiring-grants/{grantId}/acknowledge
func handleAcknowledgeGrantExpiry(w http.ResponseWriter, r *http.Request) {
	grantID := r.PathValue("grantId")
	if !trimmedNonEmpty(grantID) {
		jsonValidationErrorResponse(w, "grantId is required", map[string]string{"grantId": "required"})
		return
	}

	var req acknowledgeExpiryRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}
	if req.ExpiresAt.IsZero() {
		jsonValidationErrorResponse(w, "expires_at is required — an acknowledgement is of a specific date",
			map[string]string{"expires_at": "required"})
		return
	}

	actor := getAdminUserID(r.Context())
	if actor == "" {
		actor = "system"
	}

	userID, err := dbAcknowledgeGrantExpiry(r.Context(), grantID, req.ExpiresAt, actor, strings.TrimSpace(req.Note))
	switch {
	case errors.Is(err, db.ErrGrantNotFound):
		jsonErrorResponse(w, http.StatusNotFound, "NOT_FOUND", "No such grant — it may already have lapsed.")
		return
	case errors.Is(err, db.ErrGrantExpiryMoved):
		jsonErrorResponse(w, http.StatusConflict, "EXPIRY_CHANGED",
			"This grant's expiry changed since you loaded the page. Reload to see the new date before deciding.")
		return
	case err != nil:
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	_ = dbInsertAuditLog(r.Context(), actor, userID, "grant_expiry.acknowledged", grantID)
	jsonResponse(w, http.StatusOK, map[string]string{
		"message": "Recorded. The grant still lapses on its date.",
	})
}

// handleClearGrantExpiryAcknowledgement takes an acknowledgement back, so the row returns to the
// undecided part of the queue. Operator-gated, and not restricted to whoever made it — see
// db.ClearGrantExpiryAcknowledgement.
//
// DELETE /api/v1/review/expiring-grants/{grantId}/acknowledge
func handleClearGrantExpiryAcknowledgement(w http.ResponseWriter, r *http.Request) {
	grantID := r.PathValue("grantId")
	if !trimmedNonEmpty(grantID) {
		jsonValidationErrorResponse(w, "grantId is required", map[string]string{"grantId": "required"})
		return
	}

	actor := getAdminUserID(r.Context())
	if actor == "" {
		actor = "system"
	}

	userID, err := dbClearGrantExpiryAcknowledgement(r.Context(), grantID)
	switch {
	case errors.Is(err, db.ErrAcknowledgementNotFound):
		jsonErrorResponse(w, http.StatusNotFound, "NOT_FOUND",
			"Nothing to take back — this grant is not acknowledged.")
		return
	case err != nil:
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	_ = dbInsertAuditLog(r.Context(), actor, userID, "grant_expiry.acknowledgement_cleared", grantID)
	jsonResponse(w, http.StatusOK, map[string]string{"message": "Back in the queue."})
}
