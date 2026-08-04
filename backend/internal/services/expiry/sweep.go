package expiry

import (
	"context"
	"log"

	"syndra/internal/models"
)

// Sweep runs a single pass: fetches up to batchSize expired direct grants,
// groups them by user, and for each user executes the cleanup pipeline.
// Scheduled by a periodic.Runner in main; batchSize is clamped to [1, 10000].
//
// The fetched snapshot is advisory only. Because UpsertDirectGrant renews
// grants via ON CONFLICT DO UPDATE (same row ID, pushed-forward expires_at),
// any decision made purely off the snapshot would be vulnerable to a
// renewal that lands between fetch and mutation. The authoritative gate is
// the DELETE itself: DeleteExpiredDirectGrantsByIDs re-validates
// expires_at <= NOW() atomically and returns only rows that were still
// expired at delete time. Every downstream side-effect is driven off that
// returned set, never the pre-fetch snapshot.
//
// Per-user order of operations:
//  1. Atomic guarded delete (user-scoped, expires_at re-checked, RETURNING
//     the rows actually removed). Renewed grants do not match and survive.
//  2. For each actually-deleted grant, write an audit row. Audit lands
//     immediately after the authoritative DB commit so the trail survives
//     any downstream failure.
//  3. For each actually-deleted grant, emit a provisioning intent
//     (idempotent via grantID-discriminated key) so the sync service
//     removes the LLDAP membership.
//  4. Invalidate the user's cache once (lazy rebuild on next request,
//     matching the webhook convention).
//  5. Best-effort Zitadel cascade per unique (project, role) tuple.
//     Log-and-continue on failure — matches the deferred
//     "Partial Failure Rollback" Phase-5 compromise.
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

// processUser runs the full pipeline for a single user's candidate grants.
// The post-delete sequence is driven off the rows DeleteExpiredDirectGrantsByIDs
// actually removed — NOT the pre-fetch snapshot — so a concurrent renewal
// that lands between fetch and delete is invisible to every subsequent step.
func processUser(ctx context.Context, userID string, candidates []models.DirectGrant) {
	ids := make([]string, len(candidates))
	for i, g := range candidates {
		ids[i] = g.ID
	}

	// Step 1: atomic guarded delete. Only rows still satisfying
	// expires_at <= NOW() at delete time are removed and returned.
	deleted, err := svcDeleteExpiredDirectGrantsByIDs(ctx, userID, ids)
	if err != nil {
		log.Printf("[SCHEDULER] Guarded delete failed user=%s candidates=%d err=%v — no downstream work for this user",
			userID, len(ids), err)
		return
	}
	if len(deleted) == 0 {
		// All candidates were concurrently renewed (or already removed).
		// Nothing to clean up for this user this tick.
		log.Printf("[SCHEDULER] No grants deleted user=%s candidates=%d (all renewed or concurrently removed)",
			userID, len(ids))
		return
	}
	if len(deleted) < len(ids) {
		log.Printf("[SCHEDULER] Partial delete user=%s candidates=%d deleted=%d (remainder renewed or concurrently removed)",
			userID, len(ids), len(deleted))
	}

	// Step 2: audit each actually-deleted grant. Written before intent and
	// cascade so the audit trail survives any later side-effect failure.
	for _, g := range deleted {
		_ = svcInsertAuditLog(ctx, "system:scheduler", userID, "direct_grant.revoked_by_expiry", g.ID)
	}

	// Step 3: emit provisioning intents for actually-deleted grants only.
	// Idempotency key is grantID-discriminated so repeated sweeps across
	// renewals cannot collide on an earlier grant's key.
	for _, g := range deleted {
		if err := svcEmitIntentFromScheduler(ctx, userID, "remove", g.ProjectID, g.RoleKey, g.ID); err != nil {
			log.Printf("[SCHEDULER] Intent emit failed after delete user=%s grant=%s project=%s role=%s err=%v — LLDAP orphan possible; reconciler will reap",
				userID, g.ID, g.ProjectID, g.RoleKey, err)
		}
	}

	// Step 4: cache invalidate once per user.
	if err := cacheInvalidateUser(ctx, userID); err != nil {
		log.Printf("[SCHEDULER] Cache invalidate failed user=%s err=%v (non-fatal)", userID, err)
	}

	// Step 5: best-effort Zitadel cascade, deduped per (project, role).
	seen := make(map[string]struct{}, len(deleted))
	for _, g := range deleted {
		key := g.ProjectID + "|" + g.RoleKey
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		if err := zitadelRevokeMappingRules(ctx, userID, g.ProjectID, g.RoleKey); err != nil {
			log.Printf("[SCHEDULER] Zitadel cascade failed user=%s project=%s role=%s err=%v — derived-grant orphans may remain; reconciler will clean up",
				userID, g.ProjectID, g.RoleKey, err)
		}
	}
}

func groupByUser(grants []models.DirectGrant) map[string][]models.DirectGrant {
	m := make(map[string][]models.DirectGrant)
	for _, g := range grants {
		m[g.UserID] = append(m[g.UserID], g)
	}
	return m
}
