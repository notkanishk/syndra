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

// The differences a reconciliation may not resolve, and the resolutions that
// can actually be expressed (change `reconciliation-as-merge`).
//
// A pass that knows WHO changed a value knows which changes are not its to
// undo. What it hands over is a decision, and the decisions offered here are
// bounded by something real rather than by what reads well in a dialog: desired
// state is resolved from held roles, role mappings and allowances, so a
// resolution has to be expressible in one of those or it is not a resolution.
//
//	keep ours      always available — apply Syndra's state over the target's
//	take theirs    only where a PER-SUBJECT decision can hold the value
//	policy change  everything else, named rather than offered as a button
//
// The third is not a refusal to help. `group` comes from `target_role_mappings`,
// which has no subject column and whose own DDL says editing it "silently
// changes what every holder of that role can reach"; the lifecycle fields are
// refused as mapping targets at three separate layers because they are derived.
// There is nothing that can hold either value for one person, and inventing a
// per-subject additive grant so the dialog has a button would put an entitlement
// where no access review looks.

// handleMergeFindings lists what is still waiting for a person on one target.
func handleMergeFindings(w http.ResponseWriter, r *http.Request) {
	findings, err := dbStandingMergeFindings(r.Context(), r.PathValue("target"))
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "FINDINGS_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{
		"target": r.PathValue("target"), "findings": findings,
	})
}

type resolveMergeFindingRequest struct {
	// Resolution is what the operator chose, in the vocabulary the surface
	// offers: keep_ours, take_theirs, reprovisioned, unbound.
	Resolution string `json:"resolution"`
	// Reason is required for anything that changes desired state. An adopted
	// value becomes policy for that person, and policy with no stated reason is
	// what the allowance layer exists to replace.
	Reason string `json:"reason"`
	// The bound in time an adopted suspension carries. Exactly one is enough
	// and neither is not — the schema refuses an unbounded denial, and this
	// surface must not be the way around it.
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	ReviewDate *time.Time `json:"review_date,omitempty"`
}

// ErrResolutionNotExpressible refuses an adoption that has nowhere to live.
var ErrResolutionNotExpressible = errors.New("that value cannot be adopted for one person")

// handleResolveMergeFinding performs the operator's decision, then closes the
// finding.
//
// In that order, and never the reverse. A finding marked resolved by an action
// that then failed is a difference nothing will raise again until it changes a
// second time — the failure mode this table was created to prevent, reached
// through its own resolution path.
func handleResolveMergeFinding(w http.ResponseWriter, r *http.Request) {
	var req resolveMergeFindingRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}
	actor := resolveActor(r, "")
	id := r.PathValue("id")

	var resolved db.MergeFinding
	err := svcInTxLockingAccess(r.Context(), func(ctx context.Context) error {
		// Read inside the transaction. The resolution's meaning depends on the
		// finding's outcome and field, and a caller naming an id is not a caller
		// who has seen the row.
		finding, err := dbGetStandingMergeFinding(ctx, id)
		if err != nil {
			return err
		}
		if err := performResolution(ctx, finding, req, actor); err != nil {
			return err
		}
		resolved, err = dbResolveMergeFinding(ctx, id, actor, req.Resolution)
		return err
	})

	switch {
	case errors.Is(err, db.ErrNoSuchMergeFinding):
		jsonErrorResponse(w, http.StatusNotFound, "NO_SUCH_FINDING",
			"That finding is not standing. Somebody may have resolved it already.")
		return
	case errors.Is(err, ErrResolutionNotExpressible):
		// 422 rather than 400: the request is well formed and the system cannot
		// carry it out. The message names the policy instead of apologising.
		jsonErrorResponse(w, http.StatusUnprocessableEntity, "RESOLUTION_NOT_EXPRESSIBLE", err.Error())
		return
	case errors.Is(err, db.ErrAllowanceUnbounded), errors.Is(err, db.ErrAllowanceInvalid):
		jsonValidationErrorResponse(w, err.Error(), map[string]string{"resolution": "unbounded"})
		return
	case err != nil:
		jsonErrorResponse(w, http.StatusInternalServerError, "RESOLVE_FAILED", err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"resolved": true, "finding": resolved, "resolution": req.Resolution,
	})
}

