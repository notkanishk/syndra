package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// The entitlement plane's enqueue (change `addon-platform`, task 2.9).
//
// An entitlement change against an add-on target becomes queued work before any
// add-on is called, in the same transaction as its audit row. The ordering is
// the contract: a mutation that outruns its own trace is a mutation nobody can
// account for afterwards, and the add-on plane has no parameters retained
// anywhere to reconstruct one from.

var (
	// ErrTargetNotActive: the deployment no longer configures this target, so
	// nothing will ever drain the row. Distinct from unreachable, which is a
	// target that is still deployed and merely not answering — that one queues
	// (design §7), and this one must not, because "queued forever" reads as
	// "recorded" on every surface that counts it.
	ErrTargetNotActive = errors.New("db: the target is not active")
	ErrNoSuchTarget    = errors.New("db: no such target")
	ErrInvalidEnqueue  = errors.New("db: invalid entitlement enqueue")
	// ErrNoClaimedApproval: there is no claimed plan subject to queue work
	// under. Either it does not exist, or its plan has not been applied.
	ErrNoClaimedApproval = errors.New("db: no claimed approval authorises this work")
	// ErrAlreadyQueued: one approval, one queued convergence.
	ErrAlreadyQueued = errors.New("db: this approval already has queued work")
	// ErrNotAnEntitlementTarget: Zitadel rows carry their own project and role
	// columns, which this path cannot fill.
	ErrNotAnEntitlementTarget = errors.New("db: this target's rows are enqueued by the direct-grant path")
)

// EntitlementApply is one subject's queued convergence onto an approved
// entitlement set.
//
// It names the approval and nothing else. The subject, the target, and the
// operator who approved it are read from that approval by the INSERT itself
// rather than supplied alongside it: three values a caller passes are three
// values a caller can pass inconsistently, and the one that matters — this work
// is for the person the approval named — would then hold only where the caller
// remembered to make it hold. There is no payload field either, because the
// intent is the desired-state snapshot the approval already points at.
type EntitlementApply struct {
	// PlanSubjectID is the claimed plan's per-subject row.
	PlanSubjectID string
	// Source is how the change came about. Defaults to `direct`.
	Source string
}

// validOutboxSource reports whether s is one of the sources the outbox CHECK
// accepts. A switch over literals rather than a package-level slice: a slice is
// a mutable variable any caller can widen before the check reads it, and the
// database would then refuse the write the check had just approved.
func validOutboxSource(s string) bool {
	switch s {
	case "direct", "bundle", "rule", "external_backfill", "lifecycle_cascade":
		return true
	default:
		return false
	}
}

// EnqueueEntitlementApplyTx writes the outbox row and its audit row on an
// existing transaction, returning the outbox id.
//
// On the caller's transaction because the apply gate claims the plan in that
// same transaction: an enqueue that failed after the claim committed would
// leave an operator holding an approval spent on work that was never queued.
//
// The row is written by an INSERT … SELECT over the approval, so every bound
// field comes from the approval rather than from the caller. A foreign key
// would have proved only that the plan subject exists — not that it is this
// person's, not that its plan was ever claimed, and not that its target still
// takes work. Each of those is in the predicate, so a row that should not exist
// is never inserted rather than inserted and then argued about.
//
// `op_type` is fixed at `apply`. An entitlement convergence is level-triggered
// onto a resolved desired set — there is no add/revoke/replace distinction to
// make, so there is no way to make it wrongly.
func EnqueueEntitlementApplyTx(ctx context.Context, tx pgx.Tx, p EntitlementApply) (string, error) {
	source := p.Source
	if source == "" {
		source = "direct"
	}
	switch {
	case !looksLikeUUID(p.PlanSubjectID):
		// An entitlement apply with no approval behind it is the thing the plan
		// gate exists to prevent, so it cannot be enqueued without one. Refused
		// here rather than by the foreign key, whose violation message quotes
		// the value that broke it.
		return "", fmt.Errorf("%w: an entitlement apply must cite the plan subject that approved it", ErrInvalidEnqueue)
	case !validOutboxSource(source):
		return "", fmt.Errorf("%w: unknown source", ErrInvalidEnqueue)
	}

	key, err := newOutboxIdempotencyKey()
	if err != nil {
		return "", err
	}

	// project_id, role_keys and zitadel_grant_id stay NULL: they are the shape
	// of the one target that has them, and `p.target <> $3` keeps that target's
	// rows off this path entirely. payload_json is an empty object because the
	// column is NOT NULL and this row has nothing to say that the snapshot does
	// not — no parameter of this function can reach it.
	//
	// The NOT EXISTS gives a clean refusal for work already queued; the unique
	// index behind it is what makes that true under a concurrent second caller.
	const insertOutbox = `
		INSERT INTO propagation_outbox
			(op_type, user_id, payload_json, idempotency_key, initiated_by, source, target, plan_subject_id)
		SELECT 'apply', ps.subject_id, '{}'::jsonb, $1, p.created_by, $2, p.target, ps.id
		  FROM plan_subjects ps
		  JOIN plans   p ON p.id = ps.plan_id
		  JOIN targets t ON t.target = p.target
		 WHERE ps.id = $4::uuid
		   AND p.applied_at IS NOT NULL
		   AND p.target <> $3
		   AND t.state = 'active'
		   AND NOT EXISTS (SELECT 1 FROM propagation_outbox o WHERE o.plan_subject_id = ps.id)
		RETURNING id, user_id, initiated_by, target`

	var outboxID, subjectID, initiatedBy, target string
	err = tx.QueryRow(ctx, insertOutbox, key, source, TargetZitadel, p.PlanSubjectID).
		Scan(&outboxID, &subjectID, &initiatedBy, &target)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", explainEnqueueRefusal(ctx, tx, p.PlanSubjectID)
	}
	if err != nil {
		return "", fmt.Errorf("insert entitlement outbox row: %w", err)
	}

	const insertAudit = `INSERT INTO audit_logs
		(actor_zitadel_user_id, target_zitadel_user_id, action, resource_id) VALUES ($1,$2,$3,$4)`
	if _, err := tx.Exec(ctx, insertAudit, initiatedBy, subjectID, "entitlement."+target+".enqueued", outboxID); err != nil {
		return "", fmt.Errorf("insert entitlement audit row: %w", err)
	}
	return outboxID, nil
}

