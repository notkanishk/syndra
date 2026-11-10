package db

import (
	"context"
	"fmt"
)

// RoleDelivery is the latest Zitadel write Syndra made, or owes, for one
// person on one project: an add or revoke for one role, or a replace that
// states the project's whole role set (it gives what it names and takes the
// rest). The standing of a role reads the later of its add/revoke and its
// project's replace.
type RoleDelivery struct {
	ProjectID string
	// RoleKey is the role an add or revoke names; empty on a replace.
	RoleKey string
	OpType  string
	// RoleKeys is a replace's whole set; nil on an add or revoke.
	RoleKeys  []string
	Status    string
	Confirmed bool
	IntentSeq int64
}

// RoleDeliveries is, per person, the latest add/revoke for each role and the
// latest replace for each project, in Zitadel's outbox. A cancelled
// (superseded) or abandoned row never happened, so it never stands in front of
// the row before it.
//
// ponytail: two DISTINCT ON scans over the person's outbox rows, unindexed on
// user_id like PendingDeliveriesForUser; tens of rows per person here. Index
// (target, user_id, intent_seq) if a person's history grows.
func RoleDeliveries(ctx context.Context, userIDs []string) (map[string][]RoleDelivery, error) {
	const q = `
		WITH o AS (
			SELECT user_id, project_id, op_type, role_keys, status,
			       confirmed_at IS NOT NULL AS confirmed, intent_seq
			  FROM propagation_outbox
			 WHERE target = 'zitadel' AND user_id = ANY($1)
			   AND project_id IS NOT NULL AND role_keys IS NOT NULL
			   AND status NOT IN ('superseded', 'abandoned')
		)
		(SELECT DISTINCT ON (user_id, project_id, role_key)
		        user_id, project_id, role_key, op_type, NULL::text[], status, confirmed, intent_seq
		   FROM o, unnest(role_keys) AS role_key
		  WHERE op_type IN ('add', 'revoke')
		  ORDER BY user_id, project_id, role_key, intent_seq DESC)
		UNION ALL
		(SELECT DISTINCT ON (user_id, project_id)
		        user_id, project_id, '', op_type, role_keys, status, confirmed, intent_seq
		   FROM o
		  WHERE op_type = 'replace'
		  ORDER BY user_id, project_id, intent_seq DESC)`
	rows, err := querier(ctx).Query(ctx, q, userIDs)
	if err != nil {
		return nil, fmt.Errorf("read role deliveries: %w", err)
	}
	defer rows.Close()
	out := make(map[string][]RoleDelivery, len(userIDs))
	for rows.Next() {
		var u string
		var d RoleDelivery
		if err := rows.Scan(&u, &d.ProjectID, &d.RoleKey, &d.OpType, &d.RoleKeys, &d.Status, &d.Confirmed, &d.IntentSeq); err != nil {
			return nil, fmt.Errorf("read role deliveries: %w", err)
		}
		out[u] = append(out[u], d)
	}
	return out, rows.Err()
}
