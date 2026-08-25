package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// One-shot operations (design §4, §10).
//
// Everything here is an EVENT: it happens once, it is never queued, and it is
// never retried. That is not an omission — a retry needs the parameters, the
// parameters are the secret, and keeping the secret to enable the retry is the
// vault this whole design exists to avoid. Recovery is the member resubmitting,
// which is safe because the call id deduplicates and because setting the same
// credential twice converges anyway.

// OperationRequest is one invocation.
//
// `Params` carries the secret and nothing else does. It reaches the target and
// is discarded; no field of it is written to the store, the snapshot, or the
// mutation log, none of which has anywhere to put one.
type OperationRequest struct {
	ContractVersion int    `json:"contract_version"`
	CallID          string `json:"call_id"`
	// Operation is the name, repeated from the path. It is inside the body
	// because in signed mode the MAC covers the body and the request line, and
	// the two must agree — a name accepted from the path while a different one
	// travelled in the signed body would let an operator's approval for one
	// operation be spent on another.
	Operation string `json:"operation"`
	Subject   string `json:"subject"`
	Actor     string `json:"actor"`
	// PlanID and Fingerprint are empty for the one-shot operations this add-on
	// declares — they are approved by confirmation at the moment of invocation
	// rather than rehearsed against a state read. Declared anyway, because the
	// backend's envelope carries them for every call and a field this struct
	// does not name is a 400 rather than an ignored extra.
	PlanID      string         `json:"plan_id,omitempty"`
	Fingerprint string         `json:"fingerprint,omitempty"`
	Params      map[string]any `json:"params,omitempty"`
}

// OperationResult is what the caller learns.
//
// Deliberately thin. There is no field for the target's error payload, because
// that payload is the likeliest place for a submitted password to be echoed
// back at us — the same reason the backend's `addon_operations` row has no
// free-text column.
type OperationResult struct {
	Operation string `json:"operation"`
	Subject   string `json:"subject"`
	Outcome   string `json:"outcome"`
	Detail    string `json:"detail,omitempty"`
	// Rotated says a new credential was minted and applied. The value is not
	// here and is nowhere else either: rotation is the credential half of a
	// revocation, and returning what it minted would defeat the point.
	Rotated bool `json:"rotated,omitempty"`
	// Activity and Health are the read-only operations' payloads.
	Activity *ActivityReport `json:"activity,omitempty"`
	// Storage is a member's own account state and usage (storage.status).
	Storage *StorageStatus `json:"storage,omitempty"`
	Health  *TargetHealth  `json:"health,omitempty"`
	// AccountUID is the unix identity of the account an adoption bound.
	//
	// Returned because the backend keeps its own copy of the binding and had no
	// way to learn this: it knows the NAME the operator clicked, and a name is
	// the half that changes. The add-on's own binding has carried the uid from
	// the start, so without this the two copies of one binding disagreed about
	// the field that survives a rename — and a renamed account came back as
	// unmanaged, adoptable by somebody else, while the backend's binding still
	// claimed it.
	AccountUID int64 `json:"account_uid,omitempty"`
}

// handleOperation dispatches one named operation.
func (s *server) handleOperation(w http.ResponseWriter, r *http.Request, body []byte) {
	name := r.PathValue("name")
	var req OperationRequest
	if err := decodeStrict(body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "BAD_REQUEST"})
		return
	}
	if !writeContractRefusal(w, req.ContractVersion) {
		return
	}
	if strings.TrimSpace(req.CallID) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "BAD_REQUEST"})
		return
	}
	// The path decides which operation runs; the body says which one the caller
	// signed for. A disagreement is refused rather than resolved in either
	// direction — picking the path would let a body signed for `health.get`
	// drive `account.purge`, and picking the body would let the route the
	// operator authorised be swapped underneath them.
	if req.Operation != "" && req.Operation != name {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error":  "OPERATION_MISMATCH",
			"detail": "the operation named in the body is not the one this route runs",
		})
		return
	}

	// The same dedup the apply uses, and universal for the same reason: §16
	// declines a separate nonce store on the grounds that the call id already
	// prevents replay, and that argument only holds if nothing is exempt.
	// Keyed by the operation NAME as well as the id. One namespace, so a call
	// id reused anywhere is still caught — but a cached apply outcome decoded
	// as an operation result is an all-zero success, and a password.set replayed
	// at a different operation is a different call, not a duplicate.
	if cached, found, err := s.store.Recall(req.CallID, kindOperation+name); err != nil {
		writeRecallFailure(w, err)
		return
	} else if found {
		var previous OperationResult
		if err := json.Unmarshal(cached, &previous); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "STORE_UNREADABLE"})
			return
		}
		writeJSON(w, http.StatusOK, previous)
		return
	}

	// Reads are never gated by lifecycle state; mutations always are.
	if mutatingOperation(name) {
		done, err := s.life.Begin()
		if err != nil {
			state, _ := s.life.State()
			writeLifecycleRefusal(w, state)
			return
		}
		defer done()

		if supported, why := s.nas.MajorSupported(); !supported {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]string{
				"error": "TARGET_VERSION_UNSUPPORTED", "detail": why,
			})
			return
		}
	}

	result, status, err := s.runOperation(name, req)
	if err != nil {
		// The error's text is this add-on's own, never the target's response
		// body: that body is where a submitted password comes back.
		writeJSON(w, status, map[string]string{"error": "OPERATION_FAILED", "detail": err.Error()})
		return
	}
	// Mutations only. A cached read is a 30-day copy of a health report nobody
	// will ask for twice — the dedup exists so a replayed MUTATION does not
	// happen again, and a read that happens again is a read.
	if mutatingOperation(name) {
		if err := s.store.Remember(req.CallID, kindOperation+name, result); err != nil {
			logStoreFailure("idempotency", req.CallID, err)
		}
	}
	writeJSON(w, status, result)
}

