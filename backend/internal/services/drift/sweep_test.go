package drift

import (
	"context"
	"testing"

	"syndra/internal/models"
	"syndra/internal/zitadel"
)

func swap[T any](dst *T, v T) func() { o := *dst; *dst = v; return func() { *dst = o } }

// stubSweep sets safe no-op defaults; each test overrides only what it asserts.
func stubSweep(t *testing.T) {
	t.Cleanup(swap(&zitadelReachable, func(context.Context) bool { return true }))
	t.Cleanup(swap(&svcAllDirectGrants, func(context.Context) ([]models.DirectGrant, error) { return nil, nil }))
	t.Cleanup(swap(&svcGetActiveMappingRules, func(context.Context) ([]models.MappingRule, error) { return nil, nil }))
	t.Cleanup(swap(&svcGetExclusions, func(context.Context) ([]models.ExternalGrantExclusion, error) { return nil, nil }))
	t.Cleanup(swap(&zitadelListAllGrants, func(context.Context, zitadel.SearchParams) (*zitadel.SearchResult[zitadel.UserGrant], error) {
		return &zitadel.SearchResult[zitadel.UserGrant]{}, nil
	}))
	t.Cleanup(swap(&upsertDriftItem, func(context.Context, string, string, []string, string, string, string) (string, bool, error) {
		return "d1", true, nil
	}))
	t.Cleanup(swap(&pendingOutboxAddExists, func(context.Context, string, string, string) (bool, error) { return false, nil }))
	t.Cleanup(swap(&insertPending, func(context.Context, string, string, string, []string, string, string, string, string) (string, error) {
		return "o1", nil
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
	defer swap(&upsertDriftItem, func(_ context.Context, _, _ string, _ []string, _, _, dtype string) (string, bool, error) {
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
	defer swap(&upsertDriftItem, func(context.Context, string, string, []string, string, string, string) (string, bool, error) {
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
	defer swap(&pendingOutboxAddExists, func(context.Context, string, string, string) (bool, error) { return true, nil })()
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
	defer swap(&svcGetExclusions, func(context.Context) ([]models.ExternalGrantExclusion, error) {
		return []models.ExternalGrantExclusion{{UserID: "u1", ProjectID: "p1", RoleKey: "viewer"}}, nil
	})()
	defer swap(&zitadelListAllGrants, func(context.Context, zitadel.SearchParams) (*zitadel.SearchResult[zitadel.UserGrant], error) {
		return &zitadel.SearchResult[zitadel.UserGrant]{
			Items: []zitadel.UserGrant{{ID: "g1", UserID: "u1", ProjectID: "p1", RoleKeys: []string{"viewer"}}}, Total: 1,
		}, nil
	})()
	var created int
	defer swap(&upsertDriftItem, func(context.Context, string, string, []string, string, string, string) (string, bool, error) {
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
