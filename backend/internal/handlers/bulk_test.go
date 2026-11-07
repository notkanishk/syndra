package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"syndra/internal/db"
	"syndra/internal/extcall"
	"syndra/internal/services"
	"syndra/internal/services/propagation"
)

// bulkRequest builds the request a test makes. An apply is now the SECOND of
// two requests — it cites an approval — so the helper performs the rehearsal
// and splices its plan id in, exactly as a client must. The tests below stay
// about what the apply does; how it is authorised is asserted in
// plan_gate_test.go.
func bulkRequest(t *testing.T, body string, apply bool) *http.Request {
	t.Helper()
	const path = "/api/v1/grants/bulk"
	if !apply {
		return httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	}

	rehearse := httptest.NewRecorder()
	handleBulkGrants(rehearse, httptest.NewRequest(http.MethodPost, path, strings.NewReader(body)))
	var issued services.BulkPlan
	if err := json.Unmarshal(rehearse.Body.Bytes(), &issued); err != nil || issued.PlanID == "" {
		// A body the rehearsal refuses has no plan to cite, and refusing it on
		// the apply path is exactly what the validation tests below assert.
		return httptest.NewRequest(http.MethodPost, path+"?apply=true", strings.NewReader(body))
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	payload["plan_id"] = issued.PlanID
	cited, _ := json.Marshal(payload)
	return httptest.NewRequest(http.MethodPost, path+"?apply=true", strings.NewReader(string(cited)))
}

func decodePlan(t *testing.T, rr *httptest.ResponseRecorder) services.BulkPlan {
	t.Helper()
	var plan services.BulkPlan
	if err := json.Unmarshal(rr.Body.Bytes(), &plan); err != nil {
		t.Fatalf("decode plan: %v (%s)", err, rr.Body.String())
	}
	return plan
}

// stubRehearsal installs a fixed plan and returns a pointer to the request the
// handler passed through, so tests can assert the handler forwards what the
// client asked for rather than something it made up.
func stubRehearsal(t *testing.T, plan services.BulkPlan) *services.BulkRequest {
	t.Helper()
	// Every rehearsal is now recorded, so a test that stubs one needs somewhere
	// for it to land. Reuses the store the test already installed, if any.
	stubPlanStore(t)
	orig := svcRehearseBulk
	t.Cleanup(func() { svcRehearseBulk = orig })

	seen := &services.BulkRequest{}
	svcRehearseBulk = func(_ context.Context, req services.BulkRequest) (services.BulkPlan, error) {
		*seen = req
		// Return a copy: the handler mutates outcomes in place during apply and
		// must not corrupt the fixture across sub-tests.
		copied := plan
		copied.Outcomes = append([]services.BulkOutcome(nil), plan.Outcomes...)
		for i := range copied.Outcomes {
			// A fixture with no fingerprint would be refused at the plan store,
			// and every one of these tests is about what happens after it is
			// accepted. Derived from the subject so the apply's re-read agrees
			// with the rehearsal — a test that wants them to DISAGREE swaps the
			// rehearsal itself.
			if copied.Outcomes[i].Fingerprint == "" {
				copied.Outcomes[i].Fingerprint = "fp:" + copied.Outcomes[i].UserID
			}
		}
		return copied, nil
	}
	return seen
}

// stubNoWrites stands in for the planner's apply (services.ApplyPlannedBulk)
// as the protocol it keeps with the plan store: it claims the cited plan
// (stubPlanStore enforces surface, operator, request, once, lifetime),
// verifying each approved subject against a fresh rehearsal before the
// approval is spent, so a stale plan is refused unspent with nothing written;
// then it writes each approved row, one outbox row "ob_<id>". How the real
// apply re-plans and writes under the lock is services' tests' subject. Each
// row it writes is delivered whole (svcDeliverChange) unless a test says
// otherwise. The count it returns is the rows written.
func stubNoWrites(t *testing.T) *int {
	t.Helper()
	stubPlanStore(t)
	origApply, origRebuild := svcApplyPlannedBulk, rebuildUserCacheDetachedFn
	origDeliver, origStatus := svcDeliverChange, dbGetPropagationStatus
	t.Cleanup(func() {
		svcApplyPlannedBulk, rebuildUserCacheDetachedFn = origApply, origRebuild
		svcDeliverChange, dbGetPropagationStatus = origDeliver, origStatus
	})
	rebuildUserCacheDetachedFn = func(context.Context, string) {}
	dbGetPropagationStatus = func(context.Context, string) (string, error) { return "applied", nil }
	svcDeliverChange = func(_ context.Context, ch services.Change) services.Delivery {
		var ids []string
		for _, w := range ch.Writes {
			ids = append(ids, w.OutboxID)
		}
		return services.Delivery{Sent: ids, Drain: propagation.DrainResult{Applied: len(ids)}}
	}

	writes := 0
	svcApplyPlannedBulk = func(ctx context.Context, req services.BulkRequest, c db.PlanCitation) (services.BulkPlan, map[string]services.Change, error) {
		var live map[string]services.BulkOutcome
		_, subjects, err := dbClaimPlanVerified(ctx, c, func(rows []db.PlanSubject) error {
			fresh, err := svcRehearseBulk(ctx, req)
			if err != nil {
				return err
			}
			live = indexOutcomes(fresh.Outcomes)
			var moved []string
			for _, row := range rows {
				if o, ok := live[row.SubjectID]; !ok || o.Fingerprint != row.Fingerprint {
					moved = append(moved, row.SubjectID)
				}
			}
			if len(moved) > 0 {
				sort.Strings(moved)
				return &services.StalePlan{IDs: moved}
			}
			return nil
		})
		if err != nil {
			return services.BulkPlan{}, nil, err
		}
		plan := planApprovedRows(req.Op, subjects, live)
		written := map[string]services.Change{}
		for _, o := range plan.Outcomes {
			if o.Effect == services.EffectApply {
				writes++
				written[o.UserID] = services.Change{UserID: o.UserID, Writes: []services.Write{
					{OutboxID: "ob_" + o.UserID, Delivery: services.DeliveryStraightAway}}}
			}
		}
		return plan, written, nil
	}
	return &writes
}

// The default is a rehearsal. A POST with no ?apply must compute the plan and
// write nothing — this is the property the whole confirmation flow rests on.
func TestHandleBulkGrants_DefaultsToRehearsalAndWritesNothing(t *testing.T) {
	stubRehearsal(t, services.BulkPlan{
		Op: services.BulkOpAssignRole,
		Outcomes: []services.BulkOutcome{
			{UserID: "u1", Effect: services.EffectApply},
			{UserID: "u2", Effect: services.EffectApply},
		},
	})
	writes := stubNoWrites(t)

	rr := httptest.NewRecorder()
	handleBulkGrants(rr, bulkRequest(t, `{"op":"assign_role","user_ids":["u1","u2"],"project_id":"pLaser","role_key":"trained","reason":"cohort"}`, false))

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for a rehearsal, got %d (%s)", rr.Code, rr.Body.String())
	}
	if *writes != 0 {
		t.Errorf("a rehearsal must not write, got %d writes", *writes)
	}
	if plan := decodePlan(t, rr); plan.Applied {
		t.Error("a rehearsal must not report itself as applied")
	}
}

