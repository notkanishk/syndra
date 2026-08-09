package db

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

// 1.16 — the expiry re-check lives in the DELETE's own predicate. A renewal
// lands on the same row (ON CONFLICT DO UPDATE pushes expires_at forward), so
// anything decided from the sweep's snapshot is decided about a row that may
// already be alive again. Under READ COMMITTED the DELETE re-evaluates against
// the version it finds, and a renewed grant simply does not match.
func TestExpiryWriteCarriesItsOwnGuard(t *testing.T) {
	body := funcBody(t, readDBSource(t, "grants.go"), "DeleteExpiredDirectGrantAndEnqueueTx")

	del := regexp.MustCompile(`(?s)DELETE FROM direct_role_grants(.*?)RETURNING`).FindStringSubmatch(body)
	if del == nil {
		t.Fatal("could not isolate the expiry delete")
	}
	for _, want := range []string{
		"WHERE id = $1 AND user_id = $2",
		"expires_at IS NOT NULL",
		"expires_at <= NOW()",
	} {
		if !strings.Contains(del[1], want) {
			t.Errorf("the expiry delete must assert %q in its own predicate, not rely on the caller having checked", want)
		}
	}

	// The delta, the audit row and the ledger delete commit together or not at
	// all. Enqueuing outside this transaction would leave a revoke queued for a
	// grant that is still valid, and an audit row about a deletion that did not
	// happen.
	if !strings.Contains(body, "enqueueCascadeRows(ctx, tx,") {
		t.Error("the outbox and audit writes must run on this transaction")
	}
	if !strings.Contains(body, `Action: "direct_grant.revoked_by_expiry"`) {
		t.Error("the audit row must say a clock did this, not a person")
	}
	// Project and role are read back from the row that was actually removed, so
	// every downstream side effect names what went away rather than what the
	// snapshot said would.
	if !strings.Contains(body, "RETURNING zitadel_project_id, zitadel_role_key") {
		t.Error("the delete must return the identifiers the caller then uses")
	}
}

// A renewed grant is not a missing one. Nothing is wrong, nothing is owed, and
// the sweep must be able to tell the difference — one means leave it alone, the
// other means something is broken.
func TestRenewedIsNotMissing(t *testing.T) {
	if errors.Is(ErrGrantRenewed, ErrGrantNotFound) || errors.Is(ErrGrantNotFound, ErrGrantRenewed) {
		t.Fatal("a renewed grant and an absent one must be distinguishable")
	}
	body := funcBody(t, readDBSource(t, "grants.go"), "DeleteExpiredDirectGrantAndEnqueueTx")
	if !regexp.MustCompile(`(?s)errors\.Is\(err, pgx\.ErrNoRows\).*?explainExpiryRefusal\(ctx, tx, userID, grantID\)`).MatchString(body) {
		t.Error("a no-match delete must be explained, not assumed: two different predicates can fail here")
	}

	// The explanation runs on the same transaction, so it answers about the row
	// the predicate just rejected rather than about whatever a later snapshot
	// finds. And it reports a lookup failure rather than guessing — a renewal
	// Syndra did not observe would tell an operator everything is fine.
	explain := funcBody(t, readDBSource(t, "grants.go"), "DeleteExpiredDirectGrantAndEnqueueTx")
	if !strings.Contains(explain, "explainExpiryRefusal(ctx, tx, userID, grantID)") {
		t.Error("the explanation must run on the caller's transaction, about the same row and the same user the predicate rejected")
	}
}

// fakeRow answers one EXISTS query.
type fakeRow struct {
	exists bool
	err    error
}

func (f fakeRow) Scan(dest ...any) error {
	if f.err != nil {
		return f.err
	}
	*(dest[0].(*bool)) = f.exists
	return nil
}

// fakeQuerier records what it was asked and answers with a fixed row.
type fakeQuerier struct {
	sql  string
	args []any
	row  fakeRow
}

func (f *fakeQuerier) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	f.sql, f.args = sql, args
	return f.row
}

// Which of the two errors this returns IS the finding. Asserted behaviourally,
// because a source-level check that both appear somewhere in the function
// cannot tell them apart when they are swapped — three mutations survived on
// exactly that before this test existed.
func TestExpiryRefusalTellsRenewalFromAbsence(t *testing.T) {
	q := &fakeQuerier{row: fakeRow{exists: true}}
	if err := explainExpiryRefusal(context.Background(), q, "u1", "g1"); !errors.Is(err, ErrGrantRenewed) {
		t.Errorf("a row that is still there failed the expiry predicate: want ErrGrantRenewed, got %v", err)
	}

	q = &fakeQuerier{row: fakeRow{exists: false}}
	if err := explainExpiryRefusal(context.Background(), q, "u1", "g1"); !errors.Is(err, ErrGrantNotFound) {
		t.Errorf("a row that is gone was removed by something else: want ErrGrantNotFound, got %v", err)
	}

	// A lookup that failed establishes neither. Reporting a renewal Syndra did
	// not observe would tell an operator everything is fine.
	q = &fakeQuerier{row: fakeRow{err: errors.New("connection lost")}}
	err := explainExpiryRefusal(context.Background(), q, "u1", "g1")
	if errors.Is(err, ErrGrantRenewed) || errors.Is(err, ErrGrantNotFound) {
		t.Errorf("a failed lookup must reach neither verdict, got %v", err)
	}
	if err == nil {
		t.Error("a failed lookup must still be an error")
	}
}

