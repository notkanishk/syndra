package propagation

import (
	"context"
	"errors"
	"testing"

	"syndra/internal/db"
	"syndra/internal/models"
	"syndra/internal/zitadel"
)

// swap sets *dst to v and returns a restore closure.
func swap[T any](dst *T, v T) func() { o := *dst; *dst = v; return func() { *dst = o } }

// statusErr returns a typed status error the classifier reads by code (NOT by
// string), using the same zitadel.StatusError the production client surfaces.
func statusErr(code int) error { return &zitadel.StatusError{Code: code} }

// stubDrainDeps sets every injectable to a safe no-op so Drain runs to
// completion without touching the nil PG pool / nil MgmtClient. Tests override
// only the deps they assert on; t.Cleanup restores the real funcs after.
func stubDrainDeps(t *testing.T) {
	t.Helper()
	for _, restore := range []func(){
		swap(&zitadelReachable, func(context.Context) bool { return true }),
		swap(&claimPending, func(context.Context, string, int) ([]models.PendingPropagation, error) { return nil, nil }),
		swap(&grantIndexHasRole, func(context.Context, string, string, string) (bool, error) { return false, nil }),
		swap(&liveUserGrantRoles, func(context.Context, string, string) (map[string]bool, error) { return map[string]bool{}, nil }),
		swap(&pruneTerminal, func(context.Context, int) (int64, error) { return 0, nil }),
		swap(&awaitingDispatch, func(context.Context, string) ([]string, error) { return nil, nil }),
		swap(&undispatchable, func(context.Context, string, string) (string, error) { return "", nil }),
		swap(&markApplied, func(context.Context, string) error { return nil }),
		swap(&markFailed, func(context.Context, string, string) error { return nil }),
		swap(&requeue, func(context.Context, string, string) (int, error) { return 0, nil }),
		swap(&reconcileLedger, func(context.Context, string) error { return nil }),
		swap(&acquireDrainLock, func(context.Context) (func(), bool, error) { return func() {}, true, nil }),
		swap(&claimOne, func(context.Context, string, string) (*models.PendingPropagation, bool, error) {
			return nil, false, nil
		}),
		swap(&zitadelAddUserGrant, func(context.Context, string, string, []string) error { return nil }),
		swap(&zitadelUpdateUserGrant, func(context.Context, string, string, []string) error { return nil }),
		swap(&zitadelRemoveUserGrant, func(context.Context, string, string) error { return nil }),
	} {
		t.Cleanup(restore)
	}
}

func oneRow(id, op string) func(context.Context, string, int) ([]models.PendingPropagation, error) {
	return func(context.Context, string, int) ([]models.PendingPropagation, error) {
		return []models.PendingPropagation{{ID: id, Target: db.TargetZitadel, OpType: op, UserID: "u", ProjectID: "p", RoleKeys: []string{"r"}}}, nil
	}
}

