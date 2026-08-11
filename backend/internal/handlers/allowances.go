package handlers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"syndra/internal/addons"
	"syndra/internal/db"
	"syndra/internal/services"
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
	// Structure before storage, the same check the mapping surface runs and for
	// a sharper reason: a mapping the resolver ignores fails visibly the moment
	// somebody looks for the entitlement, and an allowance the resolver ignores
	// looks exactly like a suspension that worked.
	field, value, err := validateAllowanceAgainstTarget(r.Context(), req.Target, req.Field, req.Value)
	if err != nil {
		writeAllowanceError(w, err)
		return
	}
	// The actor is the authenticated operator, never a field of the request. An
	// allowance whose author is whoever the client said is an allowance nobody
	// can be asked about, which is the whole thing this layer is for.
	// The normalised pair, never `req`. Writing what arrived would store a term
	// the validator approved a different version of.
	created, err := dbCreateAllowance(r.Context(), db.Allowance{
		SubjectID: req.SubjectID, Target: req.Target, Field: field, Value: value,
		Direction: req.Direction, Reason: req.Reason,
		ActorID:    resolveActor(r, ""),
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
	if err := dbLiftAllowance(r.Context(), r.PathValue("id"), resolveActor(r, "")); err != nil {
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

// validateAllowanceAgainstTarget checks the term against the target's own
// schema, in the order `validateMappingAgainstTarget` does: an unregistered
// target first, because the foreign key would refuse the row anyway with a
// message about a constraint rather than about the deployment.
// It returns the CANONICAL pair, not merely a verdict. A validator that checks
// a trimmed copy and leaves the caller holding the original is how `group=x `
// gets accepted and then matches nothing — see NormaliseTerm.
func validateAllowanceAgainstTarget(ctx context.Context, target, field, value string) (string, string, error) {
	if strings.TrimSpace(target) == "" {
		return "", "", fmt.Errorf("%w: an allowance needs a target", db.ErrAllowanceInvalid)
	}
	schema, err := addonsEntitlementSchema(target)
	switch {
	case errors.Is(err, addons.ErrNotRegistered):
		return "", "", fmt.Errorf("%w: %s is not a registered add-on target", db.ErrAllowanceInvalid, target)
	case err != nil:
		// Registered and silent. Nothing about the term can be checked against a
		// manifest we do not have, and recording a denial nobody has verified is
		// the failure this whole check exists for.
		return "", "", fmt.Errorf("%w: %s has not published a capability manifest yet, so its fields are unknown",
			db.ErrAllowanceInvalid, target)
	}
	// Lifecycle fields are included rather than skipped: they are not bindable
	// by a mapping and they are exactly what an operator suspension names.
	declared := make([]string, 0, len(schema))
	for _, f := range schema {
		declared = append(declared, f.Name)
	}
	return services.ValidateAllowanceTerm(declared, field, value)
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
		// The message is this layer's own, never the driver's. An unclassified
		// failure here is a constraint name, a column list, or a fragment of the
		// statement — a description of the schema, handed to whoever asked.
		log.Printf("[ALLOWANCE] unclassified failure: %v", err)
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR",
			"The allowance could not be written. The failure is in the server log.")
	}
}
