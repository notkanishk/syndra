package db

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
)

// recordingTx implements pgx.Tx by embedding it: only Commit and Rollback are
// called here, and any other method a future InTx reached for would panic on
// the nil embedded value rather than quietly do nothing.
type recordingTx struct {
	pgx.Tx
	committed  bool
	rolledBack bool
	commitErr  error
}

func (t *recordingTx) Commit(context.Context) error   { t.committed = true; return t.commitErr }
func (t *recordingTx) Rollback(context.Context) error { t.rolledBack = true; return nil }

func withBegin(t *testing.T, tx *recordingTx, err error) {
	t.Helper()
	saved := beginTx
	t.Cleanup(func() { beginTx = saved })
	beginTx = func(context.Context) (pgx.Tx, error) {
		if err != nil {
			return nil, err
		}
		return tx, nil
	}
}

// The contract every caller in this package leans on. Callers fake InTx in
// their own tests, so this is the only place it can be checked: a body that
// fails must not commit, or the "both or neither" every enqueue and every plan
// claim promises is a promise nothing keeps.
func TestInTxCommitsOnlyWhatSucceeded(t *testing.T) {
	t.Run("a body that returns nil commits", func(t *testing.T) {
		tx := &recordingTx{}
		withBegin(t, tx, nil)

		if err := InTx(context.Background(), func(pgx.Tx) error { return nil }); err != nil {
			t.Fatalf("InTx = %v", err)
		}
		if !tx.committed {
			t.Error("a successful body did not commit")
		}
	})

	t.Run("a body that fails does not commit", func(t *testing.T) {
		tx := &recordingTx{}
		withBegin(t, tx, nil)
		boom := errors.New("the second write failed")

		err := InTx(context.Background(), func(pgx.Tx) error { return boom })
		if !errors.Is(err, boom) {
			t.Fatalf("InTx = %v, want the body's own error unwrapped", err)
		}
		if tx.committed {
			t.Error("committed a transaction whose body failed — every earlier write in it survives")
		}
		if !tx.rolledBack {
			t.Error("did not roll back")
		}
	})

	t.Run("a failed commit is reported", func(t *testing.T) {
		tx := &recordingTx{commitErr: errors.New("connection lost")}
		withBegin(t, tx, nil)

		if err := InTx(context.Background(), func(pgx.Tx) error { return nil }); err == nil {
			t.Error("a commit that failed was reported as success")
		}
	})

	t.Run("a body never runs without a transaction", func(t *testing.T) {
		withBegin(t, nil, errors.New("pool exhausted"))
		ran := false

		if err := InTx(context.Background(), func(pgx.Tx) error { ran = true; return nil }); err == nil {
			t.Error("InTx succeeded without a transaction")
		}
		if ran {
			t.Error("the body ran without a transaction to run in")
		}
	})
}
