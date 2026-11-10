package db

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"

	"syndra/internal/models"
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
//   - and no delivery of this triple is still DELIVERED — the latest
//     `applied` row for it does not give it (deliveredFor).
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
//
// The same holds the other way round (found 15 Oct): a role removed and given
// back before the removal was sent queued the add behind the revoke, two
// changes to confirm that net to nothing. An add cancels the queued revoke it
// answers, and is owed only when the grant is not still delivered. From any
// source: a cascade queues a revoke only when nothing else holds the role, so
// whatever gives it back answers that revoke.
func withdrawUndelivered(ctx context.Context, tx pgx.Tx, target string, p EnqueueParams) (owed bool, err error) {
	if !answersAny(p) {
		return true, nil
	}
	reason := withdrawnReason
	if p.OpType == "add" {
		reason = givenBackReason
	}
	// Read, decide, then cancel: under the access-mutation lock nothing moves
	// in between, and the rows refused are the rows that would have gone.
	candidates, err := scanQueuedChanges(tx.Query(ctx,
		queuedChangeColumns+candidateWhere+` ORDER BY created_at FOR UPDATE`, target, p.UserID, p.ProjectID, p.RoleKeys))
	if err != nil {
		return false, fmt.Errorf("withdraw queued delivery: %w", err)
	}
	answered := AnsweredBy(p, candidates)
	if len(answered) == 0 {
		return true, nil
	}
	if !AdmitCancellations(ctx, answered) {
		return false, ErrWouldCancel
	}
	cancelled := make([]string, 0, len(answered))
	for _, c := range answered {
		cancelled = append(cancelled, c.ID)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE propagation_outbox
		   SET status = 'superseded', completed_at = NOW(), last_error = $2
		 WHERE id = ANY($1::uuid[])`, cancelled, reason); err != nil {
		return false, fmt.Errorf("withdraw queued delivery: %w", err)
	}
	// Never invisibly (owner, 15 Oct): each cancellation is in Change history,
	// by whoever's action caused it, as well as said before they confirm.
	actor := p.GrantedBy
	if actor == "" {
		actor = "system"
	}
	for _, c := range answered {
		details, err := CancelledUnsentDetails(c, p)
		if err != nil {
			return false, err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO audit_logs (actor_zitadel_user_id, target_zitadel_user_id, action, resource_id, details)
			VALUES ($1, $2, $3, $4, $5)`, actor, p.UserID, cancelledUnsentAudit, c.ID, details); err != nil {
			return false, fmt.Errorf("audit a cancelled queued change: %w", err)
		}
	}

	// Is a delivery still standing? See deliveredFor.
	delivered, err := deliveredFor(ctx, tx, target, []string{p.UserID})
	if err != nil {
		return false, fmt.Errorf("withdraw queued delivery: %w", err)
	}
	return OwedAfterCancel(p, delivered[p.UserID][RoleRef{ProjectID: p.ProjectID, RoleKey: p.RoleKeys[0]}]), nil
}

// deliveredFor is, per person, the roles whose delivery still stands on
// target: the latest APPLIED row for the role gives it (RoleDelivery.Gives).
// An add after the last revoke means the grant is live there and a revoke is
// the only thing that takes it back; a revoke, or a replace that leaves the
// role out, after the last add means it is already gone, and a queued row just
// cancelled was a re-delivery that never went out, so there is nothing left
// to undo.
//
// Which row came last is latestDeliveries' answer, the same one a role's
// standing reads; existence is what the first version of this asked, and
// existence cannot tell an order. One statement for the write
// (withdrawUndelivered) and the plan (ReadAccessSnapshot).
func deliveredFor(ctx context.Context, q Querier, target string, userIDs []string) (map[string]map[RoleRef]bool, error) {
	latest, err := latestDeliveries(ctx, q, target, userIDs, []string{"applied"})
	if err != nil {
		return nil, err
	}
	out := map[string]map[RoleRef]bool{}
	for user, ds := range latest {
		for _, ref := range ds.Roles() {
			if d, ok := ds.Latest(ref.ProjectID, ref.RoleKey); ok && d.Gives(ref.RoleKey) {
				if out[user] == nil {
					out[user] = map[RoleRef]bool{}
				}
				out[user][ref] = true
			}
		}
	}
	return out, nil
}

// OwedAfterCancel says whether a write that cancelled a queued change is
// still owed: a revoke while the grant stands in Zitadel, an add only once it
// doesn't. delivered is the latest applied delivery of the role giving it.
// The planner says it ahead of time from the same fact.
func OwedAfterCancel(p EnqueueParams, delivered bool) bool {
	return delivered == (p.OpType == "revoke")
}

// cancelledUnsentAudit is the Change history action for each cancellation.
const cancelledUnsentAudit = "propagation.cancelled_unsent"

// CancelledUnsent is audit_logs.details for a cancelled_unsent entry: what was
// cancelled and what cancelled it, kept whole because the outbox row is
// pruned and the person's name, the project and the role are all the history
// has left. Change history reads this to say it (design A-04 §14.8).
type CancelledUnsent struct {
	Cancelled CancelledChange `json:"cancelled"`
	Cause     CancelCause     `json:"cause"`
}

