package db

import (
	"context"
	"fmt"

	"syndra/internal/models"
)

// GetExclusions returns the Zitadel exclusion triples so the reconciliation
// sweep and the webhook can filter known-external grants out of drift detection.
//
// Scoped to `target='zitadel'` rather than reading the whole table. Both callers
// classify Zitadel grants, and an unscoped read would let an exclusion recorded
// against a TrueNAS grant suppress the identical triple on Zitadel — the exact
// cross-target suppression the target column entered the primary key to prevent
// (000026). The target becomes a parameter when the sweep itself becomes
// per-target (change `addon-platform` task 1.12); until then a literal is the
// honest statement of what these callers actually mean.
func GetExclusions(ctx context.Context) ([]models.ExternalGrantExclusion, error) {
	const q = `SELECT user_id, project_id, role_key, marked_by, marked_at, COALESCE(reason,'')
		FROM external_grant_exclusions WHERE target = 'zitadel'`
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
