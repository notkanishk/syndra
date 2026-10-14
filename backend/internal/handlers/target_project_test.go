package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"syndra/internal/addons"
	"syndra/internal/db"
	"syndra/internal/models"
)

// closeTheGate puts the real rule back for one test: "truenas" has no project.
// It returns how often the gate was consulted, so a route that never asks is
// told apart from one that asked and was let through.
func closeTheGate(t *testing.T) *int {
	t.Helper()
	t.Setenv("SYNDRA_API_KEY", "test-key")
	require, resolve := dbRequireTargetProject, resolveMappingProject
	t.Cleanup(func() { dbRequireTargetProject, resolveMappingProject = require, resolve })
	asked := new(int)
	dbRequireTargetProject = func(_ context.Context, target string) error {
		*asked++
		if target == db.TargetZitadel {
			return nil
		}
		return db.ErrProjectRequired
	}
	resolveMappingProject = mappingProject
	return asked
}

func operatorRequest(method, path, body string) *http.Request {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer test-key")
	return r
}

// Every write that GIVES access on a target refuses until the target has its
// project, through the real router, so the wiring is what is tested.
func TestEveryGivingWriteWaitsForTheProject(t *testing.T) {
	closeTheGate(t)
	mux := NewRouter()
	for _, route := range []struct{ method, path, body string }{
		{"POST", "/api/v1/targets/truenas/propagations/drain", `{}`},
		{"POST", "/api/v1/targets/truenas/mappings/versions/3/rehearse-rollback", `{}`},
		{"POST", "/api/v1/targets/truenas/mappings/versions/3/rollback", `{}`},
		{"POST", "/api/v1/targets/truenas/binding-conflicts/c1/resolve", `{}`},
		{"POST", "/api/v1/targets/truenas/reconcile", `{}`},
		{"POST", "/api/v1/targets/truenas/inventory/ada/adopt", `{}`},
		{"POST", "/api/v1/targets/truenas/merge-findings/f1/resolve", `{}`},
		{"POST", "/api/v1/targets/truenas/entitlements/rehearse", `{}`},
		{"POST", "/api/v1/targets/truenas/entitlements/apply", `{}`},
		{"POST", "/api/v1/targets/mappings", `{"target":"truenas","role_key":"maker","field":"group","value":"Community"}`},
		{"POST", "/api/v1/targets/mappings/rehearse-create", `{"target":"truenas","role_key":"maker","field":"group","value":"Community"}`},
		{"POST", "/api/v1/targets/mappings/versions", `{"target":"truenas","note":"why"}`},
		{"POST", "/api/v1/allowances", `{"subject_id":"u1","target":"truenas","field":"group","value":"Community","direction":"allow","reason":"x"}`},
		{"GET", "/api/v1/targets/mappings?target=truenas", ``},
	} {
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, operatorRequest(route.method, route.path, route.body))
		if rr.Code != http.StatusConflict || !strings.Contains(rr.Body.String(), "PROJECT_REQUIRED") {
			t.Errorf("%s %s: want 409 PROJECT_REQUIRED, got %d (%s)", route.method, route.path, rr.Code, rr.Body.String())
		}
	}
}

// Taking access away never waits: a safety action gated on configuration is a
// person keeping access nobody can remove.
func TestTakingAccessAwayNeverWaitsForTheProject(t *testing.T) {
	asked := closeTheGate(t)
	mux := NewRouter()
	for _, route := range []struct{ method, path, body string }{
		{"POST", "/api/v1/targets/truenas/users/u1/revoke-access", `{}`},
		{"POST", "/api/v1/targets/truenas/accounts/dormant/sweep", `{}`},
		{"POST", "/api/v1/targets/truenas/bindings/u1/release", `{}`},
		{"POST", "/api/v1/targets/truenas/lifecycle", `{}`},
		{"POST", "/api/v1/allowances", `{"subject_id":"u1","target":"truenas","field":"group","value":"Community","direction":"deny","reason":"x"}`},
	} {
		*asked = 0
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, operatorRequest(route.method, route.path, route.body))
		if *asked != 0 || strings.Contains(rr.Body.String(), "PROJECT_REQUIRED") {
			t.Errorf("%s %s must not wait for a project, got %d (%s)", route.method, route.path, rr.Code, rr.Body.String())
		}
		if rr.Code == http.StatusUnauthorized || strings.Contains(rr.Body.String(), "auth configuration") {
			t.Fatalf("%s %s never reached its handler: %s", route.method, route.path, rr.Body.String())
		}
	}
}

