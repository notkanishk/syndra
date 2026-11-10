package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"slices"
	"strings"
	"time"

	"syndra/internal/clock"
	"syndra/internal/db"
	"syndra/internal/directory"
	"syndra/internal/extcall"
	"syndra/internal/models"
	"syndra/internal/services"
)

type UpsertDirectGrantRequest struct {
	ProjectID string `json:"project_id"`
	RoleKey   string `json:"role_key"`
	// GrantedBy is a fallback for demo/local-dev where the request context has
	// no authenticated principal (API-key auth doesn't carry user identity).
	// In production (Zitadel JWT auth) the authenticated subject takes
	// precedence over this body field — clients should not send it.
	GrantedBy    string `json:"granted_by,omitempty"`
	Reason       string `json:"reason"`
	DurationDays int    `json:"duration_days"`
	// Extend says this changes the end of a grant they already have, to a
	// later one. Without it, giving a role they already have is refused.
	Extend bool `json:"extend,omitempty"`
	// The plan the giving sheet was shown (GET /users/{id}/plan?op=give).
	planFields
}

type CreateAccessRequestRequest struct {
	RequesterID   string `json:"requester_id"`
	ProjectID     string `json:"project_id"`
	RoleKey       string `json:"role_key"`
	Justification string `json:"justification"`
	DurationDays  int    `json:"duration_days"`
	// Until is a fixed end date the ask names ("until the end of term", RFC
	// 3339). When present, approving gives the grant this end however late it
	// is decided, and it wins over DurationDays. Absent: days count from the
	// decision, as before (older clients send only days).
	Until *time.Time `json:"until,omitempty"`
	// Kind is "access" (the default) or "extend": "Ask to keep it longer".
	Kind string `json:"kind,omitempty"`
}

type ResolveAccessRequestRequest struct {
	Status string `json:"status"`
	// ReviewerID is a fallback for demo/local-dev. In production the
	// authenticated subject from the bearer token is used instead.
	ReviewerID string `json:"reviewer_id,omitempty"`
	ReviewNote string `json:"review_note"`
	// The plan the row was shown (GET /users/{id}/plan?op=approve), for an
	// approval.
	planFields
}

// resolveActor prefers the authenticated principal from the request context
// (set by withUserAuth from a Zitadel-issued JWT). Falls back to a
// caller-supplied body value when the context principal is empty (demo/API-key
// mode). Returns "system" only when both are empty.
// resolveActor names who is acting: the authenticated principal, else what the
// body said, else "system".
//
// The second argument is the BODY's value, not a default. Several surfaces
// passed the literal "operator" into it, which made an unauthenticated call on
// those surfaces record `operator` while every other surface recorded `system`
// — two synthetic ids for one caller, on a column an audit trail is read from.
// There is one fallback and it is here.
func resolveActor(r *http.Request, bodyValue string) string {
	if id := getAdminUserID(r.Context()); id != "" {
		return id
	}
	if id := strings.TrimSpace(bodyValue); id != "" {
		return id
	}
	return "system"
}

func handleGetUserDirectGrants(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	grants, err := svcUserDirectGrants(r.Context(), userID)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, grants)
}

func handleUpsertUserDirectGrant(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	var req UpsertDirectGrantRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}
	if !trimmedNonEmpty(userID) {
		jsonValidationErrorResponse(w, "id path parameter is required", map[string]string{"id": "required"})
		return
	}
	if !trimmedNonEmpty(req.ProjectID) || !trimmedNonEmpty(req.RoleKey) {
		jsonValidationErrorResponse(w, "project_id and role_key are required", map[string]string{
			"project_id": "required",
			"role_key":   "required",
		})
		return
	}
	if req.DurationDays < 0 {
		jsonValidationErrorResponse(w, "duration_days must be zero or greater", map[string]string{"duration_days": "min=0"})
		return
	}

	grantedBy := resolveActor(r, req.GrantedBy)

	if !req.Extend {
		var expiresAt *time.Time
		if req.DurationDays > 0 {
			expiry := clock.EndAfterDays(clock.Now(), req.DurationDays)
			expiresAt = &expiry
		}
		givePlanned(w, r, userID, grantedBy, expiresAt, req)
		return
	}
	extendPlanned(w, r, userID, grantedBy, req)
}

