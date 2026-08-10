package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"syndra/internal/addons"
	"syndra/internal/auth"
	"syndra/internal/db"
)

// 7.4, 8.6/8.10 — the surfaces around mappings and allowances, and the split
// validation that decides what each side may refuse.

type mappingHarness struct {
	created  []db.RoleMapping
	resolved []string
	valueErr error

	// The apply half: who holds the role, what the edit did, and what it queued.
	holders   []string
	claimed   []db.PlanCitation
	claimErr  error
	updated   []string
	deleted   []string
	converged []db.SystemConvergence
}

func stubMappingDeps(t *testing.T, schema []addons.EntitlementField) *mappingHarness {
	t.Helper()
	h := &mappingHarness{}
	get, create, resolves := addonsEntitlementSchema, dbCreateRoleMapping, addonsResolvesValue
	t.Cleanup(func() {
		addonsEntitlementSchema, dbCreateRoleMapping, addonsResolvesValue = get, create, resolves
	})

	addonsEntitlementSchema = func(target string) ([]addons.EntitlementField, error) {
		if target != "truenas" {
			return nil, addons.ErrNotRegistered
		}
		return schema, nil
	}
	dbCreateRoleMapping = func(_ context.Context, m db.RoleMapping) (db.RoleMapping, error) {
		h.created = append(h.created, m)
		m.ID = "m1"
		return m, nil
	}
	addonsResolvesValue = func(_ context.Context, target, field, value string) error {
		h.resolved = append(h.resolved, target+"|"+field+"|"+value)
		return h.valueErr
	}
	stubMappingApplyPath(t, h)
	return h
}

// stubMappingApplyPath fakes the half an edit runs after validation: the cohort
// read, the transaction it all happens in, and the convergences it queues.
//
// The default cohort is empty, which is the case that needs no citation — a
// mapping nobody holds is a definition, and there is nothing to review. A test
// about the citation sets holders and gets the plan path.
func stubMappingApplyPath(t *testing.T, h *mappingHarness) {
	t.Helper()
	holders, inTx, claim, record := dbMappingHolders, svcInTxLockingAccess, dbClaimPlanVerified, dbRecordSystemConvergence
	update, del := dbUpdateRoleMappingValue, dbDeleteRoleMapping
	t.Cleanup(func() {
		dbMappingHolders, svcInTxLockingAccess, dbClaimPlanVerified, dbRecordSystemConvergence = holders, inTx, claim, record
		dbUpdateRoleMappingValue, dbDeleteRoleMapping = update, del
	})

	dbMappingHolders = func(context.Context, string, string) ([]string, error) { return h.holders, nil }
	svcInTxLockingAccess = func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }
	dbClaimPlanVerified = func(_ context.Context, c db.PlanCitation, _ func([]db.PlanSubject) error) (db.Plan, []db.PlanSubject, error) {
		h.claimed = append(h.claimed, c)
		return db.Plan{ID: c.PlanID}, nil, h.claimErr
	}
	dbRecordSystemConvergence = func(_ context.Context, c db.SystemConvergence) (string, string, error) {
		h.converged = append(h.converged, c)
		return "plan_1", "outbox_1", nil
	}
	dbUpdateRoleMappingValue = func(_ context.Context, id, value, actor string) error {
		h.updated = append(h.updated, id+"="+value)
		return nil
	}
	dbDeleteRoleMapping = func(_ context.Context, id string) error {
		h.deleted = append(h.deleted, id)
		return nil
	}
}

func postMapping(body string) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	handleCreateRoleMapping(rr, httptest.NewRequest(http.MethodPost, "/api/v1/targets/mappings", strings.NewReader(body)))
	return rr
}

