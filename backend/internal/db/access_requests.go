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
//
// Waiting says the role is given but not yet sent: a queued add, from a grant
// or a bundle. It still counts as held, so nobody asks for or gives it twice;
// the words just say it may not work yet. A role whose removal is queued is
// not held, even while Zitadel still shows it: otherwise giving it back
// before the removal was sent was refused as "already has this".
func RequestHolding(ctx context.Context, userID, projectID, roleKey string) (models.RequestHolding, error) {
	const q = `
		WITH queued AS (
			SELECT op_type FROM propagation_outbox
			 WHERE target = 'zitadel' AND status IN ('pending', 'in_flight')
			   AND user_id = $1 AND project_id = $2 AND $3 = ANY(role_keys)
		)
		SELECT
			(SELECT expires_at FROM direct_role_grants
			  WHERE user_id = $1 AND zitadel_project_id = $2 AND zitadel_role_key = $3),
			EXISTS (SELECT 1 FROM direct_role_grants
			         WHERE user_id = $1 AND zitadel_project_id = $2 AND zitadel_role_key = $3),
			EXISTS (SELECT 1 FROM zitadel_grants_index
			         WHERE user_id = $1 AND project_id = $2 AND $3 = ANY(role_keys))
			  AND NOT EXISTS (SELECT 1 FROM queued WHERE op_type = 'revoke'),
			EXISTS (SELECT 1 FROM queued WHERE op_type IN ('add', 'replace'))`
	var until *time.Time
	var granted, observed, waiting bool
	if err := querier(ctx).QueryRow(ctx, q, userID, projectID, roleKey).Scan(&until, &granted, &observed, &waiting); err != nil {
		return models.RequestHolding{}, fmt.Errorf("read what %s holds of %s/%s: %w", userID, projectID, roleKey, err)
	}
	switch {
	case granted && until != nil:
		return models.RequestHolding{Kind: models.HoldingUntil, Until: until, Waiting: waiting}, nil
	case granted || observed || waiting:
		return models.RequestHolding{Kind: models.HoldingForGood, Waiting: waiting}, nil
	default:
		return models.RequestHolding{Kind: models.HoldingNone}, nil
	}
}

// closeGivenRequests closes the waiting requests a grant answers (found
// 15 Oct). Given directly or through a bundle instead of by approving, the
// request stayed open: the queue still asked someone to decide it, the person
// couldn't ask for that role again once it ended, and declining it would have
// told them "Not approved" for something they had. It is closed as
// `fulfilled`, in the grant's own transaction, by whoever gave the role.
//
// An approval resolves its own request first, so it never closes itself here.
func closeGivenRequests(ctx context.Context, tx pgx.Tx, p EnqueueParams) error {
	if p.OpType == "revoke" || len(p.RoleKeys) == 0 {
		return nil
	}
	by := p.GrantedBy
	if by == "" {
		by = "system"
	}
	const q = `
		WITH closed AS (
			UPDATE access_requests
			   SET status = 'fulfilled', reviewer_user_id = $1, review_note = $5, resolved_at = now()
			 WHERE status = 'pending' AND requester_user_id = $2
			   AND zitadel_project_id = $3 AND zitadel_role_key = ANY($4::text[])
			RETURNING id, requester_user_id
		)
		INSERT INTO audit_logs (actor_zitadel_user_id, target_zitadel_user_id, action, resource_id)
		SELECT $1, requester_user_id, 'access_request.closed_as_given', id::text FROM closed`
	if _, err := tx.Exec(ctx, q, by, p.UserID, p.ProjectID, p.RoleKeys, givenNote(p.Source)); err != nil {
		return fmt.Errorf("close requests the grant answers: %w", err)
	}
	return nil
}

// givenNote says how the role came, for both the person and the queue.
func givenNote(source string) string {
	switch source {
	case "", "direct":
		return "Given directly, so there was nothing left to decide."
	case "bundle":
		return "Given through a bundle, so there was nothing left to decide."
	case "rule":
		return "Given by a rule, so there was nothing left to decide."
	default:
		return "Given another way, so there was nothing left to decide."
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
