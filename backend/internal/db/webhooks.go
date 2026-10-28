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
// Returns (id, true, nil) when this delivery should be processed: a new row, or
// a redelivery of one that FAILED. Zitadel redelivers precisely because we
// answered 5xx, so treating a failed row as "already processed" turned the
// retry it asked for into a dropped event. So is a row left `received` for over
// ten minutes: a crash mid-processing leaves it there for good, and a live
// request finishes well inside that. A retake restarts the clock (created_at),
// so a second redelivery meets a row that is in progress again. ("", false, nil)
// for anything else (processed, or received recently): done or in progress.
func InsertWebhookEvent(ctx context.Context, eventType, userID, sourceProject, roleKey, idempotencyKey string) (string, bool, error) {
	query := `
		INSERT INTO webhook_events (event_type, user_id, source_project, role_key, idempotency_key)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (idempotency_key) DO UPDATE
		   SET status = 'received', error_message = NULL, processed_at = NULL, created_at = NOW()
		 WHERE webhook_events.status = 'failed'
		    OR (webhook_events.status = 'received' AND webhook_events.created_at < NOW() - INTERVAL '10 minutes')
		RETURNING id`

	var id string
	err := querier(ctx).QueryRow(ctx, query, eventType, userID, sourceProject, roleKey, idempotencyKey).Scan(&id)
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
	_, err := querier(ctx).Exec(ctx, query, eventID)
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
	_, err := querier(ctx).Exec(ctx, query, eventID, errMsg)
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
	_, err := querier(ctx).Exec(ctx, `
		INSERT INTO webhook_events (event_type, user_id, source_project, role_key, idempotency_key, status, processed_at)
		VALUES ($1, $2, '', $3, $4, $5, NOW())
		ON CONFLICT (idempotency_key) DO NOTHING
	`, eventType, userID, grantID, idempotencyKey, WebhookStatusDroppedEnrichmentIncomplete)
	if err != nil {
		return fmt.Errorf("drop webhook event enrichment incomplete: %w", err)
	}
	return nil
}

// logListLimit bounds the operations log reads. Both pages poll every few
// seconds, so an unbounded list is the whole table on every tick.
const logListLimit = 200

// WebhookEventRetentionDays is how long webhook_events rows are kept. The log
// is for operators watching recent deliveries; it is not the audit trail.
const WebhookEventRetentionDays = 90

// PruneWebhookEvents deletes events older than retentionDays and says how many.
func PruneWebhookEvents(ctx context.Context, retentionDays int) (int64, error) {
	tag, err := querier(ctx).Exec(ctx,
		`DELETE FROM webhook_events WHERE created_at < NOW() - make_interval(days => $1)`, retentionDays)
	if err != nil {
		return 0, fmt.Errorf("prune webhook events: %w", err)
	}
	return tag.RowsAffected(), nil
}

// GetWebhookEvents returns the newest logListLimit webhook events, newest first.
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
			ORDER BY created_at DESC
			LIMIT $2`
		args = []any{statusFilter, logListLimit}
	} else {
		query = `
			SELECT id, event_type, user_id, source_project, COALESCE(role_key, ''),
			       idempotency_key, status, COALESCE(error_message, ''),
			       created_at, processed_at
			FROM webhook_events
			ORDER BY created_at DESC
			LIMIT $1`
		args = []any{logListLimit}
	}

	rows, err := querier(ctx).Query(ctx, query, args...)
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
	if err := rows.Err(); err != nil {
		return nil, err
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

// GetGrantIndex fetches the cached row by grant aggregate ID. Returns
// ErrGrantIndexNotFound when the grant has never been seen.
func GetGrantIndex(ctx context.Context, grantID string) (ZitadelGrantIndex, error) {
	const query = `
		SELECT grant_id, user_id, project_id, role_keys, created_at, updated_at
		FROM zitadel_grants_index
		WHERE grant_id = $1`
	var row ZitadelGrantIndex
	err := querier(ctx).QueryRow(ctx, query, grantID).Scan(
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

// GetGrantIndexByUserProject fetches the cached grant-index row for a (user, project) pair —
// used by the cascade revoke path (Task 21) to resolve the Zitadel grant aggregate id it needs
// to call RemoveUserGrant, since a revoke computed from (user, project, role) triples never has
// a grant id handed to it the way drift/discovery revokes do (they already know it from the
// triggering event/URL param). Returns ErrGrantIndexNotFound on a cache miss — same tolerance as
// GetGrantIndex; the caller degrades to an empty ZitadelGrantID, and the drain's own 4xx handling
// fails just that row without halting the batch.
// ponytail: LIMIT 1 assumes at most one grant aggregate per (user, project), true for how Syndra
// and Zitadel model grants today; revisit if a user can hold two grants on the same project.
func GetGrantIndexByUserProject(ctx context.Context, userID, projectID string) (ZitadelGrantIndex, error) {
	const query = `
		SELECT grant_id, user_id, project_id, role_keys, created_at, updated_at
		FROM zitadel_grants_index
		WHERE user_id = $1 AND project_id = $2
		LIMIT 1`
	var row ZitadelGrantIndex
	err := querier(ctx).QueryRow(ctx, query, userID, projectID).Scan(
		&row.GrantID, &row.UserID, &row.ProjectID, &row.RoleKeys, &row.CreatedAt, &row.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ZitadelGrantIndex{}, ErrGrantIndexNotFound
		}
		return ZitadelGrantIndex{}, fmt.Errorf("get grant index (%s/%s): %w", userID, projectID, err)
	}
	return row, nil
}

// GrantIndexHasRole reports whether the webhook-maintained grant index already
// records the given (user, project, role) tuple. Used by the propagation drain
// as a zero-API-call already-exists pre-flight; a false negative is harmless
// because Zitadel's 409 AlreadyExists is the real idempotency net.
func GrantIndexHasRole(ctx context.Context, userID, projectID, role string) (bool, error) {
	const query = `
		SELECT EXISTS(
			SELECT 1 FROM zitadel_grants_index
			WHERE user_id = $1 AND project_id = $2 AND $3 = ANY(role_keys)
		)`
	var exists bool
	if err := querier(ctx).QueryRow(ctx, query, userID, projectID, role).Scan(&exists); err != nil {
		return false, fmt.Errorf("grant index has role (%s/%s/%s): %w", userID, projectID, role, err)
	}
	return exists, nil
}
