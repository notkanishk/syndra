package handlers

import (
	"errors"
	"net/http"
	"strings"

	"syndra/internal/addons"
	"syndra/internal/db"
	"syndra/internal/services/addonop"
)

// The unmanaged inventory and the one action that empties it (design §9, §12;
// change `addon-platform` 1.18/1.19, 6.8).
//
// A target holds accounts Syndra never provisioned. They are reported here and
// they are never drift: diffing them against expected state would classify every
// one of them as untraced access and bury the triage queue on the first sweep
// after deployment, and trust in a triage queue is set on the day it first fills.
//
// Adoption is the only way one becomes managed, and it is deliberately an
// operator's decision rather than an inference. The account may belong to
// somebody else entirely — adopting it hands a member their home directory,
// their shares and their group memberships, and the next convergence then makes
// that look like the intended state. There is no undo that gives the data back.

// handleTargetInventory lists what lives on the target that Syndra does not
// manage.
func handleTargetInventory(w http.ResponseWriter, r *http.Request) {
	inv, err := svcTargetInventory(r.Context(), r.PathValue("target"))
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "INVENTORY_ERROR", err.Error())
		return
	}
	if inv.Halted {
		// 503 rather than an empty list. An empty list is a statement that the
		// target holds nothing else, and the whole point of this surface is that
		// an operator acts on it.
		jsonResponse(w, http.StatusServiceUnavailable, inv)
		return
	}
	jsonResponse(w, http.StatusOK, inv)
}

type adoptRequest struct {
	// SubjectID is who the account becomes. Named in the body rather than taken
	// from the actor: an operator is adopting an account FOR somebody, and the
	// two are never the same person on this surface.
	SubjectID string `json:"subject_id"`
	// Confirmed is the operator saying the number out loud. The backend refuses
	// without it — a confirmation only the frontend enforces is a suggestion.
	Confirmed bool `json:"confirmed"`
}

// handleAdoptAccount binds an account the target already holds to a subject.
//
// One action, reached from two places: this inventory and the binding conflict
// an apply halts on. Design §11 requires them to leave identical state, and the
// way to guarantee that is for there to be one action rather than two that agree
// today.
func handleAdoptAccount(w http.ResponseWriter, r *http.Request) {
	target, username := r.PathValue("target"), r.PathValue("username")
	var req adoptRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}
	if strings.TrimSpace(req.SubjectID) == "" {
		jsonValidationErrorResponse(w, "Nobody to adopt it for", map[string]string{"subject_id": "required"})
		return
	}

	res, err := svcDispatchOperation(r.Context(), addonop.Request{
		Target:    target,
		Operation: "account.adopt",
		ActorID:   resolveActor(r, ""),
		SubjectID: req.SubjectID,
		Confirmed: req.Confirmed,
		Params:    map[string]any{"username": username},
	})
	if err != nil {
		writeAdoptionError(w, err)
		return
	}

	// An outcome that is not success is not an adoption, and it is answered as
	// what it was.
	//
	// This handler used to answer 200 "The account is now bound to that person"
	// for every outcome, including a refusal: the target said no, the binding
	// was correctly not written, and the operator was told it had worked. The
	// two states then disagreed with nothing on any surface saying so — the
	// exact shape of failure the queued/succeeded distinction exists to prevent,
	// arriving through the one action that hands somebody else's data over.
	switch res.Outcome {
	case addons.OutcomeSucceeded:
		// Falls through to the binding write below.
	case addons.OutcomeRejected:
		jsonErrorResponse(w, http.StatusConflict, "ADOPTION_REFUSED", adoptionRefusal(res.Err))
		return
	default:
		// Unreached and indeterminate differ in what may have happened on the
		// target, and neither is something to record here: the add-on's own
		// binding store is the authority, and the next inventory read reports
		// what it actually holds.
		jsonResponse(w, http.StatusAccepted, map[string]any{
			"status":    "unconfirmed",
			"operation": res.OperationID,
			"outcome":   res.Outcome,
			"detail":    "The target did not confirm the adoption. Nothing was recorded here; check the inventory before trying again.",
		})
		return
	}

	// The backend's own record, written only after the add-on confirmed it. The
	// add-on's store is what the apply consults; this is the copy the inventory
	// and the member's view read, and writing it first would have made an
	// account look managed that the add-on had refused to bind.
	if err := dbRecordTargetBinding(r.Context(), db.TargetBinding{
		Target: target, SubjectID: req.SubjectID,
		Username: username, BoundBy: resolveActor(r, ""),
		// The uid as the add-on read it off the target, not as this request
		// described the account. A binding recorded by name alone stops
		// recognising its account the moment somebody renames it on the
		// NAS, and the account then reappears as unmanaged — offered for
		// adoption to a second person while this binding still claims it.
		//
		// A pointer, and left nil when the add-on sent nothing: "the
		// account has uid 0" is root, and writing a zero for "we were not
		// told" would record the most dangerous account on the system.
		AccountUID: optionalUID(res.AccountUID),
	}); err != nil {
		// The adoption happened. Reported as a partial rather than a failure,
		// because retrying it would be a second adoption of an account that
		// is already bound — which the add-on answers from its dedup store,
		// but which an operator should not be told to do.
		jsonResponse(w, http.StatusAccepted, map[string]any{
			"status":    "adopted",
			"operation": res.OperationID,
			"warning":   "The target recorded the adoption and Syndra's own copy of it did not. It will be repaired by the next convergence.",
		})
		return
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"status":    "adopted",
		"operation": res.OperationID,
		"outcome":   res.Outcome,
		// Nothing on the account changed. Said plainly, because the natural
		// reading of "adopted" is that Syndra has now applied something to it,
		// and the next convergence is a separate decision.
		"detail": "The account is now bound to that person. Nothing on it was changed.",
	})
}