func mutatingOperation(name string) bool {
	switch name {
	case "password.set", "password.rotate", "account.purge", "account.adopt":
		return true
	}
	return false
}

func (s *server) runOperation(name string, req OperationRequest) (OperationResult, int, error) {
	switch name {
	case "password.set":
		return s.setPassword(req)
	case "password.rotate":
		return s.rotatePassword(req)
	case "account.purge":
		return s.purgeAccount(req)
	case "account.adopt":
		return s.adoptAccount(req)
	case "account.release":
		return s.releaseAccount(req)
	case "activity.get":
		return s.smbActivity(req)
	case "storage.status":
		return s.storageStatus(req)
	case "health.get":
		return s.targetHealth(req)
	}
	// Unknown ids fail closed. The backend's policy is the authority on what
	// exists and this is the second gate, not the first.
	return OperationResult{}, http.StatusNotFound, fmt.Errorf("no such operation")
}

// boundAccount resolves the subject's account, refusing rather than guessing.
//
// An operation that could not find its subject must not fall back to
// derivation: derivation names an account that may not exist, and setting a
// password on the wrong one is the whole hazard.
func (s *server) boundAccount(subject string) (Binding, error) {
	b, bound, err := s.store.GetBinding(subject)
	if err != nil {
		return Binding{}, err
	}
	if !bound {
		return Binding{}, fmt.Errorf("no account is bound to this subject yet")
	}
	return b, nil
}

// setPassword forwards a member's credential and keeps nothing.
//
// `user.update({password})`, never `user.set_password`: the latter rejects the
// call unless the session is FULL_ADMIN when the target is another user, and
// the former needs only ACCOUNT_WRITE — which is what this add-on's identity
// has and all it should have.
//
// Plaintext is mandatory. No TrueNAS API accepts an NT or unix hash, which is
// why Syndra's vault stops storing them: a hash it could store is a hash this
// could not use.
func (s *server) setPassword(req OperationRequest) (OperationResult, int, error) {
	password, err := requireSecret(req.Params, "password")
	if err != nil {
		return OperationResult{}, http.StatusBadRequest, err
	}
	binding, err := s.boundAccount(req.Subject)
	if err != nil {
		return OperationResult{}, http.StatusUnprocessableEntity, err
	}
	id, err := s.recordID(binding)
	if err != nil {
		return OperationResult{}, statusForLookup(err), lookupRefusal(err)
	}
	// Three fields, one call, and the grouping is forced by the target.
	//
	// `password_disabled` must be cleared in the SAME update: setting only
	// `password` is accepted and leaves password authentication disabled, so
	// the member gets a success message and an account that still refuses
	// them. Verified against TrueNAS 25.10.5 — the call returns ok and
	// `password_disabled` stays true.
	//
	// And SMB can only be enabled here. TrueNAS refuses `smb: true` on an
	// account with no password, and refuses it again in a later call with
	// `Password must be reset in order to enable SMB authentication` — the
	// only accepted ordering is the password and the SMB flag arriving
	// together. So a member setting their first password is also the moment
	// their share access becomes possible.
	update := map[string]any{"password": password, "password_disabled": false}
	if smb, known := s.desiredSMB(req.Subject); known && smb {
		update["smb"] = true
	}
	if err := s.nas.call("user.update", []any{id, update}, nil); err != nil {
		// A sentence an operator can read, not the security boundary. The
		// client wrapper already classifies rather than wraps, so the target's
		// own text — built from a call whose parameters include the password —
		// never reaches here at all. That guarantee is pinned where it lives,
		// on `NAS.call`; this is only the difference between a readable failure
		// and "user.update: the target refused the call".
		return OperationResult{}, statusFor(err), fmt.Errorf("the target refused the credential change")
	}
	// The event, never the value. There is no field on a Record for one.
	s.record("password.set", req.Subject, req.Actor, req.CallID, "succeeded")
	return OperationResult{
		Operation: "password.set", Subject: req.Subject, Outcome: "succeeded",
		Detail: "The credential was set on " + binding.Username + ".",
	}, http.StatusOK, nil
}

// desiredSMB reports the SMB state last applied for a subject, and whether
// anything is known about it at all.
//
// Read from the mirror rather than taken as a parameter: `password.set` is a
// member-scoped call carrying a credential and nothing else, and letting a
// caller pass an entitlement alongside it would make a password change a way to
// grant share access. The mirror is what the last convergence decided.
func (s *server) desiredSMB(subject string) (bool, bool) {
	snap, ok, err := s.store.GetSnapshot()
	if err != nil || !ok {
		return false, false
	}
	binding, err := s.boundAccount(subject)
	if err != nil {
		return false, false
	}
	for _, sub := range snap.Subjects {
		if sub.UID == binding.UID {
			return sub.SMBEnabled, true
		}
	}
	return false, false
}

