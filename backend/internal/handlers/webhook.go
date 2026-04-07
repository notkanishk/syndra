package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"mkauth/internal/zitadel"
)

// WebhookPayload represents our stripped down interpretation of a Zitadel event payload
type WebhookPayload struct {
	UserID        string `json:"user_id"`
	SourceProject string `json:"source_project"`
	RoleKey       string `json:"role_key"`
}

// HandleZitadelWebhook executes the async policy propagation flow.
// When Zitadel fires an event (e.g. User Granted Role), this endpoint receives it,
// queries the local Postgres DB for dependent logic policies, and initiates follow-up API calls.
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

	// Fire the Orchestrator loop using the inbound event data!
	err := zitadel.EnforceMappingRules(r.Context(), event.UserID, event.SourceProject, event.RoleKey)
	if err != nil {
		log.Printf("[HOOK ERROR] Execution failure inside Orchestrator: %v", err)
		jsonErrorResponse(w, http.StatusInternalServerError, "ORCHESTRATOR_FAULT", err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "Webhook processed and dependent rules enforced.")
}
