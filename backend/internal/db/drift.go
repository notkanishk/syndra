package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"syndra/internal/models"
)

// DriftFilter narrows a drift listing. Empty fields are ignored.
type DriftFilter struct {
	UserID          string
	ProjectID       string
	DetectionSource string // webhook | reconciliation_sweep
	Status          string // defaults to pending_triage when empty (see GetDriftItems)
}

// UpsertDriftItem inserts a pending drift row, deduped by the partial-unique
// index (target, user_id, project_id, drift_type, role_keys) WHERE
// status='pending_triage'. Callers pass ONE role per call (single-element
// role_keys); the role is part of the dedup key so a second drifting role on
// the same pair is NOT swallowed. `target` is part of it for the same reason at
// one level up: without it, two targets drifting on one user would silently
// suppress each other. Every caller here writes Zitadel drift and leaves the
// column to its default; add-on callers arrive with the sweep's target
// threading (change `addon-platform` task 1.12).
// Returns (id, inserted). On an existing identical pending row it returns
// ("", false) — a re-detection of the same drift is a no-op, not a second entry.
func UpsertDriftItem(ctx context.Context, userID, projectID string, roleKeys []string,
	zitadelGrantID, detectionSource, driftType string) (string, bool, error) {
	return UpsertDriftItemWithEvidence(ctx, userID, projectID, roleKeys,
		zitadelGrantID, detectionSource, driftType, DriftEvidence{})
}

// DriftEvidence is what the detector knows about the upstream change, if
// anything. A webhook carries the editor and the event time; the reconciliation
// sweep compares grant sets and knows neither. Zero values stay NULL — the
// triage row then says the actor is unknown instead of naming a plausible one.
type DriftEvidence struct {
	UpstreamActor     string
	UpstreamCreatedAt *time.Time
}

// UpsertDriftItemWithEvidence is UpsertDriftItem plus the upstream evidence a
// triage row needs to explain itself. `last_seen_at` is stamped on every call —
// including the deduped no-op — so "still there as of this morning's sweep" is
// answerable for a row first found nine days ago.
func UpsertDriftItemWithEvidence(ctx context.Context, userID, projectID string, roleKeys []string,
	zitadelGrantID, detectionSource, driftType string, ev DriftEvidence) (string, bool, error) {
	const q = `
		INSERT INTO drift_items (user_id, project_id, role_keys, zitadel_grant_id, detection_source, drift_type,
		                         upstream_actor, upstream_created_at, last_seen_at)
		VALUES ($1,$2,$3,NULLIF($4,''),$5,$6,NULLIF($7,''),$8,NOW())
		ON CONFLICT (target, user_id, project_id, drift_type, role_keys) WHERE (status = 'pending_triage')
		DO UPDATE SET
			last_seen_at        = NOW(),
			-- Never overwrite known evidence with an unknown: a sweep re-detecting
			-- what a webhook already attributed must not erase the actor.
			upstream_actor      = COALESCE(drift_items.upstream_actor, EXCLUDED.upstream_actor),
			upstream_created_at = COALESCE(drift_items.upstream_created_at, EXCLUDED.upstream_created_at)
		RETURNING id, (xmax = 0) AS inserted`
	var id string
	var inserted bool
	err := PG.QueryRow(ctx, q, userID, projectID, roleKeys, zitadelGrantID, detectionSource, driftType,
		ev.UpstreamActor, ev.UpstreamCreatedAt).Scan(&id, &inserted)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("upsert drift item: %w", err)
	}
	return id, inserted, nil
}

// GetDriftItems lists drift rows by filter, newest first (design §7 Q5:
// detected_at DESC default). An empty Status filter defaults to pending_triage.
func GetDriftItems(ctx context.Context, f DriftFilter) ([]models.DriftItem, error) {
	status := f.Status
	if status == "" {
		status = "pending_triage"
	}
	const q = `
		SELECT id, user_id, project_id, role_keys, COALESCE(zitadel_grant_id,''),
		       detected_at, detection_source, drift_type, status,
		       resolved_at, COALESCE(resolved_by,''), COALESCE(resolution_payload_json::text,''),
		       COALESCE(upstream_actor,''), upstream_created_at, last_seen_at
		FROM drift_items
		WHERE status = $1
		  AND ($2 = '' OR user_id = $2)
		  AND ($3 = '' OR project_id = $3)
		  AND ($4 = '' OR detection_source = $4)
		ORDER BY detected_at DESC`
	rows, err := PG.Query(ctx, q, status, f.UserID, f.ProjectID, f.DetectionSource)
	if err != nil {
		return nil, fmt.Errorf("get drift items: %w", err)
	}
	defer rows.Close()
	return scanDriftItems(rows)
}

// GetDriftItem fetches one row by id (any status). ErrDriftNotFound on miss.
func GetDriftItem(ctx context.Context, id string) (models.DriftItem, error) {
	const q = `
		SELECT id, user_id, project_id, role_keys, COALESCE(zitadel_grant_id,''),
		       detected_at, detection_source, drift_type, status,
		       resolved_at, COALESCE(resolved_by,''), COALESCE(resolution_payload_json::text,''),
		       COALESCE(upstream_actor,''), upstream_created_at, last_seen_at
		FROM drift_items WHERE id = $1`
	rows, err := PG.Query(ctx, q, id)
	if err != nil {
		return models.DriftItem{}, fmt.Errorf("get drift item: %w", err)
	}
	defer rows.Close()
	items, err := scanDriftItems(rows)
	if err != nil {
		return models.DriftItem{}, err
	}
	if len(items) == 0 {
		return models.DriftItem{}, ErrDriftNotFound
	}
	return items[0], nil
}

