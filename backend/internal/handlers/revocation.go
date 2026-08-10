package handlers

import (
	"net/http"
	"strings"
	"time"

	"syndra/internal/addons"
	"syndra/internal/db"
	"syndra/internal/services"
	"syndra/internal/services/addonop"
)

// Revoking somebody's access to a target (design §10; change `addon-platform`
// 6.17).
//
// This target has no way to end a session. There is no `smb.status`, no
// `smb.sessions`, and no close or disconnect method — so "revoke access" cannot
// mean what an operator reasonably assumes it means, and the honest thing is to
// compose it out of the two things that CAN be done and say plainly what they
// achieve.
//
// The two halves:
//
//   - A subtractive allowance on `enabled`. It is not a lock applied to the
//     account directly: the resolver derives lifecycle state, so a direct lock
//     would be undone by the next convergence — while an allowance is part of
//     what the resolver reads, survives re-resolution, and is lifted rather than
//     forgotten. It is also what makes the suspension visible on every surface
//     the person appears on, instead of only on the NAS.
//   - A credential rotation. The allowance stops new authentications; it does
//     nothing to a client that already holds the password and will reconnect
//     with it. Rotating is what closes that, and it is a one-shot operation
//     because a credential cannot be queued.
//
// And what neither half does, stated in the response rather than left for an
// operator to discover: an established SMB session survives until it reconnects.

// revokeCopy is the sentence the surface must show. Held here rather than in the
// frontend because it is a statement about what the system did, and a frontend
// that paraphrased it would be paraphrasing a security guarantee.
const revokeCopy = "New connections are refused now and the credential has been replaced. " +
	"Sessions already established end when they next reconnect — this target has no way to close one."

type revokeRequest struct {
	Reason string `json:"reason"`
	// Confirmed is the operator acknowledging what this does and does not do.
	// The rotation half is confirmed by policy; this is the whole composition.
	Confirmed bool `json:"confirmed"`
	// ReviewDate bounds an indefinite suspension. A subtractive allowance must
	// be bounded by an expiry or a review date, and a revocation is the case
	// where an expiry is wrong — it is not meant to lapse on its own.
	ReviewDate *time.Time `json:"review_date,omitempty"`
}

// handleRevokeTargetAccess writes the disabling allowance and rotates the
// credential, in that order.
//
// The order is not incidental. The allowance is durable and takes effect on the
// next resolution; the rotation is a single call that may fail. Rotating first
// and failing to record the suspension would leave a member with a credential
// they cannot use and no record of why — and a retry would rotate again. This
// way a failed rotation leaves the access already withdrawn, and the operator is
// told exactly which half is outstanding.
func handleRevokeTargetAccess(w http.ResponseWriter, r *http.Request) {
	target, subject := r.PathValue("target"), r.PathValue("id")
	var req revokeRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}
	if strings.TrimSpace(req.Reason) == "" {
		// The reason is the row an operator reads six months later, on the
		// surface where the person still appears as suspended.
		jsonValidationErrorResponse(w, "A revocation needs a reason", map[string]string{"reason": "required"})
		return
	}
	if !req.Confirmed {
		jsonErrorResponse(w, http.StatusUnprocessableEntity, "CONFIRMATION_REQUIRED", revokeCopy)
		return
	}

	actor := resolveActor(r, "")
	reviewDate := req.ReviewDate
	if reviewDate == nil {
		// An indefinite suspension with no review date is one nobody ever looks
		// at again. Defaulted rather than refused, because refusing would make
		// the fastest path — withdraw access now, decide later — the one with an
		// extra field in it.
		d := time.Now().UTC().Add(defaultRevocationReview)
		reviewDate = &d
	}

	allowance, err := dbCreateAllowance(r.Context(), db.Allowance{
		SubjectID: subject, Target: target,
		// The lifecycle field, denied. `value` is the state being refused, which
		// is what the resolver reads it as.
		Field: services.FieldEnabled, Value: "true",
		Direction: db.AllowanceDeny, ActorID: actor, Reason: req.Reason,
		ReviewDate: reviewDate,
	})
	if err != nil {
		writeAllowanceError(w, err)
		return
	}

	// The convergence that carries the suspension to the target. Queued, like
	// every other add-on row — the response says so rather than implying the
	// account is already locked.
	set, err := svcResolveEntitlementsFor(r.Context(), subject, target)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "RESOLVE_FAILED", err.Error())
		return
	}
	if _, _, err := dbRecordSystemConvergence(r.Context(), db.SystemConvergence{
		Target: target, SubjectID: subject, Actor: actor,
		Reason: "Access revoked", Desired: set,
	}); err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "CONVERGENCE_NOT_QUEUED",
			"The suspension was recorded and the change to the target was not queued: "+err.Error())
		return
	}

	// The credential half. Its failure does not undo the suspension, and is
	// reported as the outstanding half rather than as a failed revocation —
	// which would invite a retry that rotates twice.
	res, rotateErr := svcDispatchOperation(r.Context(), addonop.Request{
		Target: target, Operation: "password.rotate",
		ActorID: actor, SubjectID: subject, Confirmed: true,
	})
	if rotateErr != nil || res.Outcome != addons.OutcomeSucceeded {
		detail := revokeCopy
		if rotateErr != nil {
			detail = "The suspension is recorded and queued. The credential was NOT replaced: " + rotateErr.Error()
		}
		jsonResponse(w, http.StatusAccepted, map[string]any{
			"status":       "partially_revoked",
			"allowance_id": allowance.ID,
			"rotated":      false,
			"detail":       detail,
			// Named so a surface can offer the one remaining action rather than
			// the whole composition again.
			"outstanding": "password.rotate",
		})
		return
	}

	jsonResponse(w, http.StatusAccepted, map[string]any{
		"status":       "revoked",
		"allowance_id": allowance.ID,
		"rotated":      true,
		"operation":    res.OperationID,
		// Queued, not applied. The lock reaches the target when the drain runs.
		"queued": true,
		// The other half of 9.18: this one DOES drain on its own. A revocation
		// is retained access until it lands, so it must not wait for somebody
		// to open the right page — and an operator who assumes it waits, like
		// a grant does, would go looking for a button that should not exist.
		"disclosure": "The credential is already replaced. The account lock drains on its own — " +
			"revocations do not wait for an operator, because a queued one is access somebody still has.",
		"detail": revokeCopy,
	})
}

// defaultRevocationReview is how long an unbounded suspension waits before it
// surfaces for a decision.
//
// Long enough not to nag, short enough that "we suspended them during the
// investigation" does not become a permanent state nobody remembers taking.
const defaultRevocationReview = 90 * 24 * time.Hour
