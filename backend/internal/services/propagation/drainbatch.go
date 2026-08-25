package propagation

import (
	"context"
	"fmt"

	"syndra/internal/db"
)

// DrainBatch applies ONLY the outbox rows whose ids are given, under one advisory lock and one
// reachability preflight (a per-id DrainOne loop would re-lock and re-preflight every row —
// wasteful on a 200-row cascade). It reuses Drain/DrainOne's per-row processing verbatim. Rows
// not in `ids` (e.g. queued manual-mode rows) are never claimed, so an auto cascade drains its
// own rows without touching a manual operator's queued mutations.
func DrainBatch(ctx context.Context, ids []string) (DrainResult, error) {
	var res DrainResult
	if len(ids) == 0 {
		return res, nil
	}
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
	// Same target scope as DrainOne, and for the same reason: this loop hands
	// whatever it claims to the Zitadel dispatcher, which would mark an add-on
	// entitlement (`op_type='apply'`) terminally failed as an unknown operation
	// — before its own dispatcher exists, and with no way back from `failed`.
	// The claim refuses those rows instead, so the loop cannot reach them.
	seen := map[string]bool{}
	for _, id := range ids {
		row, found, err := claimOne(ctx, db.TargetZitadel, id)
		if err != nil {
			continue
		}
		if !found {
			for _, t := range undispatchableTargets(ctx, id) {
				if !seen[t] {
					seen[t] = true
					res.Awaiting = append(res.Awaiting, t)
				}
			}
			continue // already terminal, gone, or not this dispatcher's
		}
		if halt := res.processRow(ctx, *row); halt {
			break // retry budget exceeded (same halt semantics as Drain)
		}
	}
	return res, nil
}
