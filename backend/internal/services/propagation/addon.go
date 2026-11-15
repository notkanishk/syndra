package propagation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"

	"syndra/internal/addons"
	"syndra/internal/db"
	"syndra/internal/models"
	"syndra/internal/services/merge"
)

// The add-on dispatcher (design §4, §15).
//
// A drain holds exactly one dispatcher, which is why the claim is scoped to one
// target: the Zitadel pass speaks the Management API and would mark an
// `op_type='apply'` row terminally failed as an unknown operation, with no way
// back from `failed`. This is the other pass.
//
// It applies the **recorded** snapshot rather than re-resolving. An
// operator-initiated change is an approval, and the diff that was approved is
// the diff that must land; re-resolving here would dispatch whatever policy
// says at drain time, which is a different decision wearing the approval's plan
// id. Periodic reconciliation is the other read path and deliberately does
// resolve current state — conflating the two is how a reconcile loop reverts an
// intentional edit.

// ErrBuiltInTarget is the add-on dispatcher being handed Zitadel. A sentinel
// rather than a message, so the HTTP layer can answer "wrong route" instead of
// reporting a downstream failure that did not happen.
var ErrBuiltInTarget = errors.New("zitadel is the built-in target and has its own dispatcher")

// DrainAddon dispatches one add-on target's queued entitlement work.
//
// Operator-triggered like the Zitadel pass, and for the same reason: these rows
// confer access, and the drain rule narrows to grants rather than disappearing.
// An `apply` carries a resolved set rather than a direction, so the one kind
// the background runner sends without an operator is a withdraw-only row
// (EntitlementSet.WithdrawsOnly: a departure or a denied account), whose
// desired state is the lifecycle switch alone and so cannot give anything.
// Every other add-on row waits for this pass.
func DrainAddon(ctx context.Context, target string) (DrainResult, error) {
	if target == db.TargetZitadel {
		// Refused rather than ignored. The Zitadel pass exists and is not this
		// one; a caller that reached here with the built-in target has a wrong
		// model of which dispatcher does what.
		return DrainResult{}, fmt.Errorf("drain add-on: %w", ErrBuiltInTarget)
	}

	release, acquired, err := acquireDrainLock(ctx)
	if err != nil {
		return DrainResult{}, fmt.Errorf("acquire drain lock: %w", err)
	}
	if !acquired {
		return DrainResult{Halted: true, Reason: "drain_in_progress"}, nil
	}
	defer release()

	res, err := drainAddonPass(ctx, target)
	if err != nil {
		return DrainResult{}, err
	}
	// The single-target entry prunes too. Retention is global work that any
	// drain may do, and an operator who only ever resumes one target would
	// otherwise never prune anything.
	pruneAfterDrain(ctx)
	return res, nil
}

// drainAddonPass is one add-on target's leg. Caller holds the drain lock.
func drainAddonPass(ctx context.Context, target string) (DrainResult, error) {
	// One probe, before any row is claimed, exactly as the Zitadel passes do.
	// A batch dispatched into an outage spends one retry per row to learn what
	// a single call establishes — and an exhausted budget is terminal, so the
	// rows an operator approved would be failed by the target being switched
	// off rather than left queued for it coming back.
	if !addonReachable(ctx, target) {
		return unanswered(ctx, target, "target_unreachable"), nil
	}

	rows, err := claimPending(ctx, target, claimBatch)
	if err != nil {
		return DrainResult{}, fmt.Errorf("claim pending: %w", err)
	}

	var res DrainResult
	if waiting, err := awaitingDispatch(ctx, target); err != nil {
		log.Printf("[ADDON-DRAIN] could not list targets awaiting dispatch: %v (non-fatal)", err)
	} else {
		res.Awaiting = waiting
	}

	for _, row := range rows {
		if halt := res.dispatchEntitlement(ctx, row); halt {
			return res, nil
		}
	}
	return res, nil
}

