package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

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

// FieldRoleMap is a target's persisted role map, field name → role, as the
// backend last accepted it from the add-on's manifest (targets.field_roles,
// migration 000088, design §3), and the stored fields that manifest no longer
// declares (targets.retired_fields, 000090): kept so a name never returns
// meaning something else, never sent, since the target no longer accepts them.
// Shape is what that manifest says the target is (targets.accepted_shape,
// 000095): the addons package's JSON, kept opaque here; nil before any
// acceptance that saved it.
type FieldRoleMap struct {
	Roles   map[string]string
	Retired []string
	Shape   []byte
}

// TargetFieldRoles reads a target's role map. found is false before any
// contract-2 manifest was accepted.
func TargetFieldRoles(ctx context.Context, target string) (m FieldRoleMap, found bool, err error) {
	var raw []byte
	err = querier(ctx).QueryRow(ctx, `SELECT field_roles, retired_fields, accepted_shape FROM targets WHERE target = $1`, target).Scan(&raw, &m.Retired, &m.Shape)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && raw == nil) {
		return FieldRoleMap{}, false, nil
	}
	if err != nil {
		return FieldRoleMap{}, false, fmt.Errorf("read field roles of %s: %w", target, err)
	}
	if err := json.Unmarshal(raw, &m.Roles); err != nil {
		return FieldRoleMap{}, false, fmt.Errorf("field roles of %s are unreadable: %w", target, err)
	}
	return m, true, nil
}

// SaveTargetFieldRoles replaces a target's role map, retired fields and shape, and,
// in the same transaction, supersedes every queued row whose desired state
// names a field this save newly retires. The caller has already refused a map
// that changes an existing field's role.
//
// Why the rows go: a desired state is frozen when it is queued and sent as
// it was planned, and the add-on refuses a field outside its schema, so a row
// naming a retired field would be refused by the target, and a withdraw-only
// one would then hold the person's next withdrawal (runnerSends'
// target_refused rule). Superseding keeps "what is planned is what is sent":
// nothing is rewritten, the row is closed with its reason and audit row, and
// the next drift sweep queues the person again from the current roles. Only
// `pending` rows: an `in_flight` row has been claimed and may be on the wire,
// and is left to settle as it will. A superseded row never counts toward the
// target_refused hold (that reads `failed` rows only).
func SaveTargetFieldRoles(ctx context.Context, target string, m FieldRoleMap) error {
	raw, err := json.Marshal(m.Roles)
	if err != nil {
		return err
	}
	retired := m.Retired
	if retired == nil {
		retired = []string{}
	}
	tx, owned, err := beginOrJoin(ctx)
	if err != nil {
		return err
	}
	if owned {
		defer tx.Rollback(ctx)
	}
	var was []string
	err = tx.QueryRow(ctx, `SELECT retired_fields FROM targets WHERE target = $1 FOR UPDATE`, target).Scan(&was)
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("save field roles of %s: no such target", target)
	}
	if err != nil {
		return fmt.Errorf("save field roles of %s: %w", target, err)
	}
	if _, err := tx.Exec(ctx, `UPDATE targets SET field_roles = $2::jsonb, retired_fields = $3, accepted_shape = $4::jsonb WHERE target = $1`, target, raw, retired, m.Shape); err != nil {
		return fmt.Errorf("save field roles of %s: %w", target, err)
	}
	for _, field := range retired {
		if slices.Contains(was, field) {
			continue
		}
		if err := supersedeNamingField(ctx, tx, target, field); err != nil {
			return err
		}
	}
	if owned {
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("save field roles of %s: commit: %w", target, err)
		}
	}
	return nil
}

// supersedeNamingField closes the target's pending rows whose approved desired
// state names a retired field, with the reason an operator reads on the row
// and an audit row per row, in one statement (as terminateSuperseded does).
func supersedeNamingField(ctx context.Context, tx pgx.Tx, target, field string) error {
	const q = `
		WITH discarded AS (
			UPDATE propagation_outbox p
			   SET status = 'superseded', completed_at = NOW(), last_error = $3
			  FROM plan_subjects ps
			  JOIN desired_state_snapshots s ON s.id = ps.snapshot_id
			 WHERE p.target = $1
			   AND p.status = 'pending'
			   AND ps.id = p.plan_subject_id
			   AND jsonb_exists(s.state_json, $2)
			RETURNING p.id, p.user_id
		)
		INSERT INTO audit_logs (actor_zitadel_user_id, target_zitadel_user_id, action, resource_id)
		SELECT 'system', d.user_id, $4, d.id FROM discarded d`
	reason := "the add-on no longer declares " + field + "; the next check queues this again without it"
	if _, err := tx.Exec(ctx, q, target, field, reason, "entitlement."+target+".superseded"); err != nil {
		return fmt.Errorf("supersede rows naming %s on %s: %w", field, target, err)
	}
	return nil
}
