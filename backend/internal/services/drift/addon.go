package drift

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"maps"
	"sort"
	"time"

	"syndra/internal/addons"
	"syndra/internal/db"
	"syndra/internal/services"
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
	Username  string `json:"username"`
	AccountID string `json:"account_id,omitempty"`
	// Self marks the add-on's own service account.
	//
	// It is genuinely unmanaged and genuinely on the target, so it belongs in
	// this list — but it is the one account whose deletion removes Syndra's
	// access to the target altogether, and the add-on refuses to adopt or purge
	// it whatever any caller asks. Carried so a surface can say that before an
	// operator meets the refusal, and so the row is never offered as an action.
	Self bool `json:"self,omitempty"`
	// Standing is whether the account can be used yet and who is waited on,
	// in the add-on's words (contract 2, design §6).
	Standing *addons.Standing `json:"standing,omitempty"`
}

// BoundAccount is a managed account on the inventory: its binding, and the
// standing the add-on reports for it in the same read (contract 2, design §6),
// absent when the read does not list it.
type BoundAccount struct {
	db.TargetBinding
	Standing *addons.Standing `json:"standing,omitempty"`
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
	Accounts []BoundAccount `json:"accounts,omitempty"`
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
	// ReadStale is why the read is not current, as the add-on said it
	// (cached, cooldown, unreachable, ...): the inventory's "Read again" line.
	ReadStale *addons.Stale `json:"read_stale,omitempty"`
	// RetryAfter is a fresh read the add-on refused inside its cooldown, with
	// nothing to serve or with its copy: halted, readable again in this many
	// seconds (RFC 9110 §10.2.3). "Check now" waits them out and runs once
	// more (the UI's one retry, 7l).
	RetryAfter int `json:"retry_after,omitempty"`
	// LogVerdict is what the add-on's mutation-log head said when compared
	// against the anchor. Carried on the reconcile result because this is the
	// pass that reads it, and because a finding that lives only in a log line is
	// a finding nobody is looking at.
	LogVerdict string `json:"log_verdict,omitempty"`
	// Halted says the pass stopped before concluding anything, with Reason
	// naming which of the target's failures it was.
	Halted bool   `json:"halted,omitempty"`
	Reason string `json:"reason,omitempty"`
	// Detail is the add-on's own sentence for a read it refused (Reason
	// read_refused), with its next step, shown as written: "Syndra's records
	// of who has which account on TrueNAS can't be read. Check the add-on's
	// storage, then try again." Empty when it gave none.
	Detail string `json:"detail,omitempty"`
}