// dispatchEntitlement sends one row and records what came back.
func (res *DrainResult) dispatchEntitlement(ctx context.Context, row models.PendingPropagation) (halt bool) {
	if row.OpType != "apply" {
		// The claim is target-scoped, not shape-scoped, so a Zitadel-shaped row
		// on an add-on target would arrive here. Failed terminally rather than
		// requeued: it names a project and roles this dispatcher cannot send,
		// and no number of retries changes that.
		if err := markFailed(ctx, row.ID, fmt.Sprintf("op_type %q is not an entitlement apply", row.OpType)); err != nil {
			res.settleFailure(row.ID, "mark failed", err)
		} else {
			res.Failed++
		}
		return false
	}

	// Its calls start now, whatever the claim stamped for the whole batch: the
	// drift sweep reads started_at as when this row's calls began
	// (db.AppliesInFlightSince), and a row sent minutes after its batch was
	// claimed must not fall outside that window while it can still land. Not
	// stamped, not sent: it stays in flight and the next pass claims it again.
	if err := markSending(ctx, row.ID); err != nil {
		res.settleFailure(row.ID, "mark sending", err)
		return false
	}

	intent, err := readIntent(ctx, row.ID)
	if err != nil {
		if !errors.Is(err, db.ErrNoApprovedIntent) {
			// The read failed, not the citation. A connection blip must not
			// permanently fail an approved entitlement change — `failed` has no
			// way back — so the row stays claimable and this pass records why.
			res.persistErr(row.ID, "read intent", err)
			return false
		}
		// A row with no approved snapshot must not be dispatched. Dispatching
		// an empty desired state would converge the subject to nothing, which
		// is the most destructive possible reading of a missing citation.
		if markErr := markFailed(ctx, row.ID, "this row cites no approved desired state"); markErr != nil {
			res.settleFailure(row.ID, "mark failed", markErr)
			return false
		}
		log.Printf("[ADDON-DRAIN] outbox=%s has no approved intent: %v", row.ID, err)
		res.Failed++
		return false
	}

	// The two ways a citation can be present and still say nothing. `Desired`
	// returns nil for an absent or unparseable snapshot precisely so this
	// decision is made here — and the decision is to refuse, because nil sent
	// as a desired state is zero managed fields, which the add-on answers with
	// `no_change` and this drain would record as converged. An approval that
	// did nothing, marked applied.
	//
	// A missing fingerprint is the same shape of nothing: the add-on's check
	// against live state has nothing to compare, and "the diff you approved is
	// the diff that lands" stops being a property of the system.
	switch {
	case intent.Desired() == nil:
		if markErr := markFailed(ctx, row.ID, "the approved desired state could not be read"); markErr != nil {
			res.settleFailure(row.ID, "mark failed", markErr)
			return false
		}
		res.Failed++
		return false
	case intent.Fingerprint == "":
		if markErr := markFailed(ctx, row.ID, "the cited plan subject carries no fingerprint to verify against"); markErr != nil {
			res.settleFailure(row.ID, "mark failed", markErr)
			return false
		}
		res.Failed++
		return false
	}

	fingerprint, halt := res.fingerprintFor(ctx, intent)
	if halt {
		return false
	}

	resp := applyEntitlement(ctx, addons.ApplyRequest{
		Target:  intent.Target,
		Subject: intent.SubjectID,
		// The identity a username is derived from, and only for a subject with
		// no account yet — an existing binding is authoritative and a later
		// email change must never rename an account. Resolved here rather than
		// recorded on the plan: the plan store deliberately holds no name or
		// email, and the directory is where identity lives.
		Email:       subjectEmail(ctx, intent.SubjectID),
		Fingerprint: fingerprint,
		PlanID:      intent.PlanID,
		// Who decided this, carried through to the add-on's mutation log. That
		// log promises who did what to whom, and the add-on knows only the whom.
		Actor: row.InitiatedBy,
		// The outbox row's id is the deduplication token. Reusing it rather
		// than minting one is what makes a re-drive safe: the add-on returns
		// the original outcome instead of converging twice, and the row that
		// authorised the work is the row that names it.
		CallID:  row.ID,
		Desired: intent.Desired(),
	})

	switch {
	case resp.Outcome == addons.OutcomeSucceeded:
		// Recorded before the row is settled, and non-fatal: the binding is the
		// backend's mirror of a decision the add-on already made and persisted,
		// so failing to copy it does not un-apply anything. It is never a
		// conflict either: the add-on's answer is the authority, so the mirror
		// follows it and clears another subject's stale row on the same account
		// (db.RecordTargetBinding). Two live claims on one account are the
		// add-on's to report (/subjects claimed_twice), and it refuses them.
		recordBinding(ctx, intent, resp, row.InitiatedBy)
		recordMergeBase(ctx, intent, resp)
		if err := markApplied(ctx, row.ID); err != nil {
			res.settleFailure(row.ID, "apply", err)
			return false
		}
		res.Applied++

	case resp.LifecycleRefusal:
		// A deliberate maintenance window. Accounted as queued, never failed:
		// treating it as terminal would convert every pending change into a
		// failed row during exactly the window an operator is least able to
		// notice.
		//
		// RELEASED rather than requeued, so it spends no retry. Nothing was
		// attempted — the add-on declined before doing anything — and a window
		// long enough to be worth having would otherwise exhaust the budget of
		// every queued row and halt the drain on rows that were never sent.
		if err := release(ctx, row.ID, systemName(ctx, row.Target)+" is not taking changes right now"); err != nil {
			res.settleFailure(row.ID, "release", err)
			return false
		}
		res.Requeued++

	case resp.Outcome == addons.OutcomeRejected:
		// The add-on validated the call and refused; it did not act. Terminal,
		// because retrying a deterministic refusal changes nothing.
		// A stale plan is not the target refusing the change: the subject moved
		// since the approval, and a fresh plan may well be accepted, so it holds
		// nobody's next withdrawal (db.WaitReasonPlanStale). A binding conflict
		// is a standing decision for an operator, not only this row's reason:
		// the same finding the sweep raises, now, and the row's own wait reason
		// (db.WaitReasonBindingConflict) is what sends every page that lists it
		// to Waiting on you. Any other refusal a retry would repeat holds the
		// person's next withdrawal (db.WaitReasonTargetRefused).
		settle := markTargetRefused
		switch {
		case resp.Code == addons.CodePlanStale:
			settle = markPlanStale
		case raiseConflict(ctx, intent.Target, intent.SubjectID, resp.Conflict):
			settle = markBindingConflict
		}
		if err := settle(ctx, row.ID, refusalReason(resp)); err != nil {
			res.settleFailure(row.ID, "mark failed", err)
			return false
		}
		res.Failed++

	case resp.Outcome == addons.OutcomeUnreached:
		if row.Attempts >= maxRetries {
			// Terminal, and the pass continues — the same rule the Zitadel
			// drain follows, and for the same reason: a row left non-terminal
			// at the head of the queue is one every later pass re-claims and
			// halts on, which stops everything behind it without saying so.
			reason := fmt.Sprintf("out of retries after %d attempts: %s", row.Attempts, unreachedWhy(systemName(ctx, row.Target), resp))
			if err := markFailed(ctx, row.ID, reason); err != nil {
				res.settleFailure(row.ID, "mark failed", err)
				return false
			}
			res.Failed++
			res.Exhausted++
			return false
		}
		if _, err := requeue(ctx, row.ID, unreachedWhy(systemName(ctx, row.Target), resp)); err != nil {
			res.settleFailure(row.ID, "requeue", err)
			return false
		}
		res.Requeued++

	default:
		// Indeterminate: sent, answer lost, may have applied. Neither success
		// nor failure, so it is left non-terminal and counted as errored — the
		// honest state, and the one the unresolved surface is built to show.
		//
		// Left CLAIMABLE, deliberately, and this is where an entitlement apply
		// parts company with a one-shot operation. An operation carries a secret
		// and cannot be re-sent, so an indeterminate one waits for a human. An
		// apply carries level-triggered desired state under a stable call id:
		// re-driving it either learns the original outcome from the add-on's
		// dedup or converges to the same state. The next pass is the resolution
		// path, not a duplicated mutation.
		res.persistErr(row.ID, "dispatch", errIndeterminate)
	}
	return false
}

