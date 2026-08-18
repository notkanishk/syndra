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
	// ContractVersion is the wire version the caller speaks. Declared here
	// because `decodeStrict` refuses what it does not understand, so a field the
	// backend sends and this struct omits is not a tolerated extra — it is a
	// 400 on every call, which is precisely how this arrived: both sides were
	// tested against their own fake and the two fakes agreed with each other.
	ContractVersion int    `json:"contract_version"`
	CallID          string `json:"call_id"`
	Subject         string `json:"subject"`
	// Email is what a username is derived from, and only when an account has to
	// be created. An existing binding is authoritative: a later email change
	// MUST NOT rename an account, because renaming disturbs its home directory,
	// its ACL entries and its SMB identity.
	Email string `json:"email"`
	// Fingerprint is the target state the plan was computed against, echoed
	// back so this call can refuse if the subject moved in between. Required:
	// an absent one verifies vacuously, and "the diff you approved is the diff
	// that lands" is not a property a caller gets to opt out of.
	Fingerprint string `json:"fingerprint,omitempty"`
	// PlanID is the approval this apply executes, recorded rather than checked:
	// the add-on holds no plans, and what it can do is name in its own log what
	// authorised the write. In signed mode it is inside the MAC, so a call
	// re-aimed at another approval fails verification.
	PlanID string `json:"plan_id,omitempty"`
	// Actor is who the backend recorded as deciding this. The mutation log
	// promises who did what to whom, and a record naming only the subject
	// answers two thirds of that.
	Actor string `json:"actor,omitempty"`
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
	// UID is the target's stable identity for that account. Reported beside the
	// name because the name can move out of band and this cannot: a backend
	// recording only the name would lose track of the account the first time
	// somebody renamed it in the web UI.
	UID int64 `json:"uid,omitempty"`
	// Fingerprint is the subject's state AFTER this call, so the next plan
	// starts from something current.
	Fingerprint string `json:"fingerprint,omitempty"`
	// Observed is the managed fields as the TARGET reported them after the
	// write, never as this add-on asked for them.
	//
	// The distinction is the whole point of the field. A value assembled from
	// the request agrees with the request by construction, so it can confirm
	// nothing and, recorded as a merge base, would make every later difference
	// resolve as "the target is wrong" — which is the state this add-on was in.
	// Absent when the read after the write did not happen; see Unverified.
	Observed map[string]any `json:"observed,omitempty"`
	// Unverified says the mutation landed and its result could not be read.
	//
	// A separate axis from the effect, deliberately. "Did the write happen" and
	// "do we know what it produced" are different questions, and collapsing
	// them either reports a completed write as failed — inviting a retry of
	// something already done — or reports an unverified state as confirmed,
	// which is the lie this whole change is about. Fingerprint and Observed are
	// both omitted whenever this is set, so nothing downstream can mistake an
	// assumption for a reading.
	Unverified bool `json:"unverified,omitempty"`
	// Conflict is set when an unbound account already holds the derived name.
	// The operation stops and this carries what an operator needs to decide.
	Conflict *BindingConflict `json:"conflict,omitempty"`
}

