package db

import (
	"context"
	"fmt"
	"time"
)

// AccessSpan is one source's hold on one role, as Syndra's records date it
// (A-24 §9, access over time). From nil: no record says when it began. To nil:
// the record stands today.
type AccessSpan struct {
	ProjectID string
	RoleKey   string
	From, To  *time.Time
}

// accessHistorySQL is every dated span the records can place for one person,
// apart from live direct grants (GetDirectGrantsForUser has those, created_at
// and all) and rule roles (no record dates them). Only records that name the
// role, or name a bundle whose version names it, are read:
//
//   - A bundle held today: from its assignment, through its pinned version. A
//     holder moved between versions has no from: assigned_at is the first
//     version's day, and a role the move added began at the move.
//   - A direct grant that ended: direct_grant.removed and .revoked_by_expiry
//     name "project/role". The ledger row is deleted with it, and the
//     direct_grant.upserted row that gave it names only that row's id, so its
//     start is known only when an approved request gave it: the latest one for
//     the role with no other ending between the two.
//   - A bundle that ended: bundle.unassigned names the bundle. Its roles are the
//     version the person was moved to last, or else the one the assignment
//     pinned, which is the version latest published when it was made
//     (AssignBundleAndEnqueue pins the caller's latest). From is that
//     assignment, unless they were moved.
//
// direct_grant.revoked (a role Zitadel had that Syndra never gave) names no
// role, and nothing dates a rule's role, so neither is here. Nor is a retired
// bundle: RetireBundleAndEnqueue deletes every assignment at once, with no
// audit row for each holder, so its runs end where no record says.
//
// Every audit read, the lateral ones too, is by target, action and time:
// idx_audit_logs_target_action_created (000102).
const accessHistorySQL = `
SELECT project_id, role_key, from_at, to_at FROM (
	SELECT r.zitadel_project_id AS project_id, r.zitadel_role_key AS role_key,
	       CASE WHEN uba.source = 'moved' THEN NULL ELSE uba.assigned_at END AS from_at,
	       NULL::timestamptz AS to_at
	  FROM user_bundle_assignments uba
	  JOIN bundle_version_effective_roles r ON r.version_id = uba.version_id
	 WHERE uba.user_id = $1

	UNION ALL

	SELECT split_part(e.resource_id, '/', 1),
	       substr(e.resource_id, strpos(e.resource_id, '/') + 1),
	       g.resolved_at,
	       e.created_at
	  FROM audit_logs e
	  LEFT JOIN LATERAL (
	        SELECT ar.resolved_at
	          FROM access_requests ar
	         WHERE ar.requester_user_id = $1
	           AND ar.zitadel_project_id || '/' || ar.zitadel_role_key = e.resource_id
	           AND ar.status = 'approved' AND ar.kind = 'access'
	           AND ar.resolved_at <= e.created_at
	           AND NOT EXISTS (
	                 SELECT 1 FROM audit_logs e2
	                  WHERE e2.target_zitadel_user_id = $1
	                    AND e2.action IN ('direct_grant.removed', 'direct_grant.revoked_by_expiry')
	                    AND e2.resource_id = e.resource_id
	                    AND e2.created_at > ar.resolved_at AND e2.created_at < e.created_at)
	         ORDER BY ar.resolved_at DESC
	         LIMIT 1) g ON true
	 WHERE e.target_zitadel_user_id = $1
	   AND e.action IN ('direct_grant.removed', 'direct_grant.revoked_by_expiry')
	   AND strpos(e.resource_id, '/') > 0
	   AND e.created_at >= $2

	UNION ALL

	SELECT r.zitadel_project_id, r.zitadel_role_key,
	       CASE WHEN mv.version_id IS NULL THEN asg.created_at END,
	       u.created_at
	  FROM audit_logs u
	  LEFT JOIN LATERAL (
	        SELECT a.created_at FROM audit_logs a
	         WHERE a.target_zitadel_user_id = $1 AND a.action = 'bundle.assigned'
	           AND a.resource_id = u.resource_id AND a.created_at <= u.created_at
	         ORDER BY a.created_at DESC LIMIT 1) asg ON true
	  LEFT JOIN LATERAL (
	        SELECT v.id AS version_id FROM audit_logs m
	          JOIN bundle_versions v ON v.id::text = m.resource_id AND v.bundle_id::text = u.resource_id
	         WHERE m.target_zitadel_user_id = $1 AND m.action = 'bundle.holder_moved'
	           AND m.created_at <= u.created_at
	           AND (asg.created_at IS NULL OR m.created_at >= asg.created_at)
	         ORDER BY m.created_at DESC LIMIT 1) mv ON true
	  JOIN LATERAL (
	        SELECT v.id FROM bundle_versions v
	         WHERE v.bundle_id::text = u.resource_id
	           AND (v.id = mv.version_id OR (mv.version_id IS NULL AND v.published_at <= asg.created_at))
	         ORDER BY v.version DESC LIMIT 1) ver ON true
	  JOIN bundle_version_effective_roles r ON r.version_id = ver.id
	 WHERE u.target_zitadel_user_id = $1
	   AND u.action = 'bundle.unassigned'
	   AND u.created_at >= $2
) spans
ORDER BY to_at DESC NULLS FIRST
LIMIT $3`

// AccessHistory is userID's dated spans since `since`: every bundle held today
// first, then the spans that ended, newest first, at most `limit` rows.
func AccessHistory(ctx context.Context, userID string, since time.Time, limit int) ([]AccessSpan, error) {
	rows, err := querier(ctx).Query(ctx, accessHistorySQL, userID, since, limit)
	if err != nil {
		return nil, fmt.Errorf("access history for %s: %w", userID, err)
	}
	defer rows.Close()
	var out []AccessSpan
	for rows.Next() {
		var s AccessSpan
		if err := rows.Scan(&s.ProjectID, &s.RoleKey, &s.From, &s.To); err != nil {
			return nil, fmt.Errorf("scan access span: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
