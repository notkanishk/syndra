package db

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// The memory that a write landed (change `reconciliation-as-merge`).
//
// Syndra's strongest evidence about a target is the moment the target ACCEPTED
// something: a named person's decision, written at a known time, acknowledged.
// It was being thrown away — the outbox has no `confirmed` state by design and
// its terminal rows are pruned, and the grant index is deleted by the event that
// removes a grant.
//
// The cost of losing it is one specific misreading. A grant applied and removed
// between two sweeps has no observation behind it, so a reconciliation reads its
// absence as "the write never landed" and replays it — silently restoring access
// somebody removed on purpose, which is the whole failure this change exists to
// end. With this, the absence reads as what it is.
//
// Distinct from a merge base, and the distinction is load-bearing: a base is
// what the target was seen HOLDING at a read, this is what it ACCEPTED at a
// write, and they disagree in exactly the case that matters.

// Propagation is one landed write.
type Propagation struct {
	Target    string `json:"target"`
	SubjectID string `json:"subject_id"`
	// Field is what was written, in the target's own vocabulary: `project/role`
	// for Zitadel, an entitlement field for an add-on.
	Field     string    `json:"field"`
	AppliedAt time.Time `json:"applied_at"`
	// OutboxID outlives the outbox row itself, so the thread back to what
	// authorised the write survives retention.
	OutboxID string `json:"outbox_id,omitempty"`
	Actor    string `json:"actor,omitempty"`
}

// RecordPropagation remembers that the target accepted this write.
//
// Last-write-wins per (target, subject, field): the question it answers is "when
// did this last land", and a history of every apply would grow without bound to
// answer a question nothing asks.
func RecordPropagation(ctx context.Context, p Propagation) error {
	switch {
	case strings.TrimSpace(p.Target) == "":
		return fmt.Errorf("record propagation: no target")
	case strings.TrimSpace(p.SubjectID) == "":
		return fmt.Errorf("record propagation: no subject")
	case strings.TrimSpace(p.Field) == "":
		return fmt.Errorf("record propagation: nothing was written")
	}
	const q = `
		INSERT INTO target_propagations (target, subject_id, field, applied_at, outbox_id, actor)
		VALUES ($1, $2, $3, NOW(), NULLIF($4, '')::uuid, $5)
		ON CONFLICT (target, subject_id, field)
		DO UPDATE SET applied_at = EXCLUDED.applied_at,
		              outbox_id  = EXCLUDED.outbox_id,
		              actor      = EXCLUDED.actor`
	if _, err := querier(ctx).Exec(ctx, q, p.Target, p.SubjectID, p.Field, p.OutboxID, p.Actor); err != nil {
		return fmt.Errorf("record propagation of %s for %s on %s: %w", p.Field, p.SubjectID, p.Target, err)
	}
	return nil
}

// PropagationsFor is everything Syndra has landed on one target, keyed by
// subject and then by field.
//
// One query for a whole pass or a whole queue: a per-row read would make the
// drift queue cost one round trip per finding.
func PropagationsFor(ctx context.Context, target string) (map[string]map[string]Propagation, error) {
	const q = `
		SELECT subject_id, field, applied_at, COALESCE(outbox_id::text, ''), actor
		FROM target_propagations WHERE target = $1`
	rows, err := querier(ctx).Query(ctx, q, target)
	if err != nil {
		return nil, fmt.Errorf("read propagations for %s: %w", target, err)
	}
	defer rows.Close()

	out := map[string]map[string]Propagation{}
	for rows.Next() {
		p := Propagation{Target: target}
		if err := rows.Scan(&p.SubjectID, &p.Field, &p.AppliedAt, &p.OutboxID, &p.Actor); err != nil {
			return nil, fmt.Errorf("scan propagation on %s: %w", target, err)
		}
		if out[p.SubjectID] == nil {
			out[p.SubjectID] = map[string]Propagation{}
		}
		out[p.SubjectID][p.Field] = p
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read propagations for %s: %w", target, err)
	}
	return out, nil
}

// ForgetPropagatedFields drops the memory for specific things that were removed.
//
// A landed write is remembered so that a later absence reads as a removal rather
// than as a write that never happened. The moment SYNDRA removes the same thing,
// that memory becomes a lie about the future: re-grant the role later, have the
// new write fail to land, and the stale memory says the target was holding it —
// so the reconciliation reports a hand removal and suppresses the replay that
// would have restored the access.
//
// So a revoke that lands forgets exactly what it removed.
func ForgetPropagatedFields(ctx context.Context, target, subjectID string, fields []string) error {
	if len(fields) == 0 {
		return nil
	}
	const q = `
		DELETE FROM target_propagations
		WHERE target = $1 AND subject_id = $2 AND field = ANY($3)`
	if _, err := querier(ctx).Exec(ctx, q, target, subjectID, fields); err != nil {
		return fmt.Errorf("forget propagated fields for %s on %s: %w", subjectID, target, err)
	}
	return nil
}

// ForgetPropagatedFieldsExcept drops everything remembered under a prefix that
// is not in the set just written.
//
// The replace case. A replace states the WHOLE desired set for one grant, so
// anything remembered under that prefix and absent from it has just been
// removed — and leaving it remembered would make a later failed re-grant look
// like somebody's hand removal, exactly as a stale revoke would.
//
// Prefix rather than an explicit list, because the caller knows what it wrote
// and not what was remembered before it.
func ForgetPropagatedFieldsExcept(ctx context.Context, target, subjectID, prefix string, keep []string) error {
	if strings.TrimSpace(prefix) == "" {
		return fmt.Errorf("forget propagated fields: no prefix")
	}
	const q = `
		DELETE FROM target_propagations
		WHERE target = $1 AND subject_id = $2
		  AND field LIKE $3 || '/%'
		  AND NOT (field = ANY($4))`
	if _, err := querier(ctx).Exec(ctx, q, target, subjectID, prefix, keep); err != nil {
		return fmt.Errorf("prune propagated fields for %s on %s: %w", subjectID, target, err)
	}
	return nil
}

// ForgetPropagations drops everything remembered for a subject on a target.
//
// Goes with a binding being released or an account purged, like the merge base:
// a memory of writes to an account nobody manages any more is compared, on the
// next pass, against whatever that subject is bound to next.
func ForgetPropagations(ctx context.Context, target, subjectID string) error {
	const q = `DELETE FROM target_propagations WHERE target = $1 AND subject_id = $2`
	if _, err := querier(ctx).Exec(ctx, q, target, subjectID); err != nil {
		return fmt.Errorf("forget propagations for %s on %s: %w", subjectID, target, err)
	}
	return nil
}
