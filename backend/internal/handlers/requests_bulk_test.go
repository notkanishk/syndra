package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"syndra/internal/auth"
	"syndra/internal/db"
	"syndra/internal/models"
	"syndra/internal/services"
	"syndra/internal/services/propagation"
)

func bulkDecisionReq(t *testing.T, body string, apply bool) *http.Request {
	t.Helper()
	path := "/api/v1/requests/bulk-decision"
	if apply {
		path += "?apply=true"
	}
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	// An approval needs an attributable reviewer; without one the endpoint
	// refuses, which is asserted separately below.
	return req.WithContext(withPrincipal(req.Context(), &auth.Principal{Subject: "op_1"}))
}

func decodeBulkPlan(t *testing.T, rr *httptest.ResponseRecorder) services.BulkPlan {
	t.Helper()
	var plan services.BulkPlan
	if err := json.Unmarshal(rr.Body.Bytes(), &plan); err != nil {
		t.Fatalf("decode plan: %v (%s)", err, rr.Body.String())
	}
	return plan
}

func stubRequestLookup(t *testing.T, byID map[string]models.AccessRequest) *int {
	t.Helper()
	origGet := dbGetAccessRequestByID
	origApprove := dbApproveRequestAndEnqueue
	origResolve := dbResolveAccessRequest
	origAudit := dbInsertAuditLog
	origDrain := svcDrainPropagationRow
	origRebuild := cacheRebuildUser
	t.Cleanup(func() {
		cacheRebuildUser = origRebuild
		dbGetAccessRequestByID = origGet
		dbApproveRequestAndEnqueue = origApprove
		dbResolveAccessRequest = origResolve
		dbInsertAuditLog = origAudit
		svcDrainPropagationRow = origDrain
	})

	dbGetAccessRequestByID = func(_ context.Context, id string) (models.AccessRequest, error) {
		req, ok := byID[id]
		if !ok {
			return models.AccessRequest{}, errors.New("no rows in result set")
		}
		return req, nil
	}
	writes := 0
	dbApproveRequestAndEnqueue = func(context.Context, string, string, string, db.EnqueueParams) (db.EnqueueResult, error) {
		writes++
		return db.EnqueueResult{OutboxID: "ob_1", Status: "pending"}, nil
	}
	dbResolveAccessRequest = func(context.Context, string, string, string, string) error {
		writes++
		return nil
	}
	dbInsertAuditLog = func(context.Context, string, string, string, string) error { return nil }
	cacheRebuildUser = func(context.Context, string, []string) {}
	svcDrainPropagationRow = func(context.Context, string) (propagation.DrainResult, error) {
		return propagation.DrainResult{Applied: 1}, nil
	}
	return &writes
}

func pendingRequest(id, requester, role string) models.AccessRequest {
	return models.AccessRequest{
		ID: id, RequesterID: requester, ProjectID: "pLaser", RoleKey: role, Status: "pending",
	}
}

// Approving a batch is the operation with the least obvious blast radius on the
// product: each approval mints a direct grant, so "approve 9" is nine access
// changes wearing the clothes of an inbox action.
func TestBulkDecide_RehearsesByDefaultAndWritesNothing(t *testing.T) {
	writes := stubRequestLookup(t, map[string]models.AccessRequest{
		"r1": pendingRequest("r1", "u1", "trained"),
		"r2": pendingRequest("r2", "u2", "operator"),
	})

	rr := httptest.NewRecorder()
	handleBulkDecideRequests(rr, bulkDecisionReq(t, `{"ids":["r1","r2"],"status":"approved"}`, false))

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rr.Code, rr.Body.String())
	}
	if *writes != 0 {
		t.Errorf("a rehearsal must not write, got %d", *writes)
	}
	plan := decodeBulkPlan(t, rr)
	if plan.Applied || plan.Summary.Apply != 2 {
		t.Errorf("expected an unapplied plan of 2, got %+v", plan.Summary)
	}
	// The part an inbox action hides: this grants access.
	if !strings.Contains(plan.Outcomes[0].Consequence, "direct grant") {
		t.Errorf("the plan must say an approval mints a grant, got %q", plan.Outcomes[0].Consequence)
	}
}