// Structure first, and structure without a network call. A field the schema
// does not declare is wrong whatever the target says, and spending a call to be
// told so would make an outage look like a validation failure.
func TestAnUndeclaredFieldIsRejectedWithoutCallingTheAddon(t *testing.T) {
	h := stubMappingDeps(t, []addons.EntitlementField{{Name: "group", Type: "string[]"}})

	rr := postMapping(`{"target":"truenas","project_id":"pLab","role_key":"maker","field":"path_grant","value":"/mnt/lab"}`)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d (%s)", rr.Code, rr.Body.String())
	}
	if len(h.resolved) != 0 {
		t.Errorf("the add-on must not be asked about a field its schema does not declare: %v", h.resolved)
	}
	if len(h.created) != 0 {
		t.Error("and nothing may be written")
	}
	// The declared set is named: an operator whose field is not in the schema
	// needs to know what is.
	if !strings.Contains(rr.Body.String(), "group") {
		t.Errorf("the refusal must name what the add-on does declare: %s", rr.Body.String())
	}
}

// A lifecycle field is refused as a mapping target even when the add-on
// declares it. Binding a role to `enabled` would mean holding that role
// disables the account, colliding with the derived lifecycle lock and fighting
// it on every resolution.
func TestALifecycleFieldIsNotABindableMappingTarget(t *testing.T) {
	h := stubMappingDeps(t, []addons.EntitlementField{
		{Name: "group", Type: "string[]"},
		{Name: "enabled", Type: "bool", Lifecycle: true},
	})

	rr := postMapping(`{"target":"truenas","project_id":"pLab","role_key":"maker","field":"enabled","value":"true"}`)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d (%s)", rr.Code, rr.Body.String())
	}
	if len(h.created) != 0 || len(h.resolved) != 0 {
		t.Error("nothing may be written and the add-on must not be asked")
	}

	// The manifest's own flag is honoured too, on a field the backend has never
	// heard of. The backend's list covers the fields it computes; the flag
	// covers the ones a later add-on computes for itself, and only the manifest
	// can say which those are.
	future := stubMappingDeps(t, []addons.EntitlementField{
		{Name: "group", Type: "string[]"},
		{Name: "quota_locked", Type: "bool", Lifecycle: true},
	})
	rr = postMapping(`{"target":"truenas","project_id":"pLab","role_key":"maker","field":"quota_locked","value":"true"}`)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("a manifest-declared lifecycle field must not be bindable: %d (%s)", rr.Code, rr.Body.String())
	}
	if len(future.created) != 0 {
		t.Error("nothing may be written")
	}
}

// An edit validates against the target the mapping is actually on, never one
// derived from the request — the update body carries no target, so a value
// checked against the wrong add-on's schema would be checked against nothing.
func TestAnEditValidatesAgainstTheMappingsOwnTarget(t *testing.T) {
	stubMappingDeps(t, []addons.EntitlementField{{Name: "group", Type: "string[]"}})

	var asked []string
	addonsEntitlementSchema = func(target string) ([]addons.EntitlementField, error) {
		asked = append(asked, target)
		return []addons.EntitlementField{{Name: "group", Type: "string[]"}}, nil
	}
	get, update := dbGetRoleMapping, dbUpdateRoleMappingValue
	t.Cleanup(func() { dbGetRoleMapping, dbUpdateRoleMappingValue = get, update })
	dbGetRoleMapping = func(context.Context, string) (db.RoleMapping, error) {
		return db.RoleMapping{ID: "m1", Target: "unifi", Field: "group"}, nil
	}
	dbUpdateRoleMappingValue = func(context.Context, string, string, string) error { return nil }

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/targets/mappings/m1", strings.NewReader(`{"value":"door_staff"}`))
	req.SetPathValue("id", "m1")
	handleUpdateRoleMapping(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", rr.Code, rr.Body.String())
	}
	if len(asked) != 1 || asked[0] != "unifi" {
		t.Fatalf("the schema consulted must be the mapping's own target's, got %v", asked)
	}
}

// The add-on's half: the field is fine and the value names nothing.
func TestAnUnresolvableValueIsRejectedAfterTheAddonSaysSo(t *testing.T) {
	h := stubMappingDeps(t, []addons.EntitlementField{{Name: "group", Type: "string[]"}})
	h.valueErr = errMappingValueUnresolvable

	rr := postMapping(`{"target":"truenas","project_id":"pLab","role_key":"maker","field":"group","value":"no_such_group"}`)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d (%s)", rr.Code, rr.Body.String())
	}
	if len(h.resolved) != 1 {
		t.Fatalf("the add-on must have been asked exactly once: %v", h.resolved)
	}
	if len(h.created) != 0 {
		t.Error("nothing may be written")
	}
}

