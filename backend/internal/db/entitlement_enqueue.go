package db

import (
	"context"
	"encoding/json"
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

	// The target is read FOR UPDATE, by this statement, and that is what makes
	// the state check mean anything here. A plain join reads an MVCC snapshot: a
	// caller could start this INSERT while the target was active, have a
	// deregistration disable it and sweep its queued work, and still commit a
	// fresh pending row behind that sweep — undrainable, and invisible to the
	// sweep that already ran. Under FOR UPDATE the row is either locked before
	// the disable, in which case the disable waits and then abandons what this
	// wrote, or locked after it, in which case READ COMMITTED re-checks the
	// predicate against the new version, finds it disabled, and matches nothing.
	//
	// The apply gate locks the same row earlier for a different reason: to
	// refuse before spending the approval. Same row, same order, no deadlock —
	// and a caller that skips the gate is no longer relying on it.
	//
	// project_id, role_keys and zitadel_grant_id stay NULL: they are the shape
	// of the one target that has them, and `p.target <> $3` keeps that target's
	// rows off this path entirely. payload_json is an empty object because the
	// column is NOT NULL and this row has nothing to say that the snapshot does
	// not — no parameter of this function can reach it.
	//
	// ON CONFLICT rather than a NOT EXISTS predicate, because the case that
	// matters is the one a predicate cannot see: a concurrent caller whose row
	// is not committed yet. That loser would raise 23505, and a raised
	// constraint violation aborts the whole transaction — so instead of the
	// typed refusal this function promises, the caller would get a dead
	// transaction and an error about an index. DO NOTHING turns it into no row
	// returned, which is the same answer every other refusal here gives.
	//
	// The conflict target is named, and named with the index's own predicate,
	// so only this uniqueness is absorbed. An idempotency-key collision still
	// raises, which is right: that one means the key generator repeated itself.
	const insertOutbox = `
		WITH locked_target AS (
			SELECT t.target FROM targets t
			 WHERE t.target = (SELECT p.target FROM plan_subjects ps
			                     JOIN plans p ON p.id = ps.plan_id
			                    WHERE ps.id = $4::uuid)
			   AND t.state = 'active'
			   FOR UPDATE
		)
		INSERT INTO propagation_outbox
			(op_type, user_id, payload_json, idempotency_key, initiated_by, source, target, plan_subject_id)
		SELECT 'apply', ps.subject_id, '{}'::jsonb, $1, p.created_by, $2, p.target, ps.id
		  FROM plan_subjects ps
		  JOIN plans        p ON p.id = ps.plan_id
		  JOIN locked_target lt ON lt.target = p.target
		 WHERE ps.id = $4::uuid
		   AND p.applied_at IS NOT NULL
		   AND p.target <> $3
		ON CONFLICT (plan_subject_id) WHERE plan_subject_id IS NOT NULL DO NOTHING
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

// EntitlementIntent is what an add-on drain needs to dispatch one queued row.
//
// Read as one object because its parts must describe one decision: the outbox
// row, the plan subject it cites, the snapshot that subject approved, and the
// fingerprint the state was reviewed against. Assembled by a caller from
// separate reads they can describe a decision that never existed, which is the
// same reason `ReconcileLedgerOnApplied` reads its tuple off the row.
type EntitlementIntent struct {
	OutboxID string
	Target   string
	// PlanID is the approval this row executes. It travels to the add-on inside
	// the signed body so a call intercepted and replayed cannot be re-aimed at
	// another approval, and so the add-on's own record names what authorised it.
	PlanID    string
	SubjectID string
	// Fingerprint is what the add-on re-verifies against live state
	// immediately before writing.
	Fingerprint string
	// DesiredJSON is the approved snapshot's state. Carried as raw JSON so
	// nothing between here and the add-on has to know what a field means.
	DesiredJSON []byte
	// Version is the snapshot's monotonic version, for the ordering the drain
	// already enforces at the claim.
	Version int64
	// Surface is the screen whose rehearsal issued the approval. The drain reads
	// it to answer one question and no other: was there a human who reviewed a
	// diff behind this row. A system-initiated convergence has no such review,
	// so the fingerprint it carries protects nothing an operator saw — and
	// verifying against a trigger-time read would fail an ordinary access change
	// because somebody edited the target in between.
	Surface string
}

// Desired decodes the approved snapshot into the per-field shape the transport
// sends.
//
// A decode failure yields nil rather than an error, and the caller must treat
// nil as "nothing to send" rather than "send an empty set". An unreadable
// snapshot is a snapshot nobody can act on; converging a subject to an empty
// desired state on the strength of one would remove every entitlement they have
// because a JSON column could not be parsed.
func (i EntitlementIntent) Desired() map[string]json.RawMessage {
	if len(i.DesiredJSON) == 0 {
		return nil
	}
	var out map[string]json.RawMessage
	if err := json.Unmarshal(i.DesiredJSON, &out); err != nil {
		return nil
	}
	return out
}

// ReadEntitlementIntent resolves a claimed outbox row into what it approved.
//
// The RECORDED snapshot, never a fresh resolution. An operator-initiated change
// is an approval and must apply the diff that was seen; re-resolving here would
// dispatch whatever policy says now, which is a different decision wearing the
// approval's plan id. (Periodic reconciliation is the other read path and
// resolves current state deliberately — conflating the two is how a reconcile
// loop reverts an intentional edit.)
func ReadEntitlementIntent(ctx context.Context, outboxID string) (EntitlementIntent, error) {
	const q = `
		SELECT p.id, p.target, ps.plan_id, s.subject_id, ps.fingerprint, s.state_json, s.version, pl.surface
		  FROM propagation_outbox p
		  JOIN plan_subjects ps ON ps.id = p.plan_subject_id
		  JOIN plans pl ON pl.id = ps.plan_id
		  JOIN desired_state_snapshots s ON s.id = ps.snapshot_id
		 WHERE p.id = $1`
	var out EntitlementIntent
	err := PG.QueryRow(ctx, q, outboxID).Scan(
		&out.OutboxID, &out.Target, &out.PlanID, &out.SubjectID, &out.Fingerprint, &out.DesiredJSON, &out.Version, &out.Surface)
	if errors.Is(err, pgx.ErrNoRows) {
		// A row with no approval chain is not a row this drain may dispatch.
		// Reported rather than defaulted: dispatching an empty desired state
		// would converge the subject to nothing.
		return EntitlementIntent{}, fmt.Errorf("%w: %s", ErrNoApprovedIntent, outboxID)
	}
	if err != nil {
		return EntitlementIntent{}, fmt.Errorf("read entitlement intent %s: %w", outboxID, err)
	}
	return out, nil
}

// ErrNoApprovedIntent is an outbox row that cites no snapshot. It is a
// programming error rather than an operational one — the enqueue writes the
// citation — and it is refused rather than defaulted, because the default would
// be an empty desired state and an empty desired state removes every
// entitlement the subject has.
var ErrNoApprovedIntent = errors.New("db: the outbox row cites no approved desired state")
