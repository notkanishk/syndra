package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// Why a target could not be reconciled. A closed vocabulary, checked at compile
// time by a switch rather than held in a slice any package could widen: this
// value is written to a row an operator reads, and free text on that path is a
// channel — whatever a caller happens to hold ends up rendered on a governance
// surface.
const (
	// UnreconciledUnreachable — the target did not answer at all.
	UnreconciledUnreachable = "target_unreachable"
	// UnreconciledStaleRead — the target answered from its last-known snapshot.
	// The read is a mirror of some earlier moment, and diffing it would report
	// every change made since as out-of-band (design §9).
	UnreconciledStaleRead = "read_stale"
	// UnreconciledTruncated — the read hit its safety cap. What was seen is
	// real; what was not seen is unknown, so absence cannot be concluded.
	UnreconciledTruncated = "read_truncated"
	// UnreconciledReadRefused — the target answered and did not serve the read:
	// rejected credentials, insufficient permission, or a failure on its own
	// side. Distinct from unreachable because the operator's next move is
	// different in kind — there is nothing to reach for, there is something to
	// fix, and an outage that is really an expired service-account key would
	// otherwise be waited out rather than repaired.
	UnreconciledReadRefused = "read_refused"
)

func validUnreconciledReason(r string) bool {
	switch r {
	case UnreconciledUnreachable, UnreconciledStaleRead, UnreconciledTruncated, UnreconciledReadRefused:
		return true
	default:
		return false
	}
}

// ErrUnreconciledReason refuses a reason outside the vocabulary. It names the
// vocabulary and never the value, so a caller that reached here holding
// something it should not cannot write it into the error either.
var ErrUnreconciledReason = errors.New(
	"unreconciled reason must be one of: target_unreachable, read_stale, read_truncated, read_refused")

// TargetReconciliation is how current Syndra's picture of a target is.
//
// LastCurrentReadAt is nil when the target has never been read for itself. That
// is a third state, not a very old timestamp: a target registered this morning
// and never swept, one swept a month ago, and one swept a minute ago are three
// different things to an operator, and collapsing the first two into "old"
// invents a history.
type TargetReconciliation struct {
	Target             string     `json:"target"`
	LastCurrentReadAt  *time.Time `json:"last_current_read_at"`
	UnreconciledSince  *time.Time `json:"unreconciled_since,omitempty"`
	UnreconciledReason string     `json:"unreconciled_reason,omitempty"`
}

// Unreconciled reports whether the target is currently in a period of not being
// reconciled.
func (t TargetReconciliation) Unreconciled() bool { return t.UnreconciledSince != nil }

// MarkTargetUnreconciled records that a sweep could not reconcile the target,
// and returns the row as it now stands — including the age the operator is
// owed, the last time Syndra saw the target for itself.
//
// `unreconciled_since` is preserved across repeated calls by COALESCE, in the
// statement rather than by a read-then-write in the caller. An outage produces
// one sweep per tick, and restamping the start on each of them would hold the
// outage permanently one tick old: the number an operator uses to decide
// whether this is a blip or a week would never grow.
func MarkTargetUnreconciled(ctx context.Context, target, reason string) (TargetReconciliation, error) {
	if target == "" {
		return TargetReconciliation{}, ErrNoSuchTarget
	}
	if !validUnreconciledReason(reason) {
		return TargetReconciliation{}, ErrUnreconciledReason
	}
	const q = `
		INSERT INTO target_reconciliation (target, unreconciled_since, unreconciled_reason)
		VALUES ($1, NOW(), $2)
		ON CONFLICT (target) DO UPDATE SET
			unreconciled_since  = COALESCE(target_reconciliation.unreconciled_since, NOW()),
			-- The reason may change without the period restarting: a target
			-- that goes from unreachable to answering staleley has not come
			-- back, and the clock on how long Syndra has been blind to it runs
			-- from the first of those, not the latest.
			unreconciled_reason = EXCLUDED.unreconciled_reason
		RETURNING target, last_current_read_at, unreconciled_since, COALESCE(unreconciled_reason,'')`
	return scanReconciliation(PG.QueryRow(ctx, q, target, reason), "mark target unreconciled")
}

// MarkTargetReconciled records a completed reconciliation over a current read,
// and ends any unreconciled period in the same statement.
//
// The two halves are one write on purpose. A target that has been read but is
// still flagged unreconciled, or one whose flag is cleared without a read
// behind it, are both rows that say something untrue about the same moment —
// and the second is the dangerous one, because it reports confidence Syndra
// does not have.
func MarkTargetReconciled(ctx context.Context, target string) (TargetReconciliation, error) {
	if target == "" {
		return TargetReconciliation{}, ErrNoSuchTarget
	}
	const q = `
		INSERT INTO target_reconciliation (target, last_current_read_at)
		VALUES ($1, NOW())
		ON CONFLICT (target) DO UPDATE SET
			last_current_read_at = NOW(),
			unreconciled_since   = NULL,
			unreconciled_reason  = NULL
		RETURNING target, last_current_read_at, unreconciled_since, COALESCE(unreconciled_reason,'')`
	return scanReconciliation(PG.QueryRow(ctx, q, target), "mark target reconciled")
}

// GetUnreconciledTargets lists the targets Syndra currently cannot vouch for,
// oldest outage first — the one it has been blind to longest is the one worth
// looking at.
func GetUnreconciledTargets(ctx context.Context) ([]TargetReconciliation, error) {
	const q = `
		SELECT target, last_current_read_at, unreconciled_since, COALESCE(unreconciled_reason,'')
		FROM target_reconciliation
		WHERE unreconciled_since IS NOT NULL
		ORDER BY unreconciled_since`
	rows, err := PG.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("get unreconciled targets: %w", err)
	}
	defer rows.Close()
	var out []TargetReconciliation
	for rows.Next() {
		var t TargetReconciliation
		if err := rows.Scan(&t.Target, &t.LastCurrentReadAt, &t.UnreconciledSince, &t.UnreconciledReason); err != nil {
			return nil, fmt.Errorf("scan unreconciled target: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func scanReconciliation(row pgx.Row, what string) (TargetReconciliation, error) {
	var t TargetReconciliation
	if err := row.Scan(&t.Target, &t.LastCurrentReadAt, &t.UnreconciledSince, &t.UnreconciledReason); err != nil {
		return TargetReconciliation{}, fmt.Errorf("%s: %w", what, err)
	}
	return t, nil
}
