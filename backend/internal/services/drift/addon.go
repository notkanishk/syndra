package drift

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"syndra/internal/addons"
	"syndra/internal/db"
)

// Reconciling an add-on target (design §9, §15; change `addon-platform` 1.18,
// 1.22's second half).
//
// Deliberately NOT a drift sweep, and the difference is the design's own: an
// operator-initiated change applies the snapshot that was approved, and the
// periodic reconcile RESOLVES CURRENT STATE and converges to it. Conflating the
// two is how a reconcile loop reverts an intentional edit. So this raises no
// triage rows for a subject whose account has drifted — it queues the
// convergence that fixes it, which is what a level-triggered target is for.
//
// What it does raise is the thing convergence cannot answer: accounts on the
// target that Syndra never provisioned. `root`, service accounts, whatever an
// admin made by hand. Diffing those against expected state would classify every
// one of them as untraced drift and bury the queue on the first sweep after
// deployment — and trust in a triage queue is set on the day it first fills. So
// they are an INVENTORY: enumerated, reported, and never entered into triage. An
// account moving from unmanaged to bound is an explicit adoption decision and
// never something a sweep infers.

// UnmanagedAccount is one account on the target with no recorded binding.
type UnmanagedAccount struct {
	Username string `json:"username"`
	UID      int64  `json:"uid,omitempty"`
}

