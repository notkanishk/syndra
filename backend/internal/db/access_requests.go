package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"syndra/internal/models"
)

// ErrRequestNotPending is returned when a resolve targets an access request that
// is no longer pending — already approved/rejected, or a lost concurrent
// approve/reject race. Handlers map it to 409 Conflict, distinct from a genuine
// DB error (500).
var ErrRequestNotPending = errors.New("access request is not pending")

// -------------------------------------------------------------
// ACCESS REQUESTS
// -------------------------------------------------------------

// ErrAlreadyAsked is a request for something the same person is already
// waiting on. The id returned beside it is the waiting one.
var ErrAlreadyAsked = errors.New("the same request is already waiting")

// CreateAccessRequest files a request, or returns the one already waiting for
// the same person and role with ErrAlreadyAsked.
//
// One waiting request per person and role is the index's
// (idx_access_requests_one_waiting), so a second click, a second tab and a
// retried POST all meet the same row rather than adding to the queue.
func CreateAccessRequest(ctx context.Context, requesterID, projectID, roleKey, justification string, durationDays *int) (string, error) {
	const insert = `
		INSERT INTO access_requests (
			requester_user_id, zitadel_project_id, zitadel_role_key, justification, duration_days
		)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (requester_user_id, zitadel_project_id, zitadel_role_key) WHERE status = 'pending'
		DO NOTHING
		RETURNING id`

	var id string
	err := querier(ctx).QueryRow(ctx, insert, requesterID, projectID, roleKey, justification, durationDays).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("failed to create access request: %w", err)
	}
	const waiting = `
		SELECT id FROM access_requests
		 WHERE requester_user_id = $1 AND zitadel_project_id = $2 AND zitadel_role_key = $3
		   AND status = 'pending'`
	if err := querier(ctx).QueryRow(ctx, waiting, requesterID, projectID, roleKey).Scan(&id); err != nil {
		return "", fmt.Errorf("find the waiting request: %w", err)
	}
	return id, ErrAlreadyAsked
}

// RequestHolding is what a person already has of a role, for a request.
//
//   - "for_good": they hold it with no end. Syndra granted it for good, or
//     Zitadel shows them holding it with no ending grant from Syndra (a bundle,
//     a rule, or given there). Asking again asks for nothing.
//   - "until": a grant from Syndra that ends at Until. Asking is asking to
//     keep it longer.
//   - "none": they don't hold it.
func RequestHolding(ctx context.Context, userID, projectID, roleKey string) (models.RequestHolding, error) {
	const q = `
		SELECT
			(SELECT expires_at FROM direct_role_grants
			  WHERE user_id = $1 AND zitadel_project_id = $2 AND zitadel_role_key = $3),
			EXISTS (SELECT 1 FROM direct_role_grants
			         WHERE user_id = $1 AND zitadel_project_id = $2 AND zitadel_role_key = $3),
			EXISTS (SELECT 1 FROM zitadel_grants_index
			         WHERE user_id = $1 AND project_id = $2 AND $3 = ANY(role_keys))`
	var until *time.Time
	var granted, observed bool
	if err := querier(ctx).QueryRow(ctx, q, userID, projectID, roleKey).Scan(&until, &granted, &observed); err != nil {
		return models.RequestHolding{}, fmt.Errorf("read what %s holds of %s/%s: %w", userID, projectID, roleKey, err)
	}
	switch {
	case granted && until != nil:
		return models.RequestHolding{Kind: models.HoldingUntil, Until: until}, nil
	case granted || observed:
		return models.RequestHolding{Kind: models.HoldingForGood}, nil
	default:
		return models.RequestHolding{Kind: models.HoldingNone}, nil
	}
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

	rows, err := querier(ctx).Query(ctx, query, args...)
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
	if err := querier(ctx).QueryRow(ctx, query, id).Scan(&req.ID, &req.RequesterID, &req.ProjectID, &req.RoleKey, &req.Justification, &req.DurationDays, &req.Status, &req.ReviewerID, &req.ReviewNote, &req.CreatedAt, &req.ResolvedAt); err != nil {
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

	tag, err := querier(ctx).Exec(ctx, query, id, status, reviewerID, reviewNote)
	if err != nil {
		return fmt.Errorf("failed to resolve access request: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrRequestNotPending
	}
	return nil
}

// WithdrawAccessRequest resolves a request to 'withdrawn', conditional on it still being pending
// AND on the caller being the person who filed it.
//
// Both conditions live in the WHERE clause rather than in a read-then-write. The pending guard
// closes the same race ResolveAccessRequest closes — an operator deciding at the moment the
// member takes it back — and the requester guard is here rather than only in the handler because
// this is the statement that must never withdraw somebody else's ask, whatever route reaches it.
//
// reviewer_user_id is left NULL: a withdrawal is a resolution without a decision, and the row
// already names who filed it.
func WithdrawAccessRequest(ctx context.Context, id, requesterID string) error {
	const query = `
		UPDATE access_requests
		SET status = 'withdrawn',
			resolved_at = CURRENT_TIMESTAMP
		WHERE id = $1 AND requester_user_id = $2 AND status = 'pending'`

	tag, err := querier(ctx).Exec(ctx, query, id, requesterID)
	if err != nil {
		return fmt.Errorf("failed to withdraw access request: %w", err)
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
	tx, owned, err := beginOrJoin(ctx)
	if err != nil {
		return EnqueueResult{}, fmt.Errorf("begin approve tx: %w", err)
	}
	if owned {
		defer tx.Rollback(ctx) // no-op after a successful Commit
	}

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

	if owned {
		if err := tx.Commit(ctx); err != nil {
			return EnqueueResult{}, fmt.Errorf("commit approve tx: %w", err)
		}
	}
	return EnqueueResult{OutboxID: outboxID, IdempotencyKey: key, Status: "pending"}, nil
}
