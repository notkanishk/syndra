package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// TargetZitadel is the built-in target. It is seeded by migration 000026 and is
// not an add-on: it has no registration, no manifest, and no base URL, so the
// add-on registry reconciliation below must never touch it.
const TargetZitadel = "zitadel"

// The two states the registry recognises. A target is disabled, never deleted:
// propagation and drift history keeps pointing at the row and must keep
// resolving.
const (
	TargetActive   = "active"
	TargetDisabled = "disabled"
)

// UpsertTarget records a target as registered and active.
//
// Every table carrying a target resolves it against this registry by foreign
// key, so a target that exists only in the backend's configuration is a target
// whose first propagation, snapshot, plan, or drift row the database refuses.
// Registration therefore has to reach here, not only into process memory.
//
// Re-activating on conflict is deliberate: the deployment naming a target is
// the statement that it is deployed, and a target that was disabled and is now
// configured again has been re-registered.
func UpsertTarget(ctx context.Context, target string) error {
	const q = `INSERT INTO targets (target, state) VALUES ($1, 'active')
		ON CONFLICT (target) DO UPDATE SET state = 'active'`
	if _, err := querier(ctx).Exec(ctx, q, target); err != nil {
		return fmt.Errorf("upsert target %s: %w", target, err)
	}
	return nil
}

// DisableUnconfiguredTargets deactivates every add-on target the deployment no
// longer configures, and returns the names it disabled so startup can say so.
//
// Disabling, never deleting. Propagation and drift history keeps pointing at
// these rows and must keep resolving — the foreign key would correctly refuse
// the delete, and that history is the record of what the target was asked to do
// while it was live. "The drain must not dispatch work for an unregistered
// target" is a state check, not something the key provides.
//
// Removing a target from the configuration is how an operator unregisters one,
// so this is the reconciliation that makes that mean something. Zitadel is
// excluded by name because it is the built-in target and appears in no add-on
// configuration; without that exclusion, a deployment with no add-ons would
// disable the one target the whole system runs on.
func DisableUnconfiguredTargets(ctx context.Context, configured []string) ([]DisabledTarget, error) {
	if configured == nil {
		configured = []string{}
	}

	var disabled []DisabledTarget
	err := InTx(ctx, func(tx pgx.Tx) error {
		const disable = `UPDATE targets SET state = 'disabled'
			WHERE state = 'active' AND target <> $1 AND target <> ALL($2::text[])
			RETURNING target`
		rows, err := tx.Query(ctx, disable, TargetZitadel, configured)
		if err != nil {
			return fmt.Errorf("disable unconfigured targets: %w", err)
		}
		var names []string
		for rows.Next() {
			var t string
			if err := rows.Scan(&t); err != nil {
				rows.Close()
				return fmt.Errorf("scan disabled target: %w", err)
			}
			names = append(names, t)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return fmt.Errorf("disable unconfigured targets: %w", err)
		}
		if len(names) == 0 {
			return nil
		}

		for _, name := range names {
			abandoned, err := abandonQueuedWorkTx(ctx, tx, name)
			if err != nil {
				return err
			}
			disabled = append(disabled, DisabledTarget{Target: name, Abandoned: abandoned})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return disabled, nil
}

// DisabledTarget is one deregistration and what it cost.
type DisabledTarget struct {
	Target string
	// Abandoned is the queued work that died with the registration. Reported
	// rather than counted silently: these are approved changes that will now
	// never reach anyone.
	Abandoned []AbandonedWork
}

// AbandonedWork is one queued row a deregistration terminated.
type AbandonedWork struct {
	OutboxID  string
	SubjectID string
	// Dispatched says the row had already been sent when the target was
	// deregistered, so whether it applied is unknowable. The distinction is
	// read from `started_at` rather than given its own status: the column
	// already records exactly this, and a second vocabulary for it would be a
	// second thing to keep true.
	Dispatched bool
}

// abandonQueuedWorkTx terminates the unresolved outbox rows of a target being
// deregistered, on the transaction that is deregistering it.
//
// Disabling a target does not merely order itself against a concurrent apply —
// it has to resolve the work that apply may have just committed. Serialising
// the two only decides who goes first; an apply that wins the race still leaves
// a row queued against a target nothing will ever drain, and a row that never
// drains counts as queued, which reads as "recorded" on every surface. So the
// deregistration takes responsibility for the rows it strands.
//
// The alternative — refusing to deregister while work is queued — was rejected:
// it makes a deployment change fail because of a queue, and a backend that died
// mid-drain would leave a row `in_flight` forever and a target that can never
// be removed.
//
// Terminated, never deleted. The row keeps its subject, its approval reference,
// and now a reason; the plan behind it is untouched. An operator asking "what
// happened to my change" gets an answer, and an audit row says so in the
// person's own timeline.
func abandonQueuedWorkTx(ctx context.Context, tx pgx.Tx, target string) ([]AbandonedWork, error) {
	// One statement, so the audit rows cannot be omitted. Written as a loop over
	// the returned rows, they could be — by an error path, by a future edit, by
	// a slice that happened to be empty. A data-modifying CTE runs to completion
	// whether or not the primary query reads it, so "terminated" and "recorded
	// as terminated" are the same write.
	const q = `
		WITH abandoned AS (
			UPDATE propagation_outbox
			   SET status = 'abandoned',
			       completed_at = NOW(),
			       last_error = CASE WHEN started_at IS NULL
			           THEN 'the target was deregistered before this row was dispatched'
			           ELSE 'the target was deregistered with this row in flight; whether it applied is unknown'
			       END
			 WHERE target = $1 AND status IN ('pending', 'in_flight')
			RETURNING id, user_id, started_at IS NOT NULL AS dispatched
		), audited AS (
			INSERT INTO audit_logs
				(actor_zitadel_user_id, target_zitadel_user_id, action, resource_id)
			SELECT 'system', a.user_id, $2, a.id FROM abandoned a
		)
		SELECT id, user_id, dispatched FROM abandoned ORDER BY user_id`

	rows, err := tx.Query(ctx, q, target, "entitlement."+target+".abandoned")
	if err != nil {
		return nil, fmt.Errorf("abandon queued work for %s: %w", target, err)
	}
	defer rows.Close()

	var work []AbandonedWork
	for rows.Next() {
		var w AbandonedWork
		if err := rows.Scan(&w.OutboxID, &w.SubjectID, &w.Dispatched); err != nil {
			return nil, fmt.Errorf("scan abandoned work: %w", err)
		}
		work = append(work, w)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("abandon queued work for %s: %w", target, err)
	}
	return work, nil
}

// ActiveTargets returns the targets the drain and sweep may dispatch work for.
func ActiveTargets(ctx context.Context) ([]string, error) {
	rows, err := querier(ctx).Query(ctx, `SELECT target FROM targets WHERE state = 'active' ORDER BY target`)
	if err != nil {
		return nil, fmt.Errorf("list active targets: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, fmt.Errorf("scan active target: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
