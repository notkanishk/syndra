package propagation

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"

	"syndra/internal/db"
	"syndra/internal/models"
)

// DrainResult summarizes one operator-triggered drain pass.
type DrainResult struct {
	Applied  int `json:"applied"`
	Failed   int `json:"failed"`
	Requeued int `json:"requeued"`
	// Abandoned counts rows that stopped being this drain's to settle while
	// their dispatch was out — today, because the target was deregistered
	// underneath them. Its own counter because it is neither of the others: the
	// call may well have reached the target, so it is not a failure, and
	// nothing confirmed it, so it is certainly not a success.
	Abandoned int `json:"abandoned"`
	// Awaiting names active targets holding unresolved rows this pass did not
	// dispatch, because a drain carries one target's dispatcher.
	Awaiting []string `json:"awaiting,omitempty"`
	// Errored counts rows whose Zitadel outcome was decided but whose state could
	// NOT be persisted (mark-applied/failed, requeue, or ledger reconcile failed).
	// Such a row is left non-terminal (in_flight) and is neither applied nor
	// failed for this pass — the next drain reclaims and re-drives it.
	Errored int `json:"errored"`
	// Exhausted counts rows this pass made terminal because their retry budget
	// ran out. A subset of Failed, reported apart from it because the reason
	// differs in what an operator does next: an ordinary failure names what the
	// target said, and this one says nobody will try again.
	Exhausted int    `json:"exhausted,omitempty"`
	Halted    bool   `json:"halted"`
	Reason    string `json:"reason,omitempty"`
	// HaltedTarget names whose pass produced Reason. Its own field rather than a
	// prefix on Reason: the reason strings are matched by callers and read by
	// operators, and folding two facts into one string makes both harder to use.
	HaltedTarget string `json:"halted_target,omitempty"`
	// Passes reports what each target's own dispatcher did, because the summed
	// counters above cannot say WHICH target halted — and that is the whole of
	// what an operator does next. A drain that reported `halted: zitadel_offline`
	// while the NAS pass applied nine rows was telling them their change had not
	// gone through when it had.
	Passes []PassResult `json:"passes,omitempty"`
}

// PassResult is one target's leg of a drain.
//
// A separate type rather than a nested DrainResult: a pass has no passes of its
// own, and a shape that allows one invites a reader to look for a second level
// that never exists.
type PassResult struct {
	Target    string `json:"target"`
	Applied   int    `json:"applied"`
	Failed    int    `json:"failed"`
	Requeued  int    `json:"requeued"`
	Abandoned int    `json:"abandoned"`
	Errored   int    `json:"errored"`
	Exhausted int    `json:"exhausted,omitempty"`
	Halted    bool   `json:"halted"`
	Reason    string `json:"reason,omitempty"`
}

// merge folds one target's pass into the combined result.
//
// The combined `halted` is true when ANY pass halted, and the combined reason
// names that pass. Both are deliberately pessimistic: an operator who is told
// nothing halted must be able to believe it.
func (r *DrainResult) merge(target string, p DrainResult) {
	r.Applied += p.Applied
	r.Failed += p.Failed
	r.Requeued += p.Requeued
	r.Abandoned += p.Abandoned
	r.Errored += p.Errored
	r.Exhausted += p.Exhausted
	r.Passes = append(r.Passes, PassResult{
		Target: target, Applied: p.Applied, Failed: p.Failed, Requeued: p.Requeued,
		Abandoned: p.Abandoned, Errored: p.Errored, Exhausted: p.Exhausted,
		Halted: p.Halted, Reason: p.Reason,
	})
	if p.Halted && !r.Halted {
		r.Halted, r.Reason, r.HaltedTarget = true, p.Reason, target
	}
}

// persistErr records a row whose Zitadel outcome was decided but whose state
// could not be persisted. The row stays in_flight; the next drain reclaims it
// (ClaimPendingPropagations covers in_flight) and the idempotent already-exists
// check resolves it. Never counted as applied/failed/requeued — the summary must
// not claim success the database did not record.
func (r *DrainResult) persistErr(id, step string, err error) {
	r.Errored++
	log.Printf("[PROPAGATION] %s failed for row=%s: %v (left in_flight; next drain reclaims)", step, id, err)
}

