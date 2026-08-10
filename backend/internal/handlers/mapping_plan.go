package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"syndra/internal/db"
	"syndra/internal/services"
)

// Mapping edits, rehearsed (change `addon-platform` 7.11).
//
// A mapping edit is the highest-leverage change in this system and it used to be
// the cheapest: one PATCH silently changed what every holder of a role could
// reach. Deleting one is worse, because the effect is invisible — nothing on any
// screen says "these forty people just lost their storage access", it simply
// stops being true.
//
// So both go through the plan path. The diff is not computed by the add-on here,
// and it does not need to be: what changes is what the role MEANS, and Syndra
// owns that. The cohort is every holder of the role, and the effect is the same
// sentence for all of them — the value moving from one thing to another, or
// going away. What the add-on would add is per-subject noise about a change
// nobody is choosing per subject.
//
// The convergences that follow are queued in the same transaction as the edit,
// through the same system-minted approval a cascade uses, so a mapping edit that
// commits without its convergences is not a state this can reach.

const (
	planSurfaceMappingEdit   = "mappings.edit"
	planSurfaceMappingDelete = "mappings.delete"
)

type mappingPlanRequest struct {
	// Value is the new value, for an edit. Absent for a delete.
	Value            string `json:"value,omitempty"`
	AcknowledgeScope bool   `json:"acknowledge_scope"`
}

// handleRehearseMappingEdit says who a mapping change would move, and issues the
// approval for it.
func handleRehearseMappingEdit(w http.ResponseWriter, r *http.Request) {
	rehearseMappingChange(w, r, planSurfaceMappingEdit)
}

// handleRehearseMappingDelete is the same rehearsal for the removal.
//
// A separate surface rather than a flag, because a plan issued for one must not
// be citable on the other: they affect the same cohort and do opposite things to
// it, and a citation that could cross between them is an approval to change a
// value being spent to withdraw it.
func handleRehearseMappingDelete(w http.ResponseWriter, r *http.Request) {
	rehearseMappingChange(w, r, planSurfaceMappingDelete)
}

func rehearseMappingChange(w http.ResponseWriter, r *http.Request, surface string) {
	var req mappingPlanRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}

	mapping, err := dbGetRoleMapping(r.Context(), r.PathValue("id"))
	if err != nil {
		writeMappingError(w, err)
		return
	}
	if surface == planSurfaceMappingEdit {
		// Validated before the rehearsal, not after it. A plan for a value the
		// target cannot resolve is a diff an operator would approve and the apply
		// would then refuse — which teaches them the approval means nothing.
		if err := validateMappingAgainstTarget(r.Context(), mapping.Target, mapping.Field, req.Value); err != nil {
			writeMappingError(w, err)
			return
		}
		if req.Value == mapping.Value {
			jsonValidationErrorResponse(w, "Nothing to change",
				map[string]string{"value": "this is the value the mapping already has"})
			return
		}
	}

	plan, err := rehearseMapping(r.Context(), mapping, surface, req.Value)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	if err := issueMappingPlan(r.Context(), mapping, surface, resolveActor(r, ""), req, &plan); err != nil {
		writePlanIssueError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, plan)
}

// rehearseMapping states the change once per person it reaches.
//
// One sentence for the whole cohort, because it IS one change: nobody is
// choosing this per subject, and rendering forty per-subject diffs of the same
// edit would bury the only fact that matters, which is how many people it moves.
func rehearseMapping(ctx context.Context, m db.RoleMapping, surface, newValue string) (services.BulkPlan, error) {
	holders, err := dbMappingHolders(ctx, m.ProjectID, m.RoleKey)
	if err != nil {
		return services.BulkPlan{}, err
	}

	detail, consequence := mappingChangeWording(m, surface, newValue)
	plan := services.BulkPlan{Op: "edit_mapping"}
	if surface == planSurfaceMappingDelete {
		plan.Op = "delete_mapping"
	}
	for _, id := range holders {
		plan.Outcomes = append(plan.Outcomes, services.BulkOutcome{
			UserID: id, Effect: services.EffectApply,
			Detail: detail, Consequence: consequence,
			// The state being approved is the mapping as it stands, not the
			// holder's account on the target: that is what the operator read and
			// what the edit acts on. A holder whose target account moved in the
			// meantime is not a reason to refuse a change to what a role means.
			Fingerprint: services.Fingerprint("mapping", m.ID, m.Field, m.Value, id),
		})
	}
	plan.Summary = services.SummarizeOutcomes(plan.Outcomes)
	return plan, nil
}

