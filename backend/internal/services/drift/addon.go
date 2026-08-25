package drift

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"syndra/internal/addons"
	"syndra/internal/db"
	"syndra/internal/services/merge"
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
	// Self marks the add-on's own service account.
	//
	// It is genuinely unmanaged and genuinely on the target, so it belongs in
	// this list — but it is the one account whose deletion removes Syndra's
	// access to the target altogether, and the add-on refuses to adopt or purge
	// it whatever any caller asks. Carried so a surface can say that before an
	// operator meets the refusal, and so the row is never offered as an action.
	Self bool `json:"self,omitempty"`
	// PasswordSet is whether the account has a usable credential. An account
	// with none cannot have SMB enabled, whatever entitlement it is given.
	PasswordSet bool `json:"password_set"`
}

// AddonReconcileResult is what one pass saw and did.
type AddonReconcileResult struct {
	Target string `json:"target"`
	// Bound is how many subjects the backend has a recorded account for. The
	// denominator for everything else here.
	Bound int `json:"bound"`
	// Unmanaged is the inventory — reported, not triaged.
	Unmanaged []UnmanagedAccount `json:"unmanaged"`
	// Accounts is the other half of "whose accounts are on this target", and it
	// was missing: the inventory answered only which accounts Syndra does NOT
	// manage, so the ones it does had no surface at all — and they are the ones
	// an operator acts on. Read on the INVENTORY path only, because a reconcile
	// pass is a background sweep whose result nobody reads as a list.
	Accounts []db.TargetBinding `json:"accounts,omitempty"`
	// Queued counts the convergences this pass enqueued. Never "fixed": they
	// wait for the drain like every other add-on row.
	Queued int `json:"queued"`
	// Findings are the differences this pass may not resolve: a value the
	// target moved and Syndra did not, or one both moved differently. Carried
	// on the result for the caller that renders a pass, and persisted
	// separately, because a finding that lives only in the return value of the
	// sweep that found it is visible to whoever ran that sweep and to nobody
	// else.
	Findings []merge.SubjectFinding `json:"findings,omitempty"`
	// Stale are bindings whose account is no longer on the target. Reported and
	// NOT queued: the plan for one of these says "create", and a sweep that
	// acted on it would recreate an account somebody deleted. Which way to
	// resolve it — re-provision or unbind — is an operator's decision.
	Stale []StaleBinding `json:"stale,omitempty"`
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
	res.Accounts = bindings
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

	// A binding whose account is not on the target is not a convergence, it is
	// a finding.
	//
	// The plan for one says "create", and queueing that recreates an account
	// somebody deleted — or, as happened here, creates accounts that only ever
	// existed against a test stub. Three stub-era bindings sat in a live
	// deployment pointed at a production NAS, re-queueing every six hours, and
	// the only thing that had kept them from landing was an unrelated bug in
	// account creation. Fixing that bug turned them into three real accounts
	// waiting for somebody to press a button.
	//
	// So a binding is converged only while the account it names is still there.
	// The rest are separated out and reported, because "this binding points at
	// nothing" is an operator's decision — re-provision, or unbind — and not one
	// a sweep may take by writing.
	live, stale := partitionByPresence(bindings, read.Accounts)
	res.Stale = stale

	unrecorded := 0
	queued, findings, convergeErr := convergeBound(ctx, target, live, read.Accounts, &unrecorded)
	res.Queued = queued
	// The absent accounts are findings of the same kind, reached by a different
	// route: `deleted_upstream` is one of the outcomes, and reporting it beside
	// the field-level ones is what stops it being a special case with its own
	// vocabulary. `Stale` stays for the surfaces already built on it.
	//
	// PERSISTED, like every other finding. Returned-only, it lived for exactly
	// as long as the HTTP response that carried it: gone on refresh, absent
	// from the target's decision queue, uncounted by governance — which is the
	// failure this whole table exists to prevent, and it was reintroduced on the
	// one outcome that names a deleted account.
	for _, b := range stale {
		absent := merge.Absent(b.SubjectID).Findings()
		findings = append(findings, absent...)
		unrecorded += persistFindings(ctx, target, merge.Absent(b.SubjectID), absent)
	}
	res.Findings = findings

	reason := ""
	switch {
	case unrecorded > 0:
		// A pass that could not write down what it found has not reconciled the
		// target, whatever its read managed. The same rule the Zitadel sweep
		// applies to its own write failures, and it was missing here: the
		// failures were logged and the target was then marked reconciled, so the
		// surface reported a clean pass over findings nobody would ever see.
		log.Printf("[ADDON-RECONCILE] %s: %d finding(s) could not be recorded", target, unrecorded)
		reason = db.UnreconciledFindingsUnrecorded
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
		out = append(out, UnmanagedAccount{
			Username: a.Username, UID: a.UID,
			Self:        a.Self,
			PasswordSet: a.PasswordSet,
		})
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
func convergeBound(ctx context.Context, target string, bindings []db.TargetBinding,
	accounts []addons.TargetAccount, unrecorded *int) (int, []merge.SubjectFinding, error) {
	findings := make([]merge.SubjectFinding, 0)
	if len(bindings) == 0 {
		return 0, findings, nil
	}

	// The third state, read once for the whole cohort. A read that fails is not
	// a reason to stop: every subject then classifies as having no base, which
	// converges exactly as this sweep did before the mechanism existed. Failing
	// the pass instead would make a table this system can rebuild from its next
	// applies into a hard dependency of reconciliation.
	bases, err := listMergeBases(ctx, target)
	if err != nil {
		log.Printf("[ADDON-RECONCILE] %s: merge bases unavailable, classifying without them: %v", target, err)
		bases = map[string]db.MergeBase{}
	}
	current := stateByBinding(accounts)

	subjects := make([]addons.PlanSubject, 0, len(bindings))
	desired := make(map[string]map[string]json.RawMessage, len(bindings))
	for _, b := range bindings {
		set, err := resolveIntent(ctx, b.SubjectID, target)
		if err != nil {
			return 0, findings, fmt.Errorf("resolve %s: %w", b.SubjectID, err)
		}

		// The classification, before anything is planned. A subject whose
		// difference this pass may not resolve is never asked about: planning it
		// would produce an `apply` effect, and the loop below queues those.
		theirs := current.of(b)
		base := baseOf(bases, b.SubjectID)
		if theirs == nil {
			// The add-on reported no current state for this account — an add-on
			// older than the merge, or a read that carried none. Nothing can be
			// compared, so nothing is attributed: the base is dropped and the
			// subject classifies as baseless, which converges exactly as this
			// sweep did before.
			//
			// Keeping the base here would be worse than useless. Every managed
			// field would read as "the target no longer reports it", and a
			// deployment mid-upgrade would raise a finding for every subject it
			// manages — the manufactured-finding failure, arriving through a
			// version skew nobody chose.
			base = nil
		}
		state := merge.Classify(b.SubjectID, set, theirs, base)
		found := state.Findings()
		findings = append(findings, found...)
		*unrecorded += persistFindings(ctx, target, state, found)
		if !state.Convergeable() {
			continue
		}
		// The observation, for the fields this pass could account for. Recorded
		// by the SWEEP as well as by the apply, because `already merged` writes
		// nothing to the target and would otherwise be re-detected forever: a
		// hand-made change that matched Syndra's intent would be reported as an
		// agreement on every pass until something else happened to that subject.
		//
		// Only for a subject with nothing outstanding. Advancing a base past a
		// difference nobody has resolved would make the next pass read the
		// target's current state as the last agreed one — the silent revert,
		// through the bookkeeping.
		if len(found) == 0 && theirs != nil {
			observeState(ctx, target, b.SubjectID, set, theirs)
		}

		desired[b.SubjectID] = set
		subjects = append(subjects, addons.PlanSubject{Subject: b.SubjectID, Desired: set})
	}
	if len(subjects) == 0 {
		return 0, findings, nil
	}

	// Acknowledged, because this cohort is not an operator's selection — it is
	// every subject the backend already manages on this target, and there is
	// nobody to ask.
	answer := addonPlan(ctx, target, subjects, true)
	if answer.Outcome != addons.OutcomeSucceeded {
		return 0, findings, fmt.Errorf("the target could not say what would change: %v", answer.Err)
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
			return queued, findings, fmt.Errorf("queue convergence for %s: %w", out.Subject, err)
		}
		queued++
	}
	return queued, findings, nil
}

// persistFindings makes this pass's unresolvable differences outlive it, and
// closes the ones that have stopped existing.
//
// Both directions matter. A finding left as sweep output is visible to whoever
// ran the sweep and to nobody else; a finding left standing after its difference
// is gone fills the queue with problems that are already over. Neither is a
// resolution — the first is a record, the second is the record ending.
//
// Failures are logged and counted as unrecorded findings by the caller's own
// reason, never fatal: a pass that could not write one finding must still write
// the rest.
func persistFindings(ctx context.Context, target string, state merge.Subject, found []merge.SubjectFinding) int {
	failed := 0
	open := make(map[string]bool, len(found))
	for _, f := range found {
		open[f.Field] = true
		if err := saveMergeFinding(ctx, db.MergeFinding{
			Target: target, SubjectID: f.SubjectID, Field: f.Field,
			Outcome: string(f.Outcome), Base: f.Base, Ours: f.Ours, Theirs: f.Theirs,
		}); err != nil {
			log.Printf("[ADDON-RECONCILE] could not record a %s finding for %s on %s: %v",
				f.Outcome, f.SubjectID, target, err)
			failed++
		}
	}
	if state.Absent {
		// Nothing to close. An absent account has no fields, and the loop below
		// would clear nothing — but running it would also clear the
		// account-level finding this call just raised.
		return failed
	}
	// The account is present, so whatever said it was gone is over. Cleared
	// here rather than by any field: `deleted_upstream` occupies the empty-field
	// slot, and the loop below only ever names real fields.
	if err := clearMergeFinding(ctx, target, state.SubjectID, "", reconcileActor); err != nil {
		log.Printf("[ADDON-RECONCILE] could not close a settled deleted-upstream finding for %s on %s: %v",
			state.SubjectID, target, err)
		failed++
	}
	// Every managed field this pass could account for closes whatever was
	// standing against it. The classifier has just said the two sides agree, or
	// that Syndra alone moved — in either case the disagreement recorded earlier
	// is over.
	for _, f := range state.Fields {
		if open[f.Field] {
			continue
		}
		if err := clearMergeFinding(ctx, target, state.SubjectID, f.Field, reconcileActor); err != nil {
			log.Printf("[ADDON-RECONCILE] could not close a settled finding for %s on %s: %v",
				state.SubjectID, target, err)
			failed++
		}
	}
	return failed
}

// observeState records what the target was seen holding, for the managed fields.
//
// Managed only, matching what the apply's read-back records: an unmanaged field
// is out of scope, and a base claiming authority over one would raise findings
// about values nobody here decided.
func observeState(ctx context.Context, target, subjectID string,
	managed, theirs map[string]json.RawMessage) {
	observed := make(map[string]json.RawMessage, len(managed))
	for field := range managed {
		if value, seen := theirs[field]; seen {
			observed[field] = value
		}
	}
	if len(observed) == 0 {
		return
	}
	if err := saveMergeBase(ctx, db.MergeBase{
		Target: target, SubjectID: subjectID, Base: observed,
	}); err != nil {
		log.Printf("[ADDON-RECONCILE] could not record what %s was seen holding on %s: %v",
			subjectID, target, err)
	}
}

// accountState indexes the target's current values by the identity a binding is
// matched on.
//
// UID first and name second, in that order and for the reason every other match
// in this file uses it: a rename moves the name and leaves the uid, and matching
// by name alone would compare one member's binding against another account.
type accountState struct {
	byUID  map[int64]map[string]json.RawMessage
	byName map[string]map[string]json.RawMessage
}

func stateByBinding(accounts []addons.TargetAccount) accountState {
	out := accountState{
		byUID:  make(map[int64]map[string]json.RawMessage, len(accounts)),
		byName: make(map[string]map[string]json.RawMessage, len(accounts)),
	}
	for _, a := range accounts {
		if a.State == nil {
			continue
		}
		if a.UID != 0 {
			out.byUID[a.UID] = a.State
		}
		out.byName[a.Username] = a.State
	}
	return out
}

func (s accountState) of(b db.TargetBinding) map[string]json.RawMessage {
	if b.AccountUID != nil {
		if state, ok := s.byUID[*b.AccountUID]; ok {
			return state
		}
	}
	return s.byName[b.Username]
}

// baseOf is one subject's last observed state, or nil.
//
// Nil is a legitimate answer and never an error: every binding made before this
// mechanism existed has none, and the classifier treats that as "no cause can be
// determined", which converges as before.
func baseOf(bases map[string]db.MergeBase, subjectID string) map[string]json.RawMessage {
	base, found := bases[subjectID]
	if !found {
		return nil
	}
	return base.Base
}

// partitionByPresence splits bindings into those whose account is still on the
// target and those whose is not.
//
// Matched on UID first and username second, in that order and for the same
// reason the binding records both: a rename keeps the uid, and a recreated
// account keeps the name. A binding that matches on neither names an account
// that is not there.
//
// A binding with no recorded uid is treated as present when the name matches,
// because the alternative — calling it stale — would report every
// operator-adopted binding from before uids were recorded as broken.
func partitionByPresence(bindings []db.TargetBinding, accounts []addons.TargetAccount) (live []db.TargetBinding, stale []StaleBinding) {
	byUID := make(map[int64]struct{}, len(accounts))
	byName := make(map[string]struct{}, len(accounts))
	for _, a := range accounts {
		byUID[a.UID] = struct{}{}
		byName[a.Username] = struct{}{}
	}
	for _, b := range bindings {
		if b.AccountUID != nil {
			if _, ok := byUID[*b.AccountUID]; ok {
				live = append(live, b)
				continue
			}
		}
		if _, ok := byName[b.Username]; ok {
			live = append(live, b)
			continue
		}
		uid := int64(0)
		if b.AccountUID != nil {
			uid = *b.AccountUID
		}
		stale = append(stale, StaleBinding{SubjectID: b.SubjectID, Username: b.Username, UID: uid})
	}
	return live, stale
}

// StaleBinding is a managed subject whose account is no longer on the target.
type StaleBinding struct {
	SubjectID string `json:"subject_id"`
	Username  string `json:"username"`
	// UID is the account the binding recorded, or 0 for a binding made before
	// uids were kept.
	UID int64 `json:"uid,omitempty"`
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
