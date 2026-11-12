package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path"
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
//
// There is no acknowledgement field: contract 2 gives the cohort guard to the
// backend, which chunks below limits.subjects_per_plan (design §11).
type PlanRequest struct {
	ContractVersion int           `json:"contract_version"`
	Subjects        []PlanSubject `json:"subjects"`
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
	// Stale says why, exactly when Current is false.
	Stale *Stale `json:"stale,omitempty"`
}

// perRequestSubjectCap bounds one request, and is the manifest's
// limits.subjects_per_plan.
//
// A backstop against a backend asking for too much at once, and explicitly not
// the cohort guard: `/apply` is per subject, so this add-on can never compute
// "affected subject count" across a plan. Specifying the real guard here would
// have put it in the one component unable to implement it.
const perRequestSubjectCap = 200

func (s *server) handlePlan(w http.ResponseWriter, r *http.Request, body []byte) {
	var req PlanRequest
	if !decodeRequest(w, body, &req, CodeBadRequest) || !writeContractRefusal(w, req.ContractVersion) {
		return
	}
	if n := len(req.Subjects); n > perRequestSubjectCap {
		// Reports the count it computed and the limit, so the caller can chunk.
		writeError(w, http.StatusUnprocessableEntity, CodePlanTooLarge,
			fmt.Sprintf("A plan may name at most %d subjects; this one names %d.", perRequestSubjectCap, n),
			map[string]any{"subjects": n, "limit": perRequestSubjectCap})
		return
	}
	for _, subject := range req.Subjects {
		if strings.TrimSpace(subject.Subject) == "" {
			writeError(w, http.StatusBadRequest, CodeBadRequest, "Every planned subject needs a subject.", nil)
			return
		}
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
			writeError(w, statusFor(err), CodeTargetUnreadable, err.Error(), nil)
			return
		}
		snap, current = cached, false
	}
	byName := make(map[string]*Subject, len(snap.Subjects))
	byUID := make(map[int64]*Subject, len(snap.Subjects))
	for i := range snap.Subjects {
		byName[snap.Subjects[i].Username] = &snap.Subjects[i]
		byUID[snap.Subjects[i].UID] = &snap.Subjects[i]
	}
	claimed, err := s.store.BoundUsernames()
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeStoreUnreadable, "The add-on's bindings could not be read.", nil)
		return
	}

	out := PlanResponse{
		Outcomes:  make([]ApplyOutcome, 0, len(req.Subjects)),
		Current:   current,
		TakenAt:   snap.TakenAt.UTC().Format(time.RFC3339),
		Truncated: snap.Truncated,
	}
	if !current {
		out.Stale = unreachableSince(snap.TakenAt)
	}
	for _, subject := range req.Subjects {
		outcome, err := s.planOne(subject, byName, byUID, claimed, snap, current)
		switch {
		case errors.Is(err, errStoreUnreadable):
			writeError(w, http.StatusInternalServerError, CodeStoreUnreadable, "The add-on's own records could not be read, so nothing was planned.", nil)
			return
		case err != nil:
			writeError(w, statusFor(err), CodeTargetUnreadable, err.Error(), nil)
			return
		}
		out.Outcomes = append(out.Outcomes, outcome)
	}
	writeJSON(w, http.StatusOK, out)
}

// errStoreUnreadable is the add-on's own records failing to read during a plan.
var errStoreUnreadable = errors.New("the add-on's own records could not be read")