// AddonReconcileResult is what one pass saw and did.
type AddonReconcileResult struct {
	Target string `json:"target"`
	// Bound is how many subjects the backend has a recorded account for. The
	// denominator for everything else here.
	Bound int `json:"bound"`
	// Unmanaged is the inventory — reported, not triaged.
	Unmanaged []UnmanagedAccount `json:"unmanaged"`
	// Queued counts the convergences this pass enqueued. Never "fixed": they
	// wait for the drain like every other add-on row.
	Queued int `json:"queued"`
	// ReadAt and Current describe the state read everything above came from.
	ReadAt  time.Time `json:"read_at"`
	Current bool      `json:"current"`
	// Truncated says the read hit the add-on's cap, so the inventory is what was
	// seen rather than everything there is.
	Truncated bool `json:"truncated,omitempty"`
	// LogVerdict is what the add-on's mutation-log head said when compared
	// against the anchor. Carried on the reconcile result because this is the
	// pass that reads it, and because a finding that lives only in a log line is
	// a finding nobody is looking at.
	LogVerdict string `json:"log_verdict,omitempty"`
	// Halted says the pass stopped before concluding anything, with Reason
	// naming which of the target's failures it was.
	Halted bool   `json:"halted,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// Inventory is the read-only half: what lives on the target and which of it
// Syndra manages, with nothing queued and nothing recorded.
//
// Separate from the reconcile because an operator opening a page must not queue
// convergences by looking at it — and because the inventory is the one thing
// here that is still worth showing when the read is stale or capped, labelled
// with which it was.
func Inventory(ctx context.Context, target string) (AddonReconcileResult, error) {
	if target == db.TargetZitadel {
		return AddonReconcileResult{}, fmt.Errorf("inventory: %s holds no accounts of its own", target)
	}
	res := AddonReconcileResult{Target: target}

	read := addonSubjects(ctx, target)
	if read.Outcome != addons.OutcomeSucceeded {
		res.Halted, res.Reason = true, db.UnreconciledUnreachable
		return res, nil
	}
	bindings, err := listBindings(ctx, target)
	if err != nil {
		return res, fmt.Errorf("read bindings for %s: %w", target, err)
	}
	res.Bound = len(bindings)
	res.Unmanaged = unmanaged(read.Accounts, bindings)
	res.ReadAt, res.Current, res.Truncated = read.TakenAt, read.Current, read.Truncated
	if !read.Current {
		// Shown, and labelled. An operator asking what else lives on the target
		// during an outage is better served by a dated answer than by nothing —
		// what they must not do is adopt from it, and the label is what stops
		// them.
		res.Reason = db.UnreconciledStaleRead
	} else if read.Truncated {
		res.Reason = db.UnreconciledTruncated
	}
	return res, nil
}

// ReconcileAddon reads a target, converges the subjects it manages, and reports
// the accounts it does not.
func ReconcileAddon(ctx context.Context, target string) (AddonReconcileResult, error) {
	if target == db.TargetZitadel {
		// Zitadel has its own sweep, which pages the Management API and
		// concludes differently. Refused rather than ignored: a caller here with
		// the built-in target has a wrong model of which reconciler does what.
		return AddonReconcileResult{}, fmt.Errorf("reconcile add-on: %s is the built-in target", target)
	}
	res := AddonReconcileResult{Target: target}

	read := addonSubjects(ctx, target)
	switch {
	case read.Outcome != addons.OutcomeSucceeded:
		return halt(ctx, res, db.UnreconciledUnreachable,
			fmt.Sprintf("the target could not be read: %v", read.Err))
	case !read.Current:
		// The add-on answered from its mirror. Every conclusion drawn from it
		// would be about an earlier moment, and the convergences queued from one
		// would be computed against a world that has moved — so the pass records
		// an absence of evidence rather than fabricating evidence of absence.
		return halt(ctx, res, db.UnreconciledStaleRead,
			"the target answered from its mirror, so nothing here describes the present")
	}
	res.ReadAt, res.Current = read.TakenAt, true

	bindings, err := listBindings(ctx, target)
	if err != nil {
		return res, fmt.Errorf("read bindings for %s: %w", target, err)
	}
	res.Bound = len(bindings)

	// The inventory concludes from accounts actually SEEN, which is why a
	// truncated read does not invalidate it: everything reported here is on the
	// target. What a truncated read cannot support is the opposite conclusion —
	// that a bound subject has no account — and nothing here draws it. The
	// add-on refuses that one itself, at the point the cap is known.
	res.Unmanaged = unmanaged(read.Accounts, bindings)
	res.Truncated = read.Truncated

	// Before the convergences, because it is a statement about the record of
	// everything the add-on has already done — and because if the log has been
	// trimmed, that is what an operator needs to see first.
	res.LogVerdict = anchorLog(ctx, target)

	queued, convergeErr := convergeBound(ctx, target, bindings)
	res.Queued = queued

	reason := ""
	switch {
	case convergeErr != nil:
		// The picture is current and the record of it is not. Same operator
		// consequence as any other unrecorded finding: the surface understates
		// what the pass saw.
		log.Printf("[ADDON-RECONCILE] %s: %v", target, convergeErr)
		reason = db.UnreconciledFindingsUnrecorded
	case read.Truncated:
		reason = db.UnreconciledTruncated
	}
	if reason != "" {
		if _, err := markUnreconciled(ctx, target, reason); err != nil {
			log.Printf("[ADDON-RECONCILE] could not record %s as unreconciled: %v (non-fatal)", target, err)
		}
		res.Reason = reason
		return res, nil
	}

	// A complete, current read that was fully consumed. Recording the currency
	// and ending any unreconciled period are one write: a target read but still
	// flagged, or a flag cleared with no read behind it, both describe a moment
	// that did not happen.
	if _, err := markReconciled(ctx, target); err != nil {
		log.Printf("[ADDON-RECONCILE] could not record %s as reconciled: %v (non-fatal)", target, err)
	}
	return res, nil
}

// halt records why the pass concluded nothing and returns it saying so.
//
// Recording is non-fatal on purpose: the pass already failed, and failing to
// write down that it failed must not turn into a second, different failure the
// caller has to handle.
func halt(ctx context.Context, res AddonReconcileResult, reason, detail string) (AddonReconcileResult, error) {
	if _, err := markUnreconciled(ctx, res.Target, reason); err != nil {
		log.Printf("[ADDON-RECONCILE] could not record %s as unreconciled: %v (non-fatal)", res.Target, err)
	}
	res.Halted, res.Reason = true, reason
	log.Printf("[ADDON-RECONCILE] %s halted: %s", res.Target, detail)
	return res, nil
}

// unmanaged is every account on the target that no binding claims.
//
// Matched on the uid first and the name second, in that order and for the reason
// design §11 gives: a rename moves the name and leaves the uid, so matching by
// name alone would report a renamed member's own account as unmanaged and invite
// an operator to adopt it for somebody else.
func unmanaged(accounts []addons.TargetAccount, bindings []db.TargetBinding) []UnmanagedAccount {
	byUID := make(map[int64]struct{}, len(bindings))
	byName := make(map[string]struct{}, len(bindings))
	for _, b := range bindings {
		if b.AccountUID != nil {
			byUID[*b.AccountUID] = struct{}{}
		}
		byName[b.Username] = struct{}{}
	}

	out := make([]UnmanagedAccount, 0)
	for _, a := range accounts {
		if _, bound := byUID[a.UID]; bound && a.UID != 0 {
			continue
		}
		if _, bound := byName[a.Username]; bound {
			continue
		}
		out = append(out, UnmanagedAccount{Username: a.Username, UID: a.UID})
	}
	return out
}

// convergeBound asks the target what each managed subject is missing, and queues
// the convergence for the ones that are.
//
// One `/plan` call for the whole cohort, because planning forty subjects must
// not be forty state reads through a single rate-limited WebSocket.
//
// Only `apply` effects are queued. `no_change` is the target already holding
// what Syndra decided, which is the outcome this exists to confirm; `blocked` is
// something an operator has to decide — a binding conflict, most often — and
// queueing a convergence for it would be the sweep inferring a decision it is
// specifically forbidden from inferring.
func convergeBound(ctx context.Context, target string, bindings []db.TargetBinding) (int, error) {
	if len(bindings) == 0 {
		return 0, nil
	}

	subjects := make([]addons.PlanSubject, 0, len(bindings))
	desired := make(map[string]map[string]json.RawMessage, len(bindings))
	for _, b := range bindings {
		set, err := resolveIntent(ctx, b.SubjectID, target)
		if err != nil {
			return 0, fmt.Errorf("resolve %s: %w", b.SubjectID, err)
		}
		desired[b.SubjectID] = set
		subjects = append(subjects, addons.PlanSubject{Subject: b.SubjectID, Desired: set})
	}

	// Acknowledged, because this cohort is not an operator's selection — it is
	// every subject the backend already manages on this target, and there is
	// nobody to ask.
	answer := addonPlan(ctx, target, subjects, true)
	if answer.Outcome != addons.OutcomeSucceeded {
		return 0, fmt.Errorf("the target could not say what would change: %v", answer.Err)
	}

	queued := 0
	for _, out := range answer.Outcomes {
		if out.Effect != db.PlanEffectApply {
			continue
		}
		set, resolved := desired[out.Subject]
		if !resolved {
			// An outcome for a subject this pass did not ask about. Skipped
			// rather than converged from an empty set, which would remove every
			// entitlement they have.
			log.Printf("[ADDON-RECONCILE] %s answered for %s, which was not in the request", target, out.Subject)
			continue
		}
		if _, _, err := recordConvergence(ctx, db.SystemConvergence{
			Target: target, SubjectID: out.Subject, Actor: reconcileActor,
			Reason:  "Periodic reconciliation found the target out of step",
			Desired: set,
		}); err != nil {
			return queued, fmt.Errorf("queue convergence for %s: %w", out.Subject, err)
		}
		queued++
	}
	return queued, nil
}

// reconcileActor is who a reconciliation-sourced convergence is attributed to.
//
// A clock rather than a person, and named rather than left empty: the audit row
// asks who decided this, and "nobody, it was a sweep" is a real answer that the
// empty string does not give.
const reconcileActor = "system:reconcile"

// anchorLog reads the add-on's chain head and compares it against what the
// backend remembers.
//
// Non-fatal in both directions, and deliberately so. A health read that failed
// says nothing about the log — the pass has no evidence either way — and a
// violation must not stop the convergences, because access being correct and the
// forensic record being intact are two separate promises and neither is a reason
// to break the other.
//
// What a violation IS is loud: it means records that existed are gone, or the
// same number of them now hash to something else. The row carries it for the
// surface; this line carries it for whoever is reading logs at the time.
func anchorLog(ctx context.Context, target string) string {
	health := addonHealth(ctx, target)
	// A health read that did not succeed is no evidence either way, and the
	// pass has nothing to compare. This is the only condition that skips.
	//
	// An empty LOG HEAD used to skip too, and that was the hole: the cheapest
	// way to destroy the record is to delete the file, which reports no head
	// and no records — so the one tampering that needs no skill at all was the
	// one reading as "nothing to anchor". Against an anchor that remembers
	// records, an empty head is a truncation and is classified as one; against
	// no anchor at all it is refused below, where it is genuinely unanchorable.
	if health.Outcome != addons.OutcomeSucceeded {
		return ""
	}
	_, verdict, err := anchorLogHead(ctx, target, health.LogHead, health.LogRecords)
	if err != nil {
		log.Printf("[ADDON-RECONCILE] could not anchor %s's log: %v (non-fatal)", target, err)
		return ""
	}
	if db.AnchorViolation(verdict) {
		log.Printf("[ADDON-RECONCILE] SECURITY: %s reported a mutation log that is not an extension of the anchored one (%s)",
			target, verdict)
	}
	return verdict
}
