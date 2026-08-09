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

// The drain is not a cascade, but deleting a ledger row changes what somebody
// effectively holds, and every delta is computed from those rows. A cascade can
// lock, read a grant that is still present, conclude the role it was about to
// add is already covered, and commit that empty delta — while this deletion
// takes the cover away in between, with nobody left who thinks it is missing.
func TestTheDrainsLedgerReconciliationTakesTheAccessLock(t *testing.T) {
	body := funcBody(t, readDBSource(t, "propagations.go"), "ReconcileLedgerOnApplied")
	if !strings.Contains(body, "InTxLockingAccess(ctx,") {
		t.Error("the ledger reconciliation must run under the access lock")
	}
	if strings.Contains(body, "PG.Exec(ctx,") {
		t.Error("the deletes must run on the locked transaction, not on the pool")
	}
	// Taken here rather than around the dispatch: the Zitadel call has already
	// happened, so nothing holds the lock across the network.
	if strings.Contains(body, "dispatch") || strings.Contains(body, "MgmtClient") {
		t.Error("no target call may happen inside this lock")
	}
}

// A rehearsal that runs inside the access lock must not reach the directory:
// in live mode that is Zitadel behind a cache that can miss, and a name nobody
// has looked up yet would hold the one lock every expiry, grant and cascade
// waits on for as long as an unreachable identity provider takes to time out.
func TestNoDirectoryLookupInsideTheAccessLock(t *testing.T) {
	src := readServicesSource(t, "bundle_publish.go")
	for _, fn := range []string{"RehearseBundlePublish", "RehearseMoveHolders"} {
		if strings.Contains(funcBody(t, src, fn), "directory.Default") {
			t.Errorf("%s runs under the lock on the apply path and must not call the directory", fn)
		}
	}
	// Decoration still happens — outside — or the rehearsal renders subject ids.
	if !strings.Contains(funcBody(t, src, "DecoratePlan"), "directory.Default.FindUser") {
		t.Error("DecoratePlan must be where the names come from")
	}
	for _, fn := range []string{"PublishBundleVersion", "MoveHolders"} {
		body := funcBody(t, src, fn)
		lock := strings.Index(body, "svcInTxLockingAccess(ctx,")
		// Both exits decorate: the one that failed still renders a plan, and
		// counting them is what distinguishes "decorated after the lock" from
		// "decorated only on the path nobody looks at". A single call passes an
		// ordering check while the returned plan carries no names at all.
		if n := strings.Count(body, "DecoratePlan(ctx, &plan)"); n != 2 {
			t.Errorf("%s must decorate on both exits from the locked region, found %d", fn, n)
		}
		for _, at := range indicesOf(body, "DecoratePlan(ctx, &plan)") {
			if lock < 0 || at < lock {
				t.Errorf("%s decorates before or inside the locked region", fn)
			}
		}
	}
}

// Onboarding used to insert the assignment directly: the welcome bundle's roles
// were never projected anywhere, and the write changed effective access without
// the lock.
func TestOnboardingGoesThroughTheCascade(t *testing.T) {
	if strings.Contains(readDBSource(t, "bundles.go"), "func AssignBundleToUser") {
		t.Error("the bare assignment has no caller; keeping it invites a second path that projects nothing")
	}
	body := funcBody(t, readServicesSource(t, "onboarding.go"), "TriggerOnboarding")
	if !strings.Contains(body, "svcCascadeWelcomeBundle(ctx,") {
		t.Error("the welcome bundle must be assigned through the cascade, which locks, computes the delta and queues it")
	}
}

func indicesOf(hay, needle string) []int {
	var out []int
	for i := 0; ; {
		j := strings.Index(hay[i:], needle)
		if j < 0 {
			return out
		}
		out = append(out, i+j)
		i += j + len(needle)
	}
}

