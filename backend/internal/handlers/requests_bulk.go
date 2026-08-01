package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"mkauth/internal/services"
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
// later as mkauth_only drift and is diagnosed by nobody.

type bulkDecisionRequest struct {
	IDs        []string `json:"ids"`
	Status     string   `json:"status"`
	ReviewNote string   `json:"review_note"`
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

	plan := rehearseDecisionBatch(r.Context(), dedupeNonEmpty(req.IDs), status)
	if r.URL.Query().Get("apply") != "true" {
		jsonResponse(w, http.StatusOK, plan)
		return
	}

	applyDecisionPlan(r, &plan, status, req.ReviewNote)
	jsonResponse(w, http.StatusOK, plan)
}

func rehearseDecisionBatch(ctx context.Context, ids []string, status string) services.BulkPlan {
	plan := services.BulkPlan{Op: "decide_requests"}

	for _, id := range ids {
		out := services.BulkOutcome{UserID: id}

		request, err := dbGetAccessRequestByID(ctx, id)
		if err != nil {
			out.Effect = services.EffectBlocked
			out.Detail = "No such request — it may have been withdrawn."
			plan.Outcomes = append(plan.Outcomes, out)
			continue
		}

		out.Name = driftSubject(ctx, request.RequesterID)
		out.Email = request.RequesterID

		if request.Status == "approved" || request.Status == "rejected" {
			// A queue worked by two people produces this constantly, and it is
			// information rather than an error: somebody already decided.
			out.Effect = services.EffectNoChange
			out.Detail = fmt.Sprintf("Already %s.", request.Status)
			plan.Outcomes = append(plan.Outcomes, out)
			continue
		}

		out.Effect = services.EffectApply
		if status == "approved" {
			out.Detail = fmt.Sprintf("Gains %s.", request.RoleKey)
			// The part an inbox action hides: this is an access change.
			out.Consequence = "Approving mints a direct grant, the same as granting it by hand."
			if request.DurationDays != nil && *request.DurationDays > 0 {
				out.Consequence = fmt.Sprintf(
					"Approving mints a direct grant for %d days, the same as granting it by hand.",
					*request.DurationDays)
			}
		} else {
			out.Detail = fmt.Sprintf("Declined for %s.", request.RoleKey)
			out.Consequence = "Nothing about their current access changes."
		}
		plan.Outcomes = append(plan.Outcomes, out)
	}

	plan.Summary = services.SummarizeOutcomes(plan.Outcomes)
	return plan
}

func applyDecisionPlan(r *http.Request, plan *services.BulkPlan, status, note string) {
	plan.Applied = true

	for i := range plan.Outcomes {
		out := &plan.Outcomes[i]
		if out.Effect != services.EffectApply {
			continue
		}

		if err := decideOneRequest(r, out.UserID, status, note); err != nil {
			out.Effect = services.EffectFailed
			out.Detail = "Didn't go through: " + err.Error()
			continue
		}
		out.Effect = services.EffectApplied
	}

	plan.Summary = services.SummarizeOutcomes(plan.Outcomes)
}

// decideOneRequest runs the very same decision the single-request endpoint
// runs — extracted rather than replayed over HTTP, so the approval transaction,
// the race guard, the cache rebuild and the inline drain cannot drift apart
// between the one-at-a-time path and the bulk one.
func decideOneRequest(r *http.Request, id, status, note string) error {
	return resolveOneAccessRequest(r.Context(), id, status, resolveActor(r, ""), note)
}
