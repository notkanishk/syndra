package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"syndra/internal/db"
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
	requestFP := services.FingerprintBulkRequest(input)

	if r.URL.Query().Get("apply") != "true" {
		plan, err := svcRehearseBulk(r.Context(), input)
		if err != nil {
			jsonErrorResponse(w, http.StatusInternalServerError, "BULK_REHEARSAL_ERROR", err.Error())
			return
		}
		if err := issuePlan(r.Context(), planSurfaceBulkGrants, actor, requestFP, &plan); err != nil {
			// A rehearsal that could not be recorded must not be returned as
			// one: the operator would review a diff they cannot then apply, and
			// the surface would offer them the button.
			jsonErrorResponse(w, http.StatusInternalServerError, "PLAN_NOT_RECORDED", err.Error())
			return
		}
		jsonResponse(w, http.StatusOK, plan)
		return
	}

	if strings.TrimSpace(req.PlanID) == "" {
		missingPlanCitation(w)
		return
	}

	// Re-rehearsed for its FINGERPRINTS, not for its verdicts. The stored
	// outcomes are what the apply acts on; this read is how the backend asks
	// whether the state behind them still holds. Where the two agree the fresh
	// verdicts equal the stored ones anyway — same inputs, same function — and
	// where they disagree the apply is refused rather than quietly re-decided.
	//
	// Deferred until the citation is accepted: a bogus plan id must not cost a
	// full cohort read.
	var current map[string]services.BulkOutcome
	var rehearsalErr error
	subjects, err := claimPlan(r.Context(), planSurfaceBulkGrants, actor, requestFP, req.PlanID,
		func() map[string]services.BulkOutcome {
			live, err := svcRehearseBulk(r.Context(), input)
			if err != nil {
				// Nothing verifies. An empty index reports every subject as
				// moved, which is the fail-closed direction, and the error is
				// reported over the staleness below.
				rehearsalErr = err
				return nil
			}
			current = indexOutcomes(live.Outcomes)
			return current
		})
	if rehearsalErr != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "BULK_REHEARSAL_ERROR", rehearsalErr.Error())
		return
	}
	if err != nil {
		writePlanCitationError(w, err)
		return
	}

	plan := planApprovedRows(input.Op, subjects, current)
	applyBulkPlan(r, &plan, input)
	jsonResponse(w, http.StatusAccepted, plan)
}

// applyBulkPlan executes the actionable rows in place, rewriting each outcome
// to what actually happened. Rows the rehearsal blocked or found already-done
// are left exactly as they are — a report an operator can diff against the plan
// they approved.
//
// Per-person failure is isolated: one person's error marks that row failed and
// the rest continue. A bulk operation that aborts halfway leaves the operator
// with no idea which half landed, which is strictly worse than a partial result
// that says so.
func applyBulkPlan(r *http.Request, plan *services.BulkPlan, input services.BulkRequest) {
	actor := resolveActor(r, "")
	plan.Applied = true

	for i := range plan.Outcomes {
		out := &plan.Outcomes[i]
		if out.Effect != services.EffectApply {
			continue
		}
		outboxIDs, queued, err := applyBulkOne(r, actor, input, *out)
		if err != nil {
			out.Effect = services.EffectFailed
			out.Detail = "Didn't go through: " + err.Error()
			continue
		}

		// Recompile from the just-committed ledger so access is effective
		// immediately, on a context detached from the request — a client that
		// disconnects mid-batch must not leave people with stale claims.
		rebuildUserCacheDetachedFn(r.Context(), out.UserID)

		// Then project the rows upstream, exactly as the single-person handlers
		// do on ?apply=true. Without this, apply meant "wrote it down": the
		// operator read a plan, confirmed it, saw every row marked applied, and
		// the roles were still whatever they had been in Zitadel until somebody
		// happened to visit the governance queue. Bulk removal was the sharp
		// end — access reported as gone that was still live.
		//
		// Bundle ops have already answered for themselves in `queued`: their
		// cascade drains according to the bundle's own confirmation mode, and
		// overriding that from the bulk path would apply a bundle the owner
		// marked manual.
		//
		// The drain's answer is read, not discarded. Draining and then marking
		// the row applied regardless is the same lie one step later: with Zitadel
		// offline every row still reported success while a bulk revoke sat
		// unexecuted in the outbox.
		if queued == "" {
			queued = projectUpstream(r.Context(), outboxIDs)
		}
		if queued != "" {
			out.Effect = services.EffectQueued
			out.Detail = "Recorded here, not yet in Zitadel — " + queued
			continue
		}
		out.Effect = services.EffectApplied
	}

	plan.Summary = services.SummarizeOutcomes(plan.Outcomes)
}