// A registered target that has never answered is neither a valid mapping nor an
// invalid one. Saying 400 would send an operator looking for their own mistake.
func TestAMappingAgainstASilentTargetIsUnavailableRatherThanInvalid(t *testing.T) {
	stubMappingDeps(t, nil)
	addonsEntitlementSchema = func(string) ([]addons.EntitlementField, error) {
		// Registered, and it has never published capabilities.
		return nil, addons.ErrNoManifest
	}

	rr := postMapping(`{"target":"truenas","project_id":"pLab","role_key":"maker","field":"group","value":"lab_makers"}`)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503, got %d (%s)", rr.Code, rr.Body.String())
	}
}

// A target the deployment does not run is a different refusal again, and the
// foreign key would otherwise answer it with a message about a constraint.
func TestAMappingToAnUnregisteredTargetIsRefusedByName(t *testing.T) {
	stubMappingDeps(t, []addons.EntitlementField{{Name: "group", Type: "string[]"}})

	rr := postMapping(`{"target":"unifi","project_id":"pLab","role_key":"maker","field":"group","value":"x"}`)
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "not a registered") {
		t.Fatalf("want a named refusal, got %d (%s)", rr.Code, rr.Body.String())
	}
}

// A duplicate binding is a resolver returning whichever row the database
// ordered first. Reported as a conflict rather than a validation error, because
// the request is well formed and the state is what refuses it.
func TestADuplicateBindingIsAConflict(t *testing.T) {
	stubMappingDeps(t, []addons.EntitlementField{{Name: "group", Type: "string[]"}})
	create := dbCreateRoleMapping
	t.Cleanup(func() { dbCreateRoleMapping = create })
	dbCreateRoleMapping = func(context.Context, db.RoleMapping) (db.RoleMapping, error) {
		return db.RoleMapping{}, db.ErrMappingExists
	}

	rr := postMapping(`{"target":"truenas","project_id":"pLab","role_key":"maker","field":"group","value":"lab_makers"}`)
	if rr.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d (%s)", rr.Code, rr.Body.String())
	}
}

// The actor is the authenticated operator and never a field of the request. An
// allowance whose author is whoever the client said is one nobody can be asked
// about, which is the whole thing this layer exists for.
func TestAnAllowanceRecordsTheAuthenticatedOperator(t *testing.T) {
	stubMappingDeps(t, []addons.EntitlementField{{Name: "group", Type: "string[]"}})
	create := dbCreateAllowance
	t.Cleanup(func() { dbCreateAllowance = create })
	var seen db.Allowance
	dbCreateAllowance = func(_ context.Context, a db.Allowance) (db.Allowance, error) {
		seen = a
		a.ID = "a1"
		return a, nil
	}

	rr := httptest.NewRecorder()
	// Authenticated, so the assertion below is about the PRINCIPAL rather than
	// about whichever fallback the surface happens to use.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/allowances", strings.NewReader(
		`{"subject_id":"u1","target":"truenas","field":"group","value":"lab_makers","direction":"deny","reason":"safety review","expires_at":"2026-12-01T00:00:00Z"}`))
	req = req.WithContext(withPrincipal(req.Context(), &auth.Principal{Subject: "op_7"}))
	handleCreateAllowance(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d (%s)", rr.Code, rr.Body.String())
	}
	// The actor is exactly the resolved principal, and nothing else on the
	// request can become it. Asserting "not empty" would pass for any field the
	// caller controls.
	if seen.ActorID != "op_7" {
		t.Fatalf("the actor must be the authenticated operator, got %q", seen.ActorID)
	}
	for _, supplied := range []string{seen.Reason, seen.SubjectID, seen.Value, seen.Field} {
		if seen.ActorID == supplied {
			t.Fatalf("the actor must not be a field the caller supplied (%q)", supplied)
		}
	}
}

