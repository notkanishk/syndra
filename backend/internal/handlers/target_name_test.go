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
	"syndra/internal/services"
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
	for target, want := range map[string]struct{ name, from string }{
		"laser":   {"Big laser", "operator"},
		"truenas": {"TrueNAS", "product"},
		"door":    {"Door", "id"},
	} {
		if name, from := systemName(context.Background(), target); name != want.name || from != want.from {
			t.Errorf("%s: got (%q, %q), want (%q, %q)", target, name, from, want.name, want.from)
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
			Target   string `json:"target"`
			Name     string `json:"name"`
			NameFrom string `json:"name_from"`
		} `json:"targets"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &roster); err != nil || len(roster.Targets) != 2 {
		t.Fatalf("roster: %d %s", rr.Code, rr.Body.String())
	}
	if got := roster.Targets[0]; got.Name != "Big laser" || got.NameFrom != "operator" {
		t.Errorf("operator roster: %+v", got)
	}
	if got := roster.Targets[1]; got.Name != "Door" || got.NameFrom != "id" {
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
	stored := "Big laser"
	dbSetTargetName = func(_ context.Context, target, name, actor string) error {
		if len(name) > 60 {
			return db.ErrTargetNameInvalid
		}
		wrote = append(wrote, target+"="+name)
		stored = name
		return nil
	}
	// The stored name follows the writes, so a clear reads back as the product.
	dbTargetName = func(context.Context, string) (string, error) { return stored, nil }
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
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"name":"Big laser"`) ||
		!strings.Contains(rr.Body.String(), `"name_from":"operator"`) {
		t.Errorf("naming: %d %s", rr.Code, rr.Body.String())
	}
	if rr := put("laser", `{"name":"   "}`); rr.Code != http.StatusOK ||
		!strings.Contains(rr.Body.String(), `"name_from":"product"`) {
		t.Errorf("clearing: %d %s", rr.Code, rr.Body.String())
	}
	if strings.Join(wrote, ",") != "laser=Big laser,laser=" {
		t.Errorf("wrote %v, want the trimmed name then a clear", wrote)
	}
}

// A member meets a system only once its kind is known: the rail's read and
// the full view both leave a not-read registration out, so neither payload
// carries it. The rail's read is the registry and the database alone, sorted
// by name the way a person reads them ("big mill" before "TrueNAS", which
// byte order would reverse), and never calls an add-on.
func TestAMemberMeetsOnlySystemsWhoseKindIsKnown(t *testing.T) {
	stubNames(t)
	manifest, reg := addonsAcceptedManifest, addonsRegistered
	t.Cleanup(func() { addonsAcceptedManifest, addonsRegistered = manifest, reg })
	addonsAcceptedManifest = func(target string) (addons.Manifest, error) {
		if target == "mill" {
			return addons.Manifest{Product: "big mill", Kind: "machine"}, nil
		}
		return manifest(target)
	}
	// laser has a product but no kind, door no manifest at all: both not read.
	// Out of name order, so the order the rail gets is the sort's.
	addonsRegistered = func() []addons.Registration {
		return []addons.Registration{{Target: "door"}, {Target: "truenas"}, {Target: "laser"}, {Target: "mill"}}
	}
	h := &storageHarness{entitled: true}
	stubMyStorage(t, h)
	resolve := svcResolveEntitlementSet
	var called []string
	svcResolveEntitlementSet = func(ctx context.Context, subject, target string) (services.EntitlementSet, error) {
		called = append(called, target)
		return resolve(ctx, subject, target)
	}
	member := func(path string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		return r.WithContext(withPrincipal(r.Context(), &auth.Principal{Subject: "u1"}))
	}

	rr := httptest.NewRecorder()
	handleMySystems(rr, member("/api/v1/me/systems"))
	if want := `{"systems":[{"target":"mill","name":"big mill","kind":"machine"},{"target":"truenas","name":"TrueNAS","kind":"storage"}]}`; strings.TrimSpace(rr.Body.String()) != want {
		t.Errorf("GET /me/systems:\n got %s\nwant %s", rr.Body.String(), want)
	}
	if len(called) != 0 || len(h.dispatched) != 0 {
		t.Errorf("the rail's read resolved or called something: %v %v", called, h.dispatched)
	}

	rr = httptest.NewRecorder()
	handleMyTargets(rr, member("/api/v1/me/targets"))
	var mine struct {
		Targets []struct {
			Target string `json:"target"`
		} `json:"targets"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &mine); err != nil || rr.Code != http.StatusOK {
		t.Fatalf("GET /me/targets: %d %s", rr.Code, rr.Body.String())
	}
	var got []string
	for _, v := range mine.Targets {
		got = append(got, v.Target)
	}
	if strings.Join(got, ",") != "truenas,mill" {
		t.Errorf("GET /me/targets carried %v, want only the systems whose kind is known", got)
	}
}