func mappingChangeWording(m db.RoleMapping, surface, newValue string) (detail, consequence string) {
	if surface == planSurfaceMappingDelete {
		return fmt.Sprintf("%s stops conferring %s = %s on %s.", m.RoleKey, m.Field, m.Value, m.Target),
			"They keep the role and lose what it reached."
	}
	return fmt.Sprintf("%s confers %s = %s on %s instead of %s.", m.RoleKey, m.Field, newValue, m.Target, m.Value),
		fmt.Sprintf("They gain %s and lose %s.", newValue, m.Value)
}

// issueMappingPlan records the rehearsal, under the same blast-radius guard
// every other planned surface uses.
//
// The plan subjects carry no desired state: this approval is not an entitlement
// convergence, it is permission to change what a role means. The convergences
// are minted afterwards, per holder, when the edit commits — and they are
// system-minted for the same reason a cascade's are, since nobody approved a
// per-person diff here either.
func issueMappingPlan(ctx context.Context, m db.RoleMapping, surface, actor string, req mappingPlanRequest, plan *services.BulkPlan) error {
	if limit := cohortLimit(); !req.AcknowledgeScope && plan.Summary.Apply > limit {
		return &cohortTooLarge{Affected: plan.Summary.Apply, Limit: limit}
	}
	if len(plan.Outcomes) == 0 {
		// A mapping nobody holds. There is nothing to approve and nothing to
		// converge, so the edit needs no citation — and issuing one would hand
		// back an approval that applies to nobody.
		return nil
	}

	subjects := make([]db.NewPlanSubject, 0, len(plan.Outcomes))
	for _, out := range plan.Outcomes {
		subjects = append(subjects, db.NewPlanSubject{
			SubjectID: out.UserID, Fingerprint: out.Fingerprint,
			Outcome: db.PlanOutcome{Effect: db.PlanEffectApply},
		})
	}

	created, err := dbCreatePlan(ctx, db.NewPlan{
		// The mapping's own target, so a plan for a TrueNAS mapping cannot be
		// cited against another add-on's.
		Target:             m.Target,
		Surface:            surface,
		CreatedBy:          actor,
		Lifetime:           planLifetime(),
		StateReadAt:        time.Now().UTC(),
		RequestFingerprint: mappingRequestFingerprint(m, surface, req.Value),
		Subjects:           subjects,
	})
	if err != nil {
		return err
	}
	plan.PlanID = created.ID
	return nil
}

// mappingRequestFingerprint binds the approval to the mapping AND to the value
// being moved to.
//
// Without the value, an approval to change `lab_makers` → `lab_users` would be
// spendable on a request changing it to anything at all — which is the whole of
// what an operator was reviewing.
func mappingRequestFingerprint(m db.RoleMapping, surface, newValue string) string {
	return services.Fingerprint("mapping_change", surface, m.ID, m.Field, m.Value, newValue)
}

// applyMappingChange spends the approval, makes the edit, and queues one
// convergence per holder, in one transaction.
//
// One transaction and inside the access lock, because the holders are read
// twice: once to issue the plan and once here to converge them. A person who
// gained the role in between is converged by the read here — which is right, the
// mapping now means something different for them too — and a person who lost it
// is not, which is also right.
func applyMappingChange(ctx context.Context, m db.RoleMapping, surface, actor, planID, newValue string, holderCount int) (int, error) {
	converged := 0
	err := svcInTxLockingAccess(ctx, func(ctx context.Context) error {
		if holderCount > 0 {
			if strings.TrimSpace(planID) == "" {
				return errPlanCitationMissing
			}
			if err := claimMappingPlan(ctx, db.PlanCitation{
				PlanID: planID, Target: m.Target, Surface: surface, Actor: actor,
				RequestFingerprint: mappingRequestFingerprint(m, surface, newValue),
			}); err != nil {
				return err
			}
		}

		if surface == planSurfaceMappingDelete {
			if err := dbDeleteRoleMapping(ctx, m.ID); err != nil {
				return err
			}
		} else if err := dbUpdateRoleMappingValue(ctx, m.ID, newValue, actor); err != nil {
			return err
		}

		// Read AFTER the edit, so the resolved sets the convergences carry are
		// computed against the mapping as it now is. Reading before would queue
		// every holder's OLD state, which is the change not happening at all.
		holders, err := dbMappingHolders(ctx, m.ProjectID, m.RoleKey)
		if err != nil {
			return err
		}
		for _, id := range holders {
			set, err := svcResolveEntitlementsFor(ctx, id, m.Target)
			if err != nil {
				return err
			}
			if _, _, err := dbRecordSystemConvergence(ctx, db.SystemConvergence{
				Target: m.Target, SubjectID: id, Actor: actor,
				Reason:  "A role-to-target mapping changed",
				Desired: set,
			}); err != nil {
				return err
			}
			converged++
		}
		return nil
	})
	return converged, err
}

