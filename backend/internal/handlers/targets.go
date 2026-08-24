package handlers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"syndra/internal/addons"
	"syndra/internal/db"
)

// The target roster and one target's health (change `addon-platform` 9.13,
// 9.20).
//
// The roster is DEPLOYMENT CONFIGURATION, not data. That distinction is the
// whole reason this endpoint exists rather than the navigation deriving targets
// from whatever the current operator happens to be able to see: an operator on a
// deployment running a TrueNAS add-on sees the TrueNAS entry whether or not it
// currently answers, whether or not anybody is bound to it, and whether or not
// they have permission to read a single account on it. Structure never moves in
// response to data (`basic-advanced-ia`), and a nav row that appeared when the
// first person was provisioned would be exactly that.
//
// So this lists what the deployment registered, and says separately how each one
// is doing. The two are never merged into a single "status", because "not
// deployed" and "deployed and down" are different sentences and only one of them
// is an incident.

type targetSummary struct {
	Target string `json:"target"`
	// Registered is always true here — it is what this list is — and is carried
	// so a client reading one target's payload does not have to infer it from
	// the row's presence.
	Registered bool `json:"registered"`
	// AuthMode is how the backend authenticates to it. One mode now — `derived`,
	// both keys from the per-target secret — and `none` for a target that
	// configured none, which does not register and therefore never appears here.
	// Rendered because an operator's model of their own trust boundary should
	// come from the deployment rather than from memory.
	AuthMode string `json:"auth_mode"`
	// TransportStatus is whether that secret still LOADS — `ok` or `error`.
	//
	// Separate from every other state on this row, and not derivable from them.
	// Registration reads the secret once at start-up; this reads it now. A mount
	// that was unmounted, a file emptied by a half-finished rotation, a `_FILE`
	// path that stopped resolving — each leaves a target that is registered, was
	// callable, and whose next call will fail at the handshake with an error
	// naming three causes it cannot tell apart. The whole value of this field is
	// arriving before that call does.
	TransportStatus string `json:"transport_status,omitempty"`
	// TransportError is why, when it is not ok.
	TransportError string `json:"transport_error,omitempty"`
	// Callable says a manifest has been read and understood. Registration alone
	// makes nothing callable, and the gap between the two is the state a fresh
	// deployment sits in before its first refresh.
	Callable bool `json:"callable"`
	// Operations is what the effective set offers — the manifest intersected
	// with backend policy — so a surface renders from the deployment's actual
	// capability rather than from a hardcoded list.
	Operations []operationSummary `json:"operations"`
	// ManifestAge dates the capability set. A stale answer is labelled with its
	// age rather than withheld: withdrawing operations because a refresh failed
	// would make an outage look like a capability change.
	ManifestFetchedAt *time.Time `json:"manifest_fetched_at,omitempty"`
	// CircuitOpen is the backend refusing its own calls to this add-on. A third
	// state beside registered and callable, and an operator seeing a target
	// listed but not answering should be able to see why without reading a log.
	CircuitOpen bool `json:"circuit_open"`
	// LastError is why the last manifest read failed, when one did.
	LastError string `json:"last_error,omitempty"`
}

type operationSummary struct {
	ID                string `json:"id"`
	Scope             string `json:"scope"`
	Confirm           bool   `json:"confirm"`
	Available         bool   `json:"available"`
	UnavailableReason string `json:"unavailable_reason,omitempty"`
	// SecretParams names the parameters whose values are never logged, stored or
	// echoed. Rendered so a form can mark them, and carried as NAMES only —
	// there is nowhere in this payload for a value.
	SecretParams []string `json:"secret_params,omitempty"`
}

// handleListTargets returns the deployment's add-on roster.
func handleListTargets(w http.ResponseWriter, r *http.Request) {
	registrations := addonsRegistered()
	// One pass for the whole roster rather than a load per row: each entry
	// re-reads a secret from disk, and this list is rendered on every visit to
	// the targets page.
	transport := map[string]addons.TransportCredential{}
	for _, tc := range addonsTransportCredentials() {
		transport[tc.Target] = tc
	}
	out := make([]targetSummary, 0, len(registrations))
	for _, reg := range registrations {
		out = append(out, describeTarget(reg, transport[reg.Target]))
	}
	jsonResponse(w, http.StatusOK, map[string]any{"targets": out})
}

