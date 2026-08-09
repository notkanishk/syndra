package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
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
