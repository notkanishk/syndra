package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"mkauth/internal/db"
	"mkauth/internal/services"
)

// BulkGrantRequest is one access operation aimed at a set of people.
//
// There is no "apply" field: applying is a query parameter (?apply=true), so a
// rehearsal and the write it authorises are the same request body. A client
// that reviews a plan and then submits something different is a bug this shape
// makes hard to write.
type BulkGrantRequest struct {
	Op           string   `json:"op"`
	UserIDs      []string `json:"user_ids"`
	ProjectID    string   `json:"project_id,omitempty"`
	RoleKey      string   `json:"role_key,omitempty"`
	BundleID     string   `json:"bundle_id,omitempty"`
	Reason       string   `json:"reason"`
	DurationDays int      `json:"duration_days,omitempty"`
}

// handleBulkGrants rehearses a bulk access change and, on ?apply=true, executes
// the rows the rehearsal marked actionable.
//
//	POST /api/v1/grants/bulk[?apply=true]
//
// Rehearsal is the default because a bulk write is the one operation whose
// blast radius an operator cannot hold in their head from a count. Apply does
// not trust a plan the client sends back — it re-rehearses server-side and acts
// on that, so a person whose access changed in between is re-evaluated rather
// than acted on from a stale verdict.
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
	}
	if problems := services.ValidateBulkRequest(input); problems != nil {
		jsonValidationErrorResponse(w, "Invalid bulk request", problems)
		return
	}

	plan, err := svcRehearseBulk(r.Context(), input)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "BULK_REHEARSAL_ERROR", err.Error())
		return
	}

	if r.URL.Query().Get("apply") != "true" {
		jsonResponse(w, http.StatusOK, plan)
		return
	}

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

		if err := applyBulkOne(r, actor, input, *out); err != nil {
			out.Effect = services.EffectFailed
			out.Detail = "Didn't go through: " + err.Error()
			continue
		}

		// Recompile from the just-committed ledger so access is effective
		// immediately, on a context detached from the request — a client that
		// disconnects mid-batch must not leave people with stale claims.
		rebuildUserCacheDetachedFn(r.Context(), out.UserID)
		out.Effect = services.EffectApplied
	}

	plan.Summary = services.SummarizeOutcomes(plan.Outcomes)
}

func applyBulkOne(r *http.Request, actor string, input services.BulkRequest, out services.BulkOutcome) error {
	ctx := r.Context()

	switch input.Op {
	case services.BulkOpAssignRole:
		return enqueueBulkGrant(ctx, actor, out.UserID, input.ProjectID, input.RoleKey, input.Reason, input.DurationDays)

	case services.BulkOpRemoveRole:
		for _, grantID := range out.GrantIDs {
			if _, err := svcDeleteDirectGrant(ctx, out.UserID, grantID, actor); err != nil {
				return err
			}
		}
		return nil

	case services.BulkOpAssignBundle:
		_, err := svcCascadeBundleAssigned(ctx, actor, out.UserID, input.BundleID)
		return err

	case services.BulkOpRemoveBundle:
		_, err := svcCascadeBundleRemoved(ctx, actor, out.UserID, input.BundleID)
		return err

	case services.BulkOpExtend:
		// The grant ledger upserts on (user, project, role), so re-enqueuing
		// each expiring grant with a later date renews it in place rather than
		// creating a duplicate. The rehearsal already identified exactly which
		// grants those are.
		grants, err := svcUserDirectGrants(ctx, out.UserID)
		if err != nil {
			return err
		}
		wanted := map[string]struct{}{}
		for _, id := range out.GrantIDs {
			wanted[id] = struct{}{}
		}
		for _, g := range grants {
			if _, ok := wanted[g.ID]; !ok {
				continue
			}
			if err := enqueueBulkGrant(ctx, actor, out.UserID, g.ProjectID, g.RoleKey, input.Reason, input.DurationDays); err != nil {
				return err
			}
		}
		return nil
	}

	return nil
}

// enqueueBulkGrant writes the ledger + audit + outbox row in one transaction,
// exactly as the single-grant handler does. The Zitadel mutation itself happens
// on the operator-triggered drain — a bulk write is still not allowed to call
// the Management API from a handler.
func enqueueBulkGrant(ctx context.Context, actor, userID, projectID, roleKey, reason string, durationDays int) error {
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

	_, err := dbEnqueueDirectGrantPropagation(ctx, db.EnqueueParams{
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
	return err
}
