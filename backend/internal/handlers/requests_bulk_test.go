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
)

// bulkDecisionReq builds the request a test makes. An apply is the SECOND of
// two — it cites the approval the rehearsal issued — so the helper performs the
// rehearsal and splices its plan id in, exactly as a client must. The tests
// below stay about what the decision does; how it is authorised is asserted in
// plan_gate_test.go.
func bulkDecisionReq(t *testing.T, body string, apply bool) *http.Request {
	t.Helper()
	const path = "/api/v1/requests/bulk-decision"

	reviewed := func(r *http.Request) *http.Request {
		// An approval needs an attributable reviewer; without one the endpoint
		// refuses, which is asserted separately below.
		return r.WithContext(withPrincipal(r.Context(), &auth.Principal{Subject: "op_1"}))
	}
	if !apply {
		return reviewed(httptest.NewRequest(http.MethodPost, path, strings.NewReader(body)))
	}

	rehearse := httptest.NewRecorder()
	handleBulkDecideRequests(rehearse, reviewed(httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))))
	var issued services.BulkPlan
	if err := json.Unmarshal(rehearse.Body.Bytes(), &issued); err != nil || issued.PlanID == "" {
		// A body the rehearsal refuses has no plan to cite, and refusing it on
		// the apply path is what the validation cases assert.
		return reviewed(httptest.NewRequest(http.MethodPost, path+"?apply=true", strings.NewReader(body)))
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	payload["plan_id"] = issued.PlanID
	cited, _ := json.Marshal(payload)
	return reviewed(httptest.NewRequest(http.MethodPost, path+"?apply=true", strings.NewReader(string(cited))))
}

func decodeBulkPlan(t *testing.T, rr *httptest.ResponseRecorder) services.BulkPlan {
	t.Helper()
	var plan services.BulkPlan
	if err := json.Unmarshal(rr.Body.Bytes(), &plan); err != nil {
		t.Fatalf("decode plan: %v (%s)", err, rr.Body.String())
	}
	return plan
}

// stubRequestLookup stands in for what a bulk decision reads (the decline rehearsal), for
// the planner an approval goes through (stubPlannedBulk), for a decline's decision
// (svcDecideRequest: what it writes is services' tests' subject, so here it answers as a
// settled request would) and for the send (svcDeliverChange, every row applied). The count
// it returns is every write: a decline decided, or an approval the planner applied.
func stubRequestLookup(t *testing.T, byID map[string]models.AccessRequest) *int {
	t.Helper()
	// An apply cites an approval now, so every rehearsal needs somewhere to land.
	stubPlanStore(t)
	origGet := dbGetAccessRequestByID
	origRefresh := cacheRefresh
	origHolding := dbRequestHolding
	origStatus := dbGetPropagationStatus
	origDecide, origDeliver := svcDecideRequest, svcDeliverChange
	// Nobody holds anything unless a test says so.
	dbRequestHolding = func(context.Context, string, string, string) (models.RequestHolding, error) {
		return models.RequestHolding{Kind: models.HoldingNone}, nil
	}
	// A sent row is applied unless a test says so.
	dbGetPropagationStatus = func(context.Context, string) (string, error) { return "applied", nil }
	t.Cleanup(func() {
		dbGetPropagationStatus = origStatus
		dbRequestHolding = origHolding
		cacheRefresh = origRefresh
		dbGetAccessRequestByID = origGet
		svcDecideRequest, svcDeliverChange = origDecide, origDeliver
	})

	dbGetAccessRequestByID = func(_ context.Context, id string) (models.AccessRequest, error) {
		req, ok := byID[id]
		if !ok {
			return models.AccessRequest{}, errors.New("no rows in result set")
		}
		return req, nil
	}
	writes := 0
	svcDecideRequest = func(_ context.Context, id, status, _, _ string, _ *services.Seen) (services.Decision, error) {
		req, ok := byID[id]
		switch {
		case !ok:
			return services.Decision{}, services.ErrRequestNotFound
		case req.Status != "pending":
			return services.Decision{}, services.ErrRequestAlreadyResolved
		}
		writes++
		d := services.Decision{UserID: req.RequesterID}
		if status == "approved" {
			d.Change = services.Change{Writes: []services.Write{{OutboxID: "ob_1"}}}
			d.Approval = services.ApprovalPlan{Effect: services.EffectGive}
		}
		return d, nil
	}
	svcDeliverChange = func(_ context.Context, ch services.Change) services.Delivery {
		d := services.Delivery{Reason: map[string]string{}}
		for _, w := range ch.Writes {
			d.Sent = append(d.Sent, w.OutboxID)
		}
		return d
	}
	cacheRefresh = func(context.Context, []string, ...string) error { return nil }
	stubPlannedBulk(t, &writes)
	return &writes
}

// plannedBulkProbe is what a bulk surface handed the planner.
type plannedBulkProbe struct {
	rehearsed, applied services.BulkRequest
	cite               db.PlanCitation
}