// Every bulk op applies through the planner (task 1.3): the request reaches
// the rehearsal and the apply as the client sent it, and the apply cites this
// screen's plan bound to that request.
func TestHandleBulkGrants_EveryOpAppliesThroughThePlannerCitingThePlan(t *testing.T) {
	for op, body := range map[string]string{
		services.BulkOpAssignRole:   `{"op":"assign_role","user_ids":["u1"],"project_id":"p","role_key":"r","reason":"x","duration_days":30}`,
		services.BulkOpRemoveRole:   `{"op":"remove_role","user_ids":["u1"],"project_id":"p","role_key":"r","reason":"x"}`,
		services.BulkOpExtend:       `{"op":"extend","user_ids":["u1"],"reason":"x","duration_days":30,"grant_ids":["g1"]}`,
		services.BulkOpAssignBundle: `{"op":"assign_bundle","user_ids":["u1"],"bundle_id":"b1","reason":"x"}`,
		services.BulkOpRemoveBundle: `{"op":"remove_bundle","user_ids":["u1"],"bundle_id":"b1","reason":"x"}`,
	} {
		t.Run(op, func(t *testing.T) {
			cited := stubPlannedApply(t, op, []string{"ob_1"}, nil, func(ids []string) services.Delivery {
				return services.Delivery{Sent: ids, Drain: propagation.DrainResult{Applied: len(ids)}}
			})
			seen := stubRehearsal(t, services.BulkPlan{Op: op, Outcomes: []services.BulkOutcome{{UserID: "u1", Effect: services.EffectApply}}})
			origStatus := dbGetPropagationStatus
			t.Cleanup(func() { dbGetPropagationStatus = origStatus })
			dbGetPropagationStatus = func(context.Context, string) (string, error) { return "applied", nil }
			var applied services.BulkRequest
			stubbed := svcApplyPlannedBulk
			svcApplyPlannedBulk = func(ctx context.Context, req services.BulkRequest, c db.PlanCitation) (services.BulkPlan, map[string]services.Change, error) {
				applied = req
				return stubbed(ctx, req, c)
			}

			rr := httptest.NewRecorder()
			handleBulkGrants(rr, bulkRequest(t, body, true))
			if rr.Code != http.StatusAccepted {
				t.Fatalf("expected 202, got %d (%s)", rr.Code, rr.Body.String())
			}
			var want BulkGrantRequest
			_ = json.Unmarshal([]byte(body), &want)
			input := services.BulkRequest{Op: want.Op, UserIDs: want.UserIDs, ProjectID: want.ProjectID, RoleKey: want.RoleKey,
				BundleID: want.BundleID, Reason: want.Reason, DurationDays: want.DurationDays, GrantIDs: want.GrantIDs, Actor: "system"}
			for name, got := range map[string]services.BulkRequest{"rehearsal": *seen, "apply": applied} {
				if services.FingerprintBulkRequest(got) != services.FingerprintBulkRequest(input) || got.Actor != "system" {
					t.Fatalf("the %s reached the planner as %+v, want %+v", name, got, input)
				}
			}
			if cited.Surface != planSurfaceBulkGrants || cited.PlanID == "" || cited.RequestFingerprint != services.FingerprintBulkRequest(input) {
				t.Fatalf("the apply must cite this screen's plan for this request: %+v", *cited)
			}
			if plan := decodePlan(t, rr); plan.Outcomes[0].Effect != services.EffectApplied {
				t.Fatalf("a row whose write reached Zitadel is applied: %+v", plan.Outcomes)
			}
		})
	}
}