const claimBatch = 100

// Drain processes pending outbox rows in created_at order, for EVERY
// registered target. Operator-triggered.
//
// One drain, several dispatchers. Zitadel's pass speaks the Management API and
// an add-on's speaks that add-on's contract, so the passes are separate code —
// but they are not separate operator actions. "Resume now" that resumed one
// target and silently left another's approved work queued is the defect this
// shape exists to prevent, and it was a real one: nothing called the add-on
// dispatcher at all, so an approved entitlement change queued forever with no
// error anywhere.
//
// The passes are independent in failure as well as in code. A Zitadel outage
// halts Zitadel's pass and no other — they are separate deployments with
// separate outages, and holding a reachable NAS's work behind an unreachable
// identity provider is the coupling the target column was introduced to remove.
//
// Within a pass: `applied` (synchronous 2xx, or idempotent 409) is terminal
// success — there is no webhook round-trip. A 4xx (non-429/408) fails its row
// without halting the pass; a transient error (5xx/timeout/429/408) requeues. A
// pass halts only when its target is unreachable up front, or a row exceeds the
// retry budget.
func Drain(ctx context.Context) (DrainResult, error) {
	// Serialize drains: a session-level advisory lock ensures only one drain runs
	// at a time, so the in_flight reclaim (ClaimPendingPropagations) can never
	// steal a row a concurrent drain is mid-dispatch on — only crash-orphaned
	// in_flight rows (whose drain session is gone) are ever reclaimed.
	//
	// Taken ONCE for all passes rather than per pass. Per pass, a second drain
	// could interleave between two targets and reclaim rows this one is holding.
	release, acquired, err := acquireDrainLock(ctx)
	if err != nil {
		return DrainResult{}, fmt.Errorf("acquire drain lock: %w", err)
	}
	if !acquired {
		return DrainResult{Halted: true, Reason: "drain_in_progress"}, nil
	}
	defer release()

	var res DrainResult
	zitadel, err := drainZitadelPass(ctx)
	if err != nil {
		return DrainResult{}, err
	}
	res.merge(db.TargetZitadel, zitadel)

	for _, target := range addonDrainTargets() {
		pass, err := drainAddonPass(ctx, target)
		if err != nil {
			// One target's failure must not abandon the rest: the remaining
			// passes are for other deployments, and returning here would leave
			// their approved work queued because a different NAS is broken.
			log.Printf("[ADDON-DRAIN] %s: %v", target, err)
			res.merge(target, DrainResult{Halted: true, Reason: "pass_error"})
			continue
		}
		res.merge(target, pass)
	}

	// Named after every pass has run, so it lists what is genuinely still
	// waiting rather than what this drain was about to pick up.
	if waiting, err := awaitingDispatch(ctx, ""); err != nil {
		log.Printf("[PROPAGATION] could not list targets awaiting dispatch: %v (non-fatal)", err)
	} else {
		res.Awaiting = waiting
	}

	pruneAfterDrain(ctx)
	return res, nil
}

// addonDrainTargets is the registered add-on targets, in a stable order.
//
// Sorted, because an operator reading two consecutive drains should not have to
// work out whether a reordered list means something changed.
func addonDrainTargets() []string {
	regs := registeredAddons()
	out := make([]string, 0, len(regs))
	for _, reg := range regs {
		if reg.Target == db.TargetZitadel {
			continue
		}
		out = append(out, reg.Target)
	}
	sort.Strings(out)
	return out
}

// drainZitadelPass is the Management API leg. Caller holds the drain lock.
func drainZitadelPass(ctx context.Context) (DrainResult, error) {
	if !zitadelReachable(ctx) {
		return DrainResult{Halted: true, Reason: "zitadel_offline"}, nil
	}
	rows, err := claimPending(ctx, db.TargetZitadel, claimBatch)
	if err != nil {
		return DrainResult{}, fmt.Errorf("claim pending: %w", err)
	}
	var res DrainResult
	for _, row := range rows {
		if halt := res.processRow(ctx, row); halt {
			return res, nil
		}
	}
	return res, nil
}