func TestBulkDecide_SkipsRequestsSomebodyElseAlreadyDecided(t *testing.T) {
	already := pendingRequest("r2", "u2", "operator")
	already.Status = "approved"
	writes := stubRequestLookup(t, map[string]models.AccessRequest{
		"r1": pendingRequest("r1", "u1", "trained"),
		"r2": already,
	})

	rr := httptest.NewRecorder()
	handleBulkDecideRequests(rr, bulkDecisionReq(t, `{"ids":["r1","r2"],"status":"approved"}`, true))

	plan := decodeBulkPlan(t, rr)
	if plan.Summary.Succeeded != 1 || plan.Summary.NoChange != 1 {
		t.Fatalf("want 1 applied / 1 already-decided, got %+v", plan.Summary)
	}
	// Re-approving would mint a second grant for access the person already has.
	if *writes != 1 {
		t.Errorf("an already-decided request must not be re-written, got %d writes", *writes)
	}
}

func TestBulkDecide_NamesRequestsThatNoLongerExist(t *testing.T) {
	stubRequestLookup(t, map[string]models.AccessRequest{"r1": pendingRequest("r1", "u1", "trained")})

	rr := httptest.NewRecorder()
	handleBulkDecideRequests(rr, bulkDecisionReq(t, `{"ids":["r1","r_gone"],"status":"rejected"}`, false))

	plan := decodeBulkPlan(t, rr)
	if len(plan.Outcomes) != 2 {
		t.Fatalf("every selected id must produce a row, got %d", len(plan.Outcomes))
	}
	if plan.Summary.Blocked != 1 {
		t.Errorf("a withdrawn request must be blocked, not dropped: %+v", plan.Summary)
	}
}

func TestBulkDecide_DecliningChangesNoAccess(t *testing.T) {
	stubRequestLookup(t, map[string]models.AccessRequest{"r1": pendingRequest("r1", "u1", "trained")})

	rr := httptest.NewRecorder()
	handleBulkDecideRequests(rr, bulkDecisionReq(t, `{"ids":["r1"],"status":"rejected"}`, false))

	plan := decodeBulkPlan(t, rr)
	if !strings.Contains(plan.Outcomes[0].Consequence, "Nothing about their current access changes") {
		t.Errorf("a decline must say it takes nothing away, got %q", plan.Outcomes[0].Consequence)
	}
}

// A grant attributed to "system" is a grant nobody can be asked about. In bulk
// that would be true of the whole batch at once.
func TestBulkDecide_RefusesToApproveWithoutAnAttributableReviewer(t *testing.T) {
	writes := stubRequestLookup(t, map[string]models.AccessRequest{"r1": pendingRequest("r1", "u1", "trained")})

	rr := httptest.NewRecorder()
	// No admin id in context — resolveActor falls through to "system".
	handleBulkDecideRequests(rr, httptest.NewRequest(http.MethodPost,
		"/api/v1/requests/bulk-decision?apply=true",
		strings.NewReader(`{"ids":["r1"],"status":"approved"}`)))

	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected a validation failure, got %d (%s)", rr.Code, rr.Body.String())
	}
	if *writes != 0 {
		t.Errorf("nothing may be written, got %d", *writes)
	}
}

func TestBulkDecide_RejectsInvalidRequests(t *testing.T) {
	writes := stubRequestLookup(t, map[string]models.AccessRequest{"r1": pendingRequest("r1", "u1", "trained")})

	cases := []struct{ name, body string }{
		{"unknown status", `{"ids":["r1"],"status":"maybe"}`},
		{"empty selection", `{"ids":[],"status":"approved"}`},
		{"blank ids only", `{"ids":["  "],"status":"approved"}`},
		{"unknown field", `{"ids":["r1"],"status":"approved","force":true}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			handleBulkDecideRequests(rr, bulkDecisionReq(t, tc.body, true))
			if rr.Code != http.StatusBadRequest && rr.Code != http.StatusUnprocessableEntity {
				t.Errorf("expected a validation failure, got %d (%s)", rr.Code, rr.Body.String())
			}
		})
	}
	if *writes != 0 {
		t.Errorf("a rejected request must not write, got %d", *writes)
	}
}