// refuseSelfAccount rejects an operation aimed at the add-on's own target
// account.
//
// The account this add-on's API key belongs to is an ordinary row in
// `user.query`, so without this it appears in the unmanaged inventory as
// something to adopt — and one confirmation later, to purge. Deleting it does
// not remove a member's access: it deletes the credential Syndra reaches the
// target with, and nothing on either side can put it back. The operator would
// be left with a target reporting itself unreachable and an add-on that cannot
// say why.
//
// Enforced HERE, in the add-on, because this is the only component that knows
// which account that is — it asks the target rather than being told — and a
// guard in a caller is a guard the next caller does not have.
//
// Unknown identity refuses nothing. `auth.me` is a read that can fail, and
// treating "I could not find out" as "this is me" would block adoption of every
// account on the target.
func (s *server) refuseSelfAccount(username string) error {
	name, _, known := s.nas.Self()
	if !known || !strings.EqualFold(strings.TrimSpace(username), name) {
		return nil
	}
	return fmt.Errorf("%s is the account this add-on authenticates to the target with; "+
		"adopting or deleting it would remove Syndra's own access and cannot be undone", name)
}

// releaseAccount forgets a binding and touches the target not at all.
//
// The safe half of a purge, and until now it did not exist: `DeleteBinding` was
// reachable only THROUGH `account.purge`, so the only way to stop managing an
// account was to delete it. An operator whose binding pointed at the wrong
// person, or at an account that is gone, had one button and it was the
// irreversible one.
//
// It is the answer to the state the reconciliation reports as "this binding
// names an account that is no longer here": re-provision, or let it go. A
// surface that names two resolutions and implements one is a surface that will
// be resolved the wrong way.
//
// Writes nothing on the target ON PURPOSE, and says so in its own result: an
// operator releasing a binding is deciding Syndra should stop claiming the
// account, not deciding anything about the account. The account keeps working
// for whoever is using it, and appears in the unmanaged inventory on the next
// read — which is exactly what it now is.
func (s *server) releaseAccount(req OperationRequest) (OperationResult, int, error) {
	binding, found, err := s.store.GetBinding(req.Subject)
	if err != nil {
		return OperationResult{}, http.StatusInternalServerError, err
	}
	if !found {
		// Already released. Reported as done rather than refused, for the same
		// reason a repeated adoption is: two operators on two screens must not
		// produce an error for the second one.
		return OperationResult{
			Operation: "account.release", Subject: req.Subject, Outcome: "succeeded",
			Detail: "No account was bound to this subject.",
		}, http.StatusOK, nil
	}
	if err := s.store.DeleteBinding(req.Subject); err != nil {
		return OperationResult{}, http.StatusInternalServerError, fmt.Errorf("the binding could not be released")
	}
	s.record("account.release", req.Subject, req.Actor, req.CallID, "succeeded")
	return OperationResult{
		Operation: "account.release", Subject: req.Subject, Outcome: "succeeded",
		Detail: "Syndra no longer manages " + binding.Username +
			". Nothing on the target was changed — the account still exists and still works.",
	}, http.StatusOK, nil
}

// rotatePassword mints a new credential, applies it, and returns nothing.
//
// The credential half of a revocation. Established SMB sessions survive until
// they reconnect — TrueNAS exposes no session-close method at all — which is
// why the operator surface must say so rather than implying immediacy.
func (s *server) rotatePassword(req OperationRequest) (OperationResult, int, error) {
	binding, err := s.boundAccount(req.Subject)
	if err != nil {
		return OperationResult{}, http.StatusUnprocessableEntity, err
	}
	minted, err := mintCredential()
	if err != nil {
		return OperationResult{}, http.StatusInternalServerError, fmt.Errorf("could not mint a credential")
	}
	id, err := s.recordID(binding)
	if err != nil {
		return OperationResult{}, statusForLookup(err), lookupRefusal(err)
	}
	// `password_disabled` cleared here too, for the reason it is cleared in
	// setPassword: an update carrying only `password` is accepted and leaves
	// authentication disabled. A rotation deliberately does NOT touch `smb` —
	// it is the credential half of a revocation, and turning share access on
	// during one would be the opposite of what it is for.
	if err := s.nas.call("user.update", []any{id,
		map[string]any{"password": minted, "password_disabled": false}}, nil); err != nil {
		return OperationResult{}, statusFor(err), fmt.Errorf("the target refused the rotation")
	}
	s.record("password.rotate", req.Subject, req.Actor, req.CallID, "succeeded")
	return OperationResult{
		Operation: "password.rotate", Subject: req.Subject, Outcome: "succeeded", Rotated: true,
		Detail: "A new credential was set on " + binding.Username +
			". Established sessions end when they next reconnect, not immediately.",
	}, http.StatusOK, nil
}

// mintCredential produces a value nobody ever sees.
//
// 32 bytes of crypto/rand, base64. It is applied and discarded — not returned,
// not logged, not stored — because the whole point of rotation as a revocation
// is that the old credential stops working and no new one is handed out.
func mintCredential() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}