// An additive allowance is well formed and unimplemented. A 400 would send an
// operator looking for their own mistake.
func TestAnAdditiveAllowanceIsNotImplementedRatherThanInvalid(t *testing.T) {
	stubMappingDeps(t, []addons.EntitlementField{{Name: "group", Type: "string[]"}})
	create := dbCreateAllowance
	t.Cleanup(func() { dbCreateAllowance = create })
	dbCreateAllowance = func(context.Context, db.Allowance) (db.Allowance, error) {
		return db.Allowance{}, db.ErrAllowanceAdditiveUnsupported
	}

	rr := httptest.NewRecorder()
	handleCreateAllowance(rr, httptest.NewRequest(http.MethodPost, "/api/v1/allowances", strings.NewReader(
		`{"subject_id":"u1","target":"truenas","field":"group","value":"x","direction":"allow","reason":"r","expires_at":"2026-12-01T00:00:00Z"}`)))

	if rr.Code != http.StatusNotImplemented {
		t.Fatalf("want 501, got %d (%s)", rr.Code, rr.Body.String())
	}
}

// Nothing here deletes. The surface offers lifting, and the row survives.
func TestTheAllowanceSurfaceOffersNoDelete(t *testing.T) {
	mux := NewRouter()
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodDelete, "/api/v1/allowances/a1", nil))
	if rr.Code != http.StatusMethodNotAllowed && rr.Code != http.StatusNotFound {
		t.Fatalf("an allowance must not be deletable: %d", rr.Code)
	}
}

// The failure this validation exists for: `resolveLifecycle` honours a
// lifecycle denial only when the value is exactly "true", so `enabled=false` —
// the way most people would write "disable this account" — was recorded, shown
// in the lineage band as in force with an actor and a reason, and suppressed
// nothing at all. An allowance the resolver ignores is worse than a rejected
// one, because the operator has evidence they suspended somebody.
func TestALifecycleDenialWrittenAsFalseIsRefused(t *testing.T) {
	stubMappingDeps(t, []addons.EntitlementField{
		{Name: "group", Type: "string[]"},
		{Name: "enabled", Type: "bool", Lifecycle: true},
	})
	create := dbCreateAllowance
	t.Cleanup(func() { dbCreateAllowance = create })
	var stored int
	dbCreateAllowance = func(context.Context, db.Allowance) (db.Allowance, error) {
		stored++
		return db.Allowance{ID: "a1"}, nil
	}

	for _, value := range []string{"false", "1", "True", ""} {
		rr := httptest.NewRecorder()
		handleCreateAllowance(rr, httptest.NewRequest(http.MethodPost, "/api/v1/allowances", strings.NewReader(
			`{"subject_id":"u1","target":"truenas","field":"enabled","value":"`+value+`",`+
				`"direction":"deny","reason":"safety review","expires_at":"2026-12-01T00:00:00Z"}`)))
		if rr.Code != http.StatusBadRequest {
			t.Errorf("value %q must be refused, got %d (%s)", value, rr.Code, rr.Body.String())
		}
	}
	if stored != 0 {
		t.Fatalf("nothing may be stored for a denial the resolver would ignore, stored %d", stored)
	}

	// And the one spelling that does something is accepted.
	rr := httptest.NewRecorder()
	handleCreateAllowance(rr, httptest.NewRequest(http.MethodPost, "/api/v1/allowances", strings.NewReader(
		`{"subject_id":"u1","target":"truenas","field":"enabled","value":"true",`+
			`"direction":"deny","reason":"safety review","expires_at":"2026-12-01T00:00:00Z"}`)))
	if rr.Code != http.StatusCreated {
		t.Fatalf("want 201 for enabled=true, got %d (%s)", rr.Code, rr.Body.String())
	}
}

