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
