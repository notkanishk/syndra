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

	"syndra/internal/addons"
	"syndra/internal/auth"
	"syndra/internal/db"
	"syndra/internal/services"
)

// 7.4, 8.6/8.10 — the surfaces around mappings and allowances, and the split
// validation that decides what each side may refuse.

type mappingHarness struct {
	created  []db.RoleMapping
	resolved []string
	valueErr error
	// What the check was able to establish. The zero value is "nobody could be
	// asked", which is the honest default for a harness with no add-on.
	resolution addons.Resolution

	// The apply half: who holds the role, what the edit did, and what it queued.
	holders []string
	// Per-role holders, keyed "project\x00role", for the tests where the whole
	// point is that two roles reach different people. Falls back to `holders`.
	holdersBy map[string][]string
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
	addonsResolvesValue = func(_ context.Context, target, field, value string) (addons.Resolution, error) {
		h.resolved = append(h.resolved, target+"|"+field+"|"+value)
		return h.resolution, h.valueErr
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

	dbMappingHolders = func(_ context.Context, project, role string) ([]string, error) {
		if who, ok := h.holdersBy[project+"\x00"+role]; ok {
			return who, nil
		}
		return h.holders, nil
	}
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
//
// The sentinel is the ADD-ON's, and it has to be. This test used to raise a
// look-alike declared in the handler package, which nothing in the production
// path ever wrapped — so the test asserted a 400 while a real typo'd group name
// came back 500 DB_ERROR, and the two agreed with each other rather than with
// the system.
func TestAnUnresolvableValueIsRejectedAfterTheAddonSaysSo(t *testing.T) {
	h := stubMappingDeps(t, []addons.EntitlementField{{Name: "group", Type: "string[]"}})
	h.valueErr = fmt.Errorf("%w: truenas has no group named %q", addons.ErrValueNotResolvable, "no_such_group")

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

	list, roll := dbListRoleMappings, dbRollbackMappingVersion
	t.Cleanup(func() { dbListRoleMappings, dbRollbackMappingVersion = list, roll })
	// Read before the restore now, because the restore is what removes the
	// mappings whose holders would otherwise be missed. Read-only either way.
	dbListRoleMappings = func(context.Context, string) ([]db.RoleMapping, error) {
		return []db.RoleMapping{
			{ID: "m1", Target: "truenas", ProjectID: "pLab", RoleKey: "trained", Field: "group", Value: "lab_makers"},
		}, nil
	}
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

// A refusal the target gave clearly must not reach an operator as an internal
// error. Guarded at the source rather than only through the handler: the bug
// was a look-alike sentinel, and a second one would pass every behavioural test
// while breaking the classification again.
func TestTheMappingErrorsAreClassifiedOnTheAddonsOwnSentinels(t *testing.T) {
	src, err := os.ReadFile("mappings.go")
	if err != nil {
		t.Fatalf("read source: %v", err)
	}
	if regexp.MustCompile(`var errMappingValue\w* = errors\.New`).MatchString(string(src)) {
		t.Error("a locally declared value-refusal sentinel is one nothing in the production path wraps")
	}
	if !strings.Contains(string(src), "addons.ErrValueNotResolvable") {
		t.Error("the value refusal must be classified on the add-on package's own sentinel")
	}
}

// A rollback restores a SET, so it reaches everybody the set moved — including
// the people whose mapping it deletes.
//
// `RollbackMappingVersion` clears the whole working set and reinserts the
// version's entries, and the convergence loop then read the mappings that
// REMAIN. A person holding only a role whose mapping the rollback removes was
// in no list it walked: nothing was queued for them, and their account kept
// what that mapping granted until a sweep happened to notice, up to six hours
// later.
//
// Losing an entitlement is as much a change as gaining one, and it is the half
// an operator is less likely to check.
func TestARollbackReconvergesTheHoldersOfWhatItDeletes(t *testing.T) {
	h := stubMappingDeps(t, []addons.EntitlementField{{Name: "group", Type: "string[]"}})
	stubResolvedIntent(t)
	h.holdersBy = map[string][]string{
		"pLab\x00trained":  {"u1", "u2"},
		"pArchive\x00lead": {"u3"},
	}

	list, roll := dbListRoleMappings, dbRollbackMappingVersion
	t.Cleanup(func() { dbListRoleMappings, dbRollbackMappingVersion = list, roll })

	// Before the restore the working copy holds both. After it, only the first:
	// the archive mapping is what the rollback removes.
	restored := false
	dbRollbackMappingVersion = func(context.Context, string, int, string) error {
		restored = true
		return nil
	}
	dbListRoleMappings = func(context.Context, string) ([]db.RoleMapping, error) {
		if restored {
			return []db.RoleMapping{
				{ID: "m1", Target: "truenas", ProjectID: "pLab", RoleKey: "trained", Field: "group", Value: "lab_makers"},
			}, nil
		}
		return []db.RoleMapping{
			{ID: "m1", Target: "truenas", ProjectID: "pLab", RoleKey: "trained", Field: "group", Value: "lab_makers"},
			{ID: "m2", Target: "truenas", ProjectID: "pArchive", RoleKey: "lead", Field: "group", Value: "archive"},
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

	got := map[string]bool{}
	for _, c := range h.converged {
		got[c.SubjectID] = true
	}
	// u3 held only the deleted mapping's role. They are the whole test.
	for _, who := range []string{"u1", "u2", "u3"} {
		if !got[who] {
			t.Errorf("%s was moved by this rollback and was not reconverged: %+v", who, h.converged)
		}
	}
	if len(h.converged) != 3 {
		t.Errorf("one convergence per person reached, got %d: %+v", len(h.converged), h.converged)
	}
}

// A rollback rehearses, and its cohort is the union.
//
// It was the one mapping change that did not rehearse, which made the screen's
// own promise — every change here is rehearsed before it lands — untrue for the
// change that can move the most people.
//
// The number it states is distinct PEOPLE across the roles the working copy
// reaches and the roles the version reaches. Per-mapping counts cannot be added
// up: two mappings on one role reach the same people, and somebody whose
// mapping the rollback deletes appears in the current set and in no version.
func TestARollbackRehearsesTheUnionOfBothSets(t *testing.T) {
	h := stubMappingDeps(t, []addons.EntitlementField{{Name: "group", Type: "string[]"}})
	h.holdersBy = map[string][]string{
		// Overlaps deliberately: u2 holds both, so the honest count is three.
		"pLab\x00trained":  {"u1", "u2"},
		"pArchive\x00lead": {"u2", "u3"},
	}

	list, hist := dbListRoleMappings, dbListMappingHistory
	t.Cleanup(func() { dbListRoleMappings, dbListMappingHistory = list, hist })

	// The working copy reaches pArchive/lead. Version 2 reaches pLab/trained.
	// Neither set alone is the cohort.
	dbListRoleMappings = func(context.Context, string) ([]db.RoleMapping, error) {
		return []db.RoleMapping{
			{ID: "m2", Target: "truenas", ProjectID: "pArchive", RoleKey: "lead", Field: "group", Value: "archive"},
		}, nil
	}
	dbListMappingHistory = func(context.Context, string) (db.MappingHistory, error) {
		return db.MappingHistory{
			Target: "truenas", CurrentVersion: 2,
			Versions: []db.MappingVersion{{
				Version: 2,
				Entries: []db.MappingVersionEntry{
					{ProjectID: "pLab", RoleKey: "trained", Field: "group", Value: "lab_makers"},
				},
			}},
		}, nil
	}

	rr := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost,
		"/api/v1/targets/truenas/mappings/versions/2/rehearse-rollback",
		strings.NewReader(`{"acknowledge_scope":true}`))
	r.SetPathValue("target", "truenas")
	r.SetPathValue("version", "2")
	handleRehearseMappingRollback(rr, r)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", rr.Code, rr.Body.String())
	}
	var plan services.BulkPlan
	if err := json.Unmarshal(rr.Body.Bytes(), &plan); err != nil {
		t.Fatalf("decode plan: %v", err)
	}

	// Three, not four: u2 is one person however many roles reach them.
	if plan.Summary.Apply != 3 {
		t.Errorf("want three distinct people, got %d: %+v", plan.Summary.Apply, plan.Outcomes)
	}
	seen := map[string]int{}
	for _, o := range plan.Outcomes {
		seen[o.UserID]++
	}
	for _, who := range []string{"u1", "u2", "u3"} {
		if seen[who] != 1 {
			t.Errorf("%s should appear exactly once, appeared %d times", who, seen[who])
		}
	}
	// And nothing was written by rehearsing it.
	if len(h.converged) != 0 {
		t.Errorf("a rehearsal queues nothing: %+v", h.converged)
	}
}

// The same ceremony, at the same threshold, from the same place. A rollback
// that skipped it would be the largest change on the screen asking for the
// least.
func TestARollbackTooLargeIsRefusedUntilAcknowledged(t *testing.T) {
	stubMappingDeps(t, []addons.EntitlementField{{Name: "group", Type: "string[]"}})

	many := make([]string, 40)
	for i := range many {
		many[i] = fmt.Sprintf("u%02d", i)
	}
	list, hist := dbListRoleMappings, dbListMappingHistory
	t.Cleanup(func() { dbListRoleMappings, dbListMappingHistory = list, hist })
	dbListRoleMappings = func(context.Context, string) ([]db.RoleMapping, error) {
		return []db.RoleMapping{
			{ID: "m1", Target: "truenas", ProjectID: "pLab", RoleKey: "trained", Field: "group", Value: "lab_makers"},
		}, nil
	}
	dbListMappingHistory = func(context.Context, string) (db.MappingHistory, error) {
		return db.MappingHistory{Target: "truenas", CurrentVersion: 2,
			Versions: []db.MappingVersion{{Version: 2}}}, nil
	}
	holders := dbMappingHolders
	t.Cleanup(func() { dbMappingHolders = holders })
	dbMappingHolders = func(context.Context, string, string) ([]string, error) { return many, nil }

	rr := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost,
		"/api/v1/targets/truenas/mappings/versions/2/rehearse-rollback", strings.NewReader(`{}`))
	r.SetPathValue("target", "truenas")
	r.SetPathValue("version", "2")
	handleRehearseMappingRollback(rr, r)

	// 422 and COHORT_ACKNOWLEDGEMENT_REQUIRED, which is what every other
	// mapping change already answers — the board's caption labels this step
	// "409 · COHORT_LIMIT" and is wrong about both. The surface branches on the
	// code and never on the status, so the label was never load-bearing.
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %d (%s)", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "COHORT_ACKNOWLEDGEMENT_REQUIRED") {
		t.Errorf("the surface branches on this code: %s", rr.Body.String())
	}
	// The number it computed, so the ceremony can state it.
	if !strings.Contains(rr.Body.String(), "40") {
		t.Errorf("the refusal must carry the count it computed: %s", rr.Body.String())
	}
}

// A value the target does not recognise is answered by naming what it might
// have been, never by a retry.
//
// The refusal is deterministic: the same question gets the same answer, so a
// "try again" is the one response that cannot help, and an operator handed one
// presses it twice before reading. What helps is seeing the two names that do
// exist beside the one that does not.
func TestARefusedValueNamesWhatItMightHaveBeen(t *testing.T) {
	h := stubMappingDeps(t, []addons.EntitlementField{{Name: "group", Type: "string[]"}})
	h.resolution = addons.Resolution{
		Checked: true,
		Known:   []string{"fabrication", "fabrication-leads", "archive", "lab_makers"},
	}
	h.valueErr = fmt.Errorf("%w: truenas has no group named %q",
		addons.ErrValueNotResolvable, "fabrication-2026")

	rr := postMapping(`{"target":"truenas","project_id":"pLab","role_key":"maker","field":"group","value":"fabrication-2026"}`)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d (%s)", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	for _, near := range []string{"fabrication-leads", "fabrication"} {
		if !strings.Contains(body, near) {
			t.Errorf("the refusal must name %q as a candidate: %s", near, body)
		}
	}
	// And not every group on the NAS: a haystack is not a suggestion.
	if strings.Contains(body, "lab_makers") {
		t.Errorf("an unrelated name is not a near miss: %s", body)
	}
}

