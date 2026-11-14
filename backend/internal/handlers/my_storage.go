package handlers

import (
	"context"
	"errors"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"syndra/internal/addons"
	"syndra/internal/db"
	"syndra/internal/extcall"
	"syndra/internal/handlers/kinds"
	"syndra/internal/services"
	"syndra/internal/services/addonop"
)

// A member's own view of a target (design §4, §14; change `addon-platform`
// group 10).
//
// Three states, and the surface must render them as three rather than as
// "access / no access". They are what a member actually experiences, and
// collapsing any two of them produces a screen that lies:
//
//	no entitlement          — no role of theirs is mapped here. There is nothing
//	                          to set a credential for and no instructions to
//	                          follow, so neither is offered.
//	entitled, no account    — a role reached the target and the convergence has
//	                          not been drained yet. The credential affordance is
//	                          withheld: setting one would be dispatched at an
//	                          account that does not exist, and the member would
//	                          be told their password was set.
//	account present         — everything is offered.
//
// The second state is the one a two-state design gets wrong, and it is not an
// edge case: add-on rows wait for an operator to resume the drain, so it is the
// ordinary experience of every new member for as long as that takes.

type myTargetView struct {
	Target string `json:"target"`
	// Name is what the system is called (systemName), the same name the
	// operator's roster carries.
	Name string `json:"name"`
	// Entitled says at least one mapped role reaches this target for this
	// person. Derived from the resolver, never from the target's own state — a
	// member whose account exists and whose entitlement has lapsed is not
	// entitled, and the account is on its way to being disabled.
	Entitled bool `json:"entitled"`
	// Account is the add-on-reported name they connect with, or nil. Read from
	// the backend's own binding record rather than the target, so this page does
	// not put a rate-limited WebSocket behind an ordinary page load.
	Account *myTargetAccount `json:"account,omitempty"`
	// Resources is what their current entitlements reach, by field. Only what
	// resolves: an unreachable resource listed here is an instruction that
	// fails, which is worse than an instruction that is missing.
	Resources map[string][]string `json:"resources,omitempty"`
	// Suspended says an operator has withheld something they would otherwise
	// hold, with the reason. A member seeing "no access" with no explanation
	// asks an operator; a member seeing the reason does not have to.
	Suspended []services.SuppressedEntitlement `json:"suspended,omitempty"`
	// Credential is existence and last-change metadata, and never a value. The
	// vault holds no credential for this path — the member's password is
	// forwarded to the target and kept nowhere.
	Credential myCredentialStatus `json:"credential"`
	// Reachable says the target is confirmed online (Health is `online`). A
	// member whose target is down is told so rather than shown a credential form
	// that will fail.
	Reachable bool `json:"reachable"`
	// Health is what Syndra can honestly say: `online`, `offline` (three failed
	// reads in a row) or `unknown` (can't tell yet, so the page says nothing).
	// One slow answer stays online.
	Health addons.HealthStatus `json:"health"`
	// CheckedAt is the latest health read; ReachableSince is when the target
	// stopped answering, set only while it is offline.
	CheckedAt      *time.Time `json:"checked_at,omitempty"`
	ReachableSince *time.Time `json:"reachable_since,omitempty"`
	// Lifecycle is the state an operator put the target in: `active`,
	// `draining` or `read_only`.
	//
	// A member is affected by this and was never told. Under either pause their
	// ACCESS is unchanged — the file server works and their files are where
	// they left them — and what stops is Syndra making changes to their
	// account, of which the one they can start from this page is setting a
	// password. Saying nothing meant a member set one, watched it not work, and
	// had no way to learn why.
	//
	// The two states are kept apart rather than collapsed to a boolean, because
	// they differ in what a member is told: a drain lets the last few changes
	// finish before staff work on the server, and read-only is staff working on
	// it. Neither ends by itself: an operator sets the add-on back to active
	// (addons/truenas lifecycle.go, POST /lifecycle via handlers/inventory.go),
	// or a restart does after a shutdown drain. Nothing resumes on the end of a
	// pause either: a change the add-on declined is released to pending
	// (propagation/addon.go), where a revocation goes on the background drain
	// and anything else waits for an operator's Send. So neither state is
	// given a time or a "by itself" (final verification, MyStorage).
	Lifecycle string `json:"lifecycle,omitempty"`
	// Storage is what the TARGET says about this account: whether it can be
	// used yet, and how much room is left.
	//
	// Distinct from Credential above, which is Syndra's own record that a
	// password was set. That record cannot say whether the target still accepts
	// it, and the difference is not academic: an account Syndra created has
	// password authentication disabled until the member sets one, so it exists,
	// is correct, and refuses them.
	Storage *addons.StorageStatus `json:"storage,omitempty"`
	// Reads is every member read the target answered
	// (storage.status, holdings.list), for the generic renderers: its shape,
	// its result and what the add-on also says. A kind module may render the
	// ones it specialises itself (storage reads Storage above).
	Reads []addons.ReadView `json:"reads,omitempty"`
	// RecordedAt dates the middle state — entitled, no account yet — which is
	// the ordinary experience of every new member because these changes wait
	// for an operator rather than for a timer.
	//
	// Sent so the page can state an age instead of a promise. "This usually
	// clears within a day" is a guess about a person; "recorded two days ago"
	// is true, and it is what stops a member refreshing four times a day
	// against a screen that always says the same thing.
	RecordedAt *time.Time `json:"recorded_at,omitempty"`
	// Connection is how they reach it, when the add-on's deployment has said.
	//
	// From the add-on's registration rather than from a constant here, because
	// moving the NAS must not mean editing a component — and absent rather than
	// guessed, because a share path that does not work teaches a member to
	// distrust the whole page, starting with the parts that were right.
	Connection *addons.Connection `json:"connection,omitempty"`
	// RestoredAt is when access was given back, set only while the account
	// waits for a new password after it (storage.needs_new_password): the
	// latest end of a deny hold on this target.
	RestoredAt *time.Time `json:"restored_at,omitempty"`
	// Kind and EntitlementSchema are the target's accepted manifest's, as
	// GET /targets carries them, because a member cannot read that list: the
	// page renders this target's fields by role. Absent until a manifest is
	// accepted ("not read"). Target metadata only, nothing about anyone else.
	Kind              string                    `json:"kind,omitempty"`
	EntitlementSchema []addons.EntitlementField `json:"entitlement_schema,omitempty"`
	// Standing is the member's own account's standing, from the add-on's
	// storage.status reply about their account alone (contract 2, design §6):
	// usable or not, and who is waited on. Absent when there is no account or
	// the add-on did not answer.
	Standing *addons.Standing `json:"standing,omitempty"`
	// HomeKept is a home folder that no role the member holds gives any more.
	// It stays, with their files, and only they can open it.
	HomeKept bool `json:"home_kept,omitempty"`
	// Roles are the member's own roles that reach this target, for "Your
	// Laser operator role reaches TrueNAS, but no folder there is open to it".
	Roles []services.RoleSource `json:"roles,omitempty"`
	// PasswordChange is present only while their latest password change here
	// is unconfirmed: sent, its answer lost (and its one resend's too), and no
	// later confirmed change has settled it (A-13 §4 step 3). The page keeps
	// one line up until their next confirmed change clears it.
	PasswordChange *myPasswordChange `json:"password_change,omitempty"`
	// Actions are what the member can do to the world here (a member-scoped
	// physical operation the add-on can do now, item.checkout), each previewed
	// then confirmed (member_operations.go). Only with an account; never on
	// the operator's view of a person.
	Actions []myAction `json:"actions,omitempty"`
	// Unsettled is their latest confirm still being settled, so a reload keeps
	// saying where it stands instead of offering the action again.
	Unsettled *myUnsettled `json:"unsettled,omitempty"`
	// Usage is what this person has used here over time (feature stats,
	// design §9.8): their own per-person points only, from Syndra's store,
	// so it is there while the target is down. Reported false when the
	// add-on reports no statistics; absent when they could not be read.
	Usage *statsUsage `json:"usage,omitempty"`
}

