package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// --- Webhook Events ---

// WebhookEvent is the DB representation of a webhook_events row.
type WebhookEvent struct {
	ID             string     `json:"id"`
	EventType      string     `json:"event_type"`
	UserID         string     `json:"user_id"`
	SourceProject  string     `json:"source_project"`
	RoleKey        string     `json:"role_key,omitempty"`
	IdempotencyKey string     `json:"idempotency_key"`
	Status         string     `json:"status"`
	ErrorMessage   string     `json:"error_message,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	ProcessedAt    *time.Time `json:"processed_at,omitempty"`
}

// InsertWebhookEvent records a received webhook event using the idempotency key.
// Returns (id, true, nil) on insert, ("", false, nil) if the key already exists.
func InsertWebhookEvent(ctx context.Context, eventType, userID, sourceProject, roleKey, idempotencyKey string) (string, bool, error) {
	query := `
		INSERT INTO webhook_events (event_type, user_id, source_project, role_key, idempotency_key)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (idempotency_key) DO NOTHING
		RETURNING id`

	var id string
	err := PG.QueryRow(ctx, query, eventType, userID, sourceProject, roleKey, idempotencyKey).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("insert webhook event (key=%s): %w", idempotencyKey, err)
	}
	return id, true, nil
}

// CompleteWebhookEvent marks an event as processed.
func CompleteWebhookEvent(ctx context.Context, eventID string) error {
	query := `
		UPDATE webhook_events
		SET status = 'processed', processed_at = NOW()
		WHERE id = $1`
	_, err := PG.Exec(ctx, query, eventID)
	if err != nil {
		return fmt.Errorf("complete webhook event: %w", err)
	}
	return nil
}

// FailWebhookEvent marks an event as failed with an error message.
func FailWebhookEvent(ctx context.Context, eventID, errMsg string) error {
	query := `
		UPDATE webhook_events
		SET status = 'failed', error_message = $2, processed_at = NOW()
		WHERE id = $1`
	_, err := PG.Exec(ctx, query, eventID, errMsg)
	if err != nil {
		return fmt.Errorf("fail webhook event: %w", err)
	}
	return nil
}

// WebhookStatusDroppedEnrichmentIncomplete is the webhook_events.status
// value the storm-prevention path writes when a Zitadel-shape grant event
// cannot be enriched. Defined as a constant so the helper, operator-facing
// filter callers, and the migration regression test all share one source
// of truth.
const WebhookStatusDroppedEnrichmentIncomplete = "dropped_enrichment_incomplete"

// DropWebhookEventEnrichmentIncomplete records a Zitadel-shape grant event
// whose enrichment could not resolve source_project or role_keys, so the
// handler issued a storm-prevention 200-ack instead of dispatching. The row
// makes the silent drop observable via
// GET /api/v1/webhook/events?status=dropped_enrichment_incomplete
// (audit refs C11, D8). Idempotent: duplicate posts of the same unresolvable
// aggregate are deduplicated, not double-counted.
//
// source_project is written empty because the enrichment-incomplete branch
// is exactly the case where it could not be resolved. Migration 000013
// relaxes the original non-empty check for this status only — the constraint
// still rejects empty source_project on every other status.
func DropWebhookEventEnrichmentIncomplete(ctx context.Context, eventType, userID, grantID, idempotencyKey string) error {
	_, err := PG.Exec(ctx, `
		INSERT INTO webhook_events (event_type, user_id, source_project, role_key, idempotency_key, status, processed_at)
		VALUES ($1, $2, '', $3, $4, $5, NOW())
		ON CONFLICT (idempotency_key) DO NOTHING
	`, eventType, userID, grantID, idempotencyKey, WebhookStatusDroppedEnrichmentIncomplete)
	if err != nil {
		return fmt.Errorf("drop webhook event enrichment incomplete: %w", err)
	}
	return nil
}

// GetWebhookEvents returns webhook events ordered by creation time.
// If statusFilter is non-empty, only events with that status are returned.
func GetWebhookEvents(ctx context.Context, statusFilter string) ([]WebhookEvent, error) {
	var query string
	var args []any

	if statusFilter != "" {
		query = `
			SELECT id, event_type, user_id, source_project, COALESCE(role_key, ''),
			       idempotency_key, status, COALESCE(error_message, ''),
			       created_at, processed_at
			FROM webhook_events
			WHERE status = $1
			ORDER BY created_at DESC`
		args = []any{statusFilter}
	} else {
		query = `
			SELECT id, event_type, user_id, source_project, COALESCE(role_key, ''),
			       idempotency_key, status, COALESCE(error_message, ''),
			       created_at, processed_at
			FROM webhook_events
			ORDER BY created_at DESC`
	}

	rows, err := PG.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query webhook events: %w", err)
	}
	defer rows.Close()

	var events []WebhookEvent
	for rows.Next() {
		var e WebhookEvent
		if err := rows.Scan(&e.ID, &e.EventType, &e.UserID, &e.SourceProject, &e.RoleKey,
			&e.IdempotencyKey, &e.Status, &e.ErrorMessage, &e.CreatedAt, &e.ProcessedAt); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, nil
}

// -------------------------------------------------------------
// ZITADEL GRANTS INDEX
// -------------------------------------------------------------

// ErrGrantIndexNotFound signals that the requested grant aggregate has no
// row in zitadel_grants_index — typically because no `user.grant.added`
// event has been seen for it yet. Callers MUST treat this as a cache miss
// (fall back to Zitadel API), NOT as a hard error.
var ErrGrantIndexNotFound = errors.New("grant not found in local index")

// ZitadelGrantIndex is the local cache of Zitadel user_grant aggregates,
// keyed by grant aggregate ID. Populated from grant.added events; used to
// enrich grant.changed (no projectId in payload) and grant.removed (no
// roleKeys in payload) before handler validation runs.
type ZitadelGrantIndex struct {
	GrantID   string    `json:"grant_id"`
	UserID    string    `json:"user_id"`
	ProjectID string    `json:"project_id"`
	RoleKeys  []string  `json:"role_keys"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// UpsertGrantIndex inserts or updates the cache row for a Zitadel user_grant
// aggregate. Called from the grant.added (and grant.changed) processor; the
// row lets later grant.changed/removed events fill projectId/roleKeys
// fields Zitadel omits from those payloads.
func UpsertGrantIndex(ctx context.Context, grantID, userID, projectID string, roleKeys []string) error {
	if grantID == "" || userID == "" || projectID == "" {
		return fmt.Errorf("UpsertGrantIndex: grant_id, user_id, project_id are required")
	}
	if roleKeys == nil {
		roleKeys = []string{}
	}
	const query = `
		INSERT INTO zitadel_grants_index (grant_id, user_id, project_id, role_keys)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (grant_id) DO UPDATE SET
			user_id    = EXCLUDED.user_id,
			project_id = EXCLUDED.project_id,
			role_keys  = EXCLUDED.role_keys,
			updated_at = NOW()`
	if _, err := PG.Exec(ctx, query, grantID, userID, projectID, roleKeys); err != nil {
		return fmt.Errorf("upsert grant index (%s): %w", grantID, err)
	}
	return nil
}

// GetGrantIndex fetches the cached row by grant aggregate ID. Returns
// ErrGrantIndexNotFound when the grant has never been seen.
func GetGrantIndex(ctx context.Context, grantID string) (ZitadelGrantIndex, error) {
	const query = `
		SELECT grant_id, user_id, project_id, role_keys, created_at, updated_at
		FROM zitadel_grants_index
		WHERE grant_id = $1`
	var row ZitadelGrantIndex
	err := PG.QueryRow(ctx, query, grantID).Scan(
		&row.GrantID, &row.UserID, &row.ProjectID, &row.RoleKeys, &row.CreatedAt, &row.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ZitadelGrantIndex{}, ErrGrantIndexNotFound
		}
		return ZitadelGrantIndex{}, fmt.Errorf("get grant index (%s): %w", grantID, err)
	}
	return row, nil
}

// DeleteGrantIndex removes the cached row. Called from the grant.removed
// processor after the downstream effects (revoke, cache invalidation) have
// run; failure is non-fatal — the next reconciliation will clean it up.
func DeleteGrantIndex(ctx context.Context, grantID string) error {
	const query = `DELETE FROM zitadel_grants_index WHERE grant_id = $1`
	if _, err := PG.Exec(ctx, query, grantID); err != nil {
		return fmt.Errorf("delete grant index (%s): %w", grantID, err)
	}
	return nil
}