// Inventory is the read-only half: what lives on the target and which of it
// Syndra manages, with nothing queued and nothing recorded.
//
// Separate from the reconcile because an operator opening a page must not queue
// convergences by looking at it — and because the inventory is the one thing
// here that is still worth showing when the read is stale or capped, labelled
// with which it was.
//
// fresh is a person pressing "Read again" (feature freshness); a page load
// accepts the add-on's copy, which is shown dated and, not being current,
// adopted from by nobody until it is read again (owner Q2).
func Inventory(ctx context.Context, target string, fresh bool) (AddonReconcileResult, error) {
	if target == db.TargetZitadel {
		return AddonReconcileResult{}, fmt.Errorf("inventory: %s holds no accounts of its own", target)
	}
	res := AddonReconcileResult{Target: target}

	read := addonSubjects(ctx, target, fresh)
	// The one mapping (SubjectsResult.Halt) decides, cooldown included: an
	// outdated add-on stays outdated inside its cooldown.
	if reason, detail, halted := read.Halt(); halted {
		res.Halted, res.Reason, res.Detail = true, reason, detail
		if reason == db.UnreconciledStaleRead {
			res.RetryAfter = read.RetryAfter // the cooldown's wait, and only its
		}
		return res, nil
	}
	bindings, err := listBindings(ctx, target)
	if err != nil {
		return res, fmt.Errorf("read bindings for %s: %w", target, err)
	}
	res.Bound = len(bindings)
	// Each recorded binding's account is the row the add-on attributes to its
	// subject (design 7t D3), never one matched here by name or number.
	standing := map[string]*addons.Standing{}
	for _, a := range read.Accounts {
		if a.Subject != "" {
			standing[a.Subject] = a.Standing
		}
	}
	for _, b := range bindings {
		res.Accounts = append(res.Accounts, BoundAccount{TargetBinding: b, Standing: standing[b.SubjectID]})
	}
	res.Unmanaged = unmanaged(read.Accounts)
	res.ReadAt, res.Current, res.Truncated, res.ReadStale = read.TakenAt, read.Current, read.Truncated, read.Stale
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

// sweepWaitCeiling is the longest the sweep waits out a cooldown, whatever a
// target declares or asks: twice constants.cooldown_default (PT30S,
// registry/constants.json), so a target declaring a day (P1D or PT24H alike)
// holds a pass back a minute, not a day.
const sweepWaitCeiling = 60 * time.Second

// sweepWait is how long the sweep waits out a cooldown answer: the
// retry_after it was given (RFC 9110 §10.2.3), the declared cooldown when it
// was given none, never longer than the declared cooldown or the ceiling.
func sweepWait(target string, retryAfter int) time.Duration {
	wait := freshCooldown(target)
	if retryAfter > 0 {
		wait = min(wait, time.Duration(retryAfter)*time.Second)
	}
	return min(wait, sweepWaitCeiling)
}

// ReconcileAddon is the scheduled sweep of a target: it reads it, converges
// the subjects it manages, and reports the accounts it does not.
func ReconcileAddon(ctx context.Context, target string) (AddonReconcileResult, error) {
	return reconcileAddon(ctx, target, true)
}

// ReconcileAddonNow is the same pass for a person waiting on it ("Check
// now"): it never waits out a cooldown inside their request, and answers it as
// a skipped pass that says when the target can be read again. Two entry
// points rather than a flag: who calls decides, and neither call site carries
// a bare true or false.
func ReconcileAddonNow(ctx context.Context, target string) (AddonReconcileResult, error) {
	return reconcileAddon(ctx, target, false)
}

// reconcileAddon is the pass; patient is whether a cooldown answer is waited
// out once (the scheduled sweep) or answered at once (a person).
func reconcileAddon(ctx context.Context, target string, patient bool) (AddonReconcileResult, error) {
	if target == db.TargetZitadel {
		// Zitadel has its own sweep, which pages the Management API and
		// concludes differently. Refused rather than ignored: a caller here with
		// the built-in target has a wrong model of which reconciler does what.
		return AddonReconcileResult{}, fmt.Errorf("reconcile add-on: %s is the built-in target", target)
	}
	res := AddonReconcileResult{Target: target}

	// Always fresh: the sweep is the one scheduled sender (design §4) and it
	// needs a current read. Answered inside the cooldown, the scheduled sweep
	// waits sweepWait and asks once more; a second cooldown answer is a
	// skipped pass, recorded as a stale read (no new reason, no migration;
	// B6), never as unreachable. One wait, not a loop: someone else reading
	// the target that often is not the sweep's to outlast.
	//
	// readBegan is taken before the first read, so it is never later than the
	// read concluded from: what was in flight since then (appliesInFlight)
	// covers every apply that could have landed mid-read. On the database's
	// clock, which the outbox's started_at and completed_at are written on.
	readBegan, err := databaseNow(ctx)
	if err != nil {
		return res, err
	}
	read := addonSubjects(ctx, target, true)
	if retry, cooled := read.CooledDown(); cooled && patient && waitOut(ctx, sweepWait(target, retry)) {
		read = addonSubjects(ctx, target, true)
	}
	// The one mapping (SubjectsResult.Halt) first, so a cooldown never masks
	// what it says (an outdated add-on, a store it cannot read); a refusal
	// inside the cooldown is its read_stale, with the wait.
	wait, cooled := read.CooledDown()
	if reason, detail, halted := read.Halt(); halted {
		res.Detail = detail
		if reason == db.UnreconciledStaleRead {
			res.RetryAfter = max(1, wait) // the cooldown's wait, and only its
		}
		return halt(ctx, res, reason, fmt.Sprintf("the target's read concludes nothing (%s): %v", reason, read.Err))
	}
	if cooled {
		// Answered with its copy inside the cooldown.
		res.RetryAfter = max(1, wait)
		return halt(ctx, res, db.UnreconciledStaleRead,
			fmt.Sprintf("skipped: the target was read moments ago and can be read again in %d s", wait))
	}
	switch {
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
	// that a bound subject has no account — and the partition below draws it
	// only from a whole read: a truncated one yields no absence findings.
	res.Unmanaged = unmanaged(read.Accounts)
	res.Truncated = read.Truncated
	// Asked after the read, of everything since it began: an apply that landed
	// between the add-on's read of the app and its read of its bindings leaves
	// its subject on no row and in no conflict, and is not a deleted account.
	inFlight, err := appliesInFlight(ctx, target, readBegan, applyWindow())
	if err != nil {
		return res, fmt.Errorf("read applies in flight on %s: %w", target, err)
	}

	// Before the convergences, because it is a statement about the record of
	// everything the add-on has already done — and because if the log has been
	// trimmed, that is what an operator needs to see first.
	res.LogVerdict = anchorLog(ctx, target)

	// The add-on's conflicts (design 7t §3): a binding that names no account
	// because another account holds its name or number. Raised before anything
	// is classified or planned, and never planned: its plan is a create the
	// add-on refuses, and read as a missing row it was once a deleted_upstream
	// offered a re-create. Every one the add-on reports, recorded here or not:
	// which subject a binding is for is the add-on's to say.
	unrecorded := 0
	conflicted, findings := raiseConflicts(ctx, target, read.Conflicts, !read.Truncated, inFlight, &unrecorded)

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
	live, stale := partitionByPresence(bindings, read.Accounts, conflicted, inFlight)
	if read.Truncated {
		// The cap cut the list, so a binding missing from it may simply be past
		// the cut. Absence is only ever concluded from a whole read; those
		// bindings are neither converged nor reported this pass.
		stale = nil
	}
	res.Stale = stale

	queued, bound, convergeErr := convergeBound(ctx, target, live, read.Accounts, &unrecorded)
	findings = append(findings, bound...)
	res.Queued = queued
	// And the people this pass would otherwise never see: they hold a mapped
	// role and have no account here yet. A path that skipped the cascade — or
	// one that failed to queue after its grant committed — is healed here
	// within one pass, rather than waiting for an operator to apply by hand.
	if convergeErr == nil {
		created, err := convergeUnbound(ctx, target, bindings, conflicted)
		res.Queued += created
		convergeErr = err
	}
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

// raiseConflicts records each conflict the add-on reported as a standing
// binding_conflict finding in its subject's account-level slot, carrying the
// add-on's report (reason, holder, detail, next) as theirs, and returns the
// subjects it names: they are neither planned nor concluded absent this pass.
//
// Refreshed while it stands (one row per subject, RecordMergeFinding). On a
// whole read, every standing binding_conflict whose subject this read does
// not list is closed (`agreed`, or what was decided): the add-on stopped
// reporting it, whether or not the subject has a binding recorded here.
// Before the stale loop, so a conflict that ended with the account gone is
// closed and then raised again as a new deleted_upstream. Never a subject
// with an apply in flight since the read began (inFlight): the drain may have
// raised its conflict after the add-on read, and this read says nothing of it.
func raiseConflicts(ctx context.Context, target string, conflicts []addons.SubjectConflict,
	whole bool, inFlight map[string]bool, unrecorded *int) (map[string]bool, []merge.SubjectFinding) {
	conflicted := make(map[string]bool, len(conflicts))
	findings := make([]merge.SubjectFinding, 0, len(conflicts))
	if whole {
		keep := make([]string, 0, len(conflicts)+len(inFlight))
		for _, c := range conflicts {
			keep = append(keep, c.Subject)
		}
		for subject := range inFlight {
			keep = append(keep, subject)
		}
		sort.Strings(keep)
		if _, err := clearFindingsOutside(ctx, target, string(merge.BindingConflict), keep, reconcileActor); err != nil {
			log.Printf("[ADDON-RECONCILE] %s: could not close the conflicts it no longer reports: %v", target, err)
			*unrecorded++
		}
	}
	for _, c := range conflicts {
		conflicted[c.Subject] = true
		report, err := json.Marshal(c)
		if err != nil {
			log.Printf("[ADDON-RECONCILE] %s: the conflict for %s could not be encoded: %v", target, c.Subject, err)
			*unrecorded++
			continue
		}
		state := merge.Conflicted(c.Subject, report)
		found := state.Findings()
		findings = append(findings, found...)
		*unrecorded += persistFindings(ctx, target, state, found)
	}
	return conflicted, findings
}

// unmanaged is every account on the target that no binding claims: the rows
// the add-on attributes to no subject (design 7t D3).
//
// The add-on holds the bindings and says whose each account is; matching rows
// here against the backend's own copy by name or number is what let a reused
// number hand one person's binding to a stranger's account.
func unmanaged(accounts []addons.TargetAccount) []UnmanagedAccount {
	out := make([]UnmanagedAccount, 0)
	for _, a := range accounts {
		if a.Subject != "" {
			continue
		}
		out = append(out, UnmanagedAccount{
			Username: a.Username, AccountID: a.AccountID,
			Self:     a.Self,
			Standing: a.Standing,
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
	// The fields' roles, as last accepted (targets.field_roles). Without them
	// a limit could not be told from a membership, so the pass stops rather
	// than compare one it must not (comparable). The schema says how a limit
	// combines; without it no limit is compared.
	roles, err := fieldRoles(ctx, target)
	if err != nil {
		return 0, findings, fmt.Errorf("the target's field roles could not be read: %w", err)
	}
	schema, _ := entitlementSchema(target)
	current := statesBySubject(accounts)

	subjects := make([]addons.PlanSubject, 0, len(bindings))
	desired := make(map[string]services.EntitlementSet, len(bindings))
	for _, b := range bindings {
		set, err := resolveIntent(ctx, b.SubjectID, target)
		if err != nil {
			return 0, findings, fmt.Errorf("resolve %s: %w", b.SubjectID, err)
		}
		intent := set.Desired() // the wire map: what the target is told

		// The classification, before anything is planned. A subject whose
		// difference this pass may not resolve is never asked about: planning it
		// would produce an `apply` effect, and the loop below queues those.
		theirs := current[b.SubjectID]
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
		// What is compared: the intent with each limit combined as the add-on
		// combines it (task 2.7), so a hand-edited limit is a finding, not a
		// silent revert. The wire map (intent) is still what is planned.
		compared := comparable(intent, theirs, roles, schema)
		state := merge.Classify(b.SubjectID, compared, theirs, base)
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
			observeState(ctx, target, b.SubjectID, compared, theirs, set.WithdrawsOnly())
		}

		desired[b.SubjectID] = set
		subjects = append(subjects, addons.PlanSubject{Subject: b.SubjectID, Desired: intent})
	}
	if len(subjects) == 0 {
		return 0, findings, nil
	}

	// Every subject the backend already manages on this target, which is no
	// operator's selection and may be more than one plan holds: Plan asks in
	// chunks below the add-on's limits.subjects_per_plan.
	answer := addonPlan(ctx, target, subjects)
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
		// One row in Pending changes per person and difference, as for the
		// unbound below. A waiting row that wants something else is not a
		// reason to skip: it would apply its older snapshot.
		waiting, err := pendingApply(ctx, target, out.Subject, set.Desired())
		if err != nil {
			return queued, findings, err
		}
		if waiting {
			continue
		}
		// What the target said would change, in its own words ("Joins Staff."),
		// so Pending changes can say what the row does.
		reason := out.Detail
		if reason == "" {
			reason = "Out of step with their roles."
		}
		// Built from the set, so a departed person's disable is declared
		// withdraw-only and the background runner sends it (EntitlementSet.WithdrawsOnly).
		if _, _, err := recordConvergence(ctx, set.Convergence(target, out.Subject, reconcileActor, reason)); err != nil {
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
	if state.AccountLevel() {
		// Nothing to close. An absent or conflicted account has no fields, and
		// the loop below would clear nothing — but running it would also clear
		// the account-level finding this call just raised.
		return failed
	}
	// The account is present and the add-on names it this subject's, so
	// whatever said it was gone or held by another is over. Cleared here
	// rather than by any field: `deleted_upstream` and `binding_conflict`
	// occupy the empty-field slot, and the loop below only names real fields.
	if err := clearMergeFinding(ctx, target, state.SubjectID, "", reconcileActor); err != nil {
		log.Printf("[ADDON-RECONCILE] could not close a settled account-level finding for %s on %s: %v",
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
	managed, theirs map[string]json.RawMessage, partial bool) {
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
		// A withdraw-only intent manages the lifecycle fields alone; the rest
		// of the base stands (MergeBase.Partial).
		Partial: partial,
	}); err != nil {
		log.Printf("[ADDON-RECONCILE] could not record what %s was seen holding on %s: %v",
			subjectID, target, err)
	}
}

// statesBySubject indexes each bound account's state, as the add-on reported
// it, by the subject the add-on attributes it to (design 7t D3). Never by
// account_id or name: a number TrueNAS hands out again would compare one
// member's intent against a stranger's account. A limit is the effective one
// the add-on holds (design §3); comparable puts the intent in the same terms.
func statesBySubject(accounts []addons.TargetAccount) map[string]map[string]json.RawMessage {
	out := make(map[string]map[string]json.RawMessage, len(accounts))
	for _, a := range accounts {
		if a.Subject != "" && a.State != nil {
			out[a.Subject] = a.State
		}
	}
	return out
}

// comparable is the intent in the terms the target reports its state in (task
// 2.7): each `limit` field the one effective limit, combined the way the
// add-on combines (EntitlementField.CombineLimit: max, unlimited dominant).
// A limit is left out, unknown and not compared (never theirs_only), when no
// value sets it (the add-on sets nothing), when the target did not report it
// (a quota table it could not read), or when its rule is not known (no
// accepted schema). Every other field is the intent's own value.
func comparable(intent, theirs map[string]json.RawMessage, roles addons.Roles, schema []addons.EntitlementField) map[string]json.RawMessage {
	out := make(map[string]json.RawMessage, len(intent))
	for field, value := range intent {
		if roles[field] != addons.RoleLimit {
			out[field] = value
			continue
		}
		if _, reported := theirs[field]; !reported {
			continue
		}
		var values []string
		if json.Unmarshal(value, &values) != nil {
			continue
		}
		for _, f := range schema {
			if f.Name != field {
				continue
			}
			if limit, ok := f.CombineLimit(values); ok {
				out[field], _ = json.Marshal([]string{limit})
			}
		}
	}
	return out
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

// partitionByPresence splits the recorded bindings by what the add-on said of
// each subject in this read (design 7t §2.1): bound (a row carries the
// subject) is live; in conflicts is neither, raised on its own
// (raiseConflicts); on no row and in no conflict is absent, a stale binding.
//
// Except a subject with an apply in flight while the target was read: the
// add-on reads the app before its bindings, so an apply landing between the
// two leaves its subject on no row for that one read. This read says nothing
// about it, so it is neither converged nor concluded absent.
//
// The add-on attributes; this side never matches a row to a binding by
// account_id or name (AUTHORING ch. 2): an id TrueNAS hands out again would
// call a stranger's account this member's, and the recorded name would call
// a renamed member's account gone.
func partitionByPresence(bindings []db.TargetBinding, accounts []addons.TargetAccount,
	conflicted, inFlight map[string]bool) (live []db.TargetBinding, stale []StaleBinding) {
	bound := make(map[string]bool, len(accounts))
	for _, a := range accounts {
		if a.Subject != "" {
			bound[a.Subject] = true
		}
	}
	for _, b := range bindings {
		switch {
		case conflicted[b.SubjectID]:
			// A finding of its own, first: a reply that both attributed a row
			// to the subject and listed it in conflict contradicts itself
			// (the contract forbids it), and the conflict is the safe reading.
		case bound[b.SubjectID]:
			live = append(live, b)
		case inFlight[b.SubjectID]:
			// Unknown this read.
		default:
			stale = append(stale, StaleBinding{SubjectID: b.SubjectID, Username: b.Username, AccountID: b.AccountID, Version: b.Version()})
		}
	}
	return live, stale
}

// StaleBinding is a managed subject whose account is no longer on the target.
type StaleBinding struct {
	SubjectID string `json:"subject_id"`
	Username  string `json:"username"`
	// AccountID is the account the binding recorded, absent for a binding made
	// before ids were kept.
	AccountID *string `json:"account_id,omitempty"`
	// Version is Syndra's record of the binding as this read saw it (db.TargetBinding.Version),
	// sent back with a release so a page left open cannot drop a binding made since.
	Version string `json:"version,omitempty"`
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

// convergeUnbound queues the first convergence for everyone who holds a role
// mapped to this target and has no binding on it. The apply creates the account.
//
// Not someone with an unsent convergence already waiting for the same thing:
// one row in Pending changes per person, not a new one every pass. Not someone whose resolved
// access is disabled (held, but suspended): an account made only to be switched
// off is a stranger's name on the NAS for nothing. Not someone the add-on
// reports in conflict (conflicted): their create is the one it refuses, and
// the finding already waits on an operator.
func convergeUnbound(ctx context.Context, target string, bindings []db.TargetBinding, conflicted map[string]bool) (int, error) {
	mappings, err := listMappings(ctx, target)
	if err != nil {
		return 0, fmt.Errorf("read mappings for %s: %w", target, err)
	}
	bound := make(map[string]bool, len(bindings)+len(conflicted))
	maps.Copy(bound, conflicted)
	for _, b := range bindings {
		bound[b.SubjectID] = true
	}
	seenRole := map[string]bool{}
	candidates := map[string]bool{}
	for _, m := range mappings {
		key := m.ProjectID + "\x00" + m.RoleKey
		if seenRole[key] {
			continue
		}
		seenRole[key] = true
		holders, err := mappingHolders(ctx, m.ProjectID, m.RoleKey)
		if err != nil {
			return 0, fmt.Errorf("read holders of %s/%s: %w", m.ProjectID, m.RoleKey, err)
		}
		for _, h := range holders {
			if !bound[h] {
				candidates[h] = true
			}
		}
	}

	ids := make([]string, 0, len(candidates))
	for id := range candidates {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	queued := 0
	for _, id := range ids {
		set, err := resolveIntent(ctx, id, target)
		if err != nil {
			return queued, fmt.Errorf("resolve %s: %w", id, err)
		}
		if !set.Lifecycle.Account {
			continue
		}
		waiting, err := pendingApply(ctx, target, id, set.Desired())
		if err != nil {
			return queued, err
		}
		if waiting {
			continue
		}
		if _, _, err := recordConvergence(ctx, set.Convergence(target, id, reconcileActor, "Gets an account, for a role they hold.")); err != nil {
			return queued, fmt.Errorf("queue convergence for %s: %w", id, err)
		}
		queued++
	}
	return queued, nil
}
