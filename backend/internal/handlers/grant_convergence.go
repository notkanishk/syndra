package handlers

import (
	"context"
	"log"
)

// convergeAfterHold queues the convergence for one person on one target: a
// hold created or lifted, or a person the directory deactivated. Best-effort,
// after the change has committed: one that failed to queue here is queued by
// the next reconcile pass, which walks everyone who holds a mapped role. (A
// grant's own convergence is the planner's, inside its transaction:
// deferTargetConvergence.)
func convergeAfterHold(ctx context.Context, actor, userID, target string) {
	if err := svcConvergeSubjectOn(ctx, actor, userID, target); err != nil {
		log.Printf("[ALLOWANCE] could not queue the convergence for %s on %s (the next reconcile will): %v", userID, target, err)
	}
}
