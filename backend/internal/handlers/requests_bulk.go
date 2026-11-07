package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"syndra/internal/services"
)

// Bulk request decisions, rehearsed — the third consumer of the one plan shape.
//
//	POST /api/v1/requests/bulk-decision[?apply=true]
//
// Approving in bulk is the operation with the least obvious blast radius on the
// whole product: each approval mints a direct grant, so "approve 9" is nine
// access changes wearing the clothes of an inbox action. The rehearsal says
// what each one grants and to whom before any of it happens.
//
// Applying calls the same resolveOneAccessRequest the single-request endpoint
// does. That sequence — conditional transaction, race guard, cache rebuild,
// inline drain — is the part that must not diverge: a second implementation
// that drifted would leave requests approved but ungranted, which re-surfaces
// later as syndra_only drift and is diagnosed by nobody.

type bulkDecisionRequest struct {
	IDs        []string `json:"ids"`
	Status     string   `json:"status"`
	ReviewNote string   `json:"review_note"`
	// PlanID cites the rehearsal being applied. Required with ?apply=true.
	PlanID string `json:"plan_id,omitempty"`
	// AcknowledgeScope is the operator saying the affected-request count out loud.
	AcknowledgeScope bool `json:"acknowledge_scope,omitempty"`
}

func handleBulkDecideRequests(w http.ResponseWriter, r *http.Request) {
	var req bulkDecisionRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}

	status := strings.ToLower(strings.TrimSpace(req.Status))
	if status != "approved" && status != "rejected" {
		jsonValidationErrorResponse(w, "status must be approved or rejected", map[string]string{"status": "enum"})
		return
	}
	if len(dedupeNonEmpty(req.IDs)) == 0 {
		jsonValidationErrorResponse(w, "at least one request id is required", map[string]string{"ids": "required"})
		return
	}
	if len(dedupeNonEmpty(req.IDs)) > services.BulkMaxUsers {
		jsonValidationErrorResponse(w, "too many requests in one batch",
			map[string]string{"ids": fmt.Sprintf("max %d", services.BulkMaxUsers)})
		return
	}
	// The same reviewer gate the single-request endpoint applies. An approval
	// mints a grant, and a grant attributed to "system" is a grant nobody can be
	// asked about — in bulk, that would be true of the whole batch at once.
	if status == "approved" && resolveActor(r, "") == "system" {
		jsonValidationErrorResponse(w, "an authenticated reviewer is required when approving",
			map[string]string{"reviewer": "required_when=status:approved"})
		return
	}

	ids := dedupeNonEmpty(req.IDs)
	actor := resolveActor(r, "")
	// The decision itself is bound, along with the cohort: approving and
	// rejecting are opposite acts, and an apply that flipped one under the other
	// approval would mint grants from a review that declined them. The review
	// note is not bound — it is written at apply time and changes nothing.
	requestFP := services.FingerprintIDCohort("decide_requests", ids, "status", status)

	// Approving goes through the planner (task 1.3): each request is the same
	// Change a single approval is, on one snapshot for the batch. Declining
	// changes nobody's access and is not planned.
	input := services.BulkRequest{Op: services.OpApprove, UserIDs: ids, Actor: actor, Note: req.ReviewNote}
	if r.URL.Query().Get("apply") != "true" {
		var plan services.BulkPlan
		if status != "approved" {
			plan = rehearseDeclines(r.Context(), ids)
		} else {
			var err error
			if plan, err = svcRehearseBulk(r.Context(), input); err != nil {
				jsonErrorResponse(w, http.StatusInternalServerError, "BULK_REHEARSAL_ERROR", err.Error())
				return
			}
		}
		plan.Op = "decide_requests"
		if err := issuePlan(r.Context(), planSurfaceBulkDecision, actor, requestFP, req.AcknowledgeScope, &plan); err != nil {
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
	if status == "approved" {
		if plan, ok := applyPlanned(w, r, input, planCitation(planSurfaceBulkDecision, actor, requestFP, req.PlanID)); ok {
			plan.Op = "decide_requests"
			jsonResponse(w, http.StatusOK, plan)
		}
		return
	}
	var live map[string]services.BulkOutcome
	subjects, err := claimPlan(r.Context(), planSurfaceBulkDecision, actor, requestFP, req.PlanID,
		func() map[string]services.BulkOutcome {
			live = indexOutcomes(rehearseDeclines(r.Context(), ids).Outcomes)
			return live
		})
	if err != nil {
		writePlanCitationError(w, err)
		return
	}
	plan := planApprovedRows("decide_requests", subjects, live)
	for i := range plan.Outcomes {
		out := &plan.Outcomes[i]
		if out.Effect != services.EffectApply {
			continue
		}
		if _, err := svcDecideRequest(r.Context(), out.UserID, "rejected", actor, req.ReviewNote, nil); err != nil {
			out.Effect, out.Detail = services.EffectFailed, "Didn't go through: "+err.Error()
			continue
		}
		out.Effect = services.EffectApplied
	}
	plan.Summary = services.SummarizeOutcomes(plan.Outcomes)
	jsonResponse(w, http.StatusOK, plan)
}

// rehearseDeclines is the plan for declining: who asked, and that their
// access stays as it is.
func rehearseDeclines(ctx context.Context, ids []string) services.BulkPlan {
	plan := services.BulkPlan{Op: "decide_requests", Outcomes: []services.BulkOutcome{}}
	for _, id := range ids {
		out := services.BulkOutcome{UserID: id}
		request, err := dbGetAccessRequestByID(ctx, id)
		switch {
		case err != nil:
			out.Effect, out.Detail = services.EffectBlocked, "No such request. It may have been withdrawn."
			out.Consequence = "Nothing for you to do."
			out.Fingerprint = services.Fingerprint("request", id, "absent")
		case request.Status != "pending":
			// A queue worked by two people produces this constantly: somebody
			// already decided. Information, not an error.
			out.Name, out.Email = driftSubject(ctx, request.RequesterID), request.RequesterID
			out.Fingerprint = services.FingerprintAccessRequest(request)
			out.Effect, out.Detail = services.EffectNoChange, services.SettledDetail(request.Status)
		default:
			out.Name, out.Email = driftSubject(ctx, request.RequesterID), request.RequesterID
			out.Fingerprint = services.FingerprintAccessRequest(request)
			out.Effect, out.Detail = services.EffectApply, fmt.Sprintf("Declined for %s.", request.RoleKey)
			out.Consequence = "Nothing about their current access changes."
		}
		plan.Outcomes = append(plan.Outcomes, out)
	}
	plan.Summary = services.SummarizeOutcomes(plan.Outcomes)
	return plan
}
