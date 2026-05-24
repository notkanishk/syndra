package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"mkauth/internal/models"
)

// -------------------------------------------------------------
// PROVISIONING INTENTS
// -------------------------------------------------------------

// InsertProvisioningIntent records a new provisioning intent using the idempotency key.
// Returns (id, true, nil) on insert, ("", false, nil) if the key already exists.
func InsertProvisioningIntent(ctx context.Context, targetUID, action, lldapGroup,
	sourceProject, sourceRole, webhookEventID, idempotencyKey string) (string, bool, error) {
	query := `
		INSERT INTO provisioning_intents (target_uid, action, lldap_group,
			source_project, source_role, webhook_event_id, idempotency_key)
		VALUES ($1, $2, $3, $4, $5, NULLIF($6, '')::uuid, $7)
		ON CONFLICT (idempotency_key) DO NOTHING
		RETURNING id`

	var id string
	err := PG.QueryRow(ctx, query, targetUID, action, lldapGroup,
		sourceProject, sourceRole, webhookEventID, idempotencyKey).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("insert provisioning intent (key=%s): %w", idempotencyKey, err)
	}
	return id, true, nil
}

// ClaimPendingIntents atomically selects up to `limit` pending intents and transitions
// them to 'acknowledged' in a single operation. Uses FOR UPDATE SKIP LOCKED to prevent
// concurrent workers from claiming the same intents.
func ClaimPendingIntents(ctx context.Context, limit int) ([]models.ProvisioningIntent, error) {
	if limit <= 0 {
		limit = 50
	}
	query := `
		WITH claimed AS (
			SELECT id FROM provisioning_intents
			WHERE status = 'pending'
			ORDER BY created_at ASC
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		UPDATE provisioning_intents pi
		SET status = 'acknowledged', acknowledged_at = NOW()
		FROM claimed
		WHERE pi.id = claimed.id
		RETURNING pi.id, pi.target_uid, pi.action, pi.lldap_group, pi.source_project,
		          pi.source_role, COALESCE(pi.webhook_event_id::text, ''), pi.idempotency_key,
		          pi.status, COALESCE(pi.error_message, ''), pi.created_at,
		          pi.acknowledged_at, pi.completed_at`

	rows, err := PG.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("claim pending intents: %w", err)
	}
	defer rows.Close()

	var intents []models.ProvisioningIntent
	for rows.Next() {
		var i models.ProvisioningIntent
		if err := rows.Scan(&i.ID, &i.TargetUID, &i.Action, &i.LLDAPGroup,
			&i.SourceProject, &i.SourceRole, &i.WebhookEventID, &i.IdempotencyKey,
			&i.Status, &i.ErrorMessage, &i.CreatedAt, &i.AcknowledgedAt, &i.CompletedAt); err != nil {
			return nil, err
		}
		intents = append(intents, i)
	}
	return intents, nil
}

// CompleteIntent transitions an intent from 'acknowledged' to 'completed'.
func CompleteIntent(ctx context.Context, intentID string) error {
	query := `
		UPDATE provisioning_intents
		SET status = 'completed', completed_at = NOW()
		WHERE id = $1 AND status = 'acknowledged'`

	tag, err := PG.Exec(ctx, query, intentID)
	if err != nil {
		return fmt.Errorf("complete intent: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("intent not found or not in acknowledged status")
	}
	return nil
}

// FailIntent transitions an intent to 'failed' with an error message.
func FailIntent(ctx context.Context, intentID, errMsg string) error {
	query := `
		UPDATE provisioning_intents
		SET status = 'failed', error_message = $2, completed_at = NOW()
		WHERE id = $1 AND status IN ('pending', 'acknowledged')`

	tag, err := PG.Exec(ctx, query, intentID, errMsg)
	if err != nil {
		return fmt.Errorf("fail intent: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("intent not found or already terminal")
	}
	return nil
}

// GetProvisioningIntents returns provisioning intents ordered by creation time DESC.
// If statusFilter is non-empty, only intents with that status are returned.
func GetProvisioningIntents(ctx context.Context, statusFilter string) ([]models.ProvisioningIntent, error) {
	var query string
	var args []any

	if statusFilter != "" {
		query = `
			SELECT id, target_uid, action, lldap_group, source_project, source_role,
			       COALESCE(webhook_event_id::text, ''), idempotency_key, status,
			       COALESCE(error_message, ''), created_at, acknowledged_at, completed_at
			FROM provisioning_intents
			WHERE status = $1
			ORDER BY created_at DESC`
		args = []any{statusFilter}
	} else {
		query = `
			SELECT id, target_uid, action, lldap_group, source_project, source_role,
			       COALESCE(webhook_event_id::text, ''), idempotency_key, status,
			       COALESCE(error_message, ''), created_at, acknowledged_at, completed_at
			FROM provisioning_intents
			ORDER BY created_at DESC`
	}

	rows, err := PG.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query provisioning intents: %w", err)
	}
	defer rows.Close()

	var intents []models.ProvisioningIntent
	for rows.Next() {
		var i models.ProvisioningIntent
		if err := rows.Scan(&i.ID, &i.TargetUID, &i.Action, &i.LLDAPGroup,
			&i.SourceProject, &i.SourceRole, &i.WebhookEventID, &i.IdempotencyKey,
			&i.Status, &i.ErrorMessage, &i.CreatedAt, &i.AcknowledgedAt, &i.CompletedAt); err != nil {
			return nil, err
		}
		intents = append(intents, i)
	}
	return intents, nil
}
