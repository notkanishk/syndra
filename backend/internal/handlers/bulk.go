package handlers

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"

	"syndra/internal/db"
	"syndra/internal/extcall"
	"syndra/internal/services"
)

// BulkGrantRequest is one access operation aimed at a set of people.
//
// Applying is still a query parameter (?apply=true) over the same body, but the
// body is no longer what authorises the write: `PlanID` is. The body is
// re-submitted because it carries what the write needs — the project, the role,
// the duration — and the plan binds it, so an operator who reviewed a 30-day
// grant cannot apply a permanent one by editing the field after the dialog.
type BulkGrantRequest struct {
	Op           string   `json:"op"`
	UserIDs      []string `json:"user_ids"`
	ProjectID    string   `json:"project_id,omitempty"`
	RoleKey      string   `json:"role_key,omitempty"`
	BundleID     string   `json:"bundle_id,omitempty"`
	Reason       string   `json:"reason"`
	DurationDays int      `json:"duration_days,omitempty"`
	// GrantIDs narrows `extend` to the grants the operator actually ticked. Omit it to extend
	// every expiring direct grant the named people hold — which is what selecting PEOPLE means,
	// and is not what selecting grant rows means.
	GrantIDs []string `json:"grant_ids,omitempty"`
	// PlanID cites the rehearsal being applied. Required with ?apply=true.
	PlanID string `json:"plan_id,omitempty"`
	// AcknowledgeScope is the operator saying the affected-subject count out
	// loud. Required only above the configured limit, and never bound to the
	// plan: it unlocks issuing the approval, it does not change what the
	// approval does.
	AcknowledgeScope bool `json:"acknowledge_scope,omitempty"`
}

// handleBulkGrants rehearses a bulk access change and, on ?apply=true, executes
// the plan the operator approved.
//
//	POST /api/v1/grants/bulk[?apply=true]
//
// Rehearsal is the default because a bulk write is the one operation whose
// blast radius an operator cannot hold in their head from a count. It now
// persists what it showed and returns a `plan_id`.
//
// **BREAKING:** apply no longer recomputes the diff. It cites that id, and the
// rows it acts on are the rows that were approved. Recomputation was the gap:
// the operator read one diff and a second request computed another, with
// grants, bundles and directory status free to move in between.
func handleBulkGrants(w http.ResponseWriter, r *http.Request) {
	var req BulkGrantRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}

	input := services.BulkRequest{
		Op:           req.Op,
		UserIDs:      req.UserIDs,
		ProjectID:    req.ProjectID,
		RoleKey:      req.RoleKey,
		BundleID:     req.BundleID,
		Reason:       req.Reason,
		DurationDays: req.DurationDays,
		GrantIDs:     req.GrantIDs,
	}
	if problems := services.ValidateBulkRequest(input); problems != nil {
		jsonValidationErrorResponse(w, "Invalid bulk request", problems)
		return
	}

	actor := resolveActor(r, "")
	input.Actor = actor
	requestFP := services.FingerprintBulkRequest(input)

	if r.URL.Query().Get("apply") != "true" {
		plan, err := svcRehearseBulk(r.Context(), input)
		if err != nil {
			jsonErrorResponse(w, http.StatusInternalServerError, "BULK_REHEARSAL_ERROR", err.Error())
			return
		}
		if err := issuePlan(r.Context(), planSurfaceBulkGrants, actor, requestFP, req.AcknowledgeScope, &plan); err != nil {
			writePlanIssueError(w, err)
			return
		}
		jsonResponse(w, http.StatusOK, plan)
		return
	}

	if strings.TrimSpace(req.PlanID) == "" {
		missingPlanCitation(w)
		return
	}
	// Every bulk op goes through the access-change planner (task 1.3). 202:
	// the write is recorded and the outbox carries it to Zitadel.
	applyPlannedBulk(w, r, input, planCitation(planSurfaceBulkGrants, actor, requestFP, req.PlanID), http.StatusAccepted)
}

// settleBulkRow says what became of a row whose writes are committed, from
// its delivery's answer. The answer is read, not discarded: draining and then
// marking the row applied regardless is the same lie one step later. With
// Zitadel offline every row once reported success while a bulk revoke sat
// unexecuted in the outbox.
//
// It sets the state only (effect, wait reason, retry) and keeps the row's
// plan words: "Gains Member. Giving Trained waits for you to send." is still
// true and complete after Apply, and the result's fold and the row's seat say
// the state (A-21 §2.6). A refused row's seat (the refusal and its step) is
// the screen's, from the wait reason (ui/src/lib/plan-groups.ts refusedSeat).
func settleBulkRow(out *services.BulkOutcome, queued string, retry *extcall.RetryState) {
	switch {
	case db.IsRefused(queued):
		// Zitadel refused it outright: terminal, never "queued" (final
		// verification N1).
		out.Effect = services.EffectFailed
		out.WaitReason = queued
	case queued != "":
		out.Effect = services.EffectQueued
		out.WaitReason, out.Retry = queued, retry
	default:
		out.Effect = services.EffectApplied
	}
}

