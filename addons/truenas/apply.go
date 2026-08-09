package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

// The entitlement plane (design §4).
//
// Level-triggered: the request carries a resolved desired state and this
// converges the account onto it. Not a delta — `user.update({groups})` is a
// full replace, which is what makes retry safe by construction and makes
// "revoke partial" the same call as "grant partial" with a different set.
//
// Account existence is part of that convergence rather than a separate
// operation, and that dissolves an ordering problem instead of answering it.
// Keeping `account.ensure` as its own call meant the apply — a versioned
// snapshot on the outbox — and the creation — a one-shot on another path — had
// nothing sequencing them, so an apply could reach a subject whose account did
// not exist yet.

// ApplyRequest is one subject's resolved desired state.
type ApplyRequest struct {
	CallID  string `json:"call_id"`
	Subject string `json:"subject"`
	// Email is what a username is derived from, and only when an account has to
	// be created. An existing binding is authoritative: a later email change
	// MUST NOT rename an account, because renaming disturbs its home directory,
	// its ACL entries and its SMB identity.
	Email string `json:"email"`
	// Fingerprint is the target state the plan was computed against, echoed
	// back so this call can refuse if the subject moved in between.
	Fingerprint string `json:"fingerprint,omitempty"`
	// Desired is the whole set, by field. Absent and empty are different: one
	// says "do not manage this", the other says "make it empty".
	Desired map[string]json.RawMessage `json:"desired"`
}

// ApplyOutcome is what happened, in the BulkOutcome shape the operator surface
// already renders.
type ApplyOutcome struct {
	Subject string `json:"subject"`
	Effect  string `json:"effect"`
	Detail  string `json:"detail"`
	// Consequence is what the subject is left holding — the part a count never
	// tells you.
	Consequence string `json:"consequence,omitempty"`
	// Username is the derived or bound account name, reported so no separate
	// creation call has to be sequenced before this one to learn it.
	Username string `json:"username,omitempty"`
	// Fingerprint is the subject's state AFTER this call, so the next plan
	// starts from something current.
	Fingerprint string `json:"fingerprint,omitempty"`
	// Conflict is set when an unbound account already holds the derived name.
	// The operation stops and this carries what an operator needs to decide.
	Conflict *BindingConflict `json:"conflict,omitempty"`
}

// Effects, matching the backend's vocabulary so one renderer serves both.
const (
	EffectApplied  = "applied"
	EffectNoChange = "no_change"
	EffectBlocked  = "blocked"
	EffectApply    = "apply"
)

// BindingConflict is an unbound account already holding the derived name.
//
// Silently adopting it is the dangerous outcome: that account may belong to
// somebody else entirely, and adopting it hands them a subject's entitlements
// along with whatever is already in its home directory. So the operation stops
// and an operator chooses — adopt, or create under a suffixed name.
type BindingConflict struct {
	Username string `json:"username"`
	UID      int64  `json:"uid"`
	// Adoptable says the account exists and no other subject is bound to it,
	// which is the only case the adoption action can act on.
	Adoptable bool   `json:"adoptable"`
	BoundTo   string `json:"bound_to,omitempty"`
}

var errStaleFingerprint = errors.New("the subject's state on the target has changed since the plan")

// desiredState is the request's fields, decoded once into the shapes this
// add-on actually converges.
type desiredState struct {
	groups     []string
	enabled    bool
	smbEnabled bool
	// managed records which fields the request named at all. A field the
	// backend did not send is one this target does not manage for that subject,
	// and converging it to a zero value would be inventing an instruction.
	managed map[string]bool
}

func decodeDesired(raw map[string]json.RawMessage) (desiredState, error) {
	d := desiredState{managed: map[string]bool{}}
	for field, value := range raw {
		d.managed[field] = true
		switch field {
		case FieldGroup:
			if err := json.Unmarshal(value, &d.groups); err != nil {
				return desiredState{}, fmt.Errorf("field %q must be a list of group names", field)
			}
			sort.Strings(d.groups)
		case FieldEnabled:
			if err := json.Unmarshal(value, &d.enabled); err != nil {
				return desiredState{}, fmt.Errorf("field %q must be a boolean", field)
			}
		case FieldSMBEnabled:
			if err := json.Unmarshal(value, &d.smbEnabled); err != nil {
				return desiredState{}, fmt.Errorf("field %q must be a boolean", field)
			}
		default:
			// A field this add-on does not understand is refused rather than
			// ignored. Ignoring it would let the backend believe it had
			// converged something nothing acted on — the same class of silence
			// as dropping an unknown parameter.
			return desiredState{}, fmt.Errorf("field %q is not in this add-on's entitlement schema", field)
		}
	}
	return d, nil
}