// planOne says what would happen to one subject, and issues no mutating call.
//
// `blocked` is only ever a state this app cannot hold (unholdable), a conflict
// an operator decides, or an absence a truncated read cannot prove: Syndra
// records it as the target refusing, which is terminal for that row
// (propagation fingerprintFor). A read that failed is an error, and the whole
// plan is refused with it (handlePlan), so the row is planned again rather than
// given up on.
//
// live is handlePlan's `current`. A plan from the stored copy skips the home
// folder's stat, as it skips the group check, and leaves both to the apply:
// with the NAS down, one subject's home would otherwise turn the whole
// stored-copy answer into a 5xx.
func (s *server) planOne(req PlanSubject, byName map[string]*Subject, byUID map[int64]*Subject, claimed map[string]string, snap Snapshot, live bool) (ApplyOutcome, error) {
	truncated := snap.Truncated
	desired, err := s.decodeDesired(req.Desired)
	var u unholdable
	switch {
	case errors.As(err, &u):
		return ApplyOutcome{
			Subject: req.Subject, Effect: EffectBlocked,
			Detail: err.Error(), Consequence: "Nothing would change.",
		}, nil
	case err != nil:
		return ApplyOutcome{}, fmt.Errorf("%w: %w", errStoreUnreadable, err)
	}

	binding, bound, err := s.store.GetBinding(req.Subject)
	if err != nil {
		return ApplyOutcome{}, fmt.Errorf("%w: %w", errStoreUnreadable, err)
	}

	var current *Subject
	name := binding.Username
	adopts := false // leftByCrashedCreate, as the apply decides it
	if bound {
		// The rule the apply's locate uses, read from this snapshot.
		floor, ceiling := s.idRange()
		found, resolved, conflict, _ := boundAccount(binding, floor, ceiling,
			func(n string) (*Subject, error) { return byName[n], nil },
			func(u int64) (*Subject, error) { return byUID[u], nil })
		if conflict != nil {
			detail, next := conflict.words()
			return ApplyOutcome{
				Subject: req.Subject, Effect: EffectBlocked,
				Detail: detail, Consequence: "Nothing would change. " + next,
				Username:    conflict.Username,
				Fingerprint: fingerprintSubject(byName[conflict.Username]),
				Conflict:    conflict,
			}, nil
		}
		current, name = found, resolved.Username
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
			// outcome — that account may belong to somebody else entirely —
			// save the one case the apply adopts (5.3d), planned the same.
			conflict := s.unboundConflict(existing, claimed[name])
			adopts, err = s.leftByCrashedCreate(req.Subject, name, existing, conflict)
			if err != nil {
				return ApplyOutcome{}, fmt.Errorf("%w: %w", errStoreUnreadable, err)
			}
			if !adopts {
				detail, next := conflict.words()
				return ApplyOutcome{
					Subject: req.Subject, Effect: EffectBlocked,
					Detail: detail, Consequence: "Nothing would change. " + next,
					Username:    name,
					Fingerprint: fingerprintSubject(existing),
					Conflict:    conflict,
				}, nil
			}
			current = existing
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
		}, nil
	}

	if noAccountToSwitchOff(current, desired) {
		out := noAccountToSwitchOffOutcome(req.Subject)
		out.Fingerprint = fingerprint
		return out, nil
	}
	// The groups the apply would write, checked where the apply checks them
	// (groupIDsFor in converge and createAndConverge), so a group the NAS does
	// not have, or a built-in one, is blocked by both.
	// ponytail: a plan from the mirror has no group table and skips this; it
	// is current: false, which Syndra never applies, and the apply checks.
	if g := decide(current, desired, homeDecision{}).groups; g != nil && snap.groupsByName != nil {
		if _, err := groupIDs(*g, snap.groupsByName, snap.builtinGroups); err != nil {
			return ApplyOutcome{
				Subject: req.Subject, Effect: EffectBlocked,
				Detail: err.Error(), Consequence: "Nothing would change.",
			}, nil
		}
	}
	if current == nil {
		return ApplyOutcome{
			Subject: req.Subject, Effect: EffectApply,
			Detail:      fmt.Sprintf("Creates %s.", name),
			Consequence: describeHolding(projected(desired, name)),
			Username:    name, Fingerprint: fingerprint,
		}, nil
	}

	// The home and its limit, decided by the apply's own rules and reads, so
	// the plan never promises a folder or a limit the apply would not make.
	home, err := s.decideHome(desired, current.Username, current.Home, current.UID, live)
	if err != nil {
		// The target's read failing, as the apply answers it (converge): a
		// retry, never `blocked`, which Syndra takes as the target refusing.
		// Only a live plan reads, so only a live plan gets here.
		return ApplyOutcome{}, err
	}
	// The one decision `apply` also executes (apply.go decide), so what is
	// planned is what is written.
	c := decide(current, desired, home)
	after := c.applyTo(*current)
	// A limit that can't be read is not planned: the apply would fail to read
	// it too, and planning it would queue a row every pass.
	_, quota, quotaOwed, _ := s.quotaOwed(desired, after.Home, current.UID)

	changes := c.writes()
	if !changes && !quotaOwed {
		detail := "Already in the requested state."
		if desired.managed[FieldSMBEnabled] && desired.smbEnabled && !current.SMBEnabled {
			// Not "already in the requested state": SMB is still off, and
			// waiting on the member rather than on Syndra.
			detail = smbWaiting(current)
		}
		if home.note != "" {
			detail += " " + home.note
		}
		out := ApplyOutcome{
			Subject: req.Subject, Effect: EffectNoChange,
			Detail:      detail,
			Consequence: describeHolding(*current),
			Username:    current.Username, Fingerprint: fingerprint,
			AccountID: accountID(current.UID), Standing: standingOf(current),
		}
		if adopts {
			out = adoptionWords(out, true)
		}
		return out, nil
	}
	detail := ""
	if changes {
		detail = describePlannedChange(*current, after)
	}
	if quotaOwed {
		detail = strings.TrimSpace(detail + " " + quotaWords(quota))
	}
	out := ApplyOutcome{
		Subject: req.Subject, Effect: EffectApply,
		Detail:      detail,
		Consequence: describeHolding(after),
		Username:    current.Username, Fingerprint: fingerprint,
		AccountID: accountID(current.UID), Standing: standingOf(current),
	}
	if adopts {
		out = adoptionWords(out, true)
	}
	return out, nil
}

// projected is the state a NEW account would be left in: the shared decision
// (apply.go decide, nil current) applied to a blank account. An unmanaged field
// is carried through unchanged rather than defaulted, because a plan that
// predicted a change the apply does not make is a plan nobody should trust.
// An existing account is not projected here; plan applies decide to it directly.
func projected(desired desiredState, username string) Subject {
	// A new account is usable unless something says otherwise, matching what the
	// create path does. SMB stays off: the apply never switches it on, the
	// member's next password does.
	out := decide(nil, desired, homeDecision{}).applyTo(Subject{Username: username, Enabled: true})
	// A home folder is planned only where the apply would make one: a home
	// location chosen. Whether the folder is free is the apply's read to make.
	if desired.managed[FieldHome] && desired.wantsHome && desired.homeRoot != "" &&
		(!desired.managed[FieldEnabled] || desired.enabled) {
		out.Home = path.Join(desired.homeRoot, username)
	}
	return out
}

func describePlannedChange(before, after Subject) string {
	var parts []string
	if after.Home != before.Home && after.Home != "" {
		parts = append(parts, "gets a home folder at "+after.Home)
	}
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
