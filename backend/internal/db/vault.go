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

// UpsertShadowCredential inserts or replaces a user's shadow credential.
// On conflict (same user_id), the credential is updated and rotated_at is set.
func UpsertShadowCredential(ctx context.Context, userID, hash, algorithm, saltParams string) (string, error) {
	var id string
	err := PG.QueryRow(ctx, `
		INSERT INTO shadow_credentials (user_id, credential_hash, algorithm, salt_params)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id) DO UPDATE SET
			credential_hash = EXCLUDED.credential_hash,
			algorithm       = EXCLUDED.algorithm,
			salt_params     = EXCLUDED.salt_params,
			updated_at      = NOW(),
			rotated_at      = NOW()
		RETURNING id`,
		userID, hash, algorithm, saltParams).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("upsert shadow credential (user=%s): %w", userID, err)
	}
	return id, nil
}

// GetShadowCredential returns the full credential row including the hash.
// Intended for the sync service only — never expose the hash to user-facing APIs.
func GetShadowCredential(ctx context.Context, userID string) (models.ShadowCredential, error) {
	var c models.ShadowCredential
	err := PG.QueryRow(ctx, `
		SELECT id, user_id, credential_hash, algorithm, salt_params,
		       created_at, updated_at, rotated_at, expires_at
		FROM shadow_credentials WHERE user_id = $1`, userID).
		Scan(&c.ID, &c.UserID, &c.CredentialHash, &c.Algorithm, &c.SaltParams,
			&c.CreatedAt, &c.UpdatedAt, &c.RotatedAt, &c.ExpiresAt)
	if err != nil {
		return c, fmt.Errorf("get shadow credential (user=%s): %w", userID, err)
	}
	return c, nil
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

// HasShadowCredential checks whether a user has a shadow credential.
// The hash is deliberately excluded from the SELECT.
func HasShadowCredential(ctx context.Context, userID string) (models.ShadowCredentialStatus, error) {
	var s models.ShadowCredentialStatus
	var algorithm string
	var createdAt, updatedAt time.Time
	var rotatedAt, expiresAt *time.Time
	err := PG.QueryRow(ctx, `
		SELECT algorithm, created_at, updated_at, rotated_at, expires_at
		FROM shadow_credentials WHERE user_id = $1`, userID).
		Scan(&algorithm, &createdAt, &updatedAt, &rotatedAt, &expiresAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.ShadowCredentialStatus{HasCredential: false}, nil
		}
		return s, fmt.Errorf("check shadow credential (user=%s): %w", userID, err)
	}
	return models.ShadowCredentialStatus{
		HasCredential: true,
		Algorithm:     algorithm,
		CreatedAt:     &createdAt,
		UpdatedAt:     &updatedAt,
		RotatedAt:     rotatedAt,
		ExpiresAt:     expiresAt,
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
