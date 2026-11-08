package propagation

import (
	"context"
	"fmt"
	"log"

	"syndra/internal/db"
	"syndra/internal/models"
)

// DrainRevocations is the one background drain this system has. It drains
// revocations, and retries of changes already sent once that the target
// didn't take (ClaimDueRetries).
//
// Retries are not a widening of the consent rule below: a retried row is one
// the operator already sent, straight away or by Send, and it is tried again
// only after its backoff. A row that waits for the operator (its role waits)
// is never one of them (owner, 5 Nov 2026).
//
// The inherited rule is that buffered propagations drain solely on explicit
// operator action, and that rule exists to protect a consent property: nobody
// gains access without a human deciding to give it to them. A revocation confers
// nothing, so it is outside the property the rule protects — while a delayed
// revocation is retained access, which is the one case where waiting for someone
// to open the right page is the wrong dependency. So the rule narrows to grants
// and revocations get a runner (design §7).
//
// It shares the operator drain's advisory lock rather than taking one of its
// own: two drains dispatching the same subject's rows concurrently is the exact
// interleaving the lock exists to forbid, and a second lock would only serialise
// each drain against itself. Losing the lock is not an error and not a retry
// loop — the pass reports `drain_in_progress` and returns, and the runner's next
// tick tries again, so a busy operator drain slows this one instead of starving
// it and neither spins.
//
// It deliberately does not prune terminal rows or report the targets it did not
// dispatch. Both belong to a pass an operator is watching; this one runs
// unattended and says what it did in the log.
func DrainRevocations(ctx context.Context) (DrainResult, error) {
	release, acquired, err := acquireDrainLock(ctx)
	if err != nil {
		return DrainResult{}, fmt.Errorf("acquire drain lock: %w", err)
	}
	if !acquired {
		return DrainResult{Halted: true, Reason: "drain_in_progress"}, nil
	}
	defer release()

	var res DrainResult

	// Zitadel's leg. One probe, before any row is claimed: the retry budget
	// exists for real failures, and spending it on a target that is switched off
	// would exhaust a revocation's attempts on an outage and halt the runner
	// permanently on the row class where a permanent silent stop is worst.
	if !zitadelReachable(ctx) {
		res.merge(db.TargetZitadel, DrainResult{Halted: true, Reason: "zitadel_offline"})
	} else {
		// Only withdrawal rows, and only where dispatching one cannot invert its
		// subject's own intent order. Both conditions are in the claim, so this
		// runner cannot reach an access-conferring row even by mistake — a guard
		// here would be one a future caller of the claim could skip.
		rows, err := claimRevocations(ctx, db.TargetZitadel, claimBatch)
		if err != nil {
			return DrainResult{}, fmt.Errorf("claim revocations: %w", err)
		}
		retries, err := claimRetries(ctx, db.TargetZitadel, claimBatch)
		if err != nil {
			return DrainResult{}, fmt.Errorf("claim retries: %w", err)
		}
		rows = append(rows, retries...)
		var pass DrainResult
		for _, row := range rows {
			if halt := pass.automatic(ctx, row, pass.processRow); halt {
				break
			}
		}
		res.merge(db.TargetZitadel, pass)
	}

	// And every add-on's. A target revocation queues a lock, and a lock that
	// waits for somebody to open the right page is retained access — the same
	// reason this runner exists at all. The claim is the same one, so the same
	// two restrictions apply: withdrawal rows only, never ahead of older
	// conferring intent for the same subject.
	//
	// One target's outage does not stop the others. They are separate
	// deployments, and a NAS being off is not a reason to leave a door system's
	// revocation queued.
	for _, target := range addonDrainTargets() {
		if !addonReachable(ctx, target) {
			// A target with nothing of its own queued was not skipped:
			// unanswered says so, as the operator's pass does.
			res.merge(target, unanswered(ctx, target, "target_unreachable"))
			continue
		}
		rows, err := claimRevocations(ctx, target, claimBatch)
		if err == nil {
			var retries []models.PendingPropagation
			retries, err = claimRetries(ctx, target, claimBatch)
			rows = append(rows, retries...)
		}
		if err != nil {
			// Logged and skipped rather than returned: the Zitadel leg above has
			// already dispatched, and failing the pass would discard that report
			// for a claim error on an unrelated target.
			log.Printf("[REVOKE] %s: claim revocations: %v", target, err)
			res.merge(target, DrainResult{Halted: true, Reason: "claim_error"})
			continue
		}
		var pass DrainResult
		for _, row := range rows {
			if halt := pass.automatic(ctx, row, pass.dispatchEntitlement); halt {
				break
			}
		}
		res.merge(target, pass)
	}
	return res, nil
}

// automatic sends one row on the runner's own initiative. A row tried before
// (attempts > 0) is an automatic retry, and every one is audited with its try
// number and how it went, so the history never loses one.
func (res *DrainResult) automatic(ctx context.Context, row models.PendingPropagation, send func(context.Context, models.PendingPropagation) bool) bool {
	before := *res
	halt := send(ctx, row)
	if row.Attempts > 0 {
		auditRetry(ctx, row, retryOutcome(before, *res))
	}
	return halt
}
