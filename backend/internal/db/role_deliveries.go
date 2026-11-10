package db

import (
	"context"
	"fmt"
	"slices"
)

// RoleDelivery is one outbox row as it bears on a role: an add or revoke for
// one role, or a replace that states a project's whole role set (it gives what
// it names and takes the rest).
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

// Gives says whether this row gives roleKey (true) or takes it away (false).
// The one statement of what a row means for a role.
func (d RoleDelivery) Gives(roleKey string) bool {
	return d.OpType == "add" || (d.OpType == "replace" && slices.Contains(d.RoleKeys, roleKey))
}

// Deliveries is one person's latest rows: per role its latest add or revoke,
// per project its latest replace. Latest picks between the two, so "which came
// last" has one answer for every reader.
type Deliveries struct {
	byRole    map[RoleRef]RoleDelivery
	byProject map[string]RoleDelivery
}

// Latest is the row that decides roleKey on projectID: the later (by
// intent_seq, the order Syndra decided them in) of the role's own add/revoke
// and the project's replace. ok is false when neither exists.
func (ds Deliveries) Latest(projectID, roleKey string) (RoleDelivery, bool) {
	d, ok := ds.byRole[RoleRef{ProjectID: projectID, RoleKey: roleKey}]
	if r, rok := ds.byProject[projectID]; rok && (!ok || r.IntentSeq > d.IntentSeq) {
		return r, true
	}
	return d, ok
}

// Roles is every role a row names, with its project: the roles Latest can say
// something about without being asked by name.
func (ds Deliveries) Roles() []RoleRef {
	var out []RoleRef
	for ref := range ds.byRole {
		out = append(out, ref)
	}
	for _, d := range ds.byProject {
		for _, rk := range d.RoleKeys {
			out = append(out, RoleRef{ProjectID: d.ProjectID, RoleKey: rk})
		}
	}
	return out
}

// Live is every row status that stands for a decision: cancelled (superseded)
// and abandoned rows never happened, so they never stand in front of the row
// before them.
var Live = []string{"pending", "in_flight", "applied", "failed"}

// RoleDeliveries is each person's latest Zitadel rows among Live statuses: the
// outbox half of a role's standing (services.standingOf).
func RoleDeliveries(ctx context.Context, userIDs []string) (map[string]Deliveries, error) {
	return latestDeliveries(ctx, querier(ctx), "zitadel", userIDs, Live)
}

// latestDeliveries is the one read of "the latest row per role": per person on
// target, among rows whose status is in statuses, the latest add/revoke per
// role and the latest replace per project.
//
// ponytail: two DISTINCT ON scans over the people's outbox rows, unindexed on
// user_id; tens of rows per person here. Index (target, user_id, intent_seq)
// if a person's history grows.
func latestDeliveries(ctx context.Context, q Querier, target string, userIDs, statuses []string) (map[string]Deliveries, error) {
	const sql = `
		WITH o AS (
			SELECT user_id, project_id, op_type, role_keys, status,
			       confirmed_at IS NOT NULL AS confirmed, intent_seq
			  FROM propagation_outbox
			 WHERE target = $1 AND user_id = ANY($2) AND status = ANY($3)
			   AND project_id IS NOT NULL AND role_keys IS NOT NULL
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
	rows, err := q.Query(ctx, sql, target, userIDs, statuses)
	if err != nil {
		return nil, fmt.Errorf("read latest deliveries: %w", err)
	}
	defer rows.Close()
	byUser := map[string][]RoleDelivery{}
	for rows.Next() {
		var u string
		var d RoleDelivery
		if err := rows.Scan(&u, &d.ProjectID, &d.RoleKey, &d.OpType, &d.RoleKeys, &d.Status, &d.Confirmed, &d.IntentSeq); err != nil {
			return nil, fmt.Errorf("read latest deliveries: %w", err)
		}
		byUser[u] = append(byUser[u], d)
	}
	out := make(map[string]Deliveries, len(byUser))
	for u, ds := range byUser {
		out[u] = NewDeliveries(ds...)
	}
	return out, rows.Err()
}

// NewDeliveries builds a person's Deliveries from rows already chosen as the
// latest (one add/revoke per role, one replace per project).
func NewDeliveries(rows ...RoleDelivery) Deliveries {
	ds := Deliveries{byRole: map[RoleRef]RoleDelivery{}, byProject: map[string]RoleDelivery{}}
	for _, d := range rows {
		if d.OpType == "replace" {
			ds.byProject[d.ProjectID] = d
		} else {
			ds.byRole[RoleRef{ProjectID: d.ProjectID, RoleKey: d.RoleKey}] = d
		}
	}
	return ds
}
