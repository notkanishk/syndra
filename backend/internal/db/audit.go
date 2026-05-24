package db

import (
	"context"
	"fmt"
)

// -------------------------------------------------------------
// AUDIT LOG REPOSITORY
// -------------------------------------------------------------

func InsertAuditLog(ctx context.Context, actorID, targetID, action, resourceID string) error {
	query := `INSERT INTO audit_logs (actor_zitadel_user_id, target_zitadel_user_id, action, resource_id) VALUES ($1, $2, $3, $4)`
	_, err := PG.Exec(ctx, query, actorID, targetID, action, resourceID)
	if err != nil {
		return fmt.Errorf("failed to insert audit log: %w", err)
	}
	return nil
}
