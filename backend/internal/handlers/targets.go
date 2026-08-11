package handlers

import (
	"errors"
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
	// AuthMode is how the backend authenticates to it: mutual TLS or signed
	// requests. Rendered because an operator who believes mutual TLS is on while
	// running signed requests has a wrong model of their own trust boundary and
	// nothing else would tell them.
	AuthMode string `json:"auth_mode"`
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
	out := make([]targetSummary, 0, len(registrations))
	for _, reg := range registrations {
		out = append(out, describeTarget(reg))
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

func describeTarget(reg addons.Registration) targetSummary {
	s := targetSummary{Target: reg.Target, Registered: true, AuthMode: reg.AuthMode()}

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

	conflict, err := dbResolveBindingConflict(r.Context(), r.PathValue("id"),
		req.Owner, resolveActor(r, ""), req.Note)
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
		"owner": req.Owner,
		// Queued, because the losing subject's entitlements are now unrecorded
		// on this target and the winning subject's account may not match what
		// policy says. Said rather than implied: the records agree now, and the
		// TARGET has not been touched.
		"detail": "Syndra's records now agree. Nothing on the target changed — " +
			"converge the target when you are ready.",
	})
}
