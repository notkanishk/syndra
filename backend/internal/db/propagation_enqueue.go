package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// EnqueueParams is the input to the transactional enqueue. RoleKeys is plural so
// both the single-role /users/{id}/grants caller (wrapping its one role) and the
// multi-role /zitadel/* caller flow through one path. Source defaults to
// "direct" when empty.
type EnqueueParams struct {
	UserID         string
	ProjectID      string
	RoleKeys       []string
	GrantedBy      string
	Reason         string
	ExpiresAt      *time.Time
	Source         string // defaults to "direct" when empty
	SourceRef      string
	OpType         string // add | revoke | replace
	ZitadelGrantID string
	PayloadJSON    string
	// NoPropagation records the ledger and audit rows without an outbox row,
	// for the one case where Syndra owes Zitadel nothing: adopting drift.
	// external_backfill means Zitadel is already authoritative, so there is no
	// mutation to project — and an outbox row is not a passive receipt, it is a
	// live instruction. An `add` row that outlives its adoption re-creates the
	// grant on the next drain, which is precisely what happens when an operator
	// adopts a role and then removes it upstream by hand.
	NoPropagation bool
}

// EnqueueResult is the operator-facing handle returned to the HTTP caller.
type EnqueueResult struct {
	OutboxID       string `json:"outbox_id"`
	IdempotencyKey string `json:"idempotency_key"`
	Status         string `json:"status"`
}

// EnqueueDirectGrantPropagation writes the intent ledger rows (one
// direct_role_grants row per role on add/replace), one audit row, and one outbox
// row in a single transaction, then returns the outbox handle. The Zitadel call
// happens later, during the operator-triggered drain. This is the doctrine seam:
// the ledger is durable BEFORE any Zitadel mutation (B4/D3 single mutation
// authority).
func EnqueueDirectGrantPropagation(ctx context.Context, p EnqueueParams) (EnqueueResult, error) {
	key, err := newOutboxIdempotencyKey()
	if err != nil {
		return EnqueueResult{}, err
	}
	return enqueueTx(ctx, p, key)
}

// enqueueTx is the tx body with the idempotency key injected, so tests (and the
// rollback path) can pin a fixed key. All writes share one transaction: a
// failure on any insert rolls back the ledger, audit, and outbox together.
func enqueueTx(ctx context.Context, p EnqueueParams, key string) (EnqueueResult, error) {
	tx, err := PG.Begin(ctx)
	if err != nil {
		return EnqueueResult{}, fmt.Errorf("begin enqueue tx: %w", err)
	}
	defer tx.Rollback(ctx) // no-op after a successful Commit

	outboxID, err := enqueueWrites(ctx, tx, p, key)
	if err != nil {
		return EnqueueResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return EnqueueResult{}, fmt.Errorf("commit enqueue tx: %w", err)
	}
	return EnqueueResult{OutboxID: outboxID, IdempotencyKey: key, Status: "pending"}, nil
}

// enqueueWrites performs the ledger upsert (add/replace) + audit + outbox insert
// on an EXISTING transaction, returning the new outbox id. Extracted so callers
// that must bundle extra writes into the same atomic unit — e.g.
// ApproveRequestAndEnqueue, which also resolves the access request — can share
// one transaction with the enqueue rather than splitting it across two.
func enqueueWrites(ctx context.Context, tx pgx.Tx, p EnqueueParams, key string) (string, error) {
	source := p.Source
	if source == "" {
		source = "direct"
	}

	// The ledger only records grants for add/replace. A revoke removes access;
	// the direct_role_grants row (if any) is cleaned up on the applied drain or
	// by expiry — we never write a grant row for a revoke intent.
	var firstGrantID string
	if p.OpType != "revoke" {
		const upsertGrant = `
			INSERT INTO direct_role_grants
				(user_id, zitadel_project_id, zitadel_role_key, granted_by, reason, expires_at, source, source_ref)
			VALUES ($1,$2,$3,$4,$5,$6,$7,NULLIF($8,''))
			ON CONFLICT (user_id, zitadel_project_id, zitadel_role_key)
			DO UPDATE SET granted_by=EXCLUDED.granted_by, reason=EXCLUDED.reason,
			              expires_at=EXCLUDED.expires_at, source=EXCLUDED.source,
			              source_ref=EXCLUDED.source_ref, updated_at=CURRENT_TIMESTAMP
			RETURNING id`
		for i, role := range p.RoleKeys {
			var id string
			if err := tx.QueryRow(ctx, upsertGrant, p.UserID, p.ProjectID, role,
				p.GrantedBy, p.Reason, p.ExpiresAt, source, p.SourceRef).Scan(&id); err != nil {
				return "", fmt.Errorf("upsert direct grant (%s): %w", role, err)
			}
			if i == 0 {
				firstGrantID = id
			}
		}
	}

	const insertAudit = `INSERT INTO audit_logs
		(actor_zitadel_user_id, target_zitadel_user_id, action, resource_id) VALUES ($1,$2,$3,$4)`
	action := "direct_grant." + opTypeAuditVerb(p.OpType)
	if _, err := tx.Exec(ctx, insertAudit, p.GrantedBy, p.UserID, action, firstGrantID); err != nil {
		return "", fmt.Errorf("insert audit: %w", err)
	}

	// No outbox row when nothing is owed upstream. The ledger row is the durable
	// intent and the audit row is the durable trace; the outbox is the work
	// queue, and adopting drift creates no work. See EnqueueParams.NoPropagation.
	if p.NoPropagation {
		return "", nil
	}

	const insertOutbox = `
		INSERT INTO propagation_outbox
			(op_type, user_id, project_id, role_keys, zitadel_grant_id, payload_json, idempotency_key, initiated_by, source, source_ref)
		VALUES ($1,$2,$3,$4,NULLIF($5,''),$6,$7,$8,$9,NULLIF($10,''))
		RETURNING id`
	var outboxID string
	if err := tx.QueryRow(ctx, insertOutbox, p.OpType, p.UserID, p.ProjectID, p.RoleKeys,
		p.ZitadelGrantID, p.PayloadJSON, key, p.GrantedBy, source, p.SourceRef).Scan(&outboxID); err != nil {
		return "", fmt.Errorf("insert outbox: %w", err)
	}
	return outboxID, nil
}

// opTypeAuditVerb maps the outbox op_type to the audit action suffix.
func opTypeAuditVerb(opType string) string {
	switch opType {
	case "revoke":
		return "revoked"
	case "replace":
		return "replaced"
	default:
		return "upserted"
	}
}
