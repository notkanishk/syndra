package expiry

import (
	"context"
	"errors"
	"log"

	"syndra/internal/db"
	"syndra/internal/models"
)

// Sweep runs a single pass: fetches up to batchSize expired direct grants,
// groups them by user, and for each user executes the cleanup pipeline.
// Scheduled by a periodic.Runner in main; batchSize is clamped to [1, 10000].
//
// The fetched snapshot is advisory only. Because UpsertDirectGrant renews
// grants via ON CONFLICT DO UPDATE (same row ID, pushed-forward expires_at),
// any decision made purely off the snapshot would be vulnerable to a renewal
// that lands between fetch and mutation. The authoritative gate is the DELETE
// itself, inside services.ExpireDirectGrant's transaction: it re-checks
// expires_at <= NOW() and a renewed grant simply does not match, taking the
// whole transaction — audit row, outbox rows and all — down with it.
//
// This package decides WHICH grants expire and in what order. What expiring one
// means belongs to services.ExpireDirectGrant, which computes the same closure
// delta an operator's removal computes and commits it with the ledger delete.
// The sweep's own job is scheduling, batching, and not losing one user's work
// to another user's failure.
func Sweep(ctx context.Context, batchSize int) {
	// Clamp against misconfiguration — a zero batch would sweep nothing
	// forever, an unbounded one defeats the batching.
	if batchSize < 1 {
		batchSize = 1
	}
	if batchSize > 10000 {
		batchSize = 10000
	}
	grants, err := svcGetExpiredDirectGrants(ctx, batchSize)
	if err != nil {
		log.Printf("[SCHEDULER] Failed to fetch expired grants: %v", err)
		return
	}
	if len(grants) == 0 {
		return
	}

	byUser := groupByUser(grants)
	log.Printf("[SCHEDULER] Sweep starting: candidates=%d users=%d", len(grants), len(byUser))

	for userID, userGrants := range byUser {
		if ctx.Err() != nil {
			log.Printf("[SCHEDULER] Context cancelled mid-sweep; stopping gracefully")
			return
		}
		processUser(ctx, userID, userGrants)
	}
}

// processUser expires a single user's candidate grants, one at a time.
//
// One at a time rather than one batched delete, because each grant's revocation
// delta depends on what the subject still holds — including the other grants in
// this batch. Two expiring grants can both derive the same role through mapping
// rules; computed together against one snapshot, each sees the other still
// covering it and neither revokes. Sequentially, each delta is computed against
// the state the previous delete left, and the last one out produces the revoke.
//
// Every side effect is driven off what the delete actually removed, never the
// pre-fetch snapshot, so a renewal landing between fetch and write is invisible
// to all of them.
func processUser(ctx context.Context, userID string, candidates []models.DirectGrant) {
	for _, candidate := range candidates {
		if ctx.Err() != nil {
			return
		}
		expireOne(ctx, userID, candidate)
	}
	// Once per user, after every grant of theirs: the compile is per-user, so
	// invalidating between grants would only rebuild state the next delete
	// invalidates again.
	if err := cacheInvalidateUser(ctx, userID); err != nil {
		log.Printf("[SCHEDULER] Cache invalidate failed user=%s err=%v (non-fatal)", userID, err)
	}
}

func expireOne(ctx context.Context, userID string, candidate models.DirectGrant) {
	res, err := svcExpireDirectGrant(ctx, userID, candidate.ID, candidate.ProjectID, candidate.RoleKey, expiryActor)
	switch {
	case errors.Is(err, db.ErrGrantRenewed):
		// Somebody pushed the expiry forward between the fetch and the write.
		// Nothing is wrong and nothing is owed: the grant is alive again.
		log.Printf("[SCHEDULER] Grant renewed before expiry could run user=%s grant=%s (left alone)", userID, candidate.ID)
		return
	case err != nil:
		log.Printf("[SCHEDULER] Expiry failed user=%s grant=%s err=%v — the grant stands and nothing was queued", userID, candidate.ID, err)
		return
	}

	// The ledger delete, the audit row, the revocations AND any target
	// convergence the lapsed role reached committed together — the lifecycle
	// trigger runs inside the same closure diff, so there is no second queue
	// left to write to and no window in which one could be half-written.
	log.Printf("[SCHEDULER] Grant expired user=%s grant=%s %s/%s revoked=%v retained=%v queued=%d",
		userID, candidate.ID, res.ProjectID, res.RoleKey, res.Revoked, res.Retained, len(res.OutboxIDs))
}

func groupByUser(grants []models.DirectGrant) map[string][]models.DirectGrant {
	m := make(map[string][]models.DirectGrant)
	for _, g := range grants {
		m[g.UserID] = append(m[g.UserID], g)
	}
	return m
}