// A misspelled field is the same failure with a different spelling.
func TestAnAllowanceOnAFieldTheTargetLacksIsRefused(t *testing.T) {
	stubMappingDeps(t, []addons.EntitlementField{{Name: "group", Type: "string[]"}})
	create := dbCreateAllowance
	t.Cleanup(func() { dbCreateAllowance = create })
	dbCreateAllowance = func(context.Context, db.Allowance) (db.Allowance, error) {
		t.Fatal("nothing may be stored for a field the target does not have")
		return db.Allowance{}, nil
	}

	rr := httptest.NewRecorder()
	handleCreateAllowance(rr, httptest.NewRequest(http.MethodPost, "/api/v1/allowances", strings.NewReader(
		`{"subject_id":"u1","target":"truenas","field":"groups","value":"lab_makers",`+
			`"direction":"deny","reason":"r","expires_at":"2026-12-01T00:00:00Z"}`)))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d (%s)", rr.Code, rr.Body.String())
	}
	// The declared set is named, so an operator whose field is not in the schema
	// learns what is.
	if !strings.Contains(rr.Body.String(), "group") {
		t.Errorf("the refusal must name what the target does declare: %s", rr.Body.String())
	}
}

// An unregistered target reaches the foreign key otherwise, and returns 500
// with raw constraint text — the failure the mapping surface refuses early.
func TestAnAllowanceOnAnUnregisteredTargetIsRefusedBeforeTheForeignKey(t *testing.T) {
	stubMappingDeps(t, []addons.EntitlementField{{Name: "group", Type: "string[]"}})
	create := dbCreateAllowance
	t.Cleanup(func() { dbCreateAllowance = create })
	dbCreateAllowance = func(context.Context, db.Allowance) (db.Allowance, error) {
		t.Fatal("nothing may reach the database for a target the deployment does not run")
		return db.Allowance{}, nil
	}

	rr := httptest.NewRecorder()
	handleCreateAllowance(rr, httptest.NewRequest(http.MethodPost, "/api/v1/allowances", strings.NewReader(
		`{"subject_id":"u1","target":"unifi","field":"group","value":"x",`+
			`"direction":"deny","reason":"r","expires_at":"2026-12-01T00:00:00Z"}`)))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d (%s)", rr.Code, rr.Body.String())
	}
}

// 7.11/7.12 — a mapping edit is the highest-leverage change in the system, and
// it used to be one PATCH. What is asserted here is that the leverage is now
// visible before it lands, and that the change and the convergences it causes
// are one transaction.

func mappingWithHolders(t *testing.T, h *mappingHarness, holders ...string) db.RoleMapping {
	t.Helper()
	m := db.RoleMapping{ID: "m1", Target: "truenas", ProjectID: "pLab", RoleKey: "trained",
		Field: "group", Value: "lab_makers"}
	get := dbGetRoleMapping
	t.Cleanup(func() { dbGetRoleMapping = get })
	dbGetRoleMapping = func(context.Context, string) (db.RoleMapping, error) { return m, nil }
	h.holders = holders
	return m
}

func patchMapping(body string) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPatch, "/api/v1/targets/mappings/m1", strings.NewReader(body))
	r.SetPathValue("id", "m1")
	handleUpdateRoleMapping(rr, r)
	return rr
}

// An edit reaching people needs the approval that showed who they are. Without
// this the plan path is a screen an operator can skip.
func TestAnEditThatReachesPeopleNeedsTheApprovalThatShowedThem(t *testing.T) {
	h := stubMappingDeps(t, []addons.EntitlementField{{Name: "group", Type: "string[]"}})
	mappingWithHolders(t, h, "u1", "u2")

	rr := patchMapping(`{"value":"lab_users"}`)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("want the edit refused, got %d (%s)", rr.Code, rr.Body.String())
	}
	if len(h.updated) != 0 {
		t.Error("nothing may be edited without the approval")
	}
	if len(h.converged) != 0 {
		t.Error("nothing may be queued without the approval")
	}
}

