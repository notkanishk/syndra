package propagation

import (
	"context"
	"errors"
	"fmt"
	"log"

	"syndra/internal/addons"
	"syndra/internal/db"
	"syndra/internal/models"
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

// DrainAddon dispatches one add-on target's queued entitlement work.
//
// Operator-triggered like the Zitadel pass, and for the same reason: these rows
// confer access, and the drain rule narrows to grants rather than disappearing.
// Revocation-shaped add-on work is not separated out yet — an `apply` carries a
// resolved set rather than a direction, so "does this withdraw access" is a
// question only a diff against the last applied snapshot can answer.
//
// ponytail: until that diff exists, add-on rows wait for an operator in both
// directions. The conservative half of the rule, and stated rather than
// silently assumed.
func DrainAddon(ctx context.Context, target string) (DrainResult, error) {
	if target == db.TargetZitadel {
		// Refused rather than ignored. The Zitadel pass exists and is not this
		// one; a caller that reached here with the built-in target has a wrong
		// model of which dispatcher does what.
		return DrainResult{}, fmt.Errorf("drain add-on: %s is the built-in target and has its own dispatcher", target)
	}

	release, acquired, err := acquireDrainLock(ctx)
	if err != nil {
		return DrainResult{}, fmt.Errorf("acquire drain lock: %w", err)
	}
	if !acquired {
		return DrainResult{Halted: true, Reason: "drain_in_progress"}, nil
	}
	defer release()

	// One probe, before any row is claimed, exactly as the Zitadel passes do.
	// A batch dispatched into an outage spends one retry per row to learn what
	// a single call establishes — and an exhausted budget is terminal, so the
	// rows an operator approved would be failed by the target being switched
	// off rather than left queued for it coming back.
	if !addonReachable(ctx, target) {
		return DrainResult{Halted: true, Reason: "target_unreachable"}, nil
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

	resp := applyEntitlement(ctx, addons.ApplyRequest{
		Target:      intent.Target,
		Subject:     intent.SubjectID,
		Fingerprint: intent.Fingerprint,
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
		if err := release(ctx, row.ID, "the target is not accepting mutations"); err != nil {
			res.settleFailure(row.ID, "release", err)
			return false
		}
		res.Requeued++

	case resp.Outcome == addons.OutcomeRejected:
		// The add-on validated the call and refused; it did not act. Terminal,
		// because retrying a deterministic refusal changes nothing.
		if err := markFailed(ctx, row.ID, refusalReason(resp)); err != nil {
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
			reason := fmt.Sprintf("out of retries after %d attempts: the target could not be reached", row.Attempts)
			if err := markFailed(ctx, row.ID, reason); err != nil {
				res.settleFailure(row.ID, "mark failed", err)
				return false
			}
			res.Failed++
			res.Exhausted++
			return false
		}
		if _, err := requeue(ctx, row.ID, "the target could not be reached"); err != nil {
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

// refusalReason is the add-on's own answer, never its response body.
//
// The body is whatever the least trusted component chose to send back, and this
// string lands in `last_error`, which an operator reads and a surface renders.
func refusalReason(resp addons.ApplyResponse) string {
	if resp.Code == addons.CodePlanStale {
		// Named, because it is the one refusal whose next step is a re-plan
		// rather than a fix. §8's proper answer here is a distinct stale-plan
		// outcome that names the subjects that moved; the plan is already spent
		// by the time this row is dispatched, so what the drain can do is say
		// which of the two things happened in a form a surface can match on.
		return "PLAN_STALE: the subject moved on the target since the plan was approved; re-plan and re-approve"
	}
	if resp.Detail != "" {
		return "the target refused it: " + resp.Detail
	}
	return fmt.Sprintf("the target refused it (status %d)", resp.Status)
}
