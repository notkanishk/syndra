package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"syndra/internal/addons"
	"syndra/internal/auth"
	"syndra/internal/db"
	"syndra/internal/services"
)

// A person's network storage, for operators (A-10 Part 1).

const gb = int64(1 << 30)

type personStorageHarness struct {
	storageHarness
	asked      []string // subject/actor pairs the add-on was asked with
	noProject  bool
	noMappings bool
	adoptedAt  *time.Time
	failRead   bool
}

func stubPersonStorage(t *testing.T, h *personStorageHarness) {
	t.Helper()
	stubMyStorage(t, &h.storageHarness)
	prev := []any{addonsRegistered, dbTargetProjects, dbListRoleMappings, dbAllowancesInForce,
		dbAllowancesForSubject, addonsReadReach, dbLastSucceededAddonOperation, addonsConnection}
	t.Cleanup(func() {
		addonsRegistered = prev[0].(func() []addons.Registration)
		dbTargetProjects = prev[1].(func(context.Context) (map[string]string, error))
		dbListRoleMappings = prev[2].(func(context.Context, string) ([]db.RoleMapping, error))
		dbAllowancesInForce = prev[3].(func(context.Context, string, string) ([]db.Allowance, error))
		dbAllowancesForSubject = prev[4].(func(context.Context, string) ([]db.Allowance, error))
		addonsReadReach = prev[5].(func(context.Context, string, bool) addons.Reach)
		dbLastSucceededAddonOperation = prev[6].(func(context.Context, string, string, string) (db.AddonOperation, bool, error))
		addonsConnection = prev[7].(func(string) (*addons.Connection, error))
	})

	// The kind and the limit's unit as saved at acceptance: the in-memory
	// manifest is not what these pages read (R3, 7l).
	shape := addonsAcceptedShape
	t.Cleanup(func() { addonsAcceptedShape = shape })
	addonsAcceptedShape = func(context.Context, string) (addons.Shape, error) {
		return addons.Shape{Kind: "storage", EntitlementSchema: truenasSchema}, nil
	}

	bound := time.Date(2026, 9, 12, 10, 0, 0, 0, time.UTC)
	dbGetTargetBinding = func(context.Context, string, string) (db.TargetBinding, bool, error) {
		if !h.bound {
			return db.TargetBinding{}, false, nil
		}
		return db.TargetBinding{Target: "truenas", SubjectID: "u1", Username: "roman.reigns", BoundAt: bound}, true, nil
	}
	svcResolveEntitlementSet = func(context.Context, string, string) (services.EntitlementSet, error) {
		lead := services.RoleSource{ProjectID: "fab", RoleKey: "lead"}
		return services.EntitlementSet{Roles: truenasRoles,
			Fields:    map[string][]string{"group": {"fab_leads"}, "home": {"yes"}},
			Given:     map[string][]services.RoleSource{"home_quota\x0021474836480": {lead}, "group\x00fab_leads": {lead}},
			Lifecycle: services.LifecycleState{Account: h.entitled},
		}, nil
	}
	answerStorage(t, func(subject, actor string) addons.StorageStatus {
		h.asked = append(h.asked, subject+"/"+actor)
		if h.failRead || h.unreachable {
			return addons.StorageStatus{Outcome: addons.OutcomeUnreached}
		}
		home := addons.Place{Share: "main", Folder: "homes/roman.reigns", Path: "/mnt/main/homes/roman.reigns", Access: "change", Home: true}
		home.Usage = &struct {
			UsedBytes  int64 `json:"used_bytes"`
			QuotaBytes int64 `json:"quota_bytes,omitempty"`
		}{UsedBytes: 18 * gb, QuotaBytes: 50 * gb}
		renders := addons.Place{Share: "main", Folder: "projects/renders", Path: "/mnt/main/projects/renders", Access: "change"}
		return addons.StorageStatus{Outcome: addons.OutcomeSucceeded, Usable: true,
			PlacesReadable: true, Places: []addons.Place{home, renders}}
	})
	addonsRegistered = func() []addons.Registration { return []addons.Registration{{Target: "truenas"}} }
	dbTargetProjects = func(context.Context) (map[string]string, error) {
		if h.noProject {
			return map[string]string{}, nil
		}
		return map[string]string{"truenas": "proj-nas"}, nil
	}
	dbListRoleMappings = func(context.Context, string) ([]db.RoleMapping, error) {
		if h.noMappings {
			return nil, nil
		}
		return []db.RoleMapping{{}}, nil
	}
	review := time.Date(2026, 11, 17, 0, 0, 0, 0, time.UTC)
	dbAllowancesInForce = func(context.Context, string, string) ([]db.Allowance, error) {
		return []db.Allowance{
			{ID: "own", Field: "home_quota", Value: "53687091200", Direction: db.AllowanceSet, Reason: "Thesis renders", ActorID: "op-9", ReviewDate: &review},
			{ID: "hold", Field: "enabled", Value: "true", Direction: db.AllowanceDeny, Reason: "Lost their key card", ActorID: "op-9", ReviewDate: &review},
		}, nil
	}
	dbAllowancesForSubject = func(context.Context, string) ([]db.Allowance, error) { return nil, nil }
	addonsReadReach = func(context.Context, string, bool) addons.Reach {
		return addons.Reach{Read: readNow(), Groups: map[string][]addons.Place{
			"fab_leads": {
				{Share: "main", Folder: "projects/renders", Path: "/mnt/main/projects/renders", Access: "read"},
				{Share: "main", Folder: "projects/renders", Path: "/mnt/main/projects/renders", Access: "change"},
				{Share: "main", Folder: "homes", Path: "/mnt/main/homes/x", Access: "change", Home: true},
			},
		}}
	}
	dbLastSucceededAddonOperation = func(context.Context, string, string, string) (db.AddonOperation, bool, error) {
		if h.adoptedAt == nil {
			return db.AddonOperation{}, false, nil
		}
		return db.AddonOperation{ActorID: "op-9", CreatedAt: *h.adoptedAt}, true, nil
	}
	addonsConnection = func(string) (*addons.Connection, error) { return nil, nil }
}

