package handlers

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"syndra/internal/addons"
	"syndra/internal/db"
	"syndra/internal/services"
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
	inv, err := svcTargetInventory(r.Context(), r.PathValue("target"), wantsFresh(r))
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "INVENTORY_ERROR", err.Error())
		return
	}
	if inv.RetryAfter > 0 {
		// "Read again" inside the add-on's cooldown, nothing kept to show:
		// a countdown, not an outage (RFC 6585 §4), said as every cooldown
		// refusal is (FRESH_READ_COOLDOWN).
		writeCooledDown(w, inv.RetryAfter, "That target was read moments ago, so its accounts can be listed again in "+
			strconv.Itoa(inv.RetryAfter)+" seconds.")
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
		About:     username,
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
	if err := recordAdoptedBinding(r.Context(), target, req.SubjectID, username, resolveActor(r, ""), res.AccountID); err != nil {
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

// recordAdoptedBinding is the backend's half of an adoption the add-on
// confirmed: its own copy of the binding. One function for the live answer
// (handleAdoptAccount) and for the answer the add-on recorded when the live
// one was lost (settleByCall, through db.ResolveBindingCall), so both leave
// the same row. actor is who asked for the adoption.
func recordAdoptedBinding(ctx context.Context, target, subject, username, actor, accountID string) error {
	return dbRecordTargetBinding(ctx, db.TargetBinding{
		Target: target, SubjectID: subject,
		Username: username, BoundBy: actor,
		// The account id as the add-on read it off the target, not as the
		// request described the account. A binding recorded by name alone
		// stops recognising its account the moment somebody renames it on the
		// NAS, and the account then reappears as unmanaged — offered for
		// adoption to a second person while this binding still claims it.
		//
		// A pointer, and left nil when the add-on sent nothing: writing an
		// empty id for "we were not told" would record an account nobody has.
		AccountID: optionalID(accountID),
	})
}

// forgetReleasedBinding is the backend's half of a release the add-on
// confirmed, one function for the live answer (handleReleaseBinding) and the
// recorded one (settleByCall), as recordAdoptedBinding is for an adoption.
// The observation goes with the binding: both are records of an account this
// deployment no longer manages, and a base that outlives its binding is
// compared on the next pass against whatever that subject is bound to next.
func forgetReleasedBinding(ctx context.Context, target, subject string) error {
	forgetMergeBase(ctx, target, subject)
	return dbForgetTargetBinding(ctx, target, subject)
}

func writeAdoptionError(w http.ResponseWriter, err error) {
	writeOperationError(w, err, "ADOPTION_FAILED",
		"Adopting an account hands its home directory, shares and group memberships to that person. Confirm to continue.")
}

// writeOperationError answers a dispatch that never reached an outcome.
//
// The refusal an operator meets is the confirmation one, and it is the reason
// this takes the sentence rather than composing one: "confirm to continue" with
// no statement of what is being confirmed is the shape of dialog people click
// through. Everything below it is a protocol fault and reads the same whichever
// operation asked.
func writeOperationError(w http.ResponseWriter, err error, code, confirmation string) {
	switch {
	case errors.Is(err, addonop.ErrConfirmationRequired):
		jsonErrorResponse(w, http.StatusUnprocessableEntity, "CONFIRMATION_REQUIRED", confirmation)
	case errors.Is(err, addons.ErrNotRegistered):
		jsonErrorResponse(w, http.StatusNotFound, "TARGET_NOT_REGISTERED", err.Error())
	case errors.As(err, new(*addons.ErrOperationUnavailable)):
		jsonErrorResponse(w, http.StatusConflict, "OPERATION_UNAVAILABLE", err.Error())
	default:
		writeUpstream(w, err, code)
	}
}

type releaseRequest struct {
	// Confirmed is the operator saying it out loud, and the backend refuses
	// without it for the same reason every other confirmed operation does.
	Confirmed bool `json:"confirmed"`
}

// handleReleaseBinding stops Syndra managing an account, and touches the
// account not at all.
//
// The safe half of a purge, and the second of the two resolutions the
// reconciliation surface names for a binding whose account is no longer on the
// target: re-provision it, or let it go. The add-on implemented letting go and
// nothing could reach it — a surface that names two resolutions and offers one
// is a surface that gets resolved the wrong way, and the one it offered was the
// irreversible one.
//
// BOTH stores are forgotten, and that is the whole point of this handler
// existing rather than the frontend calling the operation. The add-on's binding
// is what an apply consults; the backend's row is what the inventory, the roster
// and the reconciliation read. A release that drops one of them leaves the
// account managed by half the system: absent from the unmanaged inventory
// because the backend still claims it, and planned against an add-on that no
// longer binds it. The dormant sweep learned this the expensive way and its
// comment says so; this is the same lesson applied before it costs anything.
func handleReleaseBinding(w http.ResponseWriter, r *http.Request) {
	target, subject := r.PathValue("target"), r.PathValue("subject")
	var req releaseRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}

	res, err := svcDispatchOperation(r.Context(), addonop.Request{
		Target:    target,
		Operation: "account.release",
		ActorID:   resolveActor(r, ""),
		SubjectID: subject,
		Confirmed: req.Confirmed,
		About:     boundUsername(r.Context(), target, subject),
	})
	if err != nil {
		writeOperationError(w, err, "RELEASE_FAILED",
			"Syndra will stop managing that account. Nothing on the target changes and the account keeps working. Confirm to continue.")
		return
	}

	switch res.Outcome {
	case addons.OutcomeSucceeded:
		// Falls through to the backend's half below. The add-on answers a
		// subject it does not bind as succeeded rather than refused, which is
		// what makes this handler able to repair a split: an operator whose
		// backend row outlived the add-on's binding presses the same button.
	case addons.OutcomeRejected:
		jsonErrorResponse(w, http.StatusConflict, "RELEASE_REFUSED", releaseRefusal(res.Err))
		return
	default:
		// Nothing forgotten. The add-on's store is the authority on whether it
		// still binds this subject, and dropping the backend's copy against an
		// answer nobody received would manufacture the split this handler exists
		// to close.
		jsonResponse(w, http.StatusAccepted, map[string]any{
			"status":    "unconfirmed",
			"operation": res.OperationID,
			"outcome":   res.Outcome,
			"detail":    "The target did not confirm the release. Nothing was changed here; try again once it is reachable.",
		})
		return
	}

	if err := forgetReleasedBinding(r.Context(), target, subject); err != nil {
		// The add-on has already let go. Reported as a partial rather than a
		// failure, because pressing again is safe and is exactly the repair:
		// the add-on answers the second release as done, and this line runs
		// again.
		jsonResponse(w, http.StatusAccepted, map[string]any{
			"status":    "released",
			"operation": res.OperationID,
			"warning":   "The add-on released the binding and Syndra's own copy of it was not removed. Press release again to repair it.",
		})
		return
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"status":    "released",
		"operation": res.OperationID,
		"outcome":   res.Outcome,
		// Said in full, because "released" alone is a word an operator can read
		// as "removed". The account is untouched, and the next inventory read
		// offers it for adoption — which is what it now is.
		"detail": "Syndra no longer manages that account. Nothing on the target was changed, and it will appear as an unmanaged account on the next read.",
	})
}