// A mapping nobody holds needs no citation: there is nothing to review, and
// demanding one would make defining a new binding a two-step ceremony.
func TestAnEditReachingNobodyNeedsNoApproval(t *testing.T) {
	h := stubMappingDeps(t, []addons.EntitlementField{{Name: "group", Type: "string[]"}})
	mappingWithHolders(t, h)

	rr := patchMapping(`{"value":"lab_users"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", rr.Code, rr.Body.String())
	}
	if len(h.updated) != 1 {
		t.Errorf("the edit must land: %v", h.updated)
	}
	if len(h.claimed) != 0 {
		t.Error("no approval should have been spent")
	}
}

// The cited approval reaches the claim with every dimension the predicate needs,
// and the convergences are queued for the cohort read AFTER the edit.
func TestAnApprovedEditConvergesEveryHolder(t *testing.T) {
	h := stubMappingDeps(t, []addons.EntitlementField{{Name: "group", Type: "string[]"}})
	m := mappingWithHolders(t, h, "u1", "u2")
	stubResolvedIntent(t)

	rr := patchMapping(`{"value":"lab_users","plan_id":"plan_1"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", rr.Code, rr.Body.String())
	}
	if len(h.claimed) != 1 {
		t.Fatalf("want one claim, got %d", len(h.claimed))
	}
	got := h.claimed[0]
	if got.PlanID != "plan_1" || got.Target != m.Target || got.Surface != planSurfaceMappingEdit {
		t.Errorf("the citation arrived wrong: %+v", got)
	}
	// The value being moved TO is inside the binding, or an approval to change
	// `lab_makers` → `lab_users` would be spendable on a change to anything.
	if got.RequestFingerprint != mappingRequestFingerprint(m, planSurfaceMappingEdit, "lab_users") {
		t.Error("the approval is not bound to the value it was reviewed for")
	}
	if len(h.converged) != 2 {
		t.Fatalf("every holder must be converged, got %d", len(h.converged))
	}
	for _, c := range h.converged {
		if c.Target != "truenas" || c.Actor == "" {
			t.Errorf("convergence queued wrong: %+v", c)
		}
	}
}

// A plan issued to change a value must not be spendable to withdraw it. Two
// surfaces, and the claim predicate is what separates them.
func TestAnEditApprovalCannotBeSpentOnADelete(t *testing.T) {
	h := stubMappingDeps(t, []addons.EntitlementField{{Name: "group", Type: "string[]"}})
	mappingWithHolders(t, h, "u1")
	stubResolvedIntent(t)

	rr := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodDelete, "/api/v1/targets/mappings/m1", strings.NewReader(`{"plan_id":"plan_1"}`))
	r.SetPathValue("id", "m1")
	handleDeleteRoleMapping(rr, r)

	if len(h.claimed) != 1 {
		t.Fatalf("want one claim, got %d", len(h.claimed))
	}
	if h.claimed[0].Surface != planSurfaceMappingDelete {
		t.Errorf("a delete must cite its own surface, got %q", h.claimed[0].Surface)
	}
}

// The blast-radius guard, on the surface where the blast radius is largest.
func TestAMappingEditAffectingManyNeedsTheAcknowledgement(t *testing.T) {
	t.Setenv("PLAN_COHORT_LIMIT", "2")
	h := stubMappingDeps(t, []addons.EntitlementField{{Name: "group", Type: "string[]"}})
	mappingWithHolders(t, h, "u1", "u2", "u3")

	created := 0
	orig := dbCreatePlan
	t.Cleanup(func() { dbCreatePlan = orig })
	dbCreatePlan = func(_ context.Context, p db.NewPlan) (db.Plan, error) {
		created++
		return db.Plan{ID: "plan_1"}, nil
	}

	rr := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/targets/mappings/m1/rehearse-edit",
		strings.NewReader(`{"value":"lab_users"}`))
	r.SetPathValue("id", "m1")
	handleRehearseMappingEdit(rr, r)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %d (%s)", rr.Code, rr.Body.String())
	}
	if created != 0 {
		t.Error("an unacknowledged edit must not become an approval")
	}
	if !strings.Contains(rr.Body.String(), "COHORT_ACKNOWLEDGEMENT_REQUIRED") {
		t.Errorf("the refusal must name itself: %s", rr.Body.String())
	}

	// Acknowledged, it is approvable — and the plan names every person it moves.
	rr = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodPost, "/api/v1/targets/mappings/m1/rehearse-edit",
		strings.NewReader(`{"value":"lab_users","acknowledge_scope":true}`))
	r.SetPathValue("id", "m1")
	handleRehearseMappingEdit(rr, r)
	if rr.Code != http.StatusOK || created != 1 {
		t.Fatalf("an acknowledged edit must be approvable: %d (%s)", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "u3") {
		t.Errorf("the plan must name everyone it moves: %s", rr.Body.String())
	}
}