// `replace` names the roles that SURVIVE, not the ones being taken away. Read
// as removals they would subtract exactly the access being kept, and every
// cascade would queue an add for roles nothing was removing.
func TestQueuedRevocationsReadsReplaceAsItsComplement(t *testing.T) {
	body := funcBody(t, readDBSource(t, "propagations.go"), "QueuedRevocations")
	if regexp.MustCompile(`op_type IN \('revoke', 'replace'\)`).MatchString(body) {
		t.Fatal("replace's role_keys are the surviving set; unioning them with revoke's subtracts the wrong roles")
	}
	if !strings.Contains(body, "o.op_type = 'replace'") || !strings.Contains(body, "NOT (g.zitadel_role_key = ANY(o.role_keys))") {
		t.Error("a queued replace removes the direct-sourced roles its new set omits — the same predicate the reconciliation deletes by")
	}
	// Both branches, counted: a mutation that resolves only one of them leaves
	// the other's predicate intact and passes a containment check.
	if n := strings.Count(body, "status IN ('pending', 'in_flight')"); n != 2 {
		t.Errorf("both branches must count only unresolved rows, found %d", n)
	}
}

// Once a queued revocation is visible to closure computation, a cascade that
// wants the role back queues an add behind the revoke — and that add's enqueue
// has already written the ledger row it needs. Reconciling the older revocation
// must not retract it.
func TestReconciliationDoesNotRetractANewerIntent(t *testing.T) {
	body := funcBody(t, readDBSource(t, "propagations.go"), "ReconcileLedgerOnApplied")
	i := strings.Index(body, "const newerAddExists")
	if i < 0 {
		t.Fatal("could not isolate the newer-add guard")
	}
	j := strings.Index(body[i:], "NOT EXISTS")
	k := strings.Index(body[i+j:], "$5)")
	if j < 0 || k < 0 {
		t.Fatal("the guard fragment is not where it was")
	}
	guard := []string{"", body[i+j : i+j+k+len("$5)")]}
	// Only an add that would ESTABLISH this ledger row protects it. Cascade
	// adds write no direct_role_grants row at all, so treating any queued add
	// as proof would keep a direct grant alive that nothing maintains — and it
	// then reads as coverage forever, so removing the bundle later queues no
	// revoke.
	if !strings.Contains(guard[1], "o.source = d.source") {
		t.Error("the guard must match the source, or a cascade add that writes no ledger row protects one")
	}
	// And newer than the decision being reconciled: an add queued before this
	// revocation is older intent, and the revocation is the later word.
	// By intent order, not by timestamp: created_at is fixed at BEGIN and the
	// access lock is taken after it, so a transaction can start first, block on
	// the lock, and commit second — carrying the earlier timestamp while its
	// decision is the later one.
	if !strings.Contains(guard[1], "o.intent_seq > $5") {
		t.Error("precedence must use the order allocated under the lock, not transaction-start time")
	}
	if strings.Contains(guard[1], "created_at") {
		t.Error("transaction-start time is not the order the decisions were taken in")
	}
	if !strings.Contains(guard[1], "o.op_type = 'add'") ||
		!strings.Contains(guard[1], "o.status IN ('pending', 'in_flight')") {
		t.Error("only an unresolved add is an intent still being established")
	}

	// Both branches use it. A replace narrowing A to B while a later direct add
	// re-establishes A would otherwise delete A's freshly written row, and the
	// add would reach the target with nothing durable behind it.
	if n := strings.Count(body, "newerAddExists"); n < 3 {
		t.Errorf("revoke and replace must both carry the guard, found %d uses of the fragment", n)
	}

	// Everything the reconciliation scopes by comes from the row it is
	// reconciling. Assembled by a caller, the tuple, the source and the moment
	// can describe a set that never existed together — and the ordering
	// comparison is meaningless unless the timestamp is this row's.
	if !strings.Contains(body, "FROM propagation_outbox WHERE id = $1") {
		t.Error("the reconciliation must read its own row rather than trust a caller's tuple")
	}
	// Including the moment. Compared against the clock instead, every add
	// queued before this revocation counts as newer than it.
	if !strings.Contains(body, "COALESCE(source,'direct'), intent_seq") {
		t.Error("the ordering value must be read from the row being reconciled")
	}
}

