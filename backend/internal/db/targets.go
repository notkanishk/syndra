package db

import (
	"context"
	"fmt"
)

// TargetZitadel is the built-in target. It is seeded by migration 000026 and is
// not an add-on: it has no registration, no manifest, and no base URL, so the
// add-on registry reconciliation below must never touch it.
const TargetZitadel = "zitadel"

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
	if _, err := PG.Exec(ctx, q, target); err != nil {
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
func DisableUnconfiguredTargets(ctx context.Context, configured []string) ([]string, error) {
	if configured == nil {
		configured = []string{}
	}
	const q = `UPDATE targets SET state = 'disabled'
		WHERE state = 'active' AND target <> $1 AND target <> ALL($2::text[])
		RETURNING target`
	rows, err := PG.Query(ctx, q, TargetZitadel, configured)
	if err != nil {
		return nil, fmt.Errorf("disable unconfigured targets: %w", err)
	}
	defer rows.Close()
	var disabled []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, fmt.Errorf("scan disabled target: %w", err)
		}
		disabled = append(disabled, t)
	}
	return disabled, rows.Err()
}

// ActiveTargets returns the targets the drain and sweep may dispatch work for.
func ActiveTargets(ctx context.Context) ([]string, error) {
	rows, err := PG.Query(ctx, `SELECT target FROM targets WHERE state = 'active' ORDER BY target`)
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
