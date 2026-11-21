package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"syndra/internal/addons"
	"syndra/internal/clock"
	"syndra/internal/db"
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
	// Which fields are lifecycle ones, by role. Unreadable, none is: nothing
	// is offered for adoption that might not be one.
	roles, _ := addonsFieldRoles(ctx, target)
	schema, _ := addonsEntitlementSchema(target)
	limits := map[string]bool{}
	for _, f := range schema {
		limits[f.Name] = adoptableLimit(f)
	}
	for _, f := range findings {
		view := findingView{MergeFinding: f}
		switch {
		case f.Outcome == string(merge.DeletedUpstream), f.Outcome == string(merge.BindingConflict):
			// Neither adoptable nor a policy question: the account is gone, and
			// the two answers are re-provision or stop managing it; or another
			// holds it, and the one answer is to stop managing it.
		case limits[f.Field]:
			// A limit an operator may set for one person (compared since
			// addon-contract 2.7): the target's value is adopted as their own
			// limit, an allowance with a reason and a bound, like a hold.
			view.Adoptable = true
		case !roles.IsLifecycle(f.Field):
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
	ExpiresAt  *clock.EndDay `json:"expires_at,omitempty"`
	ReviewDate *clock.EndDay `json:"review_date,omitempty"`
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
	// A finding already answered is refused — with ONE exception, and it is the
	// repair path.
	//
	// `unbound` calls the target and then writes this side. If that write fails
	// after the add-on has already let go, the two stores are split exactly the
	// way this resolution exists to prevent: the add-on has forgotten the
	// binding, the backend row remains, and the reservation makes every retry a
	// 409 — a wedge with no way out through this surface.
	//
	// So the SAME decision, repeated, is allowed to run again. It is safe
	// because every step is idempotent: `account.release` answers a subject it
	// does not bind as done rather than refused, and forgetting a row that is
	// already gone is a no-op. A DIFFERENT decision is still refused, because
	// the two answers are opposites.
	calls := callsTarget(req.Resolution)
	repair := finding.Decision != "" && finding.Decision == req.Resolution && calls
	if finding.Decision != "" && !repair {
		// Answered already. Refused here for the reading, and refused again
		// atomically below — this check is the message, not the guarantee.
		writeResolutionError(w, fmt.Errorf("%w: %s already decided as %s by %s",
			db.ErrMergeFindingDecided, id, finding.Decision, finding.DecidedBy))
		return
	}

	// `unbound`, `kept` and `adopted` are the resolutions that call the target
	// (callsTarget), and the ones whose work cannot be undone by a later
	// decision. So the decision is RESERVED first — one conditional write, under
	// the same uniqueness the standing row already has — and only then is the
	// add-on told to let go, or to bind.
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
	var done targetDone
	if calls {
		if !repair {
			// The standing row rides back with the refusal, so the surface can
			// show the second operator what was chosen and why.
			if standing, err := dbRecordMergeDecision(r.Context(), id, actor, req.Resolution, req.Reason); err != nil {
				writeResolutionError(w, mergeDecidedBy{error: err, standing: standing})
				return
			}
		}
		// The reservation goes with the work that did not happen. Left
		// standing, one unreachable add-on would wedge the finding as
		// decided-but-never-done with no way back through this surface.
		//
		// Not on a repair: that reservation is somebody's earlier answer, and
		// this attempt failing does not un-answer it — nor should it drop the
		// only record that the add-on may already have acted.
		undo := func() {
			if !repair {
				if rerr := dbReleaseMergeDecision(r.Context(), id, actor, req.Resolution); rerr != nil {
					log.Printf("[FINDINGS] %s: the call failed and its reservation could not be cleared: %v", id, rerr)
				}
			}
		}
		pair, reservedPair, err := reservePair(r.Context(), finding, req, actor)
		if err != nil {
			undo()
			writeResolutionError(w, err)
			return
		}
		done, err = callTarget(r.Context(), finding, req.Resolution, actor)
		if err != nil {
			undo()
			if reservedPair {
				if rerr := dbReleaseMergeDecision(r.Context(), pair.ID, actor, opposite(req.Resolution)); rerr != nil {
					log.Printf("[FINDINGS] %s: the other person's reservation could not be cleared: %v", pair.ID, rerr)
				}
			}
			writeResolutionError(w, err)
			return
		}
		done.pair = pair
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
		if current.Decision != "" && current.Decision != req.Resolution {
			// Somebody changed the answer between the reservation and here.
			// Refused rather than finished under the other decision's name.
			return fmt.Errorf("%w: %s already decided as %s by %s",
				db.ErrMergeFindingDecided, id, current.Decision, current.DecidedBy)
		}
		if current.Decision != "" && !calls {
			return fmt.Errorf("%w: %s already decided as %s by %s",
				db.ErrMergeFindingDecided, id, current.Decision, current.DecidedBy)
		}
		if err := performResolution(ctx, current, req, actor, done); err != nil {
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
		//
		// `kept` and `adopted` are decisions like the others: the person's
		// binding now names the account (or the other person's is gone), and
		// the sweep closes the row, carrying this decision, once the add-on
		// stops reporting the conflict. Reserved already, so nothing more to write.
		if req.Resolution == db.ResolutionUnbound {
			resolved, err = dbResolveMergeFinding(ctx, id, actor, req.Resolution)
			return err
		}
		if calls {
			resolved = current
			return nil
		}
		var standing db.MergeFinding
		standing, err = dbRecordMergeDecision(ctx, id, actor, req.Resolution, req.Reason)
		if err != nil {
			return mergeDecidedBy{error: err, standing: standing}
		}
		resolved = standing
		return nil
	})

	if err != nil {
		// A failure AFTER the add-on let go is the state this path exists to
		// survive, so it is reported as what it is rather than as a generic
		// error: the target half is done, this side is not, and pressing again
		// finishes it. Silence here is what turned a recoverable half-write into
		// a wedge — the operator saw an error, the controls disappeared, and
		// every retry came back 409.
		if calls && !errors.Is(err, db.ErrMergeFindingDecided) {
			// The cause is for the log; the body is the envelope plus the repair state.
			ref := supportReference()
			log.Printf("[HTTP] repair needed ref=%s finding=%s: %v", ref, id, err)
			detail := "The target released the account and Syndra's own records were not updated. " +
				"Nothing on the target changed. Press unbind again to finish it, every step is safe to repeat."
			switch req.Resolution {
			case db.ResolutionAdopted:
				detail = "The system bound the account to this person and Syndra's own records were not updated. " +
					"Nothing on the account changed. Choose it again to finish it, every step is safe to repeat."
			case db.ResolutionKept:
				detail = "The system let go of the other person's claim and Syndra's own records were not updated. " +
					"Nothing on the account changed. Choose it again to finish it, every step is safe to repeat."
			}
			// The target half happened, so it is audited like a finished one.
			_ = dbInsertAuditLogDetails(r.Context(), actor, finding.SubjectID, "merge_finding.decided", id, map[string]any{
				"target": finding.Target, "outcome": finding.Outcome, "resolution": req.Resolution, "reason": req.Reason,
				"repair_needed": true,
			})
			jsonResponse(w, http.StatusAccepted, struct {
				ErrorResponse
				Resolved     bool   `json:"resolved"`
				Decided      bool   `json:"decided"`
				Resolution   string `json:"resolution"`
				RepairNeeded bool   `json:"repair_needed"`
				Detail       string `json:"detail"`
			}{
				ErrorResponse{Error: "REPAIR_NEEDED", Message: detail, Details: map[string]string{"request_id": ref}},
				false, true, req.Resolution, true, detail,
			})
			return
		}
		writeResolutionError(w, err)
		return
	}

	// `resolved` says the finding is closed; `decided` says it is answered and
	// waiting. Reporting the second as the first is what made the queue refill
	// itself, and it would read to an operator as the surface being broken.
	settled := req.Resolution == db.ResolutionUnbound
	detail := "Decided. The finding stays open until a reconciliation sees the target agree."
	switch req.Resolution {
	case db.ResolutionUnbound:
		detail = "Syndra no longer manages that account. Nothing on the target was changed."
	case db.ResolutionAdopted:
		detail = "The account is now this person's. Nothing on it was changed. The finding closes at the next check."
	case db.ResolutionKept:
		detail = "The account stays this person's, and the other person's claim on it is released: their next change creates a new account for them. Both findings close at the next check."
	}
	// Who decided what, and why, in the audit log beside the row's own record
	// (decided_by, resolved_by): every resolution, whichever it is.
	_ = dbInsertAuditLogDetails(r.Context(), actor, finding.SubjectID, "merge_finding.decided", id, map[string]any{
		"target": finding.Target, "outcome": finding.Outcome, "resolution": req.Resolution, "reason": req.Reason,
	})
	jsonResponse(w, http.StatusOK, map[string]any{
		"resolved": settled, "decided": true,
		"finding": resolved, "resolution": req.Resolution, "detail": detail,
	})
}

func writeResolutionError(w http.ResponseWriter, err error) {
	if writeNotLanded(w, err) {
		return
	}
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
	case errors.Is(err, errReleaseRefused):
		// The add-on's words: what it found, and Check again. Nothing was released.
		jsonErrorResponse(w, http.StatusConflict, "RELEASE_REFUSED", strings.TrimPrefix(err.Error(), errReleaseRefused.Error()+": "))
	case errors.Is(err, errAdoptionRefused):
		// The add-on judged it again and said no (a stale adoptable): its own
		// sentence, which says what changed and to check again. Nothing changed.
		jsonErrorResponse(w, http.StatusConflict, "ADOPTION_REFUSED", strings.TrimPrefix(err.Error(), errAdoptionRefused.Error()+": "))
	case errors.Is(err, db.ErrMergeFindingDecided):
		// 409, and it names who, what and WHY. The two answers here are
		// opposites, so a second operator has to know that somebody chose — and
		// the reason is what lets them agree or disagree rather than only lose.
		// It was made mandatory for this reader and then shown to everybody
		// except them.
		//
		// The API's own sentence is not the copy: a surface renders these
		// fields, and a UUID beside a snake_case resolution is raw material.
		jsonResponse(w, http.StatusConflict, ErrorResponse{
			Error:   "ALREADY_DECIDED",
			Message: err.Error(),
			Details: decidedDetails(err),
		})
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
//
// A `binding_conflict` takes `unbound` always (stop tracking: account.release,
// the default; design 7t §3), and, by the owner's ruling of 17 Nov 2026, two
// more where the stored report says they can work: `adopted` (account.adopt
// moves the binding to the holder) only when the add-on marked the holder
// adoptable and the conflict is the binding's name or number being held; and
// `kept` (account.release of the OTHER person) only when the account is
// claimed twice and the other person is named. The add-on judges each again
// when it runs: this gate is what the stored report allows, never a promise.
// Re-provisioning is the create the add-on refuses, and there is no account of
// the subject's to keep or take a value on.
func resolutionFits(f db.MergeFinding, resolution string) error {
	if f.Outcome == string(merge.BindingConflict) {
		c := conflictOf(f)
		switch {
		case resolution == db.ResolutionUnbound:
			return nil
		case resolution == db.ResolutionAdopted:
			if c.Adoptable && c.Username != "" && c.AccountID != "" && (c.Reason == addons.ConflictNameHeld || c.Reason == addons.ConflictNumberHeld) {
				return nil
			}
			return fmt.Errorf("%w: the account holding this person's name or number is not one the system can give them",
				ErrResolutionNotExpressible)
		case resolution == db.ResolutionKept:
			if c.Reason == addons.ConflictClaimedTwice && c.BoundTo != "" && c.BoundTo != f.SubjectID {
				return nil
			}
			return fmt.Errorf("%w: keeping it for this person answers an account recorded for two people, and this one is not",
				ErrResolutionNotExpressible)
		}
		return fmt.Errorf("%w: %q does not answer a binding whose name or number another account holds; stopping tracking it does",
			ErrResolutionNotExpressible, resolution)
	}
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

// errReleaseRefused is the add-on refusing the release: its binding names another
// account than the one Syndra meant (the precondition), or another refusal in
// its words. Nothing was released.
var errReleaseRefused = errors.New("the system refused the release")

// releaseOnTarget makes the ADD-ON stop managing the account, before this side
// forgets its own record of it.
//
// Both stores or neither. The add-on's binding is what an apply consults; the
// backend's row is what the inventory and the reconciliation read. Dropping only
// this side leaves the account managed by half the system — planned and applied
// by an add-on that still binds it, while every surface here calls it unmanaged.
// That is the exact split `account.release` was added to close, and resolving a
// finding was quietly recreating it.
//
// accountID is the account Syndra means, sent as the precondition (RFC 9110
// §13.1.1) when it is known: a person released and bound again since names a
// new account, which the add-on then refuses to drop (409, errReleaseRefused).
func releaseOnTarget(ctx context.Context, target, subject, actor, accountID string) error {
	res, err := dispatched(svcDispatchOperation(ctx, addonop.Request{
		Target: target, Operation: "account.release",
		ActorID: actor, SubjectID: subject, Confirmed: true, Params: releaseParams(accountID),
		About: boundUsername(ctx, target, subject),
	}))
	if err != nil {
		return fmt.Errorf("release %s on %s: %w", subject, target, err)
	}
	if res.Outcome == addons.OutcomeRejected {
		return fmt.Errorf("%w: %s", errReleaseRefused, releaseRefusal(res.Err))
	}
	if res.Outcome != addons.OutcomeSucceeded {
		// Nothing forgotten here. The add-on's store is the authority on whether
		// it still binds the subject, and dropping this side's copy against an
		// answer nobody received manufactures the split in the direction that
		// hurts.
		return notLandedOf(ctx, target, res.Outcome, res.Why)
	}
	return nil
}

// errAdoptionRefused is the add-on refusing the adoption (a stale adoptable,
// judged again when it ran). No answer either way is a notLanded.
var errAdoptionRefused = errors.New("the system refused the adoption")

// adoptOnTarget is "It's theirs: adopt it" on a binding conflict: the one
// account.adopt (handleAdoptAccount's operation, the same policy, confirmation,
// record and audit through svcDispatchOperation), naming the holder by its
// name and, as a precondition, by the account_id the operator was shown
// (RFC 9110 §13.1.1, If-Match): the add-on refuses if that name now belongs to
// another account. It answers the account_id the add-on bound.
func adoptOnTarget(ctx context.Context, f db.MergeFinding, actor string) (string, error) {
	holder := conflictOf(f)
	res, err := dispatched(svcDispatchOperation(ctx, addonop.Request{
		Target: f.Target, Operation: "account.adopt",
		ActorID: actor, SubjectID: f.SubjectID, Confirmed: true,
		Params: adoptParams(holder.Username, holder.AccountID), About: holder.Username,
	}))
	if err != nil {
		return "", fmt.Errorf("adopt %s for %s on %s: %w", holder.Username, f.SubjectID, f.Target, err)
	}
	switch res.Outcome {
	case addons.OutcomeSucceeded:
		return res.AccountID, nil
	case addons.OutcomeRejected:
		// The add-on's own sentence: what changed, and Check again.
		return "", fmt.Errorf("%w: %s", errAdoptionRefused, adoptionRefusal(res.Err))
	default:
		// Unreached or indeterminate, said apart (writeOutcome).
		return "", notLandedOf(ctx, f.Target, res.Outcome, res.Why)
	}
}

// releaseParams is account.release's params: the account_id meant, when known.
func releaseParams(accountID string) map[string]any {
	if accountID == "" {
		return map[string]any{}
	}
	return map[string]any{"account_id": accountID}
}

// adoptParams is account.adopt's params, for every caller: the name, and the
// account_id shown when there is one (the precondition).
func adoptParams(username, accountID string) map[string]any {
	params := map[string]any{"username": username}
	if accountID != "" {
		params["account_id"] = accountID
	}
	return params
}

// targetDone is what a resolution's call to the target left for this side's
// half: the account an adoption bound, and the other claimant's standing
// finding a claimed-twice answer also settles.
type targetDone struct {
	accountID string
	pair      *db.MergeFinding
}

// callsTarget is whether a resolution calls the add-on before this side
// writes: those are reserved first and repeatable as a repair.
func callsTarget(resolution string) bool {
	switch resolution {
	case db.ResolutionUnbound, db.ResolutionAdopted, db.ResolutionKept:
		return true
	}
	return false
}

// callTarget makes the one call a resolution needs: a release of this person
// (stop tracking), a release of the OTHER person (kept: never this one), or
// the adoption.
func callTarget(ctx context.Context, f db.MergeFinding, resolution, actor string) (targetDone, error) {
	switch resolution {
	case db.ResolutionUnbound:
		return targetDone{}, releaseOnTarget(ctx, f.Target, f.SubjectID, actor, releasedAccount(ctx, f))
	case db.ResolutionKept:
		// The account both claimed: if the other person was released and bound again
		// since, their new account is not this one, and the add-on refuses. And the person it is
		// kept FOR must still be tracked: untracked elsewhere in between, releasing the other
		// would leave the account bound to nobody. Whether they are bound is what Syndra's own
		// row says (an id on it can lag, so only its presence is judged: a refusal "Check again"
		// could not clear would be the same wedge). Same on a Finish: it runs this again.
		if _, bound, err := dbGetTargetBinding(ctx, f.Target, f.SubjectID); err != nil {
			return targetDone{}, fmt.Errorf("read this person's record on %s: %w", f.Target, err)
		} else if !bound {
			if f.Decision == db.ResolutionKept {
				// A Finish: the other person's claim was already let go of, and a decided row
				// refuses any other choice, so the only true next step is to wait for the sweep.
				return targetDone{}, fmt.Errorf("%w: This person is no longer tracked for that account, so Syndra's own record of "+
					"the other person's claim was not tidied. Nothing more to do here: Syndra closes this when it next checks.", errReleaseRefused)
			}
			return targetDone{}, fmt.Errorf("%w: This person is no longer tracked for that account, so letting go of the other "+
				"person's claim would leave it with nobody. Nothing was changed. Check again to see who holds it now, "+
				"then choose from what it shows.", errReleaseRefused)
		}
		return targetDone{}, releaseOnTarget(ctx, f.Target, conflictOf(f).BoundTo, actor, conflictOf(f).AccountID)
	case db.ResolutionAdopted:
		id, err := adoptOnTarget(ctx, f, actor)
		return targetDone{accountID: id}, err
	}
	return targetDone{}, fmt.Errorf("%q does not call the target", resolution)
}

// releasedAccount is the account a Stop tracking means, sent as the release's
// precondition: the account two people claim, or the one this person's record
// names (the mirror), "" when Syndra does not know one.
func releasedAccount(ctx context.Context, f db.MergeFinding) string {
	if c := conflictOf(f); c.Reason == addons.ConflictClaimedTwice {
		return c.AccountID
	}
	return boundAccountID(ctx, f.Target, f.SubjectID)
}

// opposite is what an answer on one claimant's finding means on the other's,
// when one account is claimed twice: keeping it for one is stopping tracking
// it for the other, either way round.
func opposite(resolution string) string {
	if resolution == db.ResolutionKept {
		return db.ResolutionUnbound
	}
	return db.ResolutionKept
}

// reservePair reserves the other claimant's standing finding with the same
// answer seen from their side, before the target is called, so "Keep it for
// A" and "Keep it for B" (on either person's row) can never both run: the two
// releases would leave the account to neither. Only for claimed_twice, and
// only when the other person's finding stands; reserved reports whether this
// call took the reservation (to give back if the call fails). A finding
// already answered that way is the same decision and is not reserved again.
func reservePair(ctx context.Context, f db.MergeFinding, req resolveMergeFindingRequest, actor string) (*db.MergeFinding, bool, error) {
	c := conflictOf(f)
	if f.Outcome != string(merge.BindingConflict) || c.Reason != addons.ConflictClaimedTwice || c.BoundTo == "" ||
		(req.Resolution != db.ResolutionUnbound && req.Resolution != db.ResolutionKept) {
		return nil, false, nil
	}
	standing, err := dbStandingMergeFindings(ctx, f.Target)
	if err != nil {
		return nil, false, fmt.Errorf("read the other person's finding on %s: %w", f.Target, err)
	}
	want := opposite(req.Resolution)
	for i := range standing {
		g := standing[i]
		if g.SubjectID != c.BoundTo || g.Outcome != string(merge.BindingConflict) || g.Field != "" {
			continue
		}
		switch g.Decision {
		case want:
			return &g, false, nil
		case "":
			if held, err := dbRecordMergeDecision(ctx, g.ID, actor, want, req.Reason); err != nil {
				return nil, false, mergeDecidedBy{error: err, standing: held}
			}
			return &g, true, nil
		default:
			return nil, false, mergeDecidedBy{error: fmt.Errorf("%w: %s already decided as %s by %s",
				db.ErrMergeFindingDecided, g.ID, g.Decision, g.DecidedBy), standing: g}
		}
	}
	return nil, false, nil
}

// conflictOf is the add-on's report a binding_conflict finding stores as
// theirs (common/binding_conflict.json); zero when it is not one.
func conflictOf(f db.MergeFinding) addons.SubjectConflict {
	var c addons.SubjectConflict
	if f.Outcome == string(merge.BindingConflict) {
		_ = json.Unmarshal(f.Theirs, &c)
	}
	return c
}

// performResolution carries out the decision. It writes; it does not close.
func performResolution(ctx context.Context, f db.MergeFinding, req resolveMergeFindingRequest, actor string, done targetDone) error {
	switch req.Resolution {
	case db.ResolutionKeepOurs, db.ResolutionReprovisioned:
		// Syndra's state, applied over the target's. Queued like every other
		// convergence rather than written here: the apply path is the only thing
		// that talks to a target, and its read-back is what records the new base
		// — which is what stops this finding returning on the next pass.
		set, err := svcResolveEntitlementSet(ctx, f.SubjectID, f.Target)
		if err != nil {
			return fmt.Errorf("resolve %s on %s: %w", f.SubjectID, f.Target, err)
		}
		reason := "An operator kept Syndra's state over the target's"
		if req.Resolution == db.ResolutionReprovisioned {
			reason = "An operator asked for a deleted account to be provisioned again"
		}
		if _, _, _, err := dbRecordSystemConvergence(ctx, set.Convergence(f.Target, f.SubjectID, actor, reason)); err != nil {
			return fmt.Errorf("queue convergence for %s: %w", f.SubjectID, err)
		}
		return nil

	case db.ResolutionTakeTheirs:
		return adoptTargetValue(ctx, f, req, actor)

	case db.ResolutionUnbound:
		// The add-on has already let go — `releaseOnTarget` ran before this
		// transaction opened. This is the backend's half, and it runs only after
		// the add-on confirmed its own: the same half as a release's.
		if err := forgetReleasedBinding(ctx, f.Target, f.SubjectID); err != nil {
			return fmt.Errorf("forget the binding for %s: %w", f.SubjectID, err)
		}
		return nil

	case db.ResolutionKept:
		// The other person's binding is what the add-on let go of: this side
		// forgets theirs (never this person's), and their finding, the same
		// answer from their side, is settled with it.
		other := conflictOf(f).BoundTo
		if err := forgetReleasedBinding(ctx, f.Target, other); err != nil {
			return fmt.Errorf("forget the binding for %s: %w", other, err)
		}
		if done.pair != nil {
			if _, err := dbResolveMergeFinding(ctx, done.pair.ID, actor, db.ResolutionUnbound); err != nil &&
				!errors.Is(err, db.ErrNoSuchMergeFinding) {
				return err
			}
		}
		return nil

	case db.ResolutionAdopted:
		// The backend's half of the one adoption (recordAdoptedBinding, as
		// handleAdoptAccount and a /calls settle write it), which also forgets
		// the old account's observation when the person moves.
		return recordAdoptedBinding(ctx, f.Target, f.SubjectID, conflictOf(f).Username, actor, done.accountID)

	default:
		return fmt.Errorf("%q is not a resolution this surface offers", req.Resolution)
	}
}

// adoptableLimit is a field whose target value can be adopted for one person:
// a `limit` the add-on lets an operator set per person (design §3), whose
// one-person form is an allowance with direction "set".
func adoptableLimit(f addons.EntitlementField) bool {
	return f.Role == addons.RoleLimit && f.PerPerson
}

// adoptOwnLimit keeps the limit the target holds for this person by making it
// their own limit: a `set` allowance with the target's observed value, an
// author, a reason and an end or a review date (the store refuses one without),
// written with its audit row. The resolver then wants what the target holds,
// so the next pass agrees and settles the finding; nothing is written to the
// target.
func adoptOwnLimit(ctx context.Context, f db.MergeFinding, req resolveMergeFindingRequest, actor string) error {
	var held []string
	if err := json.Unmarshal(f.Theirs, &held); err != nil || len(held) != 1 {
		return fmt.Errorf("%w: the target's value for %s is not one limit", ErrResolutionNotExpressible, f.Field)
	}
	// Structure only: this runs under the access lock, and a limit is never
	// enumerable, so asking /values would only let a slow add-on stall every
	// grant for nothing.
	field, value, _, err := validateAllowanceStructure(ctx, f.Target, f.Field, held[0])
	if err != nil {
		return err
	}
	if n, err := strconv.ParseInt(value, 10, 64); err != nil || n < 0 {
		return fmt.Errorf("%w: the target's value for %s, %q, is not a whole number", ErrResolutionNotExpressible, f.Field, value)
	}
	expires, review := adoptionBound(req)
	_, err = dbCreateAllowance(ctx, db.Allowance{
		SubjectID: f.SubjectID, Target: f.Target,
		Field: field, Value: value, Direction: db.AllowanceSet,
		ActorID: actor, Reason: req.Reason,
		ExpiresAt: expires, ReviewDate: review,
	})
	return err
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
	schema, _ := addonsEntitlementSchema(f.Target)
	for _, field := range schema {
		if field.Name == f.Field && adoptableLimit(field) {
			return adoptOwnLimit(ctx, f, req, actor)
		}
	}
	roles, err := addonsFieldRoles(ctx, f.Target)
	if err != nil {
		return fmt.Errorf("read the field roles of %s: %w", f.Target, err)
	}
	if !roles.IsLifecycle(f.Field) {
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
	expires, review := adoptionBound(req)
	_, err = dbCreateAllowance(ctx, db.Allowance{
		SubjectID: f.SubjectID, Target: f.Target,
		Field: f.Field, Value: "true", Direction: db.AllowanceDeny,
		ActorID: actor, Reason: req.Reason,
		ExpiresAt: expires, ReviewDate: review,
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

// decidedDetails carries what the second operator needs from a refusal: what
// was chosen, by whom, and why.
//
// The reason in particular. It is mandatory on every resolution and it exists
// for exactly this reader — a finding takes one decision, so somebody arriving
// second can only agree or disagree, and neither is possible from "already
// decided as take_theirs by u_5f2c". Until now it was written and shown to
// everybody except the person it was written for.
func decidedDetails(err error) map[string]string {
	var standing db.MergeFinding
	var carrier mergeDecidedBy
	if !errors.As(err, &carrier) {
		return nil
	}
	standing = carrier.standing
	details := map[string]string{"decision": standing.Decision, "decided_by": standing.DecidedBy}
	if standing.DecisionReason != "" {
		details["decision_reason"] = standing.DecisionReason
	}
	if standing.DecidedAt != nil {
		details["decided_at"] = standing.DecidedAt.UTC().Format(time.RFC3339)
	}
	return details
}

// mergeDecidedBy carries the standing finding alongside the refusal, so the
// surface renders fields rather than parsing a sentence.
type mergeDecidedBy struct {
	error
	standing db.MergeFinding
}

func (m mergeDecidedBy) Unwrap() error { return m.error }

// adoptionReviewMonths is how long an adopted decision stands before it is
// looked at again when the operator names no bound.
const adoptionReviewMonths = 6

// adoptionBound is the bound an adopted value carries: the dates the operator
// sent, or, with neither, a review date six months on from today. The schema
// refuses an unbounded allowance; six months is a review interval, not an
// expiry (guessing when somebody else's decision lapses would be inventing
// policy). The backend owns the date, as it does the end of a day: the page
// does not work out time.
func adoptionBound(req resolveMergeFindingRequest) (expires, review *time.Time) {
	expires, review = req.ExpiresAt.Instant(), req.ReviewDate.Instant()
	if expires == nil && review == nil {
		r := clock.EndAfterMonths(time.Now(), adoptionReviewMonths)
		review = &r
	}
	return expires, review
}