type myPasswordChange struct {
	State string    `json:"state"` // "unconfirmed"
	At    time.Time `json:"at"`
}

type myTargetAccount struct {
	Username string     `json:"username"`
	BoundAt  *time.Time `json:"bound_at,omitempty"`
}

type myCredentialStatus struct {
	// Set says a credential exists on the TARGET, as far as Syndra knows.
	// Tracked as metadata rather than by holding the value, so this is the whole
	// of what can be said about it.
	Set bool `json:"set"`
	// NeedsReEnrolment is somebody who set a credential before the LLDAP bridge
	// was retired. Their hash was dropped with the vault and the system it was
	// for no longer exists, so they have to set a new one — and "you enrolled
	// before the change" is a different sentence from "you have never set one"
	// to somebody who remembers doing it (group 11, tasks 11.8/11.9).
	NeedsReEnrolment bool `json:"needs_re_enrolment,omitempty"`
	// LastChangedAt is the last time this member set or rotated it.
	LastChangedAt *time.Time `json:"last_changed_at,omitempty"`
}

// handleMyTargets is a member's own storage view, for every target a member
// meets (memberMeets).
//
// Registered targets, not entitled ones: the nav leaf is present for every
// member regardless of entitlement (10.1/10.2), so the page behind it has to
// have something to say to a member with no access at all. What changes with
// entitlement is the content, never the structure.
func handleMyTargets(w http.ResponseWriter, r *http.Request) {
	subject := resolveActor(r, "")
	if strings.TrimSpace(subject) == "" {
		jsonErrorResponse(w, http.StatusUnauthorized, "NO_SUBJECT", "This view is a person's own.")
		return
	}

	registrations := addonsRegistered()
	views := make([]myTargetView, 0, len(registrations))
	for _, reg := range registrations {
		if _, ok := memberMeets(r.Context(), reg.Target); !ok {
			continue
		}
		view, err := describeMyTarget(r, reg.Target, subject)
		if err != nil {
			jsonErrorResponse(w, http.StatusInternalServerError, "VIEW_ERROR", err.Error())
			return
		}
		view.Usage = personalUsage(r.Context(), reg.Target, subject, true)
		views = append(views, view)
	}
	jsonResponse(w, http.StatusOK, map[string]any{"targets": views})
}