// forgetMergeBase drops what the target was last seen holding for a subject
// this deployment has stopped managing.
//
// Logged rather than returned, on both paths that call it. The account is gone
// or the binding is released either way; failing to drop an observation costs
// one misattributed classification on a later pass, and reporting a completed
// release as failed would invite a retry of the one thing that has already
// happened.
func forgetMergeBase(ctx context.Context, target, subject string) {
	if err := dbForgetMergeBase(ctx, target, subject); err != nil {
		log.Printf("[TARGETS] stopped managing %s on %s and could not forget what it was last seen holding: %v",
			subject, target, err)
	}
	// The record of what Syndra landed goes too, and for the same reason: it is
	// evidence about an account this deployment no longer manages, and it would
	// be read as evidence about whatever that subject is bound to next.
	if err := dbForgetPropagations(ctx, target, subject); err != nil {
		log.Printf("[TARGETS] stopped managing %s on %s and could not forget what it had landed there: %v",
			subject, target, err)
	}
}

func releaseRefusal(err error) string {
	if err == nil {
		return "The target refused the release."
	}
	return "The target refused the release: " + err.Error()
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
	// The pipe commits the addon_operations row and its audit line (actor, target,
	// new state) before the call, so a flip is attributable. Which is why the
	// actor travels with it.
	out, err := addonsSetLifecycle(r.Context(), r.PathValue("target"), req.State, req.Reason, resolveActor(r, ""))
	if err != nil {
		writeUpstream(w, err, "LIFECYCLE_NOT_SET")
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
func optionalID(id string) *string {
	if id == "" {
		return nil
	}
	return &id
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

// handleDormantAccounts lists the accounts on a target whose reason for
// existing has gone (9.11/9.12).
//
// A read, and only a read. The removal that follows runs through the ordinary
// plan-then-apply path, so nothing here writes and nothing here queues.
func handleDormantAccounts(w http.ResponseWriter, r *http.Request) {
	report, err := svcDormantAccounts(r.Context(), r.PathValue("target"))
	if err != nil {
		if cooled := (services.ReadCooledDown{}); errors.As(err, &cooled) {
			writeCooledDown(w, cooled.RetryAfter, "That target was read moments ago, so its accounts can be listed again in "+
				strconv.Itoa(cooled.RetryAfter)+" seconds.")
			return
		}
		if errors.Is(err, services.ErrTargetUnplannable) {
			// 503, not an empty list. Every row here is a candidate for
			// removal, and an empty list is a statement that there are none —
			// which a read that did not happen cannot support.
			jsonErrorResponse(w, http.StatusServiceUnavailable, "TARGET_UNREADABLE",
				"That target could not be read, so nothing can be said about which of its accounts are dormant.")
			return
		}
		jsonErrorResponse(w, http.StatusInternalServerError, "DORMANT_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, report)
}

type dormantSweepRequest struct {
	// Accounts is an explicit list, never "everything dormant". The list an
	// operator saw is the list that is acted on, and a sweep that re-derived it
	// server-side would remove whatever had become dormant between the read and
	// the click.
	Accounts []string `json:"accounts"`
	// ElevatedKey is a delete-capable credential the operator supplies now. The
	// add-on holds none — its long-lived session can read and write accounts and
	// cannot remove one — so this is the only moment such a credential exists
	// anywhere in the deployment, and it exists for the length of one request.
	ElevatedKey string `json:"elevated_key"`
	Confirmed   bool   `json:"confirmed"`
}

// handleDormantSweep removes dormant accounts, one dispatch each.
//
// The only bulk action in the product, and the exception is principled: no
// active role grants any of these accounts, so removing them takes access from
// nobody. That is why it may be a sweep at all, and it is not a licence to add
// one elsewhere — every other revoke removes real access from a real person.
//
// The guard that makes it safe is not the ceremony, it is the RE-CHECK: every
// account named is resolved again here, and one that has become entitled since
// the operator read the list is refused rather than removed. A list is a moment,
// and this operation cannot be undone.
func handleDormantSweep(w http.ResponseWriter, r *http.Request) {
	target := r.PathValue("target")
	var req dormantSweepRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}
	switch {
	case len(req.Accounts) == 0:
		jsonValidationErrorResponse(w, "Nothing named to remove", map[string]string{"accounts": "required"})
		return
	case strings.TrimSpace(req.ElevatedKey) == "":
		jsonValidationErrorResponse(w, "A delete-capable credential is required",
			map[string]string{"elevated_key": "the add-on holds none of its own"})
		return
	case !req.Confirmed:
		jsonErrorResponse(w, http.StatusUnprocessableEntity, "CONFIRMATION_REQUIRED",
			"Their home directories and everything in them go with the accounts. There is no undo.")
		return
	}

	// Re-read, and re-resolve. The list the operator ticked was true when they
	// read it; this is the only thing that makes it true when it runs.
	report, err := svcDormantAccounts(r.Context(), target)
	if cooled := (services.ReadCooledDown{}); errors.As(err, &cooled) {
		// The list was read moments ago, inside the add-on's cooldown: the
		// re-check cannot read the target yet, and nothing is removed on the
		// strength of a copy (owner Q1). A countdown, then the same request.
		writeCooledDown(w, cooled.RetryAfter, "That target was read moments ago, so nothing was removed. You can remove them in "+
			strconv.Itoa(cooled.RetryAfter)+" seconds.")
		return
	}
	if err != nil {
		jsonErrorResponse(w, http.StatusServiceUnavailable, "TARGET_UNREADABLE",
			"That target could not be read, so nothing was removed.")
		return
	}
	removable := map[string]string{} // account -> subject
	for _, account := range report.Accounts {
		// Still-a-member rows are excluded HERE as well as in the surface.
		// Removing one locks somebody out rather than tidying up, and a
		// client-side filter is a suggestion.
		if !account.SubjectStillMember {
			removable[account.Account] = account.SubjectID
		}
	}

	actor := resolveActor(r, "")
	outcomes := make([]map[string]any, 0, len(req.Accounts))
	removed := 0
	for _, name := range req.Accounts {
		subject, ok := removable[name]
		if !ok {
			outcomes = append(outcomes, map[string]any{
				"account": name, "outcome": "refused",
				"detail": "No longer dormant, or no longer removable here. Nothing was done to it.",
			})
			continue
		}
		res, err := svcDispatchOperation(r.Context(), addonop.Request{
			Target: target, Operation: "account.purge",
			ActorID: actor, SubjectID: subject, Confirmed: true,
			Params: map[string]any{"elevated_key": req.ElevatedKey},
			About:  name,
		})
		switch {
		case err != nil:
			outcomes = append(outcomes, map[string]any{
				"account": name, "outcome": "refused", "detail": err.Error(),
			})
		case res.Outcome == addons.OutcomeSucceeded:
			removed++
			// The binding goes with the account, and this is the call that was
			// missing. `ForgetTargetBinding`'s own docblock says why: the apply
			// path reads bound-but-absent as an out-of-band deletion and
			// RECREATES the account under the recorded name — right for one
			// somebody else deleted, and exactly wrong for one we just purged.
			// The add-on drops its own binding as part of the purge; this is the
			// backend's half, and without it the two stores disagreed.
			//
			// Logged rather than failed: the account is gone either way, and
			// reporting a completed purge as refused would invite a retry of
			// the one operation where retrying is not free. A stale binding is
			// visible on the roster; an unexplained second purge is not.
			forgetMergeBase(r.Context(), target, subject)
			if err := dbForgetTargetBinding(r.Context(), target, subject); err != nil {
				log.Printf("[DORMANT] purged %s on %s and could not forget its binding: %v", name, target, err)
			}
			outcomes = append(outcomes, map[string]any{"account": name, "outcome": "removed"})
		default:
			// Unreached and indeterminate both mean nobody can say. Never
			// retried automatically: a purge that may have happened is the one
			// operation where trying again is not free.
			outcomes = append(outcomes, map[string]any{
				"account": name, "outcome": string(res.Outcome),
				"detail": "The target did not confirm this one. Check it before trying again.",
			})
		}
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"target": target, "removed": removed, "outcomes": outcomes,
	})
}

// boundUsername is the account name Syndra's own binding records for a
// subject, for a release's record; empty when there is none to name.
func boundUsername(ctx context.Context, target, subject string) string {
	if b, bound, err := dbGetTargetBinding(ctx, target, subject); err == nil && bound {
		return b.Username
	}
	return ""
}
