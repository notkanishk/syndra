package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// `POST /plan` — compute the effect of a proposed change, mutating nothing
// (design §8).
//
// It returns the same per-subject shape `/apply` returns, so the operator
// surface renders one thing and the diff an operator reads is written in the
// same words as the result they get afterwards. And it returns a state
// fingerprint per subject, which is what makes the approval mean anything: the
// apply carries it back and refuses if the subject moved.

// PlanSubject is one subject inside a proposed change.
//
// Deliberately not `ApplyRequest`. A plan has no call id to deduplicate on, no
// fingerprint to verify against — it is where the fingerprint comes FROM — and
// no plan id, because the plan being computed is what will get one. Reusing the
// apply's shape here would have meant either the backend sending four fields
// that mean nothing, or this add-on accepting a body it cannot honour.
type PlanSubject struct {
	Subject string `json:"subject"`
	// Email is what a username is derived from, for a subject with no account
	// yet. The derivation must match the apply's exactly or the plan predicts a
	// name the apply does not produce.
	Email   string                     `json:"email"`
	Desired map[string]json.RawMessage `json:"desired"`
}

// PlanRequest is a proposed change across a cohort.
type PlanRequest struct {
	ContractVersion int           `json:"contract_version"`
	Subjects        []PlanSubject `json:"subjects"`
	// AcknowledgeScope is the caller confirming an oversized request. Defence in
	// depth only — this add-on sees one request and cannot observe a cohort
	// spanning several, so the authoritative guard is the backend's at plan
	// time, where the cohort actually exists.
	AcknowledgeScope bool `json:"acknowledge_scope,omitempty"`
}

type PlanResponse struct {
	Outcomes []ApplyOutcome `json:"outcomes"`
	// Current says the plan was computed against a live read rather than the
	// mirror. A plan computed from the mirror is still worth issuing — §7's
	// fail-open rule is that an unreachable target must not block the
	// entitlement decision — but the backend has to mark it provisional, so the
	// currency travels with the plan rather than being inferred from the
	// target's health a moment later.
	Current bool `json:"current"`
	// TakenAt is when the read behind these outcomes happened. It is the AGE the
	// operator surface labels a provisional plan with; without it, "computed
	// against last-known state" is a claim with no number attached.
	TakenAt string `json:"taken_at"`
	// Truncated says the read hit the cap. Carried because it changes what an
	// outcome means, not merely how confident it is: an absence in a capped read
	// is not an absence.
	Truncated bool `json:"truncated"`
}

// perRequestSubjectCap bounds one request.
//
// A backstop against a backend asking for too much at once, and explicitly not
// the cohort guard: `/apply` is per subject, so this add-on can never compute
// "affected subject count" across a plan. Specifying the real guard here would
// have put it in the one component unable to implement it.
const perRequestSubjectCap = 200

func (s *server) handlePlan(w http.ResponseWriter, r *http.Request, body []byte) {
	var req PlanRequest
	if err := decodeStrict(body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "BAD_REQUEST"})
		return
	}
	if !writeContractRefusal(w, req.ContractVersion) {
		return
	}
	if n := len(req.Subjects); n > perRequestSubjectCap && !req.AcknowledgeScope {
		// Reports the count it computed. "Too large" leaves the caller guessing
		// at what it is being warned about, and the caller here is a backend
		// that has to decide whether to acknowledge.
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"error": "SCOPE_ACKNOWLEDGEMENT_REQUIRED", "subjects": n, "limit": perRequestSubjectCap,
		})
		return
	}

	// One read for the whole cohort. Planning forty subjects must not be forty
	// state reads through a single rate-limited WebSocket.
	//
	// A failed read falls back to the mirror rather than refusing, and the
	// answer says which it was. Refusing would make an unreachable target block
	// the entitlement decision, which is the one thing §7 says it must not do:
	// the change would be neither recorded nor rejected, and the operator would
	// be told to come back later. The mirror is enough to compute a diff an
	// operator can approve; what it cannot do is prove that diff still describes
	// the target, and the fingerprint carried into the apply is what settles
	// that at the moment of writing.
	current := true
	snap, err := s.readSubjects()
	if err != nil {
		cached, found, cacheErr := s.store.GetSnapshot()
		if cacheErr != nil || !found {
			// Nothing current and nothing remembered. Refused rather than
			// planned against an empty world, which would read as "everybody
			// needs an account creating".
			writeJSON(w, statusFor(err), map[string]string{"error": "TARGET_UNREADABLE", "detail": err.Error()})
			return
		}
		snap, current = cached, false
	}
	byName := make(map[string]*Subject, len(snap.Subjects))
	for i := range snap.Subjects {
		byName[snap.Subjects[i].Username] = &snap.Subjects[i]
	}
	claimed, err := s.store.BoundUsernames()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "STORE_UNREADABLE"})
		return
	}

	out := PlanResponse{
		Outcomes:  make([]ApplyOutcome, 0, len(req.Subjects)),
		Current:   current,
		TakenAt:   snap.TakenAt.UTC().Format(time.RFC3339),
		Truncated: snap.Truncated,
	}
	for _, subject := range req.Subjects {
		out.Outcomes = append(out.Outcomes, s.planOne(subject, byName, claimed, snap.Truncated))
	}
	writeJSON(w, http.StatusOK, out)
}

