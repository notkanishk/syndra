package drift

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"syndra/internal/db"
	"syndra/internal/models"
	"syndra/internal/zitadel"
)

func swap[T any](dst *T, v T) func() { o := *dst; *dst = v; return func() { *dst = o } }

// stubSweep sets safe no-op defaults; each test overrides only what it asserts.
func stubSweep(t *testing.T) {
	t.Cleanup(swap(&zitadelReachable, func(context.Context) bool { return true }))
	t.Cleanup(swap(&svcAllDirectGrants, func(context.Context) ([]models.DirectGrant, error) { return nil, nil }))
	t.Cleanup(swap(&svcGetActiveMappingRules, func(context.Context) ([]models.MappingRule, error) { return nil, nil }))
	t.Cleanup(swap(&svcGetExclusions, func(context.Context, string) ([]models.ExternalGrantExclusion, error) { return nil, nil }))
	t.Cleanup(swap(&zitadelListAllGrants, func(context.Context, zitadel.SearchParams) (*zitadel.SearchResult[zitadel.UserGrant], error) {
		return &zitadel.SearchResult[zitadel.UserGrant]{}, nil
	}))
	t.Cleanup(swap(&upsertDriftItem, func(context.Context, string, string, string, []string, string, string, string) (string, bool, error) {
		return "d1", true, nil
	}))
	t.Cleanup(swap(&pendingOutboxAddExists, func(context.Context, string, string, string, string) (bool, error) { return false, nil }))
	t.Cleanup(swap(&insertPending, func(context.Context, string, string, string, []string, string, string, string, string) (string, error) {
		return "o1", nil
	}))
	t.Cleanup(swap(&markUnreconciled, func(_ context.Context, target, reason string) (db.TargetReconciliation, error) {
		since := time.Unix(1_760_000_000, 0)
		return db.TargetReconciliation{Target: target, UnreconciledSince: &since, UnreconciledReason: reason}, nil
	}))
	t.Cleanup(swap(&markReconciled, func(_ context.Context, target string) (db.TargetReconciliation, error) {
		read := time.Unix(1_770_000_000, 0)
		return db.TargetReconciliation{Target: target, LastCurrentReadAt: &read}, nil
	}))
}

func TestSweep_UnexplainedZitadelGrantBecomesDrift(t *testing.T) {
	stubSweep(t)
	defer swap(&zitadelListAllGrants, func(context.Context, zitadel.SearchParams) (*zitadel.SearchResult[zitadel.UserGrant], error) {
		return &zitadel.SearchResult[zitadel.UserGrant]{
			Items: []zitadel.UserGrant{{ID: "g1", UserID: "u1", ProjectID: "p1", RoleKeys: []string{"viewer"}}},
			Total: 1,
		}, nil
	})()
	var driftType string
	defer swap(&upsertDriftItem, func(_ context.Context, _, _, _ string, _ []string, _, _, dtype string) (string, bool, error) {
		driftType = dtype
		return "d1", true, nil
	})()

	res, err := Sweep(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if driftType != "target_only" || res.DriftItemsCreated != 1 {
		t.Fatalf("unexplained zitadel grant must create a target_only drift item, got type=%q res=%+v", driftType, res)
	}
}

func TestSweep_RuleDerivedGrantIsNotDrift(t *testing.T) {
	stubSweep(t)
	defer swap(&svcAllDirectGrants, func(context.Context) ([]models.DirectGrant, error) {
		return []models.DirectGrant{{UserID: "u1", ProjectID: "p1", RoleKey: "member"}}, nil
	})()
	defer swap(&svcGetActiveMappingRules, func(context.Context) ([]models.MappingRule, error) {
		return []models.MappingRule{{SourceProject: "p1", SourceRole: "member", TargetProject: "p2", TargetRole: "contributor"}}, nil
	})()
	defer swap(&zitadelListAllGrants, func(context.Context, zitadel.SearchParams) (*zitadel.SearchResult[zitadel.UserGrant], error) {
		return &zitadel.SearchResult[zitadel.UserGrant]{
			Items: []zitadel.UserGrant{
				{ID: "g1", UserID: "u1", ProjectID: "p1", RoleKeys: []string{"member"}},
				{ID: "g2", UserID: "u1", ProjectID: "p2", RoleKeys: []string{"contributor"}},
			}, Total: 2,
		}, nil
	})()
	var created int
	defer swap(&upsertDriftItem, func(context.Context, string, string, string, []string, string, string, string) (string, bool, error) {
		created++
		return "d", true, nil
	})()

	if _, err := Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}
	if created != 0 {
		t.Fatalf("rule-derived + source grants are both explained; no drift expected, got %d", created)
	}
}

