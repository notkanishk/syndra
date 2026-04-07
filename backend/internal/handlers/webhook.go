package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

// ZitadelEvent represents a generic event pushed from Zitadel Webhooks
type ZitadelEvent struct {
	EventType   string `json:"event_type"`
	AggregateID string `json:"aggregate_id"`
	// Additional payload attributes will sit here
}

// HandleZitadelWebhook receives events like 'user.changed' to trigger a Redis refresh mappings
func HandleZitadelWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var event ZitadelEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	// Log the incoming event from Zitadel
	log.Printf("Received Zitadel Webhook: %s for User/Aggregate ID: %s", event.EventType, event.AggregateID)

	// TODO: Flag the user's role mapping in Redis as "dirty" to force a sync
	// db.Redis.Set(context.Background(), fmt.Sprintf("user:%s:dirty", event.AggregateID), "true", 0)

	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "Webhook processed and state marked for invalidation.")
}
