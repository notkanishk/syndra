package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"syndra/internal/addons"
	"syndra/internal/db"
	"syndra/internal/services"
	"syndra/internal/services/addonop"
	"syndra/internal/services/merge"
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

// governingPolicy is the mapping a field's value comes from, and how far
// editing it reaches.
//
// Carried on the finding because the surface's honest alternative to "adopt this
// value" is "change the policy", and an operator cannot act on that sentence
// without knowing WHICH policy and HOW MANY people it reaches. The mapping's own
// DDL says editing one "silently changes what every holder of that role can
// reach" — a surface that says so with a number is the difference between an
// informed decision and a surprise.
type governingPolicy struct {
	MappingID string `json:"mapping_id"`
	ProjectID string `json:"project_id"`
	RoleKey   string `json:"role_key"`
	Value     string `json:"value"`
	// Holders is how many people hold that role today. Zero is a real answer
	// and not an error: a mapping nobody holds is safe to edit, and that is
	// worth knowing too.
	Holders int `json:"holders"`
}

// findingView is a standing finding plus what can be done about it.
type findingView struct {
	db.MergeFinding
	// Adoptable says the target's value can be held for this one subject. True
	// only for a lifecycle field the target has turned OFF, which becomes a deny
	// allowance; everything else has no per-subject home, and offering a button
	// for it produces a decision that fails after somebody believes they made it.
	Adoptable bool `json:"adoptable"`
	// WhyNot is the sentence the surface shows instead of that button.
	WhyNot string `json:"why_not,omitempty"`
	// Policy is what to edit when adoption is impossible, with its blast radius.
	Policy []governingPolicy `json:"policy,omitempty"`
}

// handleMergeFindings lists what is still waiting for a person on one target.
func handleMergeFindings(w http.ResponseWriter, r *http.Request) {
	target := r.PathValue("target")
	findings, err := dbStandingMergeFindings(r.Context(), target)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "FINDINGS_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{
		"target": target, "findings": decorate(r.Context(), target, findings),
	})
}

// decorate answers, per finding, the question the surface has to render: can
// this value be adopted for one person, and if not, what would have to change?
//
// Computed here rather than in the frontend. Which fields have a per-subject
// home is a fact about the entitlement model — mappings are per role, lifecycle
// fields are derived — and a copy of that rule in a component is a second
// definition that disagrees the first time the model grows a field.
func decorate(ctx context.Context, target string, findings []db.MergeFinding) []findingView {
	out := make([]findingView, 0, len(findings))
	mappings := mappingsByField(ctx, target)
	held := heldRolesFor(ctx, findings)
	for _, f := range findings {
		view := findingView{MergeFinding: f}
		switch {
		case f.Outcome == string(merge.DeletedUpstream):
			// Neither adoptable nor a policy question: the account is gone, and
			// the two answers are re-provision or stop managing it.
		case !services.IsLifecycleField(f.Field):
			view.WhyNot = f.Field + " comes from this target's role mappings, which have no per-person form. " +
				"Editing one changes it for every holder of that role."
			view.Policy = governing(mappings[f.Field], held[f.SubjectID])
		case isPermissive(f.Theirs):
			view.WhyNot = f.Field + " is on because this person holds a mapped role for this target. " +
				"There is no per-person way to switch it on; grant them a role that maps here."
		default:
			// A lifecycle field the target has turned OFF. Adopting it writes a
			// deny allowance — the mechanism that layer was built for, and one
			// that carries an author, a reason and a bound in time.
			view.Adoptable = true
		}
		out = append(out, view)
	}
	return out
}

