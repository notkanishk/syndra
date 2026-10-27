package db

import (
	"testing"
)

// Zitadel redelivers a webhook because we answered 5xx. A row that failed must
// be taken again on that redelivery; a row that is received or processed must
// still read as a duplicate, or a retry storm would run the same event twice.
func TestInsertWebhookEvent_ARedeliveryOfAFailedRowIsTakenAgain(t *testing.T) {
	ctx := liveDB(t)
	if _, err := PG.Exec(ctx, `TRUNCATE webhook_events`); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	id, inserted, err := InsertWebhookEvent(ctx, "grant_added", "u1", "p1", "viewer", "k1")
	if err != nil || !inserted {
		t.Fatalf("first delivery: id=%q inserted=%v err=%v", id, inserted, err)
	}
	if _, again, _ := InsertWebhookEvent(ctx, "grant_added", "u1", "p1", "viewer", "k1"); again {
		t.Fatal("a redelivery while the first is still in progress must be a duplicate")
	}

	if err := FailWebhookEvent(ctx, id, "boom"); err != nil {
		t.Fatalf("fail: %v", err)
	}
	retryID, retaken, err := InsertWebhookEvent(ctx, "grant_added", "u1", "p1", "viewer", "k1")
	if err != nil || !retaken || retryID != id {
		t.Fatalf("a failed row must be retaken as the same row: id=%q retaken=%v err=%v", retryID, retaken, err)
	}
	var status string
	var msg *string
	if err := PG.QueryRow(ctx, `SELECT status, error_message FROM webhook_events WHERE id = $1`, id).Scan(&status, &msg); err != nil {
		t.Fatalf("read: %v", err)
	}
	if status != "received" || msg != nil {
		t.Fatalf("a retaken row starts clean: status=%q error=%v", status, msg)
	}

	if err := CompleteWebhookEvent(ctx, id); err != nil {
		t.Fatalf("complete: %v", err)
	}
	if _, again, _ := InsertWebhookEvent(ctx, "grant_added", "u1", "p1", "viewer", "k1"); again {
		t.Fatal("a processed event must stay a duplicate")
	}
}

func TestInsertOnboardingTrigger_ARetryOfAFailedTriggerIsTakenAgain(t *testing.T) {
	ctx := liveDB(t)
	if _, err := PG.Exec(ctx, `TRUNCATE onboarding_triggers`); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	id, inserted, err := InsertOnboardingTrigger(ctx, "u1", "webhook", "k1")
	if err != nil || !inserted {
		t.Fatalf("first trigger: id=%q inserted=%v err=%v", id, inserted, err)
	}
	if _, again, _ := InsertOnboardingTrigger(ctx, "u1", "webhook", "k1"); again {
		t.Fatal("a repeat while the first is pending must be a duplicate")
	}

	if err := FailOnboardingTrigger(ctx, id, "boom"); err != nil {
		t.Fatalf("fail: %v", err)
	}
	retryID, retaken, err := InsertOnboardingTrigger(ctx, "u1", "webhook", "k1")
	if err != nil || !retaken || retryID != id {
		t.Fatalf("a failed trigger must be retaken as the same row: id=%q retaken=%v err=%v", retryID, retaken, err)
	}

	if _, err := PG.Exec(ctx, `UPDATE onboarding_triggers SET status = 'completed' WHERE id = $1`, id); err != nil {
		t.Fatalf("complete: %v", err)
	}
	if _, again, _ := InsertOnboardingTrigger(ctx, "u1", "webhook", "k1"); again {
		t.Fatal("a completed trigger must stay a duplicate")
	}
}
