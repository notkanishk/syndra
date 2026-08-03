package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"mkauth/internal/models"
)

// -------------------------------------------------------------
// DIRECT ROLE GRANTS
// -------------------------------------------------------------

func UpsertDirectGrant(ctx context.Context, userID, projectID, roleKey, grantedBy, reason string, expiresAt *time.Time) (string, error) {
	query := `
		INSERT INTO direct_role_grants (
			user_id, zitadel_project_id, zitadel_role_key, granted_by, reason, expires_at
		)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id, zitadel_project_id, zitadel_role_key)
		DO UPDATE SET
			granted_by = EXCLUDED.granted_by,
			reason = EXCLUDED.reason,
			expires_at = EXCLUDED.expires_at,
			updated_at = CURRENT_TIMESTAMP
		RETURNING id;`

	var id string
	if err := PG.QueryRow(ctx, query, userID, projectID, roleKey, grantedBy, reason, expiresAt).Scan(&id); err != nil {
		return "", fmt.Errorf("failed to upsert direct grant: %w", err)
	}
	return id, nil
}

func GetDirectGrantsForUser(ctx context.Context, userID string, includeExpired bool) ([]models.DirectGrant, error) {
	query := `
		SELECT id, user_id, zitadel_project_id, zitadel_role_key, granted_by, COALESCE(reason, ''), expires_at, created_at, updated_at, COALESCE(source, 'direct'), COALESCE(source_ref, '')
		FROM direct_role_grants
		WHERE user_id = $1`
	if !includeExpired {
		query += ` AND (expires_at IS NULL OR expires_at > NOW())`
	}
	query += ` ORDER BY created_at DESC`

	rows, err := PG.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var grants []models.DirectGrant
	for rows.Next() {
		var grant models.DirectGrant
		if err := rows.Scan(&grant.ID, &grant.UserID, &grant.ProjectID, &grant.RoleKey, &grant.GrantedBy, &grant.Reason, &grant.ExpiresAt, &grant.CreatedAt, &grant.UpdatedAt, &grant.Source, &grant.SourceRef); err != nil {
			return nil, err
		}
		grants = append(grants, grant)
	}
	return grants, nil
}

// GetAllDirectGrants returns every MkAuth-direct grant in the system. When
// includeExpired is false the result is filtered to active grants only
// (expires_at NULL or in the future). Ordered by user/project/role for
// deterministic pairing during reconciliation.
func GetAllDirectGrants(ctx context.Context, includeExpired bool) ([]models.DirectGrant, error) {
	query := `
		SELECT id, user_id, zitadel_project_id, zitadel_role_key, granted_by, COALESCE(reason, ''), expires_at, created_at, updated_at, COALESCE(source, 'direct'), COALESCE(source_ref, '')
		FROM direct_role_grants`
	if !includeExpired {
		query += ` WHERE (expires_at IS NULL OR expires_at > NOW())`
	}
	query += ` ORDER BY user_id, zitadel_project_id, zitadel_role_key`

	rows, err := PG.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var grants []models.DirectGrant
	for rows.Next() {
		var grant models.DirectGrant
		if err := rows.Scan(&grant.ID, &grant.UserID, &grant.ProjectID, &grant.RoleKey, &grant.GrantedBy, &grant.Reason, &grant.ExpiresAt, &grant.CreatedAt, &grant.UpdatedAt, &grant.Source, &grant.SourceRef); err != nil {
			return nil, err
		}
		grants = append(grants, grant)
	}
	// Surface mid-stream iteration errors so reconciliation never compares
	// against a silently-truncated MkAuth inventory.
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return grants, nil
}

func GetExpiringDirectGrants(ctx context.Context, within time.Duration) ([]models.DirectGrant, error) {
	query := `
		SELECT id, user_id, zitadel_project_id, zitadel_role_key, granted_by, COALESCE(reason, ''), expires_at, created_at, updated_at
		FROM direct_role_grants
		WHERE expires_at IS NOT NULL
		  AND expires_at > NOW()
		  AND expires_at <= NOW() + $1::interval
		ORDER BY expires_at ASC`

	rows, err := PG.Query(ctx, query, fmt.Sprintf("%f seconds", within.Seconds()))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var grants []models.DirectGrant
	for rows.Next() {
		var grant models.DirectGrant
		if err := rows.Scan(&grant.ID, &grant.UserID, &grant.ProjectID, &grant.RoleKey, &grant.GrantedBy, &grant.Reason, &grant.ExpiresAt, &grant.CreatedAt, &grant.UpdatedAt); err != nil {
			return nil, err
		}
		grants = append(grants, grant)
	}
	return grants, nil
}

