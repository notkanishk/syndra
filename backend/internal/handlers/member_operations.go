package handlers

import (
	"errors"
	"log"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"syndra/internal/addons"
	"syndra/internal/db"
	"syndra/internal/services/addonop"
)

// A member acting on the world through a target (design §10, task 6.3): a
// physical operation, item.checkout today. Two routes, one per question:
//
//	POST /api/v1/me/targets/{target}/operations/{name}
//	     {"params": {...}, "preview": true}  what would happen; nothing is sent
//	                                          but the preview, nothing recorded
//	     {"params": {...}, "confirm": true}  do it: one record, one fresh
//	                                          call_id (the record's id), sent once
//	GET  /api/v1/me/targets/{target}/operations/{id}
//	     where that confirm stands: checking, done, no_change, not_done, or
//	     ask_desk once it has been in doubt past constants.in_doubt_bound
//
// Self-scoped as the credential route is: the subject is the authenticated
// member, never the request's, and a member reads only their own records.
// Nothing here sends anything twice: a lost answer is settled by asking the
// add-on what it recorded (GET /calls), and a second confirm while the first
// is unsettled is refused (CALL_UNSETTLED) before it has a record.

// memberActionRequest is the body of POST .../operations/{name}. Exactly one of
// preview and confirm is true.
type memberActionRequest struct {
	Params  map[string]any `json:"params"`
	Preview bool           `json:"preview,omitempty"`
	Confirm bool           `json:"confirm,omitempty"`
}

// The member's states for a physical operation's confirm. The page words each
// one and every one ends in a next step (plain-language-copy).
const (
	actionChecking = "checking" // sent, the answer lost; Syndra is asking the system
	actionDone     = "done"
	actionNoChange = "no_change" // the system says it was already so; nothing changed
	actionNotDone  = "not_done"  // nothing went out, or the system says it never happened
	actionAskDesk  = "ask_desk"  // still in doubt past in_doubt_bound: staff have it
)

var (
	addonopPreview         = addonop.Preview
	dbAddonOperationByID   = db.AddonOperationByID
	dbOpenCall             = db.OpenCall
	addonsResolveOperation = addons.ResolveOperation
	addonsViewOf           = addons.ViewOf
	// addonsOperations is a target's effective operations, none when it is
	// not registered.
	addonsOperations = func(target string) []addons.EffectiveOperation {
		a, err := addonsGet(target)
		if err != nil {
			return nil
		}
		return a.Operations()
	}
)

// handleMemberAction is POST /api/v1/me/targets/{target}/operations/{name}.
func handleMemberAction(w http.ResponseWriter, r *http.Request) {
	subject := resolveActor(r, "")
	if strings.TrimSpace(subject) == "" {
		jsonErrorResponse(w, http.StatusUnauthorized, "NO_SUBJECT", "This action is a person's own.")
		return
	}
	target, name := r.PathValue("target"), r.PathValue("name")

	var req memberActionRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}
	if req.Preview == req.Confirm {
		jsonValidationErrorResponse(w, "Ask for a preview or confirm, one of the two.",
			map[string]string{"preview": "exactly one of preview and confirm", "confirm": "exactly one of preview and confirm"})
		return
	}
	if req.Params == nil {
		req.Params = map[string]any{}
	}

	// Only what this member's page offers: a member-scoped physical operation
	// the add-on can do now. Anything else names nothing they can act on.
	op, err := addonsResolveOperation(target, name)
	var unavailable *addons.ErrOperationUnavailable
	switch {
	case errors.As(err, &unavailable):
		jsonErrorResponse(w, http.StatusConflict, "ACTION_UNAVAILABLE", "That can't be done on this system right now.")
		return
	case err != nil || op.Effect != addons.EffectPhysical || op.Scope != addons.ScopeMember:
		jsonErrorResponse(w, http.StatusNotFound, "NO_SUCH_ACTION", "There is no such action on this system.")
		return
	}

	// The same state checks as setting a password, before anything is sent: a
	// member with no role here, no account yet, a paused system or one that
	// is not answering is told so, and nothing went out.
	view, err := describeMyTarget(r, target, subject)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "VIEW_ERROR", err.Error())
		return
	}
	switch {
	case !view.Entitled:
		jsonErrorResponse(w, http.StatusConflict, "NO_ENTITLEMENT", "No role of yours reaches this system. Nothing was done.")
		return
	case view.Account == nil:
		jsonErrorResponse(w, http.StatusConflict, "ACCOUNT_PENDING", "Your account here has not been created yet. Nothing was done.")
		return
	case view.Lifecycle == "draining" || view.Lifecycle == "read_only":
		jsonErrorResponse(w, http.StatusConflict, "TARGET_PAUSED", "Changes on that system are paused. Nothing was done.")
		return
	case !view.Reachable:
		jsonErrorResponse(w, http.StatusServiceUnavailable, "TARGET_UNREACHABLE", "That system is not answering right now. Nothing went out.")
		return
	}

	opReq := addonop.Request{Target: target, Operation: name, ActorID: subject, SubjectID: subject,
		Confirmed: req.Confirm, Params: req.Params, About: aboutOf(op, req.Params)}
	if req.Preview {
		previewAction(w, r, opReq)
		return
	}
	confirmAction(w, r, opReq)
}