// observedFields is the managed half of a subject as the target reports it.
//
// Managed fields only, matching what the merge compares: an unmanaged field is
// not "unchanged", it is out of scope, and carrying it would invite a base that
// claims authority over something Syndra never set.
func observedFields(desired desiredState, s *Subject) map[string]any {
	if s == nil {
		return nil
	}
	out := map[string]any{}
	if desired.managed[FieldGroup] {
		groups := append([]string(nil), s.Groups...)
		sort.Strings(groups)
		out[FieldGroup] = groups
	}
	if desired.managed[FieldEnabled] {
		out[FieldEnabled] = s.Enabled
	}
	if desired.managed[FieldSMBEnabled] {
		out[FieldSMBEnabled] = s.SMBEnabled
	}
	if len(out) == 0 {
		return nil
	}
	return out
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

// errNoIdentityToDeriveFrom is an apply that would have to CREATE an account and
// carries nothing to name it after. The derivation is deterministic from the
// email localpart, so a call without one produces a name derived from the
// subject id — valid, stable, and not the name the plan predicted.
var errNoIdentityToDeriveFrom = errors.New("no identity to derive a username from")

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
	if err := decodeStrict(body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "BAD_REQUEST"})
		return
	}
	if !writeContractRefusal(w, req.ContractVersion) {
		return
	}
	if strings.TrimSpace(req.CallID) == "" || strings.TrimSpace(req.Subject) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "BAD_REQUEST"})
		return
	}
	if strings.TrimSpace(req.Fingerprint) == "" {
		// A missing fingerprint is refused, not skipped. §8's guarantee is that
		// the diff an operator approved is the diff that lands, and a check that
		// silently passes when the field is absent is a guarantee that holds
		// only for callers who chose to be bound by it.
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error":  "FINGERPRINT_REQUIRED",
			"detail": "this call carries no fingerprint, so there is nothing to verify the plan against",
		})
		return
	}

	// Deduplicated before the lifecycle gate, so a replay during a maintenance
	// window returns what it returned rather than being refused — the call
	// already happened, and refusing it now would report a completed mutation
	// as queued.
	if cached, found, err := s.store.Recall(req.CallID, kindApply); err != nil {
		writeRecallFailure(w, err)
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
		if errors.Is(err, errStaleFingerprint) {
			// Its own code, because it is its own operator action. Every other
			// refusal here means "fix something and try again"; this one means
			// the subject moved since the diff was approved, and what has to
			// happen next is a re-plan somebody reads.
			writeJSON(w, status, map[string]string{
				"error":   "PLAN_STALE",
				"subject": req.Subject,
				"detail":  "the subject's state on the target has changed since the plan was computed",
			})
			return
		}
		writeJSON(w, status, map[string]string{"error": "APPLY_FAILED", "detail": err.Error()})
		return
	}

	// The record is written BEFORE the result is returned, so a caller that
	// retries after a lost response gets the original outcome rather than a
	// second mutation. It is best-effort against the response, though: the
	// mutation already happened, and refusing to report it because the cache
	// write failed would lose the only account of it the caller gets.
	if err := s.store.Remember(req.CallID, kindApply, outcome); err != nil {
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

	current, binding, conflict, truncated, err := s.locate(req)
	if errors.Is(err, errNoIdentityToDeriveFrom) {
		// Blocked rather than a transport-shaped failure: nothing is wrong with
		// the target, and retrying the identical call will not fix it.
		return ApplyOutcome{
			Subject: req.Subject, Effect: EffectBlocked,
			Detail:      "This subject has no account here and the call carried no identity to derive a name from.",
			Consequence: "Nothing was changed.",
		}, http.StatusUnprocessableEntity, nil
	}
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
	if req.Fingerprint != fingerprintSubject(current) {
		return ApplyOutcome{}, http.StatusConflict, fmt.Errorf("%w: %s", errStaleFingerprint, req.Subject)
	}

	if current == nil {
		if truncated {
			// The same refusal `plan` makes, on the path that actually writes.
			// An absence read out of a capped list is not an absence, and the
			// fingerprint cannot tell the two apart — "not in the read" and
			// "not on the target" produce the identical digest. Without this
			// the plan blocked the create and the apply made it anyway, which
			// is a second account for somebody who already has one, past the
			// cap, still holding the home directory every share points at.
			return ApplyOutcome{
				Subject: req.Subject, Effect: EffectBlocked,
				Detail:      "The account list was longer than one read returns, so this subject's absence from it proves nothing.",
				Consequence: "Nothing was changed.",
			}, http.StatusConflict, nil
		}
		return s.createAndConverge(req, desired, binding)
	}
	return s.converge(req, desired, current)
}

