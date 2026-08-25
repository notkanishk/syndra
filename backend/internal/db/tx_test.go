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

// A write that runs inside an access-mutation transaction must join it, not
// open a second one. Two transactions mean the reads that decided the write are
// in one and the write is in the other — the lock spans the first, and the
// second is free to commit after everything it assumed has changed.
func TestBeginOrJoinJoinsTheAmbientTransaction(t *testing.T) {
	ambient := &recordingTx{}
	fresh := &recordingTx{}
	withBegin(t, fresh, nil)

	ctx := context.WithValue(context.Background(), txKey, pgx.Tx(ambient))
	tx, owned, err := beginOrJoin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if tx != pgx.Tx(ambient) {
		t.Fatal("the caller's transaction must be the one used")
	}
	if owned {
		t.Fatal("a joiner does not own the transaction and must not settle it")
	}
	if fresh.committed || fresh.rolledBack {
		t.Fatal("no second transaction may be opened at all")
	}
}

// Without an ambient transaction it opens its own and says so, because then it
// is the only thing that can commit it.
func TestBeginOrJoinOwnsWhatItOpens(t *testing.T) {
	fresh := &recordingTx{}
	withBegin(t, fresh, nil)

	tx, owned, err := beginOrJoin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if tx != pgx.Tx(fresh) || !owned {
		t.Fatalf("an unwrapped caller opens and owns its own transaction, got owned=%v", owned)
	}
}

// A nil in the context is not a transaction. Treating it as one would hand a
// nil transaction to every write in the package.
func TestBeginOrJoinIgnoresANilAmbientTransaction(t *testing.T) {
	fresh := &recordingTx{}
	withBegin(t, fresh, nil)

	ctx := context.WithValue(context.Background(), txKey, pgx.Tx(nil))
	tx, owned, err := beginOrJoin(ctx)
	if err != nil || tx != pgx.Tx(fresh) || !owned {
		t.Fatalf("a nil ambient transaction must be ignored, got tx=%v owned=%v err=%v", tx, owned, err)
	}
}
