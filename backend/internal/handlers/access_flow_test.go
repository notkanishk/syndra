package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"syndra/internal/auth"
	"syndra/internal/db"
	"syndra/internal/models"
	"syndra/internal/services/propagation"
)

// Approving a request creates the same kind of direct grant the operator grant
// endpoint does, so it MUST flow through the outbox (enqueue), not the bare
// ledger upsert — otherwise the grant is invisible to the Pending UI, never
// projected to Zitadel, and later re-surfaces as syndra_only drift. The chosen
// semantics are enqueue + apply inline (the approval is the operator's confirm).
func TestHandleResolveAccessRequestApprovedEnqueuesGrantAndRebuildsCache(t *testing.T) {
	resetAccessDeps(t)

	duration := 5
	dbGetAccessRequestByID = func(ctx context.Context, id string) (models.AccessRequest, error) {
		return models.AccessRequest{
			ID:           id,
			RequesterID:  "u1",
			ProjectID:    "p1",
			RoleKey:      "admin",
			Status:       "pending",
			DurationDays: &duration,
		}, nil
	}

	upsertCalled := false
	dbUpsertDirectGrant = func(context.Context, string, string, string, string, string, *time.Time) (string, error) {
		upsertCalled = true
		return "", nil
	}
	plainResolveCalled := false
	dbResolveAccessRequest = func(context.Context, string, string, string, string) error {
		plainResolveCalled = true
		return nil
	}

	// Resolution + enqueue are ONE conditional transaction: the handler must use
	// the combined path, not resolve-then-enqueue across two transactions.
	var gotID, gotReviewer, gotNote string
	var gotParams db.EnqueueParams
	dbApproveRequestAndEnqueue = func(_ context.Context, id, reviewer, note string, p db.EnqueueParams) (db.EnqueueResult, error) {
		gotID, gotReviewer, gotNote, gotParams = id, reviewer, note, p
		return db.EnqueueResult{OutboxID: "ob-appr", Status: "pending"}, nil
	}

	var drainedRow string
	svcDrainPropagationRow = func(_ context.Context, outboxID string) (propagation.DrainResult, error) {
		drainedRow = outboxID
		return propagation.DrainResult{Applied: 1}, nil
	}
	svcDrainPropagations = func(context.Context) (propagation.DrainResult, error) {
		t.Fatal("approval must drain ONLY its own row, not the global batch")
		return propagation.DrainResult{}, nil
	}

	rebuiltFor := ""
	cacheRebuildUser = func(ctx context.Context, userID string, projectIDs []string) {
		rebuiltFor = userID
	}

	auditAction := ""
	dbInsertAuditLog = func(ctx context.Context, actorID, targetID, action, resourceID string) error {
		auditAction = action
		return nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/requests/req-1/decision", strings.NewReader(`{"status":"approved","reviewer_id":"reviewer-1","review_note":"ok"}`))
	req.SetPathValue("id", "req-1")
	rr := httptest.NewRecorder()
	handleResolveAccessRequest(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if upsertCalled {
		t.Fatal("approval must enqueue through the outbox, not the bare ledger upsert")
	}
	if plainResolveCalled {
		t.Fatal("approval must resolve+enqueue in one transaction, not via the standalone resolve")
	}
	if drainedRow != "ob-appr" {
		t.Fatalf("approval must apply inline by draining ONLY its own outbox row, got %q", drainedRow)
	}
	if rebuiltFor != "u1" {
		t.Fatalf("expected cache rebuild for u1, got %s", rebuiltFor)
	}
	if auditAction != "access_request.approved" {
		t.Fatalf("unexpected audit action %s", auditAction)
	}
	if gotID != "req-1" || gotReviewer != "reviewer-1" || gotNote != "ok" {
		t.Fatalf("combined approve must pass request id/reviewer/note, got id=%q reviewer=%q note=%q", gotID, gotReviewer, gotNote)
	}
	// The enqueue must carry the request's grant, attributed to the reviewer, and
	// linked back to the originating request via source_ref.
	if gotParams.OpType != "add" || gotParams.Source != "direct" || gotParams.SourceRef != "req-1" {
		t.Fatalf("enqueue must be add/direct with source_ref=request id, got %+v", gotParams)
	}
	if gotParams.UserID != "u1" || gotParams.ProjectID != "p1" ||
		len(gotParams.RoleKeys) != 1 || gotParams.RoleKeys[0] != "admin" {
		t.Fatalf("enqueue must target the requested user/project/role, got %+v", gotParams)
	}
	if gotParams.GrantedBy != "reviewer-1" || gotParams.Reason != "Approved from access request" {
		t.Fatalf("enqueue attribution wrong, got grantedBy=%q reason=%q", gotParams.GrantedBy, gotParams.Reason)
	}
	if gotParams.PayloadJSON == "" {
		t.Fatal("payload_json must be non-empty (JSONB NOT NULL)")
	}
	if gotParams.ExpiresAt == nil {
		t.Fatalf("expected expiry for duration-backed request")
	}
	expected := time.Now().UTC().Add(5 * 24 * time.Hour)
	if diff := gotParams.ExpiresAt.Sub(expected); diff < -time.Minute || diff > time.Minute {
		t.Fatalf("expiresAt %v not within 1 minute of expected %v", gotParams.ExpiresAt, expected)
	}
}

// A lost approve/reject race (request resolved between the read and the
// conditional resolve) surfaces as ErrRequestNotPending, which the handler must
// map to 409 rather than a 500 or a silent success.
func TestHandleResolveAccessRequest_ApproveRace_Returns409(t *testing.T) {
	resetAccessDeps(t)

	dbGetAccessRequestByID = func(ctx context.Context, id string) (models.AccessRequest, error) {
		return models.AccessRequest{ID: id, RequesterID: "u1", ProjectID: "p1", RoleKey: "admin", Status: "pending"}, nil
	}
	dbApproveRequestAndEnqueue = func(context.Context, string, string, string, db.EnqueueParams) (db.EnqueueResult, error) {
		return db.EnqueueResult{}, db.ErrRequestNotPending
	}
	drainCalled := false
	svcDrainPropagationRow = func(context.Context, string) (propagation.DrainResult, error) {
		drainCalled = true
		return propagation.DrainResult{}, nil
	}
	cacheRebuildUser = func(context.Context, string, []string) {}
	dbInsertAuditLog = func(context.Context, string, string, string, string) error { return nil }

	req := httptest.NewRequest(http.MethodPost, "/api/v1/requests/req-9/decision",
		strings.NewReader(`{"status":"approved","reviewer_id":"reviewer-1"}`))
	req.SetPathValue("id", "req-9")
	rr := httptest.NewRecorder()
	handleResolveAccessRequest(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("a lost approve race must return 409, got %d: %s", rr.Code, rr.Body.String())
	}
	if drainCalled {
		t.Fatal("must not drain when the conditional resolve found nothing to approve")
	}
}

// The compiled cache (which the claim path reads) must be rebuilt from the
// committed ledger row BEFORE the best-effort inline drain, so a slow or canceled
// Zitadel call cannot starve the rebuild via the shared request context — access
// must be effective regardless of the projection outcome.
func TestHandleResolveAccessRequest_RebuildsCacheBeforeInlineDrain(t *testing.T) {
	resetAccessDeps(t)

	dbGetAccessRequestByID = func(ctx context.Context, id string) (models.AccessRequest, error) {
		return models.AccessRequest{ID: id, RequesterID: "u1", ProjectID: "p1", RoleKey: "admin", Status: "pending"}, nil
	}
	dbApproveRequestAndEnqueue = func(context.Context, string, string, string, db.EnqueueParams) (db.EnqueueResult, error) {
		return db.EnqueueResult{OutboxID: "ob-appr", Status: "pending"}, nil
	}
	var order []string
	cacheRebuildUser = func(context.Context, string, []string) { order = append(order, "rebuild") }
	svcDrainPropagationRow = func(context.Context, string) (propagation.DrainResult, error) {
		order = append(order, "drain")
		return propagation.DrainResult{Applied: 1}, nil
	}
	dbInsertAuditLog = func(context.Context, string, string, string, string) error { return nil }

	req := httptest.NewRequest(http.MethodPost, "/api/v1/requests/req-1/decision",
		strings.NewReader(`{"status":"approved","reviewer_id":"reviewer-1"}`))
	req.SetPathValue("id", "req-1")
	handleResolveAccessRequest(httptest.NewRecorder(), req)

	if len(order) != 2 || order[0] != "rebuild" || order[1] != "drain" {
		t.Fatalf("cache rebuild must run BEFORE the inline drain, got %v", order)
	}
}

// The grant is durable once the enqueue tx commits, so the compiled-cache
// rebuild that makes access effective MUST run even if the client disconnects
// before it — it runs on a context detached from the request lifecycle.
func TestHandleResolveAccessRequest_RebuildDetachedFromClientCancel(t *testing.T) {
	resetAccessDeps(t)

	dbGetAccessRequestByID = func(ctx context.Context, id string) (models.AccessRequest, error) {
		return models.AccessRequest{ID: id, RequesterID: "u1", ProjectID: "p1", RoleKey: "admin", Status: "pending"}, nil
	}
	dbApproveRequestAndEnqueue = func(context.Context, string, string, string, db.EnqueueParams) (db.EnqueueResult, error) {
		return db.EnqueueResult{OutboxID: "ob-appr", Status: "pending"}, nil
	}
	svcDrainPropagationRow = func(context.Context, string) (propagation.DrainResult, error) {
		return propagation.DrainResult{}, nil
	}
	dbInsertAuditLog = func(context.Context, string, string, string, string) error { return nil }

	var rebuilt bool
	var rebuildCtxErr error
	cacheRebuildUser = func(ctx context.Context, userID string, projectIDs []string) {
		rebuilt = true
		rebuildCtxErr = ctx.Err()
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // client already disconnected
	req := httptest.NewRequest(http.MethodPost, "/api/v1/requests/req-1/decision",
		strings.NewReader(`{"status":"approved","reviewer_id":"reviewer-1"}`)).WithContext(ctx)
	req.SetPathValue("id", "req-1")
	handleResolveAccessRequest(httptest.NewRecorder(), req)

	if !rebuilt {
		t.Fatal("cache rebuild must run even after the client disconnected (the grant is already durable)")
	}
	if rebuildCtxErr != nil {
		t.Fatalf("cache rebuild must run on a context detached from client cancellation, got %v", rebuildCtxErr)
	}
}

func TestHandleResolveAccessRequest_RejectRace_Returns409(t *testing.T) {
	resetAccessDeps(t)

	dbGetAccessRequestByID = func(ctx context.Context, id string) (models.AccessRequest, error) {
		return models.AccessRequest{ID: id, RequesterID: "u1", ProjectID: "p1", RoleKey: "admin", Status: "pending"}, nil
	}
	dbResolveAccessRequest = func(context.Context, string, string, string, string) error {
		return db.ErrRequestNotPending
	}
	dbInsertAuditLog = func(context.Context, string, string, string, string) error { return nil }

	req := httptest.NewRequest(http.MethodPost, "/api/v1/requests/req-9/decision",
		strings.NewReader(`{"status":"rejected","reviewer_id":"reviewer-2"}`))
	req.SetPathValue("id", "req-9")
	rr := httptest.NewRecorder()
	handleResolveAccessRequest(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("a lost reject race must return 409, got %d: %s", rr.Code, rr.Body.String())
	}
}

// resetAccessDeps captures and restores all access-related injectable vars.
func resetAccessDeps(t *testing.T) {
	t.Helper()
	origGetByID := dbGetAccessRequestByID
	origResolve := dbResolveAccessRequest
	origWithdraw := dbWithdrawAccessRequest
	origUpsert := dbUpsertDirectGrant
	origCreate := dbCreateAccessRequest
	origAudit := dbInsertAuditLog
	origRebuild := cacheRebuildUser
	origEnqueue := dbEnqueueDirectGrantPropagation
	origApprove := dbApproveRequestAndEnqueue
	origDrain := svcDrainPropagations
	origDrainRow := svcDrainPropagationRow
	origStatus := dbGetPropagationStatus
	t.Cleanup(func() {
		dbGetAccessRequestByID = origGetByID
		dbResolveAccessRequest = origResolve
		dbWithdrawAccessRequest = origWithdraw
		dbUpsertDirectGrant = origUpsert
		dbCreateAccessRequest = origCreate
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
	dbCreateAccessRequest = func(_ context.Context, requesterID, _, _, _ string, _ *int) (string, error) {
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

func TestHandleUpsertUserDirectGrant_EnqueuesAndReturns202(t *testing.T) {
	resetAccessDeps(t)

	var gotParams db.EnqueueParams
	dbEnqueueDirectGrantPropagation = func(_ context.Context, p db.EnqueueParams) (db.EnqueueResult, error) {
		gotParams = p
		return db.EnqueueResult{OutboxID: "ob1", IdempotencyKey: "key1", Status: "pending"}, nil
	}
	cacheRebuildUser = func(ctx context.Context, userID string, projectIDs []string) {}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/u1/grants",
		strings.NewReader(`{"project_id":"p1","role_key":"r1","reason":"lab access","duration_days":30}`))
	req.SetPathValue("id", "u1")
	rr := httptest.NewRecorder()
	handleUpsertUserDirectGrant(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("want 202, got %d: %s", rr.Code, rr.Body.String())
	}
	if gotParams.OpType != "add" || len(gotParams.RoleKeys) != 1 || gotParams.RoleKeys[0] != "r1" || gotParams.Source != "direct" {
		t.Fatalf("unexpected enqueue params: %+v", gotParams)
	}
	if gotParams.UserID != "u1" || gotParams.ProjectID != "p1" || gotParams.Reason != "lab access" {
		t.Fatalf("unexpected enqueue identity/reason: %+v", gotParams)
	}
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["outbox_id"] != "ob1" || body["status"] != "pending" {
		t.Fatalf("unexpected body: %v", body)
	}
}

func TestHandleUpsertUserDirectGrant_ExpiryCalculation(t *testing.T) {
	resetAccessDeps(t)

	var capturedExpiry *time.Time
	dbEnqueueDirectGrantPropagation = func(_ context.Context, p db.EnqueueParams) (db.EnqueueResult, error) {
		capturedExpiry = p.ExpiresAt
		return db.EnqueueResult{OutboxID: "ob1", Status: "pending"}, nil
	}
	cacheRebuildUser = func(ctx context.Context, userID string, projectIDs []string) {}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/u1/grants",
		strings.NewReader(`{"project_id":"printing","role_key":"member","duration_days":7}`))
	req.SetPathValue("id", "u1")
	rr := httptest.NewRecorder()
	handleUpsertUserDirectGrant(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rr.Code, rr.Body.String())
	}
	if capturedExpiry == nil {
		t.Fatalf("expected non-nil expiresAt for DurationDays=7")
	}
	expected := time.Now().UTC().Add(7 * 24 * time.Hour)
	diff := capturedExpiry.Sub(expected)
	if diff < -time.Minute || diff > time.Minute {
		t.Fatalf("expiresAt %v is not within 1 minute of expected %v", capturedExpiry, expected)
	}
}

func TestHandleUpsertUserDirectGrant_ZeroDuration_NoExpiry(t *testing.T) {
	resetAccessDeps(t)

	var capturedExpiry *time.Time
	dbEnqueueDirectGrantPropagation = func(_ context.Context, p db.EnqueueParams) (db.EnqueueResult, error) {
		capturedExpiry = p.ExpiresAt
		return db.EnqueueResult{OutboxID: "ob2", Status: "pending"}, nil
	}
	cacheRebuildUser = func(ctx context.Context, userID string, projectIDs []string) {}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/u1/grants",
		strings.NewReader(`{"project_id":"printing","role_key":"member","duration_days":0}`))
	req.SetPathValue("id", "u1")
	rr := httptest.NewRecorder()
	handleUpsertUserDirectGrant(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rr.Code, rr.Body.String())
	}
	if capturedExpiry != nil {
		t.Fatalf("expected nil expiresAt for DurationDays=0, got %v", capturedExpiry)
	}
}

func TestHandleUpsertUserDirectGrant_CacheRebuilt(t *testing.T) {
	resetAccessDeps(t)

	dbEnqueueDirectGrantPropagation = func(_ context.Context, p db.EnqueueParams) (db.EnqueueResult, error) {
		return db.EnqueueResult{OutboxID: "ob3", Status: "pending"}, nil
	}

	rebuiltUserID := ""
	cacheRebuildUser = func(ctx context.Context, userID string, projectIDs []string) {
		rebuiltUserID = userID
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/u42/grants",
		strings.NewReader(`{"project_id":"printing","role_key":"member"}`))
	req.SetPathValue("id", "u42")
	rr := httptest.NewRecorder()
	handleUpsertUserDirectGrant(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rr.Code, rr.Body.String())
	}
	if rebuiltUserID != "u42" {
		t.Fatalf("expected cache rebuild for u42, got %s", rebuiltUserID)
	}
}

func TestHandleUpsertUserDirectGrant_ApplyNowDrains(t *testing.T) {
	resetAccessDeps(t)

	dbEnqueueDirectGrantPropagation = func(_ context.Context, p db.EnqueueParams) (db.EnqueueResult, error) {
		return db.EnqueueResult{OutboxID: "ob4", Status: "pending"}, nil
	}
	var drainedRow string
	svcDrainPropagationRow = func(_ context.Context, outboxID string) (propagation.DrainResult, error) {
		drainedRow = outboxID
		return propagation.DrainResult{Applied: 1}, nil
	}
	svcDrainPropagations = func(context.Context) (propagation.DrainResult, error) {
		t.Fatal("apply=true must drain ONLY this row, not the global batch")
		return propagation.DrainResult{}, nil
	}
	// This request's row (ob4) actually applied.
	dbGetPropagationStatus = func(_ context.Context, id string) (string, error) {
		if id != "ob4" {
			t.Fatalf("inline apply must read THIS request's outbox row, got %q", id)
		}
		return "applied", nil
	}
	cacheRebuildUser = func(ctx context.Context, userID string, projectIDs []string) {}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/u1/grants?apply=true",
		strings.NewReader(`{"project_id":"p1","role_key":"r1"}`))
	req.SetPathValue("id", "u1")
	rr := httptest.NewRecorder()
	handleUpsertUserDirectGrant(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rr.Code, rr.Body.String())
	}
	if drainedRow != "ob4" {
		t.Fatalf("apply=true must drain THIS row (ob4), got %q", drainedRow)
	}
	var body map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if body["status"] != "applied" {
		t.Fatalf("apply=true must report THIS row's status=applied, got %v", body["status"])
	}
}

