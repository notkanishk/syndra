package db

import (
	"context"
	"errors"
	"fmt"
	"strings"

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
)

// EntitlementApply is one subject's queued convergence onto an approved
// entitlement set.
//
// There is no payload field, and that is deliberate. The intent lives in the
// desired-state snapshot the plan subject row points at; a payload column here
// would be a second copy of it, free to disagree, and JSONB accepts whatever a
// future writer puts in it.
type EntitlementApply struct {
	Target string
	// SubjectID is the person. PlanSubjectID is the approval that authorises
	// acting on them, and the drain re-verifies its fingerprint before it
	// dispatches.
	SubjectID     string
	PlanSubjectID string
	InitiatedBy   string
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
// `op_type` is fixed at `apply` rather than taken as a parameter. An entitlement
// convergence is level-triggered onto a resolved desired set — there is no
// add/revoke/replace distinction to make, so there is no way to make it wrongly.
func EnqueueEntitlementApplyTx(ctx context.Context, tx pgx.Tx, p EntitlementApply) (string, error) {
	source := p.Source
	if source == "" {
		source = "direct"
	}

	switch {
	case strings.TrimSpace(p.Target) == "":
		return "", fmt.Errorf("%w: target is required", ErrInvalidEnqueue)
	case p.Target == TargetZitadel:
		// Not a database constraint doing this by accident. A Zitadel row's
		// intent is its own project and role columns, which this path has no
		// way to fill; the table's shape CHECK would refuse the write, and a
		// constraint violation is a worse way to learn it than a refusal that
		// names the reason.
		return "", fmt.Errorf("%w: %s rows carry their own project and roles and are enqueued by the direct-grant path", ErrInvalidEnqueue, TargetZitadel)
	case strings.TrimSpace(p.SubjectID) == "":
		return "", fmt.Errorf("%w: subject is required", ErrInvalidEnqueue)
	case !looksLikeUUID(p.PlanSubjectID):
		// An entitlement apply with no approval behind it is the thing the plan
		// gate exists to prevent, so it cannot be enqueued without one.
		return "", fmt.Errorf("%w: an entitlement apply must cite the plan subject that approved it", ErrInvalidEnqueue)
	case strings.TrimSpace(p.InitiatedBy) == "":
		return "", fmt.Errorf("%w: initiated_by is required", ErrInvalidEnqueue)
	case !validOutboxSource(source):
		return "", fmt.Errorf("%w: unknown source", ErrInvalidEnqueue)
	}

	key, err := newOutboxIdempotencyKey()
	if err != nil {
		return "", err
	}

	// project_id, role_keys and zitadel_grant_id stay NULL: they are the shape
	// of the one target that has them. payload_json is an empty object because
	// the column is NOT NULL and this row has nothing to say that the snapshot
	// does not — no parameter of this function can reach it.
	const insertOutbox = `
		INSERT INTO propagation_outbox
			(op_type, user_id, payload_json, idempotency_key, initiated_by, source, target, plan_subject_id)
		VALUES ('apply', $1, '{}'::jsonb, $2, $3, $4, $5, $6)
		RETURNING id`
	var outboxID string
	if err := tx.QueryRow(ctx, insertOutbox,
		p.SubjectID, key, p.InitiatedBy, source, p.Target, p.PlanSubjectID,
	).Scan(&outboxID); err != nil {
		return "", fmt.Errorf("insert entitlement outbox row: %w", err)
	}

	const insertAudit = `INSERT INTO audit_logs
		(actor_zitadel_user_id, target_zitadel_user_id, action, resource_id) VALUES ($1,$2,$3,$4)`
	if _, err := tx.Exec(ctx, insertAudit, p.InitiatedBy, p.SubjectID, "entitlement."+p.Target+".enqueued", outboxID); err != nil {
		return "", fmt.Errorf("insert entitlement audit row: %w", err)
	}
	return outboxID, nil
}

// TargetStateTx reads a target's registration state on an existing transaction,
// so a caller deciding whether to queue work reads the same row it is about to
// write against.
func TargetStateTx(ctx context.Context, tx pgx.Tx, target string) (string, error) {
	const q = `SELECT state FROM targets WHERE target = $1`
	var state string
	if err := tx.QueryRow(ctx, q, target).Scan(&state); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", fmt.Errorf("%w: %s", ErrNoSuchTarget, target)
		}
		return "", fmt.Errorf("read target state: %w", err)
	}
	return state, nil
}