// fingerprintSubject digests a subject's state as this add-on sees it.
//
// Length-prefixed, for the reason the backend's is: any separator can appear
// inside a group name, and two field lists hashing alike is a collision an
// attacker picks rather than finds. Absence has its own token, because "no
// account" and "an account with no groups" are different states and a plan
// computed against one must not verify against the other.
func fingerprintSubject(s *Subject) string {
	h := sha256.New()
	write := func(f string) { fmt.Fprintf(h, "%d:", len(f)); h.Write([]byte(f)) }
	if s == nil {
		write("absent")
		return hex.EncodeToString(h.Sum(nil))
	}
	write("present")
	write(s.Username)
	write(strconv.FormatInt(s.UID, 10))
	write(strconv.FormatBool(s.Enabled))
	write(strconv.FormatBool(s.SMBEnabled))
	groups := append([]string(nil), s.Groups...)
	sort.Strings(groups)
	for _, g := range groups {
		write(g)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// handleApply converges one subject.
func (s *server) handleApply(w http.ResponseWriter, r *http.Request, body []byte) {
	var req ApplyRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "BAD_REQUEST"})
		return
	}
	if strings.TrimSpace(req.CallID) == "" || strings.TrimSpace(req.Subject) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "BAD_REQUEST"})
		return
	}

	// Deduplicated before the lifecycle gate, so a replay during a maintenance
	// window returns what it returned rather than being refused — the call
	// already happened, and refusing it now would report a completed mutation
	// as queued.
	if cached, found, err := s.store.Recall(req.CallID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "STORE_UNREADABLE"})
		return
	} else if found {
		// Decoded and re-encoded rather than echoed. A replay must be
		// byte-identical to the original response — a caller comparing the two
		// to decide whether its retry duplicated anything would otherwise be
		// told they differ because one went through a different encoder.
		var previous ApplyOutcome
		if err := json.Unmarshal(cached, &previous); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "STORE_UNREADABLE"})
			return
		}
		writeJSON(w, http.StatusOK, previous)
		return
	}

	done, err := s.life.Begin()
	if err != nil {
		state, _ := s.life.State()
		writeLifecycleRefusal(w, state)
		return
	}
	defer done()

	if supported, why := s.nas.MajorSupported(); !supported {
		// A mutation against an untested major is refused, not attempted. Reads
		// continue; this is the half that could break something.
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{
			"error": "TARGET_VERSION_UNSUPPORTED", "detail": why,
		})
		return
	}

	outcome, status, err := s.applyOne(req)
	if err != nil {
		writeJSON(w, status, map[string]string{"error": "APPLY_FAILED", "detail": err.Error()})
		return
	}

	// The record is written BEFORE the result is returned, so a caller that
	// retries after a lost response gets the original outcome rather than a
	// second mutation. It is best-effort against the response, though: the
	// mutation already happened, and refusing to report it because the cache
	// write failed would lose the only account of it the caller gets.
	if err := s.store.Remember(req.CallID, outcome); err != nil {
		logStoreFailure("idempotency", req.CallID, err)
	}
	writeJSON(w, status, outcome)
}

// applyOne is the convergence.
func (s *server) applyOne(req ApplyRequest) (ApplyOutcome, int, error) {
	desired, err := decodeDesired(req.Desired)
	if err != nil {
		return ApplyOutcome{}, http.StatusBadRequest, err
	}

	current, binding, conflict, err := s.locate(req)
	if err != nil {
		return ApplyOutcome{}, http.StatusBadGateway, err
	}
	if conflict != nil {
		// Halted, and surfacing THE SAME adoption action the unmanaged
		// inventory offers. One decision with two entry points, never a second
		// code path that fires mid-convergence.
		return ApplyOutcome{
			Subject: req.Subject, Effect: EffectBlocked,
			Detail:      fmt.Sprintf("An account named %s already exists and is not bound to anyone.", conflict.Username),
			Consequence: "Nothing was changed. Adopt it, or create under a suffixed name.",
			Username:    conflict.Username, Conflict: conflict,
		}, http.StatusConflict, nil
	}

	// Verified against what was just read, immediately before the write. A
	// fingerprint checked earlier would be a statement about a moment that has
	// already passed.
	if req.Fingerprint != "" && req.Fingerprint != fingerprintSubject(current) {
		return ApplyOutcome{}, http.StatusConflict, fmt.Errorf("%w: %s", errStaleFingerprint, req.Subject)
	}

	if current == nil {
		return s.createAndConverge(req, desired, binding)
	}
	return s.converge(req, desired, current)
}

