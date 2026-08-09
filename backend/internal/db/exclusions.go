package db

import (
	"context"
	"fmt"

	"syndra/internal/models"
)

// GetExclusions returns one target's exclusion triples so a sweep or the
// webhook can filter known-external grants out of drift detection.
//
// Scoped to the caller's target rather than reading the whole table: an
// unscoped read would let an exclusion recorded against a TrueNAS grant
// suppress the identical triple on Zitadel — the exact cross-target
// suppression the target column entered the primary key to prevent (000026).
// The rows carry their target back so the filter can hold the same scope
// without trusting that the read applied it (see services.IsExcluded).
func GetExclusions(ctx context.Context, target string) ([]models.ExternalGrantExclusion, error) {
	const q = `SELECT target, user_id, project_id, role_key, marked_by, marked_at, COALESCE(reason,'')
		FROM external_grant_exclusions WHERE target = $1`
	rows, err := PG.Query(ctx, q, target)
	if err != nil {
		return nil, fmt.Errorf("get exclusions: %w", err)
	}
	defer rows.Close()
	var out []models.ExternalGrantExclusion
	for rows.Next() {
		var e models.ExternalGrantExclusion
		if err := rows.Scan(&e.Target, &e.UserID, &e.ProjectID, &e.RoleKey, &e.MarkedBy, &e.MarkedAt, &e.Reason); err != nil {
			return nil, fmt.Errorf("scan exclusion: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