// extendPlanned moves the end of a grant they have through the planner (task
// 1.2a): a ledger edit, nothing sent to Zitadel (it has no end dates; Syndra
// ends access on the day), unless the new end cancels a removal still waiting,
// which the sheet asked about before the press. Extending is for a role that
// ends, and never shortens (owner, 15 Oct).
func extendPlanned(w http.ResponseWriter, r *http.Request, userID, actor string, req UpsertDirectGrantRequest) {
	payload, _ := json.Marshal(req)
	ch, err := svcApplyAct(r.Context(),
		services.Action{Op: services.OpExtend, UserID: userID, ProjectID: req.ProjectID, RoleKey: req.RoleKey, Days: req.DurationDays},
		services.ActWrite{Actor: actor, Reason: req.Reason, Give: db.EnqueueParams{PayloadJSON: string(payload)}},
		req.seen())
	switch refusedAs(err) {
	case "nothing_to_extend":
		jsonErrorResponse(w, http.StatusConflict, "NOTHING_TO_EXTEND",
			"There is no end to extend: they have this with no end, or don't have it. Nothing was changed.")
		return
	case "runs_past":
		jsonErrorResponse(w, http.StatusConflict, "ALREADY_RUNS_PAST",
			"It already runs past that date, so extending would shorten it. Nothing was changed.")
		return
	}
	if err != nil {
		writeActError(w, err)
		return
	}
	rebuildUserCacheDetached(r.Context(), userID)
	res := actResult(ch)
	deliverAct(r, ch, &res)
	jsonResponse(w, http.StatusAccepted, res)
}

// ownWait splits one delivery of a write and its rules' rows (re-audit R12):
// why the act's OWN row isn't in Zitadel ("" once it is applied), its next
// try when that is a retry, and how many rule-given rows wait for the
// operator. A rule-given role that waits is never blamed on the act, and is
// never left unsaid. With no own row, the delivery's first reason speaks.
func ownWait(d services.Delivery, own, ownStatus string) (string, *extcall.RetryState, int) {
	if own == "" {
		return d.WaitReason(), d.Drain.Retry, 0
	}
	reason := d.ReasonFor(own)
	if ownStatus == "applied" && d.Reason[own] == "" {
		reason = ""
	}
	var retry *extcall.RetryState
	if reason == db.WaitReasonHalted(db.HaltRetrying) {
		retry = d.Drain.Retry
	}
	return reason, retry, d.WaitingBesides(own)
}

func handleGetAccessRequests(w http.ResponseWriter, r *http.Request) {
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	switch status {
	case "", "pending", "approved", "rejected", "withdrawn", "fulfilled":
	default:
		jsonValidationErrorResponse(w, "status must be one of pending, approved, rejected, withdrawn", map[string]string{"status": "enum"})
		return
	}
	requests, err := dbGetAccessRequests(r.Context(), status)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	// Members see only their own requests — the org-wide list (with
	// justifications) is operator-scoped (SC3). An operator's ?mine=1 asks for
	// the same narrowing: the requests they asked for themselves.
	if !isOperator(r) || r.URL.Query().Get("mine") == "1" {
		self := getAdminUserID(r.Context())
		own := make([]models.AccessRequest, 0, len(requests))
		for _, req := range requests {
			if req.RequesterID == self {
				own = append(own, req)
			}
		}
		requests = own
	}
	jsonResponse(w, http.StatusOK, withHolding(r.Context(), requests))
}