// pruneAfterDrain is the retention work, run once per drain rather than once
// per pass: it is global, and repeating it per target would do the same scan
// once for every registered add-on.
func pruneAfterDrain(ctx context.Context) {
	// Opportunistic retention prune (design §3.1). Non-fatal; canonical intent
	// lives in direct_role_grants, so a failed prune never loses real data.
	if n, err := pruneTerminal(ctx, retentionDays); err != nil {
		log.Printf("[PROPAGATION] retention prune failed: %v (non-fatal)", err)
	} else if n > 0 {
		log.Printf("[PROPAGATION] pruned %d terminal outbox rows older than %dd", n, retentionDays)
	}
	// And the approvals behind them. Plans accumulate one per rehearsal — an
	// operator may rehearse ten times and apply once — and until this existed
	// nothing removed any of them. Same retention, same non-fatal handling, and
	// deliberately after the outbox prune: a plan still cited by a queued row is
	// refused by the foreign key, and pruning the rows first is what makes the
	// spent ones prunable.
	if n, err := prunePlans(ctx, retentionDays); err != nil {
		log.Printf("[PROPAGATION] plan retention prune failed: %v (non-fatal)", err)
	} else if n > 0 {
		log.Printf("[PROPAGATION] pruned %d spent plans older than %dd", n, retentionDays)
	}
}

// DrainOne is the targeted counterpart to Drain: it processes ONLY the outbox row
// with the given id, not the global oldest-first batch. This is the inline "apply
// now" path (operator point grant with ?apply=true, access-request approval) —
// applying one mutation must never project unrelated mutations an operator left
// queued. It shares Drain's serialization (advisory lock), reachability
// pre-flight, and per-row processing; a row that is already terminal or gone is a
// no-op (found=false).
func DrainOne(ctx context.Context, outboxID string) (DrainResult, error) {
	release, acquired, err := acquireDrainLock(ctx)
	if err != nil {
		return DrainResult{}, fmt.Errorf("acquire drain lock: %w", err)
	}
	if !acquired {
		return DrainResult{Halted: true, Reason: "drain_in_progress"}, nil
	}
	defer release()

	if !zitadelReachable(ctx) {
		return DrainResult{Halted: true, Reason: "zitadel_offline"}, nil
	}
	row, found, err := claimOne(ctx, db.TargetZitadel, outboxID)
	if err != nil {
		return DrainResult{}, fmt.Errorf("claim propagation %s: %w", outboxID, err)
	}
	var res DrainResult
	if !found {
		// Already terminal, gone, on a deregistered target, or on a target this
		// drain cannot dispatch. Nothing was claimed, so nothing has to be put
		// back — which is the point of scoping the claim rather than claiming
		// first and releasing after. Only the last of those is worth saying.
		res.Awaiting = undispatchableTargets(ctx, outboxID)
		return res, nil
	}
	res.processRow(ctx, *row)
	return res, nil
}

// processRow processes one already-claimed (in_flight) row, updating res. It
// returns true when the drain must halt (retry budget exceeded). Shared by the
// batch Drain and the targeted DrainOne so both classify, apply, and record
// identically. A state-write failure is recorded via persistErr (never counted
// as success) and leaves the row in_flight for a later reclaim.
// abandoned reports whether the row stopped being this drain's to settle, and
// records it if so. A settle that finds no in-flight row is not a persistence
// failure to retry — the row reached a terminal state legitimately, under us —
// but it is not the outcome the drain was about to claim either.
func (res *DrainResult) abandoned(id, step string, err error) bool {
	if !errors.Is(err, db.ErrPropagationNotInFlight) {
		return false
	}
	log.Printf("[DRAIN] outbox=%s %s: the row was terminated while its dispatch was out; leaving it terminal", id, step)
	res.Abandoned++
	return true
}