// Blocked and no-change rows are decisions the rehearsal already made. Apply
// must not quietly execute them — a departed account that was refused in the
// preview cannot be granted access by pressing the confirm button.
func TestHandleBulkGrants_ApplySkipsBlockedAndNoChangeRows(t *testing.T) {
	stubRehearsal(t, services.BulkPlan{
		Op: services.BulkOpAssignRole,
		Outcomes: []services.BulkOutcome{
			{UserID: "u_blocked", Effect: services.EffectBlocked, Detail: "Account is departed."},
			{UserID: "u_same", Effect: services.EffectNoChange},
			{UserID: "u_go", Effect: services.EffectApply},
		},
	})
	writes := stubNoWrites(t)

	rr := httptest.NewRecorder()
	handleBulkGrants(rr, bulkRequest(t, `{"op":"assign_role","user_ids":["u_blocked","u_same","u_go"],"project_id":"p","role_key":"r","reason":"x"}`, true))

	if *writes != 1 {
		t.Fatalf("only the actionable row may be written, got %d writes", *writes)
	}
	plan := decodePlan(t, rr)
	byID := map[string]services.BulkOutcome{}
	for _, o := range plan.Outcomes {
		byID[o.UserID] = o
	}
	if byID["u_blocked"].Effect != services.EffectBlocked {
		t.Errorf("a blocked row must stay blocked in the result: %+v", byID["u_blocked"])
	}
	if byID["u_blocked"].Detail != "Account is departed." {
		t.Errorf("the refusal reason must survive into the result: %q", byID["u_blocked"].Detail)
	}
	if byID["u_same"].Effect != services.EffectNoChange {
		t.Errorf("a no-change row must stay unchanged: %+v", byID["u_same"])
	}
	if byID["u_go"].Effect != services.EffectApplied {
		t.Errorf("the executed row must report applied: %+v", byID["u_go"])
	}
	if plan.Summary.Succeeded != 1 || plan.Summary.Blocked != 1 || plan.Summary.NoChange != 1 {
		t.Errorf("the summary must be recounted after apply: %+v", plan.Summary)
	}
}

