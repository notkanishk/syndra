package db

import (
	"context"
	"encoding/json"
	"fmt"
)

// -------------------------------------------------------------
// AUDIT LOG REPOSITORY
// -------------------------------------------------------------

func InsertAuditLog(ctx context.Context, actorID, targetID, action, resourceID string) error {
	query := `INSERT INTO audit_logs (actor_zitadel_user_id, target_zitadel_user_id, action, resource_id) VALUES ($1, $2, $3, $4)`
	_, err := querier(ctx).Exec(ctx, query, actorID, targetID, action, resourceID)
	if err != nil {
		return fmt.Errorf("failed to insert audit log: %w", err)
	}
	return nil
}

// InsertAuditLogDetails is InsertAuditLog with structured details (JSONB).
func InsertAuditLogDetails(ctx context.Context, actorID, targetID, action, resourceID string, details any) error {
	raw, err := json.Marshal(details)
	if err != nil {
		return fmt.Errorf("audit details: %w", err)
	}
	if _, err := querier(ctx).Exec(ctx, `
		INSERT INTO audit_logs (actor_zitadel_user_id, target_zitadel_user_id, action, resource_id, details)
		VALUES ($1, $2, $3, $4, $5)`, actorID, targetID, action, resourceID, raw); err != nil {
		return fmt.Errorf("failed to insert audit log: %w", err)
	}
	return nil
}
