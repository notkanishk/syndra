package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"syndra/internal/models"
)

// AccessSnapshot is what the access-change planner reads (architecture
// deepening, candidate 1, decisions 2 and 3): every selected person's
// holdings and queued changes, the rules and the roles that wait, read in one
// pass under the access-mutation lock. One snapshot per plan, not per row, so
// every row of a bulk plan is judged against the same world, and the apply
// re-reads the same shape to check the plan still holds.
type AccessSnapshot struct {
	People    map[string]PersonAccess
	Rules     []models.MappingRule
	RoleWaits map[RoleRef]bool
}

// PersonAccess is one person's part of the snapshot.
type PersonAccess struct {
	// Direct is their grants that have not ended, with their ends.
	Direct []models.DirectGrant
	// Bundles is every bundle they are assigned, whether or not its version
	// gives them anything.
	Bundles map[string]bool
	// ByBundle is what each bundle they hold gives them, through the version
	// they are pinned to.
	ByBundle map[string][]models.BundleRole
	// Revoking is the roles a queued revocation is taking away
	// (QueuedRevocations): already decided, so not counted as held.
	Revoking []RoleRef
	// Queued is their changes waiting to be sent to Zitadel, oldest first:
	// what a write may cancel (Answers).
	Queued []models.QueuedChange
	// Standing is the roles whose last delivery to Zitadel was an add: whether
	// a write that cancels a queued change is still owed (OwedAfterCancel).
	Standing map[RoleRef]bool
}

// errSnapshotUnlocked: the snapshot was asked for outside a transaction, so
// nothing held the world still while it was read.
var errSnapshotUnlocked = errors.New("db: the access snapshot is read under the access-mutation lock (InTxLockingAccess)")

// ReadAccessSnapshot reads the snapshot for userIDs. It must run inside
// InTxLockingAccess: the plan, and the apply that checks it, act on what this
// read, and only the lock keeps it true until they do.
func ReadAccessSnapshot(ctx context.Context, userIDs []string) (AccessSnapshot, error) {
	tx, ok := ctx.Value(txKey).(pgx.Tx)
	if !ok || tx == nil {
		return AccessSnapshot{}, errSnapshotUnlocked
	}
	direct, err := directGrantsFor(ctx, userIDs, false)
	if err != nil {
		return AccessSnapshot{}, fmt.Errorf("snapshot direct grants: %w", err)
	}
	byBundle, err := bundleRolesFor(ctx, userIDs)
	if err != nil {
		return AccessSnapshot{}, fmt.Errorf("snapshot bundle roles: %w", err)
	}
	assigned := map[string]map[string]bool{}
	rows, err := tx.Query(ctx, `SELECT user_id, bundle_id::text FROM user_bundle_assignments WHERE user_id = ANY($1)`, userIDs)
	if err != nil {
		return AccessSnapshot{}, fmt.Errorf("snapshot bundle assignments: %w", err)
	}
	for rows.Next() {
		var user, bundle string
		if err := rows.Scan(&user, &bundle); err != nil {
			rows.Close()
			return AccessSnapshot{}, fmt.Errorf("snapshot bundle assignments: %w", err)
		}
		if assigned[user] == nil {
			assigned[user] = map[string]bool{}
		}
		assigned[user][bundle] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return AccessSnapshot{}, fmt.Errorf("snapshot bundle assignments: %w", err)
	}
	revoking, err := queuedRevocationsFor(ctx, userIDs)
	if err != nil {
		return AccessSnapshot{}, fmt.Errorf("snapshot queued revocations: %w", err)
	}
	queued, err := scanQueuedChanges(tx.Query(ctx,
		queuedChangeColumns+`target = 'zitadel' AND status = 'pending' AND user_id = ANY($1) ORDER BY created_at, id`, userIDs))
	if err != nil {
		return AccessSnapshot{}, fmt.Errorf("snapshot queued changes: %w", err)
	}
	standing, err := standingFor(ctx, tx, "zitadel", userIDs)
	if err != nil {
		return AccessSnapshot{}, fmt.Errorf("snapshot standing deliveries: %w", err)
	}
	rules, err := GetActiveMappingRules(ctx)
	if err != nil {
		return AccessSnapshot{}, fmt.Errorf("snapshot rules: %w", err)
	}
	waits, err := ListRoleWaits(ctx)
	if err != nil {
		return AccessSnapshot{}, fmt.Errorf("snapshot roles that wait: %w", err)
	}

	snap := AccessSnapshot{People: make(map[string]PersonAccess, len(userIDs)), Rules: rules, RoleWaits: make(map[RoleRef]bool, len(waits))}
	for _, w := range waits {
		snap.RoleWaits[RoleRef{ProjectID: w.ProjectID, RoleKey: w.RoleKey}] = true
	}
	for _, id := range userIDs {
		snap.People[id] = PersonAccess{Direct: direct[id], Bundles: assigned[id], ByBundle: byBundle[id],
			Revoking: revoking[id], Standing: standing[id]}
	}
	for _, c := range queued {
		p := snap.People[c.UserID]
		p.Queued = append(p.Queued, c)
		snap.People[c.UserID] = p
	}
	return snap, nil
}
