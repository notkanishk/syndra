package db

import (
	"context"
	"testing"
	"time"
)

// Attribution and "an event was probably missed" are DERIVED at read time
// (driftItemSelect), never stored — so this is a property of GetDriftItems
// and GetDriftItem the SQL-text guards elsewhere in this package cannot
// check. It needs a real join and a real webhook_events table, which is why
// it lives here rather than in drift_target_test.go.
//
// Scoped to drift_items alone, deliberately: Syndra's own writes never reach
// webhook_events (the self-mutation guard drops them before they are ever
// stored), so running this computation over the observation store instead
// would mark every grant Syndra ever makes. It runs only over rows already
// known to be unexplained, which is what makes "no event" informative rather
// than universal.

func truncateDriftAttributionFixtures(t *testing.T, ctx context.Context) {
	t.Helper()
	if _, err := PG.Exec(ctx, `TRUNCATE drift_items`); err != nil {
		t.Fatalf("truncate drift_items: %v", err)
	}
	if _, err := PG.Exec(ctx, `TRUNCATE webhook_events`); err != nil {
		t.Fatalf("truncate webhook_events: %v", err)
	}
}

// seedWebhookEventAt inserts a webhook_events row with an explicit created_at
// — InsertWebhookEvent always stamps NOW(), and these tests need to place
// events at specific points on the timeline relative to a finding's
// detected_at.
func seedWebhookEventAt(t *testing.T, ctx context.Context, idempotencyKey string, createdAt time.Time) {
	t.Helper()
	if _, err := PG.Exec(ctx, `
		INSERT INTO webhook_events (event_type, user_id, source_project, role_key, idempotency_key, status, created_at)
		VALUES ('grant_added', 'u1', 'p1', 'viewer', $1, 'processed', $2)`,
		idempotencyKey, createdAt); err != nil {
		t.Fatalf("seed webhook event: %v", err)
	}
}

func seedTargetOnlyFinding(t *testing.T, ctx context.Context, grantID string) string {
	t.Helper()
	id, _, err := UpsertDriftItemWithEvidence(ctx, TargetZitadel, "u1", "p1", []string{"viewer"},
		grantID, "reconciliation_sweep", DriftTargetOnly, DriftEvidence{})
	if err != nil {
		t.Fatalf("seed finding: %v", err)
	}
	if id == "" {
		t.Fatal("seed finding: expected an id")
	}
	return id
}

// A grant with a matching event is not marked — an event exists, whether or
// not the sweep's own write captured it as evidence.
func TestDriftAttribution_AMatchingEventClearsBothMarkers(t *testing.T) {
	ctx := liveDB(t)
	truncateDriftAttributionFixtures(t, ctx)

	id := seedTargetOnlyFinding(t, ctx, "grant-1")
	// The event's aggregate ID is the grant ID; sequence and event type do
	// not matter to the join, only the "<grantID>:" prefix.
	seedWebhookEventAt(t, ctx, "grant-1:user.grant.added:1", time.Now().Add(-time.Hour))

	item, err := GetDriftItem(ctx, id)
	if err != nil {
		t.Fatalf("GetDriftItem: %v", err)
	}
	if item.AttributionUnavailable {
		t.Error("a grant with a matching event must not read attribution_unavailable")
	}
	if item.EventPossiblyMissed {
		t.Error("a grant with a matching event must not claim a missed event")
	}
}