// storageStatusRead is the read whose answer is also the storage kind's own
// block (Storage): the storage profile's result.
const storageStatusRead = "storage.status"

func describeMyTarget(r *http.Request, target, subject string) (myTargetView, error) {
	view, _, err := describeTargetFor(r, target, subject)
	if err == nil && view.Account != nil {
		view.Actions, view.Unsettled = memberActions(r, target, subject)
	}
	return view, err
}

// describeTargetFor is describeMyTarget with the entitlement set it resolved, for
// the operator's person page, which reads more from the same set.
func describeTargetFor(r *http.Request, target, subject string) (myTargetView, services.EntitlementSet, error) {
	view := myTargetView{Target: target}
	view.Name, _ = systemName(r.Context(), target)
	view.Kind, view.EntitlementSchema = acceptedShape(r.Context(), target)

	set, err := svcResolveEntitlementSet(r.Context(), subject, target)
	switch {
	case errors.Is(err, addons.ErrNoManifest):
		// The target has never had a manifest accepted, so nothing says which
		// field is its account: what this person holds there is not read, and
		// says so by saying nothing (not entitled, no resources) rather than
		// failing the page for every other target.
		set = services.EntitlementSet{}
	case err != nil:
		return myTargetView{}, services.EntitlementSet{}, err
	}
	view.Entitled = set.Lifecycle.Account
	view.Suspended = set.Suppressed
	view.Roles = rolesReaching(set)
	if view.Entitled {
		// Only the values that survived resolution. A suppressed value listed
		// here would be an instruction to reach something they cannot.
		view.Resources = set.Fields
	}

	binding, bound, err := dbGetTargetBinding(r.Context(), target, subject)
	if err != nil {
		return myTargetView{}, services.EntitlementSet{}, err
	}
	if bound {
		boundAt := binding.BoundAt
		view.Account = &myTargetAccount{Username: binding.Username, BoundAt: &boundAt}
	} else if view.Entitled {
		// Only in the middle state, and only because that is the state with
		// nothing else to say. Read once the binding is known absent so an
		// ordinary page load for a member who already has an account does not
		// pay for a query about when they got it.
		//
		// Non-fatal: an age is what makes the wait honest, and failing to read
		// it is a reason to say less rather than to fail the page.
		if at, found, err := dbEntitlementRecordedAt(r.Context(), target, subject); err != nil {
			log.Printf("[MY-STORAGE] could not date %s's entitlement on %s: %v (non-fatal)", subject, target, err)
		} else if found {
			view.RecordedAt = &at
		}
	}

	// Non-fatal: the line is a courtesy over a form that works either way.
	if _, at, open, err := dbUnconfirmedChangeAt(r.Context(), target, subject, "password.set", unresolvedGrace); err != nil {
		log.Printf("[MY-STORAGE] could not read %s's last password change on %s: %v (non-fatal)", subject, target, err)
	} else if open {
		view.PasswordChange = &myPasswordChange{State: "unconfirmed", At: at}
	}

	status, err := dbHasShadowCredential(r.Context(), subject, view.Target)
	if err == nil {
		view.Credential = myCredentialStatus{
			Set:              status.HasCredential,
			NeedsReEnrolment: status.NeedsReEnrolment,
		}
		if status.RotatedAt != nil {
			view.Credential.LastChangedAt = status.RotatedAt
		} else if status.UpdatedAt != nil {
			view.Credential.LastChangedAt = status.UpdatedAt
		}
	}

	// One probe, and only to decide whether to offer the form. A member told
	// their credential was set against an add-on that never answered is the one
	// outcome this page must not produce.
	//
	// Callable is not reachable, and the difference is the whole point here. An
	// add-on answers `/capabilities` from its own process; the credential is set
	// on the TARGET, over a session the add-on may not currently have. Reading
	// only the manifest offered the form to a member whose NAS was switched off,
	// and the failure arrived after they had typed a password — which is the
	// exact outcome the paragraph above forbids.
	//
	// A health read that fails is read as unreachable rather than ignored: this
	// field withdraws a form, and withdrawing one wrongly is a member told to
	// come back later, while offering one wrongly is a member told their
	// credential is set when it is not.
	if !addonsCallable(target) {
		return view, set, nil
	}
	// The live read is counted by the health monitor; the page then says what
	// the monitor can stand behind, so one slow answer does not read as an
	// outage and a confirmed one carries the time it began.
	health := addonsHealth(r.Context(), target)
	state := addonsHealthOf(target)
	view.Health = state.Status
	view.Reachable = state.Status == addons.HealthOnline
	if !state.CheckedAt.IsZero() {
		checked := state.CheckedAt
		view.CheckedAt = &checked
	}
	view.ReachableSince = state.ReachableSince
	view.Lifecycle = health.Lifecycle
	if view.Lifecycle == "" {
		// The add-on did not answer this time; the last lifecycle it reported
		// still stands, since only an operator changes it.
		view.Lifecycle = state.Lifecycle
	}

	// Only where there is an account to connect WITH. In the other two states
	// the instructions would describe reaching something that is not there —
	// the same rule the credential form follows, for the same reason.
	if view.Account != nil {
		if connection, err := addonsConnection(target); err == nil {
			view.Connection = connection
		}
		// What the target says about their account right now, which is the only
		// authority on whether it can actually be used. Syndra's own record
		// says a credential was set; it cannot say whether the target still
		// accepts it, and the gap between those is a member staring at working
		// instructions that refuse them.
		//
		// Every member read the target offers, each through the one read path
		// and each once: storage.status's answer also feeds the storage block.
		// Best-effort: a page that cannot read one is still worth serving, and
		// every field it fills is additive. The actor is whoever asked: the
		// member on their own page, the operator on the person's page (each
		// route decides who may ask).
		for _, op := range addonsMemberReads(target) {
			// fresh only when the person pressed "Read again" (wantsFresh);
			// a page load and the waiting poll accept the add-on's copy.
			reply := addonsRead(r.Context(), target, op, subject, resolveActor(r, ""), nil, wantsFresh(r))
			view.Reads = append(view.Reads, reply.View())
			if op != storageStatusRead {
				continue
			}
			if status := reply.Storage(); status.Outcome == addons.OutcomeSucceeded {
				for i := range status.Places {
					kinds.Address(view.Connection, &status.Places[i])
				}
				view.Standing = status.Standing
				attributePlaces(r.Context(), target, set, status.Places, wantsFresh(r))
				view.HomeKept = homeKept(set, status.Places)
				view.Storage = &status
				rememberStorage(target, subject, status)
				if status.NeedsNewPassword {
					view.RestoredAt = restoredAt(r.Context(), subject, target)
				}
			}
		}
	}
	return view, set, nil
}

