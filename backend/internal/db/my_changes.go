package db

import (
	"context"
	"fmt"
	"time"
)

// MyChangeRow is one change to a person's own access, as Syndra recorded it
// (B10, GET /me/changes). Raw facts only; the handler decides what a member
// is shown (who decided, which bundle or rule, whether it waits).
type MyChangeRow struct {
	// Kind: request_approved | request_declined | request_withdrawn | given | taken_away | cancelled_removal.
	Kind      string
	At        time.Time
	ProjectID string
	RoleKey   string
	// Actor is the decider (requests) or whoever queued the change (given,
	// taken away). Empty for a withdrawal, which the member did themselves.
	Actor string
	// Note is the decider's reason, or the grant's reason.
	Note string
	// Source and SourceRef say where a given or taken-away change came from:
	// direct, bundle (ref = bundle id), rule (ref = rule id), lifecycle_cascade.
	Source    string
	SourceRef string
	Until     *time.Time
	// RequestKind is a request's kind (access | extend); empty for given and
	// taken-away rows.
	RequestKind string
	// EndedBefore and EndsAfter are an extension's two ends, as settled.
	EndedBefore, EndsAfter *time.Time
	// Pending is an approval whose outbox row has not been sent yet, and
	// WaitReason is that row's propagation_outbox.wait_reason.
	Pending    bool
	WaitReason string
}