func TestSweep_SyndraOnlyDirectGrantReEnqueues(t *testing.T) {
	stubSweep(t)
	// Syndra expects u1/p1/viewer; Zitadel has nothing → re-enqueue (missed-webhook replay).
	defer swap(&svcAllDirectGrants, func(context.Context) ([]models.DirectGrant, error) {
		return []models.DirectGrant{{UserID: "u1", ProjectID: "p1", RoleKey: "viewer"}}, nil
	})()
	var opType string
	defer swap(&insertPending, func(_ context.Context, ot, _, _ string, _ []string, _, _, _, _ string) (string, error) {
		opType = ot
		return "o1", nil
	})()

	res, err := Sweep(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if opType != "add" || res.ReEnqueued != 1 {
		t.Fatalf("syndra_only direct grant must re-enqueue an add, got op=%q res=%+v", opType, res)
	}
}

func TestSweep_SyndraOnlySkipsReEnqueueWhenPendingOutboxAdd(t *testing.T) {
	stubSweep(t)
	defer swap(&svcAllDirectGrants, func(context.Context) ([]models.DirectGrant, error) {
		return []models.DirectGrant{{UserID: "u1", ProjectID: "p1", RoleKey: "viewer"}}, nil
	})()
	defer swap(&pendingOutboxAddExists, func(context.Context, string, string, string, string) (bool, error) { return true, nil })()
	var reEnqueued bool
	defer swap(&insertPending, func(context.Context, string, string, string, []string, string, string, string, string) (string, error) {
		reEnqueued = true
		return "o1", nil
	})()

	res, _ := Sweep(context.Background())
	if reEnqueued || res.ReEnqueued != 0 {
		t.Fatalf("a triple with an undrained outbox add must not auto-re-enqueue a duplicate, res=%+v", res)
	}
}

func TestSweep_HaltsWhenZitadelOffline(t *testing.T) {
	stubSweep(t)
	defer swap(&zitadelReachable, func(context.Context) bool { return false })()
	res, err := Sweep(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !res.Halted || res.Reason != "zitadel_offline" {
		t.Fatalf("offline sweep must halt cleanly, got %+v", res)
	}
}

func TestSweep_ExcludedGrantIsNotDrift(t *testing.T) {
	stubSweep(t)
	defer swap(&svcGetExclusions, func(_ context.Context, target string) ([]models.ExternalGrantExclusion, error) {
		return []models.ExternalGrantExclusion{{Target: target, UserID: "u1", ProjectID: "p1", RoleKey: "viewer"}}, nil
	})()
	defer swap(&zitadelListAllGrants, func(context.Context, zitadel.SearchParams) (*zitadel.SearchResult[zitadel.UserGrant], error) {
		return &zitadel.SearchResult[zitadel.UserGrant]{
			Items: []zitadel.UserGrant{{ID: "g1", UserID: "u1", ProjectID: "p1", RoleKeys: []string{"viewer"}}}, Total: 1,
		}, nil
	})()
	var created int
	defer swap(&upsertDriftItem, func(context.Context, string, string, string, []string, string, string, string) (string, bool, error) {
		created++
		return "d", true, nil
	})()

	if _, err := Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}
	if created != 0 {
		t.Fatalf("marked-external triple must not drift, got %d", created)
	}
}

// 1.13 — every write the sweep makes names the target it swept, and the result
// says so too. Without this the findings of two sweeps are indistinguishable
// once a second target exists, and the pending-dedupe index cannot tell a
// re-detection from a new finding.
func TestSweep_EveryWriteNamesTheTargetItSwept(t *testing.T) {
	stubSweep(t)
	defer swap(&svcAllDirectGrants, func(context.Context) ([]models.DirectGrant, error) {
		return []models.DirectGrant{{UserID: "u2", ProjectID: "p2", RoleKey: "gone"}}, nil
	})()
	defer swap(&zitadelListAllGrants, func(context.Context, zitadel.SearchParams) (*zitadel.SearchResult[zitadel.UserGrant], error) {
		return &zitadel.SearchResult[zitadel.UserGrant]{
			Items: []zitadel.UserGrant{{ID: "g1", UserID: "u1", ProjectID: "p1", RoleKeys: []string{"viewer"}}}, Total: 1,
		}, nil
	})()

	var upsertTarget, exclusionTarget, queuedTarget string
	defer swap(&upsertDriftItem, func(_ context.Context, tgt, _, _ string, _ []string, _, _, _ string) (string, bool, error) {
		upsertTarget = tgt
		return "d1", true, nil
	})()
	defer swap(&svcGetExclusions, func(_ context.Context, tgt string) ([]models.ExternalGrantExclusion, error) {
		exclusionTarget = tgt
		return nil, nil
	})()
	defer swap(&pendingOutboxAddExists, func(_ context.Context, tgt, _, _, _ string) (bool, error) {
		queuedTarget = tgt
		return false, nil
	})()

	res, err := Sweep(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for name, got := range map[string]string{
		"drift finding":     upsertTarget,
		"exclusion read":    exclusionTarget,
		"queued-work check": queuedTarget,
		"result":            res.Target,
	} {
		if got != "zitadel" {
			t.Errorf("%s must name zitadel, got %q", name, got)
		}
	}
}

// A halted sweep still says what it failed to reconcile. "Nothing to report"
// about an unnamed target reads as a clean bill of health for all of them.
func TestSweep_HaltedResultStillNamesItsTarget(t *testing.T) {
	stubSweep(t)
	defer swap(&zitadelReachable, func(context.Context) bool { return false })()
	res, _ := Sweep(context.Background())
	if res.Target != "zitadel" {
		t.Fatalf("a halted sweep must still name its target, got %+v", res)
	}
}

// The scope lives in the comparison, not only in the read. Handed an exclusion
// set that spans targets — which is what an unscoped read would return — the
// sweep must still raise the Zitadel grant it cannot explain.
func TestSweep_ExclusionOnAnotherTargetDoesNotSuppressDrift(t *testing.T) {
	stubSweep(t)
	defer swap(&svcGetExclusions, func(context.Context, string) ([]models.ExternalGrantExclusion, error) {
		return []models.ExternalGrantExclusion{{Target: "truenas", UserID: "u1", ProjectID: "p1", RoleKey: "viewer"}}, nil
	})()
	defer swap(&zitadelListAllGrants, func(context.Context, zitadel.SearchParams) (*zitadel.SearchResult[zitadel.UserGrant], error) {
		return &zitadel.SearchResult[zitadel.UserGrant]{
			Items: []zitadel.UserGrant{{ID: "g1", UserID: "u1", ProjectID: "p1", RoleKeys: []string{"viewer"}}}, Total: 1,
		}, nil
	})()
	var created int
	defer swap(&upsertDriftItem, func(context.Context, string, string, string, []string, string, string, string) (string, bool, error) {
		created++
		return "d", true, nil
	})()

	if _, err := Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}
	if created != 1 {
		t.Fatalf("an exclusion recorded against another target must not silence this one; drift rows created = %d", created)
	}
}

// 1.15 — an outage produces no findings and does record the target as
// unreconciled, with the age the operator is owed. Silence would read as "no
// drift", which is the opposite of what happened.
func TestSweep_AnOutageRecordsAnUnreconciledTargetAndFindsNothing(t *testing.T) {
	stubSweep(t)
	defer swap(&zitadelReachable, func(context.Context) bool { return false })()
	// Syndra expects a grant Zitadel is not answering about. Neither half of
	// the diff may run: one would invent drift, the other would replay it.
	defer swap(&svcAllDirectGrants, func(context.Context) ([]models.DirectGrant, error) {
		t.Fatal("an unreachable target must not be diffed at all")
		return nil, nil
	})()
	defer swap(&upsertDriftItem, func(context.Context, string, string, string, []string, string, string, string) (string, bool, error) {
		t.Fatal("an outage must not raise drift")
		return "", false, nil
	})()
	defer swap(&insertPending, func(context.Context, string, string, string, []string, string, string, string, string) (string, error) {
		t.Fatal("an outage must not re-enqueue")
		return "", nil
	})()

	lastRead := time.Unix(1_759_000_000, 0)
	since := time.Unix(1_759_900_000, 0)
	var gotTarget, gotReason string
	defer swap(&markUnreconciled, func(_ context.Context, target, reason string) (db.TargetReconciliation, error) {
		gotTarget, gotReason = target, reason
		return db.TargetReconciliation{Target: target, LastCurrentReadAt: &lastRead,
			UnreconciledSince: &since, UnreconciledReason: reason}, nil
	})()
	defer swap(&markReconciled, func(context.Context, string) (db.TargetReconciliation, error) {
		t.Fatal("an outage must not record a current read")
		return db.TargetReconciliation{}, nil
	})()

	res, err := Sweep(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !res.Halted || res.DriftItemsCreated != 0 || res.ReEnqueued != 0 {
		t.Fatalf("an outage must produce no findings, got %+v", res)
	}
	if gotTarget != "zitadel" || gotReason != db.UnreconciledUnreachable {
		t.Fatalf("the unreachable target must be recorded as such, got %q/%q", gotTarget, gotReason)
	}
	if res.Reconciliation == nil || !res.Reconciliation.Unreconciled() {
		t.Fatalf("the result must carry the unreconciled record, got %+v", res.Reconciliation)
	}
	// The age of the last current read is the whole point: "we last saw this
	// target on Tuesday" is what an operator decides on.
	if res.Reconciliation.LastCurrentReadAt == nil || !res.Reconciliation.LastCurrentReadAt.Equal(lastRead) {
		t.Fatalf("the result must carry the last current read, got %v", res.Reconciliation.LastCurrentReadAt)
	}
}

// Reconciliation resumes on return: the sweep diffs the current read, and the
// unreconciled period ends with the same write that records the read.
func TestSweep_ResumesOnReturn(t *testing.T) {
	stubSweep(t)
	defer swap(&zitadelListAllGrants, func(context.Context, zitadel.SearchParams) (*zitadel.SearchResult[zitadel.UserGrant], error) {
		return &zitadel.SearchResult[zitadel.UserGrant]{
			Items: []zitadel.UserGrant{{ID: "g1", UserID: "u1", ProjectID: "p1", RoleKeys: []string{"viewer"}}}, Total: 1,
		}, nil
	})()
	var created int
	defer swap(&upsertDriftItem, func(context.Context, string, string, string, []string, string, string, string) (string, bool, error) {
		created++
		return "d1", true, nil
	})()
	defer swap(&markUnreconciled, func(_ context.Context, _, reason string) (db.TargetReconciliation, error) {
		t.Fatalf("a current, complete read must not be recorded as unreconciled (%s)", reason)
		return db.TargetReconciliation{}, nil
	})()
	read := time.Unix(1_770_000_000, 0)
	defer swap(&markReconciled, func(_ context.Context, target string) (db.TargetReconciliation, error) {
		return db.TargetReconciliation{Target: target, LastCurrentReadAt: &read}, nil
	})()

	res, err := Sweep(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// A change made during the outage is classified on its own merits — as the
	// unexplained grant it is, not as an outage artefact.
	if created != 1 {
		t.Fatalf("a current read must be diffed, drift created = %d", created)
	}
	if res.Reconciliation == nil || res.Reconciliation.Unreconciled() {
		t.Fatalf("returning must end the unreconciled period, got %+v", res.Reconciliation)
	}
	if res.Reconciliation.LastCurrentReadAt == nil || !res.Reconciliation.LastCurrentReadAt.Equal(read) {
		t.Fatalf("the result must carry the new current read, got %v", res.Reconciliation.LastCurrentReadAt)
	}
}

// A capped read has seen everything it reports and nothing about the rest.
// Concluding absence from it would re-enqueue an `add` for every direct grant
// beyond the cap — grants that already exist.
func TestSweep_ATruncatedReadConcludesNoAbsence(t *testing.T) {
	stubSweep(t)
	defer swap(&svcAllDirectGrants, func(context.Context) ([]models.DirectGrant, error) {
		return []models.DirectGrant{{UserID: "beyond-the-cap", ProjectID: "p9", RoleKey: "viewer"}}, nil
	})()
	// Total exceeds what the page returns and the cap is reached, so the fetch
	// reports truncation.
	defer swap(&zitadelListAllGrants, func(_ context.Context, p zitadel.SearchParams) (*zitadel.SearchResult[zitadel.UserGrant], error) {
		items := make([]zitadel.UserGrant, driftSafetyCap)
		for i := range items {
			items[i] = zitadel.UserGrant{ID: "g", UserID: "u1", ProjectID: "p1", RoleKeys: []string{"viewer"}}
		}
		return &zitadel.SearchResult[zitadel.UserGrant]{Items: items, Total: driftSafetyCap * 2}, nil
	})()
	defer swap(&insertPending, func(context.Context, string, string, string, []string, string, string, string, string) (string, error) {
		t.Fatal("a capped read cannot observe an absence, so it must not replay one")
		return "", nil
	})()
	var reason string
	defer swap(&markUnreconciled, func(_ context.Context, target, r string) (db.TargetReconciliation, error) {
		reason = r
		return db.TargetReconciliation{Target: target, UnreconciledReason: r}, nil
	})()
	defer swap(&markReconciled, func(context.Context, string) (db.TargetReconciliation, error) {
		t.Fatal("a truncated read is not a read Syndra can stand behind")
		return db.TargetReconciliation{}, nil
	})()

	res, err := Sweep(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !res.Truncated || res.ReEnqueued != 0 {
		t.Fatalf("a truncated sweep must conclude no absence, got %+v", res)
	}
	if reason != db.UnreconciledTruncated {
		t.Fatalf("the cap must be recorded as the reason, got %q", reason)
	}
	// The half that concludes from what it SAW still runs: those grants were
	// observed, and suppressing them would lose real findings for the same
	// reason the other half is suppressed.
	if res.DriftItemsCreated == 0 {
		t.Error("grants actually seen must still be classified")
	}
}

// The record is a report about the pass, not the pass itself. Losing it must
// not lose the findings.
func TestSweep_AFailedCurrencyRecordDoesNotDiscardTheWork(t *testing.T) {
	stubSweep(t)
	defer swap(&zitadelListAllGrants, func(context.Context, zitadel.SearchParams) (*zitadel.SearchResult[zitadel.UserGrant], error) {
		return &zitadel.SearchResult[zitadel.UserGrant]{
			Items: []zitadel.UserGrant{{ID: "g1", UserID: "u1", ProjectID: "p1", RoleKeys: []string{"viewer"}}}, Total: 1,
		}, nil
	})()
	defer swap(&markReconciled, func(context.Context, string) (db.TargetReconciliation, error) {
		return db.TargetReconciliation{}, errors.New("database unreachable")
	})()

	res, err := Sweep(context.Background())
	if err != nil {
		t.Fatalf("a failed currency record must not fail the sweep: %v", err)
	}
	if res.DriftItemsCreated != 1 {
		t.Fatalf("the findings the pass produced must survive, got %+v", res)
	}
	if res.Reconciliation != nil {
		t.Error("an unwritten record must be absent rather than invented — Syndra cannot say how current its picture is if it could not record it")
	}
}

// The reachability pre-flight is a nil check on the client, so passing it is
// not evidence the target answers. A read that fails is the outage — on the
// first page or a later one — and must be recorded as one, or the row left
// behind keeps reporting the last current read for the whole outage.
func TestSweep_AFailedReadIsAnOutageAndSaysWhichKind(t *testing.T) {
	for _, tc := range []struct {
		name       string
		wantReason string
		wantRecord string
		pages      func(int) (*zitadel.SearchResult[zitadel.UserGrant], error)
	}{
		{"first page", "zitadel_unreachable", db.UnreconciledUnreachable, func(int) (*zitadel.SearchResult[zitadel.UserGrant], error) {
			return nil, errors.New("dial tcp: connection refused")
		}},
		// Zitadel answered. The network is fine and the host is up; what is
		// broken is a credential. Reported as unreachable it would look like
		// weather — something to wait out rather than repair.
		{"an answered 401", "zitadel_read_refused", db.UnreconciledReadRefused, func(int) (*zitadel.SearchResult[zitadel.UserGrant], error) {
			return nil, &zitadel.StatusError{Code: 401, Message: "invalid token"}
		}},
		{"an answered 403 mid-pagination", "zitadel_read_refused", db.UnreconciledReadRefused, func(call int) (*zitadel.SearchResult[zitadel.UserGrant], error) {
			if call > 1 {
				return nil, &zitadel.StatusError{Code: 403, Message: "missing permission"}
			}
			items := make([]zitadel.UserGrant, zitadelPageSize)
			for i := range items {
				items[i] = zitadel.UserGrant{ID: "g", UserID: "u1", ProjectID: "p1", RoleKeys: []string{"viewer"}}
			}
			return &zitadel.SearchResult[zitadel.UserGrant]{Items: items, Total: zitadelPageSize * 4}, nil
		}},
		// The shape a revoked machine key actually arrives in: the token
		// exchange answers 401 and doRequest wraps it. Asserting on a bare
		// StatusError would have passed while this real path did not.
		{"a revoked machine key", "zitadel_read_refused", db.UnreconciledReadRefused, func(int) (*zitadel.SearchResult[zitadel.UserGrant], error) {
			return nil, fmt.Errorf("obtain access token: %w",
				&zitadel.StatusError{Code: 401, Message: `{"error":"invalid_client"}`})
		}},
		{"a later page", "zitadel_unreachable", db.UnreconciledUnreachable, func(call int) (*zitadel.SearchResult[zitadel.UserGrant], error) {
			if call > 1 {
				return nil, errors.New("502 from the gateway mid-pagination")
			}
			items := make([]zitadel.UserGrant, zitadelPageSize)
			for i := range items {
				items[i] = zitadel.UserGrant{ID: "g", UserID: "u1", ProjectID: "p1", RoleKeys: []string{"viewer"}}
			}
			return &zitadel.SearchResult[zitadel.UserGrant]{Items: items, Total: zitadelPageSize * 4}, nil
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stubSweep(t)
			// The pre-flight passes: the client is configured. Only the read
			// knows the target is not answering.
			defer swap(&zitadelReachable, func(context.Context) bool { return true })()
			defer swap(&svcAllDirectGrants, func(context.Context) ([]models.DirectGrant, error) {
				return []models.DirectGrant{{UserID: "u9", ProjectID: "p9", RoleKey: "viewer"}}, nil
			})()
			calls := 0
			defer swap(&zitadelListAllGrants, func(context.Context, zitadel.SearchParams) (*zitadel.SearchResult[zitadel.UserGrant], error) {
				calls++
				return tc.pages(calls)
			})()
			defer swap(&upsertDriftItem, func(context.Context, string, string, string, []string, string, string, string) (string, bool, error) {
				t.Fatal("a failed read must not be diffed — a partly-read list is unseen, not absent")
				return "", false, nil
			})()
			defer swap(&insertPending, func(context.Context, string, string, string, []string, string, string, string, string) (string, error) {
				t.Fatal("a failed read must not replay an absence it never observed")
				return "", nil
			})()
			defer swap(&markReconciled, func(context.Context, string) (db.TargetReconciliation, error) {
				t.Fatal("a failed read must not be recorded as a current one")
				return db.TargetReconciliation{}, nil
			})()
			var reason string
			since := time.Unix(1_759_900_000, 0)
			defer swap(&markUnreconciled, func(_ context.Context, target, r string) (db.TargetReconciliation, error) {
				reason = r
				return db.TargetReconciliation{Target: target, UnreconciledSince: &since, UnreconciledReason: r}, nil
			})()

			res, err := Sweep(context.Background())
			if err != nil {
				t.Fatalf("a target outage is a halt, not a sweep failure: %v", err)
			}
			if !res.Halted || res.Reason != tc.wantReason {
				t.Fatalf("a failed read must halt and say why: want %q, got %+v", tc.wantReason, res)
			}
			if reason != tc.wantRecord {
				t.Fatalf("the durable reason must distinguish not-answering from answered-and-declined, want %q got %q", tc.wantRecord, reason)
			}
			if res.Reconciliation == nil || !res.Reconciliation.Unreconciled() {
				t.Fatalf("the result must carry the unreconciled record, got %+v", res.Reconciliation)
			}
		})
	}
}

// Syndra's own reads failing is not a statement about the target. Recording it
// as one would send an operator to check Zitadel because a Syndra query broke.
func TestSweep_ASyndraSideFailureIsNotTheTargetsFault(t *testing.T) {
	for _, tc := range []struct {
		name string
		bind func()
	}{
		{"direct grants", func() {
			svcAllDirectGrants = func(context.Context) ([]models.DirectGrant, error) {
				return nil, errors.New("query failed")
			}
		}},
		{"mapping rules", func() {
			svcGetActiveMappingRules = func(context.Context) ([]models.MappingRule, error) {
				return nil, errors.New("query failed")
			}
		}},
		{"exclusions", func() {
			svcGetExclusions = func(context.Context, string) ([]models.ExternalGrantExclusion, error) {
				return nil, errors.New("query failed")
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stubSweep(t)
			defer swap(&markUnreconciled, func(_ context.Context, _, r string) (db.TargetReconciliation, error) {
				t.Fatalf("a Syndra-side failure must not be recorded against the target (%s)", r)
				return db.TargetReconciliation{}, nil
			})()
			defer swap(&markReconciled, func(context.Context, string) (db.TargetReconciliation, error) {
				t.Fatal("an aborted sweep must not claim a current read")
				return db.TargetReconciliation{}, nil
			})()
			tc.bind()

			if _, err := Sweep(context.Background()); err == nil {
				t.Fatal("a Syndra-side failure must surface as an error, not a halt")
			}
		})
	}
}
