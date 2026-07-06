package handlers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
)

// WebhookPayload represents our interpretation of a Zitadel event payload.
type WebhookPayload struct {
	EventType     string   `json:"event_type"` // grant_added, grant_removed, grant_changed, user_deactivated, user_locked, user_created
	UserID        string   `json:"user_id"`
	SourceProject string   `json:"source_project"`
	RoleKey       string   `json:"role_key"`           // back-compat singular; prefer RoleKeys for new callers
	RoleKeys      []string `json:"role_keys"`          // multi-role grants from Zitadel event-trigger payloads
	ProjectIDs    []string `json:"project_ids"`        // all projects the user touches
	GrantID       string   `json:"grant_id,omitempty"` // Zitadel user_grant aggregate ID; key for the grants index
}

var validEventTypes = map[string]bool{
	"grant_added":      true,
	"grant_removed":    true,
	"grant_changed":    true,
	"user_deactivated": true,
	"user_locked":      true,
	"user_created":     true,
}

// HandleZitadelWebhook processes incoming Zitadel events.
// Authentication and freshness are enforced by the withZitadelActionSignature
// middleware on the route. The handler keeps body parsing, structural
// validation, idempotency, and dispatch.
func HandleZitadelWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonErrorResponse(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST is supported")
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		jsonErrorResponse(w, http.StatusBadRequest, "BAD_REQUEST", "Failed to read request body")
		return
	}

	// Try Zitadel-shape translation first; fall back to internal strict decode.
	var event WebhookPayload
	translated, isZitadel, terr := translateZitadelEvent(body)
	if terr == errSelfMutation {
		jsonResponse(w, http.StatusOK, map[string]string{"message": "self-mutation event dropped"})
		return
	}
	if terr != nil {
		jsonValidationErrorResponse(w, "Invalid Zitadel event payload", map[string]string{"body": terr.Error()})
		return
	}
	if isZitadel {
		if translated.EventType == "" {
			// Unknown / unsupported Zitadel event — no-op success.
			jsonResponse(w, http.StatusOK, map[string]string{"message": "event acknowledged, no dispatch"})
			return
		}
		// Fill in the projectId/roleKeys that Zitadel omits from
		// grant.changed and grant.removed payloads (local index +
		// Zitadel API fallback). Best-effort; never errors out.
		event = enrichGrantPayload(r.Context(), translated)
	} else {
		if err := decodeJSONStrict(bytes.NewReader(body), &event); err != nil {
			jsonValidationErrorResponse(w, "Invalid webhook payload", map[string]string{"body": err.Error()})
			return
		}
	}

	if !trimmedNonEmpty(event.EventType) {
		jsonValidationErrorResponse(w, "event_type is required", map[string]string{
			"event_type": "required",
		})
		return
	}
	if !validEventTypes[event.EventType] {
		jsonValidationErrorResponse(w, "Invalid event_type", map[string]string{
			"event_type": "must be one of: grant_added, grant_removed, grant_changed, user_deactivated, user_locked, user_created",
		})
		return
	}

	// user_id is required for every event; source_project is required only
	// for grant events (lifecycle events — user_created/deactivated/locked —
	// arrive without project context from native Zitadel triggers).
	isGrantEvent := event.EventType == "grant_added" || event.EventType == "grant_removed" || event.EventType == "grant_changed"
	if !trimmedNonEmpty(event.UserID) {
		jsonValidationErrorResponse(w, "user_id is required", map[string]string{
			"user_id": "required",
		})
		return
	}
	// Zitadel-origin grant events whose enrichment couldn't find the missing
	// projectId/roleKeys MUST acknowledge with 200 — bouncing 4xx triggers
	// Zitadel redelivery storms with no clean resolution (the most common
	// case is a grant.removed for an already-gone aggregate, which neither
	// the local index nor ListUserGrants can resolve). Internal-shape
	// callers (operator curl, contract tests) still get strict 400s below.
	if isZitadel && isGrantEvent && (!trimmedNonEmpty(event.SourceProject) || len(event.RoleKeys) == 0) {
		log.Printf("[WEBHOOK] grant event acknowledged without dispatch (enrichment incomplete) event=%s user=%s grant=%s project=%q roles=%v",
			event.EventType, event.UserID, event.GrantID, event.SourceProject, event.RoleKeys)
		idempotencyKey := r.Header.Get("ZITADEL-Signature")
		if idempotencyKey == "" {
			idempotencyKey = fmt.Sprintf("dropped:%s:%s:%s", event.EventType, event.UserID, event.GrantID)
		}
		if err := dbDropWebhookEventEnrichmentIncomplete(r.Context(), event.EventType, event.UserID, event.GrantID, idempotencyKey); err != nil {
			log.Printf("[WEBHOOK] failed to persist dropped event: %v (non-fatal)", err)
		}
		jsonResponse(w, http.StatusOK, map[string]string{"message": "grant event acknowledged, dispatch skipped (enrichment incomplete)"})
		return
	}
	if isGrantEvent && !trimmedNonEmpty(event.SourceProject) {
		jsonValidationErrorResponse(w, "source_project is required for grant events", map[string]string{
			"source_project": "required",
		})
		return
	}
	if isGrantEvent {
		// At least one of role_key (singular) or role_keys (plural) MUST be set.
		if !trimmedNonEmpty(event.RoleKey) && len(event.RoleKeys) == 0 {
			jsonValidationErrorResponse(w, "role_key or role_keys is required for grant events", map[string]string{
				"role_key":  "required (or use role_keys array)",
				"role_keys": "required (or use role_key string)",
			})
			return
		}
		// Normalize: when role_keys was OMITTED (nil after JSON decode), seed
		// it from singular RoleKey. An EXPLICIT empty array (`"role_keys":[]`)
		// is left alone so the empty-after-filter check below rejects it —
		// per the plural-wins rule, an explicit empty plural contradicts a
		// non-empty singular and is treated as a malformed payload, not a
		// silent fall-through to the singular.
		if event.RoleKeys == nil && trimmedNonEmpty(event.RoleKey) {
			event.RoleKeys = []string{event.RoleKey}
		}
		// Filter blank/whitespace-only entries — invalid roles must never reach
		// downstream orchestration or provisioning (could emit malformed LLDAP groups).
		filtered := make([]string, 0, len(event.RoleKeys))
		for _, rk := range event.RoleKeys {
			if trimmedNonEmpty(rk) {
				filtered = append(filtered, rk)
			}
		}
		event.RoleKeys = filtered
		if len(event.RoleKeys) == 0 {
			jsonValidationErrorResponse(w, "role_key or role_keys is required for grant events", map[string]string{
				"role_key":  "required (or use role_keys array; blank entries are rejected)",
				"role_keys": "required (or use role_key string; blank entries are rejected)",
			})
			return
		}
		// Plural is authoritative for logging/auditing/idempotency: always sync the
		// singular RoleKey with RoleKeys[0] so a mismatched singular ("alpha") plus
		// plural (["beta"]) doesn't audit "alpha" while dispatching "beta".
		event.RoleKey = event.RoleKeys[0]
	}

	log.Printf("[WEBHOOK] Event received: type=%s user=%s project=%s role=%s",
		event.EventType, event.UserID, event.SourceProject, event.RoleKey)

	// Persist event and deduplicate.
	// Use the Actions v2 ZITADEL-Signature header as idempotency key when
	// available — unique per (timestamp, body) per Zitadel signing semantics.
	// Internal-shape callers (operator curl, contracts tests) fall back to
	// payload-derived deduplication.
	idempotencyKey := r.Header.Get("ZITADEL-Signature")
	if idempotencyKey == "" {
		idempotencyKey = fmt.Sprintf("%s:%s:%s:%s",
			event.EventType, event.UserID, event.SourceProject, event.RoleKey)
	}

	eventID, inserted, err := dbInsertWebhookEvent(r.Context(), event.EventType, event.UserID, event.SourceProject, event.RoleKey, idempotencyKey)
	if err != nil {
		log.Printf("[WEBHOOK] Failed to persist event: %v", err)
		// Non-fatal: continue processing even if persistence fails
	} else if !inserted {
		log.Printf("[WEBHOOK] Duplicate event skipped: key=%s", idempotencyKey)
		jsonResponse(w, http.StatusOK, map[string]string{"message": "Duplicate event — already processed"})
		return
	}

	// Dispatch by event type. The grants-index ops (upsert/delete) are
	// best-effort cache maintenance — log on failure, never block dispatch.
	var processingErr error
	switch event.EventType {
	case "grant_added", "grant_changed":
		processingErr = processGrantAdded(r.Context(), event, eventID)
		if processingErr == nil {
			maintainGrantIndex(r.Context(), event)
		}
	case "grant_removed":
		processingErr = processGrantRemoved(r.Context(), event, eventID)
		if processingErr == nil && event.GrantID != "" {
			if derr := dbDeleteGrantIndex(r.Context(), event.GrantID); derr != nil {
				log.Printf("[WEBHOOK] index delete failed grant=%s: %v (non-fatal)", event.GrantID, derr)
			}
		}
	case "user_deactivated", "user_locked":
		processingErr = processUserDeactivated(r.Context(), event)
	case "user_created":
		processingErr = processUserCreated(r.Context(), event)
	}

	// Update event status
	if eventID != "" {
		if processingErr != nil {
			_ = dbFailWebhookEvent(r.Context(), eventID, processingErr.Error())
		} else {
			_ = dbCompleteWebhookEvent(r.Context(), eventID)
		}
	}

	if processingErr != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "PROCESSING_FAULT", processingErr.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{"message": "Webhook processed"})
}