// attributePlaces names the member's own roles behind each place: the roles
// that put them in a group that opens it. Best-effort, from the target's last
// read of its folder permissions; a place with no attribution just says less.
func attributePlaces(ctx context.Context, target string, set services.EntitlementSet, places []addons.Place, fresh bool) {
	membership := kinds.Storage(set.Roles).Membership
	groups := set.Fields[membership]
	if len(places) == 0 || len(groups) == 0 {
		return
	}
	reach := addonsReadReach(ctx, target, fresh)
	if !reach.Read {
		return
	}
	for i := range places {
		if places[i].Home {
			continue
		}
		seen := map[services.RoleSource]bool{}
		for _, g := range groups {
			for _, opened := range reach.Groups[g] {
				if opened.Path != places[i].Path {
					continue
				}
				for _, role := range set.Given[membership+"\x00"+g] {
					if !seen[role] {
						seen[role] = true
						places[i].Roles = append(places[i].Roles, addons.PlaceRole{ProjectID: role.ProjectID, RoleKey: role.RoleKey})
					}
				}
			}
		}
	}
}

// rolesReaching is every held role that gave this target anything, once each,
// in a stable order.
func rolesReaching(set services.EntitlementSet) []services.RoleSource {
	seen := map[services.RoleSource]bool{}
	var out []services.RoleSource
	for _, roles := range set.Given {
		for _, r := range roles {
			if !seen[r] {
				seen[r] = true
				out = append(out, r)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ProjectID != out[j].ProjectID {
			return out[i].ProjectID < out[j].ProjectID
		}
		return out[i].RoleKey < out[j].RoleKey
	})
	return out
}

// homeKept is a home folder on the account that none of the member's roles
// gives any more. A target with no one flag to give a home cannot say.
func homeKept(set services.EntitlementSet, places []addons.Place) bool {
	home := kinds.Storage(set.Roles).Home
	for _, p := range places {
		if p.Home && home != "" {
			return len(set.Fields[home]) == 0
		}
	}
	return false
}

// restoredAt is when a deny hold on this target last ended, lifted or lapsed.
// Nil when none has, or the history can't be read: the page then says access
// was given back without a date rather than guessing one.
func restoredAt(ctx context.Context, subject, target string) *time.Time {
	all, err := dbAllowancesForSubject(ctx, subject)
	if err != nil {
		return nil
	}
	now := time.Now()
	var latest *time.Time
	for _, a := range all {
		if a.Target != target || a.Direction != db.AllowanceDeny {
			continue
		}
		ended := a.LiftedAt
		if ended == nil && a.ExpiresAt != nil && !a.ExpiresAt.After(now) {
			ended = a.ExpiresAt
		}
		if ended != nil && (latest == nil || ended.After(*latest)) {
			t := *ended
			latest = &t
		}
	}
	return latest
}

type setCredentialRequest struct {
	Password string `json:"password"`
	// Resend is the page sending the same password once more, at the next try
	// the first answer named (`retry.next_try`, extcall.AddonResend), after
	// the first send's reply was lost (A-13 §4 step 1). Only the page can: it
	// is the only place the value still is. The server keeps no copy and never
	// retries a member's password by itself.
	Resend bool `json:"resend,omitempty"`
}

// resendWindow is how far back a resend may reach for the send it repeats:
// past the first send's own worst case (the add-on call's 30-second timeout,
// plus whatever a proxy in front adds before it gives up on the reply).
const resendWindow = 5 * time.Minute

// handleSetMyCredential forwards a member's chosen credential to the target and
// keeps nothing.
//
// The subject is the authenticated actor and is never taken from the request. A
// `member`-scoped operation binds the subject as well as the audience (design
// §14): without that, "scoped to member" would only mean "a member may call
// this", and `password.set` with somebody else's id resets their storage
// credential.
func handleSetMyCredential(w http.ResponseWriter, r *http.Request) {
	subject := resolveActor(r, "")
	if strings.TrimSpace(subject) == "" {
		jsonErrorResponse(w, http.StatusUnauthorized, "NO_SUBJECT", "This action is a person's own.")
		return
	}
	target := r.PathValue("target")

	var req setCredentialRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}

	// Refused before the dispatch, and refused for the state rather than for the
	// permission. A member holding no mapped role has nothing to set a
	// credential on; one whose account has not been created yet would have the
	// call dispatched at an account that does not exist. Both are told which.
	view, err := describeMyTarget(r, target, subject)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "VIEW_ERROR", err.Error())
		return
	}
	switch {
	case !view.Entitled:
		jsonErrorResponse(w, http.StatusConflict, "NO_ENTITLEMENT",
			"No role of yours reaches this system, so there is nothing to set a password for.")
		return
	case view.Account == nil:
		jsonErrorResponse(w, http.StatusConflict, "ACCOUNT_PENDING",
			"Your account here has not been created yet. This usually clears within a day; try again after that.")
		return
	case view.Lifecycle == "draining" || view.Lifecycle == "read_only":
		// Refused before dispatch. The add-on would decline it anyway, and the
		// member used to get "did not confirm" for a pause an operator chose.
		// Syndra keeps no copy of the password, so nothing could apply it later
		// either: the page says to change it once the pause lifts.
		jsonErrorResponse(w, http.StatusConflict, "TARGET_PAUSED",
			"Account changes on that system are paused, so your password can't be changed right now. Your current one keeps working.")
		return
	case !view.Reachable:
		// Fails closed and says so. Accepting it and reporting success would
		// tell a member their password works when nothing received it.
		jsonErrorResponse(w, http.StatusServiceUnavailable, "TARGET_UNREACHABLE",
			"That system is not answering right now. Nothing was changed. Try again shortly.")
		return
	}

	// Complexity is checked HERE, before the value leaves the process. Checking
	// it after dispatch would mean a rejected password had already reached the
	// target, and checking it in the frontend alone would mean it had not been
	// checked.
	if err := services.ValidatePasswordComplexity(req.Password); err != nil {
		jsonValidationErrorResponse(w, err.Error(), map[string]string{"password": "complexity"})
		return
	}

	// A resend names the send it repeats, found here rather than taken from
	// the page: the page may never have learnt the first send's id (its reply
	// was the thing lost). None found (the first never got as far as a
	// record, or was answered clearly) makes this an ordinary first send.
	var retryOf string
	if req.Resend {
		id, found, err := dbResendableAddonOperation(r.Context(), target, subject, "password.set", resendWindow)
		if err != nil {
			jsonErrorResponse(w, http.StatusInternalServerError, "RESEND_CHECK_FAILED", err.Error())
			return
		}
		if found {
			retryOf = id
		}
	}

	res, err := svcDispatchOperation(r.Context(), addonop.Request{
		Target: target, Operation: "password.set",
		ActorID: subject, SubjectID: subject,
		Params:  map[string]any{"password": req.Password},
		RetryOf: retryOf,
	})
	if err != nil && res.OperationID != "" {
		// Sent, and its outcome could not be written: as unknown to the member
		// as a lost reply, and answered as one. "Nothing was changed" here
		// would be the one lie this path must not tell.
		log.Printf("[MY-STORAGE] %v", err)
		res.Outcome = addons.OutcomeIndeterminate
	} else if err != nil {
		writeCredentialError(w, err)
		return
	}
	if res.Outcome != addons.OutcomeSucceeded {
		// Never success, early or otherwise (owner, 3 Nov). The page tells the
		// three apart: indeterminate is a lost reply, which it sends again once
		// with the value it still holds; rejected and unreached are clear
		// answers that nothing changed, and are not resent.
		body := map[string]any{
			"status":    string(res.Outcome),
			"operation": res.OperationID,
			"detail":    unconfirmedCredentialCopy[res.Outcome],
		}
		if res.Outcome == addons.OutcomeIndeterminate {
			body["retry"] = credentialRetry(req.Resend, time.Now())
		}
		jsonResponse(w, http.StatusAccepted, body)
		return
	}

	// Confirmed, so every earlier change of theirs here that nobody could
	// confirm is settled by this one (A-13 §4: it clears itself). Non-fatal:
	// the password is set either way, and the line stays until the next.
	if _, err := dbSupersedeAddonOperations(r.Context(), res.OperationID); err != nil {
		log.Printf("[MY-STORAGE] %s's confirmed change on %s did not settle the earlier ones: %v", subject, target, err)
	}

	// Recorded only after the target confirmed it. Metadata, never a value —
	// and a failure to write it does not un-set the password, so it is logged
	// through the audit path rather than reported as a failed change.
	if err := svcRecordCredentialSet(r.Context(), subject, target, subject, r.RemoteAddr); err != nil {
		log.Printf("[VAULT] %s set a credential on %s and it was not recorded: %v", subject, target, err)
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"status":    "set",
		"operation": res.OperationID,
		// Said plainly, because the scope is the thing members get wrong: this
		// is not their Syndra password and not their Google password.
		"detail": "Your storage password is set. It is used only for this system.",
	})
}