func writeAdoptionError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, addonop.ErrConfirmationRequired):
		jsonErrorResponse(w, http.StatusUnprocessableEntity, "CONFIRMATION_REQUIRED",
			"Adopting an account hands its home directory, shares and group memberships to that person. Confirm to continue.")
	case errors.Is(err, addons.ErrNotRegistered):
		jsonErrorResponse(w, http.StatusNotFound, "TARGET_NOT_REGISTERED", err.Error())
	case errors.As(err, new(*addons.ErrOperationUnavailable)):
		jsonErrorResponse(w, http.StatusConflict, "OPERATION_UNAVAILABLE", err.Error())
	default:
		jsonErrorResponse(w, http.StatusBadGateway, "ADOPTION_FAILED", err.Error())
	}
}

type lifecycleRequest struct {
	State  string `json:"state"`
	Reason string `json:"reason"`
}

// handleSetTargetLifecycle stops or resumes an add-on's writing, without a
// redeploy (design §18).
//
// Operator-gated and nothing more: it changes no access, and the states it sets
// are all more restrictive than serving — the worst outcome of a mistake here is
// that changes queue, which is the state the whole system is built to survive.
func handleSetTargetLifecycle(w http.ResponseWriter, r *http.Request) {
	var req lifecycleRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}
	if strings.TrimSpace(req.Reason) == "" {
		jsonValidationErrorResponse(w, "A state change needs a reason",
			map[string]string{"reason": "an operator reading the health surface has only this to go on"})
		return
	}
	out, err := addonsSetLifecycle(r.Context(), r.PathValue("target"), req.State, req.Reason)
	if err != nil {
		jsonErrorResponse(w, http.StatusBadGateway, "LIFECYCLE_NOT_SET", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, out)
}

// optionalUID distinguishes "uid zero" from "no uid was reported".
//
// They are not the same account and they are not the same statement. uid 0 is
// root; a missing value is an add-on that did not send one, and storing the
// first when the second is true would record a binding to the most privileged
// account on the target.
func optionalUID(uid int64) *int64 {
	if uid == 0 {
		return nil
	}
	return &uid
}

// adoptionRefusal is what the target said, or a sentence when it said nothing.
//
// The add-on's own words are the useful half — "already bound to another
// subject", "no adoptable account named root" — and losing them leaves an
// operator with a status code and a guess.
func adoptionRefusal(err error) string {
	if err == nil {
		return "The target refused the adoption."
	}
	return "The target refused the adoption: " + err.Error()
}
