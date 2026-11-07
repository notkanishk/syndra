package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"syndra/internal/auth"
	"syndra/internal/clock"
	"syndra/internal/db"
	"syndra/internal/models"
	"syndra/internal/services"
	"syndra/internal/services/propagation"
)

// Approving a request is services.DecideRequest (the planner's approval, the
// audit row, the ledger and outbox rows: services' tests). What the handler owns
// is what it hands the decision, the order after it (the compiled cache is
// rebuilt first, then what the plan says goes now is sent) and what it answers.
func TestHandleResolveAccessRequestApprovedRebuildsCacheThenSendsAndSaysWhatHappened(t *testing.T) {
	resetAccessDeps(t)
	until := time.Now().Add(5 * 24 * time.Hour)
	probe := stubDecide(t)
	probe.Decision = services.Decision{
		UserID:   "u1",
		Change:   services.Change{Writes: []services.Write{{OutboxID: "ob-appr"}, {OutboxID: "ob-rule"}}},
		Approval: services.ApprovalPlan{Effect: services.EffectGive, ExpiresAt: &until},
	}
	// The approval's own row went and applied; the rule's row waits for Send.
	probe.Delivery = services.Delivery{Sent: []string{"ob-appr"}, Waiting: []string{"ob-rule"},
		Reason: map[string]string{"ob-rule": "role_waits"}}
	dbGetPropagationStatus = func(_ context.Context, id string) (string, error) {
		if id != "ob-appr" {
			t.Fatalf("the status read is this approval's own row, got %q", id)
		}
		return "applied", nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/requests/req-1/decision", strings.NewReader(`{"status":"approved","reviewer_id":"reviewer-1","review_note":"ok"}`))
	req.SetPathValue("id", "req-1")
	rr := httptest.NewRecorder()
	handleResolveAccessRequest(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if len(probe.Calls) != 1 || probe.Calls[0] != (decideCall{"req-1", "approved", "reviewer-1", "ok", nil}) {
		t.Fatalf("decision called with %+v", probe.Calls)
	}
	probe.wantOrder(t, "decide", "rebuild", "deliver")
	if len(probe.RebuildUsers) != 1 || probe.RebuildUsers[0] != "u1" {
		t.Fatalf("expected cache rebuild for the requester u1, got %v", probe.RebuildUsers)
	}
	if len(probe.Delivered) != 1 || len(probe.Delivered[0].Writes) != 2 {
		t.Fatalf("what is sent is the decision's Change: %+v", probe.Delivered)
	}
	body := decodeMap(t, rr.Body.String())
	if body["effect"] != string(services.EffectGive) || body["until"] == nil {
		t.Fatalf("an approval says what it did and until when: %v", body)
	}
	if _, has := body["wait_reason"]; has {
		t.Fatalf("its own row applied, so the rule's wait is not blamed on it: %v", body)
	}
	if body["rule_waiting"] != float64(1) {
		t.Fatalf("a rule's row waits for Send and the answer says so: %v", body)
	}
}

// An approval whose own row waits says why.
func TestHandleResolveAccessRequest_AnApprovalThatWaitsSaysWhy(t *testing.T) {
	resetAccessDeps(t)
	probe := stubDecide(t)
	probe.Decision = services.Decision{UserID: "u1",
		Change:   services.Change{Writes: []services.Write{{OutboxID: "ob-appr", Delivery: services.DeliveryWaits}}},
		Approval: services.ApprovalPlan{Effect: services.EffectGive}}
	probe.Delivery = services.Delivery{Waiting: []string{"ob-appr"}, Reason: map[string]string{"ob-appr": "role_waits"}}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/requests/req-1/decision", strings.NewReader(`{"status":"approved","reviewer_id":"reviewer-1"}`))
	req.SetPathValue("id", "req-1")
	rr := httptest.NewRecorder()
	handleResolveAccessRequest(rr, req)

	body := decodeMap(t, rr.Body.String())
	if rr.Code != http.StatusOK || body["wait_reason"] != "role_waits" {
		t.Fatalf("want 200 with wait_reason=role_waits, got %d %v", rr.Code, body)
	}
	if _, has := body["until"]; has {
		t.Fatalf("no end, no until: %v", body)
	}
}

// The plan the row was shown rides in the body and becomes the Seen; the decision
// refuses a stale one.
func TestHandleResolveAccessRequest_ApprovalCarriesWhatTheRowWasShown(t *testing.T) {
	resetAccessDeps(t)
	probe := stubDecide(t)
	const id = "33333333-3333-3333-3333-333333333333"

	req := httptest.NewRequest(http.MethodPost, "/api/v1/requests/req-1/decision",
		strings.NewReader(`{"status":"approved","reviewer_id":"reviewer-1","fingerprint":"fp-7","cancel_ids":["`+id+`"]}`))
	req.SetPathValue("id", "req-1")
	handleResolveAccessRequest(httptest.NewRecorder(), req)

	if len(probe.Calls) != 1 || probe.Calls[0].Seen == nil || probe.Calls[0].Seen.Fingerprint != "fp-7" ||
		len(probe.Calls[0].Seen.CancelIDs) != 1 || probe.Calls[0].Seen.CancelIDs[0] != id {
		t.Fatalf("seen = %+v", probe.Calls)
	}
}

// What a decision refuses with, and what the handler says for each: 409 for a lost race, a
// request already decided or withdrawn, a role they hold or that already runs past the ask,
// an end that passed, a stale plan; the planner's refusals as every act says them.
func TestHandleResolveAccessRequest_RefusalsAreSaidWithTheirCodes(t *testing.T) {
	resetAccessDeps(t)
	probe := stubDecide(t)
	refused := func(code string) error { return &services.Refused{Change: services.Change{Refusal: code}} }
	cases := []struct {
		name, status string
		err          error
		code         int
		want         string
	}{
		{"already approved", "approved", fmt.Errorf("%w (already approved)", services.ErrRequestAlreadyResolved), 409, "ALREADY_RESOLVED"},
		{"lost the approve race", "approved", services.ErrRequestAlreadyResolved, 409, "ALREADY_RESOLVED"},
		{"lost the reject race", "rejected", services.ErrRequestAlreadyResolved, 409, "ALREADY_RESOLVED"},
		{"held", "approved", services.ErrRequestAlreadyHeld, 409, "ALREADY_HELD"},
		{"runs past", "approved", services.ErrExtensionShortens, 409, "ALREADY_RUNS_PAST"},
		{"ask ended", "approved", services.ErrAskEnded, 409, "END_PASSED"},
		{"stale", "approved", &services.StalePlan{IDs: []string{"q"}}, 409, "PLAN_STALE"},
		{"not offered", "approved", refused("not_offered:p/r"), 409, "ROLE_NOT_OFFERED"},
		{"catalogue unread", "approved", refused("catalogue_unread"), 503, "CATALOGUE_UNREAD"},
		{"departed", "approved", refused("departed:Departed"), 409, "DEPARTED"},
		{"no such request", "approved", fmt.Errorf("%w: no rows", services.ErrRequestNotFound), 404, "NOT_FOUND"},
		{"failure", "approved", errors.New("tx failed"), 500, "DB_ERROR"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			probe.Order, probe.DecideErr = nil, c.err
			req := httptest.NewRequest(http.MethodPost, "/api/v1/requests/req-9/decision",
				strings.NewReader(`{"status":"`+c.status+`","reviewer_id":"reviewer-1"}`))
			req.SetPathValue("id", "req-9")
			rr := httptest.NewRecorder()
			handleResolveAccessRequest(rr, req)
			wantError(t, rr, c.code, c.want)
			probe.wantOrder(t, "decide") // nothing rebuilt, nothing sent
		})
	}
}

// A request that was withdrawn names the state it is in.
func TestHandleResolveAccessRequest_TheConflictNamesTheState(t *testing.T) {
	resetAccessDeps(t)
	stubDecide(t).DecideErr = fmt.Errorf("%w (already withdrawn)", services.ErrRequestAlreadyResolved)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/requests/r1/decision",
		strings.NewReader(`{"status":"rejected","reviewer_id":"reviewer-1","review_note":"no"}`))
	req.SetPathValue("id", "r1")
	rr := httptest.NewRecorder()
	handleResolveAccessRequest(rr, req)

	if rr.Code != http.StatusConflict || !strings.Contains(rr.Body.String(), "already withdrawn") {
		t.Fatalf("the conflict must name the state it is in: %d %s", rr.Code, rr.Body.String())
	}
}

