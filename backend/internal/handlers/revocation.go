package handlers

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"syndra/internal/addons"
	"syndra/internal/clock"
	"syndra/internal/db"
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
	ReviewDate *clock.EndDay `json:"review_date,omitempty"`
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

	// The account field, by role, from the role map persisted at registration
	// (design §3): a revocation never waits on the add-on being reachable.
	roles, err := addonsFieldRoles(r.Context(), target)
	if err != nil || roles.Account() == "" {
		jsonErrorResponse(w, http.StatusConflict, "ACCOUNT_FIELD_UNKNOWN",
			"This system has not said which of its fields switches an account off, so nothing could be held. Connect its add-on once, then try again.")
		return
	}

	actor := resolveActor(r, "")
	reviewDate := req.ReviewDate.Instant()
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
		// The account field, denied. `value` is the state being refused, which
		// is what the resolver reads it as.
		Field: roles.Account(), Value: "true",
		Direction: db.AllowanceDeny, ActorID: actor, Reason: req.Reason,
		ReviewDate: reviewDate,
	})
	// The same hold already in force is how a RETRY arrives: the deny committed,
	// then the resolve, the convergence record or the rotation failed. Refusing
	// it would leave the lock unqueued and the credential unrotated for good, so
	// carry on with the existing allowance (CreateAllowance returns it).
	//
	// Only a PERMANENT one, though. A hold that lapses is somebody's
	// time-boxed pause, and "revoke" must not quietly ride on it and rotate the
	// credential: it answers 409, and the operator decides.
	retry := errors.Is(err, db.ErrAllowanceAlreadyInForce)
	if err != nil && (!retry || allowance.ExpiresAt != nil) {
		writeAllowanceError(w, err)
		return
	}

	// A retry may find its convergence already waiting (the first attempt
	// queued it and failed later, at the rotation). Queuing another would be a
	// second row for an operator to look at, for the same lock.
	var outboxID string
	if retry {
		outboxID, err = dbUnfinishedConvergence(r.Context(), subject, target)
		if err != nil {
			jsonErrorResponse(w, http.StatusInternalServerError, "CONVERGENCE_CHECK_FAILED", err.Error())
			return
		}
	}

	// The convergence that carries the suspension to the target. Queued, like
	// every other add-on row — the response says so rather than implying the
	// account is already locked.
	if outboxID == "" {
		set, err := svcResolveEntitlementSet(r.Context(), subject, target)
		if err != nil {
			jsonErrorResponse(w, http.StatusInternalServerError, "RESOLVE_FAILED", err.Error())
			return
		}
		// The set was resolved with `enabled` denied, so it disables and cannot
		// confer: EntitlementSet.WithdrawsOnly declares it, which is what lets
		// the background runner drain the lock. Without it the disclosure below
		// is a promise nothing keeps.
		if _, outboxID, err = dbRecordSystemConvergence(r.Context(), set.Convergence(target, subject, actor, "Their access was revoked.")); err != nil {
			jsonErrorResponse(w, http.StatusInternalServerError, "CONVERGENCE_NOT_QUEUED",
				"The suspension was recorded and the change to the target was not queued: "+err.Error())
			return
		}
	}
	// Recorded is the outbox row's time, not the allowance's: the row is the
	// revocation the unconfirmed list shows and revocation.landed closes, so
	// the reply, the list and the audit name one moment.
	recordedAt, err := dbPropagationRecordedAt(r.Context(), outboxID)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "CONVERGENCE_CHECK_FAILED", err.Error())
		return
	}

	// The credential half. Its failure does not undo the suspension, and is
	// reported as the outstanding half rather than as a failed revocation —
	// which would invite a retry that rotates twice.
	res, rotateErr := svcDispatchOperation(r.Context(), addonop.Request{
		Target: target, Operation: "password.rotate",
		ActorID: actor, SubjectID: subject, Confirmed: true,
	})
	rotated := rotateErr == nil && res.Outcome == addons.OutcomeSucceeded

	// Recorded and landed are two facts with two times (design §13.4). The
	// deny is recorded now; it lands only when the add-on answers succeeded to
	// the withdrawal, which the drain sends by itself, without limit. Never
	// drawn as done before then.
	system, _ := systemName(r.Context(), target)
	body := map[string]any{
		"status":       "recorded",
		"allowance_id": allowance.ID,
		"recorded_at":  recordedAt,
		"landed_at":    nil,
		"rotated":      rotated,
		"detail":       notYetLanded(r.Context(), target, system, rotated),
	}
	if rotated {
		body["operation"] = res.OperationID
	} else {
		// Named so a surface can offer the one remaining action rather than
		// the whole composition again, and said in the words of every write.
		body["outstanding"] = "password.rotate"
		body["rotation"] = rotationAnswer(r.Context(), target, system, res, rotateErr)
	}
	jsonResponse(w, http.StatusAccepted, body)
}

// notYetLanded is what a recorded revocation says until it lands: recorded,
// not yet applied, why when the monitor holds the system down, that the
// account still works meanwhile, and that Syndra applies it by itself.
func notYetLanded(ctx context.Context, target, system string, rotated bool) string {
	out := "Recorded. Not yet applied on " + system + "."
	if why, held := addonsHeld(target); held {
		n := notLandedOf(ctx, target, addons.OutcomeUnreached, why)
		out = "Recorded. Not yet applied on " + system + ": " + whySentence(n)
	}
	out += " Until it is, the account there still works. Syndra applies it by itself when " + system + " answers."
	if rotated {
		out += " The credential is already replaced. Sessions already established end when they next reconnect — this system has no way to close one."
	}
	return out
}

// rotationAnswer is the credential half that did not land, each state with
// its own next step: a refusal needs its reason dealt with; unreached and
// indeterminate answer through writeOutcome, whose indeterminate says not to
// send it again (a second rotation on an account that did rotate locks the
// member out of it).
func rotationAnswer(ctx context.Context, target, system string, res addonop.Result, dispatchErr error) any {
	switch {
	case dispatchErr != nil && res.OperationID == "":
		// Never sent. The backend refused it before it left, so the reason is
		// this deployment's own and is safe to show.
		return map[string]any{"status": "not_sent", "detail": "The credential was not replaced: " + dispatchErr.Error()}
	case dispatchErr != nil:
		// Sent, and its outcome not recorded: as unknown as a lost answer.
		res.Outcome = addons.OutcomeIndeterminate
	case res.Outcome == addons.OutcomeRejected:
		return map[string]any{"status": "refused",
			"detail": "The credential was not replaced: " + system + " refused: " + errText(res.Err) + " Replace it once that is resolved."}
	}
	_, answer, _ := outcomeAnswer(notLandedOf(ctx, target, res.Outcome, res.Why), map[string]any{"operation": res.OperationID})
	return answer
}

// defaultRevocationReview is how long an unbounded suspension waits before it
// surfaces for a decision.
//
// Long enough not to nag, short enough that "we suspended them during the
// investigation" does not become a permanent state nobody remembers taking.
const defaultRevocationReview = 90 * 24 * time.Hour
