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

		outboxIDs, err := applyBulkOne(r, actor, input, *out)
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
		// Bundle ops are absent here on purpose: their cascade already drains
		// according to the bundle's own confirmation mode, and overriding that
		// from the bulk path would apply a bundle the owner marked manual.
		for _, id := range outboxIDs {
			_, _ = svcDrainPropagationRow(r.Context(), id)
		}
		out.Effect = services.EffectApplied
	}

	plan.Summary = services.SummarizeOutcomes(plan.Outcomes)
}

// applyBulkOne performs one person's share of the operation and returns the
// outbox rows it created, for the caller to drain. Bundle ops return none: the
// cascade decides for itself whether to drain, from the bundle's confirmation
// mode.
func applyBulkOne(r *http.Request, actor string, input services.BulkRequest, out services.BulkOutcome) ([]string, error) {
	ctx := r.Context()

	switch input.Op {
	case services.BulkOpAssignRole:
		id, err := enqueueBulkGrant(ctx, actor, out.UserID, input.ProjectID, input.RoleKey, input.Reason, input.DurationDays)
		if err != nil {
			return nil, err
		}
		return []string{id}, nil

	case services.BulkOpRemoveRole:
		var ids []string
		for _, grantID := range out.GrantIDs {
			res, err := svcDeleteDirectGrant(ctx, out.UserID, grantID, actor)
			if err != nil {
				return nil, err
			}
			// There may be none — every role the grant carried is still covered
			// by another source, so nothing is queued and nothing is drained.
			ids = append(ids, res.OutboxIDs...)
		}
		return ids, nil

	case services.BulkOpAssignBundle:
		_, err := svcCascadeBundleAssigned(ctx, actor, out.UserID, input.BundleID)
		return nil, err

	case services.BulkOpRemoveBundle:
		_, err := svcCascadeBundleRemoved(ctx, actor, out.UserID, input.BundleID)
		return nil, err

	case services.BulkOpExtend:
		// The grant ledger upserts on (user, project, role), so re-enqueuing
		// each expiring grant with a later date renews it in place rather than
		// creating a duplicate. The rehearsal already identified exactly which
		// grants those are.
		grants, err := svcUserDirectGrants(ctx, out.UserID)
		if err != nil {
			return nil, err
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
				return nil, err
			}
			ids = append(ids, id)
		}
		return ids, nil
	}

	return nil, nil
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