var errIndeterminate = errors.New("the call was sent and the answer was lost; whether it applied is unknown")

// settleFailure records a state write that did not land, distinguishing a row
// somebody else terminated from one this drain simply could not settle.
func (res *DrainResult) settleFailure(id, step string, err error) {
	if !res.abandoned(id, step, err) {
		res.persistErr(id, step, err)
	}
}

// unreachedWhy is why an unreached row waits, in words true of its code and in the copy
// guide's voice: the system by its name (system, addons.SystemNameOf), never "the target".
// TARGET_CHANGED is unreached like TARGET_UNREACHABLE (addons.classifyStatus, one path); only
// the reason differs. It lands in last_error, which the pages show.
func unreachedWhy(system string, resp addons.ApplyResponse) string {
	if resp.Code == addons.CodeTargetChanged {
		// The add-on read the target and it changed underneath: reached, and still nothing sent.
		return system + " changed while Syndra was reading it"
	}
	return system + " could not be reached"
}

// refusalReason is the add-on's own answer, never its response body.
//
// The body is whatever the least trusted component chose to send back, and this
// string lands in `last_error`, which an operator reads and a surface renders
// after "{system} refused it:". So it is the add-on's words as written: what
// happened (detail) and what to do next (consequence), the next step a blocked
// apply carries (apply_response.json), never lost.
func refusalReason(resp addons.ApplyResponse) string {
	if resp.Code == addons.CodePlanStale {
		// The one refusal whose next step is to make the change again rather
		// than to fix something. Plain words, no code (the row's wait_reason,
		// db.WaitReasonPlanStale, is what a page switches on), and no system
		// name, so a renamed system cannot make it wrong.
		return planStaleReason
	}
	if words := addonWords(resp.Detail, resp.Consequence); words != "" {
		return words
	}
	// No words: stored as none (never the HTTP status, copy guide §8). The row's
	// wait_reason says it was the system's refusal; the page words the absence.
	return ""
}