// withHolding reads, for each waiting request, what the requester already has
// of the role, so whoever decides sees what approving would change: nothing
// (they have it for good), a later end, or new access. A read that fails
// leaves the row without it rather than failing the list.
func withHolding(ctx context.Context, requests []models.AccessRequest) []models.AccessRequest {
	for i := range requests {
		if requests[i].Status != "pending" {
			continue
		}
		h, err := dbRequestHolding(ctx, requests[i].RequesterID, requests[i].ProjectID, requests[i].RoleKey)
		if err != nil {
			log.Printf("[ACCESS] could not read what %s holds for request %s: %v", requests[i].RequesterID, requests[i].ID, err)
			continue
		}
		requests[i].Holding = &h
		if requests[i].Kind == services.RequestKindExtend {
			// The same end approving will set (planApproval), for the queue.
			requests[i].CurrentUntil = h.Until
			if p, err := services.PlanApproval(requests[i], h, time.Now()); err == nil {
				requests[i].Until = p.ExpiresAt
			}
		}
	}
	return requests
}

func handleCreateAccessRequest(w http.ResponseWriter, r *http.Request) {
	var req CreateAccessRequestRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}
	// A member can only file requests as themselves — the authenticated subject
	// overrides whatever requester_id the client sent (SC8). Operators may file
	// on behalf of another user; dev mode (API-key, no subject) trusts the body.
	if !isOperator(r) {
		req.RequesterID = getAdminUserID(r.Context())
	}
	if !trimmedNonEmpty(req.RequesterID) || !trimmedNonEmpty(req.ProjectID) || !trimmedNonEmpty(req.RoleKey) || !trimmedNonEmpty(req.Justification) {
		jsonValidationErrorResponse(w, "requester_id, project_id, role_key, and justification are required", map[string]string{
			"requester_id":  "required",
			"project_id":    "required",
			"role_key":      "required",
			"justification": "required",
		})
		return
	}
	if req.DurationDays < 0 {
		jsonValidationErrorResponse(w, "duration_days must be zero or greater", map[string]string{"duration_days": "min=0"})
		return
	}
	// One clock for both checks on the end date: a date that passes between them is no more
	// "ended" than it was when the ask arrived.
	now := time.Now()
	if req.Until != nil && !req.Until.After(now) {
		jsonValidationErrorResponse(w, "until must be a date that hasn't passed yet", map[string]string{"until": "future"})
		return
	}
	if req.Kind == "" {
		req.Kind = services.RequestKindAccess
	}
	if req.Kind != services.RequestKindAccess && req.Kind != services.RequestKindExtend {
		jsonValidationErrorResponse(w, `kind must be "access" or "extend"`, map[string]string{"kind": "enum"})
		return
	}

	if err := checkRoleOffered(r.Context(), req.ProjectID, req.RoleKey); err != nil {
		writeRoleOfferedError(w, err)
		return
	}

	var durationDays *int
	if req.DurationDays > 0 {
		durationDays = &req.DurationDays
	}

	// An ask is for a role they don't have; asking to keep one longer is for
	// a role that ends (owner, 15 and 16 Oct). Neither gives a role twice.
	holding, err := dbRequestHolding(r.Context(), req.RequesterID, req.ProjectID, req.RoleKey)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	switch err := services.CheckRequestKind(req.Kind, holding, durationDays, req.Until, now); {
	case errors.Is(err, services.ErrAskEnded):
		jsonValidationErrorResponse(w, "until must be a date that hasn't passed yet", map[string]string{"until": "future"})
		return
	case errors.Is(err, services.ErrRequestAlreadyHeld):
		jsonErrorResponse(w, http.StatusConflict, "ALREADY_HELD", alreadyHeldMessage(holding, "You"))
		return
	case errors.Is(err, services.ErrNothingHeldToExtend):
		jsonErrorResponse(w, http.StatusConflict, "NOTHING_TO_EXTEND", "You don't have this, so there's nothing to keep longer. Ask for it instead.")
		return
	case errors.Is(err, services.ErrNothingToExtend):
		jsonErrorResponse(w, http.StatusConflict, "NOTHING_TO_EXTEND", "You already have this with no end date, so there's nothing to keep longer.")
		return
	case errors.Is(err, services.ErrExtensionShortens):
		jsonErrorResponse(w, http.StatusConflict, "ALREADY_RUNS_PAST",
			fmt.Sprintf("You already have this %s, which is later than that. Choose a longer time.", services.EndPhrase(holding.Until)))
		return
	}

	id, err := dbCreateAccessRequestOfKind(r.Context(), req.Kind, req.RequesterID, req.ProjectID, req.RoleKey, req.Justification, durationDays, req.Until)
	if errors.Is(err, db.ErrAlreadyAsked) {
		// The same ask is already waiting: one in the queue, however many
		// times it was sent. Its id is returned so the page can point at it.
		jsonResponse(w, http.StatusConflict, map[string]any{
			"error":   "ALREADY_ASKED",
			"message": "You've already asked for this, and it's waiting for an answer.",
			"details": map[string]string{"waiting_request_id": id},
		})
		return
	}
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	// Audit actor is the authenticated principal when present (admin acting on
	// behalf of a member, or the member themselves). Falls back to the
	// requester so demo-mode self-requests still attribute correctly.
	actor := resolveActor(r, req.RequesterID)
	_ = dbInsertAuditLog(r.Context(), actor, req.RequesterID, "access_request.created", id)
	jsonResponse(w, http.StatusCreated, map[string]string{"id": id})
}

