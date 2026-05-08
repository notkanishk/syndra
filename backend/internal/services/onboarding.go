package services

import (
	"context"
	"fmt"
	"log"
)

// TriggerOnboarding initiates backend-owned welcome-bundle assignment for a new user.
//
// The idempotencyKey must uniquely identify this onboarding event (e.g. a hash of
// user_id + source + event_type). Duplicate calls with the same key are silently
// ignored — the operation is safe to retry.
//
// Failure model:
//   - trigger intake is always recorded before any mutation
//   - failures are persisted in the triggers table for operator visibility
//   - replay is safe: the idempotency key prevents duplicate assignments
func TriggerOnboarding(ctx context.Context, userID, source, idempotencyKey string) error {
	log.Printf("[ONBOARDING] Trigger received: user=%s source=%s key=%s", userID, source, idempotencyKey)

	// 1. Record the trigger — idempotent insert
	triggerID, inserted, err := svcInsertOnboardingTrigger(ctx, userID, source, idempotencyKey)
	if err != nil {
		return fmt.Errorf("record onboarding trigger: %w", err)
	}
	if !inserted {
		// Already processed — idempotent success
		log.Printf("[ONBOARDING] Duplicate trigger ignored: user=%s key=%s", userID, idempotencyKey)
		return nil
	}

	log.Printf("[ONBOARDING] Trigger recorded: id=%s user=%s", triggerID, userID)

	// 2. Find the welcome bundle to assign
	bundleID, err := svcGetWelcomeBundle(ctx)
	if err != nil {
		// db.ErrNoWelcomeBundleConfigured is the named cause; preserve via %w
		// so callers can errors.Is against it for tests + alerting.
		log.Printf("[ONBOARDING] No welcome bundle configured for user=%s: %v", userID, err)
		if failErr := svcFailOnboardingTrigger(ctx, triggerID, err.Error()); failErr != nil {
			log.Printf("[ONBOARDING] Failed to record failure for trigger=%s: %v", triggerID, failErr)
		}
		return fmt.Errorf("welcome bundle: %w", err)
	}

	// 3. Assign the welcome bundle (idempotent — uses ON CONFLICT DO NOTHING)
	if err := svcAssignBundleToUser(ctx, userID, bundleID); err != nil {
		log.Printf("[ONBOARDING] Bundle assignment failed: user=%s bundle=%s: %v", userID, bundleID, err)
		if failErr := svcFailOnboardingTrigger(ctx, triggerID, err.Error()); failErr != nil {
			log.Printf("[ONBOARDING] Failed to record failure for trigger=%s: %v", triggerID, failErr)
		}
		return fmt.Errorf("assign welcome bundle: %w", err)
	}

	// 4. Write audit log attributed to the system (automated action)
	if err := svcInsertAuditLog(ctx, "system:onboarding", userID, "welcome_bundle_assigned", bundleID); err != nil {
		// Audit failure is non-fatal but should be visible
		log.Printf("[ONBOARDING] Audit log failed for user=%s bundle=%s: %v", userID, bundleID, err)
	}

	// 5. Mark trigger as completed
	if err := svcCompleteOnboardingTrigger(ctx, triggerID, bundleID); err != nil {
		log.Printf("[ONBOARDING] Failed to mark trigger completed: id=%s: %v", triggerID, err)
		// Not returned as an error — the assignment itself succeeded
	}

	log.Printf("[ONBOARDING] Welcome bundle assigned: user=%s bundle=%s trigger=%s", userID, bundleID, triggerID)
	return nil
}