// Declining is the decision alone: no cache rebuild, nothing sent.
func TestHandleResolveAccessRequestRejectedChangesNoAccess(t *testing.T) {
	resetAccessDeps(t)
	probe := stubDecide(t)
	probe.Decision = services.Decision{UserID: "u1"}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/requests/req-2/decision", strings.NewReader(`{"status":"rejected","reviewer_id":"reviewer-2","review_note":"deny"}`))
	req.SetPathValue("id", "req-2")
	rr := httptest.NewRecorder()
	handleResolveAccessRequest(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if len(probe.Calls) != 1 || probe.Calls[0] != (decideCall{"req-2", "rejected", "reviewer-2", "deny", nil}) {
		t.Fatalf("decision called with %+v", probe.Calls)
	}
	probe.wantOrder(t, "decide")
	if body := decodeMap(t, rr.Body.String()); body["effect"] != nil || body["until"] != nil {
		t.Fatalf("a decline gives nothing, so it says no effect: %v", body)
	}
}

// Approving needs a named reviewer: the one account that is nobody is "system".
func TestHandleResolveAccessRequest_ApprovingNeedsAReviewer(t *testing.T) {
	resetAccessDeps(t)
	probe := stubDecide(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/requests/req-2/decision", strings.NewReader(`{"status":"approved"}`))
	req.SetPathValue("id", "req-2")
	rr := httptest.NewRecorder()
	handleResolveAccessRequest(rr, req)

	wantError(t, rr, http.StatusBadRequest, "VALIDATION_FAILED")
	if len(probe.Calls) != 0 {
		t.Fatal("nothing is decided without a reviewer")
	}
}

// The grant is durable once the decision commits, so the compiled-cache rebuild that makes
// access effective MUST run even if the client disconnects before it: on a context detached
// from the request lifecycle.
func TestHandleResolveAccessRequest_RebuildDetachedFromClientCancel(t *testing.T) {
	resetAccessDeps(t)
	probe := stubDecide(t)
	probe.Decision = services.Decision{UserID: "u1", Change: services.Change{Writes: []services.Write{{OutboxID: "ob-appr"}}}}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // client already disconnected
	req := httptest.NewRequest(http.MethodPost, "/api/v1/requests/req-1/decision",
		strings.NewReader(`{"status":"approved","reviewer_id":"reviewer-1"}`)).WithContext(ctx)
	req.SetPathValue("id", "req-1")
	handleResolveAccessRequest(httptest.NewRecorder(), req)

	if len(probe.RebuildUsers) != 1 {
		t.Fatal("cache rebuild must run even after the client disconnected (the grant is already durable)")
	}
	if !probe.RebuildDetached {
		t.Fatal("cache rebuild must run on a context detached from client cancellation")
	}
}

// resetAccessDeps captures and restores all access-related injectable vars.
func resetAccessDeps(t *testing.T) {
	t.Helper()
	origGetByID := dbGetAccessRequestByID
	origResolve := dbResolveAccessRequest
	origWithdraw := dbWithdrawAccessRequest
	origUpsert := dbUpsertDirectGrant
	origCreate := dbCreateAccessRequestOfKind
	origAudit := dbInsertAuditLog
	origRebuild := cacheRebuildUser
	origEnqueue := dbEnqueueDirectGrantPropagation
	origApprove := dbApproveRequestAndEnqueue
	origDrain := svcDrainPropagations
	origDrainRow := svcDrainPropagationRow
	origStatus := dbGetPropagationStatus
	origHolding := dbRequestHolding
	// Nobody holds anything unless a test says so.
	dbRequestHolding = func(context.Context, string, string, string) (models.RequestHolding, error) {
		return models.RequestHolding{Kind: models.HoldingNone}, nil
	}
	// A sent row is applied unless a test says otherwise.
	dbGetPropagationStatus = func(context.Context, string) (string, error) { return "applied", nil }
	t.Cleanup(func() {
		dbRequestHolding = origHolding
		dbGetAccessRequestByID = origGetByID
		dbResolveAccessRequest = origResolve
		dbWithdrawAccessRequest = origWithdraw
		dbUpsertDirectGrant = origUpsert
		dbCreateAccessRequestOfKind = origCreate
		dbInsertAuditLog = origAudit
		cacheRebuildUser = origRebuild
		dbEnqueueDirectGrantPropagation = origEnqueue
		dbApproveRequestAndEnqueue = origApprove
		svcDrainPropagations = origDrain
		svcDrainPropagationRow = origDrainRow
		dbGetPropagationStatus = origStatus
	})
}

// memberContext returns a request carrying a production-mode member principal
// (no admin role), as withUserAuth would stash it. Callers must also Setenv
// ZITADEL_DOMAIN so isOperator takes the role-check path instead of the
// dev-mode pass-through.
func memberContext(req *http.Request, subject string) *http.Request {
	return req.WithContext(withPrincipal(req.Context(), &auth.Principal{
		Subject:      subject,
		ProjectRoles: map[string]struct{}{"member": {}},
	}))
}

// SC3 regression: a member listing requests sees only their own — the
// org-wide list (with justifications) is operator-scoped.
func TestHandleGetAccessRequests_MemberSeesOnlyOwn(t *testing.T) {
	t.Setenv("ZITADEL_DOMAIN", "example.zitadel.cloud")
	origGet := dbGetAccessRequests
	t.Cleanup(func() { dbGetAccessRequests = origGet })
	dbGetAccessRequests = func(context.Context, string) ([]models.AccessRequest, error) {
		return []models.AccessRequest{
			{ID: "r-own", RequesterID: "member-1"},
			{ID: "r-other", RequesterID: "someone-else"},
		}, nil
	}

	req := memberContext(httptest.NewRequest(http.MethodGet, "/api/v1/requests", nil), "member-1")
	rr := httptest.NewRecorder()
	handleGetAccessRequests(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200; got %d: %s", rr.Code, rr.Body.String())
	}
	var got []models.AccessRequest
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got) != 1 || got[0].ID != "r-own" {
		t.Fatalf("expected only the member's own request; got %+v", got)
	}
}

// SC8 regression: a member cannot file a request impersonating another user —
// the authenticated subject overrides the client-supplied requester_id.
func TestHandleCreateAccessRequest_BindsRequesterToPrincipal(t *testing.T) {
	resetAccessDeps(t)
	t.Setenv("ZITADEL_DOMAIN", "example.zitadel.cloud")

	var gotRequester string
	dbCreateAccessRequestOfKind = func(_ context.Context, _ string, requesterID, _, _, _ string, _ *int, _ *time.Time) (string, error) {
		gotRequester = requesterID
		return "req-1", nil
	}
	dbInsertAuditLog = func(context.Context, string, string, string, string) error { return nil }

	body := `{"requester_id":"victim-1","project_id":"p1","role_key":"maker","justification":"3d printing"}`
	req := memberContext(httptest.NewRequest(http.MethodPost, "/api/v1/requests", strings.NewReader(body)), "member-1")
	rr := httptest.NewRecorder()
	handleCreateAccessRequest(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201; got %d: %s", rr.Code, rr.Body.String())
	}
	if gotRequester != "member-1" {
		t.Fatalf("spoofed requester_id must be overridden by the principal; persisted requester=%q", gotRequester)
	}
}

// --- handleUpsertUserDirectGrant ---
//
// Contract change (Wave 2 · Part 4, B4/D3): this endpoint no longer writes the
// grant + calls Zitadel synchronously. It now enqueues a propagation through the
// durable ledger+outbox and returns 202 Accepted with {outbox_id, status}. The
// Zitadel mutation happens later during the operator-triggered drain.

func giveRole(userID, query, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/"+userID+"/grants"+query, strings.NewReader(body))
	req.SetPathValue("id", userID)
	rr := httptest.NewRecorder()
	handleUpsertUserDirectGrant(rr, req)
	return rr
}

// A give is the planner's act (services.ApplyAct writes the ledger row, its own
// outbox row and the rules' rows). The handler hands it the Action and the
// ActWrite, answers 202 with the row, and rebuilds the cache before it sends.
func TestHandleUpsertUserDirectGrant_AppliesThePlannedGiveAndReturns202(t *testing.T) {
	resetAccessDeps(t)
	probe := stubAct(t)
	probe.Change = services.Change{Writes: []services.Write{{OutboxID: "ob1"}}}

	rr := giveRole("u1", "", `{"project_id":"p1","role_key":"r1","reason":"lab access","duration_days":30}`)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("want 202, got %d: %s", rr.Code, rr.Body.String())
	}
	a, w, seen := probe.only(t)
	if a != (services.Action{Op: services.OpGive, UserID: "u1", ProjectID: "p1", RoleKey: "r1"}) {
		t.Fatalf("unexpected action: %+v", a)
	}
	if w.Actor != "system" || w.Reason != "lab access" || w.Give.EndRule != db.EndKept {
		t.Fatalf("unexpected write: %+v", w)
	}
	if !strings.Contains(w.Give.PayloadJSON, `"reason":"lab access"`) {
		t.Fatalf("the request body is kept as sent (JSONB NOT NULL): %q", w.Give.PayloadJSON)
	}
	if seen != nil {
		t.Fatalf("no fingerprint was sent, so nothing was seen: %+v", seen)
	}
	body := decodeMap(t, rr.Body.String())
	if body["outbox_id"] != "ob1" || body["status"] != "pending" {
		t.Fatalf("unexpected body: %v", body)
	}
}

