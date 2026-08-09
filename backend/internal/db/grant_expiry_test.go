package db

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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
	body := funcBody(t, readDBSource(t, "grants.go"), "DeleteExpiredDirectGrantAndEnqueue")

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
	body := funcBody(t, readDBSource(t, "grants.go"), "DeleteExpiredDirectGrantAndEnqueue")
	if !regexp.MustCompile(`(?s)errors\.Is\(err, pgx\.ErrNoRows\).*?explainExpiryRefusal\(ctx, tx, userID, grantID\)`).MatchString(body) {
		t.Error("a no-match delete must be explained, not assumed: two different predicates can fail here")
	}

	// The explanation runs on the same transaction, so it answers about the row
	// the predicate just rejected rather than about whatever a later snapshot
	// finds. And it reports a lookup failure rather than guessing — a renewal
	// Syndra did not observe would tell an operator everything is fine.
	explain := funcBody(t, readDBSource(t, "grants.go"), "DeleteExpiredDirectGrantAndEnqueue")
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

// Every access write joins the caller's transaction when there is one, so the
// reads that decided it are inside the same lock. Opening its own would put the
// lock around the write only — which serialises the commits and leaves every
// computation racing, with the appearance of a fix on top.
func TestEveryAccessWriteJoinsAnAmbientTransaction(t *testing.T) {
	for _, tc := range []struct{ file, fn string }{
		{"grants.go", "DeleteExpiredDirectGrantAndEnqueue"},
		{"grants.go", "DeleteDirectGrantAndEnqueue"},
		{"cascade.go", "AssignBundleAndEnqueue"},
		{"cascade.go", "RemoveBundleFromUserAndEnqueue"},
		{"cascade.go", "CreateMappingRuleAndEnqueue"},
		{"cascade.go", "UpdateMappingRuleAndEnqueue"},
		{"cascade.go", "DeleteMappingRuleAndEnqueue"},
		{"bundles.go", "DeleteBundleAndEnqueue"},
		{"bundle_versions.go", "PublishVersionAndEnqueue"},
		{"bundle_versions.go", "MoveHoldersAndEnqueue"},
		{"access_requests.go", "ApproveRequestAndEnqueue"},
		{"propagation_enqueue.go", "enqueueTx"},
	} {
		body := funcBody(t, readDBSource(t, tc.file), tc.fn)
		if strings.Contains(body, "PG.Begin(") {
			t.Errorf("%s opens its own transaction, so a caller that locked before its reads cannot include it", tc.fn)
		}
		if !strings.Contains(body, "beginOrJoin(ctx)") {
			t.Errorf("%s must join an ambient access transaction when there is one", tc.fn)
		}
		// A joiner must not commit what it did not open.
		if strings.Contains(body, "tx.Commit(ctx)") && !strings.Contains(body, "if owned {") {
			t.Errorf("%s commits unconditionally; a joined transaction belongs to its caller", tc.fn)
		}
	}
}

// Holding the lock is worth nothing unless every writer has to take it. Both
// enqueue chokepoints do, so a path that has not been wrapped is still
// serialised against one that has.
func TestEveryEnqueueTakesTheAccessLock(t *testing.T) {
	for _, tc := range []struct{ file, fn string }{
		{"cascade.go", "enqueueCascadeRows"},
		{"propagation_enqueue.go", "enqueueWrites"},
	} {
		body := funcBody(t, readDBSource(t, tc.file), tc.fn)
		if !strings.Contains(body, "LockAccessMutationTx(ctx, tx)") {
			t.Errorf("%s must take the access lock, or a caller holding it protects nothing", tc.fn)
		}
		lock := strings.Index(body, "LockAccessMutationTx")
		insert := strings.Index(body, "INSERT INTO")
		if insert >= 0 && lock > insert {
			t.Errorf("%s must lock before it writes", tc.fn)
		}
	}
}

