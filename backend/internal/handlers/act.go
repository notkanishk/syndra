package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"syndra/internal/db"
	"syndra/internal/services"
)

// Single acts through the planner (architecture deepening, task 1.2; A-21
// §Q2). A sheet asks for the act's Change before anything is written
// (GET /users/{id}/plan), says before the press what it cancels, and the write
// carries the fingerprint and the cancel ids it was shown. The write is
// applied only if a fresh plan under the lock is that Change, and admits only
// those cancellations; otherwise it answers 409 PLAN_STALE and writes nothing.
// The cancel gate's 409 WOULD_CANCEL_QUEUED stays as the second safety.

var (
	svcPlanAct       = services.PlanAct
	svcApplyAct      = services.ApplyAct
	svcDecideRequest = services.DecideRequest
)

// ActPlan is one act's Change as a sheet reads it.
type ActPlan struct {
	// Effect is apply, no_change or blocked; Refusal says why it doesn't go
	// ahead ("held", "not_offered:…", "departed:…", …).
	Effect  string `json:"effect"`
	Refusal string `json:"refusal,omitempty"`
	// Detail and Said are the planner's words for the row; Consequence is
	// a refusal's step. Rules are the rule sentences on their own (A-21 §4),
	// already inside Said.
	Detail      string   `json:"detail"`
	Said        []string `json:"said"`
	Consequence string   `json:"consequence,omitempty"`
	Rules       []string `json:"rules"`
	// Delivery is straight_away, waits_for_you or none (nothing is sent);
	// Waits names the roles that wait for you, Goes the ones sent straight away.
	Delivery string   `json:"delivery,omitempty"`
	Waits    []string `json:"waits"`
	Goes     []string `json:"goes"`
	// Cancels are the queued changes it cancels, as the cancel step reads them.
	Cancels []WouldCancelChange `json:"cancels"`
	// Fingerprint and CancelIDs go back with the write.
	Fingerprint string   `json:"fingerprint"`
	CancelIDs   []string `json:"cancel_ids"`
}

// planFields is what a write carries of the plan it was shown.
type planFields struct {
	Fingerprint string   `json:"fingerprint,omitempty"`
	CancelIDs   []string `json:"cancel_ids,omitempty"`
}

func (f planFields) seen() *services.Seen {
	if f.Fingerprint == "" {
		return nil
	}
	return &services.Seen{Fingerprint: f.Fingerprint, CancelIDs: f.CancelIDs}
}

// seenFromQuery is planFields for a DELETE: ?fingerprint=&cancel_ids=a,b.
func seenFromQuery(r *http.Request) *services.Seen {
	q := r.URL.Query()
	f := planFields{Fingerprint: q.Get("fingerprint")}
	if ids := q.Get("cancel_ids"); ids != "" {
		f.CancelIDs = parseCancelIDs(ids)
	}
	return f.seen()
}

// handleActPlan is the preview: GET /users/{id}/plan?op=…, with project_id and
// role_key (give), grant_id (remove), request_id (approve) or bundle_id
// (assign_bundle, remove_bundle). Nothing is written.
func handleActPlan(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	a := services.Action{
		Op: q.Get("op"), UserID: r.PathValue("id"), BundleID: strings.TrimSpace(q.Get("bundle_id")),
		ProjectID: strings.TrimSpace(q.Get("project_id")), RoleKey: strings.TrimSpace(q.Get("role_key")),
		GrantID: strings.TrimSpace(q.Get("grant_id")), RequestID: strings.TrimSpace(q.Get("request_id")),
		DriftID: strings.TrimSpace(q.Get("drift_id")),
	}
	if field, ok := missingActField(a); !ok {
		jsonValidationErrorResponse(w, field+" is required for this op", map[string]string{field: "required"})
		return
	}
	if days := q.Get("duration_days"); days != "" {
		n, err := strconv.Atoi(days)
		if err != nil || n < 0 {
			jsonValidationErrorResponse(w, "duration_days must be zero or greater", map[string]string{"duration_days": "min=0"})
			return
		}
		a.Days = n
	}
	ch, err := svcPlanAct(r.Context(), a)
	switch {
	case errors.Is(err, services.ErrRequestNotFound):
		jsonErrorResponse(w, http.StatusNotFound, "NOT_FOUND", "No such request. It may have been withdrawn.")
		return
	case err != nil:
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, actPlanOf(r, ch))
}

// missingActField names what an op needs and the query left out.
func missingActField(a services.Action) (string, bool) {
	switch a.Op {
	case services.OpGive, services.OpExtend:
		if a.ProjectID == "" {
			return "project_id", false
		}
		if a.RoleKey == "" {
			return "role_key", false
		}
	case services.OpRemoveGrant:
		if a.GrantID == "" {
			return "grant_id", false
		}
	case services.OpApprove:
		if a.RequestID == "" {
			return "request_id", false
		}
	case services.BulkOpAssignBundle, services.BulkOpRemoveBundle:
		if a.BundleID == "" {
			return "bundle_id", false
		}
	case services.OpAdopt:
		if a.DriftID == "" {
			return "drift_id", false
		}
	default:
		return "op", false
	}
	return "", true
}