func TestHandleUpsertUserDirectGrant_ExpiryCalculation(t *testing.T) {
	resetAccessDeps(t)
	probe := stubAct(t)

	rr := giveRole("u1", "", `{"project_id":"printing","role_key":"member","duration_days":7}`)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rr.Code, rr.Body.String())
	}
	_, w, _ := probe.only(t)
	capturedExpiry := w.Give.ExpiresAt
	if capturedExpiry == nil {
		t.Fatalf("expected non-nil expiresAt for DurationDays=7")
	}
	expected := clock.EndAfterDays(time.Now(), 7)
	diff := capturedExpiry.Sub(expected)
	if diff != 0 && diff != 24*time.Hour { // the day may turn mid-test
		t.Fatalf("expiresAt %v is not within 1 minute of expected %v", capturedExpiry, expected)
	}
}

func TestHandleUpsertUserDirectGrant_ZeroDuration_NoExpiry(t *testing.T) {
	resetAccessDeps(t)
	probe := stubAct(t)

	rr := giveRole("u1", "", `{"project_id":"printing","role_key":"member","duration_days":0}`)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rr.Code, rr.Body.String())
	}
	if _, w, _ := probe.only(t); w.Give.ExpiresAt != nil {
		t.Fatalf("expected nil expiresAt for DurationDays=0, got %v", w.Give.ExpiresAt)
	}
}

