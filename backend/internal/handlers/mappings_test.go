package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"syndra/internal/addons"
	"syndra/internal/db"
)

// 7.4, 8.6/8.10 — the surfaces around mappings and allowances, and the split
// validation that decides what each side may refuse.

type mappingHarness struct {
	created  []db.RoleMapping
	resolved []string
	valueErr error
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
	return h
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
	create := dbCreateAllowance
	t.Cleanup(func() { dbCreateAllowance = create })
	var seen db.Allowance
	dbCreateAllowance = func(_ context.Context, a db.Allowance) (db.Allowance, error) {
		seen = a
		a.ID = "a1"
		return a, nil
	}

	rr := httptest.NewRecorder()
	handleCreateAllowance(rr, httptest.NewRequest(http.MethodPost, "/api/v1/allowances", strings.NewReader(
		`{"subject_id":"u1","target":"truenas","field":"group","value":"lab_makers","direction":"deny","reason":"safety review","expires_at":"2026-12-01T00:00:00Z"}`)))

	if rr.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d (%s)", rr.Code, rr.Body.String())
	}
	// The actor is exactly the resolved principal, and nothing else on the
	// request can become it. Asserting "not empty" would pass for any field the
	// caller controls.
	if seen.ActorID != resolveActor(httptest.NewRequest(http.MethodPost, "/", nil), "operator") {
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
