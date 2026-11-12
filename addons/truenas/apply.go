package main

import (
	"bytes"
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
	// AccountID is the target's stable identity for that account (the uid,
	// as contract 2's string account_id). Reported beside the name because the
	// name can move out of band and this cannot: a backend recording only the
	// name would lose track of the account the first time somebody renamed it
	// in the web UI.
	AccountID string `json:"account_id,omitempty"`
	// Standing is whether that account can be used yet, and who is waited on
	// (design §6). It travels with AccountID, and only when an account exists.
	Standing *Standing `json:"standing,omitempty"`
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

// smbWaiting is what an account wanting SMB on says while it waits for the
// member's next password, the only call TrueNAS switches it on in. The plan
// says it too, so the two never disagree about a change only a password makes.
func smbWaiting(current *Subject) string {
	if current.PasswordSet {
		return "Waiting for " + current.Username + " to set their password again: TrueNAS switches SMB on only together with a new password."
	}
	return "Waiting for " + current.Username + " to set a password before SMB can be enabled."
}

// observedFields is the managed half of a subject as the target reports it.
//
// Managed fields only, matching what the merge compares: an unmanaged field is
// not "unchanged", it is out of scope, and carrying it would invite a base that
// claims authority over something Syndra never set.
//
// A managed limit is the effective limit the target now holds, read back from
// its quota table exactly as /subjects state reports it (homeLimits), so the
// backend's merge base has the limit straight after an apply; left out when
// the table could not be read (unknown, never "no limit").
func (srv *server) observedFields(desired desiredState, s *Subject) map[string]any {
	state := entitlementState(s)
	if state == nil {
		return nil
	}
	out := map[string]any{}
	for field, value := range state {
		if desired.managed[field] {
			out[field] = value
		}
	}
	if desired.managed[FieldHomeQuota] {
		if limit, read := srv.homeLimits([]Subject{*s})[s.UID]; read {
			out[FieldHomeQuota] = []string{limit}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// entitlementState is a subject in the vocabulary the manifest declares, rather
// than the vocabulary the NAS uses.
//
// One definition, used by both directions: the observed values an apply reports
// after a write, and the current values `/subjects` serves for a reconciliation
// to compare against. Two functions producing this map would be two answers to
// "what does this account hold", and the pair would disagree the first time a
// field was added to one of them.
//
// Groups are sorted because membership is a SET, and a target that returns them
// in its own order would otherwise read as a change on every pass.
func entitlementState(s *Subject) map[string]any {
	if s == nil {
		return nil
	}
	// [], never null, for no groups (contract 2, task 1.0a): a list field is
	// a list.
	groups := append([]string{}, s.Groups...)
	sort.Strings(groups)
	// The home in the mapping's own vocabulary: whether the account has one,
	// wherever it is, so a home made before the location changed still reads
	// as agreeing with the role. The limit is not reported: a failed quota read
	// would then read as drift, and the limit is set on every apply instead.
	home := []string{}
	if s.Home != "" {
		home = []string{"true"}
	}
	return map[string]any{
		FieldGroup:      groups,
		FieldEnabled:    s.Enabled,
		FieldSMBEnabled: s.SMBEnabled,
		FieldHome:       home,
	}
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
// and an operator chooses (BindingConflict.words).
type BindingConflict struct {
	Username  string `json:"username"`
	AccountID string `json:"account_id"`
	// Adoptable says the account exists, no other subject is bound to it and
	// it is known not to be the add-on's own, which is the only case the
	// adoption action can act on.
	Adoptable bool   `json:"adoptable"`
	BoundTo   string `json:"bound_to,omitempty"`
	// serviceAccount is the add-on's own account holding the name, which
	// account.adopt refuses (refuseSelfAccount). Not on the wire: adoptable
	// false already says the one thing Syndra acts on, and words says why.
	serviceAccount bool
	// selfUnknown is the add-on not yet knowing its own account (auth.me),
	// while account.adopt refuses everything; not on the wire either.
	selfUnknown bool
}

// unboundConflict is an account already holding the name an unbound subject
// derives, built here once for plan and apply. Adoptable is account.adopt's
// own rule (adoptAccount): no other subject holds the account, the add-on
// knows its own account, and this is not it (refuseSelfAccount, the same check).
func (s *server) unboundConflict(account *Subject, owner string) *BindingConflict {
	_, _, known := s.nas.Self()
	self := s.refuseSelfAccount(account.Username) != nil
	return &BindingConflict{
		Username: account.Username, AccountID: accountID(account.UID),
		Adoptable: owner == "" && known && !self, BoundTo: owner,
		serviceAccount: self, selfUnknown: !known,
	}
}

// leftByCrashedCreate is design §5's one adoption without a person (5.3d):
// the unbound account holding this subject's derived name is the one this
// subject's own create made and never recorded, because the add-on stopped,
// or the NAS stopped answering, between the create and its record. Plan and
// apply both ask it, so the plan never says "conflict" for an account the
// apply adopts, or the other way round. Every check must hold, or it stays
// the conflict an operator decides:
//   - the account's name is exactly the derived one;
//   - account.adopt's own rule allows it (conflict.Adoptable: no binding
//     names it, the add-on knows its own account and this is not it);
//   - its uid is in this add-on's range, so it is no system or built-in
//     account and none a person made by hand, and no binding holds that uid
//     under an older name;
//   - it is still exactly as a create leaves it: no usable password and SMB
//     off (the create sends password_disabled true and smb false; only a
//     member's password.set changes either). This one does not read the log,
//     so it holds when the log does not: a whole-volume restore brings back
//     an old create in doubt with the log and the store together, and an
//     account since adopted by someone, released, and adopted by another has
//     been used in between;
//   - the log holds a user.create intent for THIS subject that asked for that
//     very uid, whose result is unknown; no binding of that uid by anyone,
//     and none of this subject, logged since; and no other subject's create
//     of the uid still open (MutationLog.CreateInDoubt). A uid is usually
//     handed out once, but not always (an orphan group's gid is reused, a
//     restored store rolls ClaimID back), so the log is read across subjects
//     rather than the uid trusted to name one create.
func (s *server) leftByCrashedCreate(subject, derived string, account *Subject, conflict *BindingConflict) (bool, error) {
	floor, ceiling := s.idRange()
	if account.Username != derived || !conflict.Adoptable || account.UID < floor || account.UID > ceiling ||
		account.PasswordSet || account.SMBEnabled {
		return false, nil
	}
	if bound, err := s.store.UIDBound(account.UID); err != nil || bound {
		return false, err
	}
	return s.log.CreateInDoubt(subject, accountID(account.UID))
}

// adoptionWords prefixes an outcome with leftByCrashedCreate's adoption, once
// for plan and apply. An outcome that converged nothing still bound the
// account, so it says applied (the plan: apply), as the create it finishes
// would; a blocked plan is left as it is, and a blocked apply says the
// binding was recorded.
func adoptionWords(out ApplyOutcome, planned bool) ApplyOutcome {
	said := fmt.Sprintf("Adopted %s, which an earlier create made and could not record.", out.Username)
	if planned {
		said = fmt.Sprintf("Adopts %s, which an earlier create made and could not record.", out.Username)
	}
	switch out.Effect {
	case EffectNoChange:
		out.Effect = EffectApplied
		if planned {
			out.Effect = EffectApply
		}
	case EffectBlocked:
		if planned {
			return out
		}
		// Only a block that arose after the checks applyOne asks first (the
		// group table changed in between): the binding was written.
		out.Consequence = "Only the binding was recorded; nothing on the account was changed."
	}
	out.Detail = strings.TrimSpace(said + " " + out.Detail)
	return out
}

// words says what a conflict is and what the operator does next, once for
// plan and apply. adoptable is exactly when account.adopt would succeed
// (adoptAccount): no other subject holds the account, it is known not to be
// the add-on's own, and this subject has no binding. A subject whose bound
// name another account now holds has a binding, so adopt refuses it: the
// operator releases first.
func (c BindingConflict) words() (detail, next string) {
	switch {
	case c.serviceAccount:
		return fmt.Sprintf("%s is the account this add-on signs in to the app with, so it is never adopted.", c.Username),
			"Create this subject's account in the app under another name, then adopt that."
	case c.Adoptable:
		return fmt.Sprintf("An account named %s already exists and is not bound to anyone.", c.Username),
			"Adopt it, or rename it in the app."
	case c.BoundTo != "":
		return fmt.Sprintf("An account named %s already exists and is bound to another subject.", c.Username),
			"If it is not theirs, release it from that subject, then adopt it here."
	case c.selfUnknown:
		return fmt.Sprintf("An account named %s already exists, and Syndra can't check yet whether it is the account this add-on signs in to the app with.", c.Username),
			"Try again shortly."
	default:
		return fmt.Sprintf("The name %s this subject is bound to is now held by a different account.", c.Username),
			"Release this subject's binding first, then adopt that account only if it is theirs, or rename it in the app."
	}
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
	// wantsHome is a role giving its holder a home folder. Where it goes is
	// the system's one home location, homeRoot, filled from the add-on's own
	// setting rather than sent per role: two roles can't disagree about it.
	wantsHome bool
	homeRoot  string
	// homeQuota is the largest limit any of their roles (or an operator, for
	// this one person) gives, in bytes. Zero when none was given.
	// homeUnlimited is a role or an operator giving "No limit" ("0"), which
	// is the largest limit there is: the folder's limit is lifted.
	homeQuota     int64
	homeUnlimited bool
	// managed records which fields the request named at all. A field the
	// backend did not send is one this target does not manage for that subject,
	// and converging it to a zero value would be inventing an instruction.
	managed map[string]bool
}

// unholdable is a desired state this app cannot hold: a key outside the
// entitlement schema, a value that does not fit its role, a group the NAS does
// not have or a built-in one. One error type, so /plan answers it `blocked`
// and /apply 422 APPLY_BLOCKED, and the two cannot disagree about which inputs
// those are (AUTHORING.md, "Every value is checked against its role").
type unholdable string

func (e unholdable) Error() string { return string(e) }

func cannotHold(format string, a ...any) error { return unholdable(fmt.Sprintf(format, a...)) }

// blockedOr answers an apply's failure before any write. One the app cannot
// hold is the blocked outcome (422 APPLY_BLOCKED, as the plan said); anything
// else keeps status.
func blockedOr(subject string, err error, status int) (ApplyOutcome, int, error) {
	var u unholdable
	if errors.As(err, &u) {
		return ApplyOutcome{
			Subject: subject, Effect: EffectBlocked,
			Detail: err.Error(), Consequence: "Nothing was changed.",
		}, http.StatusUnprocessableEntity, nil
	}
	return ApplyOutcome{}, status, err
}

// decodeDesired is decodeDesiredFields with the system's home location filled
// in, so the plan and the apply decide homes from the same place.
func (s *server) decodeDesired(raw map[string]json.RawMessage) (desiredState, error) {
	d, err := decodeDesiredFields(raw)
	if err != nil || !d.wantsHome {
		return d, err
	}
	d.homeRoot, err = s.homeRoot()
	return d, err
}

func decodeDesiredFields(raw map[string]json.RawMessage) (desiredState, error) {
	d := desiredState{managed: map[string]bool{}}
	for field, value := range raw {
		// JSON null decodes into Go's zero value without error, which here is
		// an instruction: `enabled: null` would lock the account and
		// `group: null` remove every managed group. FieldValue
		// (common/field.json) is an array or a boolean, never null, so it is
		// refused for every field before any of them is read.
		if string(bytes.TrimSpace(value)) == "null" {
			return desiredState{}, cannotHold("field %q is null, which is not a value; leave a field out to leave it alone", field)
		}
		d.managed[field] = true
		switch field {
		case FieldGroup:
			if err := json.Unmarshal(value, &d.groups); err != nil {
				return desiredState{}, cannotHold("field %q must be a list of group names", field)
			}
			sort.Strings(d.groups)
		case FieldEnabled:
			if err := json.Unmarshal(value, &d.enabled); err != nil {
				return desiredState{}, cannotHold("field %q must be a boolean", field)
			}
		case FieldSMBEnabled:
			if err := json.Unmarshal(value, &d.smbEnabled); err != nil {
				return desiredState{}, cannotHold("field %q must be a boolean", field)
			}
		case FieldHome:
			var gives []string
			if err := json.Unmarshal(value, &gives); err != nil {
				return desiredState{}, cannotHold("field %q must be a list of \"true\"", field)
			}
			for _, g := range gives {
				if g != "true" {
					return desiredState{}, cannotHold("field %q: a role either gives a home folder (\"true\") or names nothing, not %q", field, g)
				}
			}
			d.wantsHome = len(gives) > 0
		case FieldHomeQuota:
			var limits []string
			if err := json.Unmarshal(value, &limits); err != nil {
				return desiredState{}, cannotHold("field %q must be a list of byte counts", field)
			}
			for _, l := range limits {
				n, err := strconv.ParseInt(strings.TrimSpace(l), 10, 64)
				if err != nil || n < 0 {
					return desiredState{}, cannotHold("field %q: %q is not a byte count", field, l)
				}
				if n == 0 {
					d.homeUnlimited = true
				}
				if n > d.homeQuota {
					d.homeQuota = n
				}
			}
		default:
			// A field this add-on does not understand is refused rather than
			// ignored. Ignoring it would let the backend believe it had
			// converged something nothing acted on — the same class of silence
			// as dropping an unknown parameter.
			return desiredState{}, cannotHold("field %q is not in this add-on's entitlement schema", field)
		}
	}
	// A channel is off whenever the account is (design §3, role channel), so
	// an account switched off takes SMB with it whatever the request said of
	// SMB. The backend sends that already; the add-on holds it too, so no
	// caller can leave a disabled account reachable over SMB.
	if d.managed[FieldEnabled] && !d.enabled {
		d.managed[FieldSMBEnabled], d.smbEnabled = true, false
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
	if !decodeRequest(w, body, &req, CodeBadRequest) || !writeContractRefusal(w, req.ContractVersion) {
		return
	}
	if strings.TrimSpace(req.CallID) == "" || strings.TrimSpace(req.Subject) == "" {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "An apply needs a call_id and a subject.", nil)
		return
	}
	if strings.TrimSpace(req.Fingerprint) == "" {
		// A missing fingerprint is refused, not skipped. §8's guarantee is that
		// the diff an operator approved is the diff that lands, and a check that
		// silently passes when the field is absent is a guarantee that holds
		// only for callers who chose to be bound by it.
		writeError(w, http.StatusBadRequest, CodeFingerprintRequired,
			"This call carries no fingerprint, so there is nothing to verify the plan against.", nil)
		return
	}

	// Deduplicated before the lifecycle gate and the fingerprint, so a replay
	// during a maintenance window returns what it returned rather than being
	// refused — the call already happened, and refusing it now would report a
	// completed mutation as queued — and a re-plan's new fingerprint does not
	// turn a replay into PLAN_STALE (calls.go).
	identity := map[string]any{"subject": req.Subject, "desired": plainJSON(req.Desired)}
	s.once(w, req.CallID, kindApply, req.Subject, identity, func(w http.ResponseWriter) { s.applyNow(w, req) })
}

// applyNow is one apply that was not a replay, admitted by the lifecycle gate
// and alone for its subject (once).
func (s *server) applyNow(w http.ResponseWriter, req ApplyRequest) {
	if supported, why := s.nas.MajorSupported(); !supported {
		// A mutation against an untested major is refused, not attempted. Reads
		// continue; this is the half that could break something.
		writeError(w, http.StatusUnprocessableEntity, CodeTargetVersionUnsupported, why, nil)
		return
	}

	outcome, status, err := s.applyOne(req)
	switch {
	case errors.Is(err, errStaleFingerprint):
		// Its own code, because it is its own operator action. Every other
		// refusal here means "fix something and try again"; this one means
		// the subject moved since the diff was approved, and what has to
		// happen next is a re-plan somebody reads.
		writeError(w, status, CodePlanStale,
			"The subject's state on the target has changed since the plan was computed.",
			map[string]any{"subject": req.Subject})
	case errors.Is(err, ErrNotSent):
		// Nothing reached the NAS, so nothing happened, and the backend keeps
		// the change queued. Every failure after a write was sent stays
		// APPLY_FAILED, a 5xx that says "may have acted": createAndConverge
		// drops ErrNotSent from those (acted).
		writeError(w, http.StatusServiceUnavailable, CodeTargetUnreachable,
			"The NAS could not be reached, so nothing was sent to it.", nil)
	case errors.Is(err, errNotLogged):
		// 500, which Syndra reads as "may have acted". Conservative, not a
		// claim: the write that could not be recorded was not sent, and usually
		// nothing was; it is the one answer that puts the change in front of a
		// person, which a failing disk needs.
		writeError(w, http.StatusInternalServerError, CodeApplyFailed, notLoggedDetail, nil)
	case err != nil:
		writeError(w, status, CodeApplyFailed, err.Error(), nil)
	case outcome.Conflict != nil:
		// A blocked apply is a problem whose extension members are the
		// outcome (contract 2): the operator's adoption decision rides on it.
		writeOutcomeRefusal(w, status, CodeBindingConflict, outcome)
	case outcome.Effect == EffectBlocked:
		writeOutcomeRefusal(w, http.StatusUnprocessableEntity, CodeApplyBlocked, outcome)
	default:
		writeJSON(w, status, outcome)
	}
}

// applyOne is the convergence.
func (s *server) applyOne(req ApplyRequest) (ApplyOutcome, int, error) {
	desired, err := s.decodeDesired(req.Desired)
	if err != nil {
		// Not unholdable: where home folders go could not be read from the
		// add-on's own store.
		return blockedOr(req.Subject, err, http.StatusInternalServerError)
	}

	current, binding, conflict, truncated, adopt, err := s.locate(req)
	if errors.Is(err, errNoIdentityToDeriveFrom) && noAccountToSwitchOff(nil, desired) {
		// No binding, so no account of theirs here, and nothing to make: the
		// identity is only needed to name a new account. What the plan said.
		return noAccountToSwitchOffOutcome(req.Subject), http.StatusOK, nil
	}
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
		detail, next := conflict.words()
		return ApplyOutcome{
			Subject: req.Subject, Effect: EffectBlocked,
			Detail: detail, Consequence: "Nothing was changed. " + next,
			Username: conflict.Username, Conflict: conflict,
		}, http.StatusConflict, nil
	}

	// Verified against what was just read, immediately before the write. A
	// fingerprint checked earlier would be a statement about a moment that has
	// already passed.
	if req.Fingerprint != fingerprintSubject(current) {
		return ApplyOutcome{}, http.StatusConflict, fmt.Errorf("%w: %s", errStaleFingerprint, req.Subject)
	}

	if adopt {
		// What can block the converge is asked first (the groups, as converge
		// and the plan ask it), so a blocked adoption binds nothing and its
		// "Nothing was changed." is true.
		if g := decide(current, desired, homeDecision{}).groups; g != nil {
			if _, err := s.groupIDsFor(*g); err != nil {
				return blockedOr(req.Subject, err, statusFor(err))
			}
		}
		// The account this subject's own create left behind (5.3d), bound only
		// now the plan is verified, so a stale plan changes nothing. A logged
		// binding like every other (mutate), and the trace that settles the
		// create in doubt: CreateInDoubt reads no create before a binding. Under
		// the subject's lock (once), so no second create of theirs races it;
		// BindIfFree holds the name against every other subject.
		if err := s.mutate(applyCaller(req).on(binding.UID), "store.bind", func() error { return s.store.BindIfFree(binding) }); err != nil {
			logStoreFailure("binding", req.Subject, err)
			return ApplyOutcome{}, http.StatusInternalServerError, err
		}
		// Then on as the create would have gone on: the desired fields onto
		// the account, answered as a create.
		out, status, err := s.converge(req, desired, current)
		s.rememberWantedSMB(req, desired, err)
		if err != nil {
			// The binding landed; what failed came after it.
			return out, status, fmt.Errorf("%s was adopted for this subject, then: %w", binding.Username, err)
		}
		return adoptionWords(out, false), status, nil
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
		if noAccountToSwitchOff(current, desired) {
			return noAccountToSwitchOffOutcome(req.Subject), http.StatusOK, nil
		}
		out, status, err := s.createAndConverge(req, desired, binding)
		s.rememberWantedSMB(req, desired, err)
		return out, status, err
	}
	out, status, err := s.converge(req, desired, current)
	s.rememberWantedSMB(req, desired, err)
	return out, status, err
}

// noAccountToSwitchOff is a subject with no account here whose wanted state is
// off. There is nothing to lock, and creating an account only to lock it would
// give a departed person's name, groups and home back to the target, or remake
// one an operator deleted on purpose. A withdraw-only instruction is sent with
// nobody reviewing it, so this is what keeps it from ever creating anything.
// Plan and apply both ask it, so the plan never promises a create the apply
// refuses. Asked after the truncation refusal: an absence read out of a capped
// list is not an absence, and "nothing to switch off" would then be a lie.
func noAccountToSwitchOff(current *Subject, desired desiredState) bool {
	return current == nil && desired.managed[FieldEnabled] && !desired.enabled
}

func noAccountToSwitchOffOutcome(subject string) ApplyOutcome {
	return ApplyOutcome{
		Subject: subject, Effect: EffectNoChange,
		Detail:      "This subject has no account here and is wanted switched off, so there is nothing to switch off.",
		Consequence: "Nothing changes. No account is made.",
	}
}

// rememberWantedSMB keeps what this apply asked for SMB on the subject's
// binding, so the member's first password can turn it on (see Binding.WantsSMB).
// Best-effort: the apply already happened, and failing it over this record
// would report a mutation as not made. Written only when it changes, so an
// apply that asks what the binding already says adds nothing to the log.
func (s *server) rememberWantedSMB(req ApplyRequest, desired desiredState, applyErr error) {
	if applyErr != nil || !desired.managed[FieldSMBEnabled] {
		return
	}
	b, found, err := s.store.GetBinding(req.Subject)
	if err != nil || !found || (b.WantsSMB != nil && *b.WantsSMB == desired.smbEnabled) {
		return
	}
	want := desired.smbEnabled
	b.WantsSMB = &want
	if err := s.mutate(applyCaller(req).on(b.UID), "store.bind", func() error { return s.store.PutBinding(b) }); err != nil {
		logStoreFailure("binding", req.Subject, err)
	}
}

// boundAccount is the one rule for which account a binding names. Plan asks
// it of its snapshot and apply of the target, so the plan never says "absent"
// (or "creates") for an account the apply would find, or the other way round.
// byName and byUID look up one account by an exact key. It returns the account
// (nil when there is none), the binding as it should now read, and a conflict
// when the name is held by a different account of ours.
func boundAccount(binding Binding, floor, ceiling int64,
	byName func(string) (*Subject, error), byUID func(int64) (*Subject, error),
) (*Subject, Binding, *BindingConflict, error) {
	found, err := byName(binding.Username)
	if err != nil {
		return nil, binding, nil, err
	}
	owned := binding.UID >= floor && binding.UID <= ceiling
	if found != nil {
		if owned && found.UID != binding.UID {
			// A name some other account now holds (the original was deleted
			// and the name reused). Converging it would hand this subject's
			// entitlements to a stranger, so it halts like any foreign name.
			return nil, binding, &BindingConflict{Username: found.Username, AccountID: accountID(found.UID)}, nil
		}
		if !owned && found.UID >= floor && found.UID <= ceiling {
			// A legacy binding holding a record key, not a uid: the real one,
			// so the next pass is guarded by the check above.
			binding.UID = found.UID
		}
		return found, binding, nil, nil
	}
	if !owned {
		// A legacy uid of 0 or a record key must never match root or a
		// built-in account: with no name hit the account is missing.
		return nil, binding, nil, nil
	}
	// The name moved. A stable uid whose username changed out of band is a
	// RENAME, not a missing account: reporting it as missing would create a
	// replacement while the original kept the home data, or, for a subject
	// wanted off, leave the original unlocked.
	renamed, err := byUID(binding.UID)
	if err != nil {
		return nil, binding, nil, err
	}
	if renamed != nil {
		binding.Username = renamed.Username
		return renamed, binding, nil, nil
	}
	// Bound, and neither the name nor the uid is there. The account was
	// deleted out of band; convergence re-creates it under the recorded name
	// rather than re-deriving, because the recorded name is what every ACL and
	// share still refers to.
	return nil, binding, nil, nil
}

// locate finds the subject's account, by binding first and derivation second.
//
// The binding is authoritative and the derivation is a recovery path. Asking
// the other way round would rename an account every time somebody's email
// changed, which is the one thing §11 forbids.
// The fourth return value says a lookup came back cut short. Each lookup here
// is one account by an exact key, filtered at the target, so that only happens
// when a target ignores the filter and sends its whole list: carried out rather
// than acted on here, because it only matters where an ABSENCE is concluded.
// The fifth says the account found is unbound and this subject's own, left by
// a create in doubt (leftByCrashedCreate), for applyOne to bind.
func (s *server) locate(req ApplyRequest) (*Subject, Binding, *BindingConflict, bool, bool, error) {
	binding, bound, err := s.store.GetBinding(req.Subject)
	if err != nil {
		return nil, Binding{}, nil, false, false, err
	}
	if bound {
		// Each lookup is one account by an exact key; a cut-short answer only
		// matters when the conclusion is that the account is not there.
		cut := false
		lookup := func(field string, value any) (*Subject, error) {
			found, c, err := s.subjectWhere(field, value)
			cut = cut || c
			return found, err
		}
		floor, ceiling := s.idRange()
		found, resolved, conflict, err := boundAccount(binding, floor, ceiling,
			func(name string) (*Subject, error) { return lookup("username", name) },
			func(uid int64) (*Subject, error) { return lookup("uid", uid) })
		if err != nil {
			return nil, Binding{}, nil, false, false, err
		}
		if resolved.Username != binding.Username || resolved.UID != binding.UID {
			// A rename, or a legacy binding's real uid: recorded so the next
			// pass finds the account by name and is guarded by its uid.
			if err := s.mutate(applyCaller(req).on(resolved.UID), "store.bind", func() error { return s.store.PutBinding(resolved) }); err != nil {
				logStoreFailure("binding", req.Subject, err)
			}
		}
		return found, resolved, conflict, found == nil && conflict == nil && cut, false, nil
	}

	if strings.TrimSpace(req.Email) == "" {
		// No binding and no identity to derive a name from. The fallback exists
		// for a localpart that normalizes to nothing, not for a caller that sent
		// no localpart at all: applied here it would create `u3f2a9c11` for
		// somebody whose plan said `charlotte`, and the plan is what an operator
		// approved. Refused so the disagreement is visible instead of being a
		// name nobody chose.
		return nil, Binding{}, nil, false, false, errNoIdentityToDeriveFrom
	}

	claimed, err := s.store.BoundUsernames()
	if err != nil {
		return nil, Binding{}, nil, false, false, err
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

	existing, cut, err := s.subjectWhere("username", derived)
	if err != nil {
		return nil, Binding{}, nil, false, false, err
	}
	if existing != nil {
		conflict := s.unboundConflict(existing, claimed[derived])
		adopt, err := s.leftByCrashedCreate(req.Subject, derived, existing, conflict)
		if err != nil {
			return nil, Binding{}, nil, false, false, err
		}
		if !adopt {
			return nil, Binding{}, conflict, false, false, nil
		}
		// Not bound here: the apply binds it once the plan is verified.
		return existing, Binding{SubjectID: req.Subject, Username: existing.Username, UID: existing.UID, BoundBy: "apply"}, nil, false, true, nil
	}
	return nil, Binding{SubjectID: req.Subject, Username: derived}, nil, cut, false, nil
}

// change is what one subject needs, decided once. `plan` reads it to say what
// would happen and `converge` and `createAndConverge` read it to do it, so the
// apply can never decide differently from the plan an operator approved. What
// the two still own is wording (describePlannedChange / describeChange) and the
// target's ids, which only a write needs.
type change struct {
	// groups, when set, is the full replacement for the managed groups.
	groups *[]string
	// locked, when set, is the lock to write (true = account disabled).
	locked *bool
	// smbOff sends smb:false. smbPending is SMB wanted ON, which is never sent
	// here: TrueNAS accepts `smb: true` only in the call that sets a password
	// ("Password must be reset in order to enable SMB authentication"), and sent
	// alone it refuses the WHOLE update, so every other change in it failed with
	// it, on every pass. The member's next password carries it (password.set
	// reads Binding.WantsSMB). Turning it OFF is always allowed, and is the half
	// revocation needs.
	smbOff, smbPending bool
	// home is decideHome's answer, set by the caller once it has read the
	// target: it can fail, and the order of failures is the callers' to keep.
	home homeDecision
}

// decide is the pure half of a convergence. current is nil for an account that
// does not exist yet, in which case every managed field is a write.
func decide(current *Subject, desired desiredState, home homeDecision) change {
	c := change{home: home}
	if desired.managed[FieldGroup] && (current == nil || !sameSet(current.Groups, desired.groups)) {
		// A full replace, which is what makes this level-triggered: the same call
		// grants and revokes, and re-issuing it is a no-op.
		g := desired.groups
		c.groups = &g
	}
	if desired.managed[FieldEnabled] && (current == nil || current.Enabled != desired.enabled) {
		l := !desired.enabled
		c.locked = &l
	}
	if current != nil && desired.managed[FieldSMBEnabled] && current.SMBEnabled != desired.smbEnabled {
		if desired.smbEnabled {
			c.smbPending = true
		} else {
			c.smbOff = true
		}
	}
	return c
}

// writes is whether user.update has anything to carry.
func (c change) writes() bool {
	return c.groups != nil || c.locked != nil || c.smbOff || c.home.path != ""
}

// applyTo is the subject as this change leaves it.
func (c change) applyTo(s Subject) Subject {
	if c.groups != nil {
		s.Groups = *c.groups
	}
	if c.locked != nil {
		s.Enabled = !*c.locked
	}
	if c.smbOff {
		s.SMBEnabled = false
	}
	if c.home.path != "" {
		s.Home = c.home.path
	}
	return s
}

// applyCaller is what an apply's writes are logged under.
func applyCaller(req ApplyRequest) caller {
	return caller{operation: "apply", subject: req.Subject, actor: req.Actor, callID: req.CallID}
}

// converge brings an existing account onto the desired state.
func (s *server) converge(req ApplyRequest, desired desiredState, current *Subject) (ApplyOutcome, int, error) {
	who := applyCaller(req)
	update := map[string]any{}
	c := decide(current, desired, homeDecision{})

	if c.groups != nil {
		// Sent as record ids, which is what the read resolved names FROM. A
		// write in names against a read in ids is the asymmetry where an
		// account lands in the wrong groups and the next read calls it
		// converged.
		ids, err := s.groupIDsFor(*c.groups)
		if err != nil {
			return blockedOr(req.Subject, err, statusFor(err))
		}
		// Plus the built-in memberships, which current.Groups never listed:
		// the replace is of the groups Syndra manages, not of the account's.
		update["groups"] = append(ids, current.builtinGroups...)
	}
	if c.locked != nil {
		update["locked"] = *c.locked
	}
	if c.smbOff {
		update["smb"] = false
	}
	smbPending := c.smbPending
	smbWait := smbWaiting(current)

	// The home folder rides in the same update, so one write and one read-back
	// verify it. The rules (never moved, never removed, never someone else's)
	// are home.go's; what is not done is said in the outcome.
	home, err := s.decideHome(desired, current.Username, current.Home, current.UID, true)
	if err != nil {
		return ApplyOutcome{}, statusFor(err), err
	}
	c.home = home
	if home.path != "" {
		for k, v := range homeFields(home.path) {
			update[k] = v
		}
	}
	homeAfter := current.Home
	if home.path != "" {
		homeAfter = home.path
	}
	notes := func(base string, extra ...string) string {
		for _, e := range append([]string{home.note}, extra...) {
			if e != "" {
				base += " " + e
			}
		}
		return strings.TrimSpace(base)
	}

	if !c.writes() {
		// Nothing for user.update. The limit is its own call, and may still be
		// owed.
		quota, err := s.convergeQuota(who, desired, homeAfter, current.UID)
		if err != nil {
			return ApplyOutcome{}, statusFor(err), err
		}
		if quota != "" {
			detail := quota
			if smbPending {
				detail = notes(quota, smbWait)
			} else {
				detail = notes(quota)
			}
			return ApplyOutcome{
				Subject: req.Subject, Effect: EffectApplied, Detail: detail,
				Consequence: describeHolding(*current),
				Username:    current.Username, AccountID: accountID(current.UID), Standing: standingOf(current),
				Fingerprint: fingerprintSubject(current),
			}, http.StatusOK, nil
		}
	}

	if !c.writes() && smbPending {
		// Nothing to write, and something still wanted. Reported as its own
		// outcome rather than as no-change: "already in the requested state" is
		// false, and reporting it would leave an operator believing SMB is on.
		// The consequence is what the account holds, the plan's words for the
		// same outcome (the plan and the apply never disagree, design §7).
		return ApplyOutcome{
			Subject: req.Subject, Effect: EffectNoChange,
			Detail:      notes(smbWait),
			Consequence: describeHolding(*current),
			Username:    current.Username,
			AccountID:   accountID(current.UID),
			Standing:    standingOf(current),
			Fingerprint: fingerprintSubject(current),
		}, http.StatusOK, nil
	}
	if !c.writes() {
		// Re-applying an unchanged set issues no mutating call at all. Not an
		// optimisation: the drain re-drives rows, so an apply that wrote every
		// time would rewrite every account on every pass and fill the mutation
		// log with events that changed nothing.
		return ApplyOutcome{
			Subject: req.Subject, Effect: EffectNoChange,
			Detail:      notes("Already in the requested state."),
			Consequence: describeHolding(*current),
			Username:    current.Username,
			AccountID:   accountID(current.UID),
			Standing:    standingOf(current),
			Fingerprint: fingerprintSubject(current),
		}, http.StatusOK, nil
	}

	if err := s.send(who, "user.update", []any{current.ID, update}, nil); err != nil {
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
			// Its standing as last read: the write landed and what it left
			// could not be read back.
			AccountID: accountID(current.UID),
			Standing:  standingOf(current),
		}, http.StatusOK, nil
	}

	quota, err := s.convergeQuota(who, desired, homeAfter, current.UID)
	if err != nil {
		// The account update landed; the limit did not. Said, not swallowed:
		// the next pass sets it.
		quota = "The home folder limit could not be set yet: " + err.Error() + "."
	}
	detail := describeChange(update, desired.groups)
	if home.path != "" {
		detail = notes(detail, "Home folder created at "+home.path+".", quota)
	} else {
		detail = notes(detail, quota)
	}
	if smbPending {
		// The rest landed; SMB did not, and saying only what landed would read
		// as fully converged.
		detail += " " + smbWait
	}
	return ApplyOutcome{
		Subject: req.Subject, Effect: EffectApplied,
		Detail:      detail,
		Consequence: describeHolding(*applied),
		Username:    applied.Username,
		AccountID:   accountID(applied.UID),
		Standing:    standingOf(applied),
		Fingerprint: fingerprintSubject(applied),
		Observed:    s.observedFields(desired, applied),
	}, http.StatusOK, nil
}

// createAndConverge makes the account as part of the convergence.
func (s *server) createAndConverge(req ApplyRequest, desired desiredState, binding Binding) (ApplyOutcome, int, error) {
	who := applyCaller(req)
	if !ValidUsername(binding.Username) {
		return ApplyOutcome{}, http.StatusUnprocessableEntity,
			fmt.Errorf("derived name %q is not valid on this target", binding.Username)
	}
	want := decide(nil, desired, homeDecision{})
	var groupIDs []apiID
	if want.groups != nil {
		ids, err := s.groupIDsFor(*want.groups)
		if err != nil {
			return blockedOr(req.Subject, err, statusFor(err))
		}
		groupIDs = ids
	}
	create := map[string]any{
		"username": binding.Username,
		// `full_name` is required by the middleware and is display only. The
		// subject id rather than a name from the directory: this add-on is not
		// a copy of the identity provider, and a stale display name here would
		// be one more thing that disagrees with Zitadel.
		"full_name": binding.Username,
		// SMB only, never a login. Left unset, TrueNAS gives the account zsh,
		// and one SSH key added later is a working shell on the NAS. The path
		// is a key of `user.shell_choices` on 25.10, and the one every account
		// the makerspace made by hand already uses.
		"shell": "/usr/sbin/nologin",
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
		"locked": want.locked != nil && *want.locked,
	}
	if want.groups != nil {
		create["groups"] = groupIDs
	}

	// One number, from this add-on's own range, for the account and its
	// personal group (identity.go). The group is made first so the account
	// can be created in it; if the account is refused, the group goes too.
	id, err := s.nextIdentity()
	if err != nil {
		return ApplyOutcome{}, statusFor(err), err
	}
	var groupID apiID
	adopted := false
	// createMayHaveLanded: group.create got no answer after it was sent (a
	// timeout, a dropped socket), so the very group adopted below may be this
	// attempt's write. An answer (a refusal, a rate limit) or nothing sent at
	// all means this attempt wrote nothing.
	createMayHaveLanded := false
	if err := s.send(who, "group.create", []any{map[string]any{
		"name": binding.Username, "gid": id, "smb": false,
	}}, &groupID); err != nil {
		createMayHaveLanded = errors.Is(err, ErrTargetUnreachable) && !errors.Is(err, ErrNotSent)
		// An empty group of that name inside this add-on's own range is one an
		// earlier attempt left, and ours to take; anything else is somebody's,
		// and the refusal stands. Its number becomes the account's too, so the
		// pair stays one number (identity.go).
		orphan, gid, ok := s.emptyGroupNamed(binding.Username)
		if !ok {
			return ApplyOutcome{}, statusFor(err), fmt.Errorf("make %s's personal group: %w", binding.Username, err)
		}
		groupID, id, adopted = orphan, gid, true
	}
	create["uid"] = id
	// From here every write names the account it is for, by the uid asked
	// for, so a crash between user.create and its record leaves an intent the
	// next apply can match to exactly this account (CreateInDoubt, 5.3d).
	who = who.on(id)
	create["group"] = groupID
	create["group_create"] = false
	// failed answers a failure before the account exists. A group made here
	// was a write, so it is undone and the failure is acted's. An adopted
	// group is never deleted (it may predate this attempt). Its failure keeps
	// its own status (a 429 stays a 429) unless this attempt's own create got
	// no answer: then the adopted group may be this attempt's write, and the
	// failure is acted's.
	failed := func(err error) (ApplyOutcome, int, error) {
		if adopted {
			if createMayHaveLanded {
				return acted(err)
			}
			return ApplyOutcome{}, statusFor(err), err
		}
		s.undoGroup(who, binding.Username, groupID)
		return acted(err)
	}

	// The home, when one role gives one and its folder is free. Decided before
	// the account exists, so a folder that is there at all is someone else's.
	home, err := s.decideHome(desired, binding.Username, "", -1, true)
	if err != nil {
		return failed(err)
	}
	if home.path != "" {
		for k, v := range homeFields(home.path) {
			create[k] = v
		}
	}

	var recordID apiID
	if err := s.send(who, "user.create", []any{create}, &recordID); err != nil {
		return failed(err)
	}
	// `user.create` answers with the RECORD key, not the unix uid, and the
	// binding recognises an account by its uid across a rename. Read back
	// rather than assumed: a fingerprint computed from a state this add-on
	// invented is a fingerprint the next plan verifies against nothing.
	created, readErr := s.readBack(binding.Username)
	// The account exists whether or not it could be read back, so the binding
	// is recorded either way: left out, the next apply finds the name unbound
	// and halts as a conflict over an account this add-on made. The uid is the
	// one it asked for, which is what the account was created with.
	uid := id
	if readErr == nil {
		uid = created.UID
	}
	binding.SubjectID, binding.UID, binding.BoundBy = req.Subject, uid, "apply"
	if err := s.mutate(who.on(uid), "store.bind", func() error { return s.store.BindIfFree(binding) }); err != nil {
		// The account exists and the record of whose it is did not land. Not a
		// silent success: the next apply would derive the same name, find it
		// unbound, and halt as a binding conflict — which is recoverable but
		// needs an operator, so it is reported now rather than discovered then.
		logStoreFailure("binding", req.Subject, err)
		return ApplyOutcome{}, http.StatusInternalServerError,
			fmt.Errorf("account %s was created and its binding could not be recorded: %v", binding.Username, err)
	}
	if readErr != nil {
		return acted(fmt.Errorf("account %s was created and could not be read back: %w", binding.Username, readErr))
	}

	detail := fmt.Sprintf("Created %s.", binding.Username)
	if home.path != "" {
		detail += " Home folder created at " + home.path + "."
	}
	if quota, err := s.convergeQuota(who, desired, home.path, created.UID); err != nil {
		detail += " The home folder limit could not be set yet: " + err.Error() + "."
	} else if quota != "" {
		detail += " " + quota
	}
	if home.note != "" {
		detail += " " + home.note
	}

	return ApplyOutcome{
		Subject: req.Subject, Effect: EffectApplied,
		Detail:      detail,
		Consequence: describeHolding(*created),
		Username:    binding.Username,
		AccountID:   accountID(created.UID),
		Standing:    standingOf(created),
		Fingerprint: fingerprintSubject(created),
		Observed:    s.observedFields(desired, created),
	}, http.StatusOK, nil
}

// readBack reads one account as the target now holds it.
func (s *server) readBack(username string) (*Subject, error) {
	found, _, err := s.subjectWhere("username", username)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, fmt.Errorf("%w: %s is not on the target after being created", ErrTargetRefused, username)
	}
	return found, nil
}

// subjectWhere reads the one account whose field equals value, or nil when the
// target has none. Filtered at the target: the whole account list per lookup was
// a 100-person cascade's worth of full reads. The key comparison is kept on this
// side as well, so a target that ignored the filter cannot hand back the wrong
// account; the second return is that read's truncation, which is only ever set
// by such a target (an exact key matches one row) and stops an absence being
// concluded from a list that was cut short.
func (s *server) subjectWhere(field string, value any) (*Subject, bool, error) {
	snap, err := s.readSubjectsWhere([]any{[]any{field, "=", value}})
	if err != nil {
		return nil, false, err
	}
	for i := range snap.Subjects {
		got := snap.Subjects[i]
		if (field == "username" && got.Username == value) || (field == "uid" && got.UID == value) {
			return &snap.Subjects[i], snap.Truncated, nil
		}
	}
	return nil, snap.Truncated, nil
}

// emptyGroupNamed finds a group by name that nobody belongs to, primary or
// secondary, and is not one of the machine's own, and whose gid is inside this
// add-on's id range: outside it the group was made by somebody else, and an
// account created in it would own whatever files carry that gid. It returns the
// gid, which the account takes as its uid.
//
// A named type rather than an inline one: `users` is not in the recorded read
// (scripts/lib/record-truenas.py selects four fields), so the shape guard has
// nothing to check it against.
func (s *server) emptyGroupNamed(name string) (apiID, int64, bool) {
	type groupMembers struct {
		nasGroup
		Users []apiID `json:"users"`
	}
	var groups []groupMembers
	if err := s.nas.call("group.query", []any{
		[]any{[]any{"group", "=", name}},
		map[string]any{"select": []string{"id", "gid", "group", "builtin", "users"}, "limit": 1},
	}, &groups); err != nil || len(groups) != 1 {
		return apiID{}, 0, false
	}
	g := groups[0]
	floor, ceiling := s.idRange()
	return g.ID, g.GID, !g.Builtin && len(g.Users) == 0 && g.ID.known() && g.GID >= floor && g.GID <= ceiling
}

// caller is the call a write belongs to: what its log records say beside the
// write itself. Never a parameter's value: there is no field for one, which is
// what keeps a password out of the log. accountID is the account the write
// is for (Record.AccountID): set by a create from its uid on, and by every
// binding write, so the log can tell a create in doubt from an account bound,
// released or created since (CreateInDoubt).
type caller struct{ operation, subject, actor, callID, accountID string }

// on is c naming the account uid its writes are for; a zero uid (a legacy
// binding that kept none) names none.
func (c caller) on(uid int64) caller {
	if uid != 0 {
		c.accountID = accountID(uid)
	}
	return c
}

// errNotLogged is a write refused because its intent could not be made
// durable: nothing was sent. Answered 500 (APPLY_FAILED, OPERATION_FAILED,
// SETTING_NOT_SAVED), never TARGET_UNREACHABLE: the NAS was reachable, the
// add-on's own disk failed, and that needs a person.
var errNotLogged = errors.New("the add-on could not write the change to its mutation log, so it did not send it")

// notLoggedDetail is what a person reads when errNotLogged refuses a write.
const notLoggedDetail = "The add-on could not write this change to its mutation log on disk, so it did not send it. " +
	"Check the add-on's log volume (space, permissions, the disk itself), then try again."

// mutate is the one way this add-on changes anything, on the NAS or in its
// own store: write-ahead, as a database journals (5.3b, design §8). The
// intent is appended and fsynced, then do runs, then its result is appended.
// The rule: every NAS write and every change to a binding or a setting is its
// own mutate, logged under the call that caused it, whatever that call is (an
// apply's create binds, a purge unbinds, each beside its NAS write); a store
// write that would change nothing is not made, so it is not logged
// (TestEveryNASWriteGoesThroughTheSeam, TestEveryBindingChangeIsInTheLog).
// An intent that did not land is a write that does not happen: do is never
// called and the answer is errNotLogged. A result that does not land leaves
// the intent open, settled unknown at the next start (Recover); the reply still
// says what happened, because the write did.
func (s *server) mutate(c caller, write string, do func() error) error {
	if strings.TrimSpace(c.actor) == "" {
		// Unstated, not invented. The call arrived authenticated as the backend
		// and named nobody, and recording the subject as their own actor
		// makes every record answer "who" with "whom".
		c.actor = "syndra"
	}
	r := Record{Operation: c.operation, Subject: c.subject, Actor: c.actor, CallID: c.callID, Write: write, AccountID: c.accountID}
	intent := r
	intent.Outcome = outcomeIntent
	logged, err := s.log.Append(intent)
	if err != nil {
		logStoreFailure("mutation log intent", c.callID, err)
		return errNotLogged
	}
	// Every write, whatever became of it, makes every copy a read of the past:
	// the next read of anything reads the NAS (fresh.go). After the write, and
	// a read that began before it is never kept (the fresher's generation).
	defer s.fresh.forget()
	err = do()
	r.Intent, r.Outcome = logged.Seq, resultOf(write, err)
	if _, logErr := s.log.Append(r); logErr != nil {
		logStoreFailure("mutation log result", c.callID, logErr)
	}
	return err
}

// send is one NAS write through mutate: the method the intent names is the
// method sent, one variable for both.
func (s *server) send(c caller, method string, params, out any) error {
	return s.mutate(c, method, func() error { return s.nas.sendWrite(method, params, out) })
}

// resultOf is what a result record says became of a write.
func resultOf(write string, err error) string {
	switch {
	case err == nil:
		return outcomeSucceeded
	case errors.Is(err, ErrNotSent):
		return outcomeNotSent
	case errors.Is(err, ErrTargetRefused), errors.Is(err, ErrRateLimited), strings.HasPrefix(write, "store."):
		// The target answered no, or a store transaction did not commit.
		return outcomeFailed
	default:
		return outcomeUnknown
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
	if len(parts) == 0 {
		return ""
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

// acted answers a failure after this apply had already written to the target
// (here, the personal group). The one rule: after the first write, every
// failure is "may have acted", a 5xx APPLY_FAILED a person looks at, never an
// answer Syndra reads as not reached and re-queues by itself. So a later
// call's "nothing was sent" is dropped (never TARGET_UNREACHABLE), and a 429
// from a rate-limited NAS becomes 503. The text is kept, except after a log
// that could not be written: "it did not send it" would read as "nothing
// happened", and the earlier write did happen.
func acted(err error) (ApplyOutcome, int, error) {
	status := statusFor(err)
	if status == http.StatusTooManyRequests {
		status = http.StatusServiceUnavailable
	}
	switch {
	case errors.Is(err, errNotLogged):
		return ApplyOutcome{}, http.StatusInternalServerError,
			errors.New("part of this change was already sent to the NAS, then the add-on could not write the rest to its mutation log, so the rest was not sent; check the add-on's log volume, then try again")
	case errors.Is(err, ErrNotSent):
		err = errors.New(err.Error())
	}
	return ApplyOutcome{}, status, err
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
