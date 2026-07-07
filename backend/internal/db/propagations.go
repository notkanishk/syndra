package db

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"mkauth/internal/models"
)

// drainAdvisoryLockKey is the stable, arbitrary key for the session-level
// advisory lock that serializes propagation drains across the deployment.
const drainAdvisoryLockKey int64 = 771234501

// TryAcquireDrainLock takes the session-level advisory lock that serializes
// drains. It returns a release closure (unlock + return the connection to the
// pool), acquired=false if another drain already holds it, or an error on a
// connection/query failure. Serializing drains is what makes reclaiming
// in_flight rows (ClaimPendingPropagations) safe: because a live drain holds
// this lock for its whole run, the only in_flight rows a claiming drain ever
// sees are those orphaned by a crashed drain whose session (and lock) is gone.
func TryAcquireDrainLock(ctx context.Context) (func(), bool, error) {
	conn, err := PG.Acquire(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("acquire drain-lock conn: %w", err)
	}
	var got bool
	if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock($1)`, drainAdvisoryLockKey).Scan(&got); err != nil {
		conn.Release()
		return nil, false, fmt.Errorf("pg_try_advisory_lock: %w", err)
	}
	if !got {
		conn.Release() // lock held elsewhere — the advisory lock itself was never taken
		return nil, false, nil
	}
	release := func() {
		// Unlock on the SAME connection that holds it, then return it to the pool.
		// Use a background context so cleanup runs even if the drain's ctx is done.
		_, _ = conn.Exec(context.Background(), `SELECT pg_advisory_unlock($1)`, drainAdvisoryLockKey)
		conn.Release()
	}
	return release, true, nil
}

// GetPropagationStatus returns the current status of one outbox row, or "" if it
// no longer exists (e.g. pruned). The ?apply=true inline drain uses it to report
// THIS request's row outcome rather than the batch drain's aggregate.
func GetPropagationStatus(ctx context.Context, id string) (string, error) {
	const q = `SELECT status FROM pending_zitadel_propagations WHERE id=$1`
	var st string
	if err := PG.QueryRow(ctx, q, id).Scan(&st); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("get propagation status: %w", err)
	}
	return st, nil
}

// -------------------------------------------------------------
// ZITADEL PROPAGATION OUTBOX
// -------------------------------------------------------------
//
// pending_zitadel_propagations buffers every MkAuth-mediated Zitadel grant
// mutation so the operator drains them explicitly (services/propagation).
// It mirrors the provisioning_intents claim-and-process pattern: rows move
// pending -> in_flight -> applied|failed. `applied` is terminal success; there
// is NO `confirmed` state (design Decision 1).

// newOutboxIdempotencyKey returns a random RFC-4122 v4 UUID string. The repo
// carries no uuid dependency, so we mint one from crypto/rand rather than
// pulling in a new module — the outbox column is `UUID NOT NULL UNIQUE`, so the
// format must be canonical 8-4-4-4-12 hex.
func newOutboxIdempotencyKey() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate idempotency key: %w", err)
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

// NewOutboxIdempotencyKey is the exported entrypoint to the crypto/rand v4 minter
// for cross-package callers (services/drift). The repo has no uuid module.
func NewOutboxIdempotencyKey() (string, error) { return newOutboxIdempotencyKey() }

// InsertPendingPropagation inserts one outbox row and returns its id. Used by
// the drift re-enqueue path (sub-phase 2); the transactional enqueue uses its
// own tx-scoped insert (propagation_enqueue.go). idempotencyKey must be a fresh
// canonical UUID string.
func InsertPendingPropagation(ctx context.Context, opType, userID, projectID string,
	roleKeys []string, zitadelGrantID, payloadJSON, idempotencyKey, initiatedBy string) (string, error) {
	const q = `
		INSERT INTO pending_zitadel_propagations
			(op_type, user_id, project_id, role_keys, zitadel_grant_id, payload_json, idempotency_key, initiated_by)
		VALUES ($1,$2,$3,$4,NULLIF($5,''),$6,$7,$8)
		RETURNING id`
	var id string
	if err := PG.QueryRow(ctx, q, opType, userID, projectID, roleKeys, zitadelGrantID,
		payloadJSON, idempotencyKey, initiatedBy).Scan(&id); err != nil {
		return "", fmt.Errorf("insert propagation: %w", err)
	}
	return id, nil
}

// PendingOutboxAddExists reports whether an undrained add is already queued for
// the (user, project, role) triple, so the drift sweep's mkauth_only replay does
// not pile a fresh duplicate every tick for a grant that stays missing in Zitadel.
func PendingOutboxAddExists(ctx context.Context, userID, projectID, roleKey string) (bool, error) {
	const q = `SELECT EXISTS(
		SELECT 1 FROM pending_zitadel_propagations
		WHERE op_type='add' AND user_id=$1 AND project_id=$2
		  AND $3 = ANY(role_keys) AND status IN ('pending','in_flight'))`
	var exists bool
	if err := PG.QueryRow(ctx, q, userID, projectID, roleKey).Scan(&exists); err != nil {
		return false, fmt.Errorf("pending outbox add exists (%s/%s/%s): %w", userID, projectID, roleKey, err)
	}
	return exists, nil
}

// ClaimPendingPropagations atomically transitions up to `limit` claimable rows
// to in_flight and returns them in created_at order. FOR UPDATE SKIP LOCKED makes
// concurrent drains safe (mirrors ClaimPendingIntents).
//
// It claims BOTH 'pending' AND 'in_flight' rows (design.md §Drain: "status in
// (pending,in_flight)→in_flight"). Claiming in_flight is the crash-recovery path:
// a drain that dies after claiming but before recording a terminal state leaves
// orphaned in_flight rows that the worklist/count still surface (GetPending /
// CountPending use the same status set); reclaiming them lets the next drain
// re-drive each one, where the idempotent already-exists check (409→applied)
// resolves any operation that actually reached Zitadel. `started_at` is reset so
// the row's clock reflects the reclaim.
func ClaimPendingPropagations(ctx context.Context, limit int) ([]models.PendingPropagation, error) {
	if limit <= 0 {
		limit = 100
	}
	const q = `
		WITH claimed AS (
			SELECT id FROM pending_zitadel_propagations
			WHERE status IN ('pending','in_flight')
			ORDER BY created_at
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		UPDATE pending_zitadel_propagations p
		SET status = 'in_flight', started_at = NOW()
		FROM claimed
		WHERE p.id = claimed.id
		RETURNING p.id, p.op_type, p.user_id, p.project_id, p.role_keys,
		          p.source, COALESCE(p.source_ref,''),
		          COALESCE(p.zitadel_grant_id,''), p.status, p.attempts,
		          COALESCE(p.last_error,''), p.initiated_by, p.created_at, p.started_at, p.completed_at`
	rows, err := PG.Query(ctx, q, limit)
	if err != nil {
		return nil, fmt.Errorf("claim propagations: %w", err)
	}
	defer rows.Close()
	return scanPropagations(rows)
}

// ClaimPropagationByID atomically transitions ONE row (pending or in_flight) to
// in_flight and returns it — the targeted inline-apply claim behind DrainOne.
// found=false when the row no longer exists or is already terminal
// (applied/failed), which the caller treats as a no-op. It mirrors
// ClaimPendingPropagations' claimable status set and started_at reset but is
// scoped to a single id; no FOR UPDATE SKIP LOCKED is needed because the drain
// advisory lock already serializes drains.
func ClaimPropagationByID(ctx context.Context, id string) (*models.PendingPropagation, bool, error) {
	const q = `
		UPDATE pending_zitadel_propagations
		SET status='in_flight', started_at=NOW()
		WHERE id=$1 AND status IN ('pending','in_flight')
		RETURNING id, op_type, user_id, project_id, role_keys, source, COALESCE(source_ref,''),
		          COALESCE(zitadel_grant_id,''),
		          status, attempts, COALESCE(last_error,''), initiated_by, created_at, started_at, completed_at`
	rows, err := PG.Query(ctx, q, id)
	if err != nil {
		return nil, false, fmt.Errorf("claim propagation %s: %w", id, err)
	}
	defer rows.Close()
	out, err := scanPropagations(rows)
	if err != nil {
		return nil, false, err
	}
	if len(out) == 0 {
		return nil, false, nil
	}
	return &out[0], true, nil
}

// MarkPropagationApplied marks a row as terminal success and clears any prior
// transient error message.
func MarkPropagationApplied(ctx context.Context, id string) error {
	return execPropagation(ctx, id,
		`UPDATE pending_zitadel_propagations SET status='applied', completed_at=NOW(), last_error=NULL WHERE id=$1`)
}

// MarkPropagationFailed marks a row terminal-failed with the operator-facing
// error. Failed rows survive the retention window as the attention-needed audit
// trail.
func MarkPropagationFailed(ctx context.Context, id, errMsg string) error {
	const q = `UPDATE pending_zitadel_propagations
		SET status='failed', completed_at=NOW(), last_error=$2 WHERE id=$1`
	if _, err := PG.Exec(ctx, q, id, errMsg); err != nil {
		return fmt.Errorf("mark propagation failed: %w", err)
	}
	return nil
}

// RequeuePropagation returns a row to pending after a transient error and bumps
// attempts. Caller decides (via attempts vs OUTBOX_MAX_RETRIES) whether to halt.
func RequeuePropagation(ctx context.Context, id, errMsg string) (int, error) {
	const q = `UPDATE pending_zitadel_propagations
		SET status='pending', attempts=attempts+1, last_error=$2, started_at=NULL
		WHERE id=$1 RETURNING attempts`
	var attempts int
	if err := PG.QueryRow(ctx, q, id, errMsg).Scan(&attempts); err != nil {
		return 0, fmt.Errorf("requeue propagation: %w", err)
	}
	return attempts, nil
}

// GetPendingPropagations returns rows still in flight (pending or in_flight),
// oldest first — the operator's "awaiting Zitadel" worklist.
func GetPendingPropagations(ctx context.Context) ([]models.PendingPropagation, error) {
	const q = `
		SELECT id, op_type, user_id, project_id, role_keys, source, COALESCE(source_ref,''),
		       COALESCE(zitadel_grant_id,''),
		       status, attempts, COALESCE(last_error,''), initiated_by, created_at, started_at, completed_at
		FROM pending_zitadel_propagations
		WHERE status IN ('pending','in_flight')
		ORDER BY created_at`
	rows, err := PG.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("get pending propagations: %w", err)
	}
	defer rows.Close()
	return scanPropagations(rows)
}

// CountPendingPropagations counts rows still in flight (pending or in_flight) —
// the badge/callout depth. Terminal rows (applied/failed) are excluded.
func CountPendingPropagations(ctx context.Context) (int, error) {
	const q = `SELECT COUNT(*) FROM pending_zitadel_propagations WHERE status IN ('pending','in_flight')`
	var n int
	if err := PG.QueryRow(ctx, q).Scan(&n); err != nil {
		return 0, fmt.Errorf("count pending propagations: %w", err)
	}
	return n, nil
}

// PruneTerminalPropagations deletes applied/failed rows older than retentionDays.
// The outbox is ephemeral workflow state — canonical intent lives in
// direct_role_grants — so terminal rows are safe to drop after the window.
// `failed` rows are kept the full window as the audit trail of attention-needing
// mutations. Returns the number of rows pruned.
func PruneTerminalPropagations(ctx context.Context, retentionDays int) (int64, error) {
	const q = `DELETE FROM pending_zitadel_propagations
		WHERE status IN ('applied','failed')
		  AND completed_at IS NOT NULL
		  AND completed_at < NOW() - ($1 || ' days')::interval`
	tag, err := PG.Exec(ctx, q, retentionDays)
	if err != nil {
		return 0, fmt.Errorf("prune terminal propagations: %w", err)
	}
	return tag.RowsAffected(), nil
}

// ReconcileLedgerOnApplied prunes direct_role_grants so the intent ledger matches
// the desired state a just-applied revoke/replace established in Zitadel. It is
// the "cleaned up on the applied drain" path the transactional enqueue defers to
// (propagation_enqueue.go): the enqueue writes/keeps grant rows for add/replace
// but cannot know, at enqueue time, which old rows a revoke/replace supersedes —
// that is only settled once the Zitadel mutation applies.
//
//   - revoke:  delete the named (user, project, role) rows scoped to the outbox row's own
//     source. Cascades (source='bundle'|'rule') write no ledger rows, so a cascade revoke
//     deletes nothing here; an operator revoke (source='direct') deletes exactly its own row,
//     identical to pre-sub-phase-3 behavior (every row was source='direct' then). Without this
//     scoping an unscoped delete could strip an operator's direct grant that happens to share
//     the (user, project, role) triple with a cascade-sourced revoke (review P1).
//   - replace: delete any direct-sourced row on (user, project) whose role is NOT
//     in the new set; the new roles were already upserted at enqueue. Scoped to
//     source='direct' so it never prunes a bundle/rule projection sharing the
//     project (sub-phase 3).
//   - add:     no-op — the enqueue already upserted the rows.
//
// Called only AFTER the Zitadel mutation is confirmed applied, so the ledger can
// never drop a grant Zitadel still holds.
func ReconcileLedgerOnApplied(ctx context.Context, opType, userID, projectID string, roleKeys []string, source string) error {
	switch opType {
	case "revoke":
		const q = `DELETE FROM direct_role_grants
			WHERE user_id=$1 AND zitadel_project_id=$2 AND zitadel_role_key = ANY($3) AND source=$4`
		if _, err := PG.Exec(ctx, q, userID, projectID, roleKeys, source); err != nil {
			return fmt.Errorf("reconcile ledger (revoke): %w", err)
		}
	case "replace":
		const q = `DELETE FROM direct_role_grants
			WHERE user_id=$1 AND zitadel_project_id=$2 AND source='direct'
			  AND NOT (zitadel_role_key = ANY($3))`
		if _, err := PG.Exec(ctx, q, userID, projectID, roleKeys); err != nil {
			return fmt.Errorf("reconcile ledger (replace): %w", err)
		}
	}
	return nil
}

func execPropagation(ctx context.Context, id, q string) error {
	if _, err := PG.Exec(ctx, q, id); err != nil {
		return fmt.Errorf("update propagation %s: %w", id, err)
	}
	return nil
}

func scanPropagations(rows pgx.Rows) ([]models.PendingPropagation, error) {
	var out []models.PendingPropagation
	for rows.Next() {
		var p models.PendingPropagation
		if err := rows.Scan(&p.ID, &p.OpType, &p.UserID, &p.ProjectID, &p.RoleKeys,
			&p.Source, &p.SourceRef,
			&p.ZitadelGrantID, &p.Status, &p.Attempts, &p.LastError, &p.InitiatedBy,
			&p.CreatedAt, &p.StartedAt, &p.CompletedAt); err != nil {
			return nil, fmt.Errorf("scan propagation: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
