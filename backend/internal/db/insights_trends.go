package db

import (
	"context"
	"fmt"
	"time"

	"syndra/internal/models"
)

// The aggregates behind Insights trends (I4). Each is ONE read over a window
// [from, to): counts and the raw durations a median needs, never a row at a time.
// Medians and bucketing are the service's, so they can be tested without a
// database.

// TrendDay is how many audit rows of one action landed on one local day.
type TrendDay struct {
	Day    time.Time // a date; only its year/month/day mean anything
	Action string
	N      int
}

// AuditDayCounts counts audit rows of the given actions per day and action.
//
// An approved request is written twice: access_request.approved, and the direct
// grant it makes (direct_grant.upserted, whose resource_id is that grant). The
// second is left out when the grant still points back at a request through
// source_ref, so a request is counted once, as a request.
// ponytail: the pointer is the grant's CURRENT source_ref; a grant since edited
// by hand or deleted counts as by hand. Exact needs the audit row to carry its source.
//
// offsetSecs is the server's UTC offset: created_at is a timestamptz and "a day"
// is the operator's day, not Postgres's session zone, which nothing here sets.
// ponytail: one offset for the whole window, so a day within an hour of a DST
// change can land a day off. Fixed-offset zones (IST) are exact; name the zone
// and AT TIME ZONE it if this ever runs somewhere that observes DST.
func AuditDayCounts(ctx context.Context, from, to time.Time, actions []string, offsetSecs int) ([]TrendDay, error) {
	rows, err := querier(ctx).Query(ctx, `
		SELECT ((created_at AT TIME ZONE 'UTC') + $3::int * interval '1 second')::date AS day,
		       action, COUNT(*)
		  FROM audit_logs a
		 WHERE created_at >= $1 AND created_at < $2 AND action = ANY($4::text[])
		   AND NOT (action IN ('direct_grant.upserted', 'direct_grant.replaced')
		            AND EXISTS (SELECT 1 FROM direct_role_grants g
		                          JOIN access_requests r ON r.id::text = g.source_ref
		                         WHERE g.id::text = a.resource_id))
		 GROUP BY 1, 2
		 ORDER BY 1, 2`, from, to, offsetSecs, actions)
	if err != nil {
		return nil, fmt.Errorf("audit day counts: %w", err)
	}
	defer rows.Close()
	var out []TrendDay
	for rows.Next() {
		var d TrendDay
		if err := rows.Scan(&d.Day, &d.Action, &d.N); err != nil {
			return nil, fmt.Errorf("scan audit day count: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// RequestTrendRows is what the requests in a window came to. DecideSeconds is
// resolved_at - created_at for approved and declined requests that have one;
// DeclineNotes is every declined request's review note (” when none).
type RequestTrendRows struct {
	CameIn, Approved, Declined, Withdrawn int
	DecideSeconds                         []float64
	DeclineNotes                          []string
}

// RequestsInWindow reads requests CREATED in [from, to). 'rejected' is the
// stored word for declined; 'fulfilled' (closed because the role was given
// another way) counts as came in and nothing else.
func RequestsInWindow(ctx context.Context, from, to time.Time) (RequestTrendRows, error) {
	var r RequestTrendRows
	err := querier(ctx).QueryRow(ctx, `
		SELECT COUNT(*),
		       COUNT(*) FILTER (WHERE status = 'approved'),
		       COUNT(*) FILTER (WHERE status = 'rejected'),
		       COUNT(*) FILTER (WHERE status = 'withdrawn'),
		       array_agg(EXTRACT(EPOCH FROM resolved_at - created_at)::float8)
		           FILTER (WHERE status IN ('approved', 'rejected') AND resolved_at IS NOT NULL),
		       array_agg(COALESCE(review_note, '')) FILTER (WHERE status = 'rejected')
		  FROM access_requests
		 WHERE created_at >= $1 AND created_at < $2`, from, to).
		Scan(&r.CameIn, &r.Approved, &r.Declined, &r.Withdrawn, &r.DecideSeconds, &r.DeclineNotes)
	if err != nil {
		return r, fmt.Errorf("requests in window: %w", err)
	}
	return r, nil
}

// OutsideBundleRoles is the roles asked for in the window that the LATEST
// version of no bundle gives, most asked first. It reads the effective view, so
// a role a bundle gives through an include is given.
func OutsideBundleRoles(ctx context.Context, from, to time.Time, limit int) ([]models.TrendOutsideRole, error) {
	rows, err := querier(ctx).Query(ctx, `
		WITH latest AS (
			SELECT DISTINCT ON (v.bundle_id) v.id FROM bundle_versions v
			  JOIN bundles b ON b.id = v.bundle_id AND b.retired_at IS NULL
			 ORDER BY v.bundle_id, v.version DESC
		), given AS (
			SELECT DISTINCT e.zitadel_project_id AS p, e.zitadel_role_key AS k
			  FROM bundle_version_effective_roles e JOIN latest l ON l.id = e.version_id
		)
		SELECT r.zitadel_project_id, r.zitadel_role_key, COUNT(*) AS n
		  FROM access_requests r
		 WHERE r.created_at >= $1 AND r.created_at < $2
		   AND NOT EXISTS (SELECT 1 FROM given g
		                    WHERE g.p = r.zitadel_project_id AND g.k = r.zitadel_role_key)
		 GROUP BY 1, 2
		 ORDER BY n DESC, 1, 2
		 LIMIT $3`, from, to, limit)
	if err != nil {
		return nil, fmt.Errorf("outside-bundle roles: %w", err)
	}
	defer rows.Close()
	out := []models.TrendOutsideRole{}
	for rows.Next() {
		var o models.TrendOutsideRole
		if err := rows.Scan(&o.ProjectID, &o.RoleKey, &o.N); err != nil {
			return nil, fmt.Errorf("scan outside-bundle role: %w", err)
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// SendSeconds is, per applied outbox row whose source_ref is a request approved
// in [from, to), completed_at - resolved_at. Window is the APPROVAL's.
//
// ponytail: finished outbox rows are pruned after a while, so for an old window
// this is a sample of the approvals, not all of them. The caller reports the
// sample size beside the median; a durable send timestamp is the upgrade.
func SendSeconds(ctx context.Context, from, to time.Time) ([]float64, error) {
	rows, err := querier(ctx).Query(ctx, `
		SELECT GREATEST(EXTRACT(EPOCH FROM o.completed_at - r.resolved_at), 0)::float8
		  FROM propagation_outbox o
		  JOIN access_requests r ON r.id::text = o.source_ref
		 WHERE r.status = 'approved' AND r.resolved_at >= $1 AND r.resolved_at < $2
		   AND o.target = 'zitadel' AND o.status = 'applied' AND o.completed_at IS NOT NULL`, from, to)
	if err != nil {
		return nil, fmt.Errorf("send seconds: %w", err)
	}
	defer rows.Close()
	var out []float64
	for rows.Next() {
		var s float64
		if err := rows.Scan(&s); err != nil {
			return nil, fmt.Errorf("scan send seconds: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// DriftTrendRows: Found is drift detected in the window; Cleared is drift
// resolved in it (whenever it was found); ClearSeconds is detected_at to
// resolved_at for the cleared ones.
type DriftTrendRows struct {
	Found, Cleared int
	ClearSeconds   []float64
}

func DriftInWindow(ctx context.Context, from, to time.Time) (DriftTrendRows, error) {
	var d DriftTrendRows
	err := querier(ctx).QueryRow(ctx, `
		SELECT COUNT(*) FILTER (WHERE detected_at >= $1 AND detected_at < $2),
		       COUNT(*) FILTER (WHERE resolved_at >= $1 AND resolved_at < $2),
		       array_agg(EXTRACT(EPOCH FROM resolved_at - detected_at)::float8)
		           FILTER (WHERE resolved_at >= $1 AND resolved_at < $2)
		  FROM drift_items
		 WHERE (detected_at >= $1 AND detected_at < $2) OR (resolved_at >= $1 AND resolved_at < $2)`,
		from, to).Scan(&d.Found, &d.Cleared, &d.ClearSeconds)
	if err != nil {
		return d, fmt.Errorf("drift in window: %w", err)
	}
	return d, nil
}

// EndingSoonRow is how many distinct people hold a direct grant in a project
// that ends within 30, 60 and 90 days of now (cumulative).
type EndingSoonRow struct {
	ProjectID     string
	D30, D60, D90 int
}

func GrantsEndingSoon(ctx context.Context, now time.Time) ([]EndingSoonRow, error) {
	rows, err := querier(ctx).Query(ctx, `
		SELECT zitadel_project_id,
		       COUNT(DISTINCT user_id) FILTER (WHERE expires_at <= $1::timestamptz + interval '30 days'),
		       COUNT(DISTINCT user_id) FILTER (WHERE expires_at <= $1::timestamptz + interval '60 days'),
		       COUNT(DISTINCT user_id)
		  FROM direct_role_grants
		 WHERE expires_at > $1 AND expires_at <= $1::timestamptz + interval '90 days'
		 GROUP BY 1
		 ORDER BY 1`, now)
	if err != nil {
		return nil, fmt.Errorf("grants ending soon: %w", err)
	}
	defer rows.Close()
	var out []EndingSoonRow
	for rows.Next() {
		var e EndingSoonRow
		if err := rows.Scan(&e.ProjectID, &e.D30, &e.D60, &e.D90); err != nil {
			return nil, fmt.Errorf("scan grants ending soon: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
