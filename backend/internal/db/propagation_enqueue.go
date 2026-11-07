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
	// WaitReason is set by the planner on a write it decided waits for you
	// (role_waits, decision 11): the row is written waiting, with its reason,
	// in the same statement. Nothing after the commit decides it again, and
	// no drain can send it before the reason is on the row.
	WaitReason string
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

	ids, err := enqueueWrites(ctx, tx, nil, []EnqueueParams{p}, key)
	if err != nil {
		return EnqueueResult{}, err
	}
	outboxID, ruleIDs := ownAndRules(ids)

	if owned {
		if err := tx.Commit(ctx); err != nil {
			return EnqueueResult{}, fmt.Errorf("commit enqueue tx: %w", err)
		}
	}
	return EnqueueResult{OutboxID: outboxID, IdempotencyKey: key, Status: enqueuedStatus(outboxID), RuleOutboxIDs: ruleIDs}, nil
}

// enqueueWrites is the one outbox insert (architecture deepening, decision 8):
// every access change reaches propagation_outbox here, on an EXISTING
// transaction, so callers that must commit other writes with it (resolving a
// request, deleting a grant, assigning a bundle) share one atomic unit. It
// returns the ids of the rows it wrote, in order.
//
// A change comes in one of two shapes:
//
//   - A write a person named (key set; params is that one write): a grant, a
//     revoke by grant id, an approved request, an adopted drift. Its ledger
//     and audit rows are written here (enqueueLedger), its own outbox row
//     carries key and no cascade id, an identical add already waiting answers
//     it, and the mapping rules follow it (FollowRules, ADR-006). ids[0] is its
//     own row ("" when none is owed), the rest are the rules' (ownAndRules).
//   - A cascade (key empty): the rows one triggering event computed, with that
//     event's audit rows. Cascade intent lives in the bundle and rule tables,
//     so no ledger row is written.
//
// Every other row gets a key of its own and the change's one cascade id,
// which the audit rows carry too when Change history shows the writes
// (cascadeGroupVisible). Every row passes withdrawUndelivered first, the one
// place a queued change is cancelled and the cancel gate is asked
// (CancellationsNamed / CancellationsReviewed): a row that only cancelled one
// is not written. A row carries the operator's reason when the change's
// audit rows give one (CascadeAudit.Reason), and a write the planner decided
// waits for you is written waiting (EnqueueParams.WaitReason).
//
// The audit rows live HERE rather than in each caller (C6, ISC-44): the
// cascade id is minted, stamped on the audit rows and stamped on the outbox
// rows in one place, so a cascade added later cannot forget it.
func enqueueWrites(ctx context.Context, tx pgx.Tx, audits []CascadeAudit, params []EnqueueParams, key string) ([]string, error) {
	// Every access change passes here, which is what makes this the place to
	// take the subject lock: a caller that forgot it is still serialised
	// against one that took it. Taken here it does not protect THIS caller's
	// delta, computed before the call; only a caller that locks before its own
	// reads is safe. But it holds every writer back for as long as such a
	// caller holds it, which is what makes locking before the read worth
	// anything at all.
	if err := LockAccessMutationTx(ctx, tx); err != nil {
		return nil, err
	}

	// One id for the change: its rows are the writes ONE triggering event
	// produced, including the ones a chained rule contributed (the closure diff
	// is computed before this call, so a chain arrives as a single set).
	// Grouping by it is what lets Pending changes say "they confirm together
	// or not at all": a half-applied cascade is the thing that creates
	// unexplained access, and it has to be visible as one.
	cascadeID, err := newOutboxIdempotencyKey()
	if err != nil {
		return nil, err
	}
	// The operator's words, when the change came with some: on its audit row
	// and on every row it queues.
	var why string
	for _, a := range audits {
		if why == "" {
			why = a.Reason
		}
	}

	// row writes p's outbox row if one is still owed, and returns its id.
	// ownKey is set only for a named write's own row.
	row := func(p EnqueueParams, ownKey string) (string, error) {
		// Countermand before compensate. A write answering a change still in
		// this queue cancels it instead of queueing its opposite; see
		// withdrawUndelivered for why that is narrower than it sounds.
		owed, err := withdrawUndelivered(ctx, tx, "zitadel", p)
		if err != nil || !owed {
			return "", err
		}
		cascade := &cascadeID
		if ownKey != "" {
			cascade = nil
			// The same add already waiting for you to send delivers this one
			// too. Extending a grant before its first send queued a second,
			// identical add: two rows in Pending changes for one role. The
			// ledger carries the new end; the queued row is what sends it, so
			// its id is the answer.
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
		} else {
			// A revoke computed from a (user, project, role) triple never
			// arrives with a grant id, unlike drift and discovery revokes,
			// which know it from the event or the URL. Resolve it from the
			// webhook-maintained grant index; a miss leaves it empty, and the
			// drain then fails just that row (GetGrantIndexByUserProject).
			if p.OpType == "revoke" && p.ZitadelGrantID == "" {
				if idx, err := GetGrantIndexByUserProject(ctx, p.UserID, p.ProjectID); err == nil {
					p.ZitadelGrantID = idx.GrantID
				} else if !errors.Is(err, ErrGrantIndexNotFound) {
					return "", err
				}
			}
			if ownKey, err = newOutboxIdempotencyKey(); err != nil {
				return "", err
			}
		}
		var id string
		if err := tx.QueryRow(ctx, `
			INSERT INTO propagation_outbox
				(op_type, user_id, project_id, role_keys, zitadel_grant_id, payload_json,
				 idempotency_key, initiated_by, source, source_ref, cascade_id, reason, wait_reason, target)
			VALUES ($1,$2,$3,$4,NULLIF($5,''),$6,$7,$8,$9,NULLIF($10,''),$11,NULLIF($12,''),NULLIF($13,''),'zitadel')
			RETURNING id`, p.OpType, p.UserID, p.ProjectID, p.RoleKeys, p.ZitadelGrantID,
			jsonOrEmpty(p.PayloadJSON), ownKey, p.GrantedBy, outboxSource(p), p.SourceRef, cascade,
			why, p.WaitReason).Scan(&id); err != nil {
			return "", fmt.Errorf("insert outbox: %w", err)
		}
		return id, nil
	}

	ids := make([]string, 0, len(params))
	if key != "" {
		p := params[0]
		var own string
		write := func() error {
			if err := enqueueLedger(ctx, tx, p); err != nil || p.NoPropagation {
				return err
			}
			var err error
			own, err = row(p, key)
			return err
		}
		// What the rules owe beyond the write's own roles becomes the rows
		// below: source 'rule', no ledger row, one cascade.
		if FollowRules == nil {
			params, err = nil, write()
		} else {
			params, err = FollowRules(context.WithValue(ctx, txKey, tx), p, write)
		}
		if err != nil {
			return nil, err
		}
		ids = append(ids, own)
	} else {
		// NULL, not the empty string: the column is UUID, and "this event
		// caused nothing Change history shows" is exactly what NULL means.
		var auditCascadeID *string
		if cascadeGroupVisible(params) {
			auditCascadeID = &cascadeID
		}
		for _, a := range audits {
			target := a.Target
			if target == "" {
				target = "-"
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO audit_logs
					(actor_zitadel_user_id, target_zitadel_user_id, action, resource_id, cascade_id, details)
				VALUES ($1,$2,$3,$4,$5, CASE WHEN $6 = '' THEN NULL ELSE jsonb_build_object('reason', $6::text) END)`,
				a.Actor, target, a.Action, a.ResourceID, auditCascadeID, a.Reason); err != nil {
				return nil, err
			}
		}
	}

	for _, p := range params {
		if err := closeGivenRequests(ctx, tx, p); err != nil {
			return nil, err
		}
		id, err := row(p, "")
		if err != nil {
			return nil, err
		}
		if id != "" {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// ownAndRules splits a named write's ids: its own row, then the rules'.
func ownAndRules(ids []string) (string, []string) {
	if len(ids) < 2 {
		return ids[0], nil
	}
	return ids[0], ids[1:]
}

// enqueueLedger is a named write's own record: the ledger rows for exactly
// the roles it names, the requests it answers closed, and its audit row.
func enqueueLedger(ctx context.Context, tx pgx.Tx, p EnqueueParams) error {
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
				p.GrantedBy, p.Reason, p.ExpiresAt, outboxSource(p), p.SourceRef, string(p.EndRule)).Scan(&id); err != nil {
				return fmt.Errorf("upsert direct grant (%s): %w", role, err)
			}
			if i == 0 {
				firstGrantID = id
			}
		}
	}

	if err := closeGivenRequests(ctx, tx, p); err != nil {
		return err
	}

	// With NoPropagation no outbox row follows: the ledger row is the durable
	// intent and this audit row the durable trace. Adopting drift creates no
	// work for the queue.
	const insertAudit = `INSERT INTO audit_logs
		(actor_zitadel_user_id, target_zitadel_user_id, action, resource_id) VALUES ($1,$2,$3,$4)`
	action := "direct_grant." + opTypeAuditVerb(p.OpType)
	if _, err := tx.Exec(ctx, insertAudit, p.GrantedBy, p.UserID, action, firstGrantID); err != nil {
		return fmt.Errorf("insert audit: %w", err)
	}
	return nil
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