// Applying a bulk grant has to reach Zitadel in the same request. Each row the
// planner wrote is recompiled and sent as its Change decided; a blocked row
// was written by nobody and sends nothing.
func TestHandleBulkGrants_ApplySendsEachRowItWrote(t *testing.T) {
	stubRehearsal(t, services.BulkPlan{
		Op: services.BulkOpAssignRole,
		Outcomes: []services.BulkOutcome{
			{UserID: "u1", Effect: services.EffectApply},
			{UserID: "u2", Effect: services.EffectApply},
			{UserID: "u_blocked", Effect: services.EffectBlocked},
		},
	})
	stubNoWrites(t)
	var sent, rebuilt []string
	svcDeliverChange = func(_ context.Context, ch services.Change) services.Delivery {
		sent = append(sent, ch.Writes[0].OutboxID)
		return services.Delivery{Sent: []string{ch.Writes[0].OutboxID}, Drain: propagation.DrainResult{Applied: 1}}
	}
	rebuildUserCacheDetachedFn = func(_ context.Context, uid string) { rebuilt = append(rebuilt, uid) }

	rr := httptest.NewRecorder()
	handleBulkGrants(rr, bulkRequest(t, `{"op":"assign_role","user_ids":["u1","u2","u_blocked"],"project_id":"p","role_key":"r","reason":"x"}`, true))

	if strings.Join(sent, ",") != "ob_u1,ob_u2" || strings.Join(rebuilt, ",") != "u1,u2" {
		t.Fatalf("each applied row is recompiled and sent and a blocked row neither: sent %v rebuilt %v", sent, rebuilt)
	}
}