// applyPlannedBulk applies a bulk plan through the planner. The service
// spends the plan, re-reads and re-plans under the lock, and writes only if
// every row's Change is the reviewed one; a stale plan is refused unspent
// with nothing written. Delivery follows the commit, outside the lock, as
// each row's Change decided it (services.DeliverChange), and each row is
// settled from its own outbox rows.
func applyPlannedBulk(w http.ResponseWriter, r *http.Request, input services.BulkRequest, cite db.PlanCitation, status int) {
	plan, ok := applyPlanned(w, r, input, cite)
	if ok {
		jsonResponse(w, status, plan)
	}
}

// applyPlanned is applyPlannedBulk's work, for a surface that answers with
// more than the plan (a publish adds its version). False when it answered
// with the refusal.
func applyPlanned(w http.ResponseWriter, r *http.Request, input services.BulkRequest, cite db.PlanCitation) (services.BulkPlan, bool) {
	plan, written, err := svcApplyPlannedBulk(r.Context(), input, cite)
	var stale *services.StalePlan
	switch {
	case errors.As(err, &stale):
		err = &staleSubjects{IDs: stale.IDs}
	case errors.Is(err, services.ErrPlanRequired):
		err = errPlanCitationMissing
	}
	var unanswered *services.ErrIncludersUnanswered
	switch {
	case err == nil:
	case strings.HasPrefix(err.Error(), "nothing to publish"):
		// Somebody else published while the dialog was open, or the draft was
		// discarded: a conflict with the world, not a server fault.
		jsonErrorResponse(w, http.StatusConflict, "NOTHING_TO_PUBLISH", err.Error()+". Nothing was changed.")
		return plan, false
	case errors.As(err, &unanswered):
		jsonErrorResponse(w, http.StatusConflict, "INCLUDERS_UNANSWERED",
			"Say whether each bundle that includes this one takes the new version: "+strings.Join(unanswered.Names, ", ")+". Nothing was changed.")
		return plan, false
	default:
		writePlanCitationError(w, err)
		return plan, false
	}
	for i := range plan.Outcomes {
		out := &plan.Outcomes[i]
		ch, wrote := written[out.UserID]
		if out.Effect != services.EffectApply || !wrote {
			continue
		}
		rebuildUserCacheDetachedFn(r.Context(), ch.UserID)
		settlePlannedRow(r.Context(), out, ch, svcDeliverChange(r.Context(), ch), input.Op)
	}
	plan.Summary = services.SummarizeOutcomes(plan.Outcomes)
	return plan, true
}

// settlePlannedRow says what became of one row from its delivery. A row
// with an act of its own (a give, an approval) is judged by its own write,
// and the roles a rule gave that wait are counted apart, never blamed on it
// (final verification N4).
func settlePlannedRow(ctx context.Context, out *services.BulkOutcome, ch services.Change, d services.Delivery, op string) {
	own := ""
	if op == services.BulkOpAssignRole || op == services.OpApprove {
		own = ch.Writes[0].OutboxID
	}
	if own == "" {
		queued, retry := delivered(d)
		settleBulkRow(out, queued, retry)
		return
	}
	queued, retry := d.ReasonFor(own), (*extcall.RetryState)(nil)
	if queued == db.WaitReasonHalted(db.HaltRetrying) {
		retry = d.Drain.Retry
	}
	if queued == "" && slices.Contains(d.Sent, own) {
		if st, err := dbGetPropagationStatus(ctx, own); err == nil && st != "" && st != "applied" {
			queued = db.WaitReasonHalted("not_confirmed")
		}
	}
	out.RuleWaiting = d.WaitingBesides(own)
	settleBulkRow(out, queued, retry)
}

// delivered is a delivery's answer for one bulk row: "" when every row it
// sent reached Zitadel, or why not.
func delivered(d services.Delivery) (string, *extcall.RetryState) {
	if why := d.WaitReason(); why != "" {
		return why, d.Drain.Retry
	}
	if d.Drain.Applied < len(d.Sent) {
		// Failed, or claimed by a concurrent drain: this request did not watch
		// it land, and nothing here says it goes on its own.
		return db.WaitReasonHalted("not_confirmed"), nil
	}
	return "", nil
}
