package propagation

import (
	"context"
	"testing"
	"time"

	"syndra/internal/db"
	"syndra/internal/extcall"
	"syndra/internal/models"
)

// A give that Zitadel answered 503 was requeued with a backoff. Once the
// backoff has passed, the background drain sends it again on its own — the
// operator consented when they sent it straight away — and audits that try.
func TestDrainRevocations_RetriesA503dGiveAfterItsBackoff(t *testing.T) {
	stubDrainDeps(t)
	t.Cleanup(swap(&claimRevocations, revokeRows()))
	give := models.PendingPropagation{
		ID: "g1", Target: db.TargetZitadel, OpType: "add", Status: "in_flight",
		UserID: "u", ProjectID: "p", RoleKeys: []string{"r"}, Attempts: 1,
	}
	t.Cleanup(swap(&claimRetries, func(_ context.Context, target string, _ int) ([]models.PendingPropagation, error) {
		if target != db.TargetZitadel {
			return nil, nil
		}
		return []models.PendingPropagation{give}, nil
	}))
	var added bool
	zitadelAddUserGrant = func(context.Context, string, string, []string) error { added = true; return nil }
	var applied string
	markApplied = func(_ context.Context, id string) error { applied = id; return nil }
	var audited map[string]any
	var action string
	insertAudit = func(_ context.Context, actor, _, act, _ string, details any) error {
		action, audited = act, details.(map[string]any)
		if actor != "system" {
			t.Errorf("an automatic try is the system's, got actor %q", actor)
		}
		return nil
	}

	res, err := DrainRevocations(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !added || applied != "g1" || res.Applied != 1 {
		t.Fatalf("want the give sent again and settled, got added=%v applied=%q res=%+v", added, applied, res)
	}
	if action != "propagation.retried" || audited["retried_by"] != "automatic" ||
		audited["try"] != 2 || audited["of"] != RetryRule().Attempts || audited["outcome"] != "applied" {
		t.Fatalf("automatic try not audited as one: %s %v", action, audited)
	}
}

// A requeued row still pending says it is retrying, with its try and next
// try. A row whose role waits never says so: nothing sends it on its own.
func TestRowRetry_OnlyForRowsSyndraSendsAgainOnItsOwn(t *testing.T) {
	next := time.Now().Add(time.Minute)
	row := models.PendingPropagation{Status: "pending", Attempts: 2, NotBefore: &next}
	got := RowRetry(row)
	if got == nil || got.State != extcall.RetryRetrying || got.Try != 3 || got.Of != maxRetries+1 || got.NextTry != &next {
		t.Fatalf("RowRetry = %+v", got)
	}
	for name, r := range map[string]models.PendingPropagation{
		"never tried": {Status: "pending"},
		"being sent":  {Status: "in_flight", Attempts: 1, NotBefore: &next},
		"role waits":  {Status: "pending", Attempts: 1, NotBefore: &next, WaitReason: db.WaitReasonRoleWaits},
		"manual wait": {Status: "pending", Attempts: 1, NotBefore: &next, WaitReason: db.WaitReasonManual},
		"no next try": {Status: "pending", Attempts: 1},
	} {
		if s := RowRetry(r); s != nil {
			t.Errorf("%s: RowRetry = %+v, want nil", name, s)
		}
	}
}
