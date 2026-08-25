package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"syndra/internal/db"
	"syndra/internal/services"
)

// Plan-then-apply, as a backend guarantee rather than a client convention
// (design §8, change `addon-platform` group 3).
//
// The weakness this closes is not tampering — no plan ever crossed the wire.
// It is the gap between the two REQUESTS. An operator sent a rehearsal, read
// the consequences, and sent a second request with `?apply=true` that computed
// the diff again from scratch. Nothing bound the two together: grants, drift
// rows, bundles and directory status can all move in between, so the operator
// approved one diff and a different one applied.
//
// So the rehearsal becomes durable. It persists one row per affected subject,
// carrying the effect that was approved and a fingerprint of the state it was
// approved against, and the apply cites the plan id. The stored outcomes drive
// the apply; the fingerprints decide whether it is still allowed to.

// Surfaces. A plan is issued by one screen and citable only there: without
// this, a drift-triage approval could be spent on the bulk-grant endpoint,
// where its subject ids mean something entirely different.
const (
	planSurfaceBulkGrants    = "grants.bulk"
	planSurfaceBulkDecision  = "requests.bulk_decision"
	planSurfaceDriftAdopt    = "drift.bulk_attribute"
	planSurfaceDriftExternal = "drift.bulk_mark_external"
)

// ErrPlanStale is the apply's refusal when the world moved under an approval.
// Distinct from every plan-identity refusal, because the operator's next action
// is different: look at what changed, then approve it again.
var ErrPlanStale = errors.New("the state you reviewed has changed")

// staleSubjects carries which subjects moved, so the surface can say so instead
// of reporting a generic failure.
type staleSubjects struct{ IDs []string }

func (s *staleSubjects) Error() string {
	return fmt.Sprintf("%v for %d subject(s)", ErrPlanStale, len(s.IDs))
}
func (s *staleSubjects) Unwrap() error { return ErrPlanStale }

// ErrCohortTooLarge refuses to issue an approval whose blast radius the
// operator has not acknowledged.
//
// The guard lives at plan time because the backend is the only component that
// holds a cohort — a per-subject apply cannot know how many subjects an
// operation touches, so specifying it there would put it in the one place
// unable to implement it (design, Risks). And it lives in `issuePlan` rather
// than in each surface, so it covers every planned operation including the ones
// added later.
type cohortTooLarge struct{ Affected, Limit int }

func (e *cohortTooLarge) Error() string {
	return fmt.Sprintf("this affects %d subjects, above the %d that may be approved without acknowledging the scope", e.Affected, e.Limit)
}