// CountPendingDrift is the number badge for the sidebar dot + dashboard callout.
func CountPendingDrift(ctx context.Context) (int, error) {
	var n int
	if err := PG.QueryRow(ctx, `SELECT COUNT(*) FROM drift_items WHERE status='pending_triage'`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count pending drift: %w", err)
	}
	return n, nil
}

func scanDriftItems(rows pgx.Rows) ([]models.DriftItem, error) {
	var out []models.DriftItem
	for rows.Next() {
		var d models.DriftItem
		if err := rows.Scan(&d.ID, &d.UserID, &d.ProjectID, &d.RoleKeys, &d.ZitadelGrantID,
			&d.DetectedAt, &d.DetectionSource, &d.DriftType, &d.Status,
			&d.ResolvedAt, &d.ResolvedBy, &d.ResolutionPayload,
			&d.UpstreamActor, &d.UpstreamCreatedAt, &d.LastSeenAt); err != nil {
			return nil, fmt.Errorf("scan drift item: %w", err)
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

var (
	ErrDriftNotFound   = errors.New("drift item not found")
	ErrDriftNotPending = errors.New("drift item not pending")
)

// AttributeDriftTx claims a pending drift (→attributed) and writes the
// attribution's ledger + audit rows in ONE tx. p.PayloadJSON doubles as the
// resolution payload. ErrDriftNotPending on a lost race (whole tx rolled back).
//
// No outbox row, and the name says so — this is MarkDriftExternalTx's sibling,
// not EnqueueDirectGrantPropagation's. Adoption is Syndra recording access
// Zitadel already has; the outbox encodes one intent only, "make it so", and
// there is no opcode for "confirm it is there". An `add` row that outlives the
// adoption is a live instruction to re-create the grant, so an operator who
// adopts a role and then removes it upstream by hand gets it back on the next
// drain. Enqueuing one and draining it immediately only narrowed that window;
// it did not close it, because a drain that cannot reach Zitadel leaves the row
// behind.
//
// The verification that drain bought is not lost, only relocated: if the grant
// vanished between detection and adoption, the ledger row is now the thing that
// disagrees with Zitadel, the next reconcile raises it as syndra_only drift, and
// a human triages it. Surfacing that beats silently re-granting it.
func AttributeDriftTx(ctx context.Context, driftID string, p EnqueueParams) error {
	tx, err := PG.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin attribute tx: %w", err)
	}
	defer tx.Rollback(ctx) // no-op after Commit
	if err := claimDriftTx(ctx, tx, driftID, "attributed", p.GrantedBy, p.PayloadJSON); err != nil {
		return err
	}
	p.NoPropagation = true
	key, err := newOutboxIdempotencyKey()
	if err != nil {
		return err
	}
	if _, err := enqueueWrites(ctx, tx, p, key); err != nil {
		return fmt.Errorf("attribute ledger writes: %w", err)
	}
	return tx.Commit(ctx)
}

// RevokeDriftAndEnqueue claims a pending drift (→revoked) and enqueues a revoke
// outbox row in ONE tx (p.OpType must be "revoke"; enqueueWrites skips the ledger
// upsert for revoke). Returns the outbox id so the handler can drain it
// best-effort AFTER commit. ErrDriftNotPending on a lost race.
func RevokeDriftAndEnqueue(ctx context.Context, driftID string, p EnqueueParams) (string, error) {
	tx, err := PG.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin revoke tx: %w", err)
	}
	defer tx.Rollback(ctx)
	if err := claimDriftTx(ctx, tx, driftID, "revoked", p.GrantedBy, "{}"); err != nil {
		return "", err
	}
	key, err := newOutboxIdempotencyKey()
	if err != nil {
		return "", err
	}
	outboxID, err := enqueueWrites(ctx, tx, p, key)
	if err != nil {
		return "", fmt.Errorf("revoke enqueue writes: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit revoke tx: %w", err)
	}
	return outboxID, nil
}

// MarkDriftExternalTx claims a pending drift (→marked_external) and inserts the
// exclusion rows in ONE tx. ErrDriftNotPending on a lost race (no exclusion written).
func MarkDriftExternalTx(ctx context.Context, driftID, userID, projectID string,
	roleKeys []string, markedBy, reason, payloadJSON string) error {
	tx, err := PG.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin mark-external tx: %w", err)
	}
	defer tx.Rollback(ctx)
	if err := claimDriftTx(ctx, tx, driftID, "marked_external", markedBy, payloadJSON); err != nil {
		return err
	}
	const ins = `INSERT INTO external_grant_exclusions (user_id, project_id, role_key, marked_by, reason)
		VALUES ($1,$2,$3,$4,NULLIF($5,'')) ON CONFLICT (user_id, project_id, role_key) DO NOTHING`
	for _, rk := range roleKeys {
		if _, err := tx.Exec(ctx, ins, userID, projectID, rk, markedBy, reason); err != nil {
			return fmt.Errorf("insert exclusion in tx: %w", err)
		}
	}
	return tx.Commit(ctx)
}

// claimDriftTx is the shared guarded transition: flips a pending drift row to a
// terminal status inside the caller's tx, or returns ErrDriftNotPending (which
// makes the caller's deferred Rollback discard everything) when it is no longer
// pending. This is what makes the whole action atomic AND race-safe.
func claimDriftTx(ctx context.Context, tx pgx.Tx, driftID, status, resolvedBy, payloadJSON string) error {
	tag, err := tx.Exec(ctx, `UPDATE drift_items
		SET status=$2, resolved_at=NOW(), resolved_by=$3, resolution_payload_json=NULLIF($4,'')::jsonb
		WHERE id=$1 AND status='pending_triage'`, driftID, status, resolvedBy, payloadJSON)
	if err != nil {
		return fmt.Errorf("claim drift %s: %w", driftID, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrDriftNotPending
	}
	return nil
}
