package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// accessLockSpace namespaces the advisory lock so it cannot collide with any
// other advisory lock the deployment takes. Postgres advisory locks live in one
// global space keyed by two 32-bit integers; the first says what kind of thing
// is being locked, the second which one.
const accessLockSpace = 0x53594e44 // "SYND"

// accessLockKey is the single access-mutation lock. There is one, deliberately.
//
// A per-subject lock cannot be taken before the reads of a cascade whose
// subjects the reads are what determine — a rule change reaches every holder of
// the source role, and locking after that answer is locking after the question
// it was meant to settle. Splitting the lock would therefore leave exactly the
// paths that touch the most people unprotected.
const accessLockKey = 1

// LockAccessMutationTx serialises every transaction that changes what anyone
// can reach, for the life of the caller's transaction.
//
// It exists because effective access is computed from reads that are not part
// of the write. Every cascade reads a subject's grants, bundles and rules,
// diffs the closure, and enqueues the difference. Two of those interleaving
// produce a delta that is true of neither the world before nor the world after:
// a bundle assignment can read while a direct grant still covers the role,
// conclude it has nothing to add, and commit that emptiness on top of an expiry
// that just revoked the cover.
//
// Held from before the reads to the commit, this makes that interleaving
// impossible. Held only around the write it makes it invisible, which is worse
// than not holding it at all.
//
// Re-acquisition is free: Postgres counts advisory locks per session, so a
// transaction that already holds this one takes it again without blocking. That
// is what lets the enqueue take it as a backstop while a caller that locked
// before its own reads holds it already.
func LockAccessMutationTx(ctx context.Context, tx pgx.Tx) error {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1, $2)`,
		accessLockSpace, accessLockKey); err != nil {
		return fmt.Errorf("lock access mutation: %w", err)
	}
	return nil
}