// mappingsByField groups a target's mappings with their holder counts.
//
// Holder counts are per (project, role) and are read once per mapping rather
// than once per finding: a target with forty findings against the same field
// must not make forty identical queries.
//
// A failure answers with what it has. The finding is still worth showing
// without its blast radius; refusing to render the queue because one count could
// not be read would hide the decisions themselves.
func mappingsByField(ctx context.Context, target string) map[string][]governingPolicy {
	out := map[string][]governingPolicy{}
	mappings, err := dbListRoleMappings(ctx, target)
	if err != nil {
		log.Printf("[FINDINGS] could not read %s's mappings for the policy hint: %v", target, err)
		return out
	}
	for _, m := range mappings {
		holders, err := dbMappingHolders(ctx, m.ProjectID, m.RoleKey)
		if err != nil {
			log.Printf("[FINDINGS] could not count holders of %s/%s: %v", m.ProjectID, m.RoleKey, err)
		}
		out[m.Field] = append(out[m.Field], governingPolicy{
			MappingID: m.ID, ProjectID: m.ProjectID, RoleKey: m.RoleKey,
			Value: m.Value, Holders: len(holders),
		})
	}
	return out
}

// heldRolesFor is the roles each subject in the queue effectively holds, read
// once per subject rather than once per finding.
//
// A subject whose roles cannot be read is left absent, and `governing` then
// shows no policy at all. That is deliberate: the alternative is listing every
// mapping on the field, which names policies that do not reach this person —
// and a blast radius somebody read off an unrelated mapping is worse than none,
// because it is a number they will act on.
func heldRolesFor(ctx context.Context, findings []db.MergeFinding) map[string]map[string]bool {
	out := map[string]map[string]bool{}
	for _, f := range findings {
		if _, done := out[f.SubjectID]; done {
			continue
		}
		refs, err := svcHeldRoles(ctx, f.SubjectID)
		if err != nil {
			log.Printf("[FINDINGS] could not read the roles %s holds: %v", f.SubjectID, err)
			continue
		}
		roles := make(map[string]bool, len(refs))
		for _, ref := range refs {
			roles[ref.ProjectID+"/"+ref.RoleKey] = true
		}
		out[f.SubjectID] = roles
	}
	return out
}

