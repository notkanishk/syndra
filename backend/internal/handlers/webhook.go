package handlers

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

// WebhookPayload represents our interpretation of a Zitadel event payload.
type WebhookPayload struct {
	EventType     string   `json:"event_type"`      // grant_added, grant_removed, grant_changed, user_deactivated, user_locked, user_created
	UserID        string   `json:"user_id"`
	SourceProject string   `json:"source_project"`
	RoleKey       string   `json:"role_key"`
	ProjectIDs    []string `json:"project_ids"` // all projects the user touches
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
// Dispatches to event-type-specific handlers after authentication, freshness,
// and structural validation. Events are persisted for deduplication and audit.
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

	tsHeader := r.Header.Get("X-Zitadel-Timestamp")

	// 1. Authenticate: HMAC-SHA256 over timestamp + body.
	if err := verifyWebhookSignature(body, tsHeader, r.Header.Get("X-Zitadel-Signature")); err != nil {
		log.Printf("[WEBHOOK] Signature verification failed: %v", err)
		jsonErrorResponse(w, http.StatusUnauthorized, "WEBHOOK_UNAUTHORIZED", "Invalid webhook signature")
		return
	}

	// 2. Freshness check: reject stale events (> 5 minutes old).
	if err := verifyWebhookFreshness(tsHeader); err != nil {
		log.Printf("[WEBHOOK] Freshness check failed: %v", err)
		jsonErrorResponse(w, http.StatusBadRequest, "WEBHOOK_STALE", "Webhook timestamp is missing or too old")
		return
	}

	// 3. Structural validation
	var event WebhookPayload
	if err := decodeJSONStrict(bytes.NewReader(body), &event); err != nil {
		jsonValidationErrorResponse(w, "Invalid webhook payload", map[string]string{"body": err.Error()})
		return
	}

	// Default event_type for backward compatibility
	if event.EventType == "" {
		event.EventType = "grant_added"
	}

	if !validEventTypes[event.EventType] {
		jsonValidationErrorResponse(w, "Invalid event_type", map[string]string{
			"event_type": "must be one of: grant_added, grant_removed, grant_changed, user_deactivated, user_locked, user_created",
		})
		return
	}

	// role_key required for grant events only
	isGrantEvent := event.EventType == "grant_added" || event.EventType == "grant_removed" || event.EventType == "grant_changed"
	if !trimmedNonEmpty(event.UserID) || !trimmedNonEmpty(event.SourceProject) {
		jsonValidationErrorResponse(w, "user_id and source_project are required", map[string]string{
			"user_id":        "required",
			"source_project": "required",
		})
		return
	}
	if isGrantEvent && !trimmedNonEmpty(event.RoleKey) {
		jsonValidationErrorResponse(w, "role_key is required for grant events", map[string]string{
			"role_key": "required",
		})
		return
	}

	log.Printf("[WEBHOOK] Event received: type=%s user=%s project=%s role=%s",
		event.EventType, event.UserID, event.SourceProject, event.RoleKey)

	// 4. Persist event and deduplicate.
	// Use the HMAC signature as idempotency key when available — it's unique per
	// payload+timestamp and avoids lossy bucketing that could suppress distinct events.
	// In local-dev mode (no signature), fall back to payload+timestamp.
	idempotencyKey := r.Header.Get("X-Zitadel-Signature")
	if idempotencyKey == "" {
		idempotencyKey = fmt.Sprintf("%s:%s:%s:%s:%s", event.EventType, event.UserID, event.SourceProject, event.RoleKey, tsHeader)
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

	// 5. Dispatch by event type
	var processingErr error
	switch event.EventType {
	case "grant_added", "grant_changed":
		processingErr = processGrantAdded(r.Context(), event)
	case "grant_removed":
		processingErr = processGrantRemoved(r.Context(), event)
	case "user_deactivated", "user_locked":
		processingErr = processUserDeactivated(r.Context(), event)
	case "user_created":
		processingErr = processUserCreated(r.Context(), event)
	}

	// 6. Update event status
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
// rebuild cache, enforce mapping rules, trigger onboarding for new_user.
func processGrantAdded(ctx context.Context, event WebhookPayload) error {
	if len(event.ProjectIDs) > 0 {
		cacheRebuildUser(ctx, event.UserID, event.ProjectIDs)
	} else {
		_ = cacheInvalidateUser(ctx, event.UserID)
	}

	if err := webhookEnforceMappingRules(ctx, event.UserID, event.SourceProject, event.RoleKey); err != nil {
		return fmt.Errorf("orchestrator failure: %v", err)
	}

	// Legacy backward compat: role_key=new_user on grant_added also triggers onboarding.
	if event.RoleKey == "new_user" {
		idempotencyKey := fmt.Sprintf("webhook:%s:%s", event.UserID, event.SourceProject)
		if err := webhookTriggerOnboarding(ctx, event.UserID, "webhook", idempotencyKey); err != nil {
			log.Printf("[WEBHOOK] Onboarding trigger failed for user=%s: %v", event.UserID, err)
		}
	}
	return nil
}

// processGrantRemoved handles grant_removed events:
// invalidate cache, revoke derived grants through mapping rules.
func processGrantRemoved(ctx context.Context, event WebhookPayload) error {
	_ = cacheInvalidateUser(ctx, event.UserID)

	if err := webhookRevokeMappingRules(ctx, event.UserID, event.SourceProject, event.RoleKey); err != nil {
		return fmt.Errorf("revocation failure: %v", err)
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

// verifyWebhookSignature computes HMAC-SHA256 over (tsHeader + "\n" + body) and
// compares it to the provided signature header (hex-encoded).
func verifyWebhookSignature(body []byte, tsHeader, sigHeader string) error {
	secret := os.Getenv("ZITADEL_WEBHOOK_SECRET")
	if secret == "" {
		return nil
	}

	if sigHeader == "" {
		return fmt.Errorf("X-Zitadel-Signature header missing")
	}
	if tsHeader == "" {
		return fmt.Errorf("X-Zitadel-Timestamp header missing (required for signature)")
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(tsHeader))
	mac.Write([]byte("\n"))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(expected), []byte(sigHeader)) {
		return fmt.Errorf("signature mismatch")
	}
	return nil
}

// verifyWebhookFreshness checks that X-Zitadel-Timestamp is within the last 5 minutes.
func verifyWebhookFreshness(tsHeader string) error {
	secret := os.Getenv("ZITADEL_WEBHOOK_SECRET")
	if secret == "" {
		return nil
	}

	if tsHeader == "" {
		return fmt.Errorf("X-Zitadel-Timestamp header missing")
	}

	ts, err := strconv.ParseInt(tsHeader, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid timestamp: %w", err)
	}

	eventTime := time.Unix(ts, 0)
	age := time.Since(eventTime)
	if age > 5*time.Minute || age < -30*time.Second {
		return fmt.Errorf("event timestamp out of acceptable window (age=%v)", age)
	}
	return nil
}
