package handlers

import (
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"syndra/internal/addons"
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
	// Reachable says the add-on answered. A member whose target is down is told
	// so rather than shown a credential form that will fail.
	Reachable bool `json:"reachable"`
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
	// they differ in exactly the way a member needs: a drain is minutes and ends
	// by itself, and read-only is somebody working on the server and ends when
	// they say so. One deserves "shortly" and the other must not be given it.
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

// handleMyTargets is a member's own storage view, for every registered target.
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
		view, err := describeMyTarget(r, reg.Target, subject)
		if err != nil {
			jsonErrorResponse(w, http.StatusInternalServerError, "VIEW_ERROR", err.Error())
			return
		}
		views = append(views, view)
	}
	jsonResponse(w, http.StatusOK, map[string]any{"targets": views})
}

func describeMyTarget(r *http.Request, target, subject string) (myTargetView, error) {
	view := myTargetView{Target: target}

	set, err := svcResolveEntitlementSet(r.Context(), subject, target)
	if err != nil {
		return myTargetView{}, err
	}
	view.Entitled = set.Lifecycle.Enabled
	view.Suspended = set.Suppressed
	if view.Entitled {
		// Only the values that survived resolution. A suppressed value listed
		// here would be an instruction to reach something they cannot.
		view.Resources = set.Fields
	}

	binding, bound, err := dbGetTargetBinding(r.Context(), target, subject)
	if err != nil {
		return myTargetView{}, err
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
		return view, nil
	}
	health := addonsHealth(r.Context(), target)
	view.Reachable = health.Reachable
	view.Lifecycle = health.Lifecycle

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
		// Best-effort: a page that cannot read usage is still worth serving,
		// and every field it fills is additive.
		if status := addonsMyStorage(r.Context(), target, subject); status.Outcome == addons.OutcomeSucceeded {
			view.Storage = &status
		}
	}
	return view, nil
}

type setCredentialRequest struct {
	Password string `json:"password"`
}

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
	case !view.Reachable:
		// Fails closed and says so. Accepting it and reporting success would
		// tell a member their password works when nothing received it.
		jsonErrorResponse(w, http.StatusServiceUnavailable, "TARGET_UNREACHABLE",
			"That system is not answering right now. Nothing was changed — try again shortly.")
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

	res, err := svcDispatchOperation(r.Context(), addonop.Request{
		Target: target, Operation: "password.set",
		ActorID: subject, SubjectID: subject,
		Params: map[string]any{"password": req.Password},
	})
	if err != nil {
		writeCredentialError(w, err)
		return
	}
	if res.Outcome != addons.OutcomeSucceeded {
		// Indeterminate included, deliberately. The member is told to check
		// rather than told it worked: an operation carrying a secret is never
		// auto-retried, and reporting an unknown outcome as success is the one
		// lie this path must not tell.
		jsonResponse(w, http.StatusAccepted, map[string]any{
			"status":    string(res.Outcome),
			"operation": res.OperationID,
			"detail":    "The system did not confirm the change. Try connecting with the new password; if it does not work, set it again.",
		})
		return
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