func TestHandleUpsertUserDirectGrant_CacheRebuilt(t *testing.T) {
	resetAccessDeps(t)
	probe := stubAct(t)

	rr := giveRole("u42", "", `{"project_id":"printing","role_key":"member"}`)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rr.Code, rr.Body.String())
	}
	if len(probe.RebuildUsers) != 1 || probe.RebuildUsers[0] != "u42" {
		t.Fatalf("expected cache rebuild for u42, got %v", probe.RebuildUsers)
	}
}

// The row the send reports as sent is the one whose status is read back: this
// request's own, whatever else went with it.
func TestHandleUpsertUserDirectGrant_SentRowReportsItsOwnStatus(t *testing.T) {
	resetAccessDeps(t)
	probe := stubAct(t)
	probe.Change = services.Change{Writes: []services.Write{{OutboxID: "ob4"}, {OutboxID: "ob-rule"}}}
	probe.Delivery = services.Delivery{Sent: []string{"ob4", "ob-rule"}, Drain: propagation.DrainResult{Applied: 2}}
	dbGetPropagationStatus = func(_ context.Context, id string) (string, error) {
		if id != "ob4" {
			t.Fatalf("the status read must be THIS request's outbox row, got %q", id)
		}
		return "applied", nil
	}

	rr := giveRole("u1", "", `{"project_id":"p1","role_key":"r1"}`)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeMap(t, rr.Body.String())
	if body["status"] != "applied" {
		t.Fatalf("must report THIS row's status=applied, got %v", body["status"])
	}
	if ids, _ := body["rule_outbox_ids"].([]any); len(ids) != 1 || ids[0] != "ob-rule" {
		t.Fatalf("the rules' rows follow the give's own: %v", body)
	}
}

// The planner's cancel-only or no-row give has no outbox row: it is applied, and sends
// nothing worth waiting on.
func TestHandleUpsertUserDirectGrant_ARowlessGiveIsApplied(t *testing.T) {
	resetAccessDeps(t)
	probe := stubAct(t)
	probe.Change = services.Change{Writes: []services.Write{{}}}

	rr := giveRole("u1", "", `{"project_id":"p1","role_key":"r1"}`)

	body := decodeMap(t, rr.Body.String())
	if rr.Code != http.StatusAccepted || body["status"] != "applied" || body["outbox_id"] != nil && body["outbox_id"] != "" {
		t.Fatalf("got %d %v", rr.Code, body)
	}
}

// The compiled cache (which the claim path reads) is rebuilt from the committed
// ledger row BEFORE the send, so a slow or canceled Zitadel call cannot starve the
// rebuild: access is effective whatever the projection does.
func TestHandleUpsertUserDirectGrant_RebuildsCacheBeforeTheSend(t *testing.T) {
	resetAccessDeps(t)
	probe := stubAct(t)
	probe.Change = services.Change{Writes: []services.Write{{OutboxID: "ob6"}}}

	giveRole("u1", "", `{"project_id":"p1","role_key":"r1"}`)

	probe.wantOrder(t, "apply", "rebuild", "deliver")
}