// performResolution carries out the decision. It writes; it does not close.
func performResolution(ctx context.Context, f db.MergeFinding, req resolveMergeFindingRequest, actor string) error {
	switch req.Resolution {
	case db.ResolutionKeepOurs, db.ResolutionReprovisioned:
		// Syndra's state, applied over the target's. Queued like every other
		// convergence rather than written here: the apply path is the only thing
		// that talks to a target, and its read-back is what records the new base
		// — which is what stops this finding returning on the next pass.
		set, err := svcResolveEntitlementsFor(ctx, f.SubjectID, f.Target)
		if err != nil {
			return fmt.Errorf("resolve %s on %s: %w", f.SubjectID, f.Target, err)
		}
		reason := "An operator kept Syndra's state over the target's"
		if req.Resolution == db.ResolutionReprovisioned {
			reason = "An operator asked for a deleted account to be provisioned again"
		}
		if _, _, err := dbRecordSystemConvergence(ctx, db.SystemConvergence{
			Target: f.Target, SubjectID: f.SubjectID, Actor: actor,
			Reason: reason, Desired: set,
		}); err != nil {
			return fmt.Errorf("queue convergence for %s: %w", f.SubjectID, err)
		}
		return nil

	case db.ResolutionTakeTheirs:
		return adoptTargetValue(ctx, f, req, actor)

	case db.ResolutionUnbound:
		// Stop managing the account, and forget what it was last seen holding.
		// The add-on's own binding is released by the release route; this is the
		// backend's half, reached here so the finding and the record end
		// together.
		forgetMergeBase(ctx, f.Target, f.SubjectID)
		if err := dbForgetTargetBinding(ctx, f.Target, f.SubjectID); err != nil {
			return fmt.Errorf("forget the binding for %s: %w", f.SubjectID, err)
		}
		return nil

	default:
		return fmt.Errorf("%q is not a resolution this surface offers", req.Resolution)
	}
}

// adoptTargetValue writes the target's value into the desired state, where
// something can hold it for one subject.
//
// Exactly one shape can be: a lifecycle field the target has turned OFF,
// recorded as a deny allowance. That is not a workaround — it is the mechanism
// that layer was built for, and adopting through it is a strict improvement on
// the hand edit it came from: the schema refuses a denial without an actor, a
// reason, and an expiry or a review date, so a suspension somebody made on the
// NAS acquires all three.
//
// Everything else is refused with the policy named. A `group` value belongs to
// a role mapping that reaches every holder of that role; a lifecycle field the
// target has turned ON is produced by holding a mapped role, so adopting it for
// one person means granting them one.
func adoptTargetValue(ctx context.Context, f db.MergeFinding, req resolveMergeFindingRequest, actor string) error {
	if strings.TrimSpace(req.Reason) == "" {
		return fmt.Errorf("%w: adopting a value makes it policy for that person, which needs a reason",
			db.ErrAllowanceInvalid)
	}
	if !services.IsLifecycleField(f.Field) {
		return fmt.Errorf("%w: %s comes from this target's role mappings, which have no per-person form — "+
			"changing one changes it for every holder of that role. Edit the mapping, or change what roles this person holds",
			ErrResolutionNotExpressible, f.Field)
	}
	var theirs bool
	if err := json.Unmarshal(f.Theirs, &theirs); err != nil {
		return fmt.Errorf("%w: the target's value for %s is not one this field can hold",
			ErrResolutionNotExpressible, f.Field)
	}
	if theirs {
		return fmt.Errorf("%w: %s is on because this person holds a mapped role for this target. "+
			"There is no per-person way to switch it on; grant them a role that maps here",
			ErrResolutionNotExpressible, f.Field)
	}

	// The denial names the value it removes — the `true` the resolver would
	// otherwise compute — which is the shape `resolveLifecycle` matches on.
	_, err := dbCreateAllowance(ctx, db.Allowance{
		SubjectID: f.SubjectID, Target: f.Target,
		Field: f.Field, Value: "true", Direction: db.AllowanceDeny,
		ActorID: actor, Reason: req.Reason,
		ExpiresAt: req.ExpiresAt, ReviewDate: req.ReviewDate,
	})
	return err
}

// mergeFindingCount is the governance summary's read, kept beside the surface
// it counts.
func mergeFindingCount(ctx context.Context) int {
	n, err := dbCountMergeFindings(ctx)
	if err != nil {
		return 0
	}
	return n
}
