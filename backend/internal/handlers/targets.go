package handlers

import (
	"log"
	"net/http"
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
		jsonResponse(w, http.StatusOK, body)
		return
	}

	if !anchored {
		jsonResponse(w, http.StatusOK, health)
		return
	}
	// Merged rather than nested under a second request, because the operator's
	// question is one question: is this target all right.
	jsonResponse(w, http.StatusOK, targetHealthWithAnchor{TargetHealth: health, LogAnchor: &anchor})
}

// targetHealthWithAnchor is the add-on's account of itself plus the backend's
// memory of its log. Two authorities, one answer: the add-on cannot be the
// source of truth about whether its own record has been edited.
type targetHealthWithAnchor struct {
	addons.TargetHealth
	LogAnchor *db.LogAnchor `json:"log_anchor,omitempty"`
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