// approvalRef is the state of an approval the insert declined to act on.
type approvalRef struct {
	found         bool
	target        string
	targetState   string
	claimed       bool
	alreadyQueued bool
}

// explainEnqueueRefusal re-reads the approval to say why the insert matched
// nothing. It explains; it never grants.
func explainEnqueueRefusal(ctx context.Context, tx pgx.Tx, planSubjectID string) error {
	const q = `
		SELECT p.target,
		       COALESCE(t.state, ''),
		       p.applied_at IS NOT NULL,
		       EXISTS (SELECT 1 FROM propagation_outbox o WHERE o.plan_subject_id = ps.id)
		  FROM plan_subjects ps
		  JOIN plans p ON p.id = ps.plan_id
		  LEFT JOIN targets t ON t.target = p.target
		 WHERE ps.id = $1::uuid`

	var ref approvalRef
	err := tx.QueryRow(ctx, q, planSubjectID).Scan(&ref.target, &ref.targetState, &ref.claimed, &ref.alreadyQueued)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return fmt.Errorf("%w: %s", ErrNoClaimedApproval, planSubjectID)
	case err != nil:
		return fmt.Errorf("read refused approval: %w", err)
	}
	ref.found = true
	if reason := enqueueRefusal(ref, planSubjectID); reason != nil {
		return reason
	}
	// The approval now looks queueable, so it was queued and removed between
	// the insert and this read. Naming a specific reason would assert what this
	// read just contradicted.
	return fmt.Errorf("db: no work could be queued under approval %s and the reason no longer holds", planSubjectID)
}

// enqueueRefusal names why an approval cannot be queued, or nil if it can.
// Pure, and never the authority: the insert's predicate is, and the guard test
// holds the two to the same set of conditions.
//
// Ordered by what the operator does about it. "This target is not ours to queue
// for" and "this approval was never claimed" are statements about the request;
// "already queued" is a statement about the world having moved on.
func enqueueRefusal(r approvalRef, planSubjectID string) error {
	switch {
	case !r.found:
		return fmt.Errorf("%w: %s", ErrNoClaimedApproval, planSubjectID)
	case r.target == TargetZitadel:
		return fmt.Errorf("%w: %s", ErrNotAnEntitlementTarget, TargetZitadel)
	case !r.claimed:
		// The approval exists and nobody has spent it. Queuing work under it
		// would be an apply that never happened.
		return fmt.Errorf("%w: the plan behind %s has not been applied", ErrNoClaimedApproval, planSubjectID)
	case r.targetState != TargetActive:
		return fmt.Errorf("%w: %s is %q", ErrTargetNotActive, r.target, r.targetState)
	case r.alreadyQueued:
		return fmt.Errorf("%w: %s", ErrAlreadyQueued, planSubjectID)
	}
	return nil
}

// LockTargetStateTx reads a target's registration state and holds the row until
// the transaction ends.
//
// The lock is the point. An unlocked read races the registry reconciliation
// that disables targets a deployment has dropped: it could return `active`,
// the reconciliation could commit a disable, and the apply could then commit
// the permanently undrainable row this whole check exists to refuse. Holding
// the row makes the two serialize — an apply that started first finishes, and
// the disable lands behind it.
func LockTargetStateTx(ctx context.Context, tx pgx.Tx, target string) (string, error) {
	const q = `SELECT state FROM targets WHERE target = $1 FOR UPDATE`
	var state string
	if err := tx.QueryRow(ctx, q, target).Scan(&state); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", fmt.Errorf("%w: %s", ErrNoSuchTarget, target)
		}
		return "", fmt.Errorf("read target state: %w", err)
	}
	return state, nil
}