// locate finds the subject's account, by binding first and derivation second.
//
// The binding is authoritative and the derivation is a recovery path. Asking
// the other way round would rename an account every time somebody's email
// changed, which is the one thing §11 forbids.
func (s *server) locate(req ApplyRequest) (*Subject, Binding, *BindingConflict, error) {
	snap, err := s.readSubjects()
	if err != nil {
		return nil, Binding{}, nil, err
	}
	byName := make(map[string]*Subject, len(snap.Subjects))
	byUID := make(map[int64]*Subject, len(snap.Subjects))
	for i := range snap.Subjects {
		byName[snap.Subjects[i].Username] = &snap.Subjects[i]
		byUID[snap.Subjects[i].UID] = &snap.Subjects[i]
	}

	binding, bound, err := s.store.GetBinding(req.Subject)
	if err != nil {
		return nil, Binding{}, nil, err
	}
	if bound {
		if found, ok := byName[binding.Username]; ok {
			return found, binding, nil, nil
		}
		// The name moved. A stable uid whose username changed out of band is a
		// RENAME, not a missing account — reporting it as missing would create
		// a replacement while the original kept the home data.
		if found, ok := byUID[binding.UID]; ok {
			binding.Username = found.Username
			if err := s.store.PutBinding(binding); err != nil {
				logStoreFailure("binding", req.Subject, err)
			}
			return found, binding, nil, nil
		}
		// Bound, and neither the name nor the uid is there. The account was
		// deleted out of band; convergence re-creates it under the recorded
		// name rather than re-deriving, because the recorded name is what every
		// ACL and share still refers to.
		return nil, binding, nil, nil
	}

	claimed, err := s.store.BoundUsernames()
	if err != nil {
		return nil, Binding{}, nil, err
	}
	// The suffix resolves collisions with accounts THIS ADD-ON manages, and
	// nothing else. An unbound account holding the derived name is not a
	// collision to route around — it is an operator decision, because that
	// account may belong to somebody else entirely and suffixing past it would
	// leave two accounts for one person with the older one still holding the
	// home directory every share points at.
	derived := DeriveUsername(req.Email, req.Subject, func(candidate string) bool {
		owner, ok := claimed[candidate]
		return ok && owner != req.Subject
	})

	if existing, ok := byName[derived]; ok {
		owner := claimed[derived]
		return nil, Binding{}, &BindingConflict{
			Username: derived, UID: existing.UID,
			Adoptable: owner == "", BoundTo: owner,
		}, nil
	}
	return nil, Binding{SubjectID: req.Subject, Username: derived}, nil, nil
}

// converge brings an existing account onto the desired state.
func (s *server) converge(req ApplyRequest, desired desiredState, current *Subject) (ApplyOutcome, int, error) {
	update := map[string]any{}

	if desired.managed[FieldGroup] && !sameSet(current.Groups, desired.groups) {
		// A full replace, which is what makes this level-triggered: the same
		// call grants and revokes, and re-issuing it is a no-op.
		update["groups"] = desired.groups
	}
	if desired.managed[FieldEnabled] && current.Enabled != desired.enabled {
		update["locked"] = !desired.enabled
	}
	if desired.managed[FieldSMBEnabled] && current.SMBEnabled != desired.smbEnabled {
		update["smb"] = desired.smbEnabled
	}

	if len(update) == 0 {
		// Re-applying an unchanged set issues no mutating call at all. Not an
		// optimisation: the drain re-drives rows, so an apply that wrote every
		// time would rewrite every account on every pass and fill the mutation
		// log with events that changed nothing.
		return ApplyOutcome{
			Subject: req.Subject, Effect: EffectNoChange,
			Detail:      "Already in the requested state.",
			Username:    current.Username,
			Fingerprint: fingerprintSubject(current),
		}, http.StatusOK, nil
	}

	if err := s.nas.call("user.update", []any{current.UID, update}, nil); err != nil {
		return ApplyOutcome{}, statusFor(err), err
	}
	applied := *current
	if desired.managed[FieldGroup] {
		applied.Groups = desired.groups
	}
	if desired.managed[FieldEnabled] {
		applied.Enabled = desired.enabled
	}
	if desired.managed[FieldSMBEnabled] {
		applied.SMBEnabled = desired.smbEnabled
	}

	s.record("apply", req.Subject, req.CallID, "succeeded")
	return ApplyOutcome{
		Subject: req.Subject, Effect: EffectApplied,
		Detail:      describeChange(update),
		Consequence: describeHolding(applied),
		Username:    applied.Username,
		Fingerprint: fingerprintSubject(&applied),
	}, http.StatusOK, nil
}