func TestHandleUpsertUserDirectGrant_RebuildsCacheBeforeInlineDrain(t *testing.T) {
	resetAccessDeps(t)

	dbEnqueueDirectGrantPropagation = func(_ context.Context, p db.EnqueueParams) (db.EnqueueResult, error) {
		return db.EnqueueResult{OutboxID: "ob6", Status: "pending"}, nil
	}
	var order []string
	cacheRebuildUser = func(context.Context, string, []string) { order = append(order, "rebuild") }
	svcDrainPropagationRow = func(context.Context, string) (propagation.DrainResult, error) {
		order = append(order, "drain")
		return propagation.DrainResult{Applied: 1}, nil
	}
	dbGetPropagationStatus = func(context.Context, string) (string, error) { return "applied", nil }

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/u1/grants?apply=true",
		strings.NewReader(`{"project_id":"p1","role_key":"r1"}`))
	req.SetPathValue("id", "u1")
	handleUpsertUserDirectGrant(httptest.NewRecorder(), req)

	if len(order) != 2 || order[0] != "rebuild" || order[1] != "drain" {
		t.Fatalf("cache rebuild must run BEFORE the inline drain so access is effective regardless of the drain outcome, got %v", order)
	}
}

func TestHandleUpsertUserDirectGrant_RebuildDetachedFromClientCancel(t *testing.T) {
	resetAccessDeps(t)

	dbEnqueueDirectGrantPropagation = func(context.Context, db.EnqueueParams) (db.EnqueueResult, error) {
		return db.EnqueueResult{OutboxID: "ob7", Status: "pending"}, nil
	}
	var rebuilt bool
	var rebuildCtxErr error
	cacheRebuildUser = func(ctx context.Context, userID string, projectIDs []string) {
		rebuilt = true
		rebuildCtxErr = ctx.Err()
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // client already disconnected (no ?apply, so no drain is involved)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/u1/grants",
		strings.NewReader(`{"project_id":"p1","role_key":"r1"}`)).WithContext(ctx)
	req.SetPathValue("id", "u1")
	handleUpsertUserDirectGrant(httptest.NewRecorder(), req)

	if !rebuilt {
		t.Fatal("cache rebuild must run even after the client disconnected (the grant is already durable)")
	}
	if rebuildCtxErr != nil {
		t.Fatalf("cache rebuild must run on a context detached from client cancellation, got %v", rebuildCtxErr)
	}
}