func TestHandleUpsertUserDirectGrant_RebuildDetachedFromClientCancel(t *testing.T) {
	resetAccessDeps(t)
	probe := stubAct(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // client already disconnected
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/u1/grants",
		strings.NewReader(`{"project_id":"p1","role_key":"r1"}`)).WithContext(ctx)
	req.SetPathValue("id", "u1")
	handleUpsertUserDirectGrant(httptest.NewRecorder(), req)

	if len(probe.RebuildUsers) != 1 {
		t.Fatal("cache rebuild must run even after the client disconnected (the grant is already durable)")
	}
	if !probe.RebuildDetached {
		t.Fatal("cache rebuild must run on a context detached from client cancellation")
	}
}

// The send's outcome for this row is said: why it isn't in Zitadel yet, and what the rules
// left waiting.
func TestHandleUpsertUserDirectGrant_SaysWhyItWaitsAndWhatTheRulesLeftWaiting(t *testing.T) {
	resetAccessDeps(t)
	probe := stubAct(t)
	probe.Change = services.Change{Writes: []services.Write{
		{OutboxID: "ob-own", Delivery: services.DeliveryWaits},
		{OutboxID: "ob-rule", Delivery: services.DeliveryWaits},
	}}
	probe.Delivery = services.Delivery{Waiting: []string{"ob-own", "ob-rule"},
		Reason: map[string]string{"ob-own": "role_waits", "ob-rule": "role_waits"}}

	body := decodeMap(t, giveRole("u1", "", `{"project_id":"p1","role_key":"r1"}`).Body.String())

	if body["wait_reason"] != "role_waits" || body["rule_waiting"] != float64(1) || body["status"] != "pending" {
		t.Fatalf("unexpected body: %v", body)
	}
}

// The plan the sheet was shown rides in the body: fingerprint and cancel_ids become
// the Seen the planner checks against a fresh plan.
func TestHandleUpsertUserDirectGrant_CarriesWhatTheSheetWasShown(t *testing.T) {
	resetAccessDeps(t)
	probe := stubAct(t)
	const id = "44444444-4444-4444-4444-444444444444"

	rr := giveRole("u1", "", `{"project_id":"p1","role_key":"r1","fingerprint":"fp-1","cancel_ids":["`+id+`"]}`)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rr.Code, rr.Body.String())
	}
	_, _, seen := probe.only(t)
	if seen == nil || seen.Fingerprint != "fp-1" || len(seen.CancelIDs) != 1 || seen.CancelIDs[0] != id {
		t.Fatalf("seen = %+v", seen)
	}
}

// A stale plan or a refusal writes nothing and answers with its code; no rebuild, no send.
func TestHandleUpsertUserDirectGrant_RefusalsAreSaidAndNothingFollows(t *testing.T) {
	resetAccessDeps(t)
	probe := stubAct(t)
	refused := func(code string) error { return &services.Refused{Change: services.Change{Refusal: code}} }
	for _, c := range []struct {
		name string
		err  error
		code int
		want string
	}{
		{"stale", &services.StalePlan{IDs: []string{"q"}}, 409, "PLAN_STALE"},
		{"not offered", refused("not_offered:p1/r1"), 409, "ROLE_NOT_OFFERED"},
		{"catalogue unread", refused("catalogue_unread"), 503, "CATALOGUE_UNREAD"},
		{"departed", refused("departed:Departed"), 409, "DEPARTED"},
		{"failure", errors.New("tx failed"), 500, "DB_ERROR"},
	} {
		t.Run(c.name, func(t *testing.T) {
			probe.Order, probe.Err = nil, c.err
			wantError(t, giveRole("u1", "", `{"project_id":"p1","role_key":"r1"}`), c.code, c.want)
			probe.wantOrder(t, "apply")
		})
	}
}

// Held says what they hold, in the words the request queue uses.
func TestHandleUpsertUserDirectGrant_HeldSaysWhatTheyHold(t *testing.T) {
	resetAccessDeps(t)
	probe := stubAct(t)
	probe.Err = &services.Refused{Change: services.Change{Refusal: "held"}}
	dbRequestHolding = func(context.Context, string, string, string) (models.RequestHolding, error) {
		return models.RequestHolding{Kind: models.HoldingForGood}, nil
	}

	rr := giveRole("u1", "", `{"project_id":"p1","role_key":"r1"}`)

	wantError(t, rr, http.StatusConflict, "ALREADY_HELD")
	if !strings.Contains(rr.Body.String(), alreadyHeldMessage(models.RequestHolding{Kind: models.HoldingForGood}, "They")) {
		t.Fatalf("held must say what they hold: %s", rr.Body.String())
	}
	probe.wantOrder(t, "apply")
}

// --- handleCreateAccessRequest ---