// undispatchableTargets names the target of a row a claim declined because this
// drain does not dispatch it. Diagnostic only: a failure to explain must not
// become a failure to drain.
func undispatchableTargets(ctx context.Context, id string) []string {
	target, err := undispatchable(ctx, db.TargetZitadel, id)
	if err != nil {
		log.Printf("[PROPAGATION] could not identify the target of row=%s: %v (non-fatal)", id, err)
		return nil
	}
	if target == "" {
		return nil
	}
	return []string{target}
}

func (res *DrainResult) processRow(ctx context.Context, row models.PendingPropagation) (halt bool) {
	if exists, _ := alreadyExists(ctx, row); exists {
		// Already in the desired Zitadel state — but a revoke/replace still owes
		// the ledger a cleanup, so route through the same apply path.
		if err := applyRow(ctx, row); err != nil {
			if !res.abandoned(row.ID, "apply (short-circuit)", err) {
				res.persistErr(row.ID, "apply (short-circuit)", err)
			}
			return false
		}
		res.Applied++
		return false
	}
	class, errMsg := classifyDispatch(ctx, row)
	switch class {
	case ackApplied:
		if err := applyRow(ctx, row); err != nil {
			if !res.abandoned(row.ID, "apply", err) {
				res.persistErr(row.ID, "apply", err)
			}
			return false
		}
		res.Applied++
	case ackFailed:
		if err := markFailed(ctx, row.ID, errMsg); err != nil {
			if !res.abandoned(row.ID, "mark failed", err) {
				res.persistErr(row.ID, "mark failed", err)
			}
			return false
		}
		res.Failed++
	case ackTransient:
		if row.Attempts >= maxRetries {
			// The budget is spent. TERMINAL, and the pass continues.
			//
			// Halting without terminating was a poison pill. The row goes back
			// to pending, the claim orders it first, and every pass re-claims
			// it, requeues it and halts in the same place — so everything
			// behind it never drains, silently and for ever. In the operator
			// drain that is a visible stall; in the background revocation
			// runner it is retained access nobody is told about, which is the
			// one case §7 built the runner to prevent.
			//
			// Escalation (task 2.51) is where this row goes next. Until that
			// exists it is a failed row with a reason, which an operator can
			// see and re-enqueue — unlike a row that quietly blocks a queue.
			reason := fmt.Sprintf("out of retries after %d attempts: %s", row.Attempts, errMsg)
			if err := markFailed(ctx, row.ID, reason); err != nil {
				if !res.abandoned(row.ID, "mark failed", err) {
					res.persistErr(row.ID, "mark failed", err)
				}
				return false
			}
			res.Failed++
			res.Exhausted++
			return false
		}
		if _, err := requeue(ctx, row.ID, errMsg); err != nil {
			// The requeue is the dangerous one: unguarded it would return an
			// abandoned row to `pending` on a deregistered target, recreating
			// the undrainable row the deregistration had just resolved.
			if !res.abandoned(row.ID, "requeue", err) {
				res.persistErr(row.ID, "requeue", err)
			}
			return false
		}
		res.Requeued++
	}
	return false
}

// applyRow finalizes a row that reached (or already sat in) the desired Zitadel
// state: it reconciles the intent ledger first, then marks the outbox row
// applied. Ordering matters — the ledger delete runs BEFORE the terminal
// markApplied, so if the delete fails the outbox row stays in_flight and the
// next drain reclaims it and retries, rather than being stranded terminal with a
// stale ledger. add is a ledger no-op; only revoke/replace prune rows.
func applyRow(ctx context.Context, row models.PendingPropagation) error {
	if row.OpType == "revoke" || row.OpType == "replace" {
		if err := reconcileLedger(ctx, row.ID); err != nil {
			return err
		}
	}
	// BEFORE the terminal markApplied, and its error is returned — the same
	// ordering, and the same reason, as the ledger reconcile above: a row whose
	// durable side-effects did not all land must stay in_flight and be reclaimed
	// rather than be stranded terminal with a store that disagrees with it.
	if err := rememberPropagation(ctx, row); err != nil {
		return err
	}
	return markApplied(ctx, row.ID)
}