// A send that does not land must not be reported as applied. The write is
// committed and stands (this is not a failure), but the role is still whatever
// it was in Zitadel, and a bulk revoke reading "Applied" while the access is
// live is the worst thing this screen can say.
func TestHandleBulkGrants_UnconfirmedDeliveryReportsQueuedNotApplied(t *testing.T) {
	for _, tc := range []struct {
		name  string
		drain propagation.DrainResult
		want  string
	}{
		{"zitadel unreachable", propagation.DrainResult{Halted: true, Reason: "zitadel unreachable"}, "halted:"},
		{"zitadel offline", propagation.DrainResult{Halted: true, Reason: "zitadel_offline"}, "halted:"},
		{"another drain holds the lock", propagation.DrainResult{Halted: true, Reason: "drain_in_progress"}, "halted:"},
		{"requeued rather than applied", propagation.DrainResult{Requeued: 1}, "halted:retrying"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stubRehearsal(t, services.BulkPlan{
				Op:       services.BulkOpAssignRole,
				Outcomes: []services.BulkOutcome{{UserID: "u1", Effect: services.EffectApply}},
			})
			stubNoWrites(t)
			svcDeliverChange = func(_ context.Context, ch services.Change) services.Delivery {
				return services.Delivery{Sent: []string{ch.Writes[0].OutboxID}, Drain: tc.drain}
			}

			rr := httptest.NewRecorder()
			handleBulkGrants(rr, bulkRequest(t, `{"op":"assign_role","user_ids":["u1"],"project_id":"p","role_key":"r","reason":"x"}`, true))

			plan := decodePlan(t, rr)
			if plan.Outcomes[0].Effect != services.EffectQueued {
				t.Fatalf("effect = %q, want queued: the change has not reached Zitadel", plan.Outcomes[0].Effect)
			}
			// Why, as a reason the result reads (re-audit R2); the row keeps
			// the plan's words, and the screen's fold says the state (A-21
			// §2.6), so no raw code and no generic sentence replaces them.
			o := plan.Outcomes[0]
			if !strings.HasPrefix(o.WaitReason, tc.want) {
				t.Errorf("wait_reason = %q, want %q", o.WaitReason, tc.want)
			}
			if strings.HasPrefix(o.Detail, "Recorded in Syndra") || strings.ContainsAny(o.Detail, "—_") {
				t.Errorf("the row must keep the plan's words, got %q", o.Detail)
			}
			// Counted apart from success, so the headline cannot round it up.
			if plan.Summary.Queued != 1 || plan.Summary.Succeeded != 0 {
				t.Errorf("queued=%d succeeded=%d, want 1 and 0", plan.Summary.Queued, plan.Summary.Succeeded)
			}
		})
	}
}

// stubPlannedApply stands in for the planner's apply (bundle ops) and records
// the citation the handler passed, and stubs what delivery answers.
func stubPlannedApply(t *testing.T, op string, sent []string, applyErr error, deliver func([]string) services.Delivery) *db.PlanCitation {
	t.Helper()
	stubRehearsal(t, services.BulkPlan{Op: op, Outcomes: []services.BulkOutcome{{UserID: "u1", Effect: services.EffectApply}}})
	origApply, origDeliverChange, origRebuild := svcApplyPlannedBulk, svcDeliverChange, rebuildUserCacheDetachedFn
	t.Cleanup(func() {
		svcApplyPlannedBulk, svcDeliverChange, rebuildUserCacheDetachedFn = origApply, origDeliverChange, origRebuild
	})
	rebuildUserCacheDetachedFn = func(context.Context, string) {}
	cited := &db.PlanCitation{}
	written := services.Change{UserID: "u1"}
	for _, id := range sent {
		written.Writes = append(written.Writes, services.Write{Delivery: services.DeliveryStraightAway, OutboxID: id})
	}
	svcApplyPlannedBulk = func(_ context.Context, req services.BulkRequest, c db.PlanCitation) (services.BulkPlan, map[string]services.Change, error) {
		*cited = c
		if applyErr != nil {
			return services.BulkPlan{}, nil, applyErr
		}
		return services.BulkPlan{Op: req.Op, Applied: true, Outcomes: []services.BulkOutcome{
			{UserID: "u1", Effect: services.EffectApply, Detail: "Joins the Safety bundle."},
		}}, map[string]services.Change{"u1": written}, nil
	}
	// The planned path delivers the Change as written (DeliverChange, the one delivery).
	svcDeliverChange = func(_ context.Context, ch services.Change) services.Delivery {
		var ids []string
		for _, w := range ch.Writes {
			ids = append(ids, w.OutboxID)
		}
		return deliver(ids)
	}
	return cited
}

