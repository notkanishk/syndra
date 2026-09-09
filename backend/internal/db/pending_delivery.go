package db

import "context"

// PendingDelivery is one grant this person has been given that has not yet been
// sent anywhere.
//
// The person page used to build somebody's access from the assignment tables
// alone, so a bundle assigned in manual mode read as held from the moment it
// was assigned — identical on screen to one delivered a month ago. An operator
// took that page as confirmation the work was done, which is the one thing it
// could not tell them.
//
// The outbox is the honest answer and needs no new bookkeeping to give it: a
// row is in it exactly while Syndra still owes the change to a target. Nothing
// here asks whether Zitadel agrees — that is drift's question, answered on a
// six-hour sweep, and far too old to date a grant made a minute ago.
type PendingDelivery struct {
	ProjectID string
	RoleKey   string
	Source    string
	SourceRef string
}

// PendingDeliveriesForUser lists the still-unsent grants for one person.
//
// `in_flight` counts as unsent: it has left the queue and not yet been
// confirmed, and a surface that called it delivered would be making the same
// claim one state earlier.
//
// ponytail: one small query per person-page load, unindexed on user_id — the
// outbox is bounded by what is undrained, which is tens of rows at this
// deployment's scale. Add an index if a queue is ever allowed to grow.
func PendingDeliveriesForUser(ctx context.Context, userID string) ([]PendingDelivery, error) {
	const q = `
		SELECT project_id, unnest(role_keys) AS role_key, source, COALESCE(source_ref, '')
		  FROM propagation_outbox
		 WHERE user_id = $1
		   AND op_type IN ('add','replace')
		   AND status IN ('pending','in_flight')`
	rows, err := querier(ctx).Query(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []PendingDelivery
	for rows.Next() {
		var d PendingDelivery
		var project *string
		if err := rows.Scan(&project, &d.RoleKey, &d.Source, &d.SourceRef); err != nil {
			return nil, err
		}
		if project != nil {
			d.ProjectID = *project
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
