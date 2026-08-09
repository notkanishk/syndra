package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// `POST /plan` — compute the effect of a proposed change, mutating nothing
// (design §8).
//
// It returns the same per-subject shape `/apply` returns, so the operator
// surface renders one thing and the diff an operator reads is written in the
// same words as the result they get afterwards. And it returns a state
// fingerprint per subject, which is what makes the approval mean anything: the
// apply carries it back and refuses if the subject moved.

// PlanRequest is a proposed change across a cohort.
type PlanRequest struct {
	Subjects []ApplyRequest `json:"subjects"`
	// AcknowledgeScope is the caller confirming an oversized request. Defence in
	// depth only — this add-on sees one request and cannot observe a cohort
	// spanning several, so the authoritative guard is the backend's at plan
	// time, where the cohort actually exists.
	AcknowledgeScope bool `json:"acknowledge_scope,omitempty"`
}

type PlanResponse struct {
	Outcomes []ApplyOutcome `json:"outcomes"`
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
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "BAD_REQUEST"})
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
	snap, err := s.readSubjects()
	if err != nil {
		writeJSON(w, statusFor(err), map[string]string{"error": "TARGET_UNREADABLE", "detail": err.Error()})
		return
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

	out := PlanResponse{Outcomes: make([]ApplyOutcome, 0, len(req.Subjects))}
	for _, subject := range req.Subjects {
		out.Outcomes = append(out.Outcomes, s.planOne(subject, byName, claimed))
	}
	writeJSON(w, http.StatusOK, out)
}

// planOne says what would happen to one subject, and issues no mutating call.
func (s *server) planOne(req ApplyRequest, byName map[string]*Subject, claimed map[string]string) ApplyOutcome {
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