// planStaleReason is a PLAN_STALE row's last_error (copy guide §16).
const planStaleReason = "Their account changed after this change was approved, so it was not sent. Make the change again to send it."

// addonWords is an add-on's refusal as it said it: what happened, then what to
// do next, each shown as written.
func addonWords(detail, next string) string {
	return strings.TrimSpace(detail + " " + next)
}

// raiseConflict records an add-on's binding conflict from a blocked apply or
// plan as the standing finding the drift sweep raises from /subjects: the same
// outcome, the same (target, subject, "") slot, the same report as theirs
// (merge.Conflicted), so the operator sees it under Waiting on you now rather
// than at the next sweep, and the sweep then refreshes this one row rather than
// adding a second. Nil is no conflict. Non-fatal: the row still settles with the
// add-on's words, and the next sweep raises it again.
//
// It reports whether the conflict is a binding's, which is what makes the row a
// binding conflict (db.WaitReasonBindingConflict) rather than a plain refusal.
func raiseConflict(ctx context.Context, target, subject string, conflict *addons.BindingConflict) bool {
	if conflict == nil || !bindingsConflict(ctx, target, subject, conflict) {
		return false
	}
	report, err := json.Marshal(addons.SubjectConflict{Subject: subject, BindingConflict: *conflict})
	if err != nil {
		log.Printf("[ADDON-DRAIN] the conflict for %s on %s could not be encoded: %v (the next sweep raises it)", subject, target, err)
		return true
	}
	for _, f := range merge.Conflicted(subject, report).Findings() {
		if err := saveMergeFinding(ctx, db.MergeFinding{
			Target: target, SubjectID: f.SubjectID, Field: f.Field, Outcome: string(f.Outcome), Theirs: f.Theirs,
		}); err != nil {
			log.Printf("[ADDON-DRAIN] the conflict for %s on %s could not be recorded: %v (the next sweep raises it)",
				subject, target, err)
		}
	}
	return true
}