// A bundle carrying a role that waits is *meant* to leave work queued. That is
// not a failure, and it is not "applied" either; and a removal says the role
// it removes waits, not one it gives.
func TestHandleBulkGrants_WaitingBundleReportsQueued(t *testing.T) {
	waits := func(ids []string) services.Delivery {
		d := services.Delivery{Waiting: ids, Reason: map[string]string{}}
		for _, id := range ids {
			d.Reason[id] = db.WaitReasonRoleWaits
		}
		return d
	}
	for _, op := range []string{services.BulkOpAssignBundle, services.BulkOpRemoveBundle} {
		t.Run(op, func(t *testing.T) {
			cited := stubPlannedApply(t, op, []string{"ob_1", "ob_2"}, nil, waits)

			rr := httptest.NewRecorder()
			handleBulkGrants(rr, bulkRequest(t, `{"op":"`+op+`","user_ids":["u1"],"bundle_id":"b1","reason":"x"}`, true))

			plan := decodePlan(t, rr)
			if rr.Code != http.StatusAccepted || plan.Outcomes[0].Effect != services.EffectQueued {
				t.Fatalf("status %d, effect %q, want 202 and queued for a bundle whose role waits", rr.Code, plan.Outcomes[0].Effect)
			}
			// The reason says why; the row keeps the plan's words (A-21 §2.6).
			if o := plan.Outcomes[0]; o.WaitReason != db.WaitReasonRoleWaits || o.Detail != "Joins the Safety bundle." {
				t.Errorf("the row must wait for role_waits and keep its words, got %q %q", o.WaitReason, o.Detail)
			}
			// The citation the planner claims is the one claimPlan would have.
			if cited.Surface != planSurfaceBulkGrants || cited.PlanID == "" || cited.RequestFingerprint == "" {
				t.Errorf("citation = %+v", *cited)
			}
		})
	}
}

// A planned row whose rows all reached Zitadel is applied, and so is one with
// nothing to send (the closure was already right).
func TestHandleBulkGrants_DeliveredPlannedRowIsApplied(t *testing.T) {
	sentAll := func(ids []string) services.Delivery {
		return services.Delivery{Sent: ids, Drain: propagation.DrainResult{Applied: len(ids)}}
	}
	for name, sent := range map[string][]string{"sent": {"ob_1"}, "nothing to send": nil} {
		t.Run(name, func(t *testing.T) {
			stubPlannedApply(t, services.BulkOpAssignBundle, sent, nil, sentAll)
			rr := httptest.NewRecorder()
			handleBulkGrants(rr, bulkRequest(t, `{"op":"assign_bundle","user_ids":["u1"],"bundle_id":"b1","reason":"x"}`, true))
			if plan := decodePlan(t, rr); plan.Outcomes[0].Effect != services.EffectApplied || plan.Summary.Succeeded != 1 {
				t.Fatalf("effect = %q succeeded = %d, want applied", plan.Outcomes[0].Effect, plan.Summary.Succeeded)
			}
		})
	}
}

// A plan the planner found stale is the same 409 every surface answers, with
// the people who moved, and nothing is delivered: nothing was written.
func TestHandleBulkGrants_StalePlannedPlanIsRefused(t *testing.T) {
	delivered := 0
	stubPlannedApply(t, services.BulkOpRemoveBundle, nil, &services.StalePlan{IDs: []string{"u1"}},
		func([]string) services.Delivery { delivered++; return services.Delivery{} })

	rr := httptest.NewRecorder()
	handleBulkGrants(rr, bulkRequest(t, `{"op":"remove_bundle","user_ids":["u1"],"bundle_id":"b1","reason":"x"}`, true))

	var body ErrorResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if rr.Code != http.StatusConflict || body.Error != "PLAN_STALE" || body.Details["u1"] != "moved" {
		t.Fatalf("status %d body %s, want 409 PLAN_STALE naming u1", rr.Code, rr.Body.String())
	}
	if delivered != 0 {
		t.Errorf("a stale plan delivered %d time(s)", delivered)
	}
}