// projectUpstream drains the rows one person's operation enqueued and returns
// an empty string when every one of them reached Zitadel, or a short reason when
// any did not.
//
// A drain reports failure two ways and only one of them is an error: an
// unreachable Zitadel or a drain already in progress comes back as Halted with a
// nil error, so checking `err` alone reads a halted batch as success. Anything
// that is not a confirmed apply is treated as not-yet-applied, which is the
// conservative direction — under-claiming sends an operator to a queue that
// turns out to be empty, over-claiming tells them a door is locked when it is
// open.
func projectUpstream(ctx context.Context, outboxIDs []string) string {
	for _, id := range outboxIDs {
		res, err := svcDrainPropagationRow(ctx, id)
		switch {
		case err != nil:
			return err.Error()
		case res.Halted && res.Reason != "":
			return res.Reason
		case res.Applied == 0:
			// Requeued, failed, or claimed by a concurrent drain. Either way this
			// request did not watch it land.
			return "still queued for propagation"
		}
	}
	return ""
}

// applyBulkOne performs one person's share of the operation. It returns the
// outbox rows the caller must drain, plus a reason when the work is already
// known to be queued rather than applied.
//
// Bundle ops go down the second path: the cascade decides for itself whether to
// drain, from the bundle's own confirmation mode, so it reports its verdict
// instead of handing back rows. A bundle its owner set to manual is *meant* to
// leave work queued — that is not a failure, but it is not "applied" either, and
// the operator has to be told which one they got.
func applyBulkOne(r *http.Request, actor string, input services.BulkRequest, out services.BulkOutcome) ([]string, string, error) {
	ctx := r.Context()

	switch input.Op {
	case services.BulkOpAssignRole:
		id, err := enqueueBulkGrant(ctx, actor, out.UserID, input.ProjectID, input.RoleKey, input.Reason, input.DurationDays)
		if err != nil {
			return nil, "", err
		}
		return []string{id}, "", nil

	case services.BulkOpRemoveRole:
		var ids []string
		for _, grantID := range out.GrantIDs {
			res, err := svcDeleteDirectGrant(ctx, out.UserID, grantID, actor)
			if err != nil {
				return nil, "", err
			}
			// There may be none — every role the grant carried is still covered
			// by another source, so nothing is queued and nothing is drained.
			ids = append(ids, res.OutboxIDs...)
		}
		return ids, "", nil

	case services.BulkOpAssignBundle:
		res, err := svcCascadeBundleAssigned(ctx, actor, out.UserID, input.BundleID)
		return nil, cascadeQueuedReason(res), err

	case services.BulkOpRemoveBundle:
		res, err := svcCascadeBundleRemoved(ctx, actor, out.UserID, input.BundleID)
		return nil, cascadeQueuedReason(res), err

	case services.BulkOpExtend:
		// The grant ledger upserts on (user, project, role), so re-enqueuing
		// each expiring grant with a later date renews it in place rather than
		// creating a duplicate. The rehearsal already identified exactly which
		// grants those are.
		grants, err := svcUserDirectGrants(ctx, out.UserID)
		if err != nil {
			return nil, "", err
		}
		wanted := map[string]struct{}{}
		for _, id := range out.GrantIDs {
			wanted[id] = struct{}{}
		}
		var ids []string
		for _, g := range grants {
			if _, ok := wanted[g.ID]; !ok {
				continue
			}
			id, err := enqueueBulkGrant(ctx, actor, out.UserID, g.ProjectID, g.RoleKey, input.Reason, input.DurationDays)
			if err != nil {
				return nil, "", err
			}
			ids = append(ids, id)
		}
		return ids, "", nil
	}

	return nil, "", nil
}

// cascadeQueuedReason reports why a cascade's rows have not reached Zitadel, or
// "" when they have (or when there were none to send).
func cascadeQueuedReason(res services.CascadeResult) string {
	switch {
	case res.Enqueued == 0:
		return "" // nothing to propagate; the closure was already correct
	case res.Mode != "auto":
		return "this bundle applies on confirmation"
	case res.Drain.Halted && res.Drain.Reason != "":
		return res.Drain.Reason
	case res.Drain.Applied < res.Enqueued:
		return "still queued for propagation"
	}
	return ""
}

// enqueueBulkGrant writes the ledger + audit + outbox row in one transaction,
// exactly as the single-grant handler does, and returns the outbox id. The
// Zitadel mutation itself happens on the drain the caller runs afterwards — a
// bulk write is still not allowed to call the Management API from a handler.
func enqueueBulkGrant(ctx context.Context, actor, userID, projectID, roleKey, reason string, durationDays int) (string, error) {
	var expiresAt *time.Time
	if durationDays > 0 {
		expiry := time.Now().UTC().Add(time.Duration(durationDays) * 24 * time.Hour)
		expiresAt = &expiry
	}

	payload, _ := json.Marshal(map[string]any{
		"project_id":    projectID,
		"role_key":      roleKey,
		"reason":        reason,
		"duration_days": durationDays,
		"bulk":          true,
	})

	res, err := dbEnqueueDirectGrantPropagation(ctx, db.EnqueueParams{
		UserID:      userID,
		ProjectID:   projectID,
		RoleKeys:    []string{roleKey},
		GrantedBy:   actor,
		Reason:      reason,
		ExpiresAt:   expiresAt,
		Source:      "direct",
		OpType:      "add",
		PayloadJSON: string(payload),
	})
	if err != nil {
		return "", err
	}
	return res.OutboxID, nil
}
