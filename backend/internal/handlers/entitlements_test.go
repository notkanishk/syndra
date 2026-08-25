package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"syndra/internal/db"
	"syndra/internal/services"
	"syndra/internal/services/planapply"
)

// The entitlement convergence surface: what the approval it issues is made of.
//
// The rehearsal itself is tested where it lives. What is asserted here is the
// step between the two — that the diff an operator read becomes a record that
// can be cited exactly once, carrying the intent it was computed from, and that
// a plan against a stale read is stored as one.

type recordedPlans struct {
	created []db.NewPlan
	applied []planapply.Request
	err     error
}

func stubEntitlementPlan(t *testing.T, plan services.EntitlementPlan, rehearseErr error) *recordedPlans {
	t.Helper()
	rec := &recordedPlans{}
	origRehearse, origApply, origCreate := svcRehearseEntitlements, svcApplyEntitlements, dbCreatePlan
	t.Cleanup(func() {
		svcRehearseEntitlements, svcApplyEntitlements, dbCreatePlan = origRehearse, origApply, origCreate
	})

	svcRehearseEntitlements = func(context.Context, services.EntitlementRehearsal) (services.EntitlementPlan, error) {
		return plan, rehearseErr
	}
	svcApplyEntitlements = func(_ context.Context, req planapply.Request) (planapply.Result, error) {
		rec.applied = append(rec.applied, req)
		return planapply.Result{PlanID: req.PlanID, Target: req.Target}, rec.err
	}
	dbCreatePlan = func(_ context.Context, p db.NewPlan) (db.Plan, error) {
		rec.created = append(rec.created, p)
		return db.Plan{ID: "plan_1", Target: p.Target, Surface: p.Surface}, nil
	}
	return rec
}

func rehearsal(t *testing.T, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/targets/"+target+"/entitlements/rehearse", strings.NewReader(body))
	r.SetPathValue("target", target)
	rr := httptest.NewRecorder()
	handleRehearseEntitlements(rr, r)
	return rr
}

func onePersonPlan(effect string) services.EntitlementPlan {
	plan := services.EntitlementPlan{
		BulkPlan: services.BulkPlan{
			Op: services.EntitlementOp,
			Outcomes: []services.BulkOutcome{
				{UserID: "u1", Effect: effect, Detail: "Joins lab_makers.", Fingerprint: "fp-1"},
			},
		},
		StateReadAt: time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC),
		Desired: map[string]services.EntitlementSet{
			"u1": {SubjectID: "u1", Target: "truenas", Fields: map[string][]string{"group": {"lab_makers"}}},
		},
	}
	plan.Summary = services.SummarizeOutcomes(plan.Outcomes)
	return plan
}