// aboutOf is what the record says the action was about: the value of its
// target_id parameter (the item), never anything secret (a physical
// operation's parameters are not secret, and aboutOperations has the last word).
func aboutOf(op addons.EffectiveOperation, params map[string]any) string {
	for _, p := range op.Params {
		if p.Format == addons.FormatTargetID && !op.IsSecretParam(p.Name) {
			if v, ok := params[p.Name].(string); ok {
				return v
			}
		}
	}
	return ""
}

func previewAction(w http.ResponseWriter, r *http.Request, req addonop.Request) {
	reply, err := addonopPreview(r.Context(), req)
	if err != nil {
		writeActionError(w, err)
		return
	}
	switch reply.Outcome {
	case addons.OutcomeSucceeded:
		outcome := actionDone
		if reply.NoChange {
			outcome = actionNoChange
		}
		v := reply.View()
		jsonResponse(w, http.StatusOK, map[string]any{"state": "preview", "outcome": outcome, "view": v})
	case addons.OutcomeRejected:
		writeRefusal(w, reply.Code, reply.Refusal)
	default:
		// A preview has no effect by contract, so a lost answer to one is
		// still nothing done; the member can ask again.
		jsonErrorResponse(w, http.StatusServiceUnavailable, "PREVIEW_UNANSWERED",
			"That system didn't answer. Nothing went out; try again.")
	}
}

func confirmAction(w http.ResponseWriter, r *http.Request, req addonop.Request) {
	res, err := svcDispatchOperation(r.Context(), req)
	if err != nil && res.OperationID == "" {
		writeActionError(w, err)
		return
	}
	if err != nil {
		// Sent, and its outcome could not be written: as unknown as a lost
		// answer, and answered as one.
		log.Printf("[MY-ACTION] %v", err)
		res.Outcome = addons.OutcomeIndeterminate
	}
	switch res.Outcome {
	case addons.OutcomeSucceeded:
		state := actionDone
		if res.NoChange {
			state = actionNoChange
		}
		jsonResponse(w, http.StatusOK, map[string]any{"state": state, "operation": res.OperationID, "view": res.View})
	case addons.OutcomeRejected:
		writeRefusal(w, res.Code, res.Refusal)
	case addons.OutcomeUnreached:
		jsonErrorResponse(w, http.StatusServiceUnavailable, "TARGET_UNREACHABLE", "That system is not answering right now. Nothing went out.")
	default:
		// 202 (RFC 9110 §15.3.3): accepted, and not known to be done. The
		// page polls GET .../operations/{id}; nothing sends it again.
		jsonResponse(w, http.StatusAccepted, map[string]any{"state": actionChecking, "operation": res.OperationID, "sent_at": time.Now().UTC()})
	}
}

// refusalStatus is the status each registered refusal keeps on the way to the
// page (RFC 9110): 404 the param names nothing (§15.5.5), 403 the member's
// entitlements do not reach it (§15.5.4), 409 a conflict with its current
// state, held by someone, a limit, an earlier call unsettled (§15.5.10).
var refusalStatus = map[string]int{
	"UNKNOWN_RESOURCE": http.StatusNotFound,
	"NOT_ENTITLED":     http.StatusForbidden,
	"RESOURCE_TAKEN":   http.StatusConflict,
	"LIMIT_REACHED":    http.StatusConflict,
	"CALL_UNSETTLED":   http.StatusConflict,
}

