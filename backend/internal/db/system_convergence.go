package db

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// A convergence nobody approved (design §4, §15; change `addon-platform` 7.9).
//
// A role change cascades: somebody is assigned a bundle, the bundle carries a
// role, the role is mapped to a target. Nothing about that was rehearsed and
// nobody read a diff — and yet the outbox row still has to cite an approval,
// because the dispatcher reads the intent through that citation and refuses a
// row without one.
//
// So the system mints its own. Not because the citation is a formality, but
// because the alternative is a second dispatch path with no recorded intent
// behind it: the drain would have to re-resolve at dispatch time, and nothing
// would say afterwards what the system meant to do at the moment the role
// changed. One shape, one dispatcher, one audit trail.
//
// What the minted plan does NOT get is a state fingerprint, and that is the
// honest part. A fingerprint asserts "this is the world a person reviewed"; no
// person reviewed this one. The row carries a sentinel instead, and the drain
// re-reads live state for a lifecycle-surfaced row before dispatching it. The
// sentinel is deliberately not a digest-shaped string, so if that re-read is
// ever skipped the add-on refuses the call as stale rather than writing against
// a fingerprint that happened to match.

// SystemFingerprint is what a system-minted plan subject records instead of a
// state digest.
//
// It cannot equal any real fingerprint — those are hex digests — so a code path
// that stopped re-reading would fail closed and loudly at the add-on rather than
// silently verifying against nothing.
const SystemFingerprint = "system:re-verified-at-dispatch"

// SystemConvergenceSurface is the plan surface these are issued on, and the
// value the drain matches to decide whether to re-read. Named here rather than
// in the drain because the writer and the reader must agree, and one of the two
// has to own the constant.
const SystemConvergenceSurface = "entitlements.lifecycle"

// ErrNotASystemConvergence refuses to mint one of these on any other surface.
var ErrNotASystemConvergence = errors.New("db: a system convergence is issued on its own surface and no other")

// SystemConvergence is one subject's automatic convergence on one target.
type SystemConvergence struct {
	Target    string
	SubjectID string
	// Actor is who did the thing that caused this — the operator who assigned
	// the bundle, not the system. A convergence attributed to "the system" is
	// one nobody can trace back to a decision.
	Actor string
	// Reason is what happened, in the operator's language, for the audit row.
	Reason string
	// Desired is the resolved set, recorded as the intent this convergence is
	// executing. Never nil: nil records as JSON null and the drain reads that
	// back as no approved intent.
	Desired map[string]json.RawMessage
	// WithdrawsOnly says this convergence can only take access away, which is
	// what lets the background runner drain it without an operator.
	//
	// Declared by whoever queues it, and only where it is true by construction:
	// a revocation resolves its set with the lifecycle field denied. A cascade
	// leaves it false even when the delta happens to be a removal, because the
	// resolved set it carries is whatever policy now says — and on the next
	// person that same code path confers.
	WithdrawsOnly bool
}

// systemRequestFingerprint binds an approval to the convergence that caused it.
//
// Length-prefixed like every other fingerprint in this codebase: any separator
// can appear inside a target name, and two different pairs hashing alike is a
// collision somebody picks rather than finds.
func systemRequestFingerprint(target, subject string) string {
	h := sha256.New()
	for _, field := range []string{"system_convergence", target, subject} {
		fmt.Fprintf(h, "%d:", len(field))
		h.Write([]byte(field))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// RecordSystemConvergence mints the approval, spends it, and queues the work, on
// the caller's ambient transaction when there is one.
//
// All three in one transaction because they are one decision. A plan without its
// outbox row is an approval of work nobody queued; an outbox row whose plan was
// never claimed is refused by the enqueue itself. And joining the caller's
// transaction is what puts this inside the access lock the cascade already
// holds: the reads that decided the role change and the convergence that follows
// from it commit together, or a concurrent expiry lands between them.
//
// It spends the approval it just minted rather than marking it claimed at
// insert, because the claim predicate is where "one approval, one apply" and
// "an approval is a person's" live. Minting a pre-claimed row would be a second
// way to produce a claimed plan, and the second way is the one nobody checks.
func RecordSystemConvergence(ctx context.Context, c SystemConvergence) (planID, outboxID string, err error) {
	switch {
	case strings.TrimSpace(c.Target) == "" || c.Target == TargetZitadel:
		return "", "", fmt.Errorf("%w: %q is not an add-on target", ErrNotASystemConvergence, c.Target)
	case strings.TrimSpace(c.SubjectID) == "":
		return "", "", fmt.Errorf("%w: no subject", ErrNotASystemConvergence)
	case strings.TrimSpace(c.Actor) == "":
		return "", "", fmt.Errorf("%w: no actor — a convergence nobody can trace to a decision", ErrNotASystemConvergence)
	case c.Desired == nil:
		return "", "", fmt.Errorf("%w: no resolved intent", ErrNotASystemConvergence)
	}

	tx, owned, err := beginOrJoin(ctx)
	if err != nil {
		return "", "", err
	}
	if owned {
		defer tx.Rollback(ctx) // no-op after a successful Commit
	}

	// The request fingerprint binds the approval to what caused it. There is no
	// request body here, so it binds the subject and the target — enough that a
	// citation cannot be spent against a different convergence, and honest about
	// being the whole of what the system decided.
	requestFP := systemRequestFingerprint(c.Target, c.SubjectID)

	plan, err := createPlanTx(ctx, tx, NewPlan{
		Target:    c.Target,
		Surface:   SystemConvergenceSurface,
		CreatedBy: c.Actor,
		// A lifetime it will never reach: the plan is claimed in this same
		// transaction. It carries one because the store requires a confirmed
		// plan to be bounded, and this is not provisional — the state it will be
		// verified against is read at dispatch, not now.
		Lifetime:           time.Minute,
		StateReadAt:        time.Now().UTC(),
		RequestFingerprint: requestFP,
		Subjects: []NewPlanSubject{{
			SubjectID:    c.SubjectID,
			DesiredState: c.Desired,
			Fingerprint:  SystemFingerprint,
			Outcome:      PlanOutcome{Effect: PlanEffectApply},
		}},
	})
	if err != nil {
		return "", "", err
	}

	_, subjects, err := ClaimPlanTx(ctx, tx, PlanCitation{
		PlanID: plan.ID, Target: c.Target, Surface: SystemConvergenceSurface,
		Actor: c.Actor, RequestFingerprint: requestFP,
	})
	if err != nil {
		return "", "", fmt.Errorf("claim system convergence: %w", err)
	}
	if len(subjects) != 1 {
		return "", "", fmt.Errorf("%w: the approval names %d subjects", ErrNotASystemConvergence, len(subjects))
	}

	outboxID, err = EnqueueEntitlementApplyTx(ctx, tx, EntitlementApply{
		PlanSubjectID: subjects[0].ID,
		WithdrawsOnly: c.WithdrawsOnly,
	})
	if err != nil {
		return "", "", fmt.Errorf("queue system convergence: %w", err)
	}

	if owned {
		if err := tx.Commit(ctx); err != nil {
			return "", "", fmt.Errorf("commit system convergence: %w", err)
		}
	}
	return plan.ID, outboxID, nil
}
