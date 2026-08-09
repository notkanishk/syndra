package db

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"syndra/internal/models"
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
	const q = `SELECT status FROM propagation_outbox WHERE id=$1`
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
// propagation_outbox buffers every Syndra-mediated Zitadel grant
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
	// This is the Zitadel enqueue and the statement says so. Its columns are the
	// Zitadel shape — a project, role keys, a grant id — so the target is not a
	// parameter; there is no other target it could name. Add-on entitlement work
	// goes through EnqueueEntitlementApplyTx, which derives its target from the
	// approved plan.
	const q = `
		INSERT INTO propagation_outbox
			(op_type, user_id, project_id, role_keys, zitadel_grant_id, payload_json, idempotency_key, initiated_by, target)
		VALUES ($1,$2,$3,$4,NULLIF($5,''),$6,$7,$8,'zitadel')
		RETURNING id`
	var id string
	if err := PG.QueryRow(ctx, q, opType, userID, projectID, roleKeys, zitadelGrantID,
		payloadJSON, idempotencyKey, initiatedBy).Scan(&id); err != nil {
		return "", fmt.Errorf("insert propagation: %w", err)
	}
	return id, nil
}

// PendingOutboxAddExists reports whether an undrained add is already queued for
// the (target, user, project, role) tuple, so the drift sweep's syndra_only
// replay does not pile a fresh duplicate every tick for a grant that stays
// missing on the target.
//
// The target belongs in the predicate, not in the caller's head. The question
// this answers is "is something already queued that will fix this drift", and
// queued work on another target fixes nothing here: without the scope, a row
// waiting on an unreachable add-on would suppress the replay of a genuinely
// missing Zitadel grant, and the sweep would report itself satisfied.
func PendingOutboxAddExists(ctx context.Context, target, userID, projectID, roleKey string) (bool, error) {
	const q = `SELECT EXISTS(
		SELECT 1 FROM propagation_outbox
		WHERE op_type='add' AND target=$1 AND user_id=$2 AND project_id=$3
		  AND $4 = ANY(role_keys) AND status IN ('pending','in_flight'))`
	var exists bool
	if err := PG.QueryRow(ctx, q, target, userID, projectID, roleKey).Scan(&exists); err != nil {
		return false, fmt.Errorf("pending outbox add exists (%s %s/%s/%s): %w", target, userID, projectID, roleKey, err)
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
func ClaimPendingPropagations(ctx context.Context, target string, limit int) ([]models.PendingPropagation, error) {
	if limit <= 0 {
		limit = 100
	}
	// Scoped to one target, and to a target that is still registered. A drain
	// holds exactly one dispatcher, so claiming another target's rows would
	// push them through machinery shaped for a system they are not for — a
	// TrueNAS row has no project and no roles for the Zitadel path to send.
	//
	// The active-target join is in the claim rather than in the caller for the
	// usual reason: this is exported, and an invariant a caller enforces is one
	// the next caller can skip.
	const q = `
		WITH claimed AS (
			SELECT p.id FROM propagation_outbox p
			JOIN targets t ON t.target = p.target AND t.state = 'active'
			WHERE p.status IN ('pending','in_flight') AND p.target = $2
			ORDER BY p.created_at
			LIMIT $1
			FOR UPDATE OF p SKIP LOCKED
		)
		UPDATE propagation_outbox p
		SET status = 'in_flight', started_at = NOW()
		FROM claimed
		WHERE p.id = claimed.id
		RETURNING p.id, p.target, p.op_type, p.user_id, COALESCE(p.project_id,''), COALESCE(p.role_keys,'{}'),
		          p.source, COALESCE(p.source_ref,''), COALESCE(p.cascade_id::text,''),
		          COALESCE(p.zitadel_grant_id,''), p.status, p.attempts,
		          COALESCE(p.last_error,''), p.initiated_by, p.created_at, p.started_at, p.completed_at`
	rows, err := PG.Query(ctx, q, limit, target)
	if err != nil {
		return nil, fmt.Errorf("claim propagations: %w", err)
	}
	defer rows.Close()
	return scanPropagations(rows)
}

// ClaimPropagationByID atomically transitions ONE row (pending or in_flight) to
// in_flight and returns it — the targeted inline-apply claim behind DrainOne.
// found=false when the row no longer exists, is already terminal
// (applied/failed), belongs to a target that is no longer registered, or
// belongs to a target other than the one the caller can dispatch. Every one of
// those is a no-op for the caller.
//
// Target-scoped for the same reason the batch claim is, and it matters more
// here: a row claimed and then found undispatchable has to be put back, and
// every way of putting it back costs something. A requeue spends a retry and
// records a dispatch failure for a dispatch that never happened, so a handful
// of targeted applies would exhaust an add-on row's budget before its
// dispatcher exists — and its first real transient response would halt it. Not
// claiming it costs nothing.
//
// It mirrors ClaimPendingPropagations' claimable status set and started_at
// reset but is scoped to a single id; no FOR UPDATE SKIP LOCKED is needed
// because the drain advisory lock already serializes drains.
func ClaimPropagationByID(ctx context.Context, target, id string) (*models.PendingPropagation, bool, error) {
	const q = `
		UPDATE propagation_outbox p
		SET status='in_flight', started_at=NOW()
		FROM targets t
		WHERE p.id=$1 AND p.status IN ('pending','in_flight') AND p.target = $2
		  AND t.target = p.target AND t.state = 'active'
		RETURNING p.id, p.target, p.op_type, p.user_id, COALESCE(p.project_id,''), COALESCE(p.role_keys,'{}'),
		          p.source, COALESCE(p.source_ref,''),
		          COALESCE(p.cascade_id::text,''), COALESCE(p.zitadel_grant_id,''),
		          p.status, p.attempts, COALESCE(p.last_error,''), p.initiated_by, p.created_at, p.started_at, p.completed_at`
	rows, err := PG.Query(ctx, q, id, target)
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

// TargetsAwaitingDispatch lists active targets holding unresolved outbox rows,
// excluding the one just drained.
//
// It exists so a drain can SAY what it did not touch. A pass that silently
// dispatches one target's work while another's waits is indistinguishable, from
// the outside, from a system with nothing left to do.
func TargetsAwaitingDispatch(ctx context.Context, drained string) ([]string, error) {
	const q = `
		SELECT DISTINCT p.target
		  FROM propagation_outbox p
		  JOIN targets t ON t.target = p.target AND t.state = 'active'
		 WHERE p.status IN ('pending','in_flight') AND p.target <> $1
		 ORDER BY p.target`
	rows, err := PG.Query(ctx, q, drained)
	if err != nil {
		return nil, fmt.Errorf("list targets awaiting dispatch: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, fmt.Errorf("scan target awaiting dispatch: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// UndispatchableTarget reports the target of an unresolved row that a drain for
// `dispatcher` may not dispatch, or "" when there is nothing to say.
//
// Used only after a claim declined a row, and only to explain it. A targeted
// apply that quietly did nothing is worse than one that says which target it
// could not reach — and the read is the cheapest way to say it, because the
// alternative is claiming the row to find out and then paying to put it back.
func UndispatchableTarget(ctx context.Context, dispatcher, id string) (string, error) {
	const q = `
		SELECT p.target
		  FROM propagation_outbox p
		  JOIN targets t ON t.target = p.target AND t.state = 'active'
		 WHERE p.id = $1 AND p.target <> $2 AND p.status IN ('pending','in_flight')`
	var target string
	if err := PG.QueryRow(ctx, q, id, dispatcher).Scan(&target); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("read undispatchable target for %s: %w", id, err)
	}
	return target, nil
}

// ErrPropagationNotInFlight means the row this drain was settling is no longer
// the drain's to settle: something terminated it while the dispatch was out.
//
// Today that something is deregistration, which abandons a target's unresolved
// rows. It is not an error — the row reached a terminal state legitimately —
// but it is emphatically not success either, and a settle that quietly did
// nothing would be counted as one.
var ErrPropagationNotInFlight = errors.New("db: the propagation row is no longer in flight")

// Every finalizer below is guarded by `status='in_flight'`, which is the status
// the claim set and the only status a settle may act on.
//
// Unguarded, they overwrite whatever the row became while the dispatch was out.
// The worst is the requeue: it would return an abandoned row to `pending` on a
// deregistered target, recreating the undrainable row the deregistration had
// just resolved — and invisibly, because the sweep that would have caught it
// has already run.
// settleOne runs a guarded finalizer and turns "matched nothing" into the
// sentinel.
//
// One place, because the mapping is the whole safety property and three copies
// of it are three chances to write `return nil` instead. A settle that affected
// no rows did not settle: the caller is about to count an outcome it never
// recorded.
func settleOne(ctx context.Context, what, id, q string, args ...any) error {
	tag, err := PG.Exec(ctx, q, append([]any{id}, args...)...)
	if err != nil {
		return fmt.Errorf("%s: %w", what, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: %s", ErrPropagationNotInFlight, id)
	}
	return nil
}

// MarkPropagationApplied marks a row as terminal success and clears any prior
// transient error message.
func MarkPropagationApplied(ctx context.Context, id string) error {
	return settleOne(ctx, "mark propagation applied", id,
		`UPDATE propagation_outbox SET status='applied', completed_at=NOW(), last_error=NULL
		 WHERE id=$1 AND status='in_flight'`)
}

// MarkPropagationFailed marks a row terminal-failed with the operator-facing
// error. Failed rows survive the retention window as the attention-needed audit
// trail.
func MarkPropagationFailed(ctx context.Context, id, errMsg string) error {
	return settleOne(ctx, "mark propagation failed", id,
		`UPDATE propagation_outbox SET status='failed', completed_at=NOW(), last_error=$2
		 WHERE id=$1 AND status='in_flight'`, errMsg)
}

// RequeuePropagation returns a row to pending after a transient error and bumps
// attempts. Caller decides (via attempts vs OUTBOX_MAX_RETRIES) whether to halt.
func RequeuePropagation(ctx context.Context, id, errMsg string) (int, error) {
	const q = `UPDATE propagation_outbox
		SET status='pending', attempts=attempts+1, last_error=$2, started_at=NULL
		WHERE id=$1 AND status='in_flight' RETURNING attempts`
	var attempts int
	err := PG.QueryRow(ctx, q, id, errMsg).Scan(&attempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, fmt.Errorf("%w: %s", ErrPropagationNotInFlight, id)
	}
	if err != nil {
		return 0, fmt.Errorf("requeue propagation: %w", err)
	}
	return attempts, nil
}

// GetPendingPropagations returns rows still in flight (pending or in_flight),
// oldest first — the operator's "awaiting Zitadel" worklist.
func GetPendingPropagations(ctx context.Context) ([]models.PendingPropagation, error) {
	const q = `
		SELECT id, target, op_type, user_id, COALESCE(project_id,''), COALESCE(role_keys,'{}'),
		       source, COALESCE(source_ref,''),
		       COALESCE(cascade_id::text,''), COALESCE(zitadel_grant_id,''),
		       status, attempts, COALESCE(last_error,''), initiated_by, created_at, started_at, completed_at
		FROM propagation_outbox
		WHERE status IN ('pending','in_flight')
		ORDER BY cascade_id NULLS LAST, created_at`
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
	const q = `SELECT COUNT(*) FROM propagation_outbox WHERE status IN ('pending','in_flight')`
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
	const q = `DELETE FROM propagation_outbox
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
// It takes the access-mutation lock, because deleting a ledger row changes what
// somebody effectively holds and every delta is computed from those rows. The
// drain is not a cascade, but its effect on the closure is the same: a cascade
// can lock, read a direct grant that is still present, conclude the role it was
// about to add is already covered, and commit that empty delta — while this
// deletion lands in between and takes the cover away. Nobody adds it back,
// because nobody thought it was missing.
//
// The lock is taken HERE and not around the dispatch. The Zitadel call has
// already happened by the time this runs, so nothing holds the lock across the
// network, and the window this closes is between the call returning and the
// ledger catching up.
func ReconcileLedgerOnApplied(ctx context.Context, outboxID string) error {
	return InTxLockingAccess(ctx, func(ctx context.Context) error {
		tx, ok := ctx.Value(txKey).(pgx.Tx)
		if !ok || tx == nil {
			return fmt.Errorf("reconcile ledger: no transaction")
		}
		// Everything is read from the row being reconciled rather than passed
		// alongside it. What has to hold is that the tuple, the source and the
		// moment all describe THE SAME decision — a caller assembling them by
		// hand can pass a set that never existed together, and the ordering
		// comparison below is meaningless unless the timestamp is this row's.
		var opType, userID, projectID, source string
		var roleKeys []string
		var createdAt time.Time
		err := tx.QueryRow(ctx, `
			SELECT op_type, user_id, COALESCE(project_id,''), COALESCE(role_keys,'{}'),
			       COALESCE(source,'direct'), created_at
			FROM propagation_outbox WHERE id = $1`, outboxID).Scan(
			&opType, &userID, &projectID, &roleKeys, &source, &createdAt)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return fmt.Errorf("reconcile ledger: read %s: %w", outboxID, err)
		}
		if opType != "revoke" && opType != "replace" {
			return nil
		}

		// A ledger row is protected only from an add that would ESTABLISH it.
		//
		// Cascade adds deliberately write no direct_role_grants row — bundle and
		// rule intent lives in their own tables — so treating any queued add as
		// proof the ledger row is newer keeps a direct grant alive that nothing
		// is maintaining. It then reads as coverage forever: removing the bundle
		// later sees it, concludes the role is still held, and queues no revoke.
		// Matching the source is what distinguishes an add that writes this row
		// from one that writes nothing.
		//
		// And it must be NEWER than the decision being reconciled. An add queued
		// before this revocation and still waiting is older intent; the
		// revocation is the later word, and the row goes.
		const newerAddExists = `
			NOT EXISTS (
			    SELECT 1 FROM propagation_outbox o
			    WHERE o.op_type = 'add'
			      AND o.status IN ('pending', 'in_flight')
			      AND o.user_id = d.user_id
			      AND o.project_id = d.zitadel_project_id
			      AND d.zitadel_role_key = ANY(o.role_keys)
			      AND o.source = d.source
			      AND o.created_at > $5)`

		switch opType {
		case "revoke":
			if _, err := tx.Exec(ctx, `DELETE FROM direct_role_grants d
				WHERE d.user_id=$1 AND d.zitadel_project_id=$2
				  AND d.zitadel_role_key = ANY($3) AND d.source=$4
				  AND `+newerAddExists, userID, projectID, roleKeys, source, createdAt); err != nil {
				return fmt.Errorf("reconcile ledger (revoke): %w", err)
			}
		case "replace":
			// The same protection, because the same thing can happen: a replace
			// narrowing A→B while a later direct add re-establishes A would
			// otherwise delete A's freshly written row, and the add would reach
			// the target with no durable record behind it.
			if _, err := tx.Exec(ctx, `DELETE FROM direct_role_grants d
				WHERE d.user_id=$1 AND d.zitadel_project_id=$2 AND d.source='direct'
				  AND NOT (d.zitadel_role_key = ANY($3))
				  AND `+newerAddExists, userID, projectID, roleKeys, "direct", createdAt); err != nil {
				return fmt.Errorf("reconcile ledger (replace): %w", err)
			}
		}
		return nil
	})
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
		if err := rows.Scan(&p.ID, &p.Target, &p.OpType, &p.UserID, &p.ProjectID, &p.RoleKeys,
			&p.Source, &p.SourceRef, &p.CascadeID,
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

// RoleRef is a (project, role) pair the outbox names.
type RoleRef struct {
	ProjectID string
	RoleKey   string
}

// QueuedRevocations lists the roles a subject has an unresolved revocation for.
//
// A queued revocation is a decision already taken. Until the drain reaches it
// the intent ledger still carries the grant — it is deleted only after the
// target confirms, so that a failed dispatch never leaves Syndra believing
// access is gone while the target still has it. That makes the ledger, on its
// own, a wrong answer to "what does this person effectively hold" for exactly
// as long as the row waits.
//
// A closure computed from that wrong answer decides nothing is missing and
// queues nothing, and the revocation then lands anyway: the subject ends up
// holding the source and not the access, with no queued row disagreeing. So the
// effective-access reads subtract these, which makes the transition visible to
// every delta from the moment it is queued rather than from the moment it is
// confirmed.
func QueuedRevocations(ctx context.Context, userID string) ([]RoleRef, error) {
	// `replace` names the roles that SURVIVE, not the ones being taken away, so
	// it cannot be unioned with revoke's role_keys. Its removals are the
	// direct-sourced ledger roles on that project which the new set omits —
	// the same predicate ReconcileLedgerOnApplied deletes by, asked ahead of
	// time instead of after.
	const q = `
		SELECT project_id, UNNEST(role_keys) AS role_key
		FROM propagation_outbox
		WHERE user_id = $1
		  AND op_type = 'revoke'
		  AND status IN ('pending', 'in_flight')
		  AND project_id IS NOT NULL AND role_keys IS NOT NULL
		UNION
		SELECT g.zitadel_project_id, g.zitadel_role_key
		FROM propagation_outbox o
		JOIN direct_role_grants g
		  ON g.user_id = o.user_id
		 AND g.zitadel_project_id = o.project_id
		 AND g.source = 'direct'
		WHERE o.user_id = $1
		  AND o.op_type = 'replace'
		  AND o.status IN ('pending', 'in_flight')
		  AND o.project_id IS NOT NULL AND o.role_keys IS NOT NULL
		  AND NOT (g.zitadel_role_key = ANY(o.role_keys))`
	rows, err := PG.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("queued revocations for %s: %w", userID, err)
	}
	defer rows.Close()
	var out []RoleRef
	for rows.Next() {
		var r RoleRef
		if err := rows.Scan(&r.ProjectID, &r.RoleKey); err != nil {
			return nil, fmt.Errorf("scan queued revocation: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