func actPlanOf(r *http.Request, ch services.Change) ActPlan {
	p := ActPlan{
		Effect: ch.Row.Effect, Refusal: ch.Refusal, Detail: ch.Row.Detail, Said: ch.Said,
		Rules: ch.Rules, Delivery: ch.Delivery(), Waits: ch.Waits, Goes: ch.Goes,
		Cancels: nameCancels(r.Context(), ch.Cancels, nil), Fingerprint: ch.Row.Fingerprint, CancelIDs: ch.Row.CancelIDs,
	}
	if ch.Row.Effect != services.EffectApply {
		p.Consequence = ch.Row.Consequence
	}
	// Lists, never null: the sheet reads them without asking.
	for _, s := range []*[]string{&p.Said, &p.Rules, &p.Waits, &p.Goes, &p.CancelIDs} {
		if *s == nil {
			*s = []string{}
		}
	}
	return p
}

// writeActError answers a planned act that wrote nothing: stale, refused, or
// a failure. The act's own refusals (a held bundle, a missing grant) are its
// handler's to say first.
func writeActError(w http.ResponseWriter, err error) {
	var stale *services.StalePlan
	var refused *services.Refused
	switch {
	case errors.As(err, &stale):
		jsonErrorResponse(w, http.StatusConflict, "PLAN_STALE",
			"Nothing was changed. Their access moved while this was open, so read it again before you go ahead.")
	case errors.As(err, &refused):
		detail := refused.Change.Row.Detail
		if step := refused.Change.Row.Consequence; step != "" {
			detail += " " + step
		}
		switch refused.Code() {
		case "catalogue_unread":
			writeRoleOfferedError(w, errCatalogueUnread)
		case "not_offered":
			jsonErrorResponse(w, http.StatusConflict, "ROLE_NOT_OFFERED", detail+" Nothing was changed.")
		case "held":
			jsonErrorResponse(w, http.StatusConflict, "ALREADY_HELD", detail+" Nothing was changed.")
		case "departed":
			jsonErrorResponse(w, http.StatusConflict, "DEPARTED", detail+" Nothing was changed.")
		case "retired":
			jsonErrorResponse(w, http.StatusConflict, "RETIRED", detail+" Nothing was changed.")
		default:
			jsonErrorResponse(w, http.StatusConflict, "REFUSED", detail+" Nothing was changed.")
		}
	case errors.Is(err, db.ErrGrantNotFound):
		jsonErrorResponse(w, http.StatusNotFound, "GRANT_NOT_FOUND",
			"No direct grant with that id belongs to this user. It may already have been removed or expired.")
	default:
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
	}
}

// refusedAs is err's refusal code when the planner refused the act.
func refusedAs(err error) string {
	var refused *services.Refused
	if errors.As(err, &refused) {
		return refused.Code()
	}
	return ""
}

// actResult is a give's or an approval's answer in the shape the sheets read
// (db.EnqueueResult): its own row, then the rules'.
func actResult(ch services.Change) db.EnqueueResult {
	var res db.EnqueueResult
	for i, w := range ch.Writes {
		switch {
		case w.OutboxID == "":
		case i == 0:
			res.OutboxID = w.OutboxID
		default:
			res.RuleOutboxIDs = append(res.RuleOutboxIDs, w.OutboxID)
		}
	}
	res.Status = "applied"
	if res.OutboxID != "" {
		res.Status = "pending"
	}
	return res
}

// deliverAct sends what the Change says goes now and reads back the act's own
// row: its status and why it isn't in Zitadel when it isn't.
func deliverAct(r *http.Request, ch services.Change, res *db.EnqueueResult) {
	d := svcDeliverChange(r.Context(), ch)
	if res.OutboxID != "" && slices.Contains(d.Sent, res.OutboxID) {
		if st, err := dbGetPropagationStatus(r.Context(), res.OutboxID); err == nil && st != "" {
			res.Status = st
		}
	}
	if len(res.OutboxIDs()) > 0 {
		res.WaitReason, res.Retry, res.RuleWaiting = ownWait(d, res.OutboxID, res.Status)
	}
}