// alreadyHeldMessage says what someone has of a role they were about to be
// given or to ask for. who is "You" or "They".
func alreadyHeldMessage(h models.RequestHolding, who string) string {
	msg := fmt.Sprintf("%s already have this, with no end date.", who)
	if h.Kind == models.HoldingUntil && h.Until != nil {
		msg = fmt.Sprintf("%s already have this, until %s. Ask to keep it longer instead.",
			who, h.Until.In(clock.Loc).Format("2 Jan 2006"))
		if who == "They" {
			msg = fmt.Sprintf("They already have this, until %s. Extend it instead.", h.Until.In(clock.Loc).Format("2 Jan 2006"))
		}
	}
	// Given, not yet sent: still held, so not given twice, but it may not work yet.
	if h.Waiting {
		// The operator is the one who sends it; the member waits on them.
		if who == "They" {
			msg += " It's waiting for you to send, so it may not work yet."
		} else {
			msg += " Makerspace staff haven't sent it yet, so it may not work just yet."
		}
	}
	return msg
}

// handleWithdrawAccessRequest lets the person who filed a request take it back.
//
// Self-only, and self-only for operators too. An operator withdrawing somebody else's ask is a
// rejection with the reviewer's name left off — the decision endpoint exists for that, and it
// records who took it.
//
// A withdrawn request grants nothing, so there is no cascade and nothing to drain; the only
// state that changes is that it leaves the queue.
// POST /api/v1/requests/{id}/withdraw
func handleWithdrawAccessRequest(w http.ResponseWriter, r *http.Request) {
	requestID := r.PathValue("id")
	if !trimmedNonEmpty(requestID) {
		jsonValidationErrorResponse(w, "id path parameter is required", map[string]string{"id": "required"})
		return
	}

	request, err := dbGetAccessRequestByID(r.Context(), requestID)
	if err != nil {
		jsonErrorResponse(w, http.StatusNotFound, "NOT_FOUND", "No such request. It may have been withdrawn.")
		return
	}

	// Production: the authenticated subject must be the requester. Dev mode (API-key auth, no
	// subject) falls through to the requester on the row, and db.WithdrawAccessRequest still
	// scopes the UPDATE by it — the guard is in the statement, not only here.
	actor := getAdminUserID(r.Context())
	if actor != "" && actor != request.RequesterID {
		jsonErrorResponse(w, http.StatusForbidden, "FORBIDDEN", "You can only withdraw your own requests")
		return
	}

	switch err := dbWithdrawAccessRequest(r.Context(), requestID, request.RequesterID); {
	case err == nil:
		_ = dbInsertAuditLog(r.Context(), request.RequesterID, request.RequesterID, "access_request.withdrawn", requestID)
		jsonResponse(w, http.StatusOK, map[string]string{"message": "Request withdrawn"})
	case errors.Is(err, db.ErrRequestNotPending):
		// Somebody decided it in the meantime, or it was already withdrawn. Either way the
		// member's copy of this row is stale, and saying so beats reporting a failure.
		jsonErrorResponse(w, http.StatusConflict, "ALREADY_RESOLVED", "This request has already been decided.")
	default:
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
	}
}