// processGrantAdded handles grant_added and grant_changed events:
// rebuild cache, enforce mapping rules per role, trigger onboarding for new_user,
// and emit a provisioning intent for LLDAP sync per role.
func processGrantAdded(ctx context.Context, event WebhookPayload, eventID string) error {
	if len(event.ProjectIDs) > 0 {
		cacheRebuildUser(ctx, event.UserID, event.ProjectIDs)
	} else {
		_ = cacheInvalidateUser(ctx, event.UserID)
	}

	for _, role := range event.RoleKeys {
		if err := webhookEnforceMappingRules(ctx, event.UserID, event.SourceProject, role); err != nil {
			return fmt.Errorf("orchestrator failure for role=%s: %w", role, err)
		}

		// Legacy back-compat: role_key=new_user on grant_added also triggers onboarding.
		if role == "new_user" {
			idempotencyKey := fmt.Sprintf("webhook:%s:%s", event.UserID, event.SourceProject)
			if err := webhookTriggerOnboarding(ctx, event.UserID, "webhook", idempotencyKey); err != nil {
				log.Printf("[WEBHOOK] Onboarding trigger failed for user=%s: %v", event.UserID, err)
			}
		}

		if err := webhookEmitProvisioningIntent(ctx, event.UserID, "add", event.SourceProject, role, eventID); err != nil {
			log.Printf("[WEBHOOK] Provisioning intent emission failed: %v", err)
		}
	}

	// Real-time drift: a surviving (non-self, per the self-mutation guard)
	// grant event that MkAuth neither expects nor has excluded is out-of-band.
	detectWebhookDrift(ctx, event)
	return nil
}

