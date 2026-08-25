package handlers

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"syndra/internal/db"
	"syndra/internal/services"
	"syndra/internal/services/planapply"
)

// Converging a cohort's entitlements on an add-on target (design §4, §8).
//
// Two requests and one approval between them, the same shape every other
// planned surface uses: `rehearse` computes the diff and records it, `apply`
// cites the id it was given. What differs is only what the plan is ABOUT — a
// resolved desired-state set per subject rather than a grant — and where the
// diff comes from: the add-on computes it, because Syndra does not know what
// `lab_makers` means on a NAS.
//
// The apply queues; it does not converge. Add-on rows wait for an operator to
// resume the drain, exactly as Zitadel grants do, and the response says queued
// rather than applied for that reason.

// planSurfaceEntitlements is where these approvals are citable, and nowhere
// else. A plan issued here names subjects whose "outcome" is a convergence; the
// same ids on the bulk-grant endpoint mean a role assignment.
const planSurfaceEntitlements = "entitlements.converge"

type entitlementRehearseRequest struct {
	SubjectIDs       []string `json:"subject_ids"`
	AcknowledgeScope bool     `json:"acknowledge_scope"`
}

type entitlementApplyRequest struct {
	PlanID           string   `json:"plan_id"`
	SubjectIDs       []string `json:"subject_ids"`
	AcknowledgeScope bool     `json:"acknowledge_scope"`
}

// handleRehearseEntitlements computes the change and issues the approval for it.
func handleRehearseEntitlements(w http.ResponseWriter, r *http.Request) {
	target := r.PathValue("target")
	var req entitlementRehearseRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}
	if len(req.SubjectIDs) > services.BulkMaxUsers {
		jsonValidationErrorResponse(w, "Too many subjects in one request",
			map[string]string{"subject_ids": "at most " + strconv.Itoa(services.BulkMaxUsers) + " may be converged in one request"})
		return
	}

	actor := resolveActor(r, "")
	plan, err := svcRehearseEntitlements(r.Context(), services.EntitlementRehearsal{
		Target: target, SubjectIDs: req.SubjectIDs, Actor: actor,
		AcknowledgeScope: req.AcknowledgeScope,
	})
	if err != nil {
		writeEntitlementError(w, err)
		return
	}

	if err := issueEntitlementPlan(r.Context(), target, actor, req.AcknowledgeScope, &plan); err != nil {
		writePlanIssueError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, plan)
}

// handleApplyEntitlements spends an approval and queues what it approved.
func handleApplyEntitlements(w http.ResponseWriter, r *http.Request) {
	target := r.PathValue("target")
	var req entitlementApplyRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}
	if req.PlanID == "" {
		missingPlanCitation(w)
		return
	}

	// The cohort travels with the citation and is recomputed into the same
	// fingerprint the rehearsal bound. It is not used to recompute the diff —
	// that is the thing this design removed — but a citation whose cohort does
	// not match the approved one is an approval being spent on a different
	// selection, and the claim predicate is where that loses.
	res, err := svcApplyEntitlements(r.Context(), planapply.Request{
		PlanID:  req.PlanID,
		Target:  target,
		Surface: planSurfaceEntitlements,
		Actor:   resolveActor(r, ""),
		// The acknowledgement is deliberately NOT part of it. It is a
		// permission to approve a large change, spent when the plan is issued;
		// binding it here would make an apply fail because a client did not
		// echo back a flag that has already done its work.
		RequestFingerprint: services.FingerprintIDCohort(services.EntitlementOp, req.SubjectIDs),
	})
	if err != nil {
		writeEntitlementError(w, err)
		return
	}
	jsonResponse(w, http.StatusAccepted, res)
}