// handleTargetHealth returns one add-on's own account of itself and its target.
//
// A live read rather than a cached opinion: reachability that reports the last
// known answer is the field most likely to be wrong exactly when it is read.
func handleTargetHealth(w http.ResponseWriter, r *http.Request) {
	target := r.PathValue("target")
	health := addonsHealth(r.Context(), target)

	// The anchor's finding, if it is carrying one, travels with the health of
	// the target it is about — and it travels whether or not the add-on is
	// answering right now. It was previously written to a table with no reader
	// at all: the sweep detected a truncated mutation log, recorded it
	// correctly, logged one line, and no surface in the system could report it.
	// A tamper-evidence mechanism nobody is told about has done half its job.
	anchor, anchored, err := dbGetLogAnchor(r.Context(), target)
	if err != nil {
		// Non-fatal. The health of the target is the question asked, and an
		// unavailable finding must not take the answer down with it — but it is
		// said out loud rather than rendered as "no finding".
		log.Printf("[TARGETS] could not read %s's log anchor: %v (non-fatal)", target, err)
	}

	if health.Outcome != addons.OutcomeSucceeded {
		// 200 with `reachable: false`, not an error status. The add-on being
		// unreachable is the answer to the question, and an error would make a
		// health surface unable to render the one state it exists to show.
		body := map[string]any{
			"target":    target,
			"reachable": false,
			"detail":    errText(health.Err),
		}
		if anchored && anchor.Compromised() {
			body["log_anchor"] = anchor
		}
		// Carried on the unreachable path too. A disagreement about who owns an
		// account is a fact about Syndra's own records, so it stands whether or
		// not the target is answering — and hiding it during an outage would
		// hide it exactly when somebody is looking at this page.
		if conflicts := bindingConflicts(r, target); len(conflicts) > 0 {
			body["binding_conflicts"] = conflicts
		}
		jsonResponse(w, http.StatusOK, body)
		return
	}

	conflicts := bindingConflicts(r, target)
	if !anchored && len(conflicts) == 0 {
		jsonResponse(w, http.StatusOK, health)
		return
	}
	// Merged rather than nested under a second request, because the operator's
	// question is one question: is this target all right.
	merged := targetHealthWithAnchor{TargetHealth: health, BindingConflicts: conflicts}
	if anchored {
		merged.LogAnchor = &anchor
	}
	jsonResponse(w, http.StatusOK, merged)
}

// bindingConflicts reads the standing findings for a target's health payload.
//
// Beside the log anchor rather than under a separate request, for the reason
// that one governs the other: both answer "does this target need a decision",
// and an operator asking that should not have to know there are two places it
// can be answered.
func bindingConflicts(r *http.Request, target string) []db.BindingConflict {
	conflicts, err := dbStandingBindingConflicts(r.Context(), target)
	if err != nil {
		// Non-fatal and said out loud, matching the anchor's rule: the health
		// of the target is the question asked, and an unavailable finding must
		// not take the answer down with it.
		log.Printf("[TARGETS] could not read %s's binding conflicts: %v (non-fatal)", target, err)
		return nil
	}
	return conflicts
}

// targetHealthWithAnchor is the add-on's account of itself plus the backend's
// memory of its log. Two authorities, one answer: the add-on cannot be the
// source of truth about whether its own record has been edited.
type targetHealthWithAnchor struct {
	addons.TargetHealth
	LogAnchor *db.LogAnchor `json:"log_anchor,omitempty"`
	// BindingConflicts is where two of Syndra's own records disagree about who
	// owns an account. Not the add-on's opinion and not drift: both stores were
	// written by this system, and neither is authoritative, which is why it
	// needs a person rather than a reconcile.
	BindingConflicts []db.BindingConflict `json:"binding_conflicts,omitempty"`
}