// The sequence is what makes precedence mean anything. `created_at` defaults to
// NOW(), which is transaction-start time, and the access lock is taken after
// BEGIN — so the row that started first can commit second and still carry the
// earlier timestamp. A default allocated when the INSERT runs is allocated
// under the lock, which is where the order is actually decided.
func TestIntentOrderIsAllocatedUnderTheLock(t *testing.T) {
	up, down := addonMigrationSQL(t)

	if !strings.Contains(up, "CREATE SEQUENCE IF NOT EXISTS propagation_outbox_intent_seq") {
		t.Fatal("intent order needs a sequence, not a timestamp")
	}
	if !strings.Contains(up, "ALTER COLUMN intent_seq SET DEFAULT nextval('propagation_outbox_intent_seq')") {
		t.Error("the value must be allocated by the INSERT, which is what runs under the lock")
	}
	if !strings.Contains(up, "ALTER COLUMN intent_seq SET NOT NULL") {
		t.Error("a row with no intent order cannot be compared against one that has it")
	}
	// Existing rows keep the only order history has, and the sequence resumes
	// past them so a new row never collides with a backfilled one.
	back := strings.Index(up, "row_number() OVER (ORDER BY created_at, id)")
	set := strings.Index(up, "setval('propagation_outbox_intent_seq'")
	def := strings.Index(up, "ALTER COLUMN intent_seq SET DEFAULT")
	if back < 0 || set < 0 || back > set || set > def {
		t.Error("backfill, then advance the sequence past it, then make it the default")
	}

	for _, want := range []string{
		"DROP INDEX IF EXISTS idx_propagation_outbox_intent_seq",
		"ALTER TABLE propagation_outbox DROP COLUMN IF EXISTS intent_seq",
		"DROP SEQUENCE IF EXISTS propagation_outbox_intent_seq",
	} {
		if !strings.Contains(down, want) {
			t.Errorf("the rollback must undo %q", want)
		}
	}
}

// Dispatch follows intent for the same reason precedence does: claiming in
// timestamp order would dispatch a serially-older add after the revoke that
// overtook it, and the target would settle on the wrong one.
func TestTheClaimDispatchesInIntentOrder(t *testing.T) {
	body := funcBody(t, readDBSource(t, "propagations.go"), "ClaimPendingPropagations")
	if !strings.Contains(body, "ORDER BY p.intent_seq") {
		t.Error("the claim must order by intent, not by transaction-start time")
	}
	if strings.Contains(body, "ORDER BY p.created_at") {
		t.Error("transaction-start time is not the order the decisions were taken in")
	}
}

// The drift replay reads the intent ledger directly rather than through the
// effective-access closure, so the subtraction that hides an unresolved
// revocation from every delta does not reach it. The condition lives in its
// write instead — deciding it in the caller would leave the read authoritative
// and a revocation queued between the look and the insert would still be
// overtaken.
func TestTheDriftReplayIsSubordinateToAnUnresolvedRevocation(t *testing.T) {
	body := funcBody(t, readDBSource(t, "propagations.go"), "InsertPendingPropagation")

	if !strings.Contains(body, "InTxLockingAccess(ctx,") {
		t.Error("the replay must be written under the access lock, or its NOT EXISTS is a stale read")
	}
	if !strings.Contains(body, "WHERE NOT EXISTS (") {
		t.Fatal("the subordination must be a condition of the insert")
	}
	if !strings.Contains(body, "o.op_type = 'revoke' AND o.role_keys && $4") {
		t.Error("a queued revocation of any of these roles must suppress the replay")
	}
	// A replace removes what its new set omits, so a role absent from that set
	// is on its way out just as surely as one a revoke names.
	if !strings.Contains(body, "o.op_type = 'replace'") ||
		!strings.Contains(body, "WHERE NOT (rk = ANY(o.role_keys))") {
		t.Error("a queued replace that omits one of these roles must suppress the replay too")
	}
	if !strings.Contains(body, "o.status IN ('pending', 'in_flight')") {
		t.Error("only an unresolved revocation is one that has not happened yet")
	}
	// Refused, not failed: the grant is absent because somebody asked for it to
	// be, and the sweep that noticed has nothing to repair.
	// Asserted on the branch, not on the function: the sentinel is declared
	// just below and funcBody's extent reaches it, so a containment check
	// passes even when nothing returns it.
	if !regexp.MustCompile(`(?s)errors\.Is\(err, pgx\.ErrNoRows\) \{\s*return ErrSupersededByRevocation`).MatchString(body) {
		t.Error("an insert that matched nothing must return the sentinel, not silence")
	}
}
