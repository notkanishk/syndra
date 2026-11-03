package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	lastStorage.Range(func(k, _ any) bool { lastStorage.Delete(k); return true })
	prev := []any{addonsMyStorage, addonsRegistered, dbTargetProjects, dbListRoleMappings, dbAllowancesInForce,
		dbAllowancesForSubject, addonsReadReach, dbLastSucceededAddonOperation, addonsConnection}
	t.Cleanup(func() {
		addonsMyStorage = prev[0].(func(context.Context, string, string, string) addons.StorageStatus)
		addonsRegistered = prev[1].(func() []addons.Registration)
		dbTargetProjects = prev[2].(func(context.Context) (map[string]string, error))
		dbListRoleMappings = prev[3].(func(context.Context, string) ([]db.RoleMapping, error))
		dbAllowancesInForce = prev[4].(func(context.Context, string, string) ([]db.Allowance, error))
		dbAllowancesForSubject = prev[5].(func(context.Context, string) ([]db.Allowance, error))
		addonsReadReach = prev[6].(func(context.Context, string) addons.Reach)
		dbLastSucceededAddonOperation = prev[7].(func(context.Context, string, string, string) (db.AddonOperation, bool, error))
		addonsConnection = prev[8].(func(string) (*addons.Connection, error))
	})

	bound := time.Date(2026, 9, 12, 10, 0, 0, 0, time.UTC)
	dbGetTargetBinding = func(context.Context, string, string) (db.TargetBinding, bool, error) {
		if !h.bound {
			return db.TargetBinding{}, false, nil
		}
		return db.TargetBinding{Target: "truenas", SubjectID: "u1", Username: "aanya.rao", BoundAt: bound}, true, nil
	}
	svcResolveEntitlementSet = func(context.Context, string, string) (services.EntitlementSet, error) {
		lead := services.RoleSource{ProjectID: "fab", RoleKey: "lead"}
		return services.EntitlementSet{
			Fields:    map[string][]string{"group": {"fab_leads"}, "home": {"yes"}},
			Given:     map[string][]services.RoleSource{"home_quota\x0021474836480": {lead}, "group\x00fab_leads": {lead}},
			Lifecycle: services.LifecycleState{Enabled: h.entitled},
		}, nil
	}
	addonsMyStorage = func(_ context.Context, _, subject, actor string) addons.StorageStatus {
		h.asked = append(h.asked, subject+"/"+actor)
		if h.failRead || h.unreachable {
			return addons.StorageStatus{Outcome: addons.OutcomeUnreached}
		}
		home := addons.Place{Share: "main", Folder: "homes/aanya.rao", Path: "/mnt/main/homes/aanya.rao", Access: "change", Home: true}
		home.Usage = &struct {
			UsedBytes  int64 `json:"used_bytes"`
			QuotaBytes int64 `json:"quota_bytes,omitempty"`
		}{UsedBytes: 18 * gb, QuotaBytes: 50 * gb}
		renders := addons.Place{Share: "main", Folder: "projects/renders", Path: "/mnt/main/projects/renders", Access: "change"}
		return addons.StorageStatus{Outcome: addons.OutcomeSucceeded, Username: "aanya.rao", Usable: true,
			PlacesReadable: true, Places: []addons.Place{home, renders}}
	}
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
	addonsReadReach = func(context.Context, string) addons.Reach {
		return addons.Reach{Read: true, Groups: map[string][]addons.Place{
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
	if view.Account == nil || view.Account.Username != "aanya.rao" || view.Setup != "" || view.Adopted != nil {
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

func TestPersonTargets_OfflineShowsTheLastReadWithItsTime(t *testing.T) {
	h := &personStorageHarness{storageHarness: storageHarness{entitled: true, bound: true}}
	stubPersonStorage(t, h)
	getPersonTargets(t) // online: remembered

	h.unreachable = true
	view := getPersonTargets(t)
	if view.Storage != nil || view.LastKnown == nil || view.LastKnown.ReadAt.IsZero() || len(view.LastKnown.Storage.Places) != 2 {
		t.Fatalf("offline view: storage=%v last_known=%+v", view.Storage, view.LastKnown)
	}
	if view.Home == nil || view.Home.UsedBytes == nil || *view.Home.UsedBytes != 18*gb {
		t.Errorf("the home use comes from the last read: %+v", view.Home)
	}

	// Never read: no last known, and no numbers.
	lastStorage.Range(func(k, _ any) bool { lastStorage.Delete(k); return true })
	view = getPersonTargets(t)
	if view.LastKnown != nil || view.Home.UsedBytes != nil {
		t.Errorf("with no last read: last_known=%+v used=%v", view.LastKnown, view.Home.UsedBytes)
	}

	// One slow answer is not offline: no last known is offered in its place.
	h.unreachable, h.failRead = false, true
	getPersonTargets(t)
	h.failRead = false
	getPersonTargets(t)
	h.failRead = true
	if view = getPersonTargets(t); view.LastKnown != nil || view.Storage != nil {
		t.Errorf("a failed read while online: last_known=%+v", view.LastKnown)
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
	if view.Account != nil || view.RecordedAt == nil || view.Home != nil {
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
