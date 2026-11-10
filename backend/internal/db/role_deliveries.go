package db

import (
	"context"
	"fmt"
	"slices"
	"time"
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
	// CompletedAt is when Zitadel answered it (applied, failed); nil while queued.
	CompletedAt *time.Time
	IntentSeq   int64
}

// Gives says whether this row gives roleKey (true) or takes it away (false).
// The one statement of what a row means for a role.
func (d RoleDelivery) Gives(roleKey string) bool {
	return d.OpType == "add" || (d.OpType == "replace" && slices.Contains(d.RoleKeys, roleKey))
}

// Queued says the row is still Syndra's to send: decided, not yet in Zitadel.
func (d RoleDelivery) Queued() bool { return d.Status == "pending" || d.Status == "in_flight" }

// latest is, per role, the latest add or revoke and, per project, the latest
// replace, of one class of rows.
type latest struct {
	byRole    map[RoleRef]RoleDelivery
	byProject map[string]RoleDelivery
}

// at is the row that decides roleKey: the later (by intent_seq) of the role's
// own add/revoke and its project's replace.
func (l latest) at(projectID, roleKey string) (RoleDelivery, bool) {
	d, ok := l.byRole[RoleRef{ProjectID: projectID, RoleKey: roleKey}]
	if r, rok := l.byProject[projectID]; rok && (!ok || r.IntentSeq > d.IntentSeq) {
		return r, true
	}
	return d, ok
}

// Deliveries is one person's latest rows, in two classes kept apart: queued
// (still Syndra's to send) and settled (Zitadel answered).
//
// Apart, because rows are not always sent in intent order: an operator sends
// one by id, a retry waits out its backoff while a later row goes. A newer
// settled row says nothing about an older queued one, which will still be
// sent; among queued rows the latest decides (a give queued behind a revoke is
// the one that lands, since the drain claims them in intent order).
type Deliveries struct {
	queued, settled latest
}

// LatestQueued is the latest queued row deciding roleKey, if any.
func (ds Deliveries) LatestQueued(projectID, roleKey string) (RoleDelivery, bool) {
	return ds.queued.at(projectID, roleKey)
}

// LatestSettled is the latest settled row deciding roleKey, if any.
func (ds Deliveries) LatestSettled(projectID, roleKey string) (RoleDelivery, bool) {
	return ds.settled.at(projectID, roleKey)
}

// Taking says a removal of roleKey is on its way: the latest queued row for it
// takes it. The one answer to "is a removal of this queued", read by the
// role's standing, the request check and the cascades (QueuedRevocations).
func (ds Deliveries) Taking(projectID, roleKey string) bool {
	d, ok := ds.LatestQueued(projectID, roleKey)
	return ok && !d.Gives(roleKey)
}

// Giving says a give of roleKey is on its way: the latest queued row for it
// gives it. Whether the person already holds it is Zitadel's answer, not this.
func (ds Deliveries) Giving(projectID, roleKey string) bool {
	d, ok := ds.LatestQueued(projectID, roleKey)
	return ok && d.Gives(roleKey)
}

// SettledRoles is every role a settled row names, with its project.
func (ds Deliveries) SettledRoles() []RoleRef {
	var out []RoleRef
	for ref := range ds.settled.byRole {
		out = append(out, ref)
	}
	for _, d := range ds.settled.byProject {
		for _, rk := range d.RoleKeys {
			out = append(out, RoleRef{ProjectID: d.ProjectID, RoleKey: rk})
		}
	}
	return out
}

// QueuedRevokes is every role a queued revoke names.
func (ds Deliveries) QueuedRevokes() []RoleRef {
	var out []RoleRef
	for ref, d := range ds.queued.byRole {
		if d.OpType == "revoke" {
			out = append(out, ref)
		}
	}
	return out
}

// HasQueuedReplace says a replace on projectID is queued.
func (ds Deliveries) HasQueuedReplace(projectID string) bool {
	_, ok := ds.queued.byProject[projectID]
	return ok
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
// role and the latest replace per project, queued and settled apart.
//
// ponytail: two DISTINCT ON scans over the people's outbox rows, unindexed on
// user_id; tens of rows per person here. Index (target, user_id, intent_seq)
// if a person's history grows.
func latestDeliveries(ctx context.Context, q Querier, target string, userIDs, statuses []string) (map[string]Deliveries, error) {
	const sql = `
		WITH o AS (
			SELECT user_id, project_id, op_type, role_keys, status,
			       confirmed_at IS NOT NULL AS confirmed, completed_at, intent_seq,
			       status IN ('pending', 'in_flight') AS queued
			  FROM propagation_outbox
			 WHERE target = $1 AND user_id = ANY($2) AND status = ANY($3)
			   AND project_id IS NOT NULL AND role_keys IS NOT NULL
		)
		(SELECT DISTINCT ON (user_id, project_id, role_key, queued)
		        user_id, project_id, role_key, op_type, NULL::text[], status, confirmed, completed_at, intent_seq
		   FROM o, unnest(role_keys) AS role_key
		  WHERE op_type IN ('add', 'revoke')
		  ORDER BY user_id, project_id, role_key, queued, intent_seq DESC)
		UNION ALL
		(SELECT DISTINCT ON (user_id, project_id, queued)
		        user_id, project_id, '', op_type, role_keys, status, confirmed, completed_at, intent_seq
		   FROM o
		  WHERE op_type = 'replace'
		  ORDER BY user_id, project_id, queued, intent_seq DESC)`
	rows, err := q.Query(ctx, sql, target, userIDs, statuses)
	if err != nil {
		return nil, fmt.Errorf("read latest deliveries: %w", err)
	}
	defer rows.Close()
	byUser := map[string][]RoleDelivery{}
	for rows.Next() {
		var u string
		var d RoleDelivery
		if err := rows.Scan(&u, &d.ProjectID, &d.RoleKey, &d.OpType, &d.RoleKeys, &d.Status, &d.Confirmed, &d.CompletedAt, &d.IntentSeq); err != nil {
			return nil, fmt.Errorf("read latest deliveries: %w", err)
		}
		byUser[u] = append(byUser[u], d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make(map[string]Deliveries, len(byUser))
	for u, ds := range byUser {
		out[u] = NewDeliveries(ds...)
	}
	return out, nil
}

// NewDeliveries builds a person's Deliveries from rows already chosen as the
// latest of their class (one add/revoke per role, one replace per project,
// queued and settled apart).
func NewDeliveries(rows ...RoleDelivery) Deliveries {
	mk := func() latest {
		return latest{byRole: map[RoleRef]RoleDelivery{}, byProject: map[string]RoleDelivery{}}
	}
	ds := Deliveries{queued: mk(), settled: mk()}
	for _, d := range rows {
		l := ds.settled
		if d.Queued() {
			l = ds.queued
		}
		if d.OpType == "replace" {
			l.byProject[d.ProjectID] = d
		} else {
			l.byRole[RoleRef{ProjectID: d.ProjectID, RoleKey: d.RoleKey}] = d
		}
	}
	return ds
}