// governing narrows a field's mappings to the ones that produce THIS subject's
// value.
//
// Mappings are per role, and a subject's desired state comes only from the ones
// whose role they hold. Listing the rest would present unrelated policies as the
// thing to edit, with holder counts belonging to people this finding is not
// about — a blast radius that is not this decision's.
//
// No held roles means no governing policy, and an empty list rather than the
// full one. "We could not tell which" and "all of them" are different answers,
// and only one of them is true.
func governing(all []governingPolicy, held map[string]bool) []governingPolicy {
	if len(held) == 0 {
		return nil
	}
	out := make([]governingPolicy, 0, len(all))
	for _, p := range all {
		if held[p.ProjectID+"/"+p.RoleKey] {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// isPermissive reports whether the target's value is the ON one.
//
// Only that direction has no per-subject home: turning a lifecycle field off is
// a denial, and turning it on is holding a role.
func isPermissive(raw json.RawMessage) bool {
	var on bool
	if err := json.Unmarshal(raw, &on); err != nil {
		return false
	}
	return on
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

	// Read before anything, and read again inside the transaction below. The
	// resolution's meaning depends on the finding's OUTCOME — a caller naming an
	// id is not a caller who has seen the row, and the surface deciding which
	// buttons to render is not validation.
	finding, err := dbGetStandingMergeFinding(r.Context(), id)
	if err != nil {
		writeResolutionError(w, err)
		return
	}
	if err := resolutionFits(finding, req.Resolution); err != nil {
		writeResolutionError(w, err)
		return
	}
	if finding.Decision != "" {
		// Answered already. Refused here for the reading, and refused again
		// atomically below — this check is the message, not the guarantee.
		writeResolutionError(w, fmt.Errorf("%w: %s already decided as %s by %s",
			db.ErrMergeFindingDecided, id, finding.Decision, finding.DecidedBy))
		return
	}

	// `unbound` is the one resolution that calls the target, and the only one
	// whose work cannot be undone by a later decision. So the decision is
	// RESERVED first — one conditional write, under the same uniqueness the
	// standing row already has — and only then is the add-on told to let go.
	//
	// The order is the whole fix. Releasing first and deciding afterwards let a
	// second request choose the opposite while the first was mid-flight: the
	// add-on let go of an account a queued re-provision was about to recreate,
	// and the transaction discovered the changed row only after the network call
	// had already happened.
	//
	// The call stays OUTSIDE the transaction. An add-on that takes thirty
	// seconds to answer would otherwise hold the access lock for thirty seconds,
	// and a lock held across a call to a machine that may be down is an outage
	// on this side of the wire.
	if req.Resolution == db.ResolutionUnbound {
		reserved, err := dbRecordMergeDecision(r.Context(), id, actor, req.Resolution)
		if err != nil {
			writeResolutionError(w, err)
			return
		}
		if err := releaseOnTarget(r.Context(), finding, actor); err != nil {
			// The reservation goes with the work that did not happen. Left
			// standing, one unreachable add-on would wedge the finding as
			// decided-but-never-done with no way back through this surface.
			if rerr := dbReleaseMergeDecision(r.Context(), id, actor, req.Resolution); rerr != nil {
				log.Printf("[FINDINGS] %s: the release failed and its reservation could not be cleared: %v",
					id, rerr)
			}
			writeResolutionError(w, err)
			return
		}
		_ = reserved
	}

	var resolved db.MergeFinding
	err = svcInTxLockingAccess(r.Context(), func(ctx context.Context) error {
		// Re-read, because the world moved between the read above and this
		// transaction: another operator may have resolved it, and resolving one
		// twice would let the second believe they made a decision somebody else
		// had already made differently.
		current, err := dbGetStandingMergeFinding(ctx, id)
		if err != nil {
			return err
		}
		if current.Decision != "" && req.Resolution != db.ResolutionUnbound {
			return fmt.Errorf("%w: %s already decided as %s by %s",
				db.ErrMergeFindingDecided, id, current.Decision, current.DecidedBy)
		}
		if err := performResolution(ctx, current, req, actor); err != nil {
			return err
		}
		// A decision is not a settlement, except for the one that leaves nothing
		// to observe.
		//
		// Keeping Syndra's state QUEUES a convergence; adopting the target's
		// changes policy the next resolution will compute from. In both cases
		// the difference is still there when this returns, and closing the row
		// now would claim otherwise — so the next sweep would raise a second
		// finding about the same field, and one decision would produce a queue
		// that refills itself until the drain caught up. The row closes when a
		// pass sees the two sides agree, carrying this decision.
		//
		// `unbound` is the exception: the binding is gone, so no pass will ever
		// classify that subject again, and a row waiting for an observation that
		// cannot happen would stand forever.
		if req.Resolution == db.ResolutionUnbound {
			resolved, err = dbResolveMergeFinding(ctx, id, actor, req.Resolution)
			return err
		}
		resolved, err = dbRecordMergeDecision(ctx, id, actor, req.Resolution)
		return err
	})

	if err != nil {
		writeResolutionError(w, err)
		return
	}

	// `resolved` says the finding is closed; `decided` says it is answered and
	// waiting. Reporting the second as the first is what made the queue refill
	// itself, and it would read to an operator as the surface being broken.
	settled := req.Resolution == db.ResolutionUnbound
	detail := "Decided. The finding stays open until a reconciliation sees the target agree."
	if settled {
		detail = "Syndra no longer manages that account. Nothing on the target was changed."
	}
	jsonResponse(w, http.StatusOK, map[string]any{
		"resolved": settled, "decided": true,
		"finding": resolved, "resolution": req.Resolution, "detail": detail,
	})
}

func writeResolutionError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, db.ErrNoSuchMergeFinding):
		jsonErrorResponse(w, http.StatusNotFound, "NO_SUCH_FINDING",
			"That finding is not standing. Somebody may have resolved it already.")
	case errors.Is(err, ErrResolutionNotExpressible):
		// 422 rather than 400: the request is well formed and the system cannot
		// carry it out. The message names the policy instead of apologising.
		jsonErrorResponse(w, http.StatusUnprocessableEntity, "RESOLUTION_NOT_EXPRESSIBLE", err.Error())
	case errors.Is(err, db.ErrAllowanceUnbounded), errors.Is(err, db.ErrAllowanceInvalid):
		jsonValidationErrorResponse(w, err.Error(), map[string]string{"resolution": "unbounded"})
	case errors.Is(err, errReleaseNotConfirmed):
		jsonErrorResponse(w, http.StatusBadGateway, "RELEASE_NOT_CONFIRMED", err.Error())
	case errors.Is(err, db.ErrMergeFindingDecided):
		// 409, and it names who. The two answers here are opposites, so a second
		// operator has to know that somebody chose — not merely that their own
		// request failed.
		jsonErrorResponse(w, http.StatusConflict, "ALREADY_DECIDED", err.Error())
	default:
		jsonErrorResponse(w, http.StatusInternalServerError, "RESOLVE_FAILED", err.Error())
	}
}