// purgeAccount deletes, on a credential this add-on does not hold.
//
// The add-on's own TrueNAS identity has ACCOUNT_WRITE and SYSTEM_AUDIT_READ and
// deliberately not deletion, so a compromised add-on can misassign, disable and
// rotate but cannot delete an account on its own. The elevated key is held by
// the backend and injected into this single call — never stored here, never
// cached, and never reused for the session this add-on keeps open.
//
// The alternative, having an operator supply it at the moment of use, was
// rejected: it contradicts the rule that operators read target health without
// being handed target credentials, and it has no good source — either somebody
// types a personal admin password into a form, or a shared key lives in a
// password manager until it ends up in a browser.
func (s *server) purgeAccount(req OperationRequest) (OperationResult, int, error) {
	elevated, err := requireSecret(req.Params, "elevated_key")
	if err != nil {
		return OperationResult{}, http.StatusBadRequest, err
	}
	binding, err := s.boundAccount(req.Subject)
	if err != nil {
		return OperationResult{}, http.StatusUnprocessableEntity, err
	}
	// Checked again here and not only at adoption. A binding can predate this
	// guard, or predate a key reissued against a different account, and this is
	// the one operation that cannot be undone.
	if err := s.refuseSelfAccount(binding.Username); err != nil {
		return OperationResult{}, http.StatusUnprocessableEntity, err
	}

	// Resolved on the shared session, which can read but not delete. The
	// elevated one does exactly one thing.
	id, err := s.recordID(binding)
	if err != nil {
		return OperationResult{}, statusForLookup(err), lookupRefusal(err)
	}

	// A session of its own, closed immediately. Not the long-lived one: an
	// elevated credential must not outlive the single call it was injected for,
	// and reusing the shared session would leave a delete-capable connection
	// open for everything else this add-on does.
	client, err := s.elevated(elevated)
	if err != nil {
		return OperationResult{}, http.StatusBadGateway, fmt.Errorf("the elevated session could not be established")
	}
	defer client.Close()

	if err := callOnce(client, "user.delete", []any{id}, nil); err != nil {
		return OperationResult{}, statusFor(err), fmt.Errorf("the target refused the deletion")
	}
	// The binding goes with the account. Leaving it would leave this add-on
	// claiming an account that no longer exists, and the next apply takes the
	// bound-but-absent path — which re-creates under the recorded name, because
	// that path exists for an account deleted BEHIND our back. A purge is not
	// that: we deleted it, on purpose, and it must not resurrect itself.
	if err := s.store.DeleteBinding(req.Subject); err != nil {
		logStoreFailure("binding", req.Subject, err)
	}
	s.record("account.purge", req.Subject, req.Actor, req.CallID, "succeeded")
	return OperationResult{
		Operation: "account.purge", Subject: req.Subject, Outcome: "succeeded",
		Detail: "Deleted " + binding.Username + ".",
	}, http.StatusOK, nil
}

// ActivityReport is SMB activity, and what it could not see.
type ActivityReport struct {
	Events []ActivityEvent `json:"events"`
	// UncoveredShares names the shares that were not watching THIS member —
	// auditing switched off, or switched on and scoped past them by a watch or
	// ignore list. Without it an empty result reads as "no activity", which is
	// a different and much more reassuring statement than "we were not
	// watching".
	//
	// Named apart from health's `unaudited_shares` on purpose. That one asks
	// the target-level question — is auditing on at all — and this one asks
	// about one person, and a share can be audited and still be on this list.
	// One name over two questions is the defect this whole branch keeps
	// finding, and a wire field is the worst place to leave an instance of it.
	UncoveredShares []string `json:"uncovered_shares,omitempty"`
}

type ActivityEvent struct {
	At      string `json:"at"`
	Event   string `json:"event"`
	Share   string `json:"share,omitempty"`
	Success bool   `json:"success"`
	// Where it came from. Every row in a real SMB audit log carries one, and
	// without it a week of refusals is a week of refusals from nowhere — the
	// difference between "somebody's saved password is stale" and "somebody is
	// guessing".
	Address string `json:"address,omitempty"`
	// The target's own status token for the outcome, and ONLY a token.
	//
	// `NT_STATUS_NO_SUCH_USER` is what makes a refusal actionable, and it is a
	// constant rather than prose. The add-on's standing rule is that the
	// target's error TEXT never leaves this package, because the middleware
	// builds that text from a call whose parameters can include a member's
	// password. An audit status is a different thing, and it is admitted by a
	// check on its SHAPE rather than by trusting where it came from.
	Detail string `json:"detail,omitempty"`
}

