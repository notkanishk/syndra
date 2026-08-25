package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// beginTx is a seam. InTx's contract is "commit on success, roll back on
// failure", which is the atomicity every caller in this package relies on and
// which no caller's own tests can check — they fake InTx itself. Opening the
// transaction through a variable lets that contract be tested here, against a
// transaction that records what it was asked to do.
var beginTx = func(ctx context.Context) (pgx.Tx, error) { return PG.Begin(ctx) }

// InTx runs fn inside one transaction, committing when it returns nil and
// rolling back when it does not.
//
// The rollback is what callers actually want and what hand-rolled versions
// forget: a step that fails after an earlier step succeeded must undo the
// earlier one, and the deferred rollback is a no-op once Commit has run.
func InTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := beginTx(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) // no-op after a successful Commit

	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// txKey carries an in-progress access-mutation transaction. It is unexported
// and the only writer is InTxLockingAccess, so a transaction cannot arrive in a
// context by accident.
type txKeyType struct{}

var txKey txKeyType

// Querier is the subset of pgxpool.Pool that pgx.Tx also satisfies: the three
// verbs every statement in this package uses. It exists so a statement can be
// written once and run either on the pool or on an ambient transaction.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// querier is what a statement in this package runs against: the ambient
// access-mutation transaction when there is one, and the pool otherwise.
//
// Every statement goes through it, not merely the ones somebody remembered were
// inside a lock. Two failures follow from a statement that reaches past the
// transaction it is running inside, and both are silent:
//
//   - A READ cannot see the caller's own uncommitted write. The lifecycle
//     trigger resolves a subject's entitlements immediately after the grant
//     change that provoked it; on the pool it resolves the world as it was
//     BEFORE, so gaining a first mapped role queues "disable this account" and
//     losing the last one queues "keep the access". Both halves of reversible
//     deprovisioning invert.
//   - A WRITE commits on its own. A mapping edit that succeeds on the pool
//     inside a transaction that then rolls back leaves the mapping changed with
//     nothing queued to converge it.
//
// Neither is visible to a test that fakes the layer below, which is why this is
// one accessor rather than a rule about which functions must remember.
func querier(ctx context.Context) Querier {
	if tx, ok := ctx.Value(txKey).(pgx.Tx); ok && tx != nil {
		return tx
	}
	return PG
}

// beginOrJoin returns the ambient access-mutation transaction if the caller is
// running inside one, and otherwise opens a fresh transaction of its own.
//
// The second return value says which happened. A joiner must not commit or roll
// back what it did not open — the caller has more to do and owns the decision —
// so it neither defers a rollback nor commits, and an error it returns reaches
// the owner, who does both.
//
// This exists so an access change and the reads that decided it can share one
// transaction without every write in this package taking a transaction
// parameter. The alternative was threading `tx` through a dozen exported
// functions and every test that fakes them, for a property none of those
// callers can express: whether the read that justified the write is inside the
// same lock.
func beginOrJoin(ctx context.Context) (tx pgx.Tx, owned bool, err error) {
	if ambient, ok := ctx.Value(txKey).(pgx.Tx); ok && ambient != nil {
		return ambient, false, nil
	}
	tx, err = beginTx(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("begin tx: %w", err)
	}
	return tx, true, nil
}

// InTxLockingAccess runs fn inside one transaction that holds the
// access-mutation lock, and hands fn a context that every write in this package
// will join rather than open its own transaction against.
//
// The lock is taken first, before fn reads anything. An effective-access delta
// is a statement about a world, and the window that matters runs from the read
// that observed that world to the commit that acts on it. Locking only around
// the write serialises the commits and leaves every computation racing: a
// cascade can read while a grant is still live, compute a delta that adds
// nothing because the role is already covered, write its own source row, and
// only then queue behind the lock — by which time the expiry that removed the
// cover has committed a revoke, and the cascade commits an empty delta over it.
// The subject ends up holding the bundle and not the role.
//
// One lock rather than one per subject, because the subjects are not always
// known before the reads: a rule change cascades to every holder of the source
// role, and which holders those are is what the reads are for. A lock taken
// after that answer is a lock taken after the question it was meant to settle.
//
// syndra: global serialisation of access mutations. Every cascade here is a
// handful of queries against a makerspace-sized directory, and they are already
// serialised behind one operator's clicks. Per-subject locking is the upgrade
// path if a bulk action ever needs to fan out concurrently — it needs the
// subject set resolved before the reads, which means resolving membership in
// SQL rather than in Go.
func InTxLockingAccess(ctx context.Context, fn func(context.Context) error) error {
	return InTx(ctx, func(tx pgx.Tx) error {
		if err := LockAccessMutationTx(ctx, tx); err != nil {
			return err
		}
		return fn(context.WithValue(ctx, txKey, tx))
	})
}