// writeRefusal answers an add-on's refusal of a physical operation: nothing
// was done. The code passes through for the page's words; details carry what
// it names (the parameter, the limit), never a value the member typed.
func writeRefusal(w http.ResponseWriter, code string, ref addons.Refusal) {
	status, known := refusalStatus[code]
	if !known {
		// Refused for a reason the page has no words for: understood and not
		// done (RFC 9110 §15.5.21).
		code, status = "ACTION_REFUSED", http.StatusUnprocessableEntity
	}
	details := map[string]string{}
	if ref.Param != "" {
		details["param"] = ref.Param
	}
	if ref.Limit != nil && ref.Count != nil && ref.Unit != "" {
		details["limit"], details["count"], details["unit"] = strconv.Itoa(*ref.Limit), strconv.Itoa(*ref.Count), ref.Unit
	}
	resp := ErrorResponse{Error: code, Message: "The system refused it. Nothing was done."}
	if len(details) > 0 {
		resp.Details = details
	}
	jsonResponse(w, status, resp)
}

// writeActionError answers a refusal Syndra made itself, before sending.
func writeActionError(w http.ResponseWriter, err error) {
	var unsettled *addonop.UnsettledError
	switch {
	case errors.As(err, &unsettled):
		jsonResponse(w, http.StatusConflict, ErrorResponse{Error: "CALL_UNSETTLED",
			Message: "Your last one is still being checked. Nothing was sent.",
			Details: map[string]string{"operation": unsettled.OperationID, "sent_at": unsettled.SentAt.UTC().Format(time.RFC3339)}})
	case errors.Is(err, addonop.ErrCallUnsettled):
		// Could not tell whether an earlier one is unsettled: fail closed.
		jsonErrorResponse(w, http.StatusServiceUnavailable, "CALL_UNSETTLED_UNKNOWN",
			"Syndra couldn't check your last one. Nothing was sent; try again shortly.")
	case errors.Is(err, addonop.ErrRateLimited):
		jsonErrorResponse(w, http.StatusTooManyRequests, "TOO_MANY_ATTEMPTS", "You have done this a lot recently. Wait a few minutes and try again.")
	case errors.Is(err, addons.ErrInvalidParams):
		jsonValidationErrorResponse(w, "That isn't something this action takes.", map[string]string{"params": err.Error()})
	default:
		writeCredentialError(w, err)
	}
}

// handleMemberActionStatus is GET /api/v1/me/targets/{target}/operations/{id}:
// where one of the member's own confirms stands. A row still open is settled
// first by asking the add-on what it recorded (settleByCall, the operator
// list's own question), so the answer is the latest anybody can know.
func handleMemberActionStatus(w http.ResponseWriter, r *http.Request) {
	subject := resolveActor(r, "")
	if strings.TrimSpace(subject) == "" {
		jsonErrorResponse(w, http.StatusUnauthorized, "NO_SUBJECT", "This view is a person's own.")
		return
	}
	target, id := r.PathValue("target"), r.PathValue("id")
	notFound := func() {
		jsonErrorResponse(w, http.StatusNotFound, "NO_SUCH_ACTION", "There is no such action of yours here.")
	}
	if !operationID.MatchString(id) {
		notFound()
		return
	}
	o, err := dbAddonOperationByID(r.Context(), target, subject, id)
	switch {
	case errors.Is(err, db.ErrAddonOperationNotFound) || (err == nil && !slices.Contains(addons.PhysicalOperations(), o.Operation)):
		// Only the actions this route starts: another record of theirs
		// (a password change) has its own page and its own settling.
		notFound()
		return
	case err != nil:
		jsonErrorResponse(w, http.StatusInternalServerError, "ACTION_READ_FAILED", err.Error())
		return
	}
	body := map[string]any{"operation": o.ID, "sent_at": o.SentAt()}
	if !o.Dispatched() && o.Unresolved() && o.Resolution == "" {
		// Never sent: settled as the operator's list settles it, unreached
		// once past the grace, so a crash between record and claim does not
		// leave the member checking a call that never left.
		if n, err := dbSettleUnclaimedAddonOperations(r.Context(), target, unresolvedGrace); err != nil {
			log.Printf("[MY-ACTION] could not settle unsent records on %s: %v (non-fatal)", target, err)
		} else if n > 0 {
			if again, err := dbAddonOperationByID(r.Context(), target, subject, id); err == nil {
				o = again
			}
		}
	}
	if o.Dispatched() && o.Unresolved() && o.Resolution == "" {
		settled, s, _ := settleByCall(r.Context(), o)
		if settled {
			if again, err := dbAddonOperationByID(r.Context(), target, subject, id); err == nil {
				o = again
			}
		}
		if s.State == addons.CallRecorded && len(s.Reply) > 0 {
			v := addonsViewOf(target, o.Operation, s.Reply, s.Clock)
			body["view"] = v
			if replyNoChange(s.Reply) {
				o.Status = db.AddonOpNoChange
			}
		}
	}
	body["state"] = memberActionState(o, time.Now())
	jsonResponse(w, http.StatusOK, body)
}

