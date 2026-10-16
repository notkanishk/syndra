package db

import (
	"context"
	"fmt"
	"time"
)

// Usage kinds, as the usage_last_seen CHECK constraint spells them.
const (
	UsageSignIn = "sign_in"
	UsageBadge  = "badge"
)

// UsageRow is one person's last use of one project by one kind.
type UsageRow struct {
	ProjectID string    `json:"project_id"`
	Kind      string    `json:"kind"`
	LastAt    time.Time `json:"last_at"`
}

// RecordUsage notes that userID used projectID (” = app unknown) at `at`. The
// later time always wins, so a redelivered or out-of-order event can never move
// "last seen" backwards, and replaying one is a no-op.
func RecordUsage(ctx context.Context, userID, projectID, kind string, at time.Time) error {
	_, err := querier(ctx).Exec(ctx, `
		INSERT INTO usage_last_seen (user_id, project_id, kind, last_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, project_id, kind)
		DO UPDATE SET last_at = GREATEST(usage_last_seen.last_at, EXCLUDED.last_at)`,
		userID, projectID, kind, at)
	if err != nil {
		return fmt.Errorf("record usage: %w", err)
	}
	return nil
}

// UsageLatestByKind is, per kind, the most recent last_at at or after `since`.
// A kind with no row in the window is absent from the map.
func UsageLatestByKind(ctx context.Context, since time.Time) (map[string]time.Time, error) {
	rows, err := querier(ctx).Query(ctx,
		`SELECT kind, MAX(last_at) FROM usage_last_seen WHERE last_at >= $1 GROUP BY kind`, since)
	if err != nil {
		return nil, fmt.Errorf("usage latest by kind: %w", err)
	}
	defer rows.Close()
	out := map[string]time.Time{}
	for rows.Next() {
		var kind string
		var at time.Time
		if err := rows.Scan(&kind, &at); err != nil {
			return nil, fmt.Errorf("scan usage latest: %w", err)
		}
		out[kind] = at
	}
	return out, rows.Err()
}

// UsersWhoUsedProjects is, per user, the set of known projects they used by
// `kind` at or after `since`. project ” (app unknown) is left out: it is not
// use of any project.
func UsersWhoUsedProjects(ctx context.Context, kind string, since time.Time) (map[string]map[string]bool, error) {
	rows, err := querier(ctx).Query(ctx, `
		SELECT user_id, project_id FROM usage_last_seen
		 WHERE kind = $1 AND last_at >= $2 AND project_id <> ''`, kind, since)
	if err != nil {
		return nil, fmt.Errorf("users who used projects: %w", err)
	}
	defer rows.Close()
	out := map[string]map[string]bool{}
	for rows.Next() {
		var user, project string
		if err := rows.Scan(&user, &project); err != nil {
			return nil, fmt.Errorf("scan usage: %w", err)
		}
		if out[user] == nil {
			out[user] = map[string]bool{}
		}
		out[user][project] = true
	}
	return out, rows.Err()
}

// GetUserUsage is one person's rows, most recent first. Never nil.
func GetUserUsage(ctx context.Context, userID string) ([]UsageRow, error) {
	rows, err := querier(ctx).Query(ctx, `
		SELECT project_id, kind, last_at FROM usage_last_seen
		 WHERE user_id = $1 ORDER BY last_at DESC, project_id, kind`, userID)
	if err != nil {
		return nil, fmt.Errorf("get user usage: %w", err)
	}
	defer rows.Close()
	out := []UsageRow{}
	for rows.Next() {
		var r UsageRow
		if err := rows.Scan(&r.ProjectID, &r.Kind, &r.LastAt); err != nil {
			return nil, fmt.Errorf("scan user usage: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// PruneUsage deletes rows last seen before `before` and says how many.
func PruneUsage(ctx context.Context, before time.Time) (int64, error) {
	tag, err := querier(ctx).Exec(ctx, `DELETE FROM usage_last_seen WHERE last_at < $1`, before)
	if err != nil {
		return 0, fmt.Errorf("prune usage: %w", err)
	}
	return tag.RowsAffected(), nil
}