func TestHandleCreateAccessRequest_PersistenceAndAudit(t *testing.T) {
	resetAccessDeps(t)

	dbCreateAccessRequestOfKind = func(ctx context.Context, _ string, requesterID, projectID, roleKey, justification string, durationDays *int, _ *time.Time) (string, error) {
		return "req-1", nil
	}
	auditAction := ""
	dbInsertAuditLog = func(ctx context.Context, actorID, targetID, action, resourceID string) error {
		auditAction = action
		return nil
	}

	body := `{"requester_id":"u1","project_id":"printing","role_key":"member","justification":"need access"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/requests", strings.NewReader(body))
	rr := httptest.NewRecorder()
	handleCreateAccessRequest(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["id"] != "req-1" {
		t.Fatalf("expected id=req-1, got %v", resp["id"])
	}
	if auditAction != "access_request.created" {
		t.Fatalf("expected audit action access_request.created, got %s", auditAction)
	}
}

func TestHandleCreateAccessRequest_ZeroDuration_NilPointer(t *testing.T) {
	resetAccessDeps(t)

	var capturedDuration *int
	dbCreateAccessRequestOfKind = func(ctx context.Context, _ string, requesterID, projectID, roleKey, justification string, durationDays *int, _ *time.Time) (string, error) {
		capturedDuration = durationDays
		return "req-2", nil
	}
	dbInsertAuditLog = func(ctx context.Context, actorID, targetID, action, resourceID string) error { return nil }

	body := `{"requester_id":"u1","project_id":"printing","role_key":"member","justification":"need access","duration_days":0}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/requests", strings.NewReader(body))
	rr := httptest.NewRecorder()
	handleCreateAccessRequest(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	if capturedDuration != nil {
		t.Fatalf("expected nil durationDays for DurationDays=0, got %v", *capturedDuration)
	}
}

// --- handleWithdrawAccessRequest ---

func TestHandleWithdrawAccessRequest_RequesterMayTakeTheirOwnBack(t *testing.T) {
	t.Setenv("ZITADEL_DOMAIN", "example.zitadel.cloud")
	resetAccessDeps(t)

	dbGetAccessRequestByID = func(ctx context.Context, id string) (models.AccessRequest, error) {
		return models.AccessRequest{ID: id, RequesterID: "member-1", Status: "pending"}, nil
	}
	var gotID, gotRequester string
	dbWithdrawAccessRequest = func(ctx context.Context, id, requesterID string) error {
		gotID, gotRequester = id, requesterID
		return nil
	}
	var auditAction, auditActor string
	dbInsertAuditLog = func(ctx context.Context, actorID, targetID, action, resourceID string) error {
		auditActor, auditAction = actorID, action
		return nil
	}

	req := memberContext(httptest.NewRequest(http.MethodPost, "/api/v1/requests/r1/withdraw", nil), "member-1")
	req.SetPathValue("id", "r1")
	rr := httptest.NewRecorder()
	handleWithdrawAccessRequest(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	// The requester is passed from the row, never from the request — the UPDATE is scoped by it.
	if gotID != "r1" || gotRequester != "member-1" {
		t.Fatalf("withdraw(%q, %q), want (r1, member-1)", gotID, gotRequester)
	}
	if auditAction != "access_request.withdrawn" || auditActor != "member-1" {
		t.Fatalf("audit actor=%q action=%q", auditActor, auditAction)
	}
}

// Withdrawing is the requester's own act. An operator taking back somebody else's ask would be a
// rejection with the reviewer's name left off, so it is refused here too.
func TestHandleWithdrawAccessRequest_SomebodyElsesIs403(t *testing.T) {
	t.Setenv("ZITADEL_DOMAIN", "example.zitadel.cloud")
	resetAccessDeps(t)

	dbGetAccessRequestByID = func(ctx context.Context, id string) (models.AccessRequest, error) {
		return models.AccessRequest{ID: id, RequesterID: "someone-else", Status: "pending"}, nil
	}
	dbWithdrawAccessRequest = func(ctx context.Context, id, requesterID string) error {
		t.Fatal("must not withdraw a request filed by somebody else")
		return nil
	}

	req := memberContext(httptest.NewRequest(http.MethodPost, "/api/v1/requests/r1/withdraw", nil), "member-1")
	req.SetPathValue("id", "r1")
	rr := httptest.NewRecorder()
	handleWithdrawAccessRequest(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
}

// An operator decided it in the window between the member's screen loading and their click.
// Their copy of the row is stale; saying so beats reporting a failure.
func TestHandleWithdrawAccessRequest_AlreadyDecidedIs409(t *testing.T) {
	t.Setenv("ZITADEL_DOMAIN", "example.zitadel.cloud")
	resetAccessDeps(t)

	dbGetAccessRequestByID = func(ctx context.Context, id string) (models.AccessRequest, error) {
		return models.AccessRequest{ID: id, RequesterID: "member-1", Status: "pending"}, nil
	}
	dbWithdrawAccessRequest = func(ctx context.Context, id, requesterID string) error {
		return db.ErrRequestNotPending
	}

	req := memberContext(httptest.NewRequest(http.MethodPost, "/api/v1/requests/r1/withdraw", nil), "member-1")
	req.SetPathValue("id", "r1")
	rr := httptest.NewRecorder()
	handleWithdrawAccessRequest(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
}

// A withdrawn request is settled. Deciding one afterwards would resurrect an ask its author had
// already taken back — the guard is "not pending", not a list of decided statuses.
// Found on production, 15 Oct: a member asked five times for a role they held
// for good. Each copy reached the queue, and approving any of them would have
// cut their access to 64 days.

func TestACreateForARoleHeldForGoodIsRefused(t *testing.T) {
	resetAccessDeps(t)
	t.Setenv("ZITADEL_DOMAIN", "example.zitadel.cloud")
	dbRequestHolding = func(context.Context, string, string, string) (models.RequestHolding, error) {
		return models.RequestHolding{Kind: models.HoldingForGood}, nil
	}
	created := false
	dbCreateAccessRequestOfKind = func(context.Context, string, string, string, string, string, *int, *time.Time) (string, error) {
		created = true
		return "req-1", nil
	}
	body := `{"project_id":"p1","role_key":"community","justification":"again","duration_days":64}`
	rr := httptest.NewRecorder()
	handleCreateAccessRequest(rr, memberContext(httptest.NewRequest(http.MethodPost, "/api/v1/requests", strings.NewReader(body)), "member-1"))
	if rr.Code != http.StatusConflict || !strings.Contains(rr.Body.String(), "ALREADY_HELD") {
		t.Fatalf("want 409 ALREADY_HELD, got %d %s", rr.Code, rr.Body.String())
	}
	if created {
		t.Fatal("nothing may be filed for a role already held for good")
	}
}

func TestACreateForARoleThatEndsIsRefusedToo(t *testing.T) {
	resetAccessDeps(t)
	t.Setenv("ZITADEL_DOMAIN", "example.zitadel.cloud")
	until := time.Now().Add(72 * time.Hour)
	dbRequestHolding = func(context.Context, string, string, string) (models.RequestHolding, error) {
		return models.RequestHolding{Kind: models.HoldingUntil, Until: &until}, nil
	}
	created := false
	dbCreateAccessRequestOfKind = func(context.Context, string, string, string, string, string, *int, *time.Time) (string, error) {
		created = true
		return "req-1", nil
	}
	body := `{"project_id":"p1","role_key":"community","justification":"keep it","duration_days":30}`
	rr := httptest.NewRecorder()
	handleCreateAccessRequest(rr, memberContext(httptest.NewRequest(http.MethodPost, "/api/v1/requests", strings.NewReader(body)), "member-1"))
	if rr.Code != http.StatusConflict || !strings.Contains(rr.Body.String(), "ALREADY_HELD") || created {
		t.Fatalf("a request is only for a role they lack (owner, 15 Oct): %d %s (created %v)", rr.Code, rr.Body.String(), created)
	}
}

// The end date is judged once, when the ask arrives. A date that passes while the holding is
// being read is not an ask for an ended date, and must not be filed as one by a second clock.
func TestAnAskIsJudgedOnOneClockWhenItsEndDatePassesMidRequest(t *testing.T) {
	resetAccessDeps(t)
	t.Setenv("ZITADEL_DOMAIN", "example.zitadel.cloud")
	until := time.Now().Add(40 * time.Millisecond)
	dbRequestHolding = func(context.Context, string, string, string) (models.RequestHolding, error) {
		time.Sleep(80 * time.Millisecond)
		return models.RequestHolding{Kind: models.HoldingNone}, nil
	}
	dbInsertAuditLog = func(context.Context, string, string, string, string) error { return nil }
	var filedUntil *time.Time
	dbCreateAccessRequestOfKind = func(_ context.Context, _, _, _, _, _ string, _ *int, u *time.Time) (string, error) {
		filedUntil = u
		return "req-1", nil
	}
	body := fmt.Sprintf(`{"project_id":"p1","role_key":"community","justification":"term","duration_days":30,"until":%q}`,
		until.Format(time.RFC3339Nano))
	rr := httptest.NewRecorder()
	handleCreateAccessRequest(rr, memberContext(httptest.NewRequest(http.MethodPost, "/api/v1/requests", strings.NewReader(body)), "member-1"))
	if rr.Code != http.StatusCreated || filedUntil == nil {
		t.Fatalf("want it filed on the clock it arrived on: %d %s (filed %v)", rr.Code, rr.Body.String(), filedUntil)
	}
}

func TestASecondIdenticalAskPointsAtTheWaitingOne(t *testing.T) {
	resetAccessDeps(t)
	t.Setenv("ZITADEL_DOMAIN", "example.zitadel.cloud")
	audited := false
	dbCreateAccessRequestOfKind = func(context.Context, string, string, string, string, string, *int, *time.Time) (string, error) {
		return "req-first", db.ErrAlreadyAsked
	}
	dbInsertAuditLog = func(context.Context, string, string, string, string) error { audited = true; return nil }
	body := `{"project_id":"p1","role_key":"community","justification":"again"}`
	rr := httptest.NewRecorder()
	handleCreateAccessRequest(rr, memberContext(httptest.NewRequest(http.MethodPost, "/api/v1/requests", strings.NewReader(body)), "member-1"))
	if rr.Code != http.StatusConflict || !strings.Contains(rr.Body.String(), "ALREADY_ASKED") || !strings.Contains(rr.Body.String(), "req-first") {
		t.Fatalf("want 409 ALREADY_ASKED naming the waiting request, got %d %s", rr.Code, rr.Body.String())
	}
	if audited {
		t.Fatal("a refused copy is not a new request and leaves no created audit row")
	}
}

func TestApprovingARoleTheyHaveSinceComeToHoldIsRefused(t *testing.T) {
	resetAccessDeps(t)
	probe := stubDecide(t)
	probe.DecideErr = services.ErrRequestAlreadyHeld

	err := resolveOneAccessRequest(context.Background(), "r1", "approved", "op1", "")

	if !errors.Is(err, services.ErrRequestAlreadyHeld) {
		t.Fatalf("approving a role they hold must be refused: %v", err)
	}
	probe.wantOrder(t, "decide") // nothing made effective, nothing sent
}

// Giving a role someone has is refused; Extend is the one way to change an
// end, and only to a later one.
func TestGivingARoleTheyHaveIsRefusedAndExtendOnlyLengthens(t *testing.T) {
	resetAccessDeps(t)
	until := time.Now().Add(10 * 24 * time.Hour)
	dbRequestHolding = func(context.Context, string, string, string) (models.RequestHolding, error) {
		return models.RequestHolding{Kind: models.HoldingUntil, Until: &until}, nil
	}
	// A plain give is the planner's: it refuses a role they have. Extend is not on the planner yet.
	probe := stubAct(t)
	probe.Err = &services.Refused{Change: services.Change{Refusal: "held"}}
	var got []db.EnqueueParams
	dbEnqueueDirectGrantPropagation = func(_ context.Context, p db.EnqueueParams, _ ...db.EnqueueParams) (db.EnqueueResult, error) {
		got = append(got, p)
		return db.EnqueueResult{OutboxID: "o1"}, nil
	}
	svcDrainPropagationRow = func(context.Context, string) (propagation.DrainResult, error) { return propagation.DrainResult{}, nil }
	call := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/users/u1/grants", strings.NewReader(body))
		req.SetPathValue("id", "u1")
		rr := httptest.NewRecorder()
		handleUpsertUserDirectGrant(rr, req)
		return rr
	}
	if rr := call(`{"project_id":"p1","role_key":"trained","reason":"r","duration_days":30}`); rr.Code != http.StatusConflict || !strings.Contains(rr.Body.String(), "ALREADY_HELD") {
		t.Fatalf("giving a role they have: want 409 ALREADY_HELD, got %d %s", rr.Code, rr.Body.String())
	}
	if rr := call(`{"project_id":"p1","role_key":"trained","reason":"r","duration_days":3,"extend":true}`); rr.Code != http.StatusConflict || !strings.Contains(rr.Body.String(), "ALREADY_RUNS_PAST") {
		t.Fatalf("an extension to an earlier end: want 409 ALREADY_RUNS_PAST, got %d %s", rr.Code, rr.Body.String())
	}
	if len(got) != 0 {
		t.Fatalf("nothing may be written for a refusal: %+v", got)
	}
	if rr := call(`{"project_id":"p1","role_key":"trained","reason":"r","duration_days":90,"extend":true}`); rr.Code >= 300 {
		t.Fatalf("a later end is an extension: %d %s", rr.Code, rr.Body.String())
	}
	if len(got) != 1 || got[0].EndRule != db.EndSet {
		t.Fatalf("an extension sets the end: %+v", got)
	}
}

func TestWaitingRequestsSayWhatTheRequesterAlreadyHas(t *testing.T) {
	resetAccessDeps(t)
	origGet := dbGetAccessRequests
	t.Cleanup(func() { dbGetAccessRequests = origGet })
	dbGetAccessRequests = func(context.Context, string) ([]models.AccessRequest, error) {
		return []models.AccessRequest{
			{ID: "r-wait", RequesterID: "u1", ProjectID: "p1", RoleKey: "community", Status: "pending"},
			{ID: "r-done", RequesterID: "u1", ProjectID: "p1", RoleKey: "community", Status: "approved"},
		}, nil
	}
	dbRequestHolding = func(context.Context, string, string, string) (models.RequestHolding, error) {
		return models.RequestHolding{Kind: models.HoldingForGood}, nil
	}
	rr := httptest.NewRecorder()
	handleGetAccessRequests(rr, httptest.NewRequest(http.MethodGet, "/api/v1/requests", nil))
	var got []models.AccessRequest
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got[0].Holding == nil || got[0].Holding.Kind != models.HoldingForGood {
		t.Errorf("a waiting request must say what they hold: %+v", got[0].Holding)
	}
	if got[1].Holding != nil {
		t.Errorf("a decided request reads nothing: %+v", got[1].Holding)
	}
}

// An operator's own requests (A-04 §8): ?mine=1 narrows the org-wide read to
// the ones they asked for, and without it the operator still sees everyone's.
func TestHandleGetAccessRequests_MineNarrowsAnOperatorToTheirOwn(t *testing.T) {
	t.Setenv("ZITADEL_DOMAIN", "example.zitadel.cloud")
	t.Setenv("ZITADEL_ADMIN_ROLE_KEY", "admin")
	origGet := dbGetAccessRequests
	t.Cleanup(func() { dbGetAccessRequests = origGet })
	dbGetAccessRequests = func(context.Context, string) ([]models.AccessRequest, error) {
		return []models.AccessRequest{
			{ID: "r-mine", RequesterID: "op-1"},
			{ID: "r-other", RequesterID: "someone-else"},
		}, nil
	}
	operator := func(url string) *http.Request {
		req := httptest.NewRequest(http.MethodGet, url, nil)
		return req.WithContext(withPrincipal(req.Context(), &auth.Principal{
			Subject:      "op-1",
			ProjectRoles: map[string]struct{}{"admin": {}},
		}))
	}
	ids := func(req *http.Request) []string {
		rr := httptest.NewRecorder()
		handleGetAccessRequests(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("%d: %s", rr.Code, rr.Body)
		}
		var got []models.AccessRequest
		if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, r := range got {
			out = append(out, r.ID)
		}
		return out
	}
	if got := ids(operator("/api/v1/requests?mine=1")); len(got) != 1 || got[0] != "r-mine" {
		t.Fatalf("mine=1: %v", got)
	}
	if got := ids(operator("/api/v1/requests")); len(got) != 2 {
		t.Fatalf("without mine: %v", got)
	}
	// A member gets their own either way, and cannot widen it.
	for _, url := range []string{"/api/v1/requests", "/api/v1/requests?mine=1", "/api/v1/requests?mine=0"} {
		if got := ids(memberContext(httptest.NewRequest(http.MethodGet, url, nil), "someone-else")); len(got) != 1 || got[0] != "r-other" {
			t.Fatalf("member %s: %v", url, got)
		}
	}
}

// A change queued for the operator waits for them to send; the member only
// learns that staff haven't sent it.
func TestAlreadyHeldMessage_WaitingIsWordedForWhoReadsIt(t *testing.T) {
	h := models.RequestHolding{Kind: models.HoldingForGood, Waiting: true}
	if got := alreadyHeldMessage(h, "They"); !strings.Contains(got, "waiting for you to send") || strings.Contains(got, "to be sent") {
		t.Errorf("operator: %q", got)
	}
	if got := alreadyHeldMessage(h, "You"); strings.Contains(got, "waiting") || strings.Contains(got, "to be sent") {
		t.Errorf("member: %q", got)
	}
}