// detectWebhookDrift flags roles on a surviving grant event that MkAuth has no
// intent for. Best-effort and non-fatal: a detection failure must never bounce
// a 4xx back to Zitadel (redelivery storm) — the sweep is the backstop.
func detectWebhookDrift(ctx context.Context, event WebhookPayload) {
	for _, role := range event.RoleKeys {
		expected, err := svcUserExpectsRole(ctx, event.UserID, event.SourceProject, role)
		if err != nil {
			log.Printf("[DRIFT] webhook expected-check failed user=%s role=%s: %v (skipping)", event.UserID, role, err)
			continue
		}
		if expected {
			continue
		}
		excluded, err := dbHasExclusion(ctx, event.UserID, event.SourceProject, role)
		if err != nil {
			log.Printf("[DRIFT] webhook exclusion-check failed user=%s role=%s: %v (skipping — not flagging on uncertainty)", event.UserID, role, err)
			continue
		}
		if excluded {
			continue
		}
		if _, _, err := dbUpsertDriftItem(ctx, event.UserID, event.SourceProject,
			[]string{role}, event.GrantID, "webhook", "zitadel_only"); err != nil {
			log.Printf("[DRIFT] webhook upsert failed user=%s role=%s: %v (non-fatal)", event.UserID, role, err)
		}
	}
}

// processGrantRemoved handles grant_removed events:
// invalidate cache, revoke derived grants per role through mapping rules,
// and emit a provisioning intent per role for LLDAP sync.
func processGrantRemoved(ctx context.Context, event WebhookPayload, eventID string) error {
	_ = cacheInvalidateUser(ctx, event.UserID)
	for _, role := range event.RoleKeys {
		if err := webhookRevokeMappingRules(ctx, event.UserID, event.SourceProject, role); err != nil {
			return fmt.Errorf("revocation failure for role=%s: %w", role, err)
		}
		if err := webhookEmitProvisioningIntent(ctx, event.UserID, "remove", event.SourceProject, role, eventID); err != nil {
			log.Printf("[WEBHOOK] Provisioning intent emission failed: %v", err)
		}
	}
	return nil
}

// processUserDeactivated handles user_deactivated and user_locked events:
// full cache invalidation only.
func processUserDeactivated(ctx context.Context, event WebhookPayload) error {
	_ = cacheInvalidateUser(ctx, event.UserID)
	return nil
}

// processUserCreated handles user_created events:
// trigger onboarding (welcome bundle assignment).
func processUserCreated(ctx context.Context, event WebhookPayload) error {
	idempotencyKey := fmt.Sprintf("user_created:%s:%s", event.UserID, event.SourceProject)
	if err := webhookTriggerOnboarding(ctx, event.UserID, "webhook", idempotencyKey); err != nil {
		log.Printf("[WEBHOOK] Onboarding trigger failed for user=%s: %v", event.UserID, err)
	}
	return nil
}

// maintainGrantIndex refreshes the local zitadel_grants_index row from a
// successfully-processed grant_added or grant_changed event so subsequent
// grant.changed / grant.removed events can be enriched. Skips when the
// payload lacks fields the schema requires (NOT NULL on grant/user/project)
// — that case is handled upstream by validation, but defense-in-depth here.
func maintainGrantIndex(ctx context.Context, event WebhookPayload) {
	if event.GrantID == "" || event.UserID == "" || event.SourceProject == "" {
		return
	}
	if err := dbUpsertGrantIndex(ctx, event.GrantID, event.UserID, event.SourceProject, event.RoleKeys); err != nil {
		log.Printf("[WEBHOOK] index upsert failed grant=%s: %v (non-fatal)", event.GrantID, err)
	}
}
