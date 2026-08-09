package propagation

import (
	"context"
	"fmt"

	"syndra/internal/db"
)

// DrainRevocations is the one background drain this system has, and it drains
// revocations only.
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

	// One probe, before any row is claimed. The retry budget exists for real
	// failures; spending it on a target that is switched off would exhaust a
	// revocation's attempts on an outage and halt the runner permanently on the
	// row class where a permanent silent stop is worst.
	if !zitadelReachable(ctx) {
		return DrainResult{Halted: true, Reason: "zitadel_offline"}, nil
	}

	// Only withdrawal rows, and only where dispatching one cannot invert its
	// subject's own intent order. Both conditions are in the claim, so this
	// runner cannot reach an access-conferring row even by mistake — a guard here
	// would be one a future caller of the claim could skip.
	rows, err := claimRevocations(ctx, db.TargetZitadel, claimBatch)
	if err != nil {
		return DrainResult{}, fmt.Errorf("claim revocations: %w", err)
	}

	var res DrainResult
	for _, row := range rows {
		if halt := res.processRow(ctx, row); halt {
			return res, nil
		}
	}
	return res, nil
}