// resolutionFits refuses a decision that does not belong to this finding.
//
// Which buttons a surface renders is not validation. `unbound` and
// `reprovisioned` are answers to an account that is GONE — applied to a value
// disagreement they would stop Syndra managing an account that is sitting right
// there, or queue a create for one that already exists, and the API accepted
// both from any caller.
func resolutionFits(f db.MergeFinding, resolution string) error {
	gone := f.Outcome == string(merge.DeletedUpstream)
	switch resolution {
	case db.ResolutionUnbound, db.ResolutionReprovisioned:
		if !gone {
			return fmt.Errorf("%w: %q answers an account that is no longer on the target, and this one is",
				ErrResolutionNotExpressible, resolution)
		}
	case db.ResolutionKeepOurs, db.ResolutionTakeTheirs:
		if gone {
			return fmt.Errorf("%w: there is no account to apply a value to — this binding names one the target no longer has",
				ErrResolutionNotExpressible)
		}
	default:
		return fmt.Errorf("%w: %q is not a resolution this surface offers", ErrResolutionNotExpressible, resolution)
	}
	return nil
}

// errReleaseNotConfirmed is the add-on not confirming that it let go.
var errReleaseNotConfirmed = errors.New("the target did not confirm the release")

// releaseOnTarget makes the ADD-ON stop managing the account, before this side
// forgets its own record of it.
//
// Both stores or neither. The add-on's binding is what an apply consults; the
// backend's row is what the inventory and the reconciliation read. Dropping only
// this side leaves the account managed by half the system — planned and applied
// by an add-on that still binds it, while every surface here calls it unmanaged.
// That is the exact split `account.release` was added to close, and resolving a
// finding was quietly recreating it.
func releaseOnTarget(ctx context.Context, f db.MergeFinding, actor string) error {
	res, err := svcDispatchOperation(ctx, addonop.Request{
		Target: f.Target, Operation: "account.release",
		ActorID: actor, SubjectID: f.SubjectID, Confirmed: true,
	})
	if err != nil {
		return fmt.Errorf("release %s on %s: %w", f.SubjectID, f.Target, err)
	}
	if res.Outcome != addons.OutcomeSucceeded {
		// Nothing forgotten here. The add-on's store is the authority on whether
		// it still binds the subject, and dropping this side's copy against an
		// answer nobody received manufactures the split in the direction that
		// hurts.
		return fmt.Errorf("%w: %s answered %s", errReleaseNotConfirmed, f.Target, res.Outcome)
	}
	return nil
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
		// The add-on has already let go — `releaseOnTarget` ran before this
		// transaction opened. This is the backend's half, and it runs only after
		// the add-on confirmed its own.
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
