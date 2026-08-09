package db

import (
	"context"
	"fmt"
	"sort"

	"github.com/jackc/pgx/v5"
)

// subjectAccessLockSpace namespaces the advisory lock so it cannot collide with
// any other advisory lock the deployment takes. Postgres advisory locks live in
// one global space keyed by two 32-bit integers; the first identifies what kind
// of thing is being locked, the second which one.
const subjectAccessLockSpace = 0x53594e44 // "SYND"

// LockSubjectAccessTx serialises every transaction that changes what a subject
// can reach, for the life of the caller's transaction.
//
// It exists because effective access is computed OUTSIDE the transaction that
// writes it. Every cascade reads a subject's grants, bundles and rules, diffs
// the closure, and enqueues the difference — and between the read and the
// commit another transaction can change any of those inputs. The queued delta
// is then a statement about a world that no longer exists: a revoke for a role
// somebody just granted through a bundle, landing after the add that granted
// it, leaving the target without access the subject is currently owed.
//
// Holding this lock across the read as well as the write is what closes that.
// The reads need not run on the transaction — they need to run while nothing
// that could invalidate them is able to commit, and a writer that must take
// this lock to enqueue cannot commit while it is held. A reader on another
// connection therefore sees a state that stays true until the lock is released.
//
// Locks are taken in sorted order and deduplicated. A cascade that touches many
// subjects (moving a bundle's holders) would otherwise be able to take them in
// one order while a second cascade takes them in another, and the two would
// wait on each other forever.
//
// Re-acquisition is free: Postgres counts advisory locks per session, and a
// transaction that already holds one takes it again without blocking. A caller
// that locks before its reads and then calls into an enqueue that locks again
// is not a mistake, it is the two guarantees stacking.
func LockSubjectAccessTx(ctx context.Context, tx pgx.Tx, subjects ...string) error {
	unique := make(map[string]struct{}, len(subjects))
	ordered := make([]string, 0, len(subjects))
	for _, s := range subjects {
		if s == "" || s == "-" {
			// "-" is the placeholder an audit row uses when the event is about
			// an object rather than a person. There is no subject to serialise.
			continue
		}
		if _, seen := unique[s]; seen {
			continue
		}
		unique[s] = struct{}{}
		ordered = append(ordered, s)
	}
	sort.Strings(ordered)

	for _, subject := range ordered {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1, hashtext($2))`,
			subjectAccessLockSpace, subject); err != nil {
			return fmt.Errorf("lock subject access %s: %w", subject, err)
		}
	}
	return nil
}

// InTxLockingSubject opens a transaction, takes the subject's access lock as its
// first statement, and runs fn.
//
// The lock comes first on purpose. Taken after the caller has computed what it
// intends to write, it would serialise the writes and leave every computation
// racing — which is the failure it exists to prevent, with the appearance of a
// fix on top.
func InTxLockingSubject(ctx context.Context, subject string, fn func(pgx.Tx) error) error {
	return InTx(ctx, func(tx pgx.Tx) error {
		if err := LockSubjectAccessTx(ctx, tx, subject); err != nil {
			return err
		}
		return fn(tx)
	})
}