// The question is about the same row the delete refused — which means the same
// user. Asked by id alone it would answer about somebody else's grant and call
// that a renewal.
func TestExpiryRefusalAsksAboutTheSameRow(t *testing.T) {
	q := &fakeQuerier{row: fakeRow{exists: true}}
	_ = explainExpiryRefusal(context.Background(), q, "u1", "g1")

	if !strings.Contains(q.sql, "user_id = $2") {
		t.Errorf("the lookup must be scoped to the user, got %q", q.sql)
	}
	if len(q.args) != 2 || q.args[0] != "g1" || q.args[1] != "u1" {
		t.Errorf("the lookup must bind the grant and the user in that order, got %v", q.args)
	}
}

// The write runs on the caller's transaction, because the delta it is handed was
// computed under that transaction's subject lock. Opening its own would leave
// the lock around the write only — which serialises the writes and leaves every
// computation racing, with the appearance of a fix on top.
func TestTheExpiryWriteJoinsTheCallersTransaction(t *testing.T) {
	body := funcBody(t, readDBSource(t, "grants.go"), "DeleteExpiredDirectGrantAndEnqueueTx")
	if strings.Contains(body, "PG.Begin(") {
		t.Error("the expiry write must not open its own transaction")
	}
	if !strings.Contains(body, "tx.QueryRow(ctx, deleteGrant") {
		t.Error("the delete must run on the transaction it was handed")
	}
}

// Holding the subject lock is worth nothing unless every writer has to take it.
// Both enqueue chokepoints do, so a caller that locked before its own reads is
// serialised against callers that did not.
func TestEveryEnqueueTakesTheSubjectLock(t *testing.T) {
	for _, tc := range []struct{ file, fn string }{
		{"cascade.go", "enqueueCascadeRows"},
		{"propagation_enqueue.go", "enqueueWrites"},
	} {
		body := funcBody(t, readDBSource(t, tc.file), tc.fn)
		if !strings.Contains(body, "LockSubjectAccessTx(ctx, tx,") {
			t.Errorf("%s must take the subject lock, or a caller holding it protects nothing", tc.fn)
		}
		// Before anything is written: a lock taken after the first INSERT would
		// leave a window in which another writer's delta is already committed.
		lock := strings.Index(body, "LockSubjectAccessTx")
		insert := strings.Index(body, "INSERT INTO")
		if insert >= 0 && lock > insert {
			t.Errorf("%s must lock before it writes", tc.fn)
		}
	}
}

// Deadlock avoidance is the reason the order is fixed. Two cascades touching
// the same set of people in different orders would wait on each other forever.
func TestSubjectLocksAreTakenInAFixedOrder(t *testing.T) {
	body := funcBody(t, readDBSource(t, "subject_lock.go"), "LockSubjectAccessTx")
	if !strings.Contains(body, "sort.Strings(ordered)") {
		t.Error("locks must be taken in a fixed order across callers")
	}
	if !strings.Contains(body, "pg_advisory_xact_lock") {
		t.Error("the lock must be transaction-scoped, so a crashed caller cannot hold it forever")
	}
	// The placeholder an audit row uses when the event is about an object
	// rather than a person is not a subject.
	if !strings.Contains(body, `s == "-"`) || !strings.Contains(body, `s == ""`) {
		t.Error("non-subjects must not be locked")
	}
}

// The lock comes first in the transaction, before the caller computes anything.
func TestTheSubjectLockPrecedesTheCallersWork(t *testing.T) {
	body := funcBody(t, readDBSource(t, "subject_lock.go"), "InTxLockingSubject")
	lock := strings.Index(body, "LockSubjectAccessTx")
	call := strings.Index(body, "return fn(tx)")
	if lock < 0 || call < 0 || lock > call {
		t.Error("the lock must be taken before the caller's work, or it serialises the writes and leaves the reads racing")
	}
}

// The batched delete this replaced took a user's whole candidate set in one
// statement, which cannot carry a per-grant delta: two grants deriving the same
// role would each see the other still covering it.
func TestTheBatchedExpiryDeleteIsGone(t *testing.T) {
	src := readDBSource(t, "grants.go")
	if strings.Contains(src, "func DeleteExpiredDirectGrantsByIDs") {
		t.Error("the batched delete has no caller and no delta; keeping it invites a second expiry path that revokes nothing")
	}
}