// The lock is transaction-scoped, so a crashed holder cannot keep every access
// change in the deployment waiting.
func TestTheAccessLockIsTransactionScoped(t *testing.T) {
	body := funcBody(t, readDBSource(t, "subject_lock.go"), "LockAccessMutationTx")
	if !strings.Contains(body, "pg_advisory_xact_lock") {
		t.Error("a session-scoped lock outlives the work it guards")
	}
}

// The lock comes first, before fn reads anything, and the transaction it holds
// is the one every write inside will join.
func TestTheAccessLockPrecedesTheCallersReads(t *testing.T) {
	body := funcBody(t, readDBSource(t, "tx.go"), "InTxLockingAccess")
	lock := strings.Index(body, "LockAccessMutationTx")
	call := strings.Index(body, "fn(context.WithValue")
	if lock < 0 || call < 0 || lock > call {
		t.Error("the lock must be taken before the caller's reads, or it serialises the writes and leaves the reads racing")
	}
}

// Every closure-diff path must run inside the lock. A path that computes a
// delta outside it can read a world, be overtaken, and commit a statement about
// neither — which is precisely what the chokepoint lock alone cannot prevent,
// because by then the competing caller has already read and written its source.
func TestEveryClosureDiffRunsUnderTheAccessLock(t *testing.T) {
	// Pure helpers are exempt — they compute, they do not read or write. What
	// matters is the function that owns the read-to-commit window. The set is
	// pinned rather than inferred, so a new delta-computing function fails this
	// test until somebody decides which side of the lock it belongs on.
	pure := map[string]bool{"closureDelta": true, "holderDelta": true}
	wrapped := map[string]bool{
		"CascadeBundleAssignedToUser":  true,
		"CascadeRuleCreated":           true,
		"CascadeBundleRemovedFromUser": true,
		"CascadeBundleDeleted":         true,
		"CascadeRuleUpdated":           true,
		"CascadeRuleDeleted":           true,
		"DeleteDirectGrant":            true,
		"ExpireDirectGrant":            true,
		"PublishBundleVersion":         true,
		"MoveHolders":                  true,
	}
	// Every name in the set is checked directly, not only the ones that happen
	// to call closureDelta themselves. PublishBundleVersion computes its delta
	// through a rehearsal, so a scan for the call site alone would have declared
	// it wrapped while it was not — which is exactly what happened.
	sources := map[string]string{}
	for _, file := range []string{"cascade.go", "role_members.go", "bundle_publish.go"} {
		sources[file] = readServicesSource(t, file)
	}
	found := map[string]bool{}
	for file, src := range sources {
		for fn := range wrapped {
			if !regexp.MustCompile(`(?m)^func ` + fn + `\(`).MatchString(src) {
				continue
			}
			found[fn] = true
			if !strings.Contains(funcBody(t, src, fn), "svcInTxLockingAccess(ctx,") {
				t.Errorf("services/%s: %s must run its reads and its write under the access lock", file, fn)
			}
		}
	}
	for fn := range wrapped {
		if !found[fn] {
			t.Errorf("%s is in the wrapped set and no longer exists — the guard is watching nothing", fn)
		}
	}

	// And nothing new computes a delta outside the set.
	for file, src := range sources {
		for _, fn := range functionsCalling(src, "closureDelta(") {
			if pure[fn] || wrapped[fn] {
				continue
			}
			t.Errorf("services/%s: %s computes a delta and is not in the wrapped set — decide which side of the lock it belongs on", file, fn)
		}
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

func readServicesSource(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "services", name))
	if err != nil {
		t.Fatalf("read services/%s: %v", name, err)
	}
	return string(b)
}

// functionsCalling returns the top-level funcs in src whose body mentions call.
func functionsCalling(src, call string) []string {
	var out []string
	decl := regexp.MustCompile(`(?m)^func (\w+)\(`)
	locs := decl.FindAllStringSubmatchIndex(src, -1)
	for i, loc := range locs {
		end := len(src)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		if strings.Contains(src[loc[0]:end], call) {
			out = append(out, src[loc[2]:loc[3]])
		}
	}
	return out
}