// stubPlannedBulk stands in for the planner a bulk approval or adoption goes
// through (task 1.3): the rehearsal plans every named id as one Change, and
// the apply writes each with its own row "own-<id>", counted in writes. What
// the planner decides per id is services' tests' subject.
func stubPlannedBulk(t *testing.T, writes *int) *plannedBulkProbe {
	t.Helper()
	origRehearse, origApply, origRebuild := svcRehearseBulk, svcApplyPlannedBulk, rebuildUserCacheDetachedFn
	t.Cleanup(func() {
		svcRehearseBulk, svcApplyPlannedBulk, rebuildUserCacheDetachedFn = origRehearse, origApply, origRebuild
	})
	rebuildUserCacheDetachedFn = func(context.Context, string) {}
	p := &plannedBulkProbe{}
	rows := func(ids []string) []services.BulkOutcome {
		out := []services.BulkOutcome{}
		for _, id := range ids {
			out = append(out, services.BulkOutcome{UserID: id, Effect: services.EffectApply, Fingerprint: "fp:" + id})
		}
		return out
	}
	svcRehearseBulk = func(_ context.Context, req services.BulkRequest) (services.BulkPlan, error) {
		p.rehearsed = req
		plan := services.BulkPlan{Op: req.Op, Outcomes: rows(req.UserIDs)}
		plan.Summary = services.SummarizeOutcomes(plan.Outcomes)
		return plan, nil
	}
	svcApplyPlannedBulk = func(_ context.Context, req services.BulkRequest, c db.PlanCitation) (services.BulkPlan, map[string]services.Change, error) {
		p.applied, p.cite = req, c
		written := map[string]services.Change{}
		for _, id := range req.UserIDs {
			*writes++
			written[id] = services.Change{UserID: "user-" + id, Writes: []services.Write{{OutboxID: "own-" + id, Delivery: services.DeliveryStraightAway}}}
		}
		return services.BulkPlan{Op: req.Op, Applied: true, Outcomes: rows(req.UserIDs)}, written, nil
	}
	return p
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
	if plan.Applied || plan.Summary.Apply != 2 || plan.Op != "decide_requests" || plan.PlanID == "" {
		t.Errorf("expected an unapplied, issued plan of 2, got %+v", plan)
	}
}

// Approving goes through the planner (task 1.3): the rehearsal and the apply
// hand it the same request (each request id an approval, the reviewer, the
// note), and the apply cites this screen's plan, bound to the cohort and the
// decision. What each approval gives is the planner's (services' tests).
func TestBulkDecide_ApprovingGoesThroughThePlannerCitingThePlan(t *testing.T) {
	writes := stubRequestLookup(t, map[string]models.AccessRequest{
		"r1": pendingRequest("r1", "u1", "trained"),
		"r2": pendingRequest("r2", "u2", "operator"),
	})
	p := stubPlannedBulk(t, writes)
	decided := 0
	svcDecideRequest = func(context.Context, string, string, string, string, *services.Seen) (services.Decision, error) {
		decided++
		return services.Decision{}, nil
	}

	rr := httptest.NewRecorder()
	handleBulkDecideRequests(rr, bulkDecisionReq(t, `{"ids":["r1","r2"],"status":"approved","review_note":"term start"}`, true))
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rr.Code, rr.Body.String())
	}
	want := services.BulkRequest{Op: services.OpApprove, UserIDs: []string{"r1", "r2"}, Actor: "op_1", Note: "term start"}
	for name, got := range map[string]services.BulkRequest{"rehearsal": p.rehearsed, "apply": p.applied} {
		if got.Op != want.Op || strings.Join(got.UserIDs, ",") != "r1,r2" || got.Actor != want.Actor || got.Note != want.Note {
			t.Fatalf("the %s reached the planner as %+v, want %+v", name, got, want)
		}
	}
	fp := services.FingerprintIDCohort("decide_requests", []string{"r1", "r2"}, "status", "approved")
	if p.cite.Surface != planSurfaceBulkDecision || p.cite.PlanID == "" || p.cite.Actor != "op_1" || p.cite.RequestFingerprint != fp {
		t.Fatalf("the apply must cite this screen's plan for this cohort and decision: %+v", p.cite)
	}
	plan := decodeBulkPlan(t, rr)
	if plan.Op != "decide_requests" || plan.Summary.Succeeded != 2 || *writes != 2 {
		t.Fatalf("both approvals applied, reported as a decision: %+v (writes %d)", plan, *writes)
	}
	if decided != 0 {
		t.Fatalf("an approval is the planner's, never decided row by row: %d", decided)
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
	handleBulkDecideRequests(rr, bulkDecisionReq(t, `{"ids":["r1","r2"],"status":"rejected"}`, true))

	plan := decodeBulkPlan(t, rr)
	if plan.Summary.Succeeded != 1 || plan.Summary.NoChange != 1 {
		t.Fatalf("want 1 declined / 1 already-decided, got %+v", plan.Summary)
	}
	// Deciding it again would overwrite somebody else's decision.
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

// A decline names the role as every plan row does, never by its key.
func TestBulkDecide_DeclineNamesTheRole(t *testing.T) {
	stubRequestLookup(t, map[string]models.AccessRequest{"r1": pendingRequest("r1", "u1", "trained")})
	orig := services.PlannerCatalogue
	t.Cleanup(func() { services.PlannerCatalogue = orig })
	services.PlannerCatalogue = func(context.Context) ([]models.ProjectCatalog, error) {
		return []models.ProjectCatalog{{ID: "pLaser", Name: "Laser Lab", Roles: []models.ProjectRole{{Key: "trained", Label: "Trained"}}}}, nil
	}

	rr := httptest.NewRecorder()
	handleBulkDecideRequests(rr, bulkDecisionReq(t, `{"ids":["r1"],"status":"rejected"}`, false))

	plan := decodeBulkPlan(t, rr)
	if got := plan.Outcomes[0].Detail; got != "Their request for Trained in Laser Lab is declined." {
		t.Errorf("decline row = %q", got)
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