func TestAMappingTakesItsTargetsProject(t *testing.T) {
	require, project := dbRequireTargetProject, dbTargetProject
	t.Cleanup(func() { dbRequireTargetProject, dbTargetProject = require, project })
	dbRequireTargetProject = func(context.Context, string) error { return nil }
	dbTargetProject = func(context.Context, string) (string, error) { return "p-net", nil }

	if got, err := mappingProject(context.Background(), "truenas", ""); err != nil || got != "p-net" {
		t.Fatalf("an unnamed project is the target's, got %q %v", got, err)
	}
	if _, err := mappingProject(context.Background(), "truenas", "p-laser"); !errors.Is(err, db.ErrMappingInvalid) {
		t.Fatalf("a role from another project must be refused in words, got %v", err)
	}
}

func setProject(t *testing.T, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodPut, "/api/v1/targets/"+target+"/project", strings.NewReader(body))
	r.SetPathValue("target", target)
	rr := httptest.NewRecorder()
	handleSetTargetProject(rr, r)
	return rr
}

func TestSettingTheProjectNamesWhyItRefuses(t *testing.T) {
	projects, set := directoryProjects, dbSetTargetProject
	t.Cleanup(func() { directoryProjects, dbSetTargetProject = projects, set })
	directoryProjects = func(context.Context) ([]models.ProjectCatalog, error) {
		return []models.ProjectCatalog{{ID: "p-net", Name: "Network storage"}}, nil
	}
	get := addonsGet
	t.Cleanup(func() { addonsGet = get })
	addonsGet = func(target string) (*addons.Addon, error) {
		if target != "truenas" {
			return nil, addons.ErrNotRegistered
		}
		return &addons.Addon{}, nil
	}

	if rr := setProject(t, "zitadel", `{"project_id":"p-net"}`); rr.Code != http.StatusConflict {
		t.Errorf("zitadel serves no project, got %d", rr.Code)
	}
	if rr := setProject(t, "truenas", `{"project_id":"p-ghost"}`); rr.Code != http.StatusBadRequest {
		t.Errorf("an unknown project is refused, got %d", rr.Code)
	}

	dbSetTargetProject = func(context.Context, string, string) error { return db.ErrProjectTaken{Target: "door"} }
	if rr := setProject(t, "truenas", `{"project_id":"p-net"}`); rr.Code != http.StatusConflict || !strings.Contains(rr.Body.String(), "door") {
		t.Errorf("a taken project names its system, got %d %s", rr.Code, rr.Body.String())
	}
	dbSetTargetProject = func(context.Context, string, string) error { return db.ErrMappingsExist{Count: 2} }
	if rr := setProject(t, "truenas", `{"project_id":"p-net"}`); rr.Code != http.StatusConflict || !strings.Contains(rr.Body.String(), "MAPPINGS_EXIST") {
		t.Errorf("mappings in the way are named with their count, got %d %s", rr.Code, rr.Body.String())
	}
	dbSetTargetProject = func(context.Context, string, string) error { return nil }
	if rr := setProject(t, "truenas", `{"project_id":"p-net"}`); rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "Network storage") {
		t.Errorf("set returns the project by name, got %d %s", rr.Code, rr.Body.String())
	}
}

// Connected systems shows each system's project read-only: named when Zitadel
// answers, by id when it doesn't, and null until chosen.
func TestTheRosterCarriesEachSystemsProject(t *testing.T) {
	reg, tc, projects, catalog := addonsRegistered, addonsTransportCredentials, dbTargetProjects, directoryProjects
	t.Cleanup(func() {
		addonsRegistered, addonsTransportCredentials, dbTargetProjects, directoryProjects = reg, tc, projects, catalog
	})
	addonsRegistered = func() []addons.Registration {
		return []addons.Registration{{Target: "truenas"}, {Target: "door"}}
	}
	addonsTransportCredentials = func() []addons.TransportCredential { return nil }
	dbTargetProjects = func(context.Context) (map[string]string, error) { return map[string]string{"truenas": "p-net"}, nil }
	directoryProjects = func(context.Context) ([]models.ProjectCatalog, error) {
		return []models.ProjectCatalog{{ID: "p-net", Name: "Network storage"}}, nil
	}

	rr := httptest.NewRecorder()
	handleListTargets(rr, httptest.NewRequest(http.MethodGet, "/api/v1/targets", nil))
	var body struct {
		Targets []struct {
			Target  string            `json:"target"`
			Project *targetProjectRef `json:"project"`
		} `json:"targets"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	got := map[string]*targetProjectRef{}
	for _, tg := range body.Targets {
		got[tg.Target] = tg.Project
	}
	if p := got["truenas"]; p == nil || p.ID != "p-net" || p.Name != "Network storage" {
		t.Errorf("truenas must carry its named project, got %+v", p)
	}
	if got["door"] != nil {
		t.Errorf("a system with no project carries null, got %+v", got["door"])
	}
}
