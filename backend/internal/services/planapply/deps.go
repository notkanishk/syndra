package planapply

import "syndra/internal/db"

// Injectable seams. The whole content of this gate is an ORDER — the plan is
// claimed before any work is queued, and everything happens inside one
// transaction — and an ordering is not visible to a per-call assertion. Tests
// swap these for recorders and read the sequence back.
var (
	inTx        = db.InTx
	targetState = db.TargetStateTx
	claimPlan   = db.ClaimPlanTx
	enqueue     = db.EnqueueEntitlementApplyTx
)
