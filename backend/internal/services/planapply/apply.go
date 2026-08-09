// Package planapply is the gate every operator-initiated apply passes through.
//
// An apply cites the plan it was approved under and nothing else. It does not
// re-submit the original request, because recomputing a plan at apply time is
// the gap this whole mechanism closes: an operator reads one diff and a second
// request computes another, against a world that moved in between (design §8).
//
// The gate deliberately does NOT verify fingerprints. That is not an omission.
// Rows sit in the outbox until an operator resumes the drain, so a target can
// move between a verified apply and the actual write; "the diff you approved is
// the diff that lands" has to hold up to landing, not up to accepting.
// Verification happens at dispatch, against the fingerprint on the plan subject
// row this gate makes each outbox row cite.
package planapply

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"syndra/internal/db"
)

// ErrPlanRequired is the refusal of an apply that cites no plan.
//
// Its own error because it is its own operator action: an unknown or expired id
// means re-plan, and no id at all means the surface sent a request this backend
// no longer accepts.
var ErrPlanRequired = errors.New("planapply: an apply must cite the plan it was approved under")

// Request is an apply naming the approval it executes.
type Request struct {
	PlanID string
	// Target and Surface are where the apply arrived. They are checked against
	// the plan rather than trusted: a plan issued by drift triage cited on the
	// bulk-grant endpoint is a plan applied to a diff nobody reviewed there.
	Target  string
	Surface string
	Actor   string
}

// QueuedSubject is one person's queued work.
type QueuedSubject struct {
	SubjectID string `json:"subject_id"`
	OutboxID  string `json:"outbox_id"`
}

// Result is what an accepted apply produced. Queued, never applied: nothing has
// reached the target yet, and the drain has not run.
type Result struct {
	PlanID string `json:"plan_id"`
	Target string `json:"target"`
	// Provisional says the plan was computed against last-known state while the
	// target was unreachable. The surface must not present these rows as
	// applied — they are recorded and waiting for the target to come back and
	// be re-fingerprinted.
	Provisional bool            `json:"provisional"`
	Queued      []QueuedSubject `json:"queued"`
}

// Apply spends a plan and queues the work it approved.
//
// One transaction, and the ordering inside it is the point: the plan is claimed
// before any row is written, and the claim is undone if any write fails. A
// claim that committed ahead of its work would leave an operator holding an
// approval spent on nothing, with no way to re-apply it.
func Apply(ctx context.Context, req Request) (Result, error) {
	if strings.TrimSpace(req.PlanID) == "" {
		return Result{}, ErrPlanRequired
	}

	var res Result
	err := inTx(ctx, func(tx pgx.Tx) error {
		// Read the registration before spending the approval. A disabled target
		// is one the deployment removed, so its rows would sit in the outbox
		// with nothing ever coming to drain them — queued forever, and counted
		// as queued, which reads as "recorded" on every surface. Unreachable is
		// the other thing entirely, and that one does queue (design §7).
		state, err := targetState(ctx, tx, req.Target)
		if err != nil {
			return err
		}
		if state != db.TargetActive {
			return fmt.Errorf("%w: %s is %s", db.ErrTargetNotActive, req.Target, state)
		}

		plan, subjects, err := claimPlan(ctx, tx, db.PlanCitation{
			PlanID:  req.PlanID,
			Target:  req.Target,
			Surface: req.Surface,
			Actor:   req.Actor,
		})
		if err != nil {
			return err
		}

		queued := make([]QueuedSubject, 0, len(subjects))
		for _, s := range subjects {
			// The approval is the only thing passed. Subject, target, and the
			// operator who approved it are read from it by the insert itself:
			// values a caller supplies are values a caller can supply
			// inconsistently, and "this work is for the person the approval
			// named" would then hold only where the caller made it hold.
			outboxID, err := enqueue(ctx, tx, db.EntitlementApply{PlanSubjectID: s.ID})
			if err != nil {
				return fmt.Errorf("queue %s: %w", s.SubjectID, err)
			}
			queued = append(queued, QueuedSubject{SubjectID: s.SubjectID, OutboxID: outboxID})
		}

		res = Result{PlanID: plan.ID, Target: plan.Target, Provisional: plan.Provisional, Queued: queued}
		return nil
	})
	if err != nil {
		// Nothing partial escapes. A Result carrying the subjects queued before
		// the failure would describe work the rollback has already undone.
		return Result{}, err
	}
	return res, nil
}