// claimMappingPlan spends the approval and discards its subject rows.
//
// The rows are not needed: what they recorded is who the operator was shown, and
// the convergences are computed from a fresh read taken under the same lock. What
// the claim is for is the predicate — one approval, one apply, this operator,
// this surface, this request.
func claimMappingPlan(ctx context.Context, citation db.PlanCitation) error {
	_, _, err := dbClaimPlanVerified(ctx, citation, nil)
	return err
}

// writeMappingPlanError separates a citation refusal from a mapping refusal, so
// the operator is told which of the two to act on.
func writeMappingPlanError(w http.ResponseWriter, err error) {
	var oversized *cohortTooLarge
	switch {
	case errors.Is(err, errPlanCitationMissing):
		missingPlanCitation(w)
	case errors.As(err, &oversized):
		writePlanIssueError(w, err)
	case errors.Is(err, db.ErrPlanNotFound), errors.Is(err, db.ErrPlanExpired),
		errors.Is(err, db.ErrPlanAlreadyApplied), errors.Is(err, db.ErrPlanNotCitableHere),
		errors.Is(err, db.ErrPlanNotYours), errors.Is(err, db.ErrPlanRequestMismatch):
		writePlanCitationError(w, err)
	default:
		writeMappingError(w, err)
	}
}

// svcResolveEntitlementsFor is the resolver as this file needs it: the encoded
// intent rather than the whole set, so nothing here has to know the shape.
var svcResolveEntitlementsFor = func(ctx context.Context, subjectID, target string) (map[string]json.RawMessage, error) {
	set, err := services.ResolveEntitlements(ctx, subjectID, target)
	if err != nil {
		return nil, err
	}
	return set.Desired(), nil
}

// rollbackAndConverge restores a mapping version and re-resolves everyone it
// reaches, in one transaction.
//
// The convergences are the half a rollback is usually missing. Restoring the
// bindings changes what the roles MEAN; it does nothing to the accounts already
// converged under the version being reverted, and no later event would notice —
// a drift sweep would see the target holding what Syndra last told it to hold.
//
// The cohort is read after the restore and across every mapping the restored
// version carries, because a rollback can reinstate a binding for a role nobody
// currently holds a mapping for, and those holders are exactly the ones whose
// access the rollback is meant to bring back.
func rollbackAndConverge(ctx context.Context, target string, version int, actor string) (int, error) {
	converged := 0
	err := svcInTxLockingAccess(ctx, func(ctx context.Context) error {
		if err := dbRollbackMappingVersion(ctx, target, version, actor); err != nil {
			return err
		}
		mappings, err := dbListRoleMappings(ctx, target)
		if err != nil {
			return err
		}

		// One convergence per subject, not per mapping: two restored mappings on
		// one role would otherwise queue the same resolved set twice.
		seen := map[string]struct{}{}
		for _, m := range mappings {
			holders, err := dbMappingHolders(ctx, m.ProjectID, m.RoleKey)
			if err != nil {
				return err
			}
			for _, id := range holders {
				if _, dup := seen[id]; dup {
					continue
				}
				seen[id] = struct{}{}
				set, err := svcResolveEntitlementsFor(ctx, id, target)
				if err != nil {
					return err
				}
				if _, _, err := dbRecordSystemConvergence(ctx, db.SystemConvergence{
					Target: target, SubjectID: id, Actor: actor,
					Reason:  fmt.Sprintf("Mapping set rolled back to v%d", version),
					Desired: set,
				}); err != nil {
					return err
				}
				converged++
			}
		}
		return nil
	})
	return converged, err
}
