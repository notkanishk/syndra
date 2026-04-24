package services

import (
	"context"
	"fmt"
	"log"
)

// EmitProvisioningIntent creates a provisioning intent for LLDAP sync.
// It computes the flattened LLDAP group name from the stable project ID
// (not the mutable display name) and persists the intent with idempotency.
func EmitProvisioningIntent(ctx context.Context, targetUID, action, sourceProjectID,
	sourceRoleKey, webhookEventID string) error {

	lldapGroup := FlattenLLDAPGroup(sourceProjectID, sourceRoleKey)

	idempotencyKey := fmt.Sprintf("%s:%s:%s:%s", action, targetUID, lldapGroup, webhookEventID)

	intentID, inserted, err := svcInsertProvisioningIntent(ctx, targetUID, action, lldapGroup,
		sourceProjectID, sourceRoleKey, webhookEventID, idempotencyKey)
	if err != nil {
		return fmt.Errorf("persist provisioning intent: %w", err)
	}
	if !inserted {
		log.Printf("[PROVISIONING] Duplicate intent skipped: key=%s", idempotencyKey)
		return nil
	}

	log.Printf("[PROVISIONING] Intent emitted: id=%s action=%s user=%s group=%s",
		intentID, action, targetUID, lldapGroup)

	_ = svcInsertAuditLog(ctx, "system:provisioning", targetUID, "intent.emitted", intentID)

	return nil
}

// EmitProvisioningIntentFromScheduler is the scheduler-origin counterpart to
// EmitProvisioningIntent. It discriminates the idempotency key by grantID so
// repeated (grant → expire → sweep → re-grant → expire → sweep) cycles for the
// same (user, project, role) tuple emit distinct intents (the default key
// scheme would collide when webhookEventID is empty, silently skipping the
// second removal via ON CONFLICT DO NOTHING). webhook_event_id stays NULL so
// intents remain unambiguously scheduler-originated.
func EmitProvisioningIntentFromScheduler(ctx context.Context, targetUID, action,
	sourceProjectID, sourceRoleKey, grantID string) error {

	lldapGroup := FlattenLLDAPGroup(sourceProjectID, sourceRoleKey)

	idempotencyKey := fmt.Sprintf("%s:%s:%s:sched:%s", action, targetUID, lldapGroup, grantID)

	intentID, inserted, err := svcInsertProvisioningIntent(ctx, targetUID, action, lldapGroup,
		sourceProjectID, sourceRoleKey, "", idempotencyKey)
	if err != nil {
		return fmt.Errorf("persist scheduler provisioning intent: %w", err)
	}
	if !inserted {
		log.Printf("[PROVISIONING] Duplicate scheduler intent skipped: key=%s", idempotencyKey)
		return nil
	}

	log.Printf("[PROVISIONING] Scheduler intent emitted: id=%s action=%s user=%s group=%s grant=%s",
		intentID, action, targetUID, lldapGroup, grantID)

	_ = svcInsertAuditLog(ctx, "system:scheduler", targetUID, "intent.emitted", intentID)

	return nil
}
