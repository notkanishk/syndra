package handlers

import (
	"context"
	"net/http"
	"testing"
)

// An internal-shape grant carries no sequence, so the same role granted,
// removed and granted again used to produce the same key and the second grant
// was dropped as a duplicate for good. Each delivery without an event_id is its
// own event; with one, a redelivery still shares it.
func TestInternalShapeWebhook_RegrantAfterRemovalIsNotADuplicate(t *testing.T) {
	setupNoopWebhookDeps(t)
	var keys []string
	dbInsertWebhookEvent = func(_ context.Context, _, _, _, _, key string) (string, bool, error) {
		keys = append(keys, key)
		return "evt", true, nil
	}

	grant := []byte(`{"event_type":"grant_added","user_id":"u1","source_project":"p1","role_key":"editor"}`)
	remove := []byte(`{"event_type":"grant_removed","user_id":"u1","source_project":"p1","role_key":"editor"}`)
	for _, body := range [][]byte{grant, remove, grant} {
		if rr := postWebhook(t, body); rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
		}
	}
	if keys[0] == keys[2] {
		t.Fatalf("a second grant of the same role shared the first one's key: %v", keys)
	}

	keys = nil
	withID := []byte(`{"event_type":"grant_added","event_id":"evt-9","user_id":"u1","source_project":"p1","role_key":"editor"}`)
	postWebhook(t, withID)
	postWebhook(t, withID)
	if len(keys) != 2 || keys[0] != keys[1] {
		t.Fatalf("a redelivery carrying the same event_id must share its key: %v", keys)
	}
}

// A delivery whose row failed is handed back by the store as new (see
// db.InsertWebhookEvent), and the handler must run it again rather than answer
// "already processed".
func TestWebhook_ARetakenFailedDeliveryIsProcessedAgain(t *testing.T) {
	setupNoopWebhookDeps(t)
	dbInsertWebhookEvent = func(context.Context, string, string, string, string, string) (string, bool, error) {
		return "evt", true, nil
	}
	calls := 0
	webhookEnforceMappingRules = func(context.Context, string, string, string) error {
		calls++
		if calls == 1 {
			return context.DeadlineExceeded
		}
		return nil
	}
	body := []byte(`{"event_type":"grant_added","event_id":"e1","user_id":"u1","source_project":"p1","role_key":"editor"}`)

	if rr := postWebhook(t, body); rr.Code != http.StatusInternalServerError {
		t.Fatalf("first delivery should fail so Zitadel retries; got %d", rr.Code)
	}
	if rr := postWebhook(t, body); rr.Code != http.StatusOK || calls != 2 {
		t.Fatalf("the retry must run again: code=%d calls=%d", rr.Code, calls)
	}
}