// statusToken admits an audit status only if it is one.
//
// A whitelist of shape, not of values: a release that adds a new NTSTATUS keeps
// working, and a release that starts putting a sentence in this field does not.
func statusToken(s string) string {
	if s == "" || len(s) > 64 {
		return ""
	}
	for _, r := range s {
		if (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '_' {
			return ""
		}
	}
	return s
}

// smbActivity reads the audit log and says what it could not see.
func (s *server) smbActivity(req OperationRequest) (OperationResult, int, error) {
	binding, err := s.boundAccount(req.Subject)
	if err != nil {
		return OperationResult{}, http.StatusUnprocessableEntity, err
	}

	// `message_timestamp` is an INTEGER — unix seconds — and this decoded it as
	// a string. `audit.query` therefore failed to unmarshal on every call, and
	// the operation answered "the audit log could not be read" every time it
	// was asked. It had never once succeeded against a real target. Measured on
	// TrueNAS-25.10.5, 2026-08-24.
	//
	// `event_data` is deliberately raw. Its shape varies by event type, and one
	// unexpected type anywhere inside it fails the WHOLE decode — which is
	// exactly how the timestamp took the operation down. What varies is decoded
	// separately and defensively; what does not is decoded here.
	var rows []struct {
		Timestamp int64           `json:"message_timestamp"`
		Event     string          `json:"event"`
		Success   bool            `json:"success"`
		Address   string          `json:"address"`
		EventData json.RawMessage `json:"event_data"`
	}
	filters := [][]any{{"username", "=", binding.Username}}
	// The lower bound, in the target's own units. The backend hands this over
	// as RFC3339 because that is what an HTTP caller writes; the column is an
	// integer, and a string compared against it is ACCEPTED and matches nothing
	// — a silently empty answer rather than a refusal, which is the worst of
	// the three outcomes available.
	if since, ok := req.Params["since"].(string); ok && strings.TrimSpace(since) != "" {
		at, err := time.Parse(time.RFC3339, strings.TrimSpace(since))
		if err != nil {
			return OperationResult{}, http.StatusBadRequest,
				fmt.Errorf("since must be an RFC3339 timestamp")
		}
		filters = append(filters, []any{"message_timestamp", ">=", at.Unix()})
	}
	query := []any{
		map[string]any{
			"services":      []string{"SMB"},
			"query-filters": filters,
			"query-options": map[string]any{"limit": 200, "order_by": []string{"-message_timestamp"}},
		},
	}
	if err := s.nas.call("audit.query", query, &rows); err != nil {
		return OperationResult{}, statusFor(err), fmt.Errorf("the audit log could not be read")
	}

	report := ActivityReport{Events: make([]ActivityEvent, 0, len(rows))}
	for _, r := range rows {
		event := ActivityEvent{
			At:      time.Unix(r.Timestamp, 0).UTC().Format(time.RFC3339),
			Event:   r.Event,
			Success: r.Success,
			Address: r.Address,
		}
		event.Share, event.Detail = eventDetail(r.EventData)
		report.Events = append(report.Events, event)
	}
	// Reported whether or not there were events, because the case that matters
	// is the empty one.
	if blind, err := s.sharesNotWatching(binding.Username); err != nil {
		report.UncoveredShares = []string{"(the audit coverage of the shares could not be read, so coverage is unknown)"}
	} else {
		report.UncoveredShares = blind
	}

	return OperationResult{
		Operation: "activity.get", Subject: req.Subject, Outcome: "succeeded",
		Activity: &report,
	}, http.StatusOK, nil
}

// decodeAnswer refuses an answer that is absent as well as one that is wrong.
//
// `json.Unmarshal` accepts a bare `null` into ANY destination, reports no
// error, and leaves it zeroed. A source answering null would therefore have
// been recorded as read-and-empty, which on the health surface is "no alerts"
// and "no pools" — the two most reassuring things it can say, arrived at
// without reading anything.
func decodeAnswer(raw json.RawMessage, into any) error {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return fmt.Errorf("the target answered nothing")
	}
	return json.Unmarshal(trimmed, into)
}

