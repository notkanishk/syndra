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

// stubNames names "laser" by hand and gives "truenas" a manifest saying
// TrueNAS; "door" has neither.
func stubNames(t *testing.T) {
	t.Helper()
	name, manifest, get := dbTargetName, addonsAcceptedManifest, addonsGet
	t.Cleanup(func() { dbTargetName, addonsAcceptedManifest, addonsGet = name, manifest, get })
	dbTargetName = func(_ context.Context, target string) (string, error) {
		if target == "laser" {
			return "Big laser", nil
		}
		return "", nil
	}
	addonsAcceptedManifest = func(target string) (addons.Manifest, error) {
		switch target {
		case "truenas":
			return addons.Manifest{Product: "TrueNAS", Kind: "storage", EntitlementSchema: truenasSchema}, nil
		case "laser":
			return addons.Manifest{Product: "Ruby"}, nil
		}
		return addons.Manifest{}, addons.ErrNoManifest
	}
	addonsGet = func(target string) (*addons.Addon, error) {
		if target == "zitadel" || target == "ghost" {
			return nil, addons.ErrNotRegistered
		}
		return &addons.Addon{}, nil
	}
}

// The operator's name, else the manifest's product, else the id: decided in
// one place, and nothing about a product lives in core code.
func TestASystemIsNamedByTheOperatorThenItsProductThenItsID(t *testing.T) {
	stubNames(t)
	for target, want := range map[string]struct {
		name string
		set  bool
	}{
		"laser":   {"Big laser", true},
		"truenas": {"TrueNAS", false},
		"door":    {"Door", false},
	} {
		if name, set := systemName(context.Background(), target); name != want.name || set != want.set {
			t.Errorf("%s: got (%q, %v), want (%q, %v)", target, name, set, want.name, want.set)
		}
	}
}

// Both rosters carry the name systemName decides: the operator's and the
// member's draw one system by one name.
func TestBothRostersCarryTheOneName(t *testing.T) {
	stubNames(t)
	reg := addonsRegistered
	t.Cleanup(func() { addonsRegistered = reg })
	addonsRegistered = func() []addons.Registration { return []addons.Registration{{Target: "laser"}, {Target: "door"}} }
	t.Setenv("SYNDRA_API_KEY", "test-key")

	rr := httptest.NewRecorder()
	NewRouter().ServeHTTP(rr, operatorRequest(http.MethodGet, "/api/v1/targets", ""))
	var roster struct {
		Targets []struct {
			Target  string `json:"target"`
			Name    string `json:"name"`
			NameSet bool   `json:"name_set"`
		} `json:"targets"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &roster); err != nil || len(roster.Targets) != 2 {
		t.Fatalf("roster: %d %s", rr.Code, rr.Body.String())
	}
	if got := roster.Targets[0]; got.Name != "Big laser" || !got.NameSet {
		t.Errorf("operator roster: %+v", got)
	}
	if got := roster.Targets[1]; got.Name != "Door" || got.NameSet {
		t.Errorf("operator roster: %+v", got)
	}

	stubMyStorage(t, &storageHarness{entitled: true})
	r := httptest.NewRequest(http.MethodGet, "/api/v1/me/targets", nil)
	r = r.WithContext(withPrincipal(r.Context(), &auth.Principal{Subject: "u1"}))
	view, err := describeMyTarget(r, "laser", "u1")
	if err != nil {
		t.Fatal(err)
	}
	if view.Name != "Big laser" {
		t.Errorf("member view name %q, want the operator's", view.Name)
	}
}

// One write place, strict JSON, an unregistered system refused, a blank name
// clears it, and the schema's limit is said as a sentence.
func TestNamingASystem(t *testing.T) {
	stubNames(t)
	t.Setenv("SYNDRA_API_KEY", "test-key")
	set := dbSetTargetName
	t.Cleanup(func() { dbSetTargetName = set })
	var wrote []string
	dbSetTargetName = func(_ context.Context, target, name, actor string) error {
		if len(name) > 60 {
			return db.ErrTargetNameInvalid
		}
		wrote = append(wrote, target+"="+name)
		return nil
	}
	put := func(target, body string) *httptest.ResponseRecorder {
		rr := httptest.NewRecorder()
		NewRouter().ServeHTTP(rr, operatorRequest(http.MethodPut, "/api/v1/targets/"+target+"/name", body))
		return rr
	}

	if rr := put("laser", `{"name":"Big laser","colour":"red"}`); rr.Code != http.StatusBadRequest {
		t.Errorf("an unknown field must be refused (strict JSON), got %d", rr.Code)
	}
	if rr := put("ghost", `{"name":"Ghost"}`); rr.Code != http.StatusNotFound {
		t.Errorf("an unregistered system has no name to set, got %d", rr.Code)
	}
	if rr := put("zitadel", `{"name":"Directory"}`); rr.Code != http.StatusNotFound {
		t.Errorf("Zitadel is not a connected system, got %d", rr.Code)
	}
	if rr := put("laser", `{"name":"`+strings.Repeat("x", 61)+`"}`); rr.Code != http.StatusBadRequest ||
		!strings.Contains(rr.Body.String(), `"name":"too_long"`) || !strings.Contains(rr.Body.String(), "at most 60") {
		t.Errorf("a long name is a sentence, got %d %s", rr.Code, rr.Body.String())
	}
	rr := put("laser", `{"name":"  Big laser  "}`)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"name":"Big laser"`) {
		t.Errorf("naming: %d %s", rr.Code, rr.Body.String())
	}
	if rr := put("laser", `{"name":"   "}`); rr.Code != http.StatusOK {
		t.Errorf("clearing: %d %s", rr.Code, rr.Body.String())
	}
	if strings.Join(wrote, ",") != "laser=Big laser,laser=" {
		t.Errorf("wrote %v, want the trimmed name then a clear", wrote)
	}
}
