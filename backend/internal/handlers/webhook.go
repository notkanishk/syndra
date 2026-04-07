package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"mkauth/internal/cache"
	"mkauth/internal/zitadel"
)

// WebhookPayload represents our stripped down interpretation of a Zitadel event payload
type WebhookPayload struct {
	UserID        string   `json:"user_id"`
	SourceProject string   `json:"source_project"`
	RoleKey       string   `json:"role_key"`
	ProjectIDs    []string `json:"project_ids"` // all projects the user touches
}

// HandleZitadelWebhook executes the async policy propagation flow.
// When Zitadel fires an event (e.g. User Granted Role), this endpoint receives it,
// rebuilds the Redis cache, and initiates follow-up API calls.
func HandleZitadelWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var event WebhookPayload
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		jsonErrorResponse(w, http.StatusBadRequest, "INVALID_PAYLOAD", "Cannot parse webhook event")
		return
	}

	log.Printf("[HOOK] Webhook received: User: %s, Project: %s, Role: %s", event.UserID, event.SourceProject, event.RoleKey)

	// 1. Invalidate + rebuild Redis cache for this user
	if len(event.ProjectIDs) > 0 {
		cache.RebuildUserCache(r.Context(), event.UserID, event.ProjectIDs)
	} else {
		_ = cache.InvalidateUser(r.Context(), event.UserID)
	}

	// 2. Fire the Orchestrator loop for role propagation
	err := zitadel.EnforceMappingRules(r.Context(), event.UserID, event.SourceProject, event.RoleKey)
	if err != nil {
		log.Printf("[HOOK ERROR] Orchestrator failure: %v", err)
		jsonErrorResponse(w, http.StatusInternalServerError, "ORCHESTRATOR_FAULT", err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "Webhook processed: cache rebuilt + rules enforced.")
}