func TestHandleBulkGrants_RejectsInvalidRequests(t *testing.T) {
	writes := stubNoWrites(t)

	cases := []struct{ name, body string }{
		{"unknown op", `{"op":"nuke","user_ids":["u1"]}`},
		{"empty selection", `{"op":"assign_role","user_ids":[],"project_id":"p","role_key":"r"}`},
		{"role op missing role", `{"op":"assign_role","user_ids":["u1"],"project_id":"p"}`},
		{"extend by zero", `{"op":"extend","user_ids":["u1"],"duration_days":0}`},
		{"unknown field", `{"op":"assign_role","user_ids":["u1"],"project_id":"p","role_key":"r","admin":true}`},
		{"malformed json", `{"op":`},
		// The dialog requires a reason, but the endpoint is what makes it true.
		// A direct caller must not be able to move dozens of people's access
		// and leave an audit trail that says nothing about why.
		{"missing reason", `{"op":"assign_role","user_ids":["u1"],"project_id":"p","role_key":"r"}`},
		{"blank reason", `{"op":"assign_role","user_ids":["u1"],"project_id":"p","role_key":"r","reason":"   "}`},
		{"blank reason on remove", `{"op":"remove_role","user_ids":["u1"],"project_id":"p","role_key":"r","reason":""}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			handleBulkGrants(rr, bulkRequest(t, tc.body, true))
			if rr.Code != http.StatusBadRequest && rr.Code != http.StatusUnprocessableEntity {
				t.Errorf("expected a validation failure, got %d (%s)", rr.Code, rr.Body.String())
			}
		})
	}
	if *writes != 0 {
		t.Errorf("a rejected request must not write, got %d", *writes)
	}
}

// Settling sets the state and keeps the row's plan words (A-21 §2.6): the row
// that said "Giving Trained waits for you to send." before Apply says it
// after, and the result's fold and the row's seat say the state.
func TestSettleBulkRow_KeepsThePlanWordsAndSetsTheState(t *testing.T) {
	const words = "Gains Member in Laser Lab and Trained in Laser Lab. Giving Trained in Laser Lab waits for you to send."
	planned := func() services.BulkOutcome {
		return services.BulkOutcome{Effect: services.EffectApply, Detail: "Joins Safety.", Consequence: words}
	}
	out := planned()
	settleBulkRow(&out, db.WaitReasonRoleWaits, nil)
	if out.Effect != services.EffectQueued || out.WaitReason != db.WaitReasonRoleWaits {
		t.Fatalf("role waits: %+v", out)
	}
	out = planned()
	settleBulkRow(&out, "", nil)
	if out.Effect != services.EffectApplied || out.WaitReason != "" {
		t.Fatalf("everything sent is applied: %+v", out)
	}
	// A send Zitadel didn't take is retrying, with its next try (re-audit R2).
	next := &extcall.RetryState{State: extcall.RetryRetrying, Try: 2, Of: 6}
	out = planned()
	settleBulkRow(&out, "halted:retrying", next)
	if out.Effect != services.EffectQueued || out.Retry != next {
		t.Fatalf("retrying: %+v", out)
	}
	// Refused by Zitadel is terminal and failed, never queued (N1).
	out = planned()
	settleBulkRow(&out, db.WaitReasonRefused(db.RefusalNotFound), next)
	if out.Effect != services.EffectFailed || out.WaitReason != db.WaitReasonRefused(db.RefusalNotFound) || out.Retry != nil {
		t.Fatalf("refused: %+v", out)
	}
	// No state overwrites what the plan said.
	for _, reason := range []string{"", db.WaitReasonRoleWaits, "halted:retrying", db.WaitReasonRefused(db.RefusalNotAllowed)} {
		out = planned()
		settleBulkRow(&out, reason, next)
		if out.Detail != "Joins Safety." || out.Consequence != words {
			t.Fatalf("%q overwrote the plan's words: %+v", reason, out)
		}
	}
}