func stubResolvedIntent(t *testing.T) {
	t.Helper()
	orig := svcResolveEntitlementsFor
	t.Cleanup(func() { svcResolveEntitlementsFor = orig })
	svcResolveEntitlementsFor = func(context.Context, string, string) (map[string]json.RawMessage, error) {
		return map[string]json.RawMessage{"enabled": json.RawMessage(`true`)}, nil
	}
}

// 7.6 — a rollback restores the bindings AND re-resolves the people they reach.
//
// Restoring alone is the failure worth naming: it changes what the roles mean
// and leaves every account converged under the reverted version, with nothing
// that would ever notice — a drift sweep sees the target holding exactly what
// Syndra last told it to hold.
func TestARollbackReResolvesEveryoneItReaches(t *testing.T) {
	h := stubMappingDeps(t, []addons.EntitlementField{{Name: "group", Type: "string[]"}})
	stubResolvedIntent(t)
	h.holders = []string{"u1", "u2"}

	list, roll := dbListRoleMappings, dbRollbackMappingVersion
	t.Cleanup(func() { dbListRoleMappings, dbRollbackMappingVersion = list, roll })

	var rolledBack []string
	dbRollbackMappingVersion = func(_ context.Context, target string, version int, actor string) error {
		rolledBack = append(rolledBack, target)
		return nil
	}
	// Two mappings on ONE role: the same holders reached twice, and the
	// convergence must still be one per person.
	dbListRoleMappings = func(context.Context, string) ([]db.RoleMapping, error) {
		return []db.RoleMapping{
			{ID: "m1", Target: "truenas", ProjectID: "pLab", RoleKey: "trained", Field: "group", Value: "lab_makers"},
			{ID: "m2", Target: "truenas", ProjectID: "pLab", RoleKey: "trained", Field: "group", Value: "fabrication"},
		}, nil
	}

	rr := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/targets/truenas/mappings/versions/2/rollback", nil)
	r.SetPathValue("target", "truenas")
	r.SetPathValue("version", "2")
	handleRollbackMappingVersion(rr, r)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", rr.Code, rr.Body.String())
	}
	if len(rolledBack) != 1 {
		t.Fatalf("the restore must happen: %v", rolledBack)
	}
	if len(h.converged) != 2 {
		t.Fatalf("want one convergence per person reached, got %d: %+v", len(h.converged), h.converged)
	}
	for _, c := range h.converged {
		if !strings.Contains(c.Reason, "v2") {
			t.Errorf("the convergence must say what caused it: %q", c.Reason)
		}
	}
	if !strings.Contains(rr.Body.String(), "queued_convergences") {
		t.Errorf("the response must say the target has not moved yet: %s", rr.Body.String())
	}
}

// A rollback whose restore fails queues nothing. The two are one transaction,
// and convergences queued against a set that was not restored would converge
// everybody to the version the operator was trying to leave.
func TestARollbackThatDidNotRestoreQueuesNothing(t *testing.T) {
	h := stubMappingDeps(t, []addons.EntitlementField{{Name: "group", Type: "string[]"}})
	h.holders = []string{"u1"}

	roll := dbRollbackMappingVersion
	t.Cleanup(func() { dbRollbackMappingVersion = roll })
	dbRollbackMappingVersion = func(context.Context, string, int, string) error {
		return db.ErrMappingNotFound
	}

	rr := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/targets/truenas/mappings/versions/9/rollback", nil)
	r.SetPathValue("target", "truenas")
	r.SetPathValue("version", "9")
	handleRollbackMappingVersion(rr, r)

	if rr.Code == http.StatusOK {
		t.Fatalf("a failed restore must not report success: %s", rr.Body.String())
	}
	if len(h.converged) != 0 {
		t.Error("nothing may be queued for a restore that did not happen")
	}
}
