package handlers

import (
	"context"
	"log"

	"syndra/internal/db"
)

// grantDirect writes a direct grant through the ledger and the outbox, then
// queues the convergence on the system its project serves.
//
// Every direct grant goes through here, and so does the approval of a request,
// because neither passes the services cascade: a member whose first storage
// role came that way got no account until an operator ran "apply entitlements"
// by hand.
func grantDirect(ctx context.Context, p db.EnqueueParams) (db.EnqueueResult, error) {
	res, err := dbEnqueueDirectGrantPropagation(ctx, p)
	if err == nil {
		convergeAfterGrant(ctx, p.GrantedBy, p.UserID, p.ProjectID)
	}
	return res, err
}

// convergeAfterGrant is best-effort, after the grant has committed. The grant
// stands either way; a convergence that failed to queue here is queued by the
// next reconcile pass, which walks everyone who holds a mapped role.
func convergeAfterGrant(ctx context.Context, actor, userID, projectID string) {
	if err := svcConvergeProjectChange(ctx, actor, userID, projectID); err != nil {
		log.Printf("[ACCESS] could not queue the target convergence for %s in %s (the next reconcile will): %v", userID, projectID, err)
	}
}

// convergeAfterHold is the same, for one person on one target: a hold created
// or lifted, or a person the directory deactivated.
func convergeAfterHold(ctx context.Context, actor, userID, target string) {
	if err := svcConvergeSubjectOn(ctx, actor, userID, target); err != nil {
		log.Printf("[ALLOWANCE] could not queue the convergence for %s on %s (the next reconcile will): %v", userID, target, err)
	}
}