// rememberPropagation records that the target ACCEPTED this write.
//
// The strongest evidence Syndra ever has about a target was being discarded:
// the outbox has no `confirmed` state by design and its terminal rows are
// pruned, and the grant index is deleted by the very event that removes a grant.
// So a grant applied and removed between two sweeps had nothing behind it, and
// the next reconciliation read its absence as a write that never landed — and
// replayed it, restoring access somebody had removed on purpose.
//
// Only what ADDS access. A revoke landing is Syndra removing something, and
// remembering that as "this was applied" would make the next pass argue that the
// target should still hold it.
//
// The two halves fail differently, and the asymmetry is the whole of this
// function's error handling.
//
// FAILING TO RECORD a landing is non-fatal. The write happened either way, and
// the missing memory costs one pass of attribution: a later absence reads as a
// write that never landed, which is the conservative reading — the change is
// replayed rather than reported, and nobody's decision is quietly reverted.
//
// FAILING TO REMOVE one is not. Stale memory does not merely say less, it says
// the OPPOSITE of what is true: the target was holding this. Grant the role
// again, have the new write fail to reach the target, and the pass reads that
// stale evidence as somebody's hand removal and suppresses the replay that would
// have restored the access. So the error is returned, the outbox row stays
// in_flight, and the next drain reclaims it — safe, because the revoke it
// re-attempts is idempotent and the deletion it retries is too.
func rememberPropagation(ctx context.Context, row models.PendingPropagation) error {
	switch row.OpType {
	case "revoke":
		// A removal Syndra made FORGETS what it removed, and this is not
		// bookkeeping tidiness. The memory exists so a later absence reads as
		// somebody's removal rather than as a write that never landed; kept past
		// Syndra's own revoke, it becomes a lie about the future. Grant the role
		// again, have that new write fail to reach the target, and the stale
		// memory says the target was holding it — so the pass reports a hand
		// removal and suppresses the replay that would have restored the access
		// somebody just asked for.
		fields := make([]string, 0, len(row.RoleKeys))
		for _, role := range row.RoleKeys {
			fields = append(fields, row.ProjectID+"/"+role)
		}
		if err := forgetPropagatedFields(ctx, row.Target, row.UserID, fields); err != nil {
			return fmt.Errorf("revoke %s landed and its memory could not be cleared: %w", row.ID, err)
		}
		return nil
	case "add", "replace":
	default:
		return nil
	}

	for _, role := range row.RoleKeys {
		if err := savePropagation(ctx, db.Propagation{
			Target: row.Target, SubjectID: row.UserID,
			Field:    row.ProjectID + "/" + role,
			OutboxID: row.ID, Actor: row.InitiatedBy,
		}); err != nil {
			log.Printf("[DRAIN] %s landed and could not be remembered: %v (non-fatal)", row.ID, err)
		}
	}

	if row.OpType == "replace" {
		// A replace states the WHOLE desired set for one grant, so anything
		// remembered under that project and absent from it was just removed.
		// Same reasoning as the revoke above, reached by the operation that
		// removes without saying so.
		keep := make([]string, 0, len(row.RoleKeys))
		for _, role := range row.RoleKeys {
			keep = append(keep, row.ProjectID+"/"+role)
		}
		if err := forgetPropagatedFieldsExcept(ctx, row.Target, row.UserID, row.ProjectID, keep); err != nil {
			return fmt.Errorf("replace %s landed and stale memory could not be pruned: %w", row.ID, err)
		}
	}
	return nil
}