// replyNoChange is a recorded reply saying it changed nothing.
func replyNoChange(reply []byte) bool {
	var r struct {
		Outcome string `json:"outcome"`
	}
	return addons.DecodeReply(reply, &r) == nil && r.Outcome == "no_change"
}

// memberActionState is where a record stands for the member who made it.
func memberActionState(o db.AddonOperation, now time.Time) string {
	switch {
	case o.Status == db.AddonOpNoChange:
		return actionNoChange
	case o.Status == db.AddonOpSucceeded, o.Resolution == db.AddonResolutionHappened, o.Resolution == db.AddonResolutionSuperseded:
		return actionDone
	case o.Status == db.AddonOpRejected, o.Status == db.AddonOpUnreached, o.Resolution == db.AddonResolutionNotHappened:
		return actionNotDone
	case now.Sub(o.SentAt()) >= addons.InDoubtBound:
		return actionAskDesk
	}
	return actionChecking
}

// ---- the member's page -----------------------------------------------------

// myAction is a physical operation the member can start from their page,
// with the parameters the form asks for (format target_id: typed or scanned).
type myAction struct {
	ID      string        `json:"id"`
	Confirm bool          `json:"confirm"`
	Params  []myParamSpec `json:"params"`
}

type myParamSpec struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Required bool   `json:"required"`
	Format   string `json:"format,omitempty"`
}

// myUnsettled is the member's latest confirm of an action still being
// settled, so a reload keeps saying "checking" (or, past the bound, "ask at
// the desk") rather than offering the action again.
type myUnsettled struct {
	Operation string    `json:"operation"`
	ID        string    `json:"id"`
	SentAt    time.Time `json:"sent_at"`
	State     string    `json:"state"`
}

// memberActions is every available member physical operation on the target,
// and the one of them, if any, still unsettled for the subject. Best-effort:
// a page that cannot read one just offers less.
func memberActions(r *http.Request, target, subject string) ([]myAction, *myUnsettled) {
	var actions []myAction
	var unsettled *myUnsettled
	for _, op := range addonsOperations(target) {
		if !op.Available || op.Effect != addons.EffectPhysical || op.Scope != addons.ScopeMember {
			continue
		}
		a := myAction{ID: op.ID, Confirm: op.Confirm, Params: []myParamSpec{}}
		for _, p := range op.Params {
			a.Params = append(a.Params, myParamSpec{Name: p.Name, Type: p.Type, Required: p.Required, Format: p.Format})
		}
		actions = append(actions, a)
		id, at, open, err := dbOpenCall(r.Context(), target, subject, op.ID)
		if err != nil {
			log.Printf("[MY-ACTION] could not read %s's last %s on %s: %v (non-fatal)", subject, op.ID, target, err)
			continue
		}
		if open && (unsettled == nil || at.After(unsettled.SentAt)) {
			state := actionChecking
			if time.Since(at) >= addons.InDoubtBound {
				state = actionAskDesk
			}
			unsettled = &myUnsettled{Operation: op.ID, ID: id, SentAt: at, State: state}
		}
	}
	return actions, unsettled
}