// cohortLimit is the affected-subject count above which an approval needs the
// operator to say the number out loud.
//
// It counts the subjects that would CHANGE, not the ones selected — a selection
// of two hundred people that turns out to grant three of them a role is a small
// change reviewed as a large one, and refusing it would train operators to
// acknowledge everything. `BulkMaxUsers` is the different, cruder ceiling on
// how many may be selected at all, and it stays.
func cohortLimit() int {
	if v := os.Getenv("PLAN_COHORT_LIMIT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 25
}

// planLifetime bounds how long a reviewed diff may be cited.
//
// Short on purpose. It is not a session timeout: it is how long the backend is
// willing to assert that a fingerprint taken then still describes a world worth
// acting on. Long enough to read a dialog, short enough that an approval left
// open in a tab overnight is re-planned rather than applied.
func planLifetime() time.Duration {
	if v := os.Getenv("PLAN_LIFETIME"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return 15 * time.Minute
}

// issuePlan persists a rehearsal and stamps its id on the plan the caller is
// about to return.
//
// Every rehearsed subject is recorded, not only the actionable ones. A plan is
// the record of what an operator approved, and one that silently omitted the
// rows shown as blocked would be an incomplete record of the review — and
// re-verifying a blocked row is meaningful in its own right, since an account
// that was departed at review time and is active now is exactly the case the
// block existed for.
func issuePlan(ctx context.Context, surface, actor, requestFP string, ack bool, plan *services.BulkPlan) error {
	// Refused before anything is written, and reported with the number it
	// computed: an operator told only that the change is "too large" has to
	// guess at what they are being warned about.
	if limit := cohortLimit(); !ack && plan.Summary.Apply > limit {
		return &cohortTooLarge{Affected: plan.Summary.Apply, Limit: limit}
	}

	subjects := make([]db.NewPlanSubject, 0, len(plan.Outcomes))
	for _, out := range plan.Outcomes {
		effect, ok := planEffect(out.Effect)
		if !ok {
			// A rehearsal only ever produces the three plan effects; `applied`,
			// `failed` and `queued` are results. Reaching here means a caller
			// handed us a report rather than a plan, and recording it would
			// store an approval of an outcome.
			return fmt.Errorf("cannot record %q as an approved effect: a plan states what will happen", out.Effect)
		}
		subjects = append(subjects, db.NewPlanSubject{
			SubjectID:   out.UserID,
			Fingerprint: out.Fingerprint,
			Outcome:     db.PlanOutcome{Effect: effect, GrantIDs: out.GrantIDs},
		})
	}
	if len(subjects) == 0 {
		// Nothing was rehearsed, so there is nothing to approve. Issuing an id
		// here would hand back a citation that applies to nobody.
		return nil
	}

	created, err := dbCreatePlan(ctx, db.NewPlan{
		Target:             db.TargetZitadel,
		Surface:            surface,
		CreatedBy:          actor,
		Lifetime:           planLifetime(),
		StateReadAt:        time.Now().UTC(),
		RequestFingerprint: requestFP,
		Subjects:           subjects,
	})
	if err != nil {
		return err
	}
	plan.PlanID = created.ID
	return nil
}

// planEffect maps a rehearsal effect onto the closed vocabulary the plan store
// accepts, and refuses anything else rather than translating it.
func planEffect(e string) (string, bool) {
	switch e {
	case services.EffectApply:
		return db.PlanEffectApply, true
	case services.EffectNoChange:
		return db.PlanEffectNoChange, true
	case services.EffectBlocked:
		return db.PlanEffectBlocked, true
	}
	return "", false
}

// claimPlan spends an approval and hands back what it approved, refusing
// without spending it if any subject's reviewed state has moved.
//
// `live` runs the same rehearsal the operator read, freshly, indexed by
// subject. Its fingerprints answer "did this move" exactly as they asked it —
// same reader, same hash — and its sentences are what the result is rendered
// from. A value the two sides compute differently verifies nothing.
//
// It is a thunk because it costs reads, and a citation that fails on identity —
// unknown, spent, another operator's, another screen's — has nothing to
// verify. The store calls it only once the citation has been accepted.
//
// Verification is all-or-nothing, and that is deliberate. A partial apply on a
// stale plan would land the subjects that did not move and leave the operator
// to work out which — from a batch they approved as a unit, on the screen where
// a bulk mistake is hardest to unpick.
func claimPlan(
	ctx context.Context,
	surface, actor, requestFP, planID string,
	live func() map[string]services.BulkOutcome,
) ([]db.PlanSubject, error) {
	citation := db.PlanCitation{
		PlanID:             strings.TrimSpace(planID),
		Target:             db.TargetZitadel,
		Surface:            surface,
		Actor:              actor,
		RequestFingerprint: requestFP,
	}

	_, subjects, err := dbClaimPlanVerified(ctx, citation, func(rows []db.PlanSubject) error {
		current := live()
		var moved []string
		for _, row := range rows {
			// A subject on the approval and absent from the fresh rehearsal is
			// counted as moved, not skipped. The cohort is bound by the request
			// fingerprint, so it cannot be a different selection — it is a
			// subject the rehearsal could no longer evaluate, and unverifiable
			// is not verified.
			if fresh, seen := current[row.SubjectID]; !seen || fresh.Fingerprint != row.Fingerprint {
				moved = append(moved, row.SubjectID)
			}
		}
		if len(moved) > 0 {
			sort.Strings(moved)
			return &staleSubjects{IDs: moved}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return subjects, nil
}

// indexOutcomes keys a fresh rehearsal by subject, for verification and for
// rendering.
func indexOutcomes(outcomes []services.BulkOutcome) map[string]services.BulkOutcome {
	out := make(map[string]services.BulkOutcome, len(outcomes))
	for _, o := range outcomes {
		out[o.UserID] = o
	}
	return out
}

// writePlanCitationError maps a refusal to the status and code the surface acts
// on. Each one is a different operator action, so each one is its own code —
// a single 400 would tell an operator to "check the request" when what they
// need to do is re-plan, or ask the person who approved it.
// writePlanIssueError answers a rehearsal that could not become an approval.
//
// The blast-radius refusal is 422 rather than 400: the request is well formed
// and the backend understood it perfectly — it is declining to approve it at
// this size without being asked twice.
func writePlanIssueError(w http.ResponseWriter, err error) {
	var oversized *cohortTooLarge
	if errors.As(err, &oversized) {
		jsonResponse(w, http.StatusUnprocessableEntity, ErrorResponse{
			Error:   "COHORT_ACKNOWLEDGEMENT_REQUIRED",
			Message: oversized.Error(),
			Details: map[string]string{
				"affected": strconv.Itoa(oversized.Affected),
				"limit":    strconv.Itoa(oversized.Limit),
			},
		})
		return
	}
	// A rehearsal that could not be recorded must not be returned as one: the
	// operator would review a diff they cannot then apply, and the surface would
	// offer them the button anyway.
	jsonErrorResponse(w, http.StatusInternalServerError, "PLAN_NOT_RECORDED", err.Error())
}

func writePlanCitationError(w http.ResponseWriter, err error) {
	var stale *staleSubjects
	switch {
	case errors.Is(err, errPlanCitationMissing):
		missingPlanCitation(w)
	case errors.As(err, &stale):
		// 409, not 400: the request was well formed and was correct when it was
		// written. Something else changed.
		//
		// The subjects are carried structurally, not only in the sentence. A
		// surface that had to parse them out of prose would show "something
		// changed" the day the wording is edited, which is the generic failure
		// this refusal exists to replace. Keyed by subject so the client can
		// mark the rows it is already displaying.
		moved := make(map[string]string, len(stale.IDs))
		for _, id := range stale.IDs {
			moved[id] = "moved"
		}
		jsonResponse(w, http.StatusConflict, ErrorResponse{
			Error:   "PLAN_STALE",
			Message: "The state you reviewed has changed for: " + strings.Join(stale.IDs, ", ") + ". Re-plan to see what moved.",
			Details: moved,
		})
	case errors.Is(err, db.ErrPlanNotFound):
		jsonErrorResponse(w, http.StatusNotFound, "PLAN_NOT_FOUND", "No such plan. Re-plan and try again.")
	case errors.Is(err, db.ErrPlanExpired):
		jsonErrorResponse(w, http.StatusConflict, "PLAN_EXPIRED", "This plan has expired. Re-plan to review it against current state.")
	case errors.Is(err, db.ErrPlanAlreadyApplied):
		jsonErrorResponse(w, http.StatusConflict, "PLAN_ALREADY_APPLIED", "This plan has already been applied.")
	case errors.Is(err, db.ErrPlanNotCitableHere):
		jsonErrorResponse(w, http.StatusConflict, "PLAN_NOT_CITABLE_HERE", "This plan was issued by a different screen.")
	case errors.Is(err, db.ErrPlanNotYours):
		jsonErrorResponse(w, http.StatusForbidden, "PLAN_NOT_YOURS", "This plan was approved by a different operator.")
	case errors.Is(err, db.ErrPlanRequestMismatch):
		jsonErrorResponse(w, http.StatusConflict, "PLAN_REQUEST_MISMATCH",
			"This plan was computed for a different request. Re-plan with what you want to apply.")
	default:
		jsonErrorResponse(w, http.StatusInternalServerError, "PLAN_CLAIM_ERROR", err.Error())
	}
}

// errPlanCitationMissing lets a helper report a missing citation to a caller
// that writes the response, so the refusal has one wording wherever it happens.
var errPlanCitationMissing = errors.New("applying requires the plan_id returned by the rehearsal")

// missingPlanCitation answers an apply that cites nothing.
//
// Refused rather than falling back to recomputation, which is the whole change:
// an apply with no approval behind it is the case this replaces, and leaving it
// working would leave both protocols live with the weaker one governing the
// access that actually exists.
func missingPlanCitation(w http.ResponseWriter) {
	jsonErrorResponse(w, http.StatusBadRequest, "PLAN_REQUIRED",
		"Applying requires the plan_id returned by the rehearsal. Rehearse first, then apply the plan you reviewed.")
}

// planApprovedRows turns the claimed subject rows back into the plan shape the
// surfaces report in, in a stable order.
//
// The operator-facing sentences are not read back from the store — they were
// never written there. A plan records the decision, not its rendering (design
// §5), so names are resolved from the directory at read time and the detail is
// what actually became of the row.
func planApprovedRows(op string, subjects []db.PlanSubject, live map[string]services.BulkOutcome) services.BulkPlan {
	plan := services.BulkPlan{Op: op, Applied: true}
	for _, s := range subjects {
		// Effect and grants come from the APPROVAL — they are what an operator
		// agreed to, and re-deriving them here would be the recomputation this
		// whole change removes. Everything a human reads is rendered from the
		// fresh read beside it, because the plan records the decision and not
		// its prose. The two cannot disagree: verification has already
		// established that the state behind them is the state that was reviewed.
		fresh := live[s.SubjectID]
		plan.Outcomes = append(plan.Outcomes, services.BulkOutcome{
			UserID:      s.SubjectID,
			Name:        fresh.Name,
			Email:       fresh.Email,
			Effect:      s.Outcome.Effect,
			Detail:      fresh.Detail,
			Consequence: fresh.Consequence,
			GrantIDs:    s.Outcome.GrantIDs,
		})
	}
	plan.Summary = services.SummarizeOutcomes(plan.Outcomes)
	return plan
}