// eventDetail pulls the two useful things out of a payload whose shape changes
// with the event type.
//
// Decoded separately from the row so a surprise in here costs one field rather
// than the whole read. An SMB AUTHENTICATION row carries no share and a CONNECT
// row does; both are ordinary, and neither may make the other fail.
func eventDetail(raw json.RawMessage) (share, detail string) {
	if len(raw) == 0 {
		return "", ""
	}
	var data struct {
		Share  string `json:"share"`
		Result struct {
			Parsed string `json:"value_parsed"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return "", ""
	}
	return data.Share, statusToken(data.Result.Parsed)
}

// shareAudit is one share's audit settings.
//
// SMB auditing is per share AND per group. `enable` only says the feature is
// switched on; `watch_list` narrows it to named groups and `ignore_list`
// excludes them. Both hold group NAMES — `sharing.smb.update` refuses an entry
// that is not an SMB group by name, which is how their contents were learned.
type shareAudit struct {
	Name       string
	Enabled    bool
	WatchList  []string
	IgnoreList []string
}

// watches answers whether this share's auditing would record ONE account.
//
// A non-empty watch list is an allowlist: nobody outside it is recorded. The
// ignore list excludes, and excluding wins — a group named on both lists is a
// misconfiguration, and the safe reading of one is "not recorded", because the
// failure this whole surface exists to prevent is claiming coverage that is
// not there.
func (sh shareAudit) watches(memberOf map[string]bool) bool {
	if !sh.Enabled {
		return false
	}
	for _, group := range sh.IgnoreList {
		if memberOf[group] {
			return false
		}
	}
	if len(sh.WatchList) == 0 {
		return true
	}
	for _, group := range sh.WatchList {
		if memberOf[group] {
			return true
		}
	}
	return false
}

// readShareAudit reads every SMB share's audit settings.
func (s *server) readShareAudit() ([]shareAudit, error) {
	var shares []struct {
		Name  string `json:"name"`
		Audit struct {
			Enable     bool     `json:"enable"`
			WatchList  []string `json:"watch_list"`
			IgnoreList []string `json:"ignore_list"`
		} `json:"audit"`
	}
	if err := s.nas.call("sharing.smb.query", []any{[]any{}, map[string]any{"select": []string{"name", "audit"}}}, &shares); err != nil {
		return nil, err
	}
	out := make([]shareAudit, 0, len(shares))
	for _, sh := range shares {
		out = append(out, shareAudit{
			Name: sh.Name, Enabled: sh.Audit.Enable,
			WatchList: sh.Audit.WatchList, IgnoreList: sh.Audit.IgnoreList,
		})
	}
	return out, nil
}

// unauditedShares lists shares with auditing switched off.
//
// The TARGET-level question, with no member in it: the health card asks it
// about the deployment, not about anybody. `sharesNotWatching` is the
// member-level one and answers differently — a share can be switched on and
// still record nothing for a given person.
func (s *server) unauditedShares() ([]string, error) {
	shares, err := s.readShareAudit()
	if err != nil {
		return nil, err
	}
	var off []string
	for _, sh := range shares {
		if !sh.Enabled {
			off = append(off, sh.Name)
		}
	}
	return off, nil
}

// sharesNotWatching lists the shares whose auditing would record nothing for
// one account.
//
// This used to read `enable` alone, which was right until auditing was
// switched on. A share scoped by `watch_list` to groups the member is not in
// is enabled, records nothing about them, and was reported as covered — so an
// empty activity report read as "this member did nothing" when it meant
// "nobody was watching this member". That is the one distinction the field
// exists to make, and it was inverted the moment the setting it depends on
// stopped being uniformly off.
func (s *server) sharesNotWatching(username string) ([]string, error) {
	shares, err := s.readShareAudit()
	if err != nil {
		return nil, err
	}
	memberOf, err := s.groupsOf(username)
	if err != nil {
		return nil, err
	}
	var blind []string
	for _, sh := range shares {
		if !sh.watches(memberOf) {
			blind = append(blind, sh.Name)
		}
	}
	return blind, nil
}

// TargetHealth is the operator's view of the NAS itself.
//
// Every field is optional and each carries its own error, because this composes
// four independent reads and one of them failing tells an operator nothing about
// the other three — which are usually the ones that would have explained it.
//
// SHAPED, not forwarded. These four reads answer with several kilobytes each,
// including the chassis serial, the license blob and the full pool topology.
// Passing them through would put TrueNAS's schema in the backend and in the
// browser — the boundary this add-on exists to hold — and would ship hardware
// identifiers to answer questions nobody asked. What survives is what an
// operator looking at a target page needs and nothing else.
type TargetHealth struct {
	System   *SystemInfo    `json:"system,omitempty"`
	Alerts   []TargetAlert  `json:"alerts,omitempty"`
	Pools    []PoolStatus   `json:"pools,omitempty"`
	Services []ServiceState `json:"services,omitempty"`
	// Degraded names the sources that could not be read, so a partial answer
	// says which part is missing rather than looking whole.
	Degraded []string `json:"degraded,omitempty"`
}

// SystemInfo is the target identifying itself, minus the parts that identify
// the hardware. Serial, license and manufacturer are deliberately dropped.
type SystemInfo struct {
	Hostname      string  `json:"hostname,omitempty"`
	Version       string  `json:"version,omitempty"`
	UptimeSeconds float64 `json:"uptime_seconds,omitempty"`
}

// TargetAlert is one of the NAS's own alerts.
//
// The text is the alert's `formatted` field with markup removed. It is the
// target's own prose, which everywhere else in this package is refused — the
// rule exists because middleware ERROR text is built from a call whose
// parameters can include a member's password. An alert is the opposite case:
// it is written to be read by an operator, it is the entire value of the read,
// and it never sees a request's parameters. What it does carry is HTML, so the
// markup is stripped here rather than trusted to whatever renders it.
type TargetAlert struct {
	Level     string `json:"level"`
	Class     string `json:"klass,omitempty"`
	Text      string `json:"text"`
	At        string `json:"at,omitempty"`
	Dismissed bool   `json:"dismissed"`
}

// PoolStatus is one pool's health and how full it is.
type PoolStatus struct {
	Name           string `json:"name"`
	Status         string `json:"status"`
	Healthy        bool   `json:"healthy"`
	Warning        bool   `json:"warning"`
	FreeBytes      int64  `json:"free_bytes"`
	AllocatedBytes int64  `json:"allocated_bytes"`
	SizeBytes      int64  `json:"size_bytes"`
}

// ServiceState is one service, running or not, and whether it starts on boot.
//
// `cifs` stopped is the single most likely explanation for "my drive vanished",
// and it is invisible from every other read this add-on makes.
type ServiceState struct {
	Service string `json:"service"`
	State   string `json:"state"`
	Enabled bool   `json:"enable"`
}

// targetHealth composes four sources and degrades per source.
func (s *server) targetHealth(req OperationRequest) (OperationResult, int, error) {
	h := TargetHealth{}
	sources := []struct {
		name   string
		method string
		params any
		decode func(json.RawMessage) error
	}{
		{"system", "system.info", []any{}, func(raw json.RawMessage) error {
			var in struct {
				Hostname      string  `json:"hostname"`
				Version       string  `json:"version"`
				UptimeSeconds float64 `json:"uptime_seconds"`
			}
			if err := decodeAnswer(raw, &in); err != nil {
				return err
			}
			h.System = &SystemInfo{Hostname: in.Hostname, Version: in.Version, UptimeSeconds: in.UptimeSeconds}
			return nil
		}},
		{"alerts", "alert.list", []any{}, func(raw json.RawMessage) error {
			var in []struct {
				Level     string  `json:"level"`
				Klass     string  `json:"klass"`
				Formatted *string `json:"formatted"`
				Text      *string `json:"text"`
				Dismissed bool    `json:"dismissed"`
				Datetime  epochMs `json:"datetime"`
			}
			if err := decodeAnswer(raw, &in); err != nil {
				return err
			}
			h.Alerts = make([]TargetAlert, 0, len(in))
			for _, a := range in {
				body := ""
				if a.Formatted != nil {
					body = *a.Formatted
				} else if a.Text != nil {
					body = *a.Text
				}
				h.Alerts = append(h.Alerts, TargetAlert{
					Level: a.Level, Class: a.Klass, Text: stripMarkup(body),
					At: a.Datetime.RFC3339(), Dismissed: a.Dismissed,
				})
			}
			return nil
		}},
		{"pools", "pool.query", []any{}, func(raw json.RawMessage) error {
			var in []struct {
				Name      string `json:"name"`
				Status    string `json:"status"`
				Healthy   bool   `json:"healthy"`
				Warning   bool   `json:"warning"`
				Free      int64  `json:"free"`
				Allocated int64  `json:"allocated"`
				Size      int64  `json:"size"`
			}
			if err := decodeAnswer(raw, &in); err != nil {
				return err
			}
			h.Pools = make([]PoolStatus, 0, len(in))
			for _, p := range in {
				h.Pools = append(h.Pools, PoolStatus{
					Name: p.Name, Status: p.Status, Healthy: p.Healthy, Warning: p.Warning,
					FreeBytes: p.Free, AllocatedBytes: p.Allocated, SizeBytes: p.Size,
				})
			}
			return nil
		}},
		{"services", "service.query", []any{}, func(raw json.RawMessage) error {
			var in []struct {
				Service string `json:"service"`
				State   string `json:"state"`
				Enable  bool   `json:"enable"`
			}
			if err := decodeAnswer(raw, &in); err != nil {
				return err
			}
			h.Services = make([]ServiceState, 0, len(in))
			for _, sv := range in {
				h.Services = append(h.Services, ServiceState{Service: sv.Service, State: sv.State, Enabled: sv.Enable})
			}
			return nil
		}},
	}
	for _, source := range sources {
		var raw json.RawMessage
		if err := s.nas.call(source.method, source.params, &raw); err != nil {
			h.Degraded = append(h.Degraded, source.name)
			continue
		}
		// A source that ANSWERED and could not be understood is degraded too.
		// Discarding the decode error left the field absent and the source
		// unnamed, which the surface renders as "nothing raised" — the
		// reassuring answer, produced by a schema change nobody had noticed.
		if err := source.decode(raw); err != nil {
			h.Degraded = append(h.Degraded, source.name)
		}
	}
	// Every source failing is an unreachable target, not a health report with
	// four holes in it — the distinction an operator needs is "the NAS is down"
	// rather than "four things are wrong with it". Counted against the table
	// rather than against a literal, so adding a fifth source cannot quietly
	// turn this into a check that never fires.
	if len(h.Degraded) == len(sources) {
		return OperationResult{}, http.StatusServiceUnavailable, fmt.Errorf("the target answered none of the health reads")
	}
	return OperationResult{
		Operation: "health.get", Subject: req.Subject, Outcome: "succeeded", Health: &h,
	}, http.StatusOK, nil
}

// epochMs decodes TrueNAS's `{"$date": 1787077600000}` timestamp wrapper.
//
// Its own type because the same wrapper appears on several reads and because a
// plain int64 field would silently decode to zero against it — the shape of the
// defect that kept `activity.get` broken for its whole life.
type epochMs struct {
	Date int64 `json:"$date"`
}

func (e epochMs) RFC3339() string {
	if e.Date == 0 {
		return ""
	}
	return time.UnixMilli(e.Date).UTC().Format(time.RFC3339)
}

// stripMarkup turns an alert's HTML into the sentence it was trying to be.
//
// TrueNAS writes alerts with `<br>` in them. Removing the markup here means no
// consumer is ever handed something it might be tempted to render as HTML, and
// the add-on stays the only place that knows the target emits any.
func stripMarkup(s string) string {
	var b strings.Builder
	depth := 0
	for _, r := range s {
		switch {
		case r == '<':
			depth++
			// A tag boundary is a word boundary: `a<br>b` is two words.
			if b.Len() > 0 && !strings.HasSuffix(b.String(), " ") {
				b.WriteByte(' ')
			}
		case r == '>':
			if depth > 0 {
				depth--
			}
		case depth == 0:
			b.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// requireSecret pulls a declared secret parameter out of the request.
//
// The refusal names the PARAMETER and never the value, and never says anything
// about the value's shape either: an error string is logged, returned, and
// captured in traces, and "must be at least 8 characters" is a fact about a
// password somebody submitted.
func requireSecret(params map[string]any, name string) (string, error) {
	raw, present := params[name]
	if !present {
		return "", fmt.Errorf("%s is required", name)
	}
	value, ok := raw.(string)
	if !ok || strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return value, nil
}

// adoptAccount binds an account the target already holds to a subject.
//
// The one entry point for it, reached from two places: the unmanaged inventory
// an operator browses, and the binding conflict an apply halts on. Design §11
// requires them to leave identical state, and the way to guarantee that is for
// there to be one action rather than two that agree today.
//
// It writes a binding and touches the account itself in no way at all. Whatever
// groups, lock state and SMB flag it already has stay exactly as they are until
// the next convergence, which is the operator's next decision and not this one's
// side effect — an adoption that also converged would apply a diff nobody was
// shown, on an account whose contents nobody has looked at yet.
func (s *server) adoptAccount(req OperationRequest) (OperationResult, int, error) {
	username, _ := req.Params["username"].(string)
	username = strings.TrimSpace(username)
	if username == "" {
		return OperationResult{}, http.StatusBadRequest, fmt.Errorf("no account named")
	}
	if strings.TrimSpace(req.Subject) == "" {
		return OperationResult{}, http.StatusBadRequest, fmt.Errorf("no subject to bind it to")
	}
	// Before the lookup, not after. The account exists and would resolve
	// perfectly well; the reason it is not on offer has nothing to do with
	// whether it can be found.
	if err := s.refuseSelfAccount(username); err != nil {
		return OperationResult{}, http.StatusUnprocessableEntity, err
	}

	// Already bound to this subject: reported as done rather than refused. The
	// dedup store makes a replay return the original answer, and this covers the
	// other repeat — an operator adopting the same account twice from two
	// screens — without a second mutation.
	if existing, bound, err := s.store.GetBinding(req.Subject); err != nil {
		return OperationResult{}, http.StatusInternalServerError, err
	} else if bound {
		if existing.Username == username {
			return OperationResult{
				Operation: "account.adopt", Subject: req.Subject, Outcome: "succeeded",
				// The uid travels on the repeat too. Without it the second
				// adoption would hand the backend a zero and overwrite a
				// binding that was correct with one that is not — the repeat
				// path making the state worse than the first attempt left it.
				AccountUID: existing.UID,
				Detail:     fmt.Sprintf("%s was already bound to this subject.", username),
			}, http.StatusOK, nil
		}
		// Rebinding is not adoption. The subject already has an account here,
		// and moving them to another one abandons the first — its home
		// directory, its ACL entries, its shares — with nothing recording that
		// it was ever theirs.
		return OperationResult{}, http.StatusConflict,
			fmt.Errorf("this subject is already bound to %s", existing.Username)
	}

	claimed, err := s.store.BoundUsernames()
	if err != nil {
		return OperationResult{}, http.StatusInternalServerError, err
	}
	if owner, taken := claimed[username]; taken {
		// Somebody else's. Refused rather than moved: this is the mistake the
		// whole "adoption is an operator decision" rule exists to prevent, and
		// making it recoverable by rebinding would make it silent.
		return OperationResult{}, http.StatusConflict,
			fmt.Errorf("%s is already bound to another subject (%s)", username, owner)
	}

	// Read from the target rather than trusted from the request. The inventory
	// the operator was looking at is a snapshot, and an account deleted or
	// renamed since would otherwise be bound by name to nothing.
	snap, err := s.readSubjects()
	if err != nil {
		return OperationResult{}, statusFor(err), err
	}
	var account *Subject
	for i := range snap.Subjects {
		if snap.Subjects[i].Username == username {
			account = &snap.Subjects[i]
			break
		}
	}
	if account == nil {
		// "No such account" covers two cases and says both, because the second
		// one is a refusal rather than a miss: the snapshot excludes the
		// operating system's own accounts, so `root` reaches here exactly as a
		// misspelling does — and an operator who typed `root` deliberately is
		// owed the reason rather than a puzzle.
		return OperationResult{}, http.StatusNotFound,
			fmt.Errorf("the target has no adoptable account named %s (system accounts are never adoptable)", username)
	}

	if err := s.store.PutBinding(Binding{
		SubjectID: req.Subject, Username: account.Username, UID: account.UID,
		// Who decided it. An adoption that turns out to be wrong hands somebody
		// else's data to a member, and the actor is the first thing asked for.
		BoundBy: req.Actor,
	}); err != nil {
		return OperationResult{}, http.StatusInternalServerError, err
	}

	s.record("account.adopt", req.Subject, req.Actor, req.CallID, "succeeded")
	return OperationResult{
		Operation: "account.adopt", Subject: req.Subject, Outcome: "succeeded",
		AccountUID: account.UID,
		Detail:     fmt.Sprintf("%s is now bound to this subject. Nothing on the account was changed.", account.Username),
	}, http.StatusOK, nil
}