// locate finds the subject's account, by binding first and derivation second.
//
// The binding is authoritative and the derivation is a recovery path. Asking
// the other way round would rename an account every time somebody's email
// changed, which is the one thing §11 forbids.
// The fourth return value says the read hit the cap. Carried out rather than
// acted on here, because it only matters where an ABSENCE is concluded: a
// subject found by binding is found whether or not the list was capped.
func (s *server) locate(req ApplyRequest) (*Subject, Binding, *BindingConflict, bool, error) {
	snap, err := s.readSubjects()
	if err != nil {
		return nil, Binding{}, nil, false, err
	}
	byName := make(map[string]*Subject, len(snap.Subjects))
	byUID := make(map[int64]*Subject, len(snap.Subjects))
	for i := range snap.Subjects {
		byName[snap.Subjects[i].Username] = &snap.Subjects[i]
		byUID[snap.Subjects[i].UID] = &snap.Subjects[i]
	}

	binding, bound, err := s.store.GetBinding(req.Subject)
	if err != nil {
		return nil, Binding{}, nil, false, err
	}
	if bound {
		if found, ok := byName[binding.Username]; ok {
			return found, binding, nil, snap.Truncated, nil
		}
		// The name moved. A stable uid whose username changed out of band is a
		// RENAME, not a missing account — reporting it as missing would create
		// a replacement while the original kept the home data.
		if found, ok := byUID[binding.UID]; ok {
			binding.Username = found.Username
			if err := s.store.PutBinding(binding); err != nil {
				logStoreFailure("binding", req.Subject, err)
			}
			return found, binding, nil, snap.Truncated, nil
		}
		// Bound, and neither the name nor the uid is there. The account was
		// deleted out of band; convergence re-creates it under the recorded
		// name rather than re-deriving, because the recorded name is what every
		// ACL and share still refers to.
		return nil, binding, nil, snap.Truncated, nil
	}

	if strings.TrimSpace(req.Email) == "" {
		// No binding and no identity to derive a name from. The fallback exists
		// for a localpart that normalizes to nothing, not for a caller that sent
		// no localpart at all: applied here it would create `u3f2a9c11` for
		// somebody whose plan said `ada`, and the plan is what an operator
		// approved. Refused so the disagreement is visible instead of being a
		// name nobody chose.
		return nil, Binding{}, nil, false, errNoIdentityToDeriveFrom
	}

	claimed, err := s.store.BoundUsernames()
	if err != nil {
		return nil, Binding{}, nil, false, err
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
		}, snap.Truncated, nil
	}
	return nil, Binding{SubjectID: req.Subject, Username: derived}, nil, snap.Truncated, nil
}