// The batch drain reports the aggregate outcome of the OLDEST rows; this
// request's freshly-enqueued row may not be in that batch (or may requeue). The
// response must reflect THIS row's actual status, not the batch's Applied count.
func TestHandleUpsertUserDirectGrant_ApplyReportsThisRowNotBatch(t *testing.T) {
	resetAccessDeps(t)

	dbEnqueueDirectGrantPropagation = func(_ context.Context, p db.EnqueueParams) (db.EnqueueResult, error) {
		return db.EnqueueResult{OutboxID: "ob5", Status: "pending"}, nil
	}
	// The targeted drain ran, but ob5 itself requeued (transient Zitadel error).
	svcDrainPropagationRow = func(_ context.Context, outboxID string) (propagation.DrainResult, error) {
		return propagation.DrainResult{Requeued: 1}, nil
	}
	dbGetPropagationStatus = func(_ context.Context, id string) (string, error) {
		if id != "ob5" {
			t.Fatalf("must query THIS request's outbox row, got %q", id)
		}
		return "pending", nil
	}
	cacheRebuildUser = func(ctx context.Context, userID string, projectIDs []string) {}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/u1/grants?apply=true",
		strings.NewReader(`{"project_id":"p1","role_key":"r1"}`))
	req.SetPathValue("id", "u1")
	rr := httptest.NewRecorder()
	handleUpsertUserDirectGrant(rr, req)

	var body map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if body["status"] != "pending" {
		t.Fatalf("must report THIS row's status (pending), not the batch aggregate, got %v", body["status"])
	}
}