func getPersonTargets(t *testing.T) personTargetView {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/u1/targets", nil)
	req.SetPathValue("id", "u1")
	req = req.WithContext(withPrincipal(req.Context(), &auth.Principal{Subject: "op-1"}))
	rr := httptest.NewRecorder()
	handleUserTargets(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rr.Code, rr.Body.String())
	}
	var body struct {
		Targets []personTargetView `json:"targets"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil || len(body.Targets) != 1 {
		t.Fatalf("body %s (%v)", rr.Body.String(), err)
	}
	return body.Targets[0]
}

func TestPersonTargets_TheMembersViewPlusTheOperatorsFacts(t *testing.T) {
	h := &personStorageHarness{storageHarness: storageHarness{entitled: true, bound: true}}
	stubPersonStorage(t, h)
	view := getPersonTargets(t)

	// The add-on is asked about the person, by the operator.
	if len(h.asked) != 1 || h.asked[0] != "u1/op-1" {
		t.Fatalf("asked %v; want subject u1 with the operator as actor", h.asked)
	}
	if view.Account == nil || view.Account.Username != "roman.reigns" || view.Setup != "" || view.Adopted != nil {
		t.Fatalf("view %+v", view)
	}
	home := view.Home
	if home == nil || !home.GivesHome || home.RolesLimit == nil || *home.RolesLimit != 20*gb ||
		home.RolesLimitFrom == nil || home.RolesLimitFrom.RoleKey != "lead" ||
		home.Own == nil || home.Own.Bytes != 50*gb || home.Own.Reason != "Thesis renders" ||
		home.UsedBytes == nil || *home.UsedBytes != 18*gb || *home.QuotaBytes != 50*gb {
		t.Fatalf("home %+v", home)
	}
	if len(view.Holds) != 1 || view.Holds[0].Field != "enabled" || view.Holds[0].Reason != "Lost their key card" || view.Holds[0].ReviewDate == nil {
		t.Fatalf("holds %+v; only the deny is a hold", view.Holds)
	}
	if view.Preview != nil {
		t.Error("a person with an account gets no preview")
	}
}

// Syndra keeps no copy (design §13.3): with the add-on down the system's read
// is none, and no storage or use is shown, however recently it answered.
func TestPersonTargets_OfflineIsNotReadNeverTheLastRead(t *testing.T) {
	h := &personStorageHarness{storageHarness: storageHarness{entitled: true, bound: true}}
	stubPersonStorage(t, h)
	if view := getPersonTargets(t); view.Read == nil || view.Read.State != addons.ReadNow || view.Storage == nil {
		t.Fatalf("online: read %+v storage %v", view.Read, view.Storage)
	}

	h.unreachable = true
	view := getPersonTargets(t)
	if view.Read == nil || view.Read.State != addons.ReadNone || view.Read.TakenAt != nil || view.Storage != nil {
		t.Fatalf("offline: want read none and no storage, got read %+v storage %v", view.Read, view.Storage)
	}
	if view.Home == nil || view.Home.UsedBytes != nil || view.Home.Home != "" {
		t.Errorf("no use shown from a read that did not happen: %+v", view.Home)
	}
}

// The add-on's kept copy is dated by the add-on's read, not by when the
// backend heard of it: a copy read four minutes ago is four minutes old.
func TestPersonTargets_AKeptCopyKeepsTheAddOnsDate(t *testing.T) {
	h := &personStorageHarness{storageHarness: storageHarness{entitled: true, bound: true}}
	stubPersonStorage(t, h)
	fixture, err := os.ReadFile(filepath.Join("..", "..", "..", "addons", "contract", "v2", "fixtures", "operation_response_result.json"))
	if err != nil {
		t.Fatal(err)
	}
	read := time.Now().Add(-4 * time.Minute).UTC().Truncate(time.Second)
	body := strings.Replace(string(fixture), `"taken_at": "2026-11-10T09:30:00Z",
    "current": true,`, `"taken_at": "`+read.Format(time.RFC3339)+`",
    "current": false, "stale": {"reason": "cached"},`, 1)
	addonsRead = func(context.Context, string, string, string, string, map[string]any, bool) addons.ReadReply {
		return addons.ReadReply{Operation: "storage.status", Schema: "profiles/storage/status.json",
			Outcome: addons.OutcomeSucceeded, Status: 200, Body: []byte(body)}
	}
	view := getPersonTargets(t)
	if view.Storage == nil || view.Read == nil || view.Read.State != addons.ReadKept || !view.Read.TakenAt.Equal(read) {
		t.Fatalf("the kept copy did not land with its date: read %+v storage %v", view.Read, view.Storage)
	}
}

func TestPersonTargets_SetupStates(t *testing.T) {
	h := &personStorageHarness{noProject: true}
	stubPersonStorage(t, h)
	if view := getPersonTargets(t); view.Setup != "no_project" {
		t.Errorf("setup %q, want no_project", view.Setup)
	}
	h.noProject, h.noMappings = false, true
	if view := getPersonTargets(t); view.Setup != "no_mapping" {
		t.Errorf("setup %q, want no_mapping", view.Setup)
	}
}

func TestPersonTargets_NoAccountYetPreviewsWhatTheirGroupsOpen(t *testing.T) {
	h := &personStorageHarness{storageHarness: storageHarness{entitled: true, recordedAt: time.Now().Add(-time.Hour)}}
	stubPersonStorage(t, h)
	view := getPersonTargets(t)
	if view.Account != nil || view.RecordedAt == nil || view.Home == nil || view.Home.RolesLimit == nil || view.Home.UsedBytes != nil {
		t.Fatalf("view %+v", view)
	}
	if len(view.Preview) != 1 {
		t.Fatalf("preview %+v; want renders once, the home folder left to its own row", view.Preview)
	}
	p := view.Preview[0]
	if p.Path != "/mnt/main/projects/renders" || p.Access != "change" || len(p.Roles) != 1 || p.Roles[0].RoleKey != "lead" {
		t.Errorf("preview place %+v", p)
	}
	if len(h.asked) != 0 {
		t.Errorf("no account, so nothing to ask the add-on about: %v", h.asked)
	}
}

func TestPersonTargets_AdoptedOnlyWhenTheAdoptionMadeThisBinding(t *testing.T) {
	h := &personStorageHarness{storageHarness: storageHarness{entitled: true, bound: true}}
	stubPersonStorage(t, h)
	just := time.Date(2026, 9, 12, 9, 59, 0, 0, time.UTC) // a minute before the binding
	h.adoptedAt = &just
	if view := getPersonTargets(t); view.Adopted == nil || view.Adopted.ActorID != "op-9" {
		t.Fatalf("adopted %+v", view.Adopted)
	}
	long := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC) // adopted once, released, made again since
	h.adoptedAt = &long
	if view := getPersonTargets(t); view.Adopted != nil {
		t.Errorf("an old adoption is not this binding's: %+v", view.Adopted)
	}
}

// A-13 §5: a member's unconfirmed password change is theirs to settle. The
// operator's view of the same system carries no line about it.
func TestPersonTargets_CarryNoPasswordChange(t *testing.T) {
	h := &personStorageHarness{storageHarness: storageHarness{entitled: true, bound: true, unconfirmed: time.Now()}}
	stubPersonStorage(t, h)
	if view := getPersonTargets(t); view.PasswordChange != nil {
		t.Fatalf("passwords never reach an operator: %+v", view.PasswordChange)
	}
}

// A target with no stored role map (its add-on has never had a manifest
// accepted) is "not read": not entitled, no resources, beside a target that
// has one, on the operator's person page; the member's own page leaves it out
// (operators-only, owner 11 Nov 2026). Never a 500 for the whole page
// (addon-contract 2.1).
func TestATargetWithNoRoleMapIsNotReadAndTheOthersStillShow(t *testing.T) {
	h := &personStorageHarness{storageHarness: storageHarness{entitled: true, bound: true}}
	stubPersonStorage(t, h)
	addonsRegistered = func() []addons.Registration { return []addons.Registration{{Target: "truenas"}, {Target: "unifi"}} }
	manifest := addonsAcceptedShape
	t.Cleanup(func() { addonsAcceptedShape = manifest })
	addonsAcceptedShape = func(ctx context.Context, target string) (addons.Shape, error) {
		if target == "truenas" {
			return addons.Shape{Product: "TrueNAS", Kind: "storage", EntitlementSchema: truenasSchema}, nil
		}
		return addons.Shape{}, addons.ErrNoManifest
	}
	svcResolveEntitlementSet = func(_ context.Context, _, target string) (services.EntitlementSet, error) {
		if target == "unifi" {
			return services.EntitlementSet{}, fmt.Errorf("resolve entitlements for u1: %w: unifi has no accepted contract-2 manifest yet", addons.ErrNoManifest)
		}
		return services.EntitlementSet{Roles: truenasRoles, Fields: map[string][]string{"group": {"staff"}},
			Lifecycle: services.LifecycleState{Account: true}}, nil
	}
	check := func(name string, rr *httptest.ResponseRecorder, want int) {
		t.Helper()
		var body struct {
			Targets []myTargetView `json:"targets"`
		}
		if rr.Code != http.StatusOK || json.Unmarshal(rr.Body.Bytes(), &body) != nil || len(body.Targets) != want {
			t.Fatalf("%s: %d %s", name, rr.Code, rr.Body.String())
		}
		for _, v := range body.Targets {
			if v.Target == "unifi" && (v.Entitled || len(v.Resources) != 0) {
				t.Errorf("%s: a target with no role map is not read: %+v", name, v)
			}
			if v.Target == "truenas" && !v.Entitled {
				t.Errorf("%s: the target with a role map must still show: %+v", name, v)
			}
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/u1/targets", nil)
	req.SetPathValue("id", "u1")
	req = req.WithContext(withPrincipal(req.Context(), &auth.Principal{Subject: "op-1"}))
	rr := httptest.NewRecorder()
	handleUserTargets(rr, req)
	check("the operator's page", rr, 2)

	req = httptest.NewRequest(http.MethodGet, "/api/v1/me/targets", nil)
	req = req.WithContext(withPrincipal(req.Context(), &auth.Principal{Subject: "u1"}))
	rr = httptest.NewRecorder()
	handleMyTargets(rr, req)
	check("the member's page", rr, 1)

	// Any other failure still fails the page: an unreadable store is not a
	// target that has not spoken.
	svcResolveEntitlementSet = func(context.Context, string, string) (services.EntitlementSet, error) {
		return services.EntitlementSet{}, errors.New("db down")
	}
	rr = httptest.NewRecorder()
	handleMyTargets(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("want 500 for a failed read, got %d", rr.Code)
	}
}

// A restart while the add-on is down (or its manifest is refused) holds no
// manifest in memory, and what Syndra holds itself still shows, as production
// showed it (7l, R1-R3): the member's card and rail row from the kind saved
// at acceptance, and on the person page the account, entitlement, roles,
// hold, the roles' limit and the own limit, the limit field found by the
// saved schema's unit. Nothing is read from the add-on, and the card says so.
func TestARestartWithTheAddonDownKeepsWhatSyndraHolds(t *testing.T) {
	h := &personStorageHarness{storageHarness: storageHarness{entitled: true, bound: true}}
	stubPersonStorage(t, h)
	callable, schema := addonsCallable, addonsEntitlementSchema
	t.Cleanup(func() { addonsCallable, addonsEntitlementSchema = callable, schema })
	// The registry after the restart: registered, no manifest accepted yet.
	// Health is still the monitor's (here answering), read without a manifest.
	addonsCallable = func(string) bool { return false }
	addonsEntitlementSchema = func(string) ([]addons.EntitlementField, error) { return nil, addons.ErrNoManifest }

	member := httptest.NewRequest(http.MethodGet, "/api/v1/me/targets", nil)
	member = member.WithContext(withPrincipal(member.Context(), &auth.Principal{Subject: "u1"}))
	rr := httptest.NewRecorder()
	handleMyTargets(rr, member)
	var mine struct {
		Targets []myTargetView `json:"targets"`
	}
	if rr.Code != http.StatusOK || json.Unmarshal(rr.Body.Bytes(), &mine) != nil || len(mine.Targets) != 1 {
		t.Fatalf("R1: the member's storage must still show: %d %s", rr.Code, rr.Body.String())
	}
	if v := mine.Targets[0]; v.Kind != "storage" || !v.Entitled || v.Account == nil || v.Storage != nil || v.Health != addons.StateAnswering {
		t.Fatalf("R1: the card from Syndra's own facts, nothing read: %+v", v)
	}
	rr = httptest.NewRecorder()
	handleMySystems(rr, member)
	if !strings.Contains(rr.Body.String(), `"kind":"storage"`) {
		t.Fatalf("R1: the rail row must still show: %s", rr.Body.String())
	}

	view := getPersonTargets(t)
	if view.Kind != "storage" || view.Account == nil || !view.Entitled || len(view.Roles) != 1 || len(view.Holds) != 1 {
		t.Fatalf("R2: the person page from Syndra's own facts: %+v", view)
	}
	if home := view.Home; home == nil || home.RolesLimit == nil || *home.RolesLimit != 20*gb || home.Own == nil || home.Own.Bytes != 50*gb {
		t.Fatalf("R3: the roles' limit and the own limit: %+v", home)
	}
	if len(h.asked) != 0 {
		t.Errorf("an add-on with no manifest was read: %v", h.asked)
	}
}
