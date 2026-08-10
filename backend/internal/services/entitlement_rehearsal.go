package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"syndra/internal/addons"
	"syndra/internal/db"
)

// Rehearsing an entitlement change on an add-on target (design §4, §8, §15).
//
// This is the production path that connects the three halves that were built
// separately: the resolver decides who should hold what, the add-on says what
// converging to that would do and fingerprints the state it read, and the plan
// store records both so an apply can cite them. Before it existed the plan store
// and the apply gate had no caller — heavily tested, never executed.
//
// The order inside it is load-bearing and is the same order the Zitadel gate
// uses: resolve, ask, record. Recording before asking would store an approval of
// a diff nobody has seen; asking before resolving would ask about a set that is
// not the one the apply will send.

// ErrTargetIsBuiltIn refuses an entitlement rehearsal aimed at Zitadel.
//
// Zitadel is not an add-on: it has no manifest, no `/plan`, and its intent lives
// in the outbox row's own columns. A rehearsal here would resolve a set nothing
// can converge and record a snapshot nothing reads.
var ErrTargetIsBuiltIn = errors.New("services: the built-in target does not converge entitlement sets")

// ErrNoSubjects refuses a rehearsal for nobody, which would be answered with an
// empty diff, recorded as an approval, and applied cleanly while changing
// nothing.
var ErrNoSubjects = errors.New("services: a rehearsal affecting no subjects is not a rehearsal")

// ErrTargetUnplannable is the add-on itself being unreachable — distinct from
// the TARGET being unreachable, which produces a provisional plan rather than a
// refusal.
//
// The difference matters to an operator and to the fail-open rule. A target that
// is down is planned against last-known state, because the add-on keeps a
// mirror. An add-on that is down has no mirror to offer, so there is no state to
// plan against at all, and pretending otherwise would issue an approval computed
// from nothing.
var ErrTargetUnplannable = errors.New("services: the add-on could not be reached, so there is no state to plan against")

// EntitlementRehearsal is a proposed convergence across a cohort.
type EntitlementRehearsal struct {
	Target string
	// SubjectIDs is who to converge. Not a change description: the resolver
	// already knows what each of them should hold, so a rehearsal asks "bring
	// these people to their resolved state" rather than "give them X".
	SubjectIDs []string
	Actor      string
	// AcknowledgeScope is the operator confirming the blast radius. Checked
	// against the number of subjects that would CHANGE, not the number selected.
	AcknowledgeScope bool
}

// EntitlementPlan is a rehearsal and, once recorded, its citation.
type EntitlementPlan struct {
	BulkPlan
	// Provisional says the add-on answered from its mirror because the target
	// was unreachable. The surface must say so and must never present these rows
	// as applied.
	Provisional bool `json:"provisional"`
	// StateReadAt dates the read the outcomes were computed from. Rendered
	// beside a provisional plan, because "computed against last-known state"
	// with no number attached is a label an operator cannot act on.
	StateReadAt time.Time `json:"state_read_at"`
	// Truncated says the add-on's read hit its cap, which is why some rows came
	// back blocked without anything being wrong with the subject.
	Truncated bool `json:"truncated,omitempty"`
	// Desired is the resolved set each outcome was computed from, keyed by
	// subject. Carried on the rehearsal rather than re-resolved by whoever
	// records it: resolving twice is two answers to one question, and the second
	// one would be the one written into the immutable record while the first is
	// the one the operator read.
	//
	// Never serialised. It is the intent being approved, not the diff being
	// reviewed, and the diff is what a client renders.
	Desired map[string]EntitlementSet `json:"-"`
}

// EntitlementOp is the op name these plans are rendered under.
const EntitlementOp = "converge_entitlements"

// RehearseEntitlements computes the change and returns it unrecorded.
//
// Unrecorded on purpose: `IssueEntitlementPlan` is the step that makes it
// approvable, and separating them keeps the read-only rehearsal free of a write
// — an operator may look three times and apply once.
func RehearseEntitlements(ctx context.Context, req EntitlementRehearsal) (EntitlementPlan, error) {
	if req.Target == db.TargetZitadel {
		return EntitlementPlan{}, ErrTargetIsBuiltIn
	}
	ids := dedupeIDs(req.SubjectIDs)
	if len(ids) == 0 {
		return EntitlementPlan{}, ErrNoSubjects
	}

	// Resolved first, and for everybody, because this is the set the apply will
	// send. Asking the add-on about a different set than the one that lands is
	// the failure this whole mechanism exists to prevent.
	subjects := make([]addons.PlanSubject, 0, len(ids))
	resolved := make(map[string]EntitlementSet, len(ids))
	profiles := make(map[string]string, len(ids))
	for _, id := range ids {
		set, err := ResolveEntitlements(ctx, id, req.Target)
		if err != nil {
			return EntitlementPlan{}, fmt.Errorf("resolve %s: %w", id, err)
		}
		resolved[id] = set

		// The email is what a username is derived from for a subject with no
		// account yet. A lookup that fails is not fatal: an existing binding is
		// authoritative, so only a first-time creation needs it, and the add-on
		// reports a blocked outcome naming the reason if it cannot derive one.
		if p, found, err := directoryFindUser(ctx, id); err == nil && found {
			profiles[id] = p.Email
		}
		subjects = append(subjects, addons.PlanSubject{
			Subject: id, Email: profiles[id], Desired: set.Desired(),
		})
	}

	answer := addonsPlan(ctx, req.Target, subjects, req.AcknowledgeScope)
	if answer.Outcome != addons.OutcomeSucceeded {
		return EntitlementPlan{}, fmt.Errorf("%w: %v", ErrTargetUnplannable, answer.Err)
	}

	plan := EntitlementPlan{
		BulkPlan:    BulkPlan{Op: EntitlementOp},
		Provisional: !answer.Current,
		StateReadAt: answer.TakenAt,
		Truncated:   answer.Truncated,
		Desired:     resolved,
	}

	bySubject := make(map[string]addons.SubjectOutcome, len(answer.Outcomes))
	for _, o := range answer.Outcomes {
		bySubject[o.Subject] = o
	}
	for _, id := range ids {
		out, answered := bySubject[id]
		if !answered {
			// A subject the add-on did not answer for. Blocked rather than
			// dropped: dropping it would silently shrink the cohort an operator
			// selected, and a plan is the record of what they reviewed.
			plan.Outcomes = append(plan.Outcomes, BulkOutcome{
				UserID: id, Effect: EffectBlocked,
				Detail: "The target did not report what would happen to this person.",
				// A fingerprint that cannot match anything the apply reads back,
				// so a row that was never evaluated cannot verify.
				Fingerprint: Fingerprint("entitlement", id, "unanswered"),
			})
			continue
		}
		plan.Outcomes = append(plan.Outcomes, BulkOutcome{
			UserID:      id,
			Email:       profiles[id],
			Effect:      out.Effect,
			Detail:      out.Detail,
			Consequence: out.Consequence,
			Fingerprint: out.Fingerprint,
		})
	}
	plan.Summary = SummarizeOutcomes(plan.Outcomes)
	return plan, nil
}
