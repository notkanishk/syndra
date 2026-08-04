package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"syndra/internal/models"
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

// GetAllDirectGrants returns every Syndra-direct grant in the system. When
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
	// against a silently-truncated Syndra inventory.
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

// ErrGrantExpiryMoved means the caller acknowledged an expiry date the grant no longer carries —
// somebody extended or re-granted it between the operator's page load and their click. The
// acknowledgement is refused rather than stored against the stale date, because storing it would
// silently do nothing: the read compares dates, so it would never apply.
var ErrGrantExpiryMoved = errors.New("grant expiry has changed since it was read")

// ErrAcknowledgementNotFound means there was nothing to take back.
var ErrAcknowledgementNotFound = errors.New("grant expiry acknowledgement not found")

// GetExpiringDirectGrantsWithAcknowledgements is the Review › Expiring access read: the same
// window as GetExpiringDirectGrants, plus the acknowledgement that currently applies to each row.
//
// Deliberately a second function rather than a wider return on the first. Four service callers
// (the governance summary, Today's queue, role members) ask "what is expiring" and have no use for
// an acknowledgement; widening the shared read would push a type through all of them and their
// tests to serve one screen.
//
// The join carries the reopen rule and is the whole of it: an acknowledgement is returned only
// while `acknowledged_expires_at` still equals the grant's `expires_at`. Move the date and the row
// comes back undecided, with nothing having had to notice.
func GetExpiringDirectGrantsWithAcknowledgements(ctx context.Context, within time.Duration) ([]models.ExpiringGrant, error) {
	const query = `
		SELECT g.id, g.user_id, g.zitadel_project_id, g.zitadel_role_key, g.granted_by,
		       COALESCE(g.reason, ''), g.expires_at, g.created_at, g.updated_at,
		       a.acknowledged_by, a.acknowledged_at, COALESCE(a.note, '')
		FROM direct_role_grants g
		LEFT JOIN grant_expiry_acknowledgements a
		       ON a.grant_id = g.id
		      AND a.acknowledged_expires_at = g.expires_at
		WHERE g.expires_at IS NOT NULL
		  AND g.expires_at > NOW()
		  AND g.expires_at <= NOW() + $1::interval
		ORDER BY g.expires_at ASC`

	rows, err := PG.Query(ctx, query, fmt.Sprintf("%f seconds", within.Seconds()))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.ExpiringGrant
	for rows.Next() {
		var grant models.ExpiringGrant
		var by *string
		var at *time.Time
		var note string
		if err := rows.Scan(&grant.ID, &grant.UserID, &grant.ProjectID, &grant.RoleKey,
			&grant.GrantedBy, &grant.Reason, &grant.ExpiresAt, &grant.CreatedAt, &grant.UpdatedAt,
			&by, &at, &note); err != nil {
			return nil, err
		}
		if by != nil && at != nil {
			grant.Acknowledged = &models.GrantExpiryAcknowledgement{By: *by, At: *at, Note: note}
		}
		out = append(out, grant)
	}
	return out, rows.Err()
}

// AcknowledgeGrantExpiry records that an operator has seen a grant's expiry and is letting it
// lapse. Returns the grant's user id, which the caller needs for the audit row.
//
// expiresAt is what the operator was looking at, and it is checked against the row rather than
// trusted. This is the same posture as the request-decision and withdraw endpoints: a stale page
// gets told what changed, not a write that appears to succeed.
//
// One row per grant: acknowledging again replaces the previous one. The table holds the current
// annotation; audit_logs holds every decision.
func AcknowledgeGrantExpiry(ctx context.Context, grantID string, expiresAt time.Time, actor, note string) (string, error) {
	tx, err := PG.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	// Read the current date under the transaction, so the comparison the caller is relying on
	// cannot be overtaken by an extension committing alongside it.
	var userID string
	var current *time.Time
	switch err := tx.QueryRow(ctx,
		`SELECT user_id, expires_at FROM direct_role_grants WHERE id = $1 FOR UPDATE`,
		grantID).Scan(&userID, &current); {
	case errors.Is(err, pgx.ErrNoRows):
		return "", ErrGrantNotFound
	case err != nil:
		return "", err
	}
	// A grant with no expiry is not on this screen and has nothing to acknowledge — the same
	// refusal as a moved date, because in both cases the date the operator read is not the date
	// the grant has.
	if current == nil || !current.Equal(expiresAt) {
		return "", ErrGrantExpiryMoved
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO grant_expiry_acknowledgements
			(grant_id, acknowledged_expires_at, acknowledged_by, note)
		VALUES ($1, $2, $3, NULLIF($4, ''))
		ON CONFLICT (grant_id) DO UPDATE
		   SET acknowledged_expires_at = EXCLUDED.acknowledged_expires_at,
		       acknowledged_by         = EXCLUDED.acknowledged_by,
		       acknowledged_at         = CURRENT_TIMESTAMP,
		       note                    = EXCLUDED.note`,
		grantID, expiresAt, actor, note); err != nil {
		return "", fmt.Errorf("acknowledge grant expiry %s: %w", grantID, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return userID, nil
}

// ClearGrantExpiryAcknowledgement takes an acknowledgement back. Returns the grant's user id for
// the audit row.
//
// Any operator may clear any acknowledgement. The queue is shared and so is the decision — the
// row names who made it, which is what makes it accountable; requiring the same person would
// leave a decision unrevisable the moment they leave for the summer.
func ClearGrantExpiryAcknowledgement(ctx context.Context, grantID string) (string, error) {
	var userID string
	err := PG.QueryRow(ctx, `
		DELETE FROM grant_expiry_acknowledgements a
		USING direct_role_grants g
		WHERE a.grant_id = $1 AND g.id = a.grant_id
		RETURNING g.user_id`, grantID).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrAcknowledgementNotFound
	}
	if err != nil {
		return "", fmt.Errorf("clear grant expiry acknowledgement %s: %w", grantID, err)
	}
	return userID, nil
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
// This is the Syndra-side delete. The Zitadel-side grant delete
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