// converge brings an existing account onto the desired state.
func (s *server) converge(req ApplyRequest, desired desiredState, current *Subject) (ApplyOutcome, int, error) {
	update := map[string]any{}

	if desired.managed[FieldGroup] && !sameSet(current.Groups, desired.groups) {
		// A full replace, which is what makes this level-triggered: the same
		// call grants and revokes, and re-issuing it is a no-op.
		//
		// Sent as record ids, which is what the read resolved names FROM. A
		// write in names against a read in ids is the asymmetry where an
		// account lands in the wrong groups and the next read calls it
		// converged.
		ids, err := s.groupIDsFor(desired.groups)
		if err != nil {
			return ApplyOutcome{}, http.StatusUnprocessableEntity, err
		}
		update["groups"] = ids
	}
	if desired.managed[FieldEnabled] && current.Enabled != desired.enabled {
		update["locked"] = !desired.enabled
	}
	// Turning SMB ON requires a credential to hash, and TrueNAS says so by
	// refusing the call — not by ignoring the field. Attempting it against an
	// account whose member has not set a password yet fails the whole apply,
	// on every pass, for a state the target considers normal. Turning it OFF is
	// always allowed, and is the half that matters for revocation.
	smbPending := false
	if desired.managed[FieldSMBEnabled] && current.SMBEnabled != desired.smbEnabled {
		if desired.smbEnabled && !current.PasswordSet {
			smbPending = true
		} else {
			update["smb"] = desired.smbEnabled
		}
	}

	if len(update) == 0 && smbPending {
		// Nothing to write, and something still wanted. Reported as its own
		// outcome rather than as no-change: "already in the requested state" is
		// false, and reporting it would leave an operator believing SMB is on.
		return ApplyOutcome{
			Subject: req.Subject, Effect: EffectNoChange,
			Detail: "Waiting for " + current.Username + " to set a password before SMB can be enabled.",
			Consequence: "The account exists and is otherwise converged. TrueNAS refuses SMB " +
				"on an account with no password, so this completes itself when the member sets one.",
			Username:    current.Username,
			UID:         current.UID,
			Fingerprint: fingerprintSubject(current),
		}, http.StatusOK, nil
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
			UID:         current.UID,
			Fingerprint: fingerprintSubject(current),
		}, http.StatusOK, nil
	}

	if err := s.nas.call("user.update", []any{current.ID, update}, nil); err != nil {
		return ApplyOutcome{}, statusFor(err), err
	}

	// Read back what the target now holds. This used to be
	//
	//     applied := *current                      // then overwrite the
	//     applied.Groups = desired.groups          // managed fields with the
	//     applied.Enabled = desired.enabled        // values we REQUESTED
	//
	// and fingerprint that. Which meant the dominant path — every update to an
	// account that already exists — reported intent in the shape of an
	// observation. The create path's own comment already stated the rule it
	// broke: a fingerprint computed from a state this add-on invented is a
	// fingerprint the next plan verifies against nothing.
	//
	// It is not hypothetical. `smb` is refused on an account with no password;
	// TrueNAS normalises and resolves values on write; a middleware that
	// coerces a field answers 200 and stores something else. In every one of
	// those the projection says the write landed as asked, the next plan
	// compares its fingerprint against that claim, and the check passes over a
	// difference it exists to catch.
	//
	// The same call the plan path already makes, so this is a second read per
	// applied subject rather than a new class of work.
	applied, err := s.readBack(current.Username)
	if err != nil {
		// The write happened and its result is unknown. Reported as both of
		// those things: the effect is `applied`, because it was, and nothing
		// that would have to be an observation is sent. Recording it as a
		// failure would invite a retry of a mutation already performed;
		// recording it as an ordinary success would hand the next plan a
		// fingerprint nobody read.
		s.record("apply.unverified", req.Subject, req.Actor, req.CallID, "succeeded")
		return ApplyOutcome{
			Subject: req.Subject, Effect: EffectApplied, Unverified: true,
			// The statement goes in the DETAIL, not only in the consequence:
			// the backend decodes `detail` and does not decode `consequence`,
			// so a sentence that lives only in the second reaches no operator
			// surface at all. The flag beside it is for the consumer that will
			// store bases; the sentence is for the person reading today.
			Detail: describeChange(update, desired.groups) +
				" The account could not be read back afterwards, so what the target now holds has not been confirmed.",
			Consequence: "The change was written to " + current.Username +
				" and the account could not be read back, so what the target now holds " +
				"has not been confirmed. The next plan reads it fresh.",
			Username: current.Username,
			UID:      current.UID,
		}, http.StatusOK, nil
	}

	s.record("apply", req.Subject, req.Actor, req.CallID, "succeeded")
	return ApplyOutcome{
		Subject: req.Subject, Effect: EffectApplied,
		Detail:      describeChange(update, desired.groups),
		Consequence: describeHolding(*applied),
		Username:    applied.Username,
		UID:         applied.UID,
		Fingerprint: fingerprintSubject(applied),
		Observed:    observedFields(desired, applied),
	}, http.StatusOK, nil
}