func handleResolveAccessRequest(w http.ResponseWriter, r *http.Request) {
	requestID := r.PathValue("id")
	if !trimmedNonEmpty(requestID) {
		jsonValidationErrorResponse(w, "id path parameter is required", map[string]string{"id": "required"})
		return
	}

	var req ResolveAccessRequestRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}

	status := strings.ToLower(strings.TrimSpace(req.Status))
	if status != "approved" && status != "rejected" {
		jsonValidationErrorResponse(w, "status must be approved or rejected", map[string]string{"status": "enum"})
		return
	}

	reviewer := resolveActor(r, req.ReviewerID)
	if status == "approved" && reviewer == "system" {
		jsonValidationErrorResponse(w, "an authenticated reviewer is required when approving a request", map[string]string{"reviewer_id": "required_when=status:approved"})
		return
	}

	decided, err := resolveAccessRequestOutcome(r.Context(), requestID, status, reviewer, req.ReviewNote, req.seen())
	switch {
	case err == nil:
		// For an approval: effect (given, extended, given_again), until (the
		// end it now has; absent for none), and wait_reason when it isn't in
		// Zitadel yet (role_waits or halted:<reason>). The outcome says each.
		resp := map[string]any{"message": "Request resolved"}
		if decided.Effect != "" {
			resp["effect"] = decided.Effect
		}
		if decided.Until != nil {
			resp["until"] = decided.Until
		}
		if decided.WaitReason != "" {
			resp["wait_reason"] = decided.WaitReason
		}
		if decided.Retry != nil {
			resp["retry"] = decided.Retry
		}
		if decided.RuleWaiting > 0 {
			resp["rule_waiting"] = decided.RuleWaiting
		}
		jsonResponse(w, http.StatusOK, resp)
	case errors.Is(err, services.ErrRequestNotFound):
		jsonErrorResponse(w, http.StatusNotFound, "NOT_FOUND", err.Error())
	case errors.Is(err, services.ErrRequestAlreadyResolved):
		jsonErrorResponse(w, http.StatusConflict, "ALREADY_RESOLVED", err.Error())
	case errors.Is(err, services.ErrRequestAlreadyHeld):
		jsonErrorResponse(w, http.StatusConflict, "ALREADY_HELD", "They already have this, so approving would change nothing. Decline it to close the request.")
	case errors.Is(err, services.ErrExtensionShortens):
		jsonErrorResponse(w, http.StatusConflict, "ALREADY_RUNS_PAST", "They already have this past the end this asks for, so approving would shorten it. Decline it to close the request.")
	case errors.Is(err, services.ErrAskEnded):
		jsonErrorResponse(w, http.StatusConflict, "END_PASSED", "The end date this asked for has already passed, so approving would give access that has already ended. Decline it, and they can ask again.")
	default:
		// Stale, a role Zitadel no longer offers, the catalogue unread, a
		// departed account: the planner's refusals, said as every act says them.
		writeActError(w, err)
	}
}

