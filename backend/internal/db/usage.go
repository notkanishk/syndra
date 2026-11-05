package db

import (
	"context"
	"fmt"
	"time"

	"syndra/internal/clock"

	"github.com/jackc/pgx/v5"
)

// Usage kinds, as the usage_last_seen CHECK constraint spells them.
const (
	UsageSignIn = "sign_in"
	UsageBadge  = "badge"
)

// UsageRow is one person's last use of one app (or, with no app, one project)
// by one kind. AppID ” is "app unknown": a row from before sign-ins were kept
// per app (migration 69), or a sign-in whose application did not resolve.
type UsageRow struct {
	ProjectID string    `json:"project_id"`
	AppID     string    `json:"app_id"`
	Kind      string    `json:"kind"`
	LastAt    time.Time `json:"last_at"`
}

// RecordUsage notes that userID used appID of projectID (either ” when
// unknown) at `at`. The later time always wins, so a redelivered or
// out-of-order event can never move "last seen" backwards.
//
// A sign-in also adds one to its app's total for that day, in the same
// statement and only when the row moved strictly later. A redelivery carries
// the same time, so it is never counted twice.
// ponytail: a distinct sign-in that arrives out of order (older than the
// person's last one to that app) is not counted; an event-id ledger is the
// upgrade if the totals ever need to be exact.
func RecordUsage(ctx context.Context, userID, projectID, appID, kind string, at time.Time) error {
	_, err := querier(ctx).Exec(ctx, `
		WITH seen AS (
			INSERT INTO usage_last_seen (user_id, project_id, app_id, kind, last_at)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (user_id, project_id, app_id, kind)
			DO UPDATE SET last_at = EXCLUDED.last_at WHERE usage_last_seen.last_at < EXCLUDED.last_at
			RETURNING kind
		)
		INSERT INTO sign_in_daily (app_id, day, count)
		SELECT $3, $6::date, 1 FROM seen WHERE seen.kind = 'sign_in'
		ON CONFLICT (app_id, day) DO UPDATE SET count = sign_in_daily.count + 1`,
		userID, projectID, appID, kind, at, clock.Day(at))
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
// `kind` at or after `since`, through any of its apps. project ” (app
// unknown) is left out: it is not use of any project.
func UsersWhoUsedProjects(ctx context.Context, kind string, since time.Time) (map[string]map[string]bool, error) {
	rows, err := querier(ctx).Query(ctx, `
		SELECT DISTINCT user_id, project_id FROM usage_last_seen
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
		SELECT project_id, app_id, kind, last_at FROM usage_last_seen
		 WHERE user_id = $1 ORDER BY last_at DESC, project_id, app_id, kind`, userID)
	if err != nil {
		return nil, fmt.Errorf("get user usage: %w", err)
	}
	defer rows.Close()
	out := []UsageRow{}
	for rows.Next() {
		var r UsageRow
		if err := rows.Scan(&r.ProjectID, &r.AppID, &r.Kind, &r.LastAt); err != nil {
			return nil, fmt.Errorf("scan user usage: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// PruneUsage deletes rows last seen before `before`, and daily totals for the
// days before it, and says how many last-seen rows went.
func PruneUsage(ctx context.Context, before time.Time) (int64, error) {
	tag, err := querier(ctx).Exec(ctx, `DELETE FROM usage_last_seen WHERE last_at < $1`, before)
	if err != nil {
		return 0, fmt.Errorf("prune usage: %w", err)
	}
	if _, err := querier(ctx).Exec(ctx, `DELETE FROM sign_in_daily WHERE day < $1::date`, clock.Day(before)); err != nil {
		return 0, fmt.Errorf("prune sign-in totals: %w", err)
	}
	return tag.RowsAffected(), nil
}

// SignInsSince is the earliest moment Syndra is known to have been receiving
// sign-ins: the oldest last-seen time or daily total still kept. Nil when there
// is neither. A lower bound, so "no sign-in since then" is always true. A day
// starts at the makerspace's midnight, not the session's (the cluster's is UTC).
func SignInsSince(ctx context.Context) (*time.Time, error) {
	var at *time.Time
	err := querier(ctx).QueryRow(ctx, `
		SELECT LEAST(
			(SELECT MIN(last_at) FROM usage_last_seen WHERE kind = 'sign_in'),
			(SELECT MIN(day)::timestamp AT TIME ZONE $1 FROM sign_in_daily))`, clock.Name).Scan(&at)
	if err != nil {
		return nil, fmt.Errorf("sign-ins since: %w", err)
	}
	return at, nil
}

// SignInDay is one app's total for one makerspace day.
type SignInDay struct {
	Day   string `json:"day"` // YYYY-MM-DD
	Count int    `json:"count"`
}

// AppUsage is one app's sign-ins, naming nobody: how many people signed in at
// or after `recent` and at or after `window` (service accounts left out), its
// latest sign-in, and its daily totals from `window` on.
type AppUsage struct {
	RecentPeople int         `json:"recent_people"`
	WindowPeople int         `json:"window_people"`
	LastAt       *time.Time  `json:"last_at,omitempty"`
	Days         []SignInDay `json:"days"`
}

// GetAppUsage reads AppUsage for one application id. Days is never nil.
//
// Distinct people from the last-seen rows is exact for any window: whoever
// signed in inside it has a last sign-in inside it.
func GetAppUsage(ctx context.Context, appID string, recent, window time.Time) (AppUsage, error) {
	out := AppUsage{Days: []SignInDay{}}
	if err := querier(ctx).QueryRow(ctx, `
		SELECT COUNT(DISTINCT user_id) FILTER (WHERE last_at >= $2),
		       COUNT(DISTINCT user_id) FILTER (WHERE last_at >= $3),
		       MAX(last_at)
		  FROM usage_last_seen
		 WHERE app_id = $1 AND kind = 'sign_in'
		   AND user_id NOT IN (SELECT user_id FROM service_accounts)`,
		appID, recent, window).Scan(&out.RecentPeople, &out.WindowPeople, &out.LastAt); err != nil {
		return AppUsage{}, fmt.Errorf("app usage: %w", err)
	}
	rows, err := querier(ctx).Query(ctx, `
		SELECT to_char(day, 'YYYY-MM-DD'), count FROM sign_in_daily
		 WHERE app_id = $1 AND day >= $2::date ORDER BY day`, appID, clock.Day(window))
	if err != nil {
		return AppUsage{}, fmt.Errorf("app sign-in days: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var d SignInDay
		if err := rows.Scan(&d.Day, &d.Count); err != nil {
			return AppUsage{}, fmt.Errorf("scan sign-in day: %w", err)
		}
		out.Days = append(out.Days, d)
	}
	return out, rows.Err()
}

// SignInDayKey is one app ("" is "app unknown") on one makerspace day.
type SignInDayKey struct{ AppID, Day string }

// SignInRecountPending is what 000079 left to recount: the first day
// sign_in_daily holds and the last one still on the UTC day. through is nil
// when nothing waits; first is nil when no total is left at all.
func SignInRecountPending(ctx context.Context) (first, through *string, err error) {
	err = querier(ctx).QueryRow(ctx, `
		SELECT (SELECT MIN(day)::text FROM sign_in_daily),
		       (SELECT MAX(through)::text FROM sign_in_daily_recount)`).Scan(&first, &through)
	if err != nil {
		return nil, nil, fmt.Errorf("sign-in recount pending: %w", err)
	}
	return first, through, nil
}

// ReplaceSignInDays swaps every total from `first` through `through`
// (inclusive days) for `counts`, and marks the recount done, in one
// transaction: a reader sees the old totals or the new ones, never a mix.
func ReplaceSignInDays(ctx context.Context, first, through string, counts map[SignInDayKey]int) error {
	return InTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM sign_in_daily WHERE day BETWEEN $1::date AND $2::date`, first, through); err != nil {
			return fmt.Errorf("clear sign-in totals: %w", err)
		}
		for k, n := range counts {
			if n < 1 || k.Day < first || k.Day > through {
				continue // only the days asked for, and only days with a sign-in
			}
			if _, err := tx.Exec(ctx, `INSERT INTO sign_in_daily (app_id, day, count) VALUES ($1, $2::date, $3)`, k.AppID, k.Day, n); err != nil {
				return fmt.Errorf("write sign-in total: %w", err)
			}
		}
		if _, err := tx.Exec(ctx, `DELETE FROM sign_in_daily_recount WHERE through <= $1::date`, through); err != nil {
			return fmt.Errorf("close sign-in recount: %w", err)
		}
		return nil
	})
}
