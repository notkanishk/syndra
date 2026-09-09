package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// A revoke exists to undo a delivery. If the delivery never left, there is
// nothing to undo — and queueing its opposite is not a cancellation, it is a
// second instruction that has to be carried out in order to cancel the first.
//
// Assigning a bundle and removing it again before sending produced exactly
// that: three `add` rows and three `revoke` rows for the same three roles,
// which an operator then had to confirm in order to arrive back where they
// started. Six calls to Zitadel that net to nothing, three entries under
// Unfinished revocations describing access that was never delivered, and a
// Pending changes screen that asks somebody to approve both halves of a
// no-op.
//
// So: before a revoke is queued, look for the undelivered delivery it is
// answering and cancel that instead. `superseded` is the status this is for —
// it already means "this row was overtaken and will not be dispatched" — and
// the row stays in the table with its reason, because the record of what was
// asked for is not the same thing as the queue of what is still owed.
const withdrawnReason = "withdrawn before it was sent: the change that queued it was undone"

// withdrawUndelivered cancels a queued delivery of exactly this grant and
// reports whether a revoke is still owed.
//
// Both halves are deliberately narrow, because the two mistakes here are not
// equally expensive. Skipping a revoke that WAS needed leaves access in place
// that somebody asked to remove; queueing one that was not needed costs a
// no-op call. So the revoke is dropped only where nothing could have heard
// about the grant:
//
//   - a `pending` row for this exact triple was cancelled just now — not
//     `in_flight`, which may already be mid-dispatch and whose effect
//     therefore still has to be undone;
//   - it came from the SAME source that is now being taken away, so a grant
//     delivered by some other route is never spared by a bundle's removal;
//   - and no delivery of this triple is still STANDING — the last `applied`
//     row for it is not an add.
//
// Any of those failing leaves today's behaviour untouched.
//
// That last condition was first written as "no add for this triple has ever
// reached `applied`", which is a different question and the wrong one. A role
// granted in the morning and revoked at lunch has an applied add in its
// history and nothing in force, so the rule kept queueing a revocation for a
// grant that had already been taken back — and the operator who reported the
// original defect watched the same six rows appear again with only the wording
// changed. What matters is which of the two came last.
//
// Single-role rows only. `deltaParams` writes one role per row, so every
// cascade revoke qualifies; a multi-role row belongs to a direct grant that
// covers more than what is being taken away, and cancelling the whole of it
// would withdraw deliveries nobody countermanded.
func withdrawUndelivered(ctx context.Context, tx pgx.Tx, target string, p EnqueueParams) (owed bool, err error) {
	if p.OpType != "revoke" || len(p.RoleKeys) != 1 {
		return true, nil
	}
	source := p.Source
	if source == "" {
		source = "direct"
	}

	const cancel = `
		UPDATE propagation_outbox
		   SET status = 'superseded', completed_at = NOW(), last_error = $1
		 WHERE target = $2 AND status = 'pending'
		   AND op_type IN ('add','replace')
		   AND user_id = $3 AND project_id = $4 AND role_keys = $5::text[]
		   AND source = $6 AND source_ref IS NOT DISTINCT FROM NULLIF($7,'')
		RETURNING id`
	rows, err := tx.Query(ctx, cancel, withdrawnReason, target,
		p.UserID, p.ProjectID, p.RoleKeys, source, p.SourceRef)
	if err != nil {
		return false, fmt.Errorf("withdraw queued delivery: %w", err)
	}
	cancelled := 0
	for rows.Next() {
		cancelled++
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("withdraw queued delivery: %w", err)
	}
	if cancelled == 0 {
		return true, nil
	}

	// Is a delivery still standing? Only the LAST applied row decides: an add
	// after the last revoke means the grant is live in Zitadel and this revoke
	// is the only thing that takes it back. A revoke after the last add means
	// it is already gone, and the row just cancelled was a re-delivery that
	// never went out — so there is nothing left to undo.
	//
	// `MAX ... FILTER`, not `EXISTS`: existence is what the first version of
	// this asked, and existence cannot tell an order.
	const standing = `
		SELECT COALESCE(MAX(intent_seq) FILTER (WHERE op_type IN ('add','replace')), 0)
		     > COALESCE(MAX(intent_seq) FILTER (WHERE op_type = 'revoke'), 0)
		  FROM propagation_outbox
		 WHERE target = $1 AND user_id = $2 AND project_id = $3
		   AND role_keys @> $4::text[] AND status = 'applied'`
	var live bool
	if err := tx.QueryRow(ctx, standing, target, p.UserID, p.ProjectID, p.RoleKeys).
		Scan(&live); err != nil {
		return false, fmt.Errorf("withdraw queued delivery: %w", err)
	}
	return live, nil
}