// createAndConverge makes the account as part of the convergence.
func (s *server) createAndConverge(req ApplyRequest, desired desiredState, binding Binding) (ApplyOutcome, int, error) {
	if !ValidUsername(binding.Username) {
		return ApplyOutcome{}, http.StatusUnprocessableEntity,
			fmt.Errorf("derived name %q is not valid on this target", binding.Username)
	}
	var groupIDs []apiID
	if desired.managed[FieldGroup] {
		ids, err := s.groupIDsFor(desired.groups)
		if err != nil {
			return ApplyOutcome{}, http.StatusUnprocessableEntity, err
		}
		groupIDs = ids
	}
	create := map[string]any{
		"username": binding.Username,
		// `full_name` is required by the middleware and is display only. The
		// subject id rather than a name from the directory: this add-on is not
		// a copy of the identity provider, and a stale display name here would
		// be one more thing that disagrees with Zitadel.
		"full_name":    binding.Username,
		"group_create": true,
		// A credential decision is REQUIRED at creation: TrueNAS refuses
		// `user.create` with neither `password` nor `password_disabled`
		// (`user_create.password: Password is required`). Syndra has no
		// password here and must not invent one — the member sets their own
		// through `password.set`, and a random one minted here would be a
		// credential that exists, is returned in the create response, and
		// nobody asked for.
		"password_disabled": true,
		// And SMB is NOT requested here even when it is wanted, because
		// TrueNAS refuses the pair outright: `Password authentication may not
		// be disabled for SMB users.` It is turned on by the same call that
		// sets the first password, which is the only ordering TrueNAS accepts
		// — a later `user.update({smb: true})` is refused with `Password must
		// be reset in order to enable SMB authentication`.
		"smb":    false,
		"locked": desired.managed[FieldEnabled] && !desired.enabled,
	}
	if desired.managed[FieldGroup] {
		create["groups"] = groupIDs
	}

	var recordID apiID
	if err := s.nas.call("user.create", []any{create}, &recordID); err != nil {
		return ApplyOutcome{}, statusFor(err), err
	}
	// `user.create` answers with the RECORD key, not the unix uid, and the
	// binding recognises an account by its uid across a rename. Read back
	// rather than assumed: a fingerprint computed from a state this add-on
	// invented is a fingerprint the next plan verifies against nothing.
	created, err := s.readBack(binding.Username)
	if err != nil {
		return ApplyOutcome{}, statusFor(err),
			fmt.Errorf("account %s was created and could not be read back: %w", binding.Username, err)
	}

	binding.SubjectID, binding.UID, binding.BoundBy = req.Subject, created.UID, "apply"
	if err := s.store.PutBinding(binding); err != nil {
		// The account exists and the record of whose it is did not land. Not a
		// silent success: the next apply would derive the same name, find it
		// unbound, and halt as a binding conflict — which is recoverable but
		// needs an operator, so it is reported now rather than discovered then.
		logStoreFailure("binding", req.Subject, err)
		return ApplyOutcome{}, http.StatusInternalServerError,
			fmt.Errorf("account %s was created and its binding could not be recorded: %w", binding.Username, err)
	}

	s.record("apply.create", req.Subject, req.Actor, req.CallID, "succeeded")
	return ApplyOutcome{
		Subject: req.Subject, Effect: EffectApplied,
		Detail:      fmt.Sprintf("Created %s.", binding.Username),
		Consequence: describeHolding(*created),
		Username:    binding.Username,
		UID:         created.UID,
		Fingerprint: fingerprintSubject(created),
		Observed:    observedFields(desired, created),
	}, http.StatusOK, nil
}

// readBack reads one account as the target now holds it.
func (s *server) readBack(username string) (*Subject, error) {
	snap, err := s.readSubjects()
	if err != nil {
		return nil, err
	}
	for i := range snap.Subjects {
		if snap.Subjects[i].Username == username {
			return &snap.Subjects[i], nil
		}
	}
	return nil, fmt.Errorf("%w: %s is not on the target after being created", ErrTargetRefused, username)
}

// record appends to the mutation log, best-effort AFTER the fact.
//
// Deliberately not fatal to the response: the mutation already happened, and
// failing the call because the record failed would tell the caller nothing
// happened when something did. The failure is logged loudly and the head digest
// the backend anchors is what makes a lost record visible.
func (s *server) record(operation, subject, actor, callID, outcome string) {
	if strings.TrimSpace(actor) == "" {
		// Unstated, not invented. The call arrived authenticated as the backend
		// and named nobody, and recording the subject as their own actor —
		// which this did — makes every record answer "who" with "whom".
		actor = "syndra"
	}
	if _, err := s.log.Append(operation, subject, actor, callID, outcome); err != nil {
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

// describeChange says what changed, in the names an operator reads.
//
// The names come alongside rather than out of the update, because the update
// carries the ids the target takes and "groups set to 45, 46" is not something
// anybody can check.
func describeChange(update map[string]any, groupNames []string) string {
	var parts []string
	if _, ok := update["groups"]; ok {
		if len(groupNames) == 0 {
			parts = append(parts, "removed from every managed group")
		} else {
			parts = append(parts, "groups set to "+strings.Join(groupNames, ", "))
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
