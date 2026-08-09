package propagation

import (
	"context"
	"errors"
	"fmt"
	"log"

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
	Errored int    `json:"errored"`
	Halted  bool   `json:"halted"`
	Reason  string `json:"reason,omitempty"`
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

// Drain processes pending outbox rows in created_at order. Operator-triggered.
// `applied` (synchronous 2xx, or idempotent 409) is terminal success — there is
// no webhook round-trip. A 4xx (non-429/408) fails its row without halting the
// batch; a transient error (5xx/timeout/429/408) requeues. The whole drain
// halts only when Zitadel is unreachable up front, or a row exceeds the retry
// budget.
func Drain(ctx context.Context) (DrainResult, error) {
	// Serialize drains: a session-level advisory lock ensures only one drain runs
	// at a time, so the in_flight reclaim (ClaimPendingPropagations) can never
	// steal a row a concurrent drain is mid-dispatch on — only crash-orphaned
	// in_flight rows (whose drain session is gone) are ever reclaimed.
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
	// One target per pass, and this pass is Zitadel's: the dispatcher below
	// speaks the Management API and nothing else. Add-on targets have their own
	// dispatcher and their own pass (group 4); until that exists their rows are
	// left alone rather than pushed through machinery shaped for a system they
	// are not for.
	rows, err := claimPending(ctx, db.TargetZitadel, claimBatch)
	if err != nil {
		return DrainResult{}, fmt.Errorf("claim pending: %w", err)
	}
	var res DrainResult
	// Said out loud, because a pass that drained one target while another's
	// work waits looks exactly like a pass with nothing left to do.
	if waiting, err := awaitingDispatch(ctx, db.TargetZitadel); err != nil {
		log.Printf("[PROPAGATION] could not list targets awaiting dispatch: %v (non-fatal)", err)
	} else {
		res.Awaiting = waiting
	}
	for _, row := range rows {
		if halt := res.processRow(ctx, row); halt {
			return res, nil
		}
	}
	// Opportunistic retention prune (design §3.1). Non-fatal; canonical intent
	// lives in direct_role_grants, so a failed prune never loses real data.
	if n, err := pruneTerminal(ctx, retentionDays); err != nil {
		log.Printf("[PROPAGATION] retention prune failed: %v (non-fatal)", err)
	} else if n > 0 {
		log.Printf("[PROPAGATION] pruned %d terminal outbox rows older than %dd", n, retentionDays)
	}
	return res, nil
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
	row, found, err := claimOne(ctx, outboxID)
	if err != nil {
		return DrainResult{}, fmt.Errorf("claim propagation %s: %w", outboxID, err)
	}
	var res DrainResult
	if !found {
		return res, nil // already terminal, gone, or on a target no longer active
	}
	if row.Target != db.TargetZitadel {
		// Claimed but not dispatchable here. Put it back rather than pushing it
		// through the Zitadel path, and say so: leaving it in_flight would make
		// it look like a dispatch nobody can account for.
		if _, err := requeue(ctx, row.ID, "no dispatcher for target "+row.Target); err != nil && !errors.Is(err, db.ErrPropagationNotInFlight) {
			return res, fmt.Errorf("release %s: %w", row.ID, err)
		}
		res.Awaiting = []string{row.Target}
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
		attempts, err := requeue(ctx, row.ID, errMsg)
		if err != nil {
			// The requeue is the dangerous one: unguarded it would return an
			// abandoned row to `pending` on a deregistered target, recreating
			// the undrainable row the deregistration had just resolved.
			if !res.abandoned(row.ID, "requeue", err) {
				res.persistErr(row.ID, "requeue", err)
			}
			return false
		}
		res.Requeued++
		if attempts > maxRetries {
			res.Halted = true
			res.Reason = "max_retries_exceeded"
			return true
		}
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
		if err := reconcileLedger(ctx, row.OpType, row.UserID, row.ProjectID, row.RoleKeys, row.Source); err != nil {
			return err
		}
	}
	return markApplied(ctx, row.ID)
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
			// least-destructive choice is to treat this as transient and let the
			// next drain retry once the live list is readable again — never
			// guess and never fall through to an unconditional Remove.
			return ackTransient, fmt.Sprintf("revoke: could not read live grant roles: %v", liveErr)
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