// GetExpiredDirectGrants returns direct role grants whose expires_at is in the
// past, ordered by expires_at ascending (oldest first). Used by the expiry
// scheduler to sweep grants needing cleanup. limit caps the batch size.
func GetExpiredDirectGrants(ctx context.Context, limit int) ([]models.DirectGrant, error) {
	query := `
		SELECT id, user_id, zitadel_project_id, zitadel_role_key, granted_by, COALESCE(reason, ''), expires_at, created_at, updated_at
		FROM direct_role_grants
		WHERE expires_at IS NOT NULL
		  AND expires_at <= NOW()
		ORDER BY expires_at ASC
		LIMIT $1`

	rows, err := PG.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var grants []models.DirectGrant
	for rows.Next() {
		var grant models.DirectGrant
		if err := rows.Scan(&grant.ID, &grant.UserID, &grant.ProjectID, &grant.RoleKey, &grant.GrantedBy, &grant.Reason, &grant.ExpiresAt, &grant.CreatedAt, &grant.UpdatedAt); err != nil {
			return nil, err
		}
		grants = append(grants, grant)
	}
	return grants, nil
}

// DeleteExpiredDirectGrantsByIDs atomically hard-deletes direct role grants
// for a specific user BUT only rows whose expires_at is still in the past at
// the moment of the DELETE. Rows that were concurrently renewed via
// UpsertDirectGrant (which uses ON CONFLICT DO UPDATE and therefore preserves
// the row ID while pushing expires_at forward) will not satisfy the predicate
// and will survive.
//
// The RETURNING clause guarantees the caller sees the exact set of rows that
// were actually removed; downstream steps (intent emission, audit, cascade)
// MUST be driven from this returned slice, not from the pre-fetch snapshot.
//
// The user_id scoping is defensive: it guarantees a caller cannot delete
// another user's grants even if IDs were mis-grouped upstream.
func DeleteExpiredDirectGrantsByIDs(ctx context.Context, userID string, ids []string) ([]models.DirectGrant, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	query := `
		DELETE FROM direct_role_grants
		WHERE user_id = $1
		  AND id = ANY($2::uuid[])
		  AND expires_at IS NOT NULL
		  AND expires_at <= NOW()
		RETURNING id, user_id, zitadel_project_id, zitadel_role_key, granted_by, COALESCE(reason, ''), expires_at, created_at, updated_at`

	rows, err := PG.Query(ctx, query, userID, ids)
	if err != nil {
		return nil, fmt.Errorf("failed to delete expired direct grants: %w", err)
	}
	defer rows.Close()

	var deleted []models.DirectGrant
	for rows.Next() {
		var g models.DirectGrant
		if err := rows.Scan(&g.ID, &g.UserID, &g.ProjectID, &g.RoleKey, &g.GrantedBy, &g.Reason, &g.ExpiresAt, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, err
		}
		deleted = append(deleted, g)
	}
	return deleted, nil
}

// ErrGrantNotFound is returned when the (user, grant) pair names no row. The
// handler maps it to 404 rather than a generic 500 — an operator clicking
// remove twice should be told the grant is already gone, not that the server
// broke.
var ErrGrantNotFound = errors.New("direct grant not found")

// DeleteDirectGrantAndEnqueue removes one direct grant and enqueues the
// caller-computed effective-access delta in a single transaction: ledger
// delete, audit row, outbox rows.
//
// `params` is a DELTA, not "a revoke". The caller (services.DeleteDirectGrant)
// computes the user's effective-role closure before and after the deletion, so
// a role the person still holds through a bundle or a mapping rule produces no
// revoke at all. Enqueuing an unconditional revoke here would contradict the
// confirmation dialog's promise that the role is retained, and would take the
// access away upstream until the next compile put it back.
//
// params may be empty — every role stayed covered — and the grant row is still
// deleted. Nothing reaches Zitadel here; the drain does that later, from rows
// that are already durable.
//
// This is the MkAuth-side delete. The Zitadel-side grant delete
// (DELETE /zitadel/users/{id}/grants/{grantId}) removes a different object and
// leaves this row behind, so the next cache compile would restore the access.
func DeleteDirectGrantAndEnqueue(ctx context.Context, actor, userID, grantID string, params []EnqueueParams) ([]string, error) {
	tx, err := PG.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin delete grant tx: %w", err)
	}
	defer tx.Rollback(ctx) // no-op after a successful Commit

	// Delete and read back in one statement: the returned project/role are what
	// the audit row names, and taking them from the deleted row makes the record
	// provably about the grant that just went away.
	const deleteGrant = `
		DELETE FROM direct_role_grants
		WHERE id = $1 AND user_id = $2
		RETURNING zitadel_project_id, zitadel_role_key`
	var projectID, roleKey string
	if err := tx.QueryRow(ctx, deleteGrant, grantID, userID).Scan(&projectID, &roleKey); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrGrantNotFound
		}
		return nil, fmt.Errorf("delete direct grant %s: %w", grantID, err)
	}

	ids, err := enqueueCascadeRows(ctx, tx,
		[]CascadeAudit{{Actor: actor, Target: userID, Action: "direct_grant.removed", ResourceID: projectID + "/" + roleKey}},
		params)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit delete grant tx: %w", err)
	}
	return ids, nil
}
