package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"syndra/internal/extcall"
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
	// EndRule is what a write does to the end of a grant that already exists.
	//
	// Every write used to replace it, so approving a 64-day request from
	// someone holding the role for good cut them to 64 days, and a bundle, the
	// Zitadel users page or a drift adoption, none of which says anything about
	// an end, turned a grant that ended into one that never does. Giving or
	// approving a role someone already has is now refused before it gets here
	// (owner, 15 Oct: one request per role, and only for a role they lack), so:
	//
	//   - EndKept (the default): an existing grant keeps its end.
	//   - EndSet: the end is the one given. Only an extension, the one act
	//     whose whole point is a grant's end, and only to a later one (the
	//     handler refuses anything else).
	//
	// A new grant always takes ExpiresAt as given.
	EndRule EndRule
}

// EndRule decides what a write does to an existing grant's end.
type EndRule string

const (
	EndKept EndRule = ""
	EndSet  EndRule = "set"
)

// EnqueueResult is the operator-facing handle returned to the HTTP caller.
type EnqueueResult struct {
	OutboxID       string `json:"outbox_id"`
	IdempotencyKey string `json:"idempotency_key"`
	Status         string `json:"status"`
	// WaitReason says why the change isn't in Zitadel yet, when it isn't:
	// role_waits, manual, or halted:<reason> (services.Deliver).
	WaitReason string `json:"wait_reason,omitempty"`
	// Retry is the next try and which try, when Zitadel didn't take it.
	Retry *extcall.RetryState `json:"retry,omitempty"`
	// RuleOutboxIDs are the rows the mapping rules added to this write: what a
	// chain of rules derives from the role given, or takes back with the role
	// removed. Written in the same transaction, delivered with OutboxID.
	RuleOutboxIDs []string `json:"rule_outbox_ids,omitempty"`
	// RuleWaiting counts the rule rows held for the operator because their
	// role waits: said apart from WaitReason, which is the write's own.
	RuleWaiting int `json:"rule_waiting,omitempty"`
}

// OutboxIDs is every row this write queued: its own, then the rules'.
func (r EnqueueResult) OutboxIDs() []string {
	ids := make([]string, 0, 1+len(r.RuleOutboxIDs))
	if r.OutboxID != "" {
		ids = append(ids, r.OutboxID)
	}
	return append(ids, r.RuleOutboxIDs...)
}

// FollowRules carries a direct write through the mapping rules. Wired by
// services (services.followRules), which owns the one definition of what a
// person holds and the one closure (access.Closure) the token is compiled
// from.
//
// enqueueWrites hands it the write as a function. It reads the person's
// closure, runs the write, reads the closure again — all inside this
// transaction and under the access-mutation lock — and returns the writes the
// difference owes Zitadel beyond the write's own roles: source 'rule', no
// ledger row, the same outbox and drain every bundle and rule cascade uses.
//
// This is the chokepoint because every direct write passes it: a grant, a
// replace or revoke by grant id, an approved request, a bulk give or extend,
// an adopted drift. Rules used to fire only from Zitadel's webhook, which drops
// Syndra's own changes, so a role Syndra gave never reached its rule targets.
//
// Nil only in this package's own tests, where services is not linked: the write
// then goes alone. The binary always links services, whose init wires it.
var FollowRules func(ctx context.Context, p EnqueueParams, write func() error) ([]EnqueueParams, error)

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
	tx, owned, err := beginOrJoin(ctx)
	if err != nil {
		return EnqueueResult{}, fmt.Errorf("begin enqueue tx: %w", err)
	}
	if owned {
		defer tx.Rollback(ctx) // no-op after a successful Commit
	}

	outboxID, ruleIDs, err := enqueueWrites(ctx, tx, p, key)
	if err != nil {
		return EnqueueResult{}, err
	}

	if owned {
		if err := tx.Commit(ctx); err != nil {
			return EnqueueResult{}, fmt.Errorf("commit enqueue tx: %w", err)
		}
	}
	return EnqueueResult{OutboxID: outboxID, IdempotencyKey: key, Status: enqueuedStatus(outboxID), RuleOutboxIDs: ruleIDs}, nil
}