// CancelledChange is the row that never went: Kind is "give" or "remove".
type CancelledChange struct {
	Kind        string    `json:"kind"`
	ProjectID   string    `json:"project_id"`
	RoleKey     string    `json:"role_key"`
	UserID      string    `json:"user_id"`
	InitiatedBy string    `json:"initiated_by,omitempty"`
	QueuedAt    time.Time `json:"queued_at"`
	Reason      string    `json:"reason,omitempty"`
}

// CancelCause is the act that cancelled it. Action is "grant", "remove" or
// "approve" (a request approved); Via is where that act came from: direct,
// bundle or rule. Who did it is the entry's actor, and "system" when nobody did.
type CancelCause struct {
	Action string `json:"action"`
	Via    string `json:"via"`
}

// CancelledUnsentDetails is the details JSON for one cancellation by p.
func CancelledUnsentDetails(c models.QueuedChange, p EnqueueParams) ([]byte, error) {
	kind, role := "give", ""
	if c.OpType == "revoke" {
		kind = "remove"
	}
	if len(c.RoleKeys) > 0 {
		role = c.RoleKeys[0]
	}
	via := p.Source
	if via == "" {
		via = "direct"
	}
	action := "grant"
	switch {
	case p.OpType == "revoke":
		action = "remove"
	case via == "direct" && p.SourceRef != "":
		action = "approve"
	}
	b, err := json.Marshal(CancelledUnsent{
		Cancelled: CancelledChange{Kind: kind, ProjectID: c.ProjectID, RoleKey: role, UserID: c.UserID,
			InitiatedBy: c.InitiatedBy, QueuedAt: c.QueuedAt.UTC(), Reason: Said(c.Reason)},
		Cause: CancelCause{Action: action, Via: via},
	})
	if err != nil {
		return nil, fmt.Errorf("audit a cancelled queued change: %w", err)
	}
	return b, nil
}

const givenBackReason = "cancelled before it was sent: the role was given back"

// Answers is the one statement of which queued change a write cancels.
// The planner (a plan's row, the preview) and withdrawUndelivered (the
// action) both ask it, so what an operator is told is
// what happens. c is a pending row on the write's target; the SQL only
// narrows to the person, project and role.
//
// Single-role writes only, adds and revokes only. An add answers a queued
// revoke from any source: a cascade queues a revoke only when nothing else
// holds the role, so whatever gives it back answers it. A revoke answers adds
// from its own source. For a direct grant the source is enough: there is one
// direct grant per person and role, and its add carries the request id or
// nothing while its removal carries the grant id, so matching the reference
// too meant removing an unsent direct grant queued the removal behind the add.
//
// A rule's revoke answers every queued rule give of the role, whichever rule
// row it was (task 1.6). A rule row is only ever a closure difference, so a
// rule revoke is queued only when no grant, bundle or rule reaches the role any
// more, and then no queued rule give of it is still wanted. Matching the rule
// id too would let two rules reaching one role, picked in a different order on
// the way in and out, queue the removal behind the give. A bundle's own rows
// still match on the bundle; a direct or bundle revoke never cancels a rule's
// row, and any give answers a rule's queued revoke, as for every source.
func Answers(p EnqueueParams, c models.QueuedChange) bool {
	if !answersAny(p) || c.UserID != p.UserID || c.ProjectID != p.ProjectID || !slices.Equal(c.RoleKeys, p.RoleKeys) {
		return false
	}
	if p.OpType == "add" {
		return c.OpType == "revoke"
	}
	if c.OpType != "add" && c.OpType != "replace" {
		return false
	}
	source := outboxSource(p)
	return c.Source == source && (source == "direct" || source == "rule" || c.SourceRef == p.SourceRef)
}

// AnsweredBy is the changes among queued that p cancels, in their order.
func AnsweredBy(p EnqueueParams, queued []models.QueuedChange) []models.QueuedChange {
	var out []models.QueuedChange
	for _, c := range queued {
		if Answers(p, c) {
			out = append(out, c)
		}
	}
	return out
}

// answersAny is false for a change that cancels nothing: more than one role,
// or not an add or a revoke.
func answersAny(p EnqueueParams) bool {
	return len(p.RoleKeys) == 1 && (p.OpType == "add" || p.OpType == "revoke")
}

// candidateWhere narrows the pending rows to the write's person, project and
// role; Answers decides.
const candidateWhere = `target = $1 AND status = 'pending'
		   AND user_id = $2 AND project_id = $3 AND role_keys = $4::text[]`

const queuedChangeColumns = `
		SELECT id, op_type, user_id, project_id, role_keys, source, COALESCE(source_ref, ''), COALESCE(initiated_by, ''), COALESCE(reason, ''), created_at
		  FROM propagation_outbox
		 WHERE `

func scanQueuedChanges(rows pgx.Rows, err error) ([]models.QueuedChange, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.QueuedChange
	for rows.Next() {
		var c models.QueuedChange
		if err := rows.Scan(&c.ID, &c.OpType, &c.UserID, &c.ProjectID, &c.RoleKeys, &c.Source, &c.SourceRef, &c.InitiatedBy, &c.Reason, &c.QueuedAt); err != nil {
			return nil, err
		}
		c.Said = Said(c.Reason)
		out = append(out, c)
	}
	return out, rows.Err()
}
