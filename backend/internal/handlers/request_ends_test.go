package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"syndra/internal/db"
	"syndra/internal/models"
	"syndra/internal/services/propagation"
)

// A settled request to keep a role remembers both ends: approval records the
// end they had and the end it set, a decline only the end they had, and an
// ordinary request neither.

func decideForEnds(t *testing.T, kind, status string, held models.RequestHolding) (before, after *time.Time, resolveCalls, approveCalls int) {
	t.Helper()
	resetAccessDeps(t)
	days := 31
	dbGetAccessRequestByID = func(_ context.Context, id string) (models.AccessRequest, error) {
		return models.AccessRequest{ID: id, Kind: kind, RequesterID: "u1", ProjectID: "p1", RoleKey: "r", Status: "pending", DurationDays: &days}, nil
	}
	dbRequestHolding = func(context.Context, string, string, string) (models.RequestHolding, error) { return held, nil }
	dbApproveRequestAndEnqueue = func(_ context.Context, _, _, _ string, _ db.EnqueueParams, b, a *time.Time) (db.EnqueueResult, error) {
		before, after, approveCalls = b, a, approveCalls+1
		return db.EnqueueResult{OutboxID: "ob", Status: "pending"}, nil
	}
	dbResolveAccessRequest = func(_ context.Context, _, _, _, _ string, b *time.Time) error {
		before, resolveCalls = b, resolveCalls+1
		return nil
	}
	svcDrainPropagationRow = func(context.Context, string) (propagation.DrainResult, error) {
		return propagation.DrainResult{Applied: 1}, nil
	}
	cacheRebuildUser = func(context.Context, string, []string) {}
	origConverge := svcConvergeProjectChange
	t.Cleanup(func() { svcConvergeProjectChange = origConverge })
	svcConvergeProjectChange = func(context.Context, string, string, string) error { return nil }
	dbInsertAuditLog = func(context.Context, string, string, string, string) error { return nil }

	req := httptest.NewRequest(http.MethodPost, "/api/v1/requests/r1/decision",
		strings.NewReader(`{"status":"`+status+`","reviewer_id":"op-2","review_note":"x"}`))
	req.SetPathValue("id", "r1")
	rr := httptest.NewRecorder()
	handleResolveAccessRequest(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("%s %s: %d %s", kind, status, rr.Code, rr.Body.String())
	}
	return
}

func TestApprovingAnExtensionRecordsTheEndTheyHadAndTheEndItSet(t *testing.T) {
	had := time.Now().Add(10 * 24 * time.Hour).UTC()
	before, after, _, approves := decideForEnds(t, requestKindExtend, "approved",
		models.RequestHolding{Kind: models.HoldingUntil, Until: &had})
	if approves != 1 || before == nil || !before.Equal(had) {
		t.Fatalf("ended_before = %v, want the end they had (%v)", before, had)
	}
	if want := had.Add(31 * 24 * time.Hour); after == nil || !after.Equal(want) {
		t.Fatalf("ends_after = %v, want %v (counted from the end they had)", after, want)
	}
}

func TestAnExtensionThatLapsedRecordsNoEndBefore(t *testing.T) {
	before, after, _, _ := decideForEnds(t, requestKindExtend, "approved", models.RequestHolding{Kind: models.HoldingNone})
	if before != nil || after == nil {
		t.Fatalf("given again: ended_before %v (want nil), ends_after %v (want set)", before, after)
	}
}

func TestDecliningAnExtensionRecordsOnlyTheEndTheyHad(t *testing.T) {
	had := time.Now().Add(10 * 24 * time.Hour).UTC()
	before, _, resolves, _ := decideForEnds(t, requestKindExtend, "rejected",
		models.RequestHolding{Kind: models.HoldingUntil, Until: &had})
	if resolves != 1 || before == nil || !before.Equal(had) {
		t.Fatalf("declined ended_before = %v, want %v", before, had)
	}
}

func TestAnOrdinaryRequestRecordsNeitherEnd(t *testing.T) {
	had := time.Now().Add(24 * time.Hour).UTC()
	// A held role is only reachable by the decline path for an ordinary ask.
	before, _, _, _ := decideForEnds(t, requestKindAccess, "rejected", models.RequestHolding{Kind: models.HoldingUntil, Until: &had})
	if before != nil {
		t.Fatalf("a declined ordinary request has no ends to remember, got %v", before)
	}
	before, after, _, _ := decideForEnds(t, requestKindAccess, "approved", models.RequestHolding{Kind: models.HoldingNone})
	if before != nil || after != nil {
		t.Fatalf("an approved ordinary request records neither end: %v %v", before, after)
	}
}

func TestAFailedHoldingReadDoesNotStopADecline(t *testing.T) {
	resetAccessDeps(t)
	dbGetAccessRequestByID = func(_ context.Context, id string) (models.AccessRequest, error) {
		return models.AccessRequest{ID: id, Kind: requestKindExtend, RequesterID: "u1", ProjectID: "p1", RoleKey: "r", Status: "pending"}, nil
	}
	dbRequestHolding = func(context.Context, string, string, string) (models.RequestHolding, error) {
		return models.RequestHolding{}, context.DeadlineExceeded
	}
	var called bool
	dbResolveAccessRequest = func(_ context.Context, _, _, _, _ string, b *time.Time) error {
		called = b == nil
		return nil
	}
	dbInsertAuditLog = func(context.Context, string, string, string, string) error { return nil }
	req := httptest.NewRequest(http.MethodPost, "/api/v1/requests/r1/decision", strings.NewReader(`{"status":"rejected","reviewer_id":"op-2"}`))
	req.SetPathValue("id", "r1")
	rr := httptest.NewRecorder()
	handleResolveAccessRequest(rr, req)
	if rr.Code != http.StatusOK || !called {
		t.Fatalf("decline must still resolve, with the end left unsaid: %d called=%v", rr.Code, called)
	}
}

func TestSettledRequestJSONOmitsEndsWhenNull(t *testing.T) {
	end := time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC)
	out := jsonString(t, models.AccessRequest{ID: "a", EndedBefore: &end})
	if !strings.Contains(out, `"ended_before":"2026-11-18T00:00:00Z"`) || strings.Contains(out, "ends_after") {
		t.Fatalf("ended_before present, ends_after omitted: %s", out)
	}
	if out := jsonString(t, models.AccessRequest{ID: "a"}); strings.Contains(out, "ended_before") || strings.Contains(out, "ends_after") {
		t.Fatalf("both omitted when null: %s", out)
	}
}

func TestMyChangesCarryTheEndsOfASettledExtension(t *testing.T) {
	a := time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC)
	b := a.AddDate(0, 1, 0)
	out := jsonString(t, shapeMyChanges([]db.MyChangeRow{
		{Kind: "request_approved", RequestKind: "extend", EndedBefore: &a, EndsAfter: &b},
		{Kind: "request_declined", RequestKind: "extend", EndedBefore: &a},
		{Kind: "given"},
	}, nil))
	if strings.Count(out, `"ended_before"`) != 2 || strings.Count(out, `"ends_after"`) != 1 {
		t.Fatalf("ends ride on the settled extensions only: %s", out)
	}
}

func jsonString(t *testing.T, v any) string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