func describeTarget(reg addons.Registration, transport addons.TransportCredential) targetSummary {
	s := targetSummary{Target: reg.Target, Registered: true, AuthMode: reg.AuthMode()}
	s.TransportStatus, s.TransportError = transport.Status, transport.Error

	a, err := addonsGet(reg.Target)
	if err != nil {
		return s
	}
	if fetched := a.FetchedAt(); !fetched.IsZero() {
		s.ManifestFetchedAt = &fetched
		s.Callable = true
	}
	s.CircuitOpen = a.CircuitOpen()
	if lastErr := a.LastError(); lastErr != nil {
		s.LastError = lastErr.Error()
	}
	for _, op := range a.Operations() {
		s.Operations = append(s.Operations, operationSummary{
			ID: op.ID, Scope: string(op.Scope), Confirm: op.Confirm,
			Available: op.Available, UnavailableReason: op.UnavailableReason,
			SecretParams: op.SecretParams,
		})
	}
	return s
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// resolveFindingRequest is an operator adopting a reported head as the new
// baseline.
type resolveFindingRequest struct {
	// Head is the violating head they were shown. Cited rather than implied,
	// because "re-baseline to whatever is there now" would adopt a chain that
	// changed again while the dialog was open — which is the event this
	// mechanism exists to notice.
	Head string `json:"head"`
	// Note is why. The one finding whose explanation is the whole of its value:
	// "we replaced the volume" and "we do not know" are the same anchor state
	// and completely different facts.
	Note      string `json:"note"`
	Confirmed bool   `json:"confirmed"`
}

// resolveFindingCopy is the confirmation an operator reads before clearing a
// tamper finding.
//
// It names what is given up rather than what is done. Clearing this is the only
// action in the product that discards evidence, and the sentence has to say so
// in those words.
const resolveFindingCopy = "Resolving adopts the log the target is reporting NOW as the new baseline. " +
	"The records that went missing stay missing, and Syndra stops being able to tell you they did. " +
	"Do this when you know why the log changed — a rebuilt add-on, a replaced volume — and not to clear a warning."

// handleResolveLogFinding clears a log-anchor finding by re-baselining to it.
//
// The surface has always said "this stays until somebody resolves it". Until
// this endpoint there was no way to, so a legitimate volume replacement pinned
// a target as compromised permanently and the sentence named an action that did
// not exist.
func handleResolveLogFinding(w http.ResponseWriter, r *http.Request) {
	target := r.PathValue("target")
	var req resolveFindingRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}
	if strings.TrimSpace(req.Note) == "" {
		jsonValidationErrorResponse(w, "Resolving a finding takes an explanation",
			map[string]string{"note": "required"})
		return
	}
	if !req.Confirmed {
		jsonErrorResponse(w, http.StatusUnprocessableEntity, "CONFIRMATION_REQUIRED", resolveFindingCopy)
		return
	}

	anchor, err := dbResolveLogViolation(r.Context(), target, resolveActor(r, ""), req.Note, req.Head)
	switch {
	case errors.Is(err, db.ErrNoAnchorFinding):
		jsonErrorResponse(w, http.StatusConflict, "NO_FINDING", err.Error())
		return
	case errors.Is(err, db.ErrAnchorMoved):
		jsonErrorResponse(w, http.StatusConflict, "FINDING_MOVED",
			"The target's log changed again since you read this finding. Re-read it before resolving — "+
				"what you were about to adopt is not what the target is reporting now.")
		return
	case err != nil:
		jsonErrorResponse(w, http.StatusInternalServerError, "RESOLVE_FAILED", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, anchor)
}

// resolveConflictRequest is an operator deciding who owns a disputed account.
type resolveConflictRequest struct {
	// Owner must be one of the two subjects the finding names. A third party is
	// not a resolution of this disagreement; it is a different decision with no
	// rehearsal behind it, and the store refuses it.
	Owner string `json:"owner"`
	// Note is what they decided and why. The row somebody reads when the other
	// subject asks where their account went.
	Note      string `json:"note"`
	Confirmed bool   `json:"confirmed"`
}

// resolveConflictCopy is the confirmation.
//
// It names the person who LOSES the account, because that is the half an
// operator can get wrong and the half nobody is notified about. Resolving is
// not filing the finding away — it moves an account from one person to another
// in Syndra's records, and the loser's page stops showing it.
const resolveConflictCopy = "Deciding who owns this account moves it in Syndra's records. " +
	"The other subject stops holding it here, immediately and without being told. " +
	"Their data on the target is untouched — this changes who Syndra says it belongs to, " +
	"which is what every later revocation, sweep and convergence will act on."

// handleResolveBindingConflict records who a disputed account belongs to.
func handleResolveBindingConflict(w http.ResponseWriter, r *http.Request) {
	var req resolveConflictRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}
	if strings.TrimSpace(req.Owner) == "" {
		jsonValidationErrorResponse(w, "Nobody to give the account to",
			map[string]string{"owner": "required"})
		return
	}
	if strings.TrimSpace(req.Note) == "" {
		jsonValidationErrorResponse(w, "Resolving a conflict takes an explanation",
			map[string]string{"note": "required"})
		return
	}
	if !req.Confirmed {
		jsonErrorResponse(w, http.StatusUnprocessableEntity, "CONFIRMATION_REQUIRED", resolveConflictCopy)
		return
	}

	// The rebind and the convergences in ONE transaction, and this is the half
	// the first version left out. Agreeing Syndra's two records does not touch
	// the target, and the target is wrong in both directions: the losing
	// subject's groups were overwritten by the convergence that caused the
	// conflict, so they hold a mapped role with no account — and the winning
	// subject's account is carrying the OTHER person's resolved set, which is
	// access they were never granted, sitting under a finding somebody has just
	// marked resolved.
	//
	// Every other decision on this branch that changes who holds what enqueues
	// in the same transaction it commits. A disclosure telling the operator to
	// go and converge two people by hand would make this the one place a
	// resolved decision leaves the work on a human.
	actor := resolveActor(r, "")
	var conflict db.BindingConflict
	var queued []string
	err := svcInTxLockingAccess(r.Context(), func(ctx context.Context) error {
		var err error
		conflict, err = dbResolveBindingConflict(ctx, r.PathValue("id"), req.Owner, actor, req.Note)
		if err != nil {
			return err
		}
		// Both claimants, always. Which one needs what depends on which way the
		// operator decided, and resolving each is cheaper than reasoning about
		// it — a convergence for somebody already correct is a no-op the add-on
		// answers without writing.
		for _, subject := range []string{conflict.ConvergedSubjectID, conflict.BoundSubjectID} {
			set, resolveErr := svcResolveEntitlementsFor(ctx, subject, conflict.Target)
			if resolveErr != nil {
				return fmt.Errorf("resolve %s on %s: %w", subject, conflict.Target, resolveErr)
			}
			if _, _, qerr := dbRecordSystemConvergence(ctx, db.SystemConvergence{
				Target: conflict.Target, SubjectID: subject, Actor: actor,
				Reason:  "A disputed account was assigned an owner",
				Desired: set,
			}); qerr != nil {
				return fmt.Errorf("queue convergence for %s: %w", subject, qerr)
			}
			queued = append(queued, subject)
		}
		return nil
	})
	switch {
	case errors.Is(err, db.ErrNoSuchConflict):
		jsonErrorResponse(w, http.StatusNotFound, "NO_SUCH_CONFLICT", err.Error())
		return
	case errors.Is(err, db.ErrInvalidTargetBinding):
		// The third-party case, and it is a validation failure rather than a
		// refusal to act: the operator named somebody the finding does not.
		jsonValidationErrorResponse(w, err.Error(), map[string]string{"owner": "not_a_claimant"})
		return
	case err != nil:
		jsonErrorResponse(w, http.StatusInternalServerError, "RESOLVE_FAILED", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{
		"resolved": true, "target": conflict.Target, "username": conflict.Username,
		"owner": req.Owner, "queued": queued,
		// Queued, not applied, and it says which. The convergence that caused
		// the conflict left the account holding the other person's resolved set
		// — access the owner was never granted — and it stays there until this
		// drains. That is the sentence an operator needs, not "resolved".
		"detail": "Syndra's records now agree, and a convergence is queued for both people. " +
			"Until it drains the account still holds whatever the change that caused this wrote to it.",
	})
}

// handleReconcileTarget runs one add-on target's reconciliation now.
//
// The scheduler has driven `ReconcileAddon` since it was written and nothing
// else could: [Reconcile now] existed for Zitadel and for no target, so an
// operator asking "is this in step?" had to wait up to six hours and read a log
// line. The same pass, on demand, returning what it found.
//
// It queues; it does not apply. Every convergence it records waits for the
// drain like any other add-on row, which is what makes an operator-triggered
// sweep safe to press twice.
func handleReconcileTarget(w http.ResponseWriter, r *http.Request) {
	target := r.PathValue("target")
	if _, err := addonsGet(target); err != nil {
		// Registered is a deployment fact; reconciling one that is not
		// registered would report an empty pass as a healthy one.
		jsonErrorResponse(w, http.StatusNotFound, "TARGET_NOT_REGISTERED", err.Error())
		return
	}
	res, err := svcReconcileAddon(r.Context(), target)
	if err != nil {
		jsonErrorResponse(w, http.StatusBadGateway, "RECONCILE_FAILED", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, res)
}

// handleTargetActivity reports what the target's own audit log holds for one
// subject.
//
// GET /api/v1/targets/{target}/activity?subject={id}&since={rfc3339}
//
// Operator-gated and subject-scoped. A member does not reach this: the
// member-facing read is `storage.status`, which takes no subject at all, and
// giving a member a subject parameter here would be the one shape in which
// somebody could ask about another account.
//
// The add-on has implemented `activity.get` since the platform landed and the
// policy table has always declared it, and until now no route called it — so
// the two TrueNAS roles it needs were configured on the deployment's key and
// exercised by nothing.
// handleTargetSystemHealth reads what the TARGET says about itself.
//
// Distinct from `handleTargetHealth`, which reads what the ADD-ON says about
// the target. That one is cheap and polled by every target page; this costs
// four calls to the NAS and is read once when somebody opens it.
//
// Answers 200 with `readable: false` when the read fails, for the same reason
// the activity surface does: "the target could not be asked" is an answer, and
// it must never render as "the target reported nothing wrong". A pool that
// could not be read and a healthy pool are opposite facts.
func handleTargetSystemHealth(w http.ResponseWriter, r *http.Request) {
	target := r.PathValue("target")

	report := addonsSystemHealth(r.Context(), target)
	if report.Outcome != addons.OutcomeSucceeded {
		jsonResponse(w, http.StatusOK, map[string]any{
			"target":   target,
			"readable": false,
			"detail":   errText(report.Err),
		})
		return
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"target":   target,
		"readable": true,
		"system":   report.System,
		"alerts":   report.Alerts,
		"pools":    report.Pools,
		"services": report.Services,
		// Named sources, not a count. "alerts could not be read" and "there are
		// no alerts" are the same empty list without it.
		"degraded": report.Degraded,
	})
}

func handleTargetActivity(w http.ResponseWriter, r *http.Request) {
	target := r.PathValue("target")
	subject := strings.TrimSpace(r.URL.Query().Get("subject"))
	if subject == "" {
		jsonValidationErrorResponse(w, "subject is required",
			map[string]string{"subject": "required"})
		return
	}
	// Bounded rather than free-form: `since` reaches the target's query
	// builder, and a value this cannot parse is one the add-on would have to
	// decide about.
	since := strings.TrimSpace(r.URL.Query().Get("since"))
	if since != "" {
		if _, err := time.Parse(time.RFC3339, since); err != nil {
			jsonValidationErrorResponse(w, "since must be an RFC3339 timestamp",
				map[string]string{"since": "must_be_rfc3339"})
			return
		}
	}

	report := addonsActivity(r.Context(), target, subject, since)
	if report.Outcome != addons.OutcomeSucceeded {
		// 200 with `readable: false`, for the same reason the health surface
		// answers 200 when the add-on is down: "the log could not be read" is
		// the answer to the question, and it must not render as "no activity".
		// Those two are the whole point of this surface.
		jsonResponse(w, http.StatusOK, map[string]any{
			"target":   target,
			"subject":  subject,
			"readable": false,
			"detail":   errText(report.Err),
		})
		return
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"target":           target,
		"subject":          subject,
		"readable":         true,
		"events":           report.Events,
		"unaudited_shares": report.UnauditedShares,
	})
}