// credentialRetry is where a lost-reply password change stands under the one
// retry model (extcall.AddonResend): after the first send, retrying, with the
// moment the page sends it again and "try 2 of 2"; after the resend, nothing
// tries again until the member does (waiting).
func credentialRetry(resend bool, now time.Time) extcall.RetryState {
	p := extcall.AddonResend
	if resend {
		return p.Retrying(p.Attempts, nil)
	}
	next := p.NextTryAfter(now, 1)
	return p.Retrying(1, &next)
}

// unconfirmedCredentialCopy is the backend's sentence for each answer that is
// not "set". The page words its own lines; this is for any other reader.
var unconfirmedCredentialCopy = map[addons.Outcome]string{
	addons.OutcomeIndeterminate: "The system didn't answer, so your password may not have changed.",
	addons.OutcomeRejected:      "The system refused the change. Nothing was changed.",
	addons.OutcomeUnreached:     "The change didn't reach the system. Nothing was changed.",
}

func writeCredentialError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, addonop.ErrSubjectNotActor):
		jsonErrorResponse(w, http.StatusForbidden, "NOT_YOURS", err.Error())
	case errors.Is(err, addonop.ErrRateLimited):
		jsonErrorResponse(w, http.StatusTooManyRequests, "TOO_MANY_ATTEMPTS",
			"You have changed this recently. Wait a few minutes and try again.")
	case errors.Is(err, addons.ErrNotRegistered):
		jsonErrorResponse(w, http.StatusNotFound, "TARGET_NOT_REGISTERED", err.Error())
	default:
		// The error text is Syndra's own. The add-on's response body is where a
		// submitted password comes back, and it never reaches here.
		jsonErrorResponse(w, http.StatusBadGateway, "CREDENTIAL_NOT_SET",
			"That could not be applied. Nothing was changed.")
	}
}
