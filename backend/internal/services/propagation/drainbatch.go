package propagation

import (
	"context"
	"fmt"
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
	for _, id := range ids {
		row, found, err := claimOne(ctx, id)
		if err != nil || !found {
			continue // already terminal, gone, or unclaimable — skip
		}
		if halt := res.processRow(ctx, *row); halt {
			break // retry budget exceeded (same halt semantics as Drain)
		}
	}
	return res, nil
}
