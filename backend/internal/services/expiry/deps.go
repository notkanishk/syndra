// Package expiry contains the backend-side scheduler that sweeps expired
// direct role grants and performs the end-to-end cleanup side effects
// (hard-delete, the closure delta and the convergences it queues, cache
// invalidation, audit, and
// best-effort Zitadel derived-grant cascade).
//
// Effective access for expired grants is already correct at query time via
// GetDirectGrantsForUser(..., includeExpired=false). This package closes the
// *cleanup* gap: without it, expired rows linger in the DB, target access keeps
// stale members, and Zitadel derived grants remain authoritative-looking.
package expiry

import (
	"context"
	"syndra/internal/cache"
	"syndra/internal/db"
	"syndra/internal/services"
)

// Injectable dependencies. Mirrors the save-swap-restore pattern used across
// the backend (see services/deps.go, cache/deps.go, zitadel/deps.go). Tests
// exercise sweep logic without a live DB/Redis/Zitadel by swapping these.
var (
	svcGetExpiredDirectGrants = db.GetExpiredDirectGrants
	// One grant's whole expiry: closure delta, guarded ledger delete, audit and
	// outbox rows, in one transaction. The sweep decides WHICH grants and in
	// what order; it does not decide what expiring one means.
	svcExpireDirectGrant = services.ExpireDirectGrant
	cacheInvalidateUser  = cache.InvalidateUser

	// Allowance expiry. Its own seams and its own pass: grant expiry removes
	// access and this restores it, and a batch that aborted halfway through the
	// first would silently skip the second — on the half that gives access back
	// to somebody who is owed it.
	dbLapsedAllowances       = db.LapsedAllowances
	dbResolveLapsedAllowance = db.ResolveLapsedAllowance

	// reconvergeSubject tells the target that a suspension ended.
	//
	// Resolution is already correct the moment the date passes — the resolver
	// compares the expiry in its predicate — so what this closes is the gap
	// between Syndra being right and the target being told. Without it, a
	// suspension that lapsed at midnight leaves the person locked out until
	// somebody happens to drive a change through them.
	//
	// It queues rather than applies, like every other add-on row: the
	// convergence waits for the drain. And it is system-minted, for the same
	// reason a cascade's is — nobody reviewed a diff here either, a clock did.
	reconvergeSubject = func(ctx context.Context, subjectID, target string) error {
		set, err := services.ResolveEntitlements(ctx, subjectID, target)
		if err != nil {
			return err
		}
		_, _, err = db.RecordSystemConvergence(ctx, db.SystemConvergence{
			Target: target, SubjectID: subjectID, Actor: expiryActor,
			Reason: "A subtractive allowance lapsed", Desired: set.Desired(),
		})
		return err
	}
)

// expiryActor is the actor recorded on every row this sweep writes. Nobody
// clicked anything; a clock did, and the audit trail says so.
const expiryActor = "system:scheduler"