// A grant with no matching event is marked unavailable, and — because the
// event log reaches back before it was detected — a miss is inferred too.
func TestDriftAttribution_NoEventMarksUnavailableAndMissed(t *testing.T) {
	ctx := liveDB(t)
	truncateDriftAttributionFixtures(t, ctx)

	// The log's horizon: an unrelated event held from well before this
	// finding is detected, so the log DOES cover the period in question.
	seedWebhookEventAt(t, ctx, "some-other-grant:user.grant.added:1", time.Now().Add(-30*24*time.Hour))

	id := seedTargetOnlyFinding(t, ctx, "grant-2")

	item, err := GetDriftItem(ctx, id)
	if err != nil {
		t.Fatalf("GetDriftItem: %v", err)
	}
	if !item.AttributionUnavailable {
		t.Fatal("a grant with no matching event must read attribution_unavailable")
	}
	if !item.EventPossiblyMissed {
		t.Error("the log covers this period and holds nothing for this grant — a miss must be inferred")
	}
}

// A grant first detected BEFORE the oldest event the log still holds must not
// claim a miss: the log simply does not reach back that far, so its silence
// proves nothing about whether an event ever arrived.
func TestDriftAttribution_ALogThatDoesNotReachBackMakesNoClaimOfAMiss(t *testing.T) {
	ctx := liveDB(t)
	truncateDriftAttributionFixtures(t, ctx)

	id := seedTargetOnlyFinding(t, ctx, "grant-3")
	// The oldest event held arrives AFTER this finding was detected — the log
	// does not go back far enough to say anything about this grant.
	seedWebhookEventAt(t, ctx, "some-other-grant:user.grant.added:1", time.Now().Add(time.Hour))

	item, err := GetDriftItem(ctx, id)
	if err != nil {
		t.Fatalf("GetDriftItem: %v", err)
	}
	if !item.AttributionUnavailable {
		t.Fatal("still true: nothing says who made this change")
	}
	if item.EventPossiblyMissed {
		t.Error("the log does not reach back to when this was detected — it must not claim a miss")
	}
}

// A late-arriving event — one inserted after the finding already exists —
// clears the marker on the very next read, with no row to reconcile. This is
// the whole point of deriving rather than storing: the sweep and the webhook
// race, and the read is always honest about who is currently ahead.
func TestDriftAttribution_ALateArrivingEventClearsTheMarkerOnTheNextRead(t *testing.T) {
	ctx := liveDB(t)
	truncateDriftAttributionFixtures(t, ctx)

	id := seedTargetOnlyFinding(t, ctx, "grant-4")

	before, err := GetDriftItem(ctx, id)
	if err != nil {
		t.Fatalf("GetDriftItem (before): %v", err)
	}
	if !before.AttributionUnavailable {
		t.Fatal("before the event arrives, attribution must read unavailable")
	}

	seedWebhookEventAt(t, ctx, "grant-4:user.grant.added:1", time.Now())

	after, err := GetDriftItem(ctx, id)
	if err != nil {
		t.Fatalf("GetDriftItem (after): %v", err)
	}
	if after.AttributionUnavailable {
		t.Error("the event that just arrived must clear attribution_unavailable on this read — nothing was stored to go stale")
	}
}

// A finding the sweep already has direct evidence for (upstream_actor set,
// e.g. because a webhook attributed it before the sweep re-detected it) is
// never marked, even with no matching event in the log — the row already
// knows who, from a source stronger than the join.
func TestDriftAttribution_KnownEvidenceIsNeverMarkedRegardlessOfTheJoin(t *testing.T) {
	ctx := liveDB(t)
	truncateDriftAttributionFixtures(t, ctx)

	actor := "svc-badge-sync"
	id, _, err := UpsertDriftItemWithEvidence(ctx, TargetZitadel, "u1", "p1", []string{"viewer"},
		"grant-5", "webhook", DriftTargetOnly, DriftEvidence{UpstreamActor: actor})
	if err != nil {
		t.Fatalf("seed finding: %v", err)
	}

	item, err := GetDriftItem(ctx, id)
	if err != nil {
		t.Fatalf("GetDriftItem: %v", err)
	}
	if item.AttributionUnavailable {
		t.Error("a finding with known upstream evidence must never read attribution_unavailable")
	}
	if item.EventPossiblyMissed {
		t.Error("known evidence means nothing was missed")
	}
}