// bindingsConflict says whether a conflict is a recorded binding's, rather than
// a create's (the name a new account would be made under is taken). Only a
// binding's is in /subjects' conflicts, so only it is a finding: a create's
// would never be listed, and `unbound` would release nothing. That refusal
// keeps its own path: the refused row and the add-on's words.
//
// The contract says which (common/binding_conflict.json): number_held is a
// recorded number, ambiguous the key a binding is looked up by, claimed_twice
// two bindings on one account, so each is a binding's whatever the mirror
// holds (the mirror may already have cleared the row, db.RecordTargetBinding,
// when another person's account took the number). Only name_held is also a
// create's, and for it the mirror answers; a subject it lacks, or cannot be
// read for, the next sweep raises from /subjects if it is a binding's.
func bindingsConflict(ctx context.Context, target, subject string, c *addons.BindingConflict) bool {
	if c.Reason != addons.ConflictNameHeld {
		return true
	}
	_, bound, err := getBinding(ctx, target, subject)
	if err != nil {
		log.Printf("[ADDON-DRAIN] could not read %s's binding on %s; the next sweep raises its conflict: %v",
			subject, target, err)
	}
	return err == nil && bound
}

// fingerprintFor decides what this row verifies against.
//
// An operator-approved row carries the fingerprint of the state the operator
// reviewed, and the add-on refuses if the subject moved since. That is the whole
// guarantee: the diff you approved is the diff that lands.
//
// A system-initiated row has no such review behind it. Its "plan" was minted by
// a cascade at the moment a role changed, and the target may legitimately have
// moved between then and the drain — which is not an operator's mistake and not
// a diff anybody agreed to. Carrying the trigger-time fingerprint would make an
// ordinary access change fail terminally because somebody edited an account on
// the NAS in the meantime, and a member's account creation would stall until a
// human noticed. So it re-reads and converges against what is there now.
//
// The re-read is a `/plan` for one subject: the same call the rehearsal makes,
// so the fingerprint is produced by the same code that verifies it. A value the
// two sides computed differently verifies nothing.
func (res *DrainResult) fingerprintFor(ctx context.Context, intent db.EntitlementIntent) (string, bool) {
	if intent.Surface != db.SystemConvergenceSurface {
		return intent.Fingerprint, false
	}

	answer := planOne(ctx, intent.Target, []addons.PlanSubject{{
		Subject: intent.SubjectID,
		Email:   subjectEmail(ctx, intent.SubjectID),
		Desired: intent.Desired(),
	}})
	if answer.Outcome != addons.OutcomeSucceeded || len(answer.Outcomes) != 1 {
		// No fresh fingerprint and no licence to send the stale one: the apply
		// would be verified against a moment nobody chose. Left claimable and
		// counted as errored, which is what "we could not establish the state"
		// means — the row is still queued and the next pass tries again.
		res.persistErr(intent.OutboxID, "re-read state", fmt.Errorf("the target could not be re-read before dispatch"))
		return "", true
	}
	// A 200 is not the same as a usable answer, and reading only the outcome is
	// how the two got conflated. The add-on answers from its mirror when the
	// target is unreachable and says so in `current`; it caps a large read and
	// says so in `truncated`. A fingerprint from either describes something
	// other than the target's live state, and dispatching against it converges
	// a subject onto a set computed from a read that could not see them.
	switch {
	case !answer.Current:
		res.persistErr(intent.OutboxID, "re-read state",
			fmt.Errorf("the add-on answered from its mirror; a provisional fingerprint cannot gate an apply"))
		return "", true
	case answer.Truncated:
		res.persistErr(intent.OutboxID, "re-read state",
			fmt.Errorf("the re-read hit the target's cap; an absence it reports may be the cap rather than the target"))
		return "", true
	case answer.Outcomes[0].Effect == db.PlanEffectBlocked:
		// The plan itself says this subject cannot be applied — a name held by
		// an account nobody has bound, most often. Dispatching it anyway is the
		// one case where the apply might succeed and be wrong: it would write
		// into somebody else's account. Terminal, because no retry resolves a
		// conflict; an operator does.
		out := answer.Outcomes[0]
		settle := markTargetRefused
		if raiseConflict(ctx, intent.Target, intent.SubjectID, out.Conflict) {
			settle = markBindingConflict
		}
		if err := settle(ctx, intent.OutboxID, addonWords(out.Detail, out.Consequence)); err != nil {
			res.settleFailure(intent.OutboxID, "mark failed", err)
		} else {
			res.Failed++
		}
		return "", true
	}
	return answer.Outcomes[0].Fingerprint, false
}