var (
	// errRoleNotOffered: the role isn't in Zitadel's catalogue (removed since,
	// or never there). Nothing is given; a grant of it would fail at send with
	// the request already approved.
	errRoleNotOffered = errors.New("this role no longer exists in Zitadel")
	// errCatalogueUnread: the catalogue couldn't be read, or Syndra is serving
	// the demo one while Zitadel is configured. Fail closed.
	errCatalogueUnread = errors.New("couldn't read Zitadel's list of roles")
)

// checkRoleOffered refuses a role the directory doesn't offer, before
// anything is asked for, given or approved. The API accepted any project and
// key; only the pickers kept to the catalogue. A var so handler tests about
// something else can open it, as TestMain does for the project gate.
var checkRoleOffered = roleOfferedInDirectory

func roleOfferedInDirectory(ctx context.Context, projectID, roleKey string) error {
	projects, err := offeredCatalogue(ctx)
	if err != nil {
		return err
	}
	for _, p := range projects {
		if p.ID != projectID {
			continue
		}
		for _, role := range p.Roles {
			if role.Key == roleKey {
				return nil
			}
		}
	}
	return errRoleNotOffered
}

// offeredCatalogue is Zitadel's catalogue as the role-offered guard reads it,
// or errCatalogueUnread: unreadable, or the demo one served while Zitadel is
// configured. The planner reads the same (services.PlannerCatalogue), so a
// bulk bundle give and a single give refuse the same roles.
func offeredCatalogue(ctx context.Context) ([]models.ProjectCatalog, error) {
	if systemZitadelConfigured() && directorySource().Tag() != "zitadel" {
		return nil, errCatalogueUnread
	}
	projects, err := directoryProjects(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errCatalogueUnread, err)
	}
	return projects, nil
}

func init() {
	services.PlannerCatalogue = func(ctx context.Context) ([]models.ProjectCatalog, error) { return offeredCatalogue(ctx) }
}

// writeRoleOfferedError says why, and that nothing changed.
func writeRoleOfferedError(w http.ResponseWriter, err error) {
	if errors.Is(err, errRoleNotOffered) {
		jsonErrorResponse(w, http.StatusConflict, "ROLE_NOT_OFFERED",
			"This role no longer exists in Zitadel. Nothing was changed.")
		return
	}
	jsonErrorResponse(w, http.StatusServiceUnavailable, "CATALOGUE_UNREAD",
		"Syndra couldn't read Zitadel's list of roles to check this one exists, so nothing was changed. Try again shortly.")
}

// decisionResult is what deciding a request did: for an approval, what it
// gave (given, extended, given_again), until when, and why it isn't in
// Zitadel yet when it isn't.
type decisionResult struct {
	WaitReason string
	// Retry is the next try and which try, when Zitadel didn't take it.
	Retry  *extcall.RetryState
	Effect services.ApprovalEffect
	Until  *time.Time
	// RuleWaiting counts the roles an automatic rule gave with the approval
	// that wait for the operator to send (re-audit R12).
	RuleWaiting int
}

func resolveOneAccessRequest(ctx context.Context, requestID, status, reviewer, reviewNote string) error {
	_, err := resolveAccessRequestOutcome(ctx, requestID, status, reviewer, reviewNote, nil)
	return err
}