// The other half of the pair, and the one that would otherwise read as a bug:
// the add-on could not be asked, the edit is allowed through, and the surface
// has to be able to say so. "Checked and fine" and "nobody could be asked" both
// arrived as success before this.
func TestARehearsalSaysWhenTheValueCouldNotBeChecked(t *testing.T) {
	h := stubMappingDeps(t, []addons.EntitlementField{{Name: "group", Type: "string[]"}})
	h.resolution = addons.Resolution{Checked: false}
	stubResolvedIntent(t)

	get := dbGetRoleMapping
	t.Cleanup(func() { dbGetRoleMapping = get })
	dbGetRoleMapping = func(context.Context, string) (db.RoleMapping, error) {
		return db.RoleMapping{
			ID: "m1", Target: "truenas", ProjectID: "pLab", RoleKey: "maker",
			Field: "group", Value: "lab_makers",
		}, nil
	}

	rr := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/targets/mappings/m1/rehearse-edit",
		strings.NewReader(`{"value":"archive-write"}`))
	r.SetPathValue("id", "m1")
	handleRehearseMappingEdit(rr, r)

	if rr.Code != http.StatusOK {
		t.Fatalf("the edit is allowed through: want 200, got %d (%s)", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"value_checked":false`) {
		t.Errorf("the plan must say the check did not run: %s", rr.Body.String())
	}
}
