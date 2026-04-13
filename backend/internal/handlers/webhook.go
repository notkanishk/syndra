package handlers

import (
	"bytes"
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

	"mkauth/internal/cache"
	"mkauth/internal/services"
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
//
// Security: validates HMAC-SHA256 signature and request freshness before
// processing any payload. This prevents replay attacks and spoofed events.
func HandleZitadelWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonErrorResponse(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST is supported")
		return
	}

	// Read body first so we can verify the signature over the raw bytes
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1 MB max
	if err != nil {
		jsonErrorResponse(w, http.StatusBadRequest, "BAD_REQUEST", "Failed to read request body")
		return
	}

	tsHeader := r.Header.Get("X-Zitadel-Timestamp")

	// 1. Authenticate: HMAC-SHA256 over timestamp + body together.
	// The timestamp is part of the signed input so an attacker cannot swap in a
	// fresh timestamp to bypass the freshness check while reusing a captured signature.
	if err := verifyWebhookSignature(body, tsHeader, r.Header.Get("X-Zitadel-Signature")); err != nil {
		log.Printf("[WEBHOOK] Signature verification failed: %v", err)
		jsonErrorResponse(w, http.StatusUnauthorized, "WEBHOOK_UNAUTHORIZED", "Invalid webhook signature")
		return
	}

	// 2. Freshness check: reject stale events (> 5 minutes old).
	// Kept as a separate gate so the timestamp window is enforced even when the
	// signature is valid (guards against exact-replay within the signing window).
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
	if !trimmedNonEmpty(event.UserID) || !trimmedNonEmpty(event.SourceProject) || !trimmedNonEmpty(event.RoleKey) {
		jsonValidationErrorResponse(w, "user_id, source_project, and role_key are required", map[string]string{
			"user_id":        "required",
			"source_project": "required",
			"role_key":       "required",
		})
		return
	}

	log.Printf("[WEBHOOK] Event received: user=%s project=%s role=%s", event.UserID, event.SourceProject, event.RoleKey)

	// 4. Invalidate + rebuild Redis cache for this user
	if len(event.ProjectIDs) > 0 {
		cache.RebuildUserCache(r.Context(), event.UserID, event.ProjectIDs)
	} else {
		_ = cache.InvalidateUser(r.Context(), event.UserID)
	}

	// 5. Fire the Orchestrator loop for role propagation
	if err := zitadel.EnforceMappingRules(r.Context(), event.UserID, event.SourceProject, event.RoleKey); err != nil {
		log.Printf("[WEBHOOK] Orchestrator failure for user=%s: %v", event.UserID, err)
		jsonErrorResponse(w, http.StatusInternalServerError, "ORCHESTRATOR_FAULT", err.Error())
		return
	}

	// 6. Backend-owned onboarding: trigger welcome-bundle assignment for new users
	if event.RoleKey == "new_user" {
		idempotencyKey := fmt.Sprintf("webhook:%s:%s", event.UserID, event.SourceProject)
		if err := services.TriggerOnboarding(r.Context(), event.UserID, "webhook", idempotencyKey); err != nil {
			// Non-fatal: log and continue. Onboarding failures are visible in the triggers table.
			log.Printf("[WEBHOOK] Onboarding trigger failed for user=%s: %v", event.UserID, err)
		}
	}

	jsonResponse(w, http.StatusOK, map[string]string{"message": "Webhook processed: cache rebuilt + rules enforced"})
}

// verifyWebhookSignature computes HMAC-SHA256 over (tsHeader + "\n" + body) and
// compares it to the provided signature header (hex-encoded).
//
// The timestamp is included in the signed input so that an attacker who captures a
// valid request cannot substitute a fresh timestamp to bypass the freshness check
// while reusing the original signature.
//
// Returns nil if ZITADEL_WEBHOOK_SECRET is unset (local-dev mode).
func verifyWebhookSignature(body []byte, tsHeader, sigHeader string) error {
	secret := os.Getenv("ZITADEL_WEBHOOK_SECRET")
	if secret == "" {
		// Local-dev mode: signature verification skipped
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

// verifyWebhookFreshness checks that X-Zitadel-Timestamp is within the last 5
// minutes. Returns nil if the header is absent in local-dev mode (no secret set).
func verifyWebhookFreshness(tsHeader string) error {
	secret := os.Getenv("ZITADEL_WEBHOOK_SECRET")
	if secret == "" {
		// Local-dev mode: freshness check skipped
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
