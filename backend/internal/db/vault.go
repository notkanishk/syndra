package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"syndra/internal/models"
)

// ---------------------------------------------------------------------------
// Shadow Password Vault
// ---------------------------------------------------------------------------

// RecordCredentialSet notes that a member has set a credential on a target,
// and when (change `addon-platform` group 11).
//
// It takes no credential and there is nowhere to put one. The member's password
// is forwarded to the target by the operation that received it and is kept
// nowhere: no API in this system accepts a hash, so the only thing a stored one
// could ever do is leak. What survives is the metadata the member's own view
// renders and the answer to "have they enrolled".
func RecordCredentialSet(ctx context.Context, userID string) (string, error) {
	var id string
	err := PG.QueryRow(ctx, `
		INSERT INTO shadow_credentials (user_id)
		VALUES ($1)
		ON CONFLICT (user_id) DO UPDATE SET
			updated_at              = NOW(),
			rotated_at              = NOW(),
			-- Setting one through the new path is what clears the mark: the
			-- member has now enrolled against the system that exists.
			enrolled_before_cutover = FALSE
		RETURNING id`, userID).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("record credential for %s: %w", userID, err)
	}
	return id, nil
}

// DeleteShadowCredential removes a user's shadow credential.
func DeleteShadowCredential(ctx context.Context, userID string) error {
	tag, err := PG.Exec(ctx, `DELETE FROM shadow_credentials WHERE user_id = $1`, userID)
	if err != nil {
		return fmt.Errorf("delete shadow credential (user=%s): %w", userID, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("shadow credential not found for user %s: %w", userID, pgx.ErrNoRows)
	}
	return nil
}

// HasShadowCredential answers whether a member has enrolled, and when.
//
// There is nothing else it could answer: the table holds no credential, and
// this SELECT names every column that survives.
func HasShadowCredential(ctx context.Context, userID string) (models.ShadowCredentialStatus, error) {
	var s models.ShadowCredentialStatus
	var createdAt, updatedAt time.Time
	var rotatedAt, expiresAt *time.Time
	var beforeCutover bool
	err := PG.QueryRow(ctx, `
		SELECT created_at, updated_at, rotated_at, expires_at, enrolled_before_cutover
		FROM shadow_credentials WHERE user_id = $1`, userID).
		Scan(&createdAt, &updatedAt, &rotatedAt, &expiresAt, &beforeCutover)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.ShadowCredentialStatus{HasCredential: false}, nil
		}
		return s, fmt.Errorf("check shadow credential (user=%s): %w", userID, err)
	}
	return models.ShadowCredentialStatus{
		// A pre-cutover row is NOT a credential the member can use. The hash it
		// described is gone and the system it was for does not exist, so
		// reporting it as set would tell somebody they had enrolled when the
		// next connection attempt will fail (task 11.9).
		HasCredential:    !beforeCutover,
		NeedsReEnrolment: beforeCutover,
		CreatedAt:        &createdAt,
		UpdatedAt:        &updatedAt,
		RotatedAt:        rotatedAt,
		ExpiresAt:        expiresAt,
	}, nil
}

// InsertShadowCredentialAudit records a credential lifecycle event.
func InsertShadowCredentialAudit(ctx context.Context, userID, action, actorID, ipAddress string) error {
	_, err := PG.Exec(ctx, `
		INSERT INTO shadow_credential_audit (user_id, action, actor_id, ip_address)
		VALUES ($1, $2, $3, $4)`,
		userID, action, actorID, ipAddress)
	if err != nil {
		return fmt.Errorf("insert shadow credential audit (user=%s action=%s): %w", userID, action, err)
	}
	return nil
}

// GetShadowCredentialAudit returns the audit trail for a user's shadow credential.
func GetShadowCredentialAudit(ctx context.Context, userID string) ([]models.ShadowCredentialAudit, error) {
	rows, err := PG.Query(ctx, `
		SELECT id, user_id, action, actor_id, ip_address, created_at
		FROM shadow_credential_audit
		WHERE user_id = $1
		ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("query shadow credential audit (user=%s): %w", userID, err)
	}
	defer rows.Close()

	var entries []models.ShadowCredentialAudit
	for rows.Next() {
		var e models.ShadowCredentialAudit
		if err := rows.Scan(&e.ID, &e.UserID, &e.Action, &e.ActorID, &e.IPAddress, &e.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, nil
}
