package db

import (
	"context"
	"errors"
	"fmt"

	"mkauth/internal/models"
)

// ErrRequestNotPending is returned when a resolve targets an access request that
// is no longer pending — already approved/rejected, or a lost concurrent
// approve/reject race. Handlers map it to 409 Conflict, distinct from a genuine
// DB error (500).
var ErrRequestNotPending = errors.New("access request is not pending")

// -------------------------------------------------------------
// ACCESS REQUESTS
// -------------------------------------------------------------

func CreateAccessRequest(ctx context.Context, requesterID, projectID, roleKey, justification string, durationDays *int) (string, error) {
	query := `
		INSERT INTO access_requests (
			requester_user_id, zitadel_project_id, zitadel_role_key, justification, duration_days
		)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id`

	var id string
	if err := PG.QueryRow(ctx, query, requesterID, projectID, roleKey, justification, durationDays).Scan(&id); err != nil {
		return "", fmt.Errorf("failed to create access request: %w", err)
	}
	return id, nil
}

func GetAccessRequests(ctx context.Context, status string) ([]models.AccessRequest, error) {
	query := `
		SELECT id, requester_user_id, zitadel_project_id, zitadel_role_key, justification, duration_days, status, COALESCE(reviewer_user_id, ''), COALESCE(review_note, ''), created_at, resolved_at
		FROM access_requests`
	args := []interface{}{}
	if status != "" {
		query += ` WHERE status = $1`
		args = append(args, status)
	}
	query += ` ORDER BY created_at DESC`

	rows, err := PG.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var requests []models.AccessRequest
	for rows.Next() {
		var req models.AccessRequest
		if err := rows.Scan(&req.ID, &req.RequesterID, &req.ProjectID, &req.RoleKey, &req.Justification, &req.DurationDays, &req.Status, &req.ReviewerID, &req.ReviewNote, &req.CreatedAt, &req.ResolvedAt); err != nil {
			return nil, err
		}
		requests = append(requests, req)
	}
	return requests, nil
}

func GetAccessRequestByID(ctx context.Context, id string) (models.AccessRequest, error) {
	query := `
		SELECT id, requester_user_id, zitadel_project_id, zitadel_role_key, justification, duration_days, status, COALESCE(reviewer_user_id, ''), COALESCE(review_note, ''), created_at, resolved_at
		FROM access_requests
		WHERE id = $1`

	var req models.AccessRequest
	if err := PG.QueryRow(ctx, query, id).Scan(&req.ID, &req.RequesterID, &req.ProjectID, &req.RoleKey, &req.Justification, &req.DurationDays, &req.Status, &req.ReviewerID, &req.ReviewNote, &req.CreatedAt, &req.ResolvedAt); err != nil {
		return req, fmt.Errorf("failed to fetch access request: %w", err)
	}
	return req, nil
}

// ResolveAccessRequest transitions a request to approved/rejected, conditional on
// it still being pending. The `status='pending'` guard closes the concurrent
// approve/reject race (two decisions landing on the same request): the second
// affects 0 rows and gets ErrRequestNotPending. Used for the reject path;
// approvals go through ApproveRequestAndEnqueue so the grant is atomic with the
// resolution.
func ResolveAccessRequest(ctx context.Context, id, status, reviewerID, reviewNote string) error {
	const query = `
		UPDATE access_requests
		SET status = $2,
			reviewer_user_id = $3,
			review_note = $4,
			resolved_at = CURRENT_TIMESTAMP
		WHERE id = $1 AND status = 'pending'`

	tag, err := PG.Exec(ctx, query, id, status, reviewerID, reviewNote)
	if err != nil {
		return fmt.Errorf("failed to resolve access request: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrRequestNotPending
	}
	return nil
}

// ApproveRequestAndEnqueue resolves an access request to 'approved' AND enqueues
// its direct grant (ledger + audit + outbox) in ONE transaction, conditional on
// the request still being pending. Either both happen or neither: a failed
// enqueue can no longer strand an approved-but-ungranted request, and the
// `status='pending'` guard closes the concurrent approve/reject race. Returns
// ErrRequestNotPending if the request was already resolved.
func ApproveRequestAndEnqueue(ctx context.Context, requestID, reviewer, reviewNote string, p EnqueueParams) (EnqueueResult, error) {
	key, err := newOutboxIdempotencyKey()
	if err != nil {
		return EnqueueResult{}, err
	}
	tx, err := PG.Begin(ctx)
	if err != nil {
		return EnqueueResult{}, fmt.Errorf("begin approve tx: %w", err)
	}
	defer tx.Rollback(ctx) // no-op after a successful Commit

	const resolveQ = `
		UPDATE access_requests
		SET status = 'approved', reviewer_user_id = $2, review_note = $3, resolved_at = CURRENT_TIMESTAMP
		WHERE id = $1 AND status = 'pending'`
	tag, err := tx.Exec(ctx, resolveQ, requestID, reviewer, reviewNote)
	if err != nil {
		return EnqueueResult{}, fmt.Errorf("resolve access request: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return EnqueueResult{}, ErrRequestNotPending
	}

	outboxID, err := enqueueWrites(ctx, tx, p, key)
	if err != nil {
		return EnqueueResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return EnqueueResult{}, fmt.Errorf("commit approve tx: %w", err)
	}
	return EnqueueResult{OutboxID: outboxID, IdempotencyKey: key, Status: "pending"}, nil
}
