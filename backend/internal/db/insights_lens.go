package db

import (
	"context"
	"fmt"
	"time"
)

// ExpiringGrant is one direct grant that has an end date and has not ended.
type ExpiringGrant struct {
	UserID, ProjectID, RoleKey string
	ExpiresAt                  time.Time
}

// GrantsWithExpiry lists every direct grant whose end is after `after`. The
// window itself is applied by the caller, so its edges are testable.
//
// ponytail: all future-dated grants, not only those in the window; a handful
// at this deployment's size. Push the upper bound into SQL if that ever grows.
func GrantsWithExpiry(ctx context.Context, after time.Time) ([]ExpiringGrant, error) {
	rows, err := querier(ctx).Query(ctx, `
		SELECT user_id, zitadel_project_id, zitadel_role_key, expires_at
		  FROM direct_role_grants
		 WHERE expires_at IS NOT NULL AND expires_at > $1`, after)
	if err != nil {
		return nil, fmt.Errorf("grants with expiry: %w", err)
	}
	defer rows.Close()
	var out []ExpiringGrant
	for rows.Next() {
		var g ExpiringGrant
		if err := rows.Scan(&g.UserID, &g.ProjectID, &g.RoleKey, &g.ExpiresAt); err != nil {
			return nil, fmt.Errorf("scan grant with expiry: %w", err)
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// UsersFirstSeen is, per person, the earliest moment Syndra has any record of
// them: their first direct grant or bundle assignment. The directory carries no
// creation date, so this is "first seen", not "created"; a person nothing has
// ever been recorded for is absent.
//
// The grants index is not a source: it is rebuilt every sweep, so its created_at
// is the last sweep and would date everybody to this morning.
func UsersFirstSeen(ctx context.Context) (map[string]time.Time, error) {
	rows, err := querier(ctx).Query(ctx, `
		SELECT user_id, MIN(at) FROM (
			SELECT user_id, created_at AS at FROM direct_role_grants
			UNION ALL SELECT user_id, assigned_at FROM user_bundle_assignments
		) s WHERE at IS NOT NULL GROUP BY user_id`)
	if err != nil {
		return nil, fmt.Errorf("users first seen: %w", err)
	}
	defer rows.Close()
	out := map[string]time.Time{}
	for rows.Next() {
		var id string
		var at time.Time
		if err := rows.Scan(&id, &at); err != nil {
			return nil, fmt.Errorf("scan first seen: %w", err)
		}
		out[id] = at
	}
	return out, rows.Err()
}
