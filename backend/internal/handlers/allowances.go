package handlers

import (
	"errors"
	"net/http"
	"time"

	"syndra/internal/db"
)

// Allowance authoring and governance (change `addon-platform` group 8).
//
// The layer exists for the case where the role should stay — because the
// reason, the actor and the review date need to stay attached to the person
// rather than being erased into an absence. That is why nothing here deletes:
// an allowance is lifted, and the row survives.

type allowanceRequest struct {
	SubjectID string `json:"subject_id"`
	Target    string `json:"target"`
	Field     string `json:"field"`
	Value     string `json:"value"`
	Direction string `json:"direction"`
	Reason    string `json:"reason"`
	// Exactly one of these is enough, and neither is not. A denial with no
	// bound is an open-ended carve-out nobody is prompted to revisit.
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	ReviewDate *time.Time `json:"review_date,omitempty"`
}

func handleCreateAllowance(w http.ResponseWriter, r *http.Request) {
	var req allowanceRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}
	// The actor is the authenticated operator, never a field of the request. An
	// allowance whose author is whoever the client said is an allowance nobody
	// can be asked about, which is the whole thing this layer is for.
	created, err := dbCreateAllowance(r.Context(), db.Allowance{
		SubjectID: req.SubjectID, Target: req.Target, Field: req.Field, Value: req.Value,
		Direction: req.Direction, Reason: req.Reason,
		ActorID:    resolveActor(r, "operator"),
		ExpiresAt:  req.ExpiresAt,
		ReviewDate: req.ReviewDate,
	})
	if err != nil {
		writeAllowanceError(w, err)
		return
	}
	jsonResponse(w, http.StatusCreated, created)
}

func handleLiftAllowance(w http.ResponseWriter, r *http.Request) {
	if err := dbLiftAllowance(r.Context(), r.PathValue("id"), resolveActor(r, "operator")); err != nil {
		writeAllowanceError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{"status": "lifted"})
}

// handleSubjectAllowances returns every allowance ever recorded for a subject,
// lifted and lapsed included.
//
// The whole history, because the question a surface asks here is "what has been
// decided about this person", and an answer that showed only what still applies
// would erase every suspension that ended — which is the record the layer keeps
// attached to the person on purpose.
func handleSubjectAllowances(w http.ResponseWriter, r *http.Request) {
	rows, err := dbAllowancesForSubject(r.Context(), r.PathValue("id"))
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	now := time.Now()
	out := make([]map[string]any, 0, len(rows))
	for _, a := range rows {
		// Derived here rather than stored, so "in force" cannot go stale in a
		// column while the date it depends on passes.
		out = append(out, map[string]any{
			"allowance":  a,
			"in_force":   a.InForce(now),
			"review_due": a.ReviewDue(now),
		})
	}
	jsonResponse(w, http.StatusOK, map[string]any{"allowances": out})
}

// handleAllowancesDueForReview is the governance surface for indefinite
// suspensions whose review date has passed.
//
// They appear here and stay in force. Surfacing is a prompt, never a lapse:
// ending a suspension because nobody looked at it would restore access by
// inattention, which is the failure a review date exists to prevent running
// backwards.
func handleAllowancesDueForReview(w http.ResponseWriter, r *http.Request) {
	rows, err := dbAllowancesDueForReview(r.Context())
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	if rows == nil {
		rows = []db.Allowance{}
	}
	jsonResponse(w, http.StatusOK, map[string]any{"due_for_review": rows, "count": len(rows)})
}

func writeAllowanceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, db.ErrAllowanceUnbounded):
		jsonValidationErrorResponse(w, err.Error(), map[string]string{"expires_at": "one_of", "review_date": "one_of"})
	case errors.Is(err, db.ErrAllowanceAdditiveUnsupported):
		// 501, not 400: the request is well formed and the backend has not
		// built the arm yet. A 400 would send an operator looking for their own
		// mistake.
		jsonErrorResponse(w, http.StatusNotImplemented, "ALLOWANCE_ADDITIVE_UNSUPPORTED", err.Error())
	case errors.Is(err, db.ErrAllowanceInvalid):
		jsonValidationErrorResponse(w, err.Error(), map[string]string{"allowance": "invalid"})
	case errors.Is(err, db.ErrAllowanceNotFound):
		jsonErrorResponse(w, http.StatusNotFound, "ALLOWANCE_NOT_FOUND", err.Error())
	default:
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
	}
}