// Requests come from access_requests. A fulfilled request is left out: it was
// closed by a grant, and the grant is its own "given" row.
//
// Given and taken away come from the Zitadel outbox, not the audit log: a
// revoke's audit row carries no grant id (the ledger row is never written for
// a revoke), so only the outbox row, written in the same transaction, still
// names the project and role. A row cancelled before it was sent (superseded,
// abandoned) never reached the person and is not a change to their access.
// The row an approval queued is the approval itself, so it is not repeated.
//
// A cancelled removal is the one cancelled send the member hears about: it
// means they keep a role they were going to lose.
const myChangesSQL = `
SELECT kind, changed_at, project_id, role_key, actor, note, source, source_ref, ends_at, pending, wait_reason, request_kind, ended_before, ends_after
FROM (
	SELECT CASE ar.status WHEN 'approved' THEN 'request_approved'
	                      WHEN 'rejected' THEN 'request_declined'
	                      ELSE 'request_withdrawn' END                       AS kind,
	       ar.resolved_at                                                    AS changed_at,
	       ar.zitadel_project_id                                             AS project_id,
	       ar.zitadel_role_key                                               AS role_key,
	       COALESCE(ar.reviewer_user_id, '')                                 AS actor,
	       COALESCE(ar.review_note, '')                                      AS note,
	       ''                                                                AS source,
	       ''                                                                AS source_ref,
	       -- An extension counts from the end they had, so its end is the one the
	       -- decision recorded; the grant row may since have moved on. Before
	       -- 000063 nothing was recorded, and the grant is the best left.
	       CASE WHEN ar.status = 'approved' AND ar.kind = 'extend'
	                 THEN COALESCE(ar.ends_after, g.expires_at)
	            WHEN ar.status = 'approved' THEN COALESCE(g.expires_at, ar.asked_until,
	            CASE WHEN ar.duration_days > 0
	                 THEN ar.resolved_at + make_interval(days => ar.duration_days) END)
	       END                                                               AS ends_at,
	       w.id IS NOT NULL                                                  AS pending,
	       COALESCE(w.wait_reason, '')                                       AS wait_reason,
	       ar.kind                                                           AS request_kind,
	       ar.ended_before                                                   AS ended_before,
	       ar.ends_after                                                     AS ends_after
	  FROM access_requests ar
	  LEFT JOIN direct_role_grants g
	         ON ar.status = 'approved'
	        AND g.user_id = ar.requester_user_id
	        AND g.zitadel_project_id = ar.zitadel_project_id
	        AND g.zitadel_role_key = ar.zitadel_role_key
	        AND g.source_ref = ar.id::text
	  LEFT JOIN LATERAL (
	        SELECT o.id, o.wait_reason
	          FROM propagation_outbox o
	         WHERE ar.status = 'approved'
	           AND o.target = 'zitadel'
	           AND o.source_ref = ar.id::text
	           AND o.status = 'pending'
	         ORDER BY o.created_at DESC
	         LIMIT 1) w ON true
	 WHERE ar.requester_user_id = $1
	   AND ar.status IN ('approved', 'rejected', 'withdrawn')
	   AND ar.resolved_at >= NOW() - make_interval(days => $2::int)

	UNION ALL

	SELECT CASE o.op_type WHEN 'revoke' THEN 'taken_away' ELSE 'given' END,
	       o.created_at,
	       COALESCE(o.project_id, ''),
	       rk.role_key,
	       o.initiated_by,
	       COALESCE(g.reason, o.payload_json->>'reason', ''),
	       o.source,
	       COALESCE(o.source_ref, ''),
	       g.expires_at,
	       false,
	       '',
	       '',
	       NULL::timestamptz,
	       NULL::timestamptz
	  FROM propagation_outbox o
	 CROSS JOIN LATERAL unnest(o.role_keys) AS rk(role_key)
	  LEFT JOIN direct_role_grants g
	         ON o.op_type = 'add'
	        AND o.source = 'direct'
	        AND g.source = 'direct'
	        AND g.user_id = o.user_id
	        AND g.zitadel_project_id = o.project_id
	        AND g.zitadel_role_key = rk.role_key
	 WHERE o.user_id = $1
	   AND o.target = 'zitadel'
	   AND o.op_type IN ('add', 'revoke')
	   AND o.status NOT IN ('superseded', 'abandoned')
	   AND o.created_at >= NOW() - make_interval(days => $2::int)
	   AND NOT EXISTS (SELECT 1 FROM access_requests ar WHERE ar.id::text = o.source_ref)

	UNION ALL

	-- A removal that was queued and never sent, cancelled because the role was
	-- given back: they keep it. The audit entry (migration 000064) is the only
	-- record left once the outbox row is pruned. Nothing of who queued it or
	-- why is read: that is the operator's.
	SELECT 'cancelled_removal',
	       a.created_at,
	       COALESCE(a.details->'cancelled'->>'project_id', ''),
	       COALESCE(a.details->'cancelled'->>'role_key', ''),
	       '', '', '', '',
	       NULL::timestamptz,
	       false,
	       '',
	       '',
	       NULL::timestamptz,
	       NULL::timestamptz
	  FROM audit_logs a
	 WHERE a.target_zitadel_user_id = $1
	   AND a.action = 'propagation.cancelled_unsent'
	   AND a.details->'cancelled'->>'kind' = 'remove'
	   AND a.created_at >= NOW() - make_interval(days => $2::int)
) changes
ORDER BY changed_at DESC
LIMIT 200` // ponytail: a fixed cap on one person's month; page it if anyone outgrows it

// ListMyChanges returns the changes to one person's access in the last `days`
// days, newest first, whether or not they hold anything now.
func ListMyChanges(ctx context.Context, userID string, days int) ([]MyChangeRow, error) {
	rows, err := querier(ctx).Query(ctx, myChangesSQL, userID, days)
	if err != nil {
		return nil, fmt.Errorf("list my changes: %w", err)
	}
	defer rows.Close()
	out := []MyChangeRow{}
	for rows.Next() {
		var c MyChangeRow
		if err := rows.Scan(&c.Kind, &c.At, &c.ProjectID, &c.RoleKey, &c.Actor, &c.Note,
			&c.Source, &c.SourceRef, &c.Until, &c.Pending, &c.WaitReason, &c.RequestKind, &c.EndedBefore, &c.EndsAfter); err != nil {
			return nil, fmt.Errorf("scan my change: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