// issueEntitlementPlan records the rehearsal so it can be cited.
//
// The blast-radius guard runs first and counts the subjects that would CHANGE
// rather than the ones selected: a selection of two hundred people that turns
// out to converge three of them is a small change reviewed as a large one, and
// refusing it would train operators to acknowledge everything.
//
// Every rehearsed subject is recorded, including the blocked and unchanged ones.
// A plan is the record of what an operator approved, and re-verifying a blocked
// row is meaningful in its own right — a subject blocked by a binding conflict
// at review time and resolved since is exactly the case the block existed for.
func issueEntitlementPlan(ctx context.Context, target, actor string, ack bool, plan *services.EntitlementPlan) error {
	if limit := cohortLimit(); !ack && plan.Summary.Apply > limit {
		return &cohortTooLarge{Affected: plan.Summary.Apply, Limit: limit}
	}

	subjects := make([]db.NewPlanSubject, 0, len(plan.Outcomes))
	for _, out := range plan.Outcomes {
		effect, ok := planEffect(out.Effect)
		if !ok {
			return errUnapprovableEffect
		}
		set, resolved := plan.Desired[out.UserID]
		if !resolved {
			// A row with no resolved set behind it. Refused rather than recorded
			// with an empty one: an empty desired state converges the subject to
			// nothing, which is the most destructive possible reading of a
			// missing intent.
			return errNoResolvedIntent
		}
		subjects = append(subjects, db.NewPlanSubject{
			SubjectID:    out.UserID,
			DesiredState: set.Desired(),
			Fingerprint:  out.Fingerprint,
			Outcome:      db.PlanOutcome{Effect: effect},
		})
	}
	if len(subjects) == 0 {
		return nil
	}

	newPlan := db.NewPlan{
		Target:             target,
		Surface:            planSurfaceEntitlements,
		CreatedBy:          actor,
		StateReadAt:        plan.StateReadAt,
		RequestFingerprint: services.FingerprintIDCohort(services.EntitlementOp, subjectIDsOf(plan.Outcomes)),
		Subjects:           subjects,
	}
	if plan.Provisional {
		// A provisional plan carries no lifetime, and the store refuses one that
		// does. Its gate is the re-fingerprint when the target returns, not a
		// clock: expiring it would discard an approved change because an outage
		// outlasted a timer the operator had no part in.
		newPlan.Provisional = true
		if newPlan.StateReadAt.IsZero() {
			// The store refuses this too. Guarded here so the refusal names the
			// missing thing rather than arriving as a validation error about a
			// column.
			return errProvisionalWithoutAge
		}
	} else {
		newPlan.Lifetime = planLifetime()
		if newPlan.StateReadAt.IsZero() {
			newPlan.StateReadAt = time.Now().UTC()
		}
	}

	created, err := dbCreatePlan(ctx, newPlan)
	if err != nil {
		return err
	}
	plan.PlanID = created.ID
	return nil
}

var (
	errUnapprovableEffect    = errors.New("cannot record a result as an approved effect: a plan states what will happen")
	errNoResolvedIntent      = errors.New("a rehearsed subject carries no resolved desired state, so there is nothing to approve")
	errProvisionalWithoutAge = errors.New("a provisional plan must record the age of the state it was computed against")
)

func subjectIDsOf(outcomes []services.BulkOutcome) []string {
	ids := make([]string, 0, len(outcomes))
	for _, o := range outcomes {
		ids = append(ids, o.UserID)
	}
	return ids
}

// writeEntitlementError maps the refusals to codes a surface can act on. Each
// one is a different operator action, which is why none of them collapses into
// a generic 400.
func writeEntitlementError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, services.ErrTargetIsBuiltIn):
		jsonErrorResponse(w, http.StatusBadRequest, "TARGET_NOT_AN_ADDON", err.Error())
	case errors.Is(err, services.ErrNoSubjects):
		jsonValidationErrorResponse(w, "Nothing to converge", map[string]string{"subject_ids": "required"})
	case errors.Is(err, services.ErrTargetUnplannable):
		// 503, not 500: the backend is fine and the operator's request was fine.
		// Retrying later is the action, and it is the only one.
		jsonErrorResponse(w, http.StatusServiceUnavailable, "ADDON_UNREACHABLE", err.Error())
	case errors.Is(err, planapply.ErrPlanRequired):
		missingPlanCitation(w)
	case errors.Is(err, db.ErrTargetNotActive):
		// A target the deployment removed. Distinct from unreachable, and the
		// distinction is the whole point: nothing will ever drain rows queued for
		// a disabled target, so queueing them would read as "recorded" forever.
		jsonErrorResponse(w, http.StatusConflict, "TARGET_NOT_ACTIVE", err.Error())
	default:
		writePlanCitationError(w, err)
	}
}