// recordBinding copies what the add-on reported into the backend's own record.
//
// Downstream of the add-on's answer, never a guess: the apply resolves a subject
// to an account through the add-on's store, and this follows that decision. Two
// stores deciding it would be two answers to the one question where being wrong
// hands somebody else's home directory to a member.
//
// Non-fatal, and logged rather than returned. The convergence happened; refusing
// to record it as applied because a mirror write failed would leave the row
// claimable and re-drive a mutation that already landed.
func recordBinding(ctx context.Context, intent db.EntitlementIntent, resp addons.ApplyResponse, actor string) {
	if resp.Username == "" {
		// An outcome that reported no account name. Nothing to record, and
		// nothing wrong: a `no_change` on a subject the add-on could not name is
		// already an outcome the surface shows.
		return
	}
	binding := db.TargetBinding{
		Target: intent.Target, SubjectID: intent.SubjectID,
		Username: resp.Username, BoundBy: actor,
	}
	if resp.AccountID != "" {
		id := resp.AccountID
		binding.AccountID = &id
	}
	if err := saveBinding(ctx, binding); err != nil {
		log.Printf("[ADDON-DRAIN] converged %s on %s but could not record the binding: %v",
			intent.SubjectID, intent.Target, err)
	}
}

// recordMergeBase stores what the target reported after this write.
//
// The merge base is what lets the next reconciliation say WHO changed a value.
// Without it every difference is a two-way diff, which produces no conflict —
// only a winner, always Syndra, so a hand edit on the target is silently
// reverted.
//
// Two things it must never do, and both are refusals rather than conventions:
//
// It records nothing when the add-on reported no observation. `unverified`
// means the write landed and could not be read back, and `observed` is then
// absent by construction — recording anything for that subject would mean
// recording what Syndra ASKED for, which is the exact failure the base exists to
// prevent, arriving through the error path. The subject stays baseless, which
// the classifier already handles as "no cause can be determined".
//
// And it is non-fatal. The convergence happened; failing to write the base does
// not un-apply it, and settling the row as failed would re-drive a mutation that
// already landed. What a missing base costs is one pass of attribution, which
// the next successful apply restores.
func recordMergeBase(ctx context.Context, intent db.EntitlementIntent, resp addons.ApplyResponse) {
	if resp.Unverified || len(resp.Observed) == 0 {
		// Logged, because a target that never reports observations leaves every
		// subject baseless and every difference unattributable — a silent
		// degradation of the whole mechanism, visible nowhere else.
		log.Printf("[ADDON-DRAIN] %s on %s reported no observed state (unverified=%t); no merge base recorded",
			intent.SubjectID, intent.Target, resp.Unverified)
		return
	}
	if err := saveMergeBase(ctx, db.MergeBase{
		Target: intent.Target, SubjectID: intent.SubjectID, Base: resp.Observed,
		Partial: intent.WithdrawsOnly,
	}); err != nil {
		log.Printf("[ADDON-DRAIN] converged %s on %s and could not record its merge base: %v (non-fatal)",
			intent.SubjectID, intent.Target, err)
	}
}