// enqueueWrites performs the ledger upsert (add/replace) + audit + outbox insert
// on an EXISTING transaction, returning the new outbox id. Extracted so callers
// that must bundle extra writes into the same atomic unit — e.g.
// ApproveRequestAndEnqueue, which also resolves the access request — can share
// one transaction with the enqueue rather than splitting it across two.
//
// It returns the write's own outbox id and the ids of the rows the mapping
// rules owe on top of it (FollowRules), all in tx.
func enqueueWrites(ctx context.Context, tx pgx.Tx, p EnqueueParams, key string) (string, []string, error) {
	// The second of the two enqueue chokepoints. See enqueueCascadeRows: a
	// writer that computed its delta under this lock is only protected while
	// every other writer has to wait for it.
	if err := LockAccessMutationTx(ctx, tx); err != nil {
		return "", nil, err
	}
	if FollowRules == nil {
		id, err := enqueueOwn(ctx, tx, p, key)
		return id, nil, err
	}
	var own string
	derived, err := FollowRules(context.WithValue(ctx, txKey, tx), p, func() error {
		var err error
		own, err = enqueueOwn(ctx, tx, p, key)
		return err
	})
	if err != nil {
		return "", nil, err
	}
	if len(derived) == 0 {
		return own, nil, nil
	}
	ruleIDs, err := enqueueCascadeRows(ctx, tx, nil, derived)
	if err != nil {
		return "", nil, fmt.Errorf("enqueue rule writes: %w", err)
	}
	return own, ruleIDs, nil
}

// enqueueOwn is the write itself: ledger, audit and outbox for exactly the
// roles the caller named.
func enqueueOwn(ctx context.Context, tx pgx.Tx, p EnqueueParams, key string) (string, error) {
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
			              expires_at = CASE WHEN $9 = 'set' THEN EXCLUDED.expires_at
			                                ELSE direct_role_grants.expires_at END,
			              source=EXCLUDED.source,
			              source_ref=EXCLUDED.source_ref, updated_at=CURRENT_TIMESTAMP
			RETURNING id`
		for i, role := range p.RoleKeys {
			var id string
			if err := tx.QueryRow(ctx, upsertGrant, p.UserID, p.ProjectID, role,
				p.GrantedBy, p.Reason, p.ExpiresAt, source, p.SourceRef, string(p.EndRule)).Scan(&id); err != nil {
				return "", fmt.Errorf("upsert direct grant (%s): %w", role, err)
			}
			if i == 0 {
				firstGrantID = id
			}
		}
	}

	if err := closeGivenRequests(ctx, tx, p); err != nil {
		return "", err
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

	// The same countermand the cascade path takes, for the same reason: a
	// direct grant revoked before it was ever sent should leave an empty queue,
	// not two rows an operator has to approve to arrive back where they were.
	owed, err := withdrawUndelivered(ctx, tx, "zitadel", p)
	if err != nil {
		return "", err
	}
	if !owed {
		return "", nil
	}

	// The same add already waiting for you to send delivers this one too. Extending
	// a grant before its first send queued a second, identical add: two rows
	// in Pending changes for one role. The ledger carries the new end; the
	// queued row is what sends it, so its id is the answer.
	if p.OpType == "add" {
		var queued string
		err := tx.QueryRow(ctx, `
			SELECT id FROM propagation_outbox
			 WHERE target = 'zitadel' AND status = 'pending' AND op_type = 'add'
			   AND user_id = $1 AND project_id = $2 AND role_keys = $3::text[]
			 ORDER BY created_at LIMIT 1`, p.UserID, p.ProjectID, p.RoleKeys).Scan(&queued)
		if err == nil {
			return queued, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return "", fmt.Errorf("find a queued add: %w", err)
		}
	}

	const insertOutbox = `
		INSERT INTO propagation_outbox
			(op_type, user_id, project_id, role_keys, zitadel_grant_id, payload_json, idempotency_key, initiated_by, source, source_ref, target)
		VALUES ($1,$2,$3,$4,NULLIF($5,''),$6,$7,$8,$9,NULLIF($10,''),'zitadel')
		RETURNING id`
	var outboxID string
	if err := tx.QueryRow(ctx, insertOutbox, p.OpType, p.UserID, p.ProjectID, p.RoleKeys,
		p.ZitadelGrantID, p.PayloadJSON, key, p.GrantedBy, source, p.SourceRef).Scan(&outboxID); err != nil {
		return "", fmt.Errorf("insert outbox: %w", err)
	}
	return outboxID, nil
}

// enqueuedStatus: no row queued means nothing is owed upstream (a queued
// removal was cancelled and the grant still stands, or nothing is sent at all),
// so nothing is waiting; "pending" there told the operator it was.
func enqueuedStatus(outboxID string) string {
	if outboxID == "" {
		return "applied"
	}
	return "pending"
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