// The approval carries the intent, not a reference to one computed later. A
// plan subject with no desired state behind it would be dispatched as an empty
// set, and an empty set removes every entitlement the subject has.
func TestTheApprovalCarriesTheResolvedSetItWasComputedFrom(t *testing.T) {
	rec := stubEntitlementPlan(t, onePersonPlan(services.EffectApply), nil)

	rr := rehearsal(t, "truenas", `{"subject_ids":["u1"]}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", rr.Code, rr.Body.String())
	}
	if len(rec.created) != 1 {
		t.Fatalf("want one plan written, got %d", len(rec.created))
	}
	written := rec.created[0]
	if written.Target != "truenas" || written.Surface != planSurfaceEntitlements {
		t.Errorf("plan written to the wrong place: target=%s surface=%s", written.Target, written.Surface)
	}
	if len(written.Subjects) != 1 {
		t.Fatalf("want one subject row, got %d", len(written.Subjects))
	}
	sub := written.Subjects[0]
	if sub.DesiredState == nil {
		t.Fatal("the subject row carries no desired state, so the drain would dispatch an empty set")
	}
	if string(sub.DesiredState["group"]) != `["lab_makers"]` {
		t.Errorf("the recorded intent is not the resolved one: %s", sub.DesiredState["group"])
	}
	// The lifecycle fields are part of the instruction and must be recorded even
	// though no mapping produces them.
	if _, present := sub.DesiredState["enabled"]; !present {
		t.Error("the recorded intent omits the lifecycle state")
	}
	if sub.Fingerprint != "fp-1" {
		t.Errorf("fingerprint = %q", sub.Fingerprint)
	}
	if sub.SnapshotID != "" {
		t.Error("the subject row must not cite a snapshot it did not write")
	}

	var out services.EntitlementPlan
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.PlanID != "plan_1" {
		t.Errorf("the rehearsal must return the citation it issued, got %q", out.PlanID)
	}
}

// A provisional plan carries no lifetime and must carry the age of the read it
// was computed against. Its gate is the re-fingerprint when the target returns,
// not a clock — expiring it would discard an approved change because an outage
// outlasted a timer the operator had no part in.
func TestAProvisionalApprovalIsStoredWithoutALifetime(t *testing.T) {
	plan := onePersonPlan(services.EffectApply)
	plan.Provisional = true
	plan.StateReadAt = time.Date(2026, 8, 8, 9, 0, 0, 0, time.UTC)
	rec := stubEntitlementPlan(t, plan, nil)

	if rr := rehearsal(t, "truenas", `{"subject_ids":["u1"]}`); rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", rr.Code, rr.Body.String())
	}
	written := rec.created[0]
	if !written.Provisional {
		t.Error("a plan computed from the mirror must be recorded as provisional")
	}
	if written.Lifetime != 0 {
		t.Errorf("a provisional plan must carry no lifetime, got %v", written.Lifetime)
	}
	if !written.StateReadAt.Equal(plan.StateReadAt) {
		t.Errorf("state read time = %v, want %v", written.StateReadAt, plan.StateReadAt)
	}
}

// A provisional plan with no read time is refused rather than stored. The store
// refuses it too; this refusal exists so the operator is told what is missing
// rather than being handed a validation error about a column.
func TestAProvisionalApprovalWithNoAgeIsRefused(t *testing.T) {
	plan := onePersonPlan(services.EffectApply)
	plan.Provisional, plan.StateReadAt = true, time.Time{}
	rec := stubEntitlementPlan(t, plan, nil)

	rr := rehearsal(t, "truenas", `{"subject_ids":["u1"]}`)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("want the rehearsal refused, got %d (%s)", rr.Code, rr.Body.String())
	}
	if len(rec.created) != 0 {
		t.Error("nothing may be recorded for a plan that cannot be dated")
	}
}

// The blast-radius guard counts what would CHANGE, not what was selected.
func TestTheBlastRadiusGuardCountsChangesAndNotSelections(t *testing.T) {
	t.Setenv("PLAN_COHORT_LIMIT", "2")

	plan := services.EntitlementPlan{
		BulkPlan:    services.BulkPlan{Op: services.EntitlementOp},
		StateReadAt: time.Now().UTC(),
		Desired:     map[string]services.EntitlementSet{},
	}
	for _, id := range []string{"u1", "u2", "u3"} {
		plan.Outcomes = append(plan.Outcomes, services.BulkOutcome{
			UserID: id, Effect: services.EffectApply, Fingerprint: "fp-" + id,
		})
		plan.Desired[id] = services.EntitlementSet{SubjectID: id, Target: "truenas"}
	}
	plan.Summary = services.SummarizeOutcomes(plan.Outcomes)
	rec := stubEntitlementPlan(t, plan, nil)

	rr := rehearsal(t, "truenas", `{"subject_ids":["u1","u2","u3"]}`)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %d (%s)", rr.Code, rr.Body.String())
	}
	if len(rec.created) != 0 {
		t.Error("an unacknowledged oversized change must not become an approval")
	}
	if !strings.Contains(rr.Body.String(), "COHORT_ACKNOWLEDGEMENT_REQUIRED") {
		t.Errorf("the refusal must name itself: %s", rr.Body.String())
	}

	// The same change, acknowledged, is recorded. Without this the guard could
	// be a refusal nobody can get past.
	rr = rehearsal(t, "truenas", `{"subject_ids":["u1","u2","u3"],"acknowledge_scope":true}`)
	if rr.Code != http.StatusOK || len(rec.created) != 1 {
		t.Fatalf("an acknowledged change must be approvable: %d (%s)", rr.Code, rr.Body.String())
	}
}

// Rows the plan said would not change are still recorded. A plan is the record
// of what an operator reviewed, and re-verifying a blocked row is meaningful:
// a subject blocked by a binding conflict then and resolved since is exactly
// the case the block existed for.
func TestUnchangedAndBlockedRowsAreStillPartOfTheApproval(t *testing.T) {
	plan := services.EntitlementPlan{
		BulkPlan:    services.BulkPlan{Op: services.EntitlementOp},
		StateReadAt: time.Now().UTC(),
		Desired: map[string]services.EntitlementSet{
			"u1": {SubjectID: "u1"}, "u2": {SubjectID: "u2"},
		},
	}
	plan.Outcomes = []services.BulkOutcome{
		{UserID: "u1", Effect: services.EffectNoChange, Fingerprint: "fp-1"},
		{UserID: "u2", Effect: services.EffectBlocked, Fingerprint: "fp-2"},
	}
	plan.Summary = services.SummarizeOutcomes(plan.Outcomes)
	rec := stubEntitlementPlan(t, plan, nil)

	if rr := rehearsal(t, "truenas", `{"subject_ids":["u1","u2"]}`); rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
	if got := len(rec.created[0].Subjects); got != 2 {
		t.Fatalf("want both rows recorded, got %d", got)
	}
}

// An apply with no citation is refused before anything is opened. This is the
// case the whole mechanism replaces: an apply that recomputes its own diff.
func TestAnApplyCitingNothingIsRefused(t *testing.T) {
	rec := stubEntitlementPlan(t, onePersonPlan(services.EffectApply), nil)

	r := httptest.NewRequest(http.MethodPost, "/api/v1/targets/truenas/entitlements/apply",
		strings.NewReader(`{"subject_ids":["u1"]}`))
	r.SetPathValue("target", "truenas")
	rr := httptest.NewRecorder()
	handleApplyEntitlements(rr, r)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d (%s)", rr.Code, rr.Body.String())
	}
	if len(rec.applied) != 0 {
		t.Error("the gate must not be reached by an apply that cites nothing")
	}
}

// The citation reaches the gate with every dimension the claim predicates on,
// untransposed. Two of these are strings that would swap silently.
func TestTheCitationReachesTheGateIntact(t *testing.T) {
	rec := stubEntitlementPlan(t, onePersonPlan(services.EffectApply), nil)

	r := httptest.NewRequest(http.MethodPost, "/api/v1/targets/truenas/entitlements/apply",
		strings.NewReader(`{"plan_id":"plan_1","subject_ids":["u1"]}`))
	r.SetPathValue("target", "truenas")
	rr := httptest.NewRecorder()
	handleApplyEntitlements(rr, r)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("want 202, got %d (%s)", rr.Code, rr.Body.String())
	}
	if len(rec.applied) != 1 {
		t.Fatalf("want one apply, got %d", len(rec.applied))
	}
	got := rec.applied[0]
	if got.PlanID != "plan_1" || got.Target != "truenas" || got.Surface != planSurfaceEntitlements {
		t.Errorf("citation arrived wrong: %+v", got)
	}
	// The cohort is bound by fingerprint, so an apply naming a different
	// selection loses in the database rather than being applied to it.
	if got.RequestFingerprint != services.FingerprintIDCohort(services.EntitlementOp, []string{"u1"}) {
		t.Errorf("the cohort binding does not match the one the rehearsal recorded: %q", got.RequestFingerprint)
	}
}

// The two unreachabilities are different operator actions and must not collapse
// into one status.
func TestAnUnreachableAddonAndADisabledTargetAnswerDifferently(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want int
		code string
	}{
		{"the add-on is down", services.ErrTargetUnplannable, http.StatusServiceUnavailable, "ADDON_UNREACHABLE"},
		{"the deployment dropped the target", db.ErrTargetNotActive, http.StatusConflict, "TARGET_NOT_ACTIVE"},
		{"not an add-on at all", services.ErrTargetIsBuiltIn, http.StatusBadRequest, "TARGET_NOT_AN_ADDON"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stubEntitlementPlan(t, services.EntitlementPlan{}, tc.err)
			rr := rehearsal(t, "truenas", `{"subject_ids":["u1"]}`)
			if rr.Code != tc.want {
				t.Fatalf("want %d, got %d (%s)", tc.want, rr.Code, rr.Body.String())
			}
			if !strings.Contains(rr.Body.String(), tc.code) {
				t.Errorf("want %s in the body: %s", tc.code, rr.Body.String())
			}
		})
	}
}

func TestARehearsalEffectVocabularyIsClosed(t *testing.T) {
	plan := onePersonPlan("applied")
	rec := stubEntitlementPlan(t, plan, nil)

	rr := rehearsal(t, "truenas", `{"subject_ids":["u1"]}`)
	if rr.Code == http.StatusOK {
		t.Fatal("a result must not be recordable as an approved effect: a plan states what will happen")
	}
	if len(rec.created) != 0 {
		t.Error("nothing may be recorded")
	}
}