// createAndConverge makes the account as part of the convergence.
func (s *server) createAndConverge(req ApplyRequest, desired desiredState, binding Binding) (ApplyOutcome, int, error) {
	if !ValidUsername(binding.Username) {
		return ApplyOutcome{}, http.StatusUnprocessableEntity,
			fmt.Errorf("derived name %q is not valid on this target", binding.Username)
	}
	create := map[string]any{
		"username": binding.Username,
		// `full_name` is required by the middleware and is display only. The
		// subject id rather than a name from the directory: this add-on is not
		// a copy of the identity provider, and a stale display name here would
		// be one more thing that disagrees with Zitadel.
		"full_name":    binding.Username,
		"group_create": true,
		"smb":          desired.managed[FieldSMBEnabled] && desired.smbEnabled,
		"locked":       desired.managed[FieldEnabled] && !desired.enabled,
	}
	if desired.managed[FieldGroup] {
		create["groups"] = desired.groups
	}

	var uid int64
	if err := s.nas.call("user.create", []any{create}, &uid); err != nil {
		return ApplyOutcome{}, statusFor(err), err
	}

	binding.SubjectID, binding.UID, binding.BoundBy = req.Subject, uid, "apply"
	if err := s.store.PutBinding(binding); err != nil {
		// The account exists and the record of whose it is did not land. Not a
		// silent success: the next apply would derive the same name, find it
		// unbound, and halt as a binding conflict — which is recoverable but
		// needs an operator, so it is reported now rather than discovered then.
		logStoreFailure("binding", req.Subject, err)
		return ApplyOutcome{}, http.StatusInternalServerError,
			fmt.Errorf("account %s was created and its binding could not be recorded: %w", binding.Username, err)
	}

	created := Subject{
		Username: binding.Username, UID: uid, Groups: desired.groups,
		Enabled:    !desired.managed[FieldEnabled] || desired.enabled,
		SMBEnabled: desired.managed[FieldSMBEnabled] && desired.smbEnabled,
	}
	s.record("apply.create", req.Subject, req.CallID, "succeeded")
	return ApplyOutcome{
		Subject: req.Subject, Effect: EffectApplied,
		Detail:      fmt.Sprintf("Created %s.", binding.Username),
		Consequence: describeHolding(created),
		Username:    binding.Username,
		Fingerprint: fingerprintSubject(&created),
	}, http.StatusOK, nil
}

// record appends to the mutation log, best-effort AFTER the fact.
//
// Deliberately not fatal to the response: the mutation already happened, and
// failing the call because the record failed would tell the caller nothing
// happened when something did. The failure is logged loudly and the head digest
// the backend anchors is what makes a lost record visible.
func (s *server) record(operation, subject, callID, outcome string) {
	if _, err := s.log.Append(operation, subject, subject, callID, outcome); err != nil {
		logStoreFailure("mutation log", callID, err)
	}
}

func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	x := append([]string(nil), a...)
	y := append([]string(nil), b...)
	sort.Strings(x)
	sort.Strings(y)
	for i := range x {
		if x[i] != y[i] {
			return false
		}
	}
	return true
}

func describeChange(update map[string]any) string {
	var parts []string
	if g, ok := update["groups"].([]string); ok {
		if len(g) == 0 {
			parts = append(parts, "removed from every managed group")
		} else {
			parts = append(parts, "groups set to "+strings.Join(g, ", "))
		}
	}
	if locked, ok := update["locked"].(bool); ok {
		if locked {
			parts = append(parts, "account disabled")
		} else {
			parts = append(parts, "account enabled")
		}
	}
	if smb, ok := update["smb"].(bool); ok {
		if smb {
			parts = append(parts, "SMB enabled")
		} else {
			parts = append(parts, "SMB disabled")
		}
	}
	return strings.Join(parts, "; ") + "."
}

// describeHolding is what the subject is left with — the part a count never
// tells you, and what `BulkOutcome.Consequence` is for.
func describeHolding(s Subject) string {
	switch {
	case !s.Enabled:
		return "The account is disabled and reaches nothing until it is enabled again."
	case len(s.Groups) == 0:
		return "The account is enabled and in no managed group, so it reaches nothing."
	default:
		return "Reaches " + strings.Join(s.Groups, ", ") + "."
	}
}

func statusFor(err error) int {
	switch {
	case errors.Is(err, ErrTargetUnreachable):
		return http.StatusServiceUnavailable
	case errors.Is(err, ErrRateLimited):
		return http.StatusTooManyRequests
	default:
		return http.StatusBadGateway
	}
}