func TestDrain_AppliedOn2xx(t *testing.T) {
	stubDrainDeps(t)
	claimPending = oneRow("o1", "add")
	var addCalled bool
	zitadelAddUserGrant = func(context.Context, string, string, []string) error { addCalled = true; return nil }
	var appliedID string
	markApplied = func(_ context.Context, id string) error { appliedID = id; return nil }

	res, err := Drain(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !addCalled || appliedID != "o1" || res.Applied != 1 {
		t.Fatalf("want add called + o1 applied, got addCalled=%v applied=%q res=%+v", addCalled, appliedID, res)
	}
}

func TestDrain_HaltsWhenZitadelOffline(t *testing.T) {
	stubDrainDeps(t)
	zitadelReachable = func(context.Context) bool { return false }
	res, err := Drain(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !res.Halted || res.Reason != "zitadel_offline" {
		t.Fatalf("want halted/zitadel_offline, got %+v", res)
	}
}

func TestDrain_AlreadyExistsShortCircuits(t *testing.T) {
	stubDrainDeps(t)
	claimPending = oneRow("o2", "add")
	grantIndexHasRole = func(context.Context, string, string, string) (bool, error) { return true, nil }
	var addCalled bool
	zitadelAddUserGrant = func(context.Context, string, string, []string) error { addCalled = true; return nil }

	res, _ := Drain(context.Background())
	if addCalled {
		t.Fatal("add must be skipped when grant already exists")
	}
	if res.Applied != 1 {
		t.Fatalf("want 1 applied via short-circuit, got %+v", res)
	}
}

func TestDrain_FailedOn4xx_DoesNotHaltOthers(t *testing.T) {
	stubDrainDeps(t)
	claimPending = func(context.Context, string, int) ([]models.PendingPropagation, error) {
		return []models.PendingPropagation{
			{ID: "bad", OpType: "add", UserID: "u", ProjectID: "p", RoleKeys: []string{"r1"}},
			{ID: "ok", OpType: "add", UserID: "u", ProjectID: "p", RoleKeys: []string{"r2"}},
		}, nil
	}
	zitadelAddUserGrant = func(_ context.Context, _, _ string, roles []string) error {
		if roles[0] == "r1" {
			return statusErr(400)
		}
		return nil
	}
	var failedID, appliedID string
	markFailed = func(_ context.Context, id, _ string) error { failedID = id; return nil }
	markApplied = func(_ context.Context, id string) error { appliedID = id; return nil }

	res, _ := Drain(context.Background())
	if failedID != "bad" || appliedID != "ok" || res.Failed != 1 || res.Applied != 1 {
		t.Fatalf("4xx must fail its row but not halt others: %+v", res)
	}
}

func TestDrain_RequeuesOn429(t *testing.T) {
	stubDrainDeps(t)
	claimPending = oneRow("t1", "add")
	zitadelAddUserGrant = func(context.Context, string, string, []string) error { return statusErr(429) }
	var requeued bool
	requeue = func(_ context.Context, _, _ string) (int, error) { requeued = true; return 1, nil }
	markFailed = func(context.Context, string, string) error { t.Fatal("429 must NOT mark failed"); return nil }

	res, _ := Drain(context.Background())
	if !requeued || res.Requeued != 1 {
		t.Fatalf("429 must requeue (transient), got %+v", res)
	}
}

func TestDrain_AppliedOnAlreadyExists409(t *testing.T) {
	stubDrainDeps(t)
	claimPending = oneRow("e1", "add")
	// Index miss + live miss → we DO call Zitadel, which returns 409. That must
	// resolve as applied (idempotent success), never failed.
	zitadelAddUserGrant = func(context.Context, string, string, []string) error { return statusErr(409) }
	var appliedID string
	markApplied = func(_ context.Context, id string) error { appliedID = id; return nil }
	markFailed = func(context.Context, string, string) error { t.Fatal("409 must NOT mark failed"); return nil }

	res, _ := Drain(context.Background())
	if appliedID != "e1" || res.Applied != 1 {
		t.Fatalf("409 AlreadyExists must be idempotent success, got %+v", res)
	}
}

func TestDrain_RequeuesOn503AndTimeout(t *testing.T) {
	for _, code := range []int{503, 408} {
		stubDrainDeps(t)
		claimPending = oneRow("x", "add")
		zitadelAddUserGrant = func(context.Context, string, string, []string) error { return statusErr(code) }
		var requeued bool
		requeue = func(_ context.Context, _, _ string) (int, error) { requeued = true; return 1, nil }
		markFailed = func(context.Context, string, string) error { t.Fatalf("status %d must NOT fail", code); return nil }

		res, _ := Drain(context.Background())
		if !requeued || res.Requeued != 1 {
			t.Fatalf("status %d must requeue (transient), got %+v", code, res)
		}
	}
}

func TestDrain_HaltsWhenRetriesExhausted(t *testing.T) {
	stubDrainDeps(t)
	claimPending = oneRow("loop", "add")
	zitadelAddUserGrant = func(context.Context, string, string, []string) error { return statusErr(500) }
	requeue = func(context.Context, string, string) (int, error) { return maxRetries + 1, nil }

	res, _ := Drain(context.Background())
	if !res.Halted || res.Reason != "max_retries_exceeded" {
		t.Fatalf("exceeding the retry budget must halt the drain, got %+v", res)
	}
}

func TestDrain_PrunesTerminalRowsAtTail(t *testing.T) {
	stubDrainDeps(t)
	var prunedWith int
	pruneTerminal = func(_ context.Context, days int) (int64, error) { prunedWith = days; return 4, nil }

	if _, err := Drain(context.Background()); err != nil {
		t.Fatal(err)
	}
	if prunedWith != retentionDays {
		t.Fatalf("drain must prune with retentionDays=%d, got %d", retentionDays, prunedWith)
	}
}

func TestDrain_RevokeShortCircuitsWhenAbsent(t *testing.T) {
	stubDrainDeps(t)
	claimPending = func(context.Context, string, int) ([]models.PendingPropagation, error) {
		return []models.PendingPropagation{{ID: "rv", OpType: "revoke", UserID: "u", ProjectID: "p", RoleKeys: []string{"r"}, ZitadelGrantID: "g1"}}, nil
	}
	// Live grants don't contain role "r" → nothing to revoke → applied without a call.
	liveUserGrantRoles = func(context.Context, string, string) (map[string]bool, error) { return map[string]bool{}, nil }
	var removeCalled bool
	zitadelRemoveUserGrant = func(context.Context, string, string) error { removeCalled = true; return nil }
	var appliedID string
	markApplied = func(_ context.Context, id string) error { appliedID = id; return nil }

	res, _ := Drain(context.Background())
	if removeCalled {
		t.Fatal("revoke must be skipped when the role is already absent")
	}
	if appliedID != "rv" || res.Applied != 1 {
		t.Fatalf("absent revoke must short-circuit to applied, got %+v", res)
	}
}

// --- Finding 1: a per-row state-persistence failure must not be reported as
// success, and must not halt the rest of the batch. The row is left non-terminal
// (in_flight) so the next drain reclaims it (finding 2). ---

func TestDrain_PersistFailureNotReportedApplied(t *testing.T) {
	stubDrainDeps(t)
	claimPending = oneRow("p1", "add")
	// Zitadel returns 2xx, but recording the applied state fails.
	markApplied = func(context.Context, string) error { return errors.New("db unavailable") }

	res, err := Drain(context.Background())
	if err != nil {
		t.Fatalf("a per-row persistence error must not fail the whole drain: %v", err)
	}
	if res.Applied != 0 {
		t.Fatalf("must NOT count applied when the state write failed, got %+v", res)
	}
	if res.Errored != 1 {
		t.Fatalf("a persistence failure must be counted as errored, got %+v", res)
	}
}

func TestDrain_PersistFailureDoesNotHaltBatch(t *testing.T) {
	stubDrainDeps(t)
	claimPending = func(context.Context, string, int) ([]models.PendingPropagation, error) {
		return []models.PendingPropagation{
			{ID: "p1", OpType: "add", UserID: "u", ProjectID: "p", RoleKeys: []string{"r1"}},
			{ID: "p2", OpType: "add", UserID: "u", ProjectID: "p", RoleKeys: []string{"r2"}},
		}, nil
	}
	markApplied = func(_ context.Context, id string) error {
		if id == "p1" {
			return errors.New("db unavailable")
		}
		return nil
	}

	res, _ := Drain(context.Background())
	if res.Applied != 1 || res.Errored != 1 {
		t.Fatalf("the second row must still apply after the first row's persist error, got %+v", res)
	}
}

func TestDrain_RequeuePersistFailureNotReportedRequeued(t *testing.T) {
	stubDrainDeps(t)
	claimPending = oneRow("t1", "add")
	zitadelAddUserGrant = func(context.Context, string, string, []string) error { return statusErr(503) }
	requeue = func(context.Context, string, string) (int, error) { return 0, errors.New("db unavailable") }

	res, _ := Drain(context.Background())
	if res.Requeued != 0 {
		t.Fatalf("must NOT count requeued when the requeue write failed, got %+v", res)
	}
	if res.Errored != 1 {
		t.Fatalf("a failed requeue must be counted as errored, got %+v", res)
	}
}

// --- Targeted drain: inline "apply now" drains ONLY the triggering row, not the
// global oldest-first batch, so applying one grant never projects unrelated
// queued mutations. ---

func TestDrainOne_ProcessesOnlyTargetRow(t *testing.T) {
	stubDrainDeps(t)
	var claimedID string
	claimOne = func(_ context.Context, _ string, id string) (*models.PendingPropagation, bool, error) {
		claimedID = id
		return &models.PendingPropagation{ID: id, Target: db.TargetZitadel, OpType: "add", UserID: "u", ProjectID: "p", RoleKeys: []string{"r"}}, true, nil
	}
	claimPending = func(context.Context, string, int) ([]models.PendingPropagation, error) {
		t.Fatal("DrainOne must NOT claim the global batch")
		return nil, nil
	}
	var appliedID string
	markApplied = func(_ context.Context, id string) error { appliedID = id; return nil }

	res, err := DrainOne(context.Background(), "ob-x")
	if err != nil {
		t.Fatal(err)
	}
	if claimedID != "ob-x" || appliedID != "ob-x" || res.Applied != 1 {
		t.Fatalf("DrainOne must claim+apply only ob-x, got claimed=%q applied=%q res=%+v", claimedID, appliedID, res)
	}
}

func TestDrainOne_NotFoundIsNoop(t *testing.T) {
	stubDrainDeps(t)
	claimOne = func(context.Context, string, string) (*models.PendingPropagation, bool, error) {
		return nil, false, nil
	}

	res, err := DrainOne(context.Background(), "gone")
	if err != nil {
		t.Fatal(err)
	}
	if res.Applied != 0 || res.Failed != 0 || res.Requeued != 0 || res.Errored != 0 {
		t.Fatalf("DrainOne on a terminal/absent row must be a no-op, got %+v", res)
	}
}

func TestDrainOne_SerializedByLock(t *testing.T) {
	stubDrainDeps(t)
	acquireDrainLock = func(context.Context) (func(), bool, error) { return func() {}, false, nil }
	claimOne = func(context.Context, string, string) (*models.PendingPropagation, bool, error) {
		t.Fatal("DrainOne must not claim when another drain holds the lock")
		return nil, false, nil
	}

	res, _ := DrainOne(context.Background(), "ob-x")
	if !res.Halted || res.Reason != "drain_in_progress" {
		t.Fatalf("DrainOne must respect the drain lock, got %+v", res)
	}
}

func TestDrainOne_HaltsWhenZitadelOffline(t *testing.T) {
	stubDrainDeps(t)
	zitadelReachable = func(context.Context) bool { return false }
	claimOne = func(context.Context, string, string) (*models.PendingPropagation, bool, error) {
		t.Fatal("DrainOne must not claim when Zitadel is offline")
		return nil, false, nil
	}

	res, _ := DrainOne(context.Background(), "ob-x")
	if !res.Halted || res.Reason != "zitadel_offline" {
		t.Fatalf("DrainOne must halt when Zitadel is offline, got %+v", res)
	}
}

// --- Concurrency: drains are serialized by an advisory lock so that in_flight
// reclaim only ever picks up crash-orphaned rows, never rows a live drain is
// mid-dispatch on. ---

func TestDrain_SkippedWhenAnotherDrainHoldsLock(t *testing.T) {
	stubDrainDeps(t)
	acquireDrainLock = func(context.Context) (func(), bool, error) { return func() {}, false, nil }
	var claimed bool
	claimPending = func(context.Context, string, int) ([]models.PendingPropagation, error) {
		claimed = true
		return nil, nil
	}

	res, err := Drain(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !res.Halted || res.Reason != "drain_in_progress" {
		t.Fatalf("a second concurrent drain must halt with drain_in_progress, got %+v", res)
	}
	if claimed {
		t.Fatal("must not claim/dispatch rows when another drain already holds the lock")
	}
}

func TestDrain_ReleasesLockWhenDone(t *testing.T) {
	stubDrainDeps(t)
	var released bool
	acquireDrainLock = func(context.Context) (func(), bool, error) { return func() { released = true }, true, nil }

	if _, err := Drain(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !released {
		t.Fatal("the drain lock must be released when the drain returns")
	}
}

func TestDrain_ReleasesLockOnEarlyHalt(t *testing.T) {
	stubDrainDeps(t)
	zitadelReachable = func(context.Context) bool { return false } // early return before the loop
	var released bool
	acquireDrainLock = func(context.Context) (func(), bool, error) { return func() { released = true }, true, nil }

	res, _ := Drain(context.Background())
	if !res.Halted || res.Reason != "zitadel_offline" {
		t.Fatalf("want zitadel_offline halt, got %+v", res)
	}
	if !released {
		t.Fatal("the drain lock must be released even when the drain halts early")
	}
}

func TestDrain_LockAcquireErrorSurfaces(t *testing.T) {
	stubDrainDeps(t)
	acquireDrainLock = func(context.Context) (func(), bool, error) { return nil, false, errors.New("pool exhausted") }

	if _, err := Drain(context.Background()); err == nil {
		t.Fatal("a lock-acquire error must surface (not silently skip the drain)")
	}
}

// --- Finding 4: replace must reach the EXACT desired role set. A superset in
// Zitadel (a superseded role still present) must NOT short-circuit. ---

func TestDrain_ReplaceDoesNotShortCircuitOnExtraRole(t *testing.T) {
	stubDrainDeps(t)
	claimPending = func(context.Context, string, int) ([]models.PendingPropagation, error) {
		return []models.PendingPropagation{{ID: "rp", OpType: "replace", UserID: "u", ProjectID: "p", RoleKeys: []string{"new"}, ZitadelGrantID: "g1"}}, nil
	}
	// Zitadel currently holds {old,new}; target is {new}. The extra "old" means
	// the desired state is not yet reached — replace must run.
	liveUserGrantRoles = func(context.Context, string, string) (map[string]bool, error) {
		return map[string]bool{"old": true, "new": true}, nil
	}
	var updateCalled bool
	zitadelUpdateUserGrant = func(context.Context, string, string, []string) error { updateCalled = true; return nil }

	res, _ := Drain(context.Background())
	if !updateCalled {
		t.Fatal("replace must call UpdateUserGrant when Zitadel still holds a superseded role")
	}
	if res.Applied != 1 {
		t.Fatalf("replace should apply after the update call, got %+v", res)
	}
}

func TestDrain_ReplaceShortCircuitsOnExactMatch(t *testing.T) {
	stubDrainDeps(t)
	claimPending = func(context.Context, string, int) ([]models.PendingPropagation, error) {
		return []models.PendingPropagation{{ID: "rp2", OpType: "replace", UserID: "u", ProjectID: "p", RoleKeys: []string{"a", "b"}, ZitadelGrantID: "g1"}}, nil
	}
	liveUserGrantRoles = func(context.Context, string, string) (map[string]bool, error) {
		return map[string]bool{"a": true, "b": true}, nil
	}
	var updateCalled bool
	zitadelUpdateUserGrant = func(context.Context, string, string, []string) error { updateCalled = true; return nil }

	res, _ := Drain(context.Background())
	if updateCalled {
		t.Fatal("replace must short-circuit when Zitadel already holds exactly the desired roles")
	}
	if res.Applied != 1 {
		t.Fatalf("exact-match replace should short-circuit to applied, got %+v", res)
	}
}

// --- Finding 3: an applied revoke/replace must reconcile direct_role_grants so
// the ledger stops treating removed roles as expected grants. ---

func TestDrain_RevokeReconcilesLedgerOnApplied(t *testing.T) {
	stubDrainDeps(t)
	claimPending = func(context.Context, string, int) ([]models.PendingPropagation, error) {
		return []models.PendingPropagation{{ID: "rv", OpType: "revoke", UserID: "u", ProjectID: "p", RoleKeys: []string{"r"}, ZitadelGrantID: "g1", Source: "direct"}}, nil
	}
	liveUserGrantRoles = func(context.Context, string, string) (map[string]bool, error) { return map[string]bool{"r": true}, nil }
	var gotID string
	reconcileLedger = func(_ context.Context, outboxID string) error {
		gotID = outboxID
		return nil
	}

	res, _ := Drain(context.Background())
	// The row identifies itself. Everything the reconciliation scopes by — the
	// tuple, the source, the moment — is read from that row, so the drain
	// cannot hand it a set that never existed together.
	if gotID != "rv" {
		t.Fatalf("an applied revoke must reconcile the ledger for its own row, got %q", gotID)
	}
	if res.Applied != 1 {
		t.Fatalf("revoke should be applied, got %+v", res)
	}
}

// TestDrain_CascadeRevokeReconcilesScopedToItsOwnSource is the review-P1 regression: a
// cascade-sourced revoke (source='bundle'|'rule') must reconcile the ledger scoped to ITS OWN
// source, not the operator's 'direct'. Since cascades write no direct_role_grants rows, this
// means a cascade revoke's reconcile deletes nothing — it never touches an operator's row that
// happens to share the (user, project, role) triple. The SQL-level scoping itself
// (WHERE ... AND source=$4) is covered by TestReconcileLedgerOnApplied_RevokeIsSourceScoped in
// db/propagations_migration_test.go (the db package has no live-DB harness); this test only
// verifies the drain plumbs row.Source through, not the operator's assumed 'direct'.
func TestDrain_CascadeRevokeReconcilesScopedToItsOwnSource(t *testing.T) {
	stubDrainDeps(t)
	claimPending = func(context.Context, string, int) ([]models.PendingPropagation, error) {
		return []models.PendingPropagation{{ID: "rv-b", OpType: "revoke", UserID: "u", ProjectID: "p", RoleKeys: []string{"r"}, ZitadelGrantID: "g1", Source: "bundle", SourceRef: "b1"}}, nil
	}
	liveUserGrantRoles = func(context.Context, string, string) (map[string]bool, error) { return map[string]bool{"r": true}, nil }
	var gotID string
	reconcileLedger = func(_ context.Context, outboxID string) error {
		gotID = outboxID
		return nil
	}

	res, _ := Drain(context.Background())
	if gotID != "rv-b" {
		t.Fatalf("a cascade revoke must reconcile its own row, got %q", gotID)
	}
	if res.Applied != 1 {
		t.Fatalf("got %+v", res)
	}
}

func TestDrain_RevokeShortCircuitAlsoReconcilesLedger(t *testing.T) {
	stubDrainDeps(t)
	claimPending = func(context.Context, string, int) ([]models.PendingPropagation, error) {
		return []models.PendingPropagation{{ID: "rv2", OpType: "revoke", UserID: "u", ProjectID: "p", RoleKeys: []string{"r"}, ZitadelGrantID: "g1"}}, nil
	}
	// Already absent in Zitadel → short-circuit. The stale ledger row must still go.
	liveUserGrantRoles = func(context.Context, string, string) (map[string]bool, error) { return map[string]bool{}, nil }
	var reconciled bool
	reconcileLedger = func(context.Context, string) error { reconciled = true; return nil }

	res, _ := Drain(context.Background())
	if !reconciled {
		t.Fatal("a short-circuited revoke (already absent in Zitadel) must still remove the stale ledger row")
	}
	if res.Applied != 1 {
		t.Fatalf("got %+v", res)
	}
}

func TestDrain_AddDoesNotReconcileLedger(t *testing.T) {
	stubDrainDeps(t)
	claimPending = oneRow("ad", "add")
	reconcileLedger = func(context.Context, string) error {
		t.Fatal("add must not delete ledger rows — the enqueue already upserted them")
		return nil
	}

	if _, err := Drain(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestDrain_LedgerReconcileFailureLeavesRowForRetry(t *testing.T) {
	stubDrainDeps(t)
	claimPending = func(context.Context, string, int) ([]models.PendingPropagation, error) {
		return []models.PendingPropagation{{ID: "rv3", OpType: "revoke", UserID: "u", ProjectID: "p", RoleKeys: []string{"r"}, ZitadelGrantID: "g1"}}, nil
	}
	liveUserGrantRoles = func(context.Context, string, string) (map[string]bool, error) { return map[string]bool{"r": true}, nil }
	reconcileLedger = func(context.Context, string) error {
		return errors.New("db unavailable")
	}
	markApplied = func(context.Context, string) error {
		t.Fatal("must not mark applied when the ledger reconcile failed")
		return nil
	}

	res, _ := Drain(context.Background())
	if res.Applied != 0 || res.Errored != 1 {
		t.Fatalf("a reconcile failure must leave the row non-terminal (errored, not applied), got %+v", res)
	}
}

// --- Critical fix: a per-role revoke must never remove the WHOLE grant when
// other roles (from other bundles/rules/direct grants) still live on it. ---

func TestDrain_RevokePartialCallsUpdateWithRemainingRoles(t *testing.T) {
	stubDrainDeps(t)
	claimPending = func(context.Context, string, int) ([]models.PendingPropagation, error) {
		return []models.PendingPropagation{{ID: "rv-partial", OpType: "revoke", UserID: "u", ProjectID: "p", RoleKeys: []string{"r1"}, ZitadelGrantID: "g1"}}, nil
	}
	// The grant currently holds r1 (from this row's source) AND r2 (from some
	// other source) — only r1 may go.
	liveUserGrantRoles = func(context.Context, string, string) (map[string]bool, error) {
		return map[string]bool{"r1": true, "r2": true}, nil
	}
	var removeCalled bool
	var updateRoles []string
	zitadelRemoveUserGrant = func(context.Context, string, string) error { removeCalled = true; return nil }
	zitadelUpdateUserGrant = func(_ context.Context, _, _ string, roles []string) error { updateRoles = roles; return nil }

	res, _ := Drain(context.Background())
	if removeCalled {
		t.Fatal("a partial revoke on a multi-role grant must NOT call RemoveUserGrant — that would strip the surviving role")
	}
	if len(updateRoles) != 1 || updateRoles[0] != "r2" {
		t.Fatalf("want UpdateUserGrant called with remaining=[r2], got %v", updateRoles)
	}
	if res.Applied != 1 {
		t.Fatalf("got %+v", res)
	}
}

func TestDrain_RevokeSoleRoleCallsRemoveUserGrant(t *testing.T) {
	stubDrainDeps(t)
	claimPending = func(context.Context, string, int) ([]models.PendingPropagation, error) {
		return []models.PendingPropagation{{ID: "rv-sole", OpType: "revoke", UserID: "u", ProjectID: "p", RoleKeys: []string{"r1"}, ZitadelGrantID: "g1"}}, nil
	}
	// The grant holds only r1 — nothing survives the revoke, so the whole grant
	// must go (identical to pre-fix behavior for the sole-role case).
	liveUserGrantRoles = func(context.Context, string, string) (map[string]bool, error) {
		return map[string]bool{"r1": true}, nil
	}
	var removeCalled, updateCalled bool
	zitadelRemoveUserGrant = func(context.Context, string, string) error { removeCalled = true; return nil }
	zitadelUpdateUserGrant = func(context.Context, string, string, []string) error { updateCalled = true; return nil }

	res, _ := Drain(context.Background())
	if !removeCalled || updateCalled {
		t.Fatalf("a sole-role revoke must call RemoveUserGrant (not UpdateUserGrant), got remove=%v update=%v", removeCalled, updateCalled)
	}
	if res.Applied != 1 {
		t.Fatalf("got %+v", res)
	}
}

func TestDrain_RevokeLiveLookupErrorRequeuesInsteadOfRemoving(t *testing.T) {
	stubDrainDeps(t)
	claimPending = func(context.Context, string, int) ([]models.PendingPropagation, error) {
		return []models.PendingPropagation{{ID: "rv-err", OpType: "revoke", UserID: "u", ProjectID: "p", RoleKeys: []string{"r1"}, ZitadelGrantID: "g1"}}, nil
	}
	liveUserGrantRoles = func(context.Context, string, string) (map[string]bool, error) {
		return nil, errors.New("zitadel unreachable")
	}
	var removeCalled, updateCalled bool
	zitadelRemoveUserGrant = func(context.Context, string, string) error { removeCalled = true; return nil }
	zitadelUpdateUserGrant = func(context.Context, string, string, []string) error { updateCalled = true; return nil }
	var requeued bool
	requeue = func(_ context.Context, _, _ string) (int, error) { requeued = true; return 1, nil }

	res, _ := Drain(context.Background())
	if removeCalled || updateCalled {
		t.Fatal("a live-lookup error must never fall through to Remove/Update — that risks destroying a surviving role blind")
	}
	if !requeued || res.Requeued != 1 {
		t.Fatalf("a live-lookup error must requeue for retry, got requeued=%v res=%+v", requeued, res)
	}
}

func TestClassifyZitadelError_ByStatus(t *testing.T) {
	cases := []struct {
		code int
		want ackClass
	}{
		{409, ackApplied},
		{429, ackTransient},
		{408, ackTransient},
		{400, ackFailed},
		{401, ackFailed},
		{404, ackFailed},
		{500, ackTransient},
		{503, ackTransient},
	}
	for _, c := range cases {
		if got := classifyZitadelError(statusErr(c.code)); got != c.want {
			t.Errorf("status %d: want class %d, got %d", c.code, c.want, got)
		}
	}
	// A non-status error (network/timeout) has no code → transient.
	if got := classifyZitadelError(context.DeadlineExceeded); got != ackTransient {
		t.Errorf("non-status error must be transient, got %d", got)
	}
}

// P1 — a row terminated while its dispatch was out is left terminal, and is
// counted as neither a success nor a failure.
//
// Deregistration abandons a target's unresolved rows. If that lands while the
// drain's request is in the air, the settle that follows is no longer this
// drain's to make: the row already reached a terminal state, legitimately, and
// the drain must not overwrite it or claim the outcome it was about to record.
func TestDrain_AbandonedRowIsLeftTerminal(t *testing.T) {
	cases := []struct {
		name    string
		arrange func()
		assert  func(t *testing.T, res DrainResult)
	}{
		{
			name:    "while recording success",
			arrange: func() { markApplied = func(context.Context, string) error { return db.ErrPropagationNotInFlight } },
			assert: func(t *testing.T, res DrainResult) {
				if res.Applied != 0 {
					t.Errorf("counted an abandoned row as applied: %+v", res)
				}
			},
		},
		{
			name: "while recording failure",
			arrange: func() {
				markFailed = func(context.Context, string, string) error { return db.ErrPropagationNotInFlight }
				zitadelAddUserGrant = func(context.Context, string, string, []string) error { return statusErr(400) }
			},
			assert: func(t *testing.T, res DrainResult) {
				if res.Failed != 0 {
					t.Errorf("counted an abandoned row as failed: %+v", res)
				}
			},
		},
		{
			name: "while requeueing",
			arrange: func() {
				requeue = func(context.Context, string, string) (int, error) { return 0, db.ErrPropagationNotInFlight }
				zitadelAddUserGrant = func(context.Context, string, string, []string) error { return statusErr(429) }
			},
			assert: func(t *testing.T, res DrainResult) {
				if res.Requeued != 0 {
					t.Errorf("counted an abandoned row as requeued: %+v", res)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stubDrainDeps(t)
			claimPending = oneRow("p1", "add")
			tc.arrange()

			res, err := Drain(context.Background())
			if err != nil {
				t.Fatalf("Drain: %v", err)
			}
			if res.Abandoned != 1 {
				t.Errorf("the row was not recorded as abandoned: %+v", res)
			}
			// Not an errored row either: nothing failed to persist. Counting it
			// as a persistence error would send an operator looking for a
			// database problem that is not there, and — for the requeue — the
			// old code would have put the row back to `pending` on a target
			// that no longer exists.
			if res.Errored != 0 {
				t.Errorf("an abandoned row was reported as a persistence failure: %+v", res)
			}
			if res.Halted {
				t.Errorf("an abandoned row halted the drain: %+v", res)
			}
			tc.assert(t, res)
		})
	}
}

// 1.10, 1.11 — a drain carries one target's dispatcher, so it claims one
// target's rows. Before this, a drain would claim a TrueNAS row and push it
// through the Zitadel path, where it has no project and no roles to send.
func TestDrain_ClaimsOnlyItsOwnTarget(t *testing.T) {
	stubDrainDeps(t)
	var asked []string
	claimPending = func(_ context.Context, target string, _ int) ([]models.PendingPropagation, error) {
		asked = append(asked, target)
		return nil, nil
	}

	if _, err := Drain(context.Background()); err != nil {
		t.Fatalf("Drain: %v", err)
	}
	if len(asked) != 1 || asked[0] != db.TargetZitadel {
		t.Fatalf("the drain claimed %v, want exactly one pass for %q", asked, db.TargetZitadel)
	}
}

// 1.11 — and it says what it left alone. A pass that dispatched one target's
// work while another's waits is, from the outside, indistinguishable from a
// pass with nothing left to do.
func TestDrain_ReportsTargetsItCouldNotDispatch(t *testing.T) {
	stubDrainDeps(t)
	awaitingDispatch = func(_ context.Context, drained string) ([]string, error) {
		if drained != db.TargetZitadel {
			t.Errorf("awaiting was asked to exclude %q, want the target just drained", drained)
		}
		return []string{"truenas"}, nil
	}

	res, err := Drain(context.Background())
	if err != nil {
		t.Fatalf("Drain: %v", err)
	}
	if len(res.Awaiting) != 1 || res.Awaiting[0] != "truenas" {
		t.Fatalf("awaiting = %v, want the undispatched target named", res.Awaiting)
	}
}

// 1.11 — and failing to list them is not a failed drain. The work it did
// dispatch still happened; refusing to report the result would lose that.
func TestDrain_SurvivesAFailedAwaitingLookup(t *testing.T) {
	stubDrainDeps(t)
	claimPending = oneRow("p1", "add")
	awaitingDispatch = func(context.Context, string) ([]string, error) {
		return nil, errors.New("db unavailable")
	}

	res, err := Drain(context.Background())
	if err != nil {
		t.Fatalf("a failed awaiting lookup must not fail the drain: %v", err)
	}
	if res.Applied != 1 {
		t.Errorf("the row it did dispatch was lost: %+v", res)
	}
}

// 1.10, P1 — the inline apply path never claims a row it cannot dispatch, so
// there is nothing to put back.
//
// Claiming first and releasing after was the earlier shape and it cost twice:
// the release spent a retry and recorded a dispatch failure for a dispatch that
// never happened, so a handful of targeted applies would exhaust an add-on
// row's budget before its dispatcher existed — and its first real transient
// response would then halt it immediately.
func TestDrainOne_NeverClaimsARowForAnotherTarget(t *testing.T) {
	stubDrainDeps(t)
	var askedFor string
	claimOne = func(_ context.Context, target, _ string) (*models.PendingPropagation, bool, error) {
		askedFor = target
		return nil, false, nil // the claim itself declines: wrong target
	}
	undispatchable = func(_ context.Context, dispatcher, id string) (string, error) {
		if dispatcher != db.TargetZitadel || id != "ob-1" {
			t.Errorf("explained the wrong row: dispatcher=%q id=%q", dispatcher, id)
		}
		return "truenas", nil
	}
	var dispatched, requeued bool
	zitadelAddUserGrant = func(context.Context, string, string, []string) error { dispatched = true; return nil }
	requeue = func(context.Context, string, string) (int, error) { requeued = true; return 1, nil }

	res, err := DrainOne(context.Background(), "ob-1")
	if err != nil {
		t.Fatalf("DrainOne: %v", err)
	}
	if askedFor != db.TargetZitadel {
		t.Errorf("the claim was asked for target %q, want the one this drain dispatches", askedFor)
	}
	if dispatched {
		t.Error("a row for another target was dispatched through the Zitadel path")
	}
	if requeued {
		t.Error("a retry was spent releasing a row that was never claimed and never dispatched")
	}
	if res.Applied != 0 || res.Failed != 0 || res.Requeued != 0 {
		t.Errorf("an undispatchable row was counted as an outcome: %+v", res)
	}
	if len(res.Awaiting) != 1 || res.Awaiting[0] != "truenas" {
		t.Errorf("awaiting = %v, want the target that has no dispatcher yet", res.Awaiting)
	}
}

// 1.10 — and explaining is diagnostic. A row that is simply gone or already
// terminal says nothing, and a failure to explain must not become a failure.
func TestDrainOne_SaysNothingAboutARowThatIsSimplyGone(t *testing.T) {
	stubDrainDeps(t)
	claimOne = func(context.Context, string, string) (*models.PendingPropagation, bool, error) {
		return nil, false, nil
	}
	undispatchable = func(context.Context, string, string) (string, error) {
		return "", errors.New("db unavailable")
	}

	res, err := DrainOne(context.Background(), "ob-1")
	if err != nil {
		t.Fatalf("a failed explanation must not fail the apply: %v", err)
	}
	if len(res.Awaiting) != 0 {
		t.Errorf("awaiting = %v, want nothing said", res.Awaiting)
	}
}

// P1 — the batch path shares the scope. It hands whatever it claims to the
// Zitadel dispatcher, which would mark an add-on entitlement (`op_type=apply`)
// terminally failed as an unknown operation — with no way back from `failed`.
func TestDrainBatch_NeverClaimsARowForAnotherTarget(t *testing.T) {
	stubDrainDeps(t)
	var targets []string
	claimOne = func(_ context.Context, target, _ string) (*models.PendingPropagation, bool, error) {
		targets = append(targets, target)
		return nil, false, nil
	}
	undispatchable = func(context.Context, string, string) (string, error) { return "truenas", nil }
	var dispatched bool
	zitadelAddUserGrant = func(context.Context, string, string, []string) error { dispatched = true; return nil }

	res, err := DrainBatch(context.Background(), []string{"a", "b"})
	if err != nil {
		t.Fatalf("DrainBatch: %v", err)
	}
	for _, target := range targets {
		if target != db.TargetZitadel {
			t.Errorf("the batch claim asked for %q", target)
		}
	}
	if dispatched || res.Failed != 0 {
		t.Errorf("an add-on row reached the Zitadel dispatcher: dispatched=%v res=%+v", dispatched, res)
	}
	// Named once, not once per row: two rows on one target are one thing the
	// operator has to do something about.
	if len(res.Awaiting) != 1 || res.Awaiting[0] != "truenas" {
		t.Errorf("awaiting = %v, want the undispatchable target named once", res.Awaiting)
	}
}