// alreadyExists is a latency optimization, NOT a correctness gate — Zitadel's
// 409 AlreadyExists (classified ackApplied) is the real safety net, so a false
// "no" here is harmless (we call, Zitadel absorbs the dup idempotently). It uses
// the webhook index first; on any miss it does ONE live grant list per row.
func alreadyExists(ctx context.Context, row models.PendingPropagation) (bool, error) {
	switch row.OpType {
	case "add":
		// add only needs the desired roles PRESENT (superset is fine): the index
		// can confirm presence, and 409 absorbs any dup if the index is stale.
		allIndexed := true
		for _, role := range row.RoleKeys {
			if ok, err := grantIndexHasRole(ctx, row.UserID, row.ProjectID, role); err != nil || !ok {
				allIndexed = false
				break
			}
		}
		if allIndexed {
			return true, nil // index covers every role; skip the API call
		}
		live, err := liveUserGrantRoles(ctx, row.UserID, row.ProjectID) // one list, not per-role
		if err != nil {
			return false, nil // can't confirm → proceed; 409 absorbs any dup
		}
		for _, role := range row.RoleKeys {
			if !live[role] {
				return false, nil
			}
		}
		return true, nil
	case "replace":
		// replace targets an EXACT role set, so a superset must NOT short-circuit:
		// a superseded role still present in Zitadel means UpdateUserGrant must run
		// to remove it. The presence-only grant index cannot prove the absence of
		// extras, so replace always uses the live list and requires an exact match.
		live, err := liveUserGrantRoles(ctx, row.UserID, row.ProjectID)
		if err != nil {
			return false, nil // can't confirm → proceed; UpdateUserGrant sets exact state
		}
		if len(live) != len(row.RoleKeys) {
			return false, nil // extra or missing role → not yet in desired state
		}
		for _, role := range row.RoleKeys {
			if !live[role] {
				return false, nil
			}
		}
		return true, nil
	case "revoke":
		live, err := liveUserGrantRoles(ctx, row.UserID, row.ProjectID)
		if err != nil {
			return false, nil // can't confirm absence → let the revoke run
		}
		for _, role := range row.RoleKeys {
			if live[role] {
				return false, nil // still present → revoke must run
			}
		}
		return true, nil // nothing left to revoke → already in desired state
	}
	return false, nil
}

// classifyDispatch issues the Zitadel call and classifies the result, returning
// the ACK class and the error message to record (empty on success).
func classifyDispatch(ctx context.Context, row models.PendingPropagation) (ackClass, string) {
	var err error
	switch row.OpType {
	case "add":
		err = zitadelAddUserGrant(ctx, row.UserID, row.ProjectID, row.RoleKeys)
	case "replace":
		err = zitadelUpdateUserGrant(ctx, row.UserID, row.ZitadelGrantID, row.RoleKeys)
	case "revoke":
		// A Zitadel grant is ONE aggregate per (user, project) holding ALL of that
		// project's roles in role_keys[] — but the cascade enqueues PER-ROLE
		// revokes (row.RoleKeys is typically one role). Removing the whole grant
		// unconditionally would silently strip the user's OTHER roles on this
		// project that came from a different bundle/rule/operator grant. Mirror
		// zitadel/orchestrator.go's sole-vs-multi logic: read the grant's live
		// roles, subtract what this row revokes, and only remove the whole grant
		// when nothing survives.
		live, liveErr := liveUserGrantRoles(ctx, row.UserID, row.ProjectID)
		if liveErr != nil {
			// Can't confirm what would survive. Removing/updating blind here is
			// the exact data-loss path this fix exists to close, so the
			// least-destructive choice is never to guess and never to fall
			// through to an unconditional Remove.
			//
			// Classified like every other Zitadel error rather than assumed
			// transient: a user deleted in Zitadel answers 404 for ever, and
			// retrying that until the budget runs out is how the revocation
			// queue acquires a row that can never settle.
			return classifyZitadelError(liveErr), fmt.Sprintf("revoke: could not read live grant roles: %v", liveErr)
		}
		revoked := make(map[string]bool, len(row.RoleKeys))
		for _, rk := range row.RoleKeys {
			revoked[rk] = true
		}
		remaining := make([]string, 0, len(live))
		for rk := range live {
			if !revoked[rk] {
				remaining = append(remaining, rk)
			}
		}
		if len(remaining) == 0 {
			err = zitadelRemoveUserGrant(ctx, row.UserID, row.ZitadelGrantID)
		} else {
			err = zitadelUpdateUserGrant(ctx, row.UserID, row.ZitadelGrantID, remaining)
		}
	default:
		log.Printf("[PROPAGATION] unknown op_type=%s row=%s", row.OpType, row.ID)
		return ackFailed, fmt.Sprintf("unknown op_type %q", row.OpType)
	}
	if err == nil {
		return ackApplied, ""
	}
	return classifyZitadelError(err), err.Error()
}
