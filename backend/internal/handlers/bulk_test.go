package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
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

// stubDrain captures the outbox rows the handler projects upstream. Applying is
// not "wrote it down": every row a bulk apply enqueues has to reach Zitadel in
// the same request, exactly as the single-person handlers do on ?apply=true.
func stubDrain(t *testing.T) *[]string {
	t.Helper()
	orig := svcDrainPropagationRow
	t.Cleanup(func() { svcDrainPropagationRow = orig })

	var drained []string
	svcDrainPropagationRow = func(_ context.Context, id string) (propagation.DrainResult, error) {
		drained = append(drained, id)
		return propagation.DrainResult{Applied: 1}, nil
	}
	return &drained
}

func stubNoWrites(t *testing.T) *int {
	t.Helper()
	origEnqueue := dbEnqueueDirectGrantPropagation
	origRebuild := rebuildUserCacheDetachedFn
	t.Cleanup(func() {
		dbEnqueueDirectGrantPropagation = origEnqueue
		rebuildUserCacheDetachedFn = origRebuild
	})
	stubDrain(t)

	writes := 0
	dbEnqueueDirectGrantPropagation = func(context.Context, db.EnqueueParams) (db.EnqueueResult, error) {
		writes++
		return db.EnqueueResult{OutboxID: "ob_1", Status: "pending"}, nil
	}
	rebuildUserCacheDetachedFn = func(context.Context, string) {}
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

// Apply acts on the approval the backend issued, never on a cohort the client
// asserts. The selection still reaches the rehearsal verbatim — that is what is
// being reviewed — but what the apply executes is what was recorded.
func TestHandleBulkGrants_ApplyActsOnTheApprovalNotTheClientsClaim(t *testing.T) {
	seen := stubRehearsal(t, services.BulkPlan{
		Op:       services.BulkOpAssignRole,
		Outcomes: []services.BulkOutcome{{UserID: "u_server_says", Effect: services.EffectApply}},
	})
	writes := stubNoWrites(t)

	rr := httptest.NewRecorder()
	handleBulkGrants(rr, bulkRequest(t, `{"op":"assign_role","user_ids":["u_client_says"],"project_id":"pLaser","role_key":"trained","reason":"x"}`, true))

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d (%s)", rr.Code, rr.Body.String())
	}
	if len(seen.UserIDs) != 1 || seen.UserIDs[0] != "u_client_says" {
		t.Errorf("the selection must reach the rehearsal verbatim: %v", seen.UserIDs)
	}
	plan := decodePlan(t, rr)
	if len(plan.Outcomes) != 1 || plan.Outcomes[0].UserID != "u_server_says" {
		t.Errorf("apply must act on the server's plan, not the client's: %+v", plan.Outcomes)
	}
	if *writes != 1 {
		t.Errorf("expected exactly one write, got %d", *writes)
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

// One person's failure must not abort the batch. Stopping halfway leaves the
// operator unable to tell which half landed, which is worse than a partial
// result that names the failures.
func TestHandleBulkGrants_PartialFailureIsIsolatedAndReported(t *testing.T) {
	stubRehearsal(t, services.BulkPlan{
		Op: services.BulkOpAssignRole,
		Outcomes: []services.BulkOutcome{
			{UserID: "u_ok1", Effect: services.EffectApply},
			{UserID: "u_bad", Effect: services.EffectApply},
			{UserID: "u_ok2", Effect: services.EffectApply},
		},
	})

	origEnqueue := dbEnqueueDirectGrantPropagation
	origRebuild := rebuildUserCacheDetachedFn
	t.Cleanup(func() {
		dbEnqueueDirectGrantPropagation = origEnqueue
		rebuildUserCacheDetachedFn = origRebuild
	})

	stubDrain(t)
	var rebuilt []string
	rebuildUserCacheDetachedFn = func(_ context.Context, uid string) { rebuilt = append(rebuilt, uid) }
	dbEnqueueDirectGrantPropagation = func(_ context.Context, p db.EnqueueParams) (db.EnqueueResult, error) {
		if p.UserID == "u_bad" {
			return db.EnqueueResult{}, errors.New("ledger write rejected")
		}
		return db.EnqueueResult{OutboxID: "ob", Status: "pending"}, nil
	}

	rr := httptest.NewRecorder()
	handleBulkGrants(rr, bulkRequest(t, `{"op":"assign_role","user_ids":["u_ok1","u_bad","u_ok2"],"project_id":"p","role_key":"r","reason":"x"}`, true))

	if rr.Code != http.StatusAccepted {
		t.Fatalf("a partial failure is still an accepted batch, got %d", rr.Code)
	}
	plan := decodePlan(t, rr)
	byID := map[string]services.BulkOutcome{}
	for _, o := range plan.Outcomes {
		byID[o.UserID] = o
	}
	if byID["u_bad"].Effect != services.EffectFailed {
		t.Errorf("the failing row must be marked failed: %+v", byID["u_bad"])
	}
	if !strings.Contains(byID["u_bad"].Detail, "ledger write rejected") {
		t.Errorf("the failure must carry its cause: %q", byID["u_bad"].Detail)
	}
	if byID["u_ok1"].Effect != services.EffectApplied || byID["u_ok2"].Effect != services.EffectApplied {
		t.Error("a failure in the middle must not abort the rows around it")
	}
	if plan.Summary.Failed != 1 || plan.Summary.Succeeded != 2 {
		t.Errorf("summary must separate failures from successes: %+v", plan.Summary)
	}
	// A failed write leaves nothing to recompile; recompiling anyway would be
	// harmless but claiming it happened would not.
	if len(rebuilt) != 2 {
		t.Errorf("only successful rows recompile, got %v", rebuilt)
	}
}

// Removal acts on the grant ids the rehearsal identified, not on a fresh guess
// at which grant matches — the ledger may have moved since.
func TestHandleBulkGrants_RemoveActsOnRehearsedGrantIDs(t *testing.T) {
	stubRehearsal(t, services.BulkPlan{
		Op: services.BulkOpRemoveRole,
		Outcomes: []services.BulkOutcome{
			{UserID: "u1", Effect: services.EffectApply, GrantIDs: []string{"g_77"}},
		},
	})

	origDelete, origRebuild := svcDeleteDirectGrant, rebuildUserCacheDetachedFn
	t.Cleanup(func() {
		svcDeleteDirectGrant = origDelete
		rebuildUserCacheDetachedFn = origRebuild
	})
	rebuildUserCacheDetachedFn = func(context.Context, string) {}
	drained := stubDrain(t)

	var gotUser, gotGrant string
	svcDeleteDirectGrant = func(_ context.Context, uid, gid, _ string) (services.DirectGrantRemoval, error) {
		gotUser, gotGrant = uid, gid
		return services.DirectGrantRemoval{Status: "pending", OutboxIDs: []string{"ob_rm"}}, nil
	}

	rr := httptest.NewRecorder()
	handleBulkGrants(rr, bulkRequest(t, `{"op":"remove_role","user_ids":["u1"],"project_id":"p","role_key":"r","reason":"x"}`, true))

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d (%s)", rr.Code, rr.Body.String())
	}
	if gotUser != "u1" || gotGrant != "g_77" {
		t.Errorf("wrong grant removed: user=%q grant=%q", gotUser, gotGrant)
	}
	// The removal's own outbox rows must be projected, not left queued. Reporting
	// access as removed while the role is still live in Zitadel is the worst
	// version of this bug: the screen and the door disagree.
	if len(*drained) != 1 || (*drained)[0] != "ob_rm" {
		t.Errorf("removal must drain its own outbox rows, drained=%v", *drained)
	}
}

// Applying a bulk grant has to reach Zitadel in the same request. Enqueuing and
// stopping there marked every row "applied" while the roles sat in the
// governance queue, waiting for somebody to notice them.
func TestHandleBulkGrants_ApplyProjectsEachRowUpstream(t *testing.T) {
	stubRehearsal(t, services.BulkPlan{
		Op: services.BulkOpAssignRole,
		Outcomes: []services.BulkOutcome{
			{UserID: "u1", Effect: services.EffectApply},
			{UserID: "u2", Effect: services.EffectApply},
			{UserID: "u_blocked", Effect: services.EffectBlocked},
		},
	})

	origEnqueue, origRebuild := dbEnqueueDirectGrantPropagation, rebuildUserCacheDetachedFn
	t.Cleanup(func() {
		dbEnqueueDirectGrantPropagation = origEnqueue
		rebuildUserCacheDetachedFn = origRebuild
	})
	rebuildUserCacheDetachedFn = func(context.Context, string) {}
	drained := stubDrain(t)

	dbEnqueueDirectGrantPropagation = func(_ context.Context, p db.EnqueueParams) (db.EnqueueResult, error) {
		return db.EnqueueResult{OutboxID: "ob_" + p.UserID, Status: "pending"}, nil
	}

	rr := httptest.NewRecorder()
	handleBulkGrants(rr, bulkRequest(t, `{"op":"assign_role","user_ids":["u1","u2","u_blocked"],"project_id":"p","role_key":"r","reason":"x"}`, true))

	if len(*drained) != 2 || (*drained)[0] != "ob_u1" || (*drained)[1] != "ob_u2" {
		t.Fatalf("each applied row drains its own outbox row and a blocked row drains nothing, got %v", *drained)
	}
}

// A drain that does not land must not be reported as applied. The ledger write
// is committed and must stand — this is not a failure — but the role is still
// whatever it was in Zitadel, and a bulk revoke reading "Applied" while the
// access is live is the worst thing this screen can say.
//
// Both shapes of not-landing are covered. A drain returns an error OR comes back
// Halted with a nil error, and reading only the error treats an unreachable
// Zitadel as success.
func TestHandleBulkGrants_UnconfirmedDrainReportsQueuedNotApplied(t *testing.T) {
	for _, tc := range []struct {
		name string
		res  propagation.DrainResult
		err  error
		want string
	}{
		{"drain errors", propagation.DrainResult{}, errors.New("zitadel unreachable"), "halted:"},
		{"zitadel offline halts with no error", propagation.DrainResult{Halted: true, Reason: "zitadel_offline"}, nil, "halted:"},
		{"another drain holds the lock", propagation.DrainResult{Halted: true, Reason: "drain_in_progress"}, nil, "halted:"},
		{"requeued rather than applied", propagation.DrainResult{Requeued: 1}, nil, "halted:retrying"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stubRehearsal(t, services.BulkPlan{
				Op:       services.BulkOpAssignRole,
				Outcomes: []services.BulkOutcome{{UserID: "u1", Effect: services.EffectApply}},
			})

			origEnqueue, origRebuild, origDrain := dbEnqueueDirectGrantPropagation, rebuildUserCacheDetachedFn, svcDrainPropagationRow
			t.Cleanup(func() {
				dbEnqueueDirectGrantPropagation = origEnqueue
				rebuildUserCacheDetachedFn = origRebuild
				svcDrainPropagationRow = origDrain
			})
			rebuildUserCacheDetachedFn = func(context.Context, string) {}
			dbEnqueueDirectGrantPropagation = func(context.Context, db.EnqueueParams) (db.EnqueueResult, error) {
				return db.EnqueueResult{OutboxID: "ob_1", Status: "pending"}, nil
			}
			svcDrainPropagationRow = func(context.Context, string) (propagation.DrainResult, error) {
				return tc.res, tc.err
			}

			rr := httptest.NewRecorder()
			handleBulkGrants(rr, bulkRequest(t, `{"op":"assign_role","user_ids":["u1"],"project_id":"p","role_key":"r","reason":"x"}`, true))

			plan := decodePlan(t, rr)
			if plan.Outcomes[0].Effect != services.EffectQueued {
				t.Fatalf("effect = %q, want queued — the change has not reached Zitadel", plan.Outcomes[0].Effect)
			}
			// Why, as a reason the result reads (re-audit R2), and a sentence
			// in plain words: never the raw code, never an em dash.
			o := plan.Outcomes[0]
			if !strings.HasPrefix(o.WaitReason, tc.want) {
				t.Errorf("wait_reason = %q, want %q", o.WaitReason, tc.want)
			}
			if !strings.HasPrefix(o.Detail, "Recorded in Syndra") || strings.ContainsAny(o.Detail, "—_") {
				t.Errorf("the row must say why it is queued in plain words, got %q", o.Detail)
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
	origApply, origDeliver, origDeliverChange, origRebuild := svcApplyPlannedBulk, svcDeliver, svcDeliverChange, rebuildUserCacheDetachedFn
	t.Cleanup(func() {
		svcApplyPlannedBulk, svcDeliver, svcDeliverChange, rebuildUserCacheDetachedFn = origApply, origDeliver, origDeliverChange, origRebuild
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
	// The planned path delivers the Change as written, never by asking again
	// which roles wait.
	svcDeliver = func(context.Context, []string, bool) services.Delivery {
		t.Error("a planned apply must deliver its Change (DeliverChange), not ask Deliver")
		return services.Delivery{}
	}
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
	for _, tc := range []struct{ op, want string }{
		{services.BulkOpAssignBundle, "A role it gives always waits for you"},
		{services.BulkOpRemoveBundle, "A role it removes always waits for you"},
	} {
		t.Run(tc.op, func(t *testing.T) {
			cited := stubPlannedApply(t, tc.op, []string{"ob_1", "ob_2"}, nil, waits)

			rr := httptest.NewRecorder()
			handleBulkGrants(rr, bulkRequest(t, `{"op":"`+tc.op+`","user_ids":["u1"],"bundle_id":"b1","reason":"x"}`, true))

			plan := decodePlan(t, rr)
			if rr.Code != http.StatusAccepted || plan.Outcomes[0].Effect != services.EffectQueued {
				t.Fatalf("status %d, effect %q, want 202 and queued for a bundle whose role waits", rr.Code, plan.Outcomes[0].Effect)
			}
			if !strings.Contains(plan.Outcomes[0].Detail, tc.want) {
				t.Errorf("the row must say %q, got %q", tc.want, plan.Outcomes[0].Detail)
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

// A bundle whose role waits says so, rather than blaming the bundle's mode or
// calling it merely "still queued".
func TestSettleBulkRow_SaysWhyItIsQueued(t *testing.T) {
	out := services.BulkOutcome{Effect: services.EffectApply}
	settleBulkRow(&out, db.WaitReasonRoleWaits, nil, services.BulkOpAssignBundle)
	if out.Effect != services.EffectQueued || !strings.Contains(out.Detail, "A role it gives always waits for you") {
		t.Fatalf("role waits: %+v", out)
	}
	out = services.BulkOutcome{Effect: services.EffectApply}
	settleBulkRow(&out, "", nil, services.BulkOpRemoveBundle)
	if out.Effect != services.EffectApplied {
		t.Fatalf("everything sent is applied: %+v", out)
	}
	// A send Zitadel didn't take is retrying, with its next try, and says it
	// goes on its own (re-audit R2).
	next := &extcall.RetryState{State: extcall.RetryRetrying, Try: 2, Of: 6}
	out = services.BulkOutcome{Effect: services.EffectApply}
	settleBulkRow(&out, "halted:retrying", next, services.BulkOpAssignBundle)
	if out.Retry != next || !strings.Contains(out.Detail, "sends it again on its own") {
		t.Fatalf("retrying: %+v", out)
	}
	// Refused by Zitadel is terminal and failed, never queued (N1).
	out = services.BulkOutcome{Effect: services.EffectApply}
	settleBulkRow(&out, db.WaitReasonRefused(db.RefusalNotFound), nil, services.BulkOpRemoveBundle)
	if out.Effect != services.EffectFailed || !strings.Contains(out.Detail, "Drift") {
		t.Fatalf("refused removal: %+v", out)
	}
	// With the background drain off, nothing sends it on its own.
	propagation.SetAutoRetry(false)
	t.Cleanup(func() { propagation.SetAutoRetry(true) })
	if d := queuedDetail("halted:retrying", false); !strings.Contains(d, "until you send it") {
		t.Fatalf("drain off: %q", d)
	}
}