// --- handleCreateAccessRequest ---

func TestHandleCreateAccessRequest_PersistenceAndAudit(t *testing.T) {
	resetAccessDeps(t)

	dbCreateAccessRequest = func(ctx context.Context, requesterID, projectID, roleKey, justification string, durationDays *int) (string, error) {
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
	dbCreateAccessRequest = func(ctx context.Context, requesterID, projectID, roleKey, justification string, durationDays *int) (string, error) {
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

// --- handleResolveAccessRequest (idempotency) ---

func TestHandleResolveAccessRequest_AlreadyApproved_Returns409(t *testing.T) {
	resetAccessDeps(t)

	dbGetAccessRequestByID = func(ctx context.Context, id string) (models.AccessRequest, error) {
		return models.AccessRequest{
			ID:          id,
			RequesterID: "u1",
			ProjectID:   "p1",
			RoleKey:     "admin",
			Status:      "approved",
		}, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/requests/req-1/decision",
		strings.NewReader(`{"status":"approved","reviewer_id":"reviewer-1"}`))
	req.SetPathValue("id", "req-1")
	rr := httptest.NewRecorder()
	handleResolveAccessRequest(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "ALREADY_RESOLVED" {
		t.Fatalf("expected ALREADY_RESOLVED, got %v", resp["error"])
	}
}

func TestHandleResolveAccessRequest_AlreadyRejected_Returns409(t *testing.T) {
	resetAccessDeps(t)

	dbGetAccessRequestByID = func(ctx context.Context, id string) (models.AccessRequest, error) {
		return models.AccessRequest{
			ID:          id,
			RequesterID: "u1",
			ProjectID:   "p1",
			RoleKey:     "admin",
			Status:      "rejected",
		}, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/requests/req-2/decision",
		strings.NewReader(`{"status":"approved","reviewer_id":"reviewer-1"}`))
	req.SetPathValue("id", "req-2")
	rr := httptest.NewRecorder()
	handleResolveAccessRequest(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "ALREADY_RESOLVED" {
		t.Fatalf("expected ALREADY_RESOLVED, got %v", resp["error"])
	}
}

func TestHandleResolveAccessRequestRejectedDoesNotCreateGrant(t *testing.T) {
	resetAccessDeps(t)

	dbGetAccessRequestByID = func(ctx context.Context, id string) (models.AccessRequest, error) {
		return models.AccessRequest{
			ID:          id,
			RequesterID: "u1",
			ProjectID:   "p1",
			RoleKey:     "admin",
			Status:      "pending",
		}, nil
	}
	dbResolveAccessRequest = func(ctx context.Context, id, status, reviewerID, reviewNote string) error {
		return nil
	}

	grantCreated := false
	dbUpsertDirectGrant = func(ctx context.Context, userID, projectID, roleKey, grantedBy, reason string, expiresAt *time.Time) (string, error) {
		grantCreated = true
		return "", nil
	}
	enqueued := false
	dbEnqueueDirectGrantPropagation = func(context.Context, db.EnqueueParams) (db.EnqueueResult, error) {
		enqueued = true
		return db.EnqueueResult{}, nil
	}
	defer func() {
		if enqueued {
			t.Fatal("rejection must not enqueue a grant propagation")
		}
	}()

	rebuilt := false
	cacheRebuildUser = func(ctx context.Context, userID string, projectIDs []string) {
		rebuilt = true
	}

	auditAction := ""
	dbInsertAuditLog = func(ctx context.Context, actorID, targetID, action, resourceID string) error {
		auditAction = action
		return nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/requests/req-2/decision", strings.NewReader(`{"status":"rejected","reviewer_id":"reviewer-2","review_note":"deny"}`))
	req.SetPathValue("id", "req-2")
	rr := httptest.NewRecorder()
	handleResolveAccessRequest(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if grantCreated {
		t.Fatalf("did not expect direct grant on rejection")
	}
	if rebuilt {
		t.Fatalf("did not expect cache rebuild on rejection")
	}
	if auditAction != "access_request.rejected" {
		t.Fatalf("unexpected audit action %s", auditAction)
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
func TestHandleResolveAccessRequest_WithdrawnCannotBeDecided(t *testing.T) {
	resetAccessDeps(t)

	dbGetAccessRequestByID = func(ctx context.Context, id string) (models.AccessRequest, error) {
		return models.AccessRequest{ID: id, RequesterID: "u1", ProjectID: "p1", RoleKey: "admin",
			Status: "withdrawn"}, nil
	}
	dbResolveAccessRequest = func(ctx context.Context, id, status, reviewerID, reviewNote string) error {
		t.Fatal("a withdrawn request must not be decidable")
		return nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/requests/r1/decision",
		strings.NewReader(`{"status":"rejected","reviewer_id":"reviewer-1","review_note":"no"}`))
	req.SetPathValue("id", "r1")
	rr := httptest.NewRecorder()
	handleResolveAccessRequest(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "already withdrawn") {
		t.Fatalf("the conflict must name the state it is in: %s", rr.Body.String())
	}
}
