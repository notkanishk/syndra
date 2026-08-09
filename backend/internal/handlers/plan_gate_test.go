package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"syndra/internal/db"
	"syndra/internal/services"
)

// 3.1–3.7 — plan-then-apply as a backend guarantee.
//
// The store is faked rather than mocked away: the properties under test are
// about what a citation must match and what an apply may act on, and a fake
// that accepts every citation would let all of them pass.

type storedPlan struct {
	meta     db.Plan
	subjects []db.PlanSubject
}

type fakePlanStore struct {
	plans   map[string]*storedPlan
	created []db.NewPlan
	claims  int
	next    int
}

// activePlanStore is the fake installed for the running test, so a helper that
// needs one can reuse it instead of shadowing the store the test is holding.
var activePlanStore *fakePlanStore

// stubPlanStore installs an in-memory plan store that enforces every rule the
// real one does: one apply per approval, the issuing surface, the approving
// operator, the bound request, and the lifetime.
func stubPlanStore(t *testing.T) *fakePlanStore {
	t.Helper()
	if activePlanStore != nil {
		return activePlanStore
	}
	origCreate, origClaim := dbCreatePlan, dbClaimPlanVerified
	t.Cleanup(func() { dbCreatePlan, dbClaimPlanVerified = origCreate, origClaim; activePlanStore = nil })

	store := &fakePlanStore{plans: map[string]*storedPlan{}}
	activePlanStore = store

	dbCreatePlan = func(_ context.Context, p db.NewPlan) (db.Plan, error) {
		for _, s := range p.Subjects {
			if strings.TrimSpace(s.Fingerprint) == "" {
				return db.Plan{}, fmt.Errorf("%w: subject %s has no fingerprint", db.ErrInvalidPlan, s.SubjectID)
			}
		}
		store.next++
		id := fmt.Sprintf("plan_%d", store.next)
		expires := time.Now().Add(p.Lifetime)
		rec := &storedPlan{meta: db.Plan{
			ID: id, Target: p.Target, Surface: p.Surface, CreatedBy: p.CreatedBy,
			ExpiresAt: &expires, RequestFingerprint: p.RequestFingerprint,
		}}
		for _, s := range p.Subjects {
			rec.subjects = append(rec.subjects, db.PlanSubject{
				PlanID: id, SubjectID: s.SubjectID, Fingerprint: s.Fingerprint, Outcome: s.Outcome,
			})
		}
		store.plans[id] = rec
		store.created = append(store.created, p)
		return rec.meta, nil
	}

	dbClaimPlanVerified = func(_ context.Context, c db.PlanCitation, verify func([]db.PlanSubject) error) (db.Plan, []db.PlanSubject, error) {
		store.claims++
		rec, ok := store.plans[c.PlanID]
		if !ok {
			return db.Plan{}, nil, fmt.Errorf("%w: %s", db.ErrPlanNotFound, c.PlanID)
		}
		switch {
		case rec.meta.Target != c.Target, rec.meta.Surface != c.Surface:
			return db.Plan{}, nil, db.ErrPlanNotCitableHere
		case rec.meta.CreatedBy != c.Actor:
			return db.Plan{}, nil, db.ErrPlanNotYours
		case rec.meta.RequestFingerprint != c.RequestFingerprint:
			return db.Plan{}, nil, db.ErrPlanRequestMismatch
		case rec.meta.AppliedAt != nil:
			return db.Plan{}, nil, db.ErrPlanAlreadyApplied
		case rec.meta.ExpiresAt != nil && !rec.meta.ExpiresAt.After(time.Now()):
			return db.Plan{}, nil, db.ErrPlanExpired
		}
		if verify != nil {
			// Verified BEFORE the approval is spent, exactly as the real store
			// does it: a stale plan must leave the operator able to re-plan and
			// apply, not holding an approval consumed by an apply that never ran.
			if err := verify(rec.subjects); err != nil {
				return db.Plan{}, nil, err
			}
		}
		now := time.Now()
		rec.meta.AppliedAt = &now
		return rec.meta, rec.subjects, nil
	}
	return store
}