// resolveAccessRequestOutcome decides the request (services.DecideRequest, the
// one decision the single and the bulk paths share) and, for an approval,
// makes it effective and sends what the plan says goes now: the compiled
// cache is rebuilt first, on a context detached from the request, so access
// works whatever the send does; then DeliverChange, targeted to this
// approval's rows only. It says why the access isn't in Zitadel yet when it
// isn't, so the outcome can say "This role always waits for you" or "Zitadel
// didn't answer".
func resolveAccessRequestOutcome(ctx context.Context, requestID, status, reviewer, reviewNote string, seen *services.Seen) (decisionResult, error) {
	decided, err := svcDecideRequest(ctx, requestID, status, reviewer, reviewNote, seen)
	if err != nil || status != "approved" {
		return decisionResult{}, err
	}
	rebuildUserCacheDetached(ctx, decided.UserID)
	res := decisionResult{Effect: decided.Approval.Effect, Until: decided.Approval.ExpiresAt}
	queued := actResult(decided.Change)
	if ids := queued.OutboxIDs(); len(ids) > 0 {
		d := svcDeliverChange(ctx, decided.Change)
		own := ""
		if slices.Contains(d.Sent, queued.OutboxID) {
			own, _ = dbGetPropagationStatus(ctx, queued.OutboxID)
		}
		res.WaitReason, res.Retry, res.RuleWaiting = ownWait(d, queued.OutboxID, own)
	}
	return res, nil
}

func handleGetGovernanceSummary(w http.ResponseWriter, r *http.Request) {
	summary, err := services.Governance(r.Context())
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	// Home's queue decides requests too, so it says what approving would change.
	summary.PendingRequests = withHolding(r.Context(), summary.PendingRequests)
	jsonResponse(w, http.StatusOK, summary)
}

// cacheRebuildDetachTimeout bounds the detached compiled-cache rebuild so a
// wedged directory/cache call can't leak a goroutine-blocking request.
const cacheRebuildDetachTimeout = 10 * time.Second

// rebuildUserCacheDetached rebuilds the user's compiled claims on a context
// DETACHED from the request lifecycle (client cancellation), bounded by
// cacheRebuildDetachTimeout. Once a grant's ledger row is committed, the rebuild
// that makes access effective must complete regardless of whether the client is
// still connected — otherwise "effective immediately after commit" would hold
// only for clients that wait for the response. It stays synchronous so the
// handler still completes the rebuild before responding; it just no longer
// inherits the request's cancellation.
func rebuildUserCacheDetached(ctx context.Context, userID string) {
	detached, cancel := context.WithTimeout(context.WithoutCancel(ctx), cacheRebuildDetachTimeout)
	defer cancel()
	rebuildUserCacheOrSkip(detached, userID)
}

// rebuildUserCacheOrSkip pulls the project scope from the directory and
// rebuilds the user's compiled claims. On directory failure it logs and
// skips the rebuild, leaving previously compiled claims in place — see
// cacheRebuildProjectIDs for the rationale.
func rebuildUserCacheOrSkip(ctx context.Context, userID string) {
	projectIDs, err := cacheRebuildProjectIDs(ctx)
	if err != nil {
		log.Printf("[ACCESS] Skipping cache rebuild for user %s: directory lookup failed: %v", userID, err)
		return
	}
	refreshOrLog(ctx, userID, projectIDs...)
}

// refreshOrLog recompiles one person's claims after a write whose caller has
// nothing to do about a failure but say so: the 12-hour warm is the backstop.
func refreshOrLog(ctx context.Context, userID string, projectIDs ...string) {
	if err := cacheRefresh(ctx, []string{userID}, projectIDs...); err != nil {
		log.Printf("[CACHE] refresh failed for %s: %v", userID, err)
	}
}

// cacheRebuildProjectIDs returns the set of project IDs the cache compiler
// should rebuild for. Pulled from directory.Projects (live Zitadel or demo
// fallback) — NOT directory.Applications, because Applications can return a
// partial catalog when a per-project ListApplications call fails.
//
// Returns an error on directory failure so the caller can skip the rebuild:
// the user's compiled claims stay as they were, which is safer than
// recompiling against a list that Zitadel only half answered.
func cacheRebuildProjectIDs(ctx context.Context) ([]string, error) {
	projects, err := directory.Default.Projects(ctx)
	if err != nil {
		return nil, err
	}
	projectIDs := make([]string, 0, len(projects))
	for _, p := range projects {
		projectIDs = append(projectIDs, p.ID)
	}
	return projectIDs, nil
}