// planOne says what would happen to one subject, and issues no mutating call.
func (s *server) planOne(req PlanSubject, byName map[string]*Subject, claimed map[string]string, truncated bool) ApplyOutcome {
	desired, err := decodeDesired(req.Desired)
	if err != nil {
		return ApplyOutcome{
			Subject: req.Subject, Effect: EffectBlocked,
			Detail: err.Error(),
		}
	}

	binding, bound, err := s.store.GetBinding(req.Subject)
	if err != nil {
		return ApplyOutcome{Subject: req.Subject, Effect: EffectBlocked, Detail: "The local binding store could not be read."}
	}

	var current *Subject
	name := binding.Username
	if bound {
		current = byName[binding.Username]
	} else {
		// The same rule the apply uses, or the plan predicts a name the apply
		// does not produce — and a plan that disagrees with its apply is worse
		// than no plan.
		name = DeriveUsername(req.Email, req.Subject, func(candidate string) bool {
			owner, ok := claimed[candidate]
			return ok && owner != req.Subject
		})
		if existing, ok := byName[name]; ok {
			// Reported, not resolved. Adopting silently is the dangerous
			// outcome — that account may belong to somebody else entirely.
			return ApplyOutcome{
				Subject: req.Subject, Effect: EffectBlocked,
				Detail:      fmt.Sprintf("An account named %s already exists and is not bound to anyone.", name),
				Consequence: "Nothing would change. Adopt it, or create under a suffixed name.",
				Username:    name,
				Fingerprint: fingerprintSubject(existing),
				Conflict: &BindingConflict{
					Username: name, UID: existing.UID,
					Adoptable: claimed[name] == "", BoundTo: claimed[name],
				},
			}
		}
	}

	// The fingerprint is of what was READ, not of what would result. It is what
	// the apply re-verifies against, so it has to describe the world the
	// operator is approving a change to.
	fingerprint := fingerprintSubject(current)

	if current == nil && truncated {
		// An absence read out of a capped list is not an absence. Planning a
		// CREATE from one would make a second account for a subject who already
		// has one past the cap — and the fingerprint would not catch it, because
		// "not in the read" and "not on the target" produce the same one.
		// Staleness is covered by the fingerprint; truncation has to be refused
		// here, where the cap is known.
		return ApplyOutcome{
			Subject: req.Subject, Effect: EffectBlocked,
			Detail:      "The account list was longer than one read returns, so this subject's absence from it proves nothing.",
			Consequence: "Nothing would change.",
			Fingerprint: fingerprint,
		}
	}

	if current == nil {
		return ApplyOutcome{
			Subject: req.Subject, Effect: EffectApply,
			Detail:      fmt.Sprintf("Creates %s.", name),
			Consequence: describeHolding(projected(nil, desired, name)),
			Username:    name, Fingerprint: fingerprint,
		}
	}

	after := projected(current, desired, current.Username)
	if sameSet(after.Groups, current.Groups) && after.Enabled == current.Enabled && after.SMBEnabled == current.SMBEnabled {
		return ApplyOutcome{
			Subject: req.Subject, Effect: EffectNoChange,
			Detail:      "Already in the requested state.",
			Consequence: describeHolding(*current),
			Username:    current.Username, Fingerprint: fingerprint,
		}
	}
	return ApplyOutcome{
		Subject: req.Subject, Effect: EffectApply,
		Detail:      describePlannedChange(*current, after),
		Consequence: describeHolding(after),
		Username:    current.Username, Fingerprint: fingerprint,
	}
}

// projected is the state a subject would be left in.
//
// Computed by the same rules the apply applies, from the same decoded desired
// state — an unmanaged field is carried through unchanged rather than defaulted,
// because a plan that predicted a change the apply does not make is a plan
// nobody should trust.
func projected(current *Subject, desired desiredState, username string) Subject {
	out := Subject{Username: username}
	if current != nil {
		out = *current
	}
	if desired.managed[FieldGroup] {
		out.Groups = desired.groups
	}
	if desired.managed[FieldEnabled] {
		out.Enabled = desired.enabled
	} else if current == nil {
		// A new account is usable unless something says otherwise, matching
		// what the create path does.
		out.Enabled = true
	}
	if desired.managed[FieldSMBEnabled] {
		out.SMBEnabled = desired.smbEnabled
	}
	return out
}

func describePlannedChange(before, after Subject) string {
	var parts []string
	if !sameSet(before.Groups, after.Groups) {
		gained, lost := diffSets(before.Groups, after.Groups)
		if len(gained) > 0 {
			parts = append(parts, "joins "+strings.Join(gained, ", "))
		}
		if len(lost) > 0 {
			parts = append(parts, "leaves "+strings.Join(lost, ", "))
		}
	}
	if before.Enabled != after.Enabled {
		if after.Enabled {
			parts = append(parts, "account enabled")
		} else {
			parts = append(parts, "account disabled")
		}
	}
	if before.SMBEnabled != after.SMBEnabled {
		if after.SMBEnabled {
			parts = append(parts, "SMB enabled")
		} else {
			parts = append(parts, "SMB disabled")
		}
	}
	if len(parts) == 0 {
		return "No change."
	}
	return strings.ToUpper(parts[0][:1]) + parts[0][1:] + suffixList(parts[1:]) + "."
}

func suffixList(rest []string) string {
	if len(rest) == 0 {
		return ""
	}
	return "; " + strings.Join(rest, "; ")
}

// diffSets reports what a change adds and removes, which is what an operator
// reads — a before-and-after pair of full lists makes them do the diff.
func diffSets(before, after []string) (gained, lost []string) {
	in := func(list []string, v string) bool {
		for _, x := range list {
			if x == v {
				return true
			}
		}
		return false
	}
	for _, a := range after {
		if !in(before, a) {
			gained = append(gained, a)
		}
	}
	for _, b := range before {
		if !in(after, b) {
			lost = append(lost, b)
		}
	}
	return gained, lost
}