// givePlanned gives one role through the planner (task 1.2). The ledger row,
// its own outbox row and the rules' rows are written as the plan decided,
// then sent as it decided (DeliverChange).
func givePlanned(w http.ResponseWriter, r *http.Request, userID, actor string, expiresAt *time.Time, req UpsertDirectGrantRequest) {
	payload, _ := json.Marshal(req)
	ch, err := svcApplyAct(r.Context(),
		services.Action{Op: services.OpGive, UserID: userID, ProjectID: req.ProjectID, RoleKey: req.RoleKey},
		services.ActWrite{Actor: actor, Reason: req.Reason,
			Give: db.EnqueueParams{ExpiresAt: expiresAt, EndRule: db.EndKept, PayloadJSON: string(payload)}},
		req.seen())
	if refusedAs(err) == "held" {
		// What they hold, in the words the request queue uses.
		if holding, herr := dbRequestHolding(r.Context(), userID, req.ProjectID, req.RoleKey); herr == nil {
			jsonErrorResponse(w, http.StatusConflict, "ALREADY_HELD", alreadyHeldMessage(holding, "They"))
			return
		}
	}
	if err != nil {
		writeActError(w, err)
		return
	}
	// Effective from the ledger at once, before (and whatever) the send.
	rebuildUserCacheDetached(r.Context(), userID)
	res := actResult(ch)
	deliverAct(r, ch, &res)
	jsonResponse(w, http.StatusAccepted, res)
}

// removeGrantPlanned revokes one direct grant through the planner: the
// closure delta as the plan decided it (a role still held another way is
// never revoked; a give still waiting is cancelled, and nothing is sent).
func removeGrantPlanned(w http.ResponseWriter, r *http.Request, userID, grantID string) {
	ch, err := svcApplyAct(r.Context(), services.Action{Op: services.OpRemoveGrant, UserID: userID, GrantID: grantID},
		services.ActWrite{Actor: resolveActor(r, "")}, seenFromQuery(r))
	if refusedAs(err) == "not_held" {
		err = db.ErrGrantNotFound
	}
	if err != nil {
		writeActError(w, err)
		return
	}
	// Recompile before returning, on a context detached from the request, so
	// the access is gone from the token path even if the caller disconnects.
	rebuildUserCacheDetachedFn(r.Context(), userID)

	res := services.DirectGrantRemoval{Status: "pending", Revoked: []string{}, Retained: []string{}}
	revokesOwn := false
	for _, wr := range ch.Writes {
		if wr.OpType != "revoke" {
			continue
		}
		res.Revoked = append(res.Revoked, wr.ProjectID+"/"+wr.RoleKeys[0])
		revokesOwn = revokesOwn || (wr.ProjectID == ch.ProjectID && wr.RoleKeys[0] == ch.RoleKey)
		if wr.OutboxID != "" {
			res.OutboxIDs = append(res.OutboxIDs, wr.OutboxID)
		}
	}
	if !revokesOwn {
		res.Retained = append(res.Retained, ch.ProjectID+"/"+ch.RoleKey)
	}
	d := svcDeliverChange(r.Context(), ch)
	res.WaitReason, res.Retry = d.WaitReason(), d.Drain.Retry
	for _, id := range d.Sent {
		if st, serr := dbGetPropagationStatus(r.Context(), id); serr == nil && st != "" {
			res.Status = st
		}
	}
	jsonResponse(w, http.StatusAccepted, res)
}

// bundlePlanned assigns a bundle to one person, or removes it, through the
// planner: the same Change a bulk row of the same act would be.
func bundlePlanned(w http.ResponseWriter, r *http.Request, op, userID, bundleID string, seen *services.Seen) {
	actor := getAdminUserID(r.Context())
	if actor == "" {
		actor = "system"
	}
	ch, err := svcApplyAct(r.Context(), services.Action{Op: op, UserID: userID, BundleID: bundleID},
		services.ActWrite{Actor: actor}, seen)
	if code := refusedAs(err); code == "held" || code == "not_held" {
		// Idempotent, and it says which of the two happened. "Bundle
		// assigned" after a no-op reads as a change that was made.
		message := "Already holds this bundle, so nothing changed."
		if code == "not_held" {
			message = "Doesn't hold this bundle, so nothing changed."
		}
		jsonResponse(w, http.StatusOK, map[string]any{"message": message, "cascade": services.CascadeResult{NoOp: true}})
		return
	}
	if err != nil {
		writeActError(w, err)
		return
	}
	rebuildUserCacheDetachedFn(r.Context(), userID)
	d := svcDeliverChange(r.Context(), ch)
	cascade := services.CascadeResult{Drain: d.Drain, Waiting: len(d.Waiting), WaitReason: d.WaitReason()}
	for _, wr := range ch.Writes {
		if wr.OutboxID != "" {
			cascade.Enqueued++
		}
	}
	message := "Bundle assigned to user"
	if op == services.BulkOpRemoveBundle {
		message = "Bundle removed from user"
	}
	jsonResponse(w, http.StatusOK, map[string]any{"message": message, "cascade": cascade})
}