// rehearseThenApply runs the two requests an operator's confirmation flow makes
// and returns the apply's recorder. `mutate` may edit the apply body, which is
// how the binding tests submit a different request under one approval.
func rehearseThenApply(t *testing.T, handler http.HandlerFunc, path, body string, mutate func(map[string]any)) *httptest.ResponseRecorder {
	t.Helper()

	rehearse := httptest.NewRecorder()
	handler(rehearse, httptest.NewRequest(http.MethodPost, path, strings.NewReader(body)))
	if rehearse.Code != http.StatusOK {
		t.Fatalf("rehearsal failed: %d (%s)", rehearse.Code, rehearse.Body.String())
	}
	var issued services.BulkPlan
	if err := json.Unmarshal(rehearse.Body.Bytes(), &issued); err != nil {
		t.Fatalf("decode rehearsal: %v", err)
	}
	if issued.PlanID == "" {
		t.Fatal("a rehearsal must return the plan id its apply will cite")
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	payload["plan_id"] = issued.PlanID
	if mutate != nil {
		mutate(payload)
	}
	applyBody, _ := json.Marshal(payload)

	apply := httptest.NewRecorder()
	handler(apply, httptest.NewRequest(http.MethodPost, path+"?apply=true", strings.NewReader(string(applyBody))))
	return apply
}

// An apply with no citation is refused rather than falling back to
// recomputation. Leaving the fallback would leave both protocols live, with the
// weaker one governing the access that actually exists.
func TestApplyWithoutAPlanIsRefusedOnEverySurface(t *testing.T) {
	stubPlanStore(t)
	stubRehearsal(t, services.BulkPlan{
		Op:       services.BulkOpAssignRole,
		Outcomes: []services.BulkOutcome{{UserID: "u1", Effect: services.EffectApply}},
	})
	writes := stubNoWrites(t)

	for _, tc := range []struct {
		name    string
		handler http.HandlerFunc
		path    string
		body    string
	}{
		{"bulk grants", handleBulkGrants, "/api/v1/grants/bulk",
			`{"op":"assign_role","user_ids":["u1"],"project_id":"p","role_key":"r","reason":"x"}`},
		{"bulk decision", handleBulkDecideRequests, "/api/v1/requests/bulk-decision",
			`{"ids":["r1"],"status":"rejected"}`},
		{"drift adopt", handleBulkAttributeDrift, "/api/v1/governance/drift/bulk-attribute",
			`{"ids":["d1"],"source":"external_backfill"}`},
		{"drift mark external", handleBulkMarkDriftExternal, "/api/v1/governance/drift/bulk-mark-external",
			`{"ids":["d1"],"reason":"owned by IT"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			tc.handler(rr, httptest.NewRequest(http.MethodPost, tc.path+"?apply=true", strings.NewReader(tc.body)))
			if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "PLAN_REQUIRED") {
				t.Fatalf("an apply citing no plan must be refused: %d (%s)", rr.Code, rr.Body.String())
			}
		})
	}
	if *writes != 0 {
		t.Errorf("a refused apply must write nothing, got %d", *writes)
	}
}

// The gap this change closes: an operator reviews one diff and the apply
// carries a different request. The plan binds the request that produced it.
func TestApplyCannotCarryADifferentRequestUnderTheSameApproval(t *testing.T) {
	stubPlanStore(t)
	stubRehearsal(t, services.BulkPlan{
		Op:       services.BulkOpAssignRole,
		Outcomes: []services.BulkOutcome{{UserID: "u1", Effect: services.EffectApply}},
	})
	writes := stubNoWrites(t)

	for _, tc := range []struct {
		name  string
		edit  func(map[string]any)
		match string
	}{
		{"a longer grant than was reviewed", func(b map[string]any) { b["duration_days"] = 3650 }, "PLAN_REQUEST_MISMATCH"},
		{"a wider cohort than was reviewed", func(b map[string]any) { b["user_ids"] = []string{"u1", "u2"} }, "PLAN_REQUEST_MISMATCH"},
		{"a different role than was reviewed", func(b map[string]any) { b["role_key"] = "admin" }, "PLAN_REQUEST_MISMATCH"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := *writes
			rr := rehearseThenApply(t, handleBulkGrants, "/api/v1/grants/bulk",
				`{"op":"assign_role","user_ids":["u1"],"project_id":"p","role_key":"r","reason":"x","duration_days":30}`, tc.edit)
			if rr.Code != http.StatusConflict || !strings.Contains(rr.Body.String(), tc.match) {
				t.Fatalf("want %s, got %d (%s)", tc.match, rr.Code, rr.Body.String())
			}
			if *writes != before {
				t.Error("a refused apply must mutate nothing")
			}
		})
	}
}

// An annotation is not part of the diff. Binding it would make correcting a
// typo cost a re-plan, which teaches operators to click through the very dialog
// that exists to stop them.
func TestAnAnnotationDoesNotInvalidateTheApproval(t *testing.T) {
	stubPlanStore(t)
	stubRehearsal(t, services.BulkPlan{
		Op:       services.BulkOpAssignRole,
		Outcomes: []services.BulkOutcome{{UserID: "u1", Effect: services.EffectApply}},
	})
	stubNoWrites(t)

	rr := rehearseThenApply(t, handleBulkGrants, "/api/v1/grants/bulk",
		`{"op":"assign_role","user_ids":["u1"],"project_id":"p","role_key":"r","reason":"typo"}`,
		func(b map[string]any) { b["reason"] = "corrected" })
	if rr.Code != http.StatusAccepted {
		t.Fatalf("editing the reason must not invalidate the plan: %d (%s)", rr.Code, rr.Body.String())
	}
}

// One approval, one apply. Expiry bounds how LONG a plan may be cited, never
// how many times, and while the first apply's rows are still queued the target
// has not moved for a fingerprint check to notice.
func TestAnApprovalIsSpentOnce(t *testing.T) {
	store := stubPlanStore(t)
	stubRehearsal(t, services.BulkPlan{
		Op:       services.BulkOpAssignRole,
		Outcomes: []services.BulkOutcome{{UserID: "u1", Effect: services.EffectApply}},
	})
	writes := stubNoWrites(t)

	body := `{"op":"assign_role","user_ids":["u1"],"project_id":"p","role_key":"r","reason":"x"}`
	first := rehearseThenApply(t, handleBulkGrants, "/api/v1/grants/bulk", body, nil)
	if first.Code != http.StatusAccepted {
		t.Fatalf("first apply: %d (%s)", first.Code, first.Body.String())
	}
	planID := store.created[len(store.created)-1]
	_ = planID

	var applied services.BulkPlan
	_ = json.Unmarshal(first.Body.Bytes(), &applied)
	after := *writes

	replay := httptest.NewRecorder()
	handleBulkGrants(replay, httptest.NewRequest(http.MethodPost, "/api/v1/grants/bulk?apply=true",
		strings.NewReader(`{"op":"assign_role","user_ids":["u1"],"project_id":"p","role_key":"r","reason":"x","plan_id":"plan_1"}`)))
	if replay.Code != http.StatusConflict || !strings.Contains(replay.Body.String(), "PLAN_ALREADY_APPLIED") {
		t.Fatalf("a spent approval must not be citable again: %d (%s)", replay.Code, replay.Body.String())
	}
	if *writes != after {
		t.Error("the replay must mutate nothing")
	}
}

// A plan is issued by one screen and citable only there. Without this, a
// drift-triage approval could be spent on the bulk-grant endpoint, where its
// subject ids mean something entirely different.
func TestAPlanIssuedByOneSurfaceCannotBeCitedOnAnother(t *testing.T) {
	store := stubPlanStore(t)
	stubRehearsal(t, services.BulkPlan{
		Op:       services.BulkOpAssignRole,
		Outcomes: []services.BulkOutcome{{UserID: "u1", Effect: services.EffectApply}},
	})
	stubNoWrites(t)

	rehearse := httptest.NewRecorder()
	handleBulkGrants(rehearse, httptest.NewRequest(http.MethodPost, "/api/v1/grants/bulk",
		strings.NewReader(`{"op":"assign_role","user_ids":["u1"],"project_id":"p","role_key":"r","reason":"x"}`)))
	var issued services.BulkPlan
	_ = json.Unmarshal(rehearse.Body.Bytes(), &issued)

	rr := httptest.NewRecorder()
	handleBulkMarkDriftExternal(rr, httptest.NewRequest(http.MethodPost,
		"/api/v1/governance/drift/bulk-mark-external?apply=true",
		strings.NewReader(`{"ids":["d1"],"plan_id":"`+issued.PlanID+`"}`)))
	if rr.Code != http.StatusConflict || !strings.Contains(rr.Body.String(), "PLAN_NOT_CITABLE_HERE") {
		t.Fatalf("want PLAN_NOT_CITABLE_HERE, got %d (%s)", rr.Code, rr.Body.String())
	}
	if len(store.plans) != 1 {
		t.Errorf("the drift surface must not have issued a plan of its own on an apply")
	}
}

// A rehearsal that could not be recorded must not be returned as one: the
// operator would review a diff they cannot then apply, and the surface would
// offer them the button anyway.
func TestARehearsalThatCannotBeRecordedIsNotReturnedAsAPlan(t *testing.T) {
	stubPlanStore(t)
	dbCreatePlan = func(context.Context, db.NewPlan) (db.Plan, error) {
		return db.Plan{}, fmt.Errorf("disk on fire")
	}
	stubRehearsal(t, services.BulkPlan{
		Op:       services.BulkOpAssignRole,
		Outcomes: []services.BulkOutcome{{UserID: "u1", Effect: services.EffectApply}},
	})
	stubNoWrites(t)

	rr := httptest.NewRecorder()
	handleBulkGrants(rr, httptest.NewRequest(http.MethodPost, "/api/v1/grants/bulk",
		strings.NewReader(`{"op":"assign_role","user_ids":["u1"],"project_id":"p","role_key":"r","reason":"x"}`)))
	if rr.Code != http.StatusInternalServerError || !strings.Contains(rr.Body.String(), "PLAN_NOT_RECORDED") {
		t.Fatalf("want PLAN_NOT_RECORDED, got %d (%s)", rr.Code, rr.Body.String())
	}
}

// 3.7 — reconciliation stays read-only. It is a diff view; its rows reach
// mutation through drift triage, which is planned. A mutating verb here would
// be a fifth apply surface with no plan behind it.
func TestReconciliationHasNoApplyPath(t *testing.T) {
	mux := NewRouter()
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, httptest.NewRequest(method, "/api/v1/reconciliation/grants", nil))
		if rr.Code != http.StatusMethodNotAllowed && rr.Code != http.StatusNotFound {
			t.Errorf("%s on the reconciliation diff must not be routed, got %d", method, rr.Code)
		}
	}
}

// 3.6 — the case the whole retrofit exists for. Between the review and the
// apply, one subject moves. The apply fails, mutates nothing, and says who.
func TestASubjectThatMovedFailsTheApplyAndIsNamed(t *testing.T) {
	stubPlanStore(t)
	seen := stubRehearsal(t, services.BulkPlan{
		Op: services.BulkOpAssignRole,
		Outcomes: []services.BulkOutcome{
			{UserID: "u_still", Effect: services.EffectApply},
			{UserID: "u_moved", Effect: services.EffectApply},
		},
	})
	_ = seen
	writes := stubNoWrites(t)

	const body = `{"op":"assign_role","user_ids":["u_still","u_moved"],"project_id":"p","role_key":"r","reason":"x"}`
	rehearse := httptest.NewRecorder()
	handleBulkGrants(rehearse, httptest.NewRequest(http.MethodPost, "/api/v1/grants/bulk", strings.NewReader(body)))
	var issued services.BulkPlan
	if err := json.Unmarshal(rehearse.Body.Bytes(), &issued); err != nil || issued.PlanID == "" {
		t.Fatalf("rehearsal: %s", rehearse.Body.String())
	}

	// The world moves under one subject only. Everything else is identical, so
	// a verification that merely checked "some state exists" would pass.
	stubRehearsal(t, services.BulkPlan{
		Op: services.BulkOpAssignRole,
		Outcomes: []services.BulkOutcome{
			{UserID: "u_still", Effect: services.EffectApply},
			{UserID: "u_moved", Effect: services.EffectApply, Fingerprint: "fp:u_moved:changed"},
		},
	})

	rr := httptest.NewRecorder()
	handleBulkGrants(rr, httptest.NewRequest(http.MethodPost, "/api/v1/grants/bulk?apply=true",
		strings.NewReader(`{"op":"assign_role","user_ids":["u_still","u_moved"],"project_id":"p","role_key":"r","reason":"x","plan_id":"`+issued.PlanID+`"}`)))

	if rr.Code != http.StatusConflict {
		t.Fatalf("a moved subject must fail the apply, got %d (%s)", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "PLAN_STALE") || !strings.Contains(rr.Body.String(), "u_moved") {
		t.Errorf("the refusal must name what moved rather than failing generically: %s", rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), "u_still") {
		t.Error("only the subjects that moved may be named; listing the rest hides the one that matters")
	}
	if *writes != 0 {
		t.Fatalf("a stale apply must mutate nothing, not even the subjects that did not move: %d writes", *writes)
	}
}

// A stale plan must leave the approval unspent, or the operator's only recovery
// — re-plan and apply — is refused as already-applied for something that never
// happened.
func TestAStaleApplyDoesNotSpendTheApproval(t *testing.T) {
	store := stubPlanStore(t)
	stubRehearsal(t, services.BulkPlan{
		Op:       services.BulkOpAssignRole,
		Outcomes: []services.BulkOutcome{{UserID: "u1", Effect: services.EffectApply}},
	})
	stubNoWrites(t)

	const body = `{"op":"assign_role","user_ids":["u1"],"project_id":"p","role_key":"r","reason":"x"}`
	rehearse := httptest.NewRecorder()
	handleBulkGrants(rehearse, httptest.NewRequest(http.MethodPost, "/api/v1/grants/bulk", strings.NewReader(body)))
	var issued services.BulkPlan
	_ = json.Unmarshal(rehearse.Body.Bytes(), &issued)

	stubRehearsal(t, services.BulkPlan{
		Op:       services.BulkOpAssignRole,
		Outcomes: []services.BulkOutcome{{UserID: "u1", Effect: services.EffectApply, Fingerprint: "moved"}},
	})
	stale := httptest.NewRecorder()
	handleBulkGrants(stale, httptest.NewRequest(http.MethodPost, "/api/v1/grants/bulk?apply=true",
		strings.NewReader(`{"op":"assign_role","user_ids":["u1"],"project_id":"p","role_key":"r","reason":"x","plan_id":"`+issued.PlanID+`"}`)))
	if stale.Code != http.StatusConflict {
		t.Fatalf("expected a stale refusal, got %d", stale.Code)
	}
	if rec := store.plans[issued.PlanID]; rec == nil || rec.meta.AppliedAt != nil {
		t.Fatal("a refused apply must leave the approval unspent")
	}
}

// A citation that fails on identity has nothing to verify, and verification
// costs a full cohort read. A bogus id must not buy one.
func TestARejectedCitationCostsNoRead(t *testing.T) {
	stubPlanStore(t)
	stubNoWrites(t)
	reads := 0
	orig := svcRehearseBulk
	t.Cleanup(func() { svcRehearseBulk = orig })
	svcRehearseBulk = func(context.Context, services.BulkRequest) (services.BulkPlan, error) {
		reads++
		return services.BulkPlan{}, nil
	}

	rr := httptest.NewRecorder()
	handleBulkGrants(rr, httptest.NewRequest(http.MethodPost, "/api/v1/grants/bulk?apply=true",
		strings.NewReader(`{"op":"assign_role","user_ids":["u1"],"project_id":"p","role_key":"r","reason":"x","plan_id":"nope"}`)))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("want PLAN_NOT_FOUND, got %d (%s)", rr.Code, rr.Body.String())
	}
	if reads != 0 {
		t.Errorf("an unknown plan must not cost a rehearsal, got %d", reads)
	}
}

// The retrofit's central claim, stated as a test: what an apply executes is
// what was APPROVED, not what a fresh computation would decide now.
//
// The two are made to disagree while the fingerprints agree — which is
// artificial on purpose. In production a verdict that changed would carry a
// changed fingerprint and the apply would be refused; the point here is that
// the approval is the authority even so, because the alternative is the old
// behaviour wearing the new protocol's clothes.
func TestApplyExecutesTheApprovedEffectRatherThanARecomputedOne(t *testing.T) {
	stubPlanStore(t)
	stubRehearsal(t, services.BulkPlan{
		Op: services.BulkOpAssignRole,
		Outcomes: []services.BulkOutcome{
			{UserID: "u_approved", Effect: services.EffectApply},
			{UserID: "u_declined", Effect: services.EffectBlocked, Detail: "Account is departed"},
		},
	})
	writes := stubNoWrites(t)

	const body = `{"op":"assign_role","user_ids":["u_approved","u_declined"],"project_id":"p","role_key":"r","reason":"x"}`
	rehearse := httptest.NewRecorder()
	handleBulkGrants(rehearse, httptest.NewRequest(http.MethodPost, "/api/v1/grants/bulk", strings.NewReader(body)))
	var issued services.BulkPlan
	if err := json.Unmarshal(rehearse.Body.Bytes(), &issued); err != nil || issued.PlanID == "" {
		t.Fatalf("rehearsal: %s", rehearse.Body.String())
	}

	// Same subjects, same fingerprints, opposite verdicts.
	stubRehearsal(t, services.BulkPlan{
		Op: services.BulkOpAssignRole,
		Outcomes: []services.BulkOutcome{
			{UserID: "u_approved", Effect: services.EffectBlocked, Fingerprint: "fp:u_approved", Detail: "recomputed block"},
			{UserID: "u_declined", Effect: services.EffectApply, Fingerprint: "fp:u_declined", Detail: "recomputed apply"},
		},
	})

	rr := httptest.NewRecorder()
	handleBulkGrants(rr, httptest.NewRequest(http.MethodPost, "/api/v1/grants/bulk?apply=true",
		strings.NewReader(`{"op":"assign_role","user_ids":["u_approved","u_declined"],"project_id":"p","role_key":"r","reason":"x","plan_id":"`+issued.PlanID+`"}`)))
	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d (%s)", rr.Code, rr.Body.String())
	}

	rows := map[string]services.BulkOutcome{}
	for _, o := range decodePlan(t, rr).Outcomes {
		rows[o.UserID] = o
	}
	if got := rows["u_approved"].Effect; got != services.EffectApplied {
		t.Errorf("the approved row must be executed, got %q — a recomputed verdict overrode the approval", got)
	}
	if got := rows["u_declined"].Effect; got != services.EffectBlocked {
		t.Errorf("a row the operator was shown as blocked must stay blocked, got %q — the apply granted access nobody approved", got)
	}
	if *writes != 1 {
		t.Fatalf("exactly the approved row may be written, got %d", *writes)
	}

	// The sentences are rendered fresh, which is the one thing that MAY move:
	// a plan records the decision, never its prose.
	if rows["u_declined"].Detail != "recomputed apply" {
		t.Errorf("human text is rendered from the current read, got %q", rows["u_declined"].Detail)
	}
}

// And it acts on the grant ROWS that were approved. `GrantIDs` is what a
// removal deletes, so re-deriving it at apply would delete rows nobody was
// shown — the same class of mistake as re-deciding the effect, one level down
// and harder to notice, because the row count would still look right.
func TestApplyActsOnTheApprovedGrantRows(t *testing.T) {
	stubPlanStore(t)
	stubRehearsal(t, services.BulkPlan{
		Op:       services.BulkOpRemoveRole,
		Outcomes: []services.BulkOutcome{{UserID: "u1", Effect: services.EffectApply, GrantIDs: []string{"g_reviewed"}}},
	})
	stubNoWrites(t)

	origDelete := svcDeleteDirectGrant
	t.Cleanup(func() { svcDeleteDirectGrant = origDelete })
	var deleted []string
	svcDeleteDirectGrant = func(_ context.Context, _, grantID, _ string) (services.DirectGrantRemoval, error) {
		deleted = append(deleted, grantID)
		return services.DirectGrantRemoval{}, nil
	}

	const body = `{"op":"remove_role","user_ids":["u1"],"project_id":"p","role_key":"r","reason":"x"}`
	rehearse := httptest.NewRecorder()
	handleBulkGrants(rehearse, httptest.NewRequest(http.MethodPost, "/api/v1/grants/bulk", strings.NewReader(body)))
	var issued services.BulkPlan
	if err := json.Unmarshal(rehearse.Body.Bytes(), &issued); err != nil || issued.PlanID == "" {
		t.Fatalf("rehearsal: %s", rehearse.Body.String())
	}

	// The world offers a different row under the same fingerprint.
	stubRehearsal(t, services.BulkPlan{
		Op: services.BulkOpRemoveRole,
		Outcomes: []services.BulkOutcome{
			{UserID: "u1", Effect: services.EffectApply, Fingerprint: "fp:u1", GrantIDs: []string{"g_never_shown"}},
		},
	})

	rr := httptest.NewRecorder()
	handleBulkGrants(rr, httptest.NewRequest(http.MethodPost, "/api/v1/grants/bulk?apply=true",
		strings.NewReader(`{"op":"remove_role","user_ids":["u1"],"project_id":"p","role_key":"r","reason":"x","plan_id":"`+issued.PlanID+`"}`)))
	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d (%s)", rr.Code, rr.Body.String())
	}
	if len(deleted) != 1 || deleted[0] != "g_reviewed" {
		t.Fatalf("the apply must delete the reviewed grant row, got %v", deleted)
	}
}

// 2.39/2.40 — the cohort guard, at the only place the cohort exists.
//
// A per-subject apply cannot know how many subjects an operation touches, so a
// guard specified there would sit in the component unable to implement it. It
// lives in `issuePlan`, which is every planned surface's one way to become an
// approval — including the ones added later.
func TestAnOversizedCohortIsRefusedBeforeAnythingIsRecorded(t *testing.T) {
	t.Setenv("PLAN_COHORT_LIMIT", "3")
	store := stubPlanStore(t)

	outcomes := make([]services.BulkOutcome, 0, 6)
	for _, id := range []string{"u1", "u2", "u3", "u4", "u5", "u_no"} {
		effect := services.EffectApply
		if id == "u_no" {
			effect = services.EffectNoChange
		}
		outcomes = append(outcomes, services.BulkOutcome{UserID: id, Effect: effect})
	}
	stubRehearsal(t, services.BulkPlan{
		Op:       services.BulkOpAssignRole,
		Outcomes: outcomes,
		Summary:  services.BulkSummary{Total: 6, Apply: 5, NoChange: 1},
	})
	writes := stubNoWrites(t)

	const body = `{"op":"assign_role","user_ids":["u1","u2","u3","u4","u5","u_no"],"project_id":"p","role_key":"r","reason":"x"}`
	rr := httptest.NewRecorder()
	handleBulkGrants(rr, httptest.NewRequest(http.MethodPost, "/api/v1/grants/bulk", strings.NewReader(body)))

	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("an unacknowledged oversized cohort must be refused, got %d (%s)", rr.Code, rr.Body.String())
	}
	// The number it computed, not "too large": an operator told only that
	// something is big has to guess what they are being warned about.
	if !strings.Contains(rr.Body.String(), `"affected":"5"`) || !strings.Contains(rr.Body.String(), `"limit":"3"`) {
		t.Errorf("the refusal must report the computed count and the limit: %s", rr.Body.String())
	}
	if len(store.plans) != 0 || *writes != 0 {
		t.Error("a refused rehearsal must record no approval and write nothing")
	}

	// Acknowledged, it proceeds — and the acknowledgement is not bound to the
	// plan, since it unlocks issuing the approval rather than changing what the
	// approval does. An apply that omits it must still be accepted.
	ack := httptest.NewRecorder()
	handleBulkGrants(ack, httptest.NewRequest(http.MethodPost, "/api/v1/grants/bulk",
		strings.NewReader(`{"op":"assign_role","user_ids":["u1","u2","u3","u4","u5","u_no"],"project_id":"p","role_key":"r","reason":"x","acknowledge_scope":true}`)))
	if ack.Code != http.StatusOK {
		t.Fatalf("an acknowledged cohort must be planned, got %d (%s)", ack.Code, ack.Body.String())
	}
	var issued services.BulkPlan
	_ = json.Unmarshal(ack.Body.Bytes(), &issued)

	apply := httptest.NewRecorder()
	handleBulkGrants(apply, httptest.NewRequest(http.MethodPost, "/api/v1/grants/bulk?apply=true",
		strings.NewReader(`{"op":"assign_role","user_ids":["u1","u2","u3","u4","u5","u_no"],"project_id":"p","role_key":"r","reason":"x","plan_id":"`+issued.PlanID+`"}`)))
	if apply.Code != http.StatusAccepted {
		t.Fatalf("the acknowledgement must not have to be repeated on apply: %d (%s)", apply.Code, apply.Body.String())
	}
}

// It counts the subjects that would CHANGE, not the ones selected. A selection
// of two hundred that grants three of them a role is a small change reviewed as
// a large one, and refusing it teaches operators to acknowledge everything.
func TestTheCohortGuardCountsWhatWouldChange(t *testing.T) {
	t.Setenv("PLAN_COHORT_LIMIT", "3")
	stubPlanStore(t)

	outcomes := make([]services.BulkOutcome, 0, 40)
	for i := range 40 {
		effect := services.EffectNoChange
		if i < 2 {
			effect = services.EffectApply
		}
		outcomes = append(outcomes, services.BulkOutcome{UserID: fmt.Sprintf("u%d", i), Effect: effect})
	}
	stubRehearsal(t, services.BulkPlan{
		Op:       services.BulkOpAssignRole,
		Outcomes: outcomes,
		Summary:  services.BulkSummary{Total: 40, Apply: 2, NoChange: 38},
	})
	stubNoWrites(t)

	rr := httptest.NewRecorder()
	handleBulkGrants(rr, httptest.NewRequest(http.MethodPost, "/api/v1/grants/bulk",
		strings.NewReader(`{"op":"assign_role","user_ids":["u0"],"project_id":"p","role_key":"r","reason":"x"}`)))
	if rr.Code != http.StatusOK {
		t.Fatalf("a wide selection with a narrow effect must not need an acknowledgement: %d (%s)", rr.Code, rr.Body.String())
	}
}

// Every planned surface gets it, because it lives in the one function they all
// go through rather than in each of them.
func TestTheCohortGuardCoversEverySurface(t *testing.T) {
	src, err := os.ReadFile("plan_gate.go")
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`(?s)func issuePlan\([^)]*\) error \{.*?cohortTooLarge\{`).Match(src) {
		t.Fatal("the guard must live in issuePlan: a copy per surface is a surface that will be added without one")
	}
	for _, f := range []string{"bulk.go", "drift.go", "requests_bulk.go"} {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(b), "cohortLimit()") {
			t.Errorf("%s must not enforce the cohort limit itself", f)
		}
	}
}
