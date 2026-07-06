package db

import (
	"context"
	"fmt"

	"mkauth/internal/models"
)

// InsertExclusion records an operator "legitimately external" marker. Idempotent
// on (user, project, role) — re-marking the same triple is a no-op.
func InsertExclusion(ctx context.Context, userID, projectID, roleKey, markedBy, reason string) error {
	const q = `
		INSERT INTO external_grant_exclusions (user_id, project_id, role_key, marked_by, reason)
		VALUES ($1,$2,$3,$4,NULLIF($5,''))
		ON CONFLICT (user_id, project_id, role_key) DO NOTHING`
	if _, err := PG.Exec(ctx, q, userID, projectID, roleKey, markedBy, reason); err != nil {
		return fmt.Errorf("insert exclusion: %w", err)
	}
	return nil
}

// GetExclusions returns all exclusion triples so the reconciliation sweep and
// the webhook can filter known-external grants out of drift detection.
func GetExclusions(ctx context.Context) ([]models.ExternalGrantExclusion, error) {
	const q = `SELECT user_id, project_id, role_key, marked_by, marked_at, COALESCE(reason,'')
		FROM external_grant_exclusions`
	rows, err := PG.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("get exclusions: %w", err)
	}
	defer rows.Close()
	var out []models.ExternalGrantExclusion
	for rows.Next() {
		var e models.ExternalGrantExclusion
		if err := rows.Scan(&e.UserID, &e.ProjectID, &e.RoleKey, &e.MarkedBy, &e.MarkedAt, &e.Reason); err != nil {
			return nil, fmt.Errorf("scan exclusion: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
