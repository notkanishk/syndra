package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"syndra/internal/models"
	"syndra/internal/services"
)

// Bulk drift resolution, rehearsed — the same two-pass shape as bulk grants,
// returning the same plan type.
//
// That sameness is the point. An operator should not have to learn what "will
// change" looks like separately on each screen, and one plan shape means one
// renderer in the UI rather than a second dialect of the same idea.
//
// Adopting writes a direct-grant record; marking-external suppresses future
// detection for that grant. Neither is trivially undone from the console, and
// a triage queue is exactly where an operator is moving fast — so the plan is
// computed first and shown, and only then applied.
//
// There is still deliberately no bulk revoke. Adopting and marking-external are
// reversible bookkeeping; revoking removes real access from real machines, and
// reading twelve consequences at once is not something anyone actually does.

const (
	driftOpAdopt    = "adopt"
	driftOpExternal = "mark_external"
)

// rehearseDriftBatch evaluates each id against current state without writing.
// Rows that have already left the queue are reported as such rather than
// silently dropped: a triage queue is worked by more than one person, and
// "somebody resolved this while you were reading" is information.
func rehearseDriftBatch(ctx context.Context, ids []string, op string) services.BulkPlan {
	plan := services.BulkPlan{Op: op}
	seen := map[string]bool{}

	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		plan.Outcomes = append(plan.Outcomes, rehearseOneDrift(ctx, id, op))
	}

	plan.Summary = services.SummarizeOutcomes(plan.Outcomes)
	return plan
}

func rehearseOneDrift(ctx context.Context, id, op string) services.BulkOutcome {
	out := services.BulkOutcome{UserID: id}

	item, err := dbGetDriftItem(ctx, id)
	if err != nil {
		out.Effect = services.EffectBlocked
		out.Detail = "No longer in the queue — somebody may have resolved it already."
		// Absence is a state to re-verify like any other: a row that reappears
		// between the review and the apply is a row nobody reviewed.
		out.Fingerprint = services.Fingerprint("drift", id, "unreadable")
		return out
	}
	// The drift row's own status is in here, and that is the point (design §8).
	// Somebody resolving a row while the operator reads the list is the ordinary
	// case in a shared queue, and it is invisible to any fingerprint taken over
	// the access alone — the grants have not moved and the person has not moved.
	out.Fingerprint = services.FingerprintDriftItem(item)

	// Name the person, not the drift row's uuid. A plan an operator cannot read
	// is a plan they will approve without reading.
	out.Name = driftSubject(ctx, item.UserID)
	out.Email = item.UserID

	if item.Status != "" && item.Status != "pending_triage" {
		out.Effect = services.EffectNoChange
		out.Detail = fmt.Sprintf("Already resolved as %s.", humanDriftStatus(item.Status))
		return out
	}

	out.Effect = services.EffectApply
	switch op {
	case driftOpAdopt:
		out.Detail = fmt.Sprintf("Adopted into Syndra (%s).", roleSummary(item.RoleKeys))
		out.Consequence = "Records a direct grant, so the access stops being unexplained and starts being owned."
	case driftOpExternal:
		out.Detail = fmt.Sprintf("Marked as owned elsewhere (%s).", roleSummary(item.RoleKeys))
		out.Consequence = "Syndra stops reporting it. The access itself is left exactly as it is."
	}
	return out
}

func driftSubject(ctx context.Context, userID string) string {
	profile, found, err := directorySource().FindUser(ctx, userID)
	if err != nil || !found || strings.TrimSpace(profile.Name) == "" {
		return "Unknown account"
	}
	return profile.Name
}

func roleSummary(roles []string) string {
	switch len(roles) {
	case 0:
		return "no roles"
	case 1:
		return roles[0]
	case 2:
		return roles[0] + " and " + roles[1]
	default:
		return fmt.Sprintf("%s and %d more", roles[0], len(roles)-1)
	}
}

func humanDriftStatus(status string) string {
	switch status {
	case "attributed":
		return "adopted"
	case "marked_external":
		return "owned elsewhere"
	case "revoked":
		return "revoked"
	}
	return status
}

// claimDriftPlan spends the approval a drift apply cites and re-verifies every
// row against the queue as it stands now.
//
// The re-read is the same one `rehearseOneDrift` does, hashed by the same
// function, so "did this row move" is asked exactly as it was answered. A row
// somebody else resolved in the meantime fails here — mutating nothing, and
// naming itself — rather than being adopted twice or marked external over
// somebody's revocation.
func claimDriftPlan(r *http.Request, surface, op, actor, requestFP, planID string, ids []string) (services.BulkPlan, error) {
	if strings.TrimSpace(planID) == "" {
		return services.BulkPlan{}, errPlanCitationMissing
	}
	// The same rehearsal the operator read, run again: its fingerprints answer
	// "did this row move" exactly as they asked it, and its sentences are what
	// the result is rendered from. Deferred until the citation is accepted.
	var live map[string]services.BulkOutcome
	subjects, err := claimPlan(r.Context(), surface, actor, requestFP, planID,
		func() map[string]services.BulkOutcome {
			live = indexOutcomes(rehearseDriftBatch(r.Context(), ids, op).Outcomes)
			return live
		})
	if err != nil {
		return services.BulkPlan{}, err
	}
	return planApprovedRows(op, subjects, live), nil
}

// applyDriftPlan executes the actionable rows in place, rewriting each outcome
// to what actually happened. Per-row failure is isolated: a batch that aborts
// halfway leaves an operator unable to tell which half landed.
func applyDriftPlan(r *http.Request, plan *services.BulkPlan, op, actor, reason string) {
	plan.Applied = true

	for i := range plan.Outcomes {
		out := &plan.Outcomes[i]
		if out.Effect != services.EffectApply {
			continue
		}

		item, err := dbGetDriftItem(r.Context(), out.UserID)
		if err != nil {
			out.Effect = services.EffectFailed
			out.Detail = "Couldn't re-read it: " + err.Error()
			continue
		}

		if err := applyOneDrift(r, item, op, actor, reason); err != nil {
			out.Effect = services.EffectFailed
			out.Detail = "Didn't go through: " + err.Error()
			continue
		}
		out.Effect = services.EffectApplied
	}

	plan.Summary = services.SummarizeOutcomes(plan.Outcomes)
}

func applyOneDrift(r *http.Request, item models.DriftItem, op, actor, reason string) error {
	switch op {
	case driftOpAdopt:
		return attributeOneDrift(r.Context(), item, attributeRequest{Source: "external_backfill"}, actor)
	case driftOpExternal:
		return dbMarkDriftExternalTx(r.Context(), item.ID, item.UserID, item.ProjectID,
			item.RoleKeys, actor, reason, "{}")
	}
	return fmt.Errorf("unsupported drift operation %q", op)
}
