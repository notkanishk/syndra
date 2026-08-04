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
	"syndra/internal/services"
	"syndra/internal/services/propagation"
)

func bulkRequest(t *testing.T, body string, apply bool) *http.Request {
	t.Helper()
	path := "/api/v1/grants/bulk"
	if apply {
		path += "?apply=true"
	}
	return httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
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
	orig := svcRehearseBulk
	t.Cleanup(func() { svcRehearseBulk = orig })

	seen := &services.BulkRequest{}
	svcRehearseBulk = func(_ context.Context, req services.BulkRequest) (services.BulkPlan, error) {
		*seen = req
		// Return a copy: the handler mutates outcomes in place during apply and
		// must not corrupt the fixture across sub-tests.
		copied := plan
		copied.Outcomes = append([]services.BulkOutcome(nil), plan.Outcomes...)
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

// Apply recomputes the plan server-side. The client cannot hand back a doctored
// plan — or an honest but stale one — and have it executed.
func TestHandleBulkGrants_ApplyRerehearsesRatherThanTrustingTheClient(t *testing.T) {
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
		{"drain errors", propagation.DrainResult{}, errors.New("zitadel unreachable"), "zitadel unreachable"},
		{"zitadel offline halts with no error", propagation.DrainResult{Halted: true, Reason: "zitadel_offline"}, nil, "zitadel_offline"},
		{"another drain holds the lock", propagation.DrainResult{Halted: true, Reason: "drain_in_progress"}, nil, "drain_in_progress"},
		{"requeued rather than applied", propagation.DrainResult{Requeued: 1}, nil, "still queued"},
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
			if !strings.Contains(plan.Outcomes[0].Detail, tc.want) {
				t.Errorf("the row must say why it is queued, got %q", plan.Outcomes[0].Detail)
			}
			// Counted apart from success, so the headline cannot round it up.
			if plan.Summary.Queued != 1 || plan.Summary.Succeeded != 0 {
				t.Errorf("queued=%d succeeded=%d, want 1 and 0", plan.Summary.Queued, plan.Summary.Succeeded)
			}
		})
	}
}

// A bundle whose owner set it to manual confirmation is *meant* to leave work
// queued. That is not a failure, and it is not "applied" either.
func TestHandleBulkGrants_ManualBundleReportsQueued(t *testing.T) {
	stubRehearsal(t, services.BulkPlan{
		Op:       services.BulkOpAssignBundle,
		Outcomes: []services.BulkOutcome{{UserID: "u1", Effect: services.EffectApply}},
	})

	origCascade, origRebuild := svcCascadeBundleAssigned, rebuildUserCacheDetachedFn
	t.Cleanup(func() {
		svcCascadeBundleAssigned = origCascade
		rebuildUserCacheDetachedFn = origRebuild
	})
	rebuildUserCacheDetachedFn = func(context.Context, string) {}
	stubDrain(t)
	svcCascadeBundleAssigned = func(context.Context, string, string, string) (services.CascadeResult, error) {
		return services.CascadeResult{Enqueued: 3, Mode: "manual"}, nil
	}

	rr := httptest.NewRecorder()
	handleBulkGrants(rr, bulkRequest(t, `{"op":"assign_bundle","user_ids":["u1"],"bundle_id":"b1","reason":"x"}`, true))

	plan := decodePlan(t, rr)
	if plan.Outcomes[0].Effect != services.EffectQueued {
		t.Fatalf("effect = %q, want queued for a confirmation-gated bundle", plan.Outcomes[0].Effect)
	}
	if !strings.Contains(plan.Outcomes[0].Detail, "applies on confirmation") {
		t.Errorf("the row must name the bundle's own gate, got %q", plan.Outcomes[0].Detail)
	}
}

// An auto bundle whose cascade drained everything is applied, and a cascade with
// nothing to send is applied too — the closure was already correct.
func TestHandleBulkGrants_DrainedCascadeIsApplied(t *testing.T) {
	for _, tc := range []struct {
		name string
		res  services.CascadeResult
	}{
		{"auto bundle, fully drained", services.CascadeResult{Enqueued: 2, Mode: "auto", Drain: propagation.DrainResult{Applied: 2}}},
		{"nothing to propagate", services.CascadeResult{Enqueued: 0, Mode: "manual"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stubRehearsal(t, services.BulkPlan{
				Op:       services.BulkOpAssignBundle,
				Outcomes: []services.BulkOutcome{{UserID: "u1", Effect: services.EffectApply}},
			})

			origCascade, origRebuild := svcCascadeBundleAssigned, rebuildUserCacheDetachedFn
			t.Cleanup(func() {
				svcCascadeBundleAssigned = origCascade
				rebuildUserCacheDetachedFn = origRebuild
			})
			rebuildUserCacheDetachedFn = func(context.Context, string) {}
			stubDrain(t)
			svcCascadeBundleAssigned = func(context.Context, string, string, string) (services.CascadeResult, error) {
				return tc.res, nil
			}

			rr := httptest.NewRecorder()
			handleBulkGrants(rr, bulkRequest(t, `{"op":"assign_bundle","user_ids":["u1"],"bundle_id":"b1","reason":"x"}`, true))

			plan := decodePlan(t, rr)
			if plan.Outcomes[0].Effect != services.EffectApplied {
				t.Fatalf("effect = %q, want applied", plan.Outcomes[0].Effect)
			}
		})
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
