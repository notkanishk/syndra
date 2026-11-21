package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"syndra/internal/addons"
	"syndra/internal/db"
	"syndra/internal/handlers/kinds"
	"syndra/internal/services"
)

// Each managed account's home folder limit: the largest its roles give and
// which role gives it, its own limit when one is in force, what is saved there,
// and an own limit that ended with the folder still over the roles' limit.
func TestTargetHomesSaysEachLimitAndWhereItComesFrom(t *testing.T) {
	bindings, resolve, inForce, all, homes, shape := dbListTargetBindings, svcResolveEntitlementSet, dbAllowancesInForce, dbAllowancesForSubject, addonsReadHomes, addonsAcceptedShape
	t.Cleanup(func() {
		dbListTargetBindings, svcResolveEntitlementSet, dbAllowancesInForce, dbAllowancesForSubject, addonsReadHomes, addonsAcceptedShape = bindings, resolve, inForce, all, homes, shape
	})
	// The limit's unit as saved at acceptance (acceptedShape), not the
	// manifest held in memory.
	addonsAcceptedShape = func(context.Context, string) (addons.Shape, error) {
		return addons.Shape{Kind: "storage", EntitlementSchema: truenasSchema}, nil
	}
	const gb = int64(1 << 30)
	dbListTargetBindings = func(context.Context, string) ([]db.TargetBinding, error) {
		return []db.TargetBinding{{SubjectID: "u-charlotte", Username: "charlotte"}, {SubjectID: "u-booker", Username: "booker"}}, nil
	}
	svcResolveEntitlementSet = func(_ context.Context, subject, _ string) (services.EntitlementSet, error) {
		return services.EntitlementSet{Roles: truenasRoles,
			Fields: map[string][]string{"home": {"true"}, "home_quota": {"21474836480"}},
			Given: map[string][]services.RoleSource{
				"home\x00true":              {{ProjectID: "p", RoleKey: "fab_lead"}},
				"home_quota\x0010737418240": {{ProjectID: "p", RoleKey: "member"}},
				"home_quota\x0021474836480": {{ProjectID: "p", RoleKey: "fab_lead"}},
			},
		}, nil
	}
	review := time.Now().Add(-24 * time.Hour)
	ended := time.Now().Add(-48 * time.Hour)
	dbAllowancesInForce = func(_ context.Context, subject, _ string) ([]db.Allowance, error) {
		if subject != "u-charlotte" {
			return nil, nil
		}
		return []db.Allowance{{ID: "a1", Target: "truenas", Field: "home_quota", Value: "53687091200",
			Direction: db.AllowanceSet, Reason: "thesis", ReviewDate: &review}}, nil
	}
	dbAllowancesForSubject = func(_ context.Context, subject string) ([]db.Allowance, error) {
		return []db.Allowance{{ID: "a0", Target: "truenas", Field: "home_quota", Value: "53687091200",
			Direction: db.AllowanceSet, ExpiresAt: &ended}}, nil
	}
	addonsReadHomes = func(context.Context, string, bool) addons.Homes {
		return addons.Homes{Read: true, UsageRead: true, Homes: []addons.HomeUsage{
			{Username: "charlotte", Subject: "u-charlotte", Home: "/mnt/p/main/home_dir/charlotte", UsedBytes: 18 * gb},
			// booker's account was renamed on the NAS; a stranger took the
			// name Syndra's copy recorded. The home is matched by subject.
			{Username: "booker.t", Subject: "u-booker", Home: "/mnt/p/main/home_dir/booker", UsedBytes: 34 * gb},
			{Username: "booker", Home: "/mnt/p/main/home_dir/stranger", UsedBytes: 1 * gb},
		}}
	}

	r := httptest.NewRequest(http.MethodGet, "/api/v1/targets/truenas/homes", nil)
	r.SetPathValue("target", "truenas")
	rr := httptest.NewRecorder()
	handleTargetHomes(rr, r)
	var body struct {
		Accounts []accountHome `json:"accounts"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil || len(body.Accounts) != 2 {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
	booker, charlotte := body.Accounts[0], body.Accounts[1]
	if !charlotte.GivesHome || charlotte.RolesLimit == nil || *charlotte.RolesLimit != 20*gb || charlotte.RolesLimitFrom.RoleKey != "fab_lead" {
		t.Errorf("charlotte's roles' limit: %+v from %+v", charlotte.RolesLimit, charlotte.RolesLimitFrom)
	}
	if charlotte.Own == nil || charlotte.Own.Bytes != 50*gb || !charlotte.Own.ReviewDue || charlotte.OwnEnded != nil {
		t.Errorf("charlotte's own limit: %+v ended %+v", charlotte.Own, charlotte.OwnEnded)
	}
	if booker.Own != nil || booker.OwnEnded == nil || booker.OwnEnded.Bytes != 50*gb {
		t.Errorf("booker's own limit ended at 50 GB with 34 GB in a 20 GB home: %+v", booker.OwnEnded)
	}
	if booker.UsedBytes == nil || *booker.UsedBytes != 34*gb || booker.Username != "booker.t" || booker.Home != "/mnt/p/main/home_dir/booker" {
		t.Errorf("booker's home is the one the add-on attributes to him, under its own name: %q %q %v", booker.Username, booker.Home, booker.UsedBytes)
	}
}

// A home the add-on could not count (locked) has no use, never 0 B, and the
// others keep theirs: one locked home affects only itself.
func TestALockedHomeHasNoUseAndTheOthersKeepTheirs(t *testing.T) {
	bindings, resolve, inForce, all, homes, shape := dbListTargetBindings, svcResolveEntitlementSet, dbAllowancesInForce, dbAllowancesForSubject, addonsReadHomes, addonsAcceptedShape
	t.Cleanup(func() {
		dbListTargetBindings, svcResolveEntitlementSet, dbAllowancesInForce, dbAllowancesForSubject, addonsReadHomes, addonsAcceptedShape = bindings, resolve, inForce, all, homes, shape
	})
	addonsAcceptedShape = func(context.Context, string) (addons.Shape, error) {
		return addons.Shape{Kind: "storage", EntitlementSchema: truenasSchema}, nil
	}
	dbListTargetBindings = func(context.Context, string) ([]db.TargetBinding, error) {
		return []db.TargetBinding{{SubjectID: "u-charlotte", Username: "charlotte"}, {SubjectID: "u-booker", Username: "booker"}}, nil
	}
	svcResolveEntitlementSet = func(context.Context, string, string) (services.EntitlementSet, error) {
		return services.EntitlementSet{}, nil
	}
	dbAllowancesInForce = func(context.Context, string, string) ([]db.Allowance, error) { return nil, nil }
	dbAllowancesForSubject = func(context.Context, string) ([]db.Allowance, error) { return nil, nil }
	addonsReadHomes = func(context.Context, string, bool) addons.Homes {
		return addons.Homes{Read: true, UsageRead: true, Homes: []addons.HomeUsage{
			{Username: "charlotte", Subject: "u-charlotte", Home: "/mnt/p/main/home_dir/charlotte", UsedBytes: 0, NotCounted: "locked"},
			{Username: "booker", Subject: "u-booker", Home: "/mnt/p/main/home_dir/booker", UsedBytes: 400},
		}}
	}
	r := httptest.NewRequest(http.MethodGet, "/api/v1/targets/truenas/homes", nil)
	r.SetPathValue("target", "truenas")
	rr := httptest.NewRecorder()
	handleTargetHomes(rr, r)
	var body struct {
		UsageRead bool          `json:"usage_read"`
		Accounts  []accountHome `json:"accounts"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil || len(body.Accounts) != 2 {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
	booker, charlotte := body.Accounts[0], body.Accounts[1]
	if charlotte.UsedBytes != nil {
		t.Errorf("a locked home reads %d bytes, want no use", *charlotte.UsedBytes)
	}
	if !body.UsageRead || booker.UsedBytes == nil || *booker.UsedBytes != 400 {
		t.Errorf("the home beside a locked one: usage_read %v, %v", body.UsageRead, booker.UsedBytes)
	}
}

// "No limit" is the largest limit, as the add-on applies it: beside a 20 GB
// role it wins, and it never reads as a 0-byte limit the folder is "over".
func TestNoLimitIsTheLargestRolesLimit(t *testing.T) {
	set := services.EntitlementSet{Roles: truenasRoles, Given: map[string][]services.RoleSource{
		"home_quota\x0021474836480": {{ProjectID: "p", RoleKey: "member"}},
		"home_quota\x000":           {{ProjectID: "p", RoleKey: "staff"}},
	}}
	limit, from := largestGiven(set, truenasSchema[2])
	if limit != nil || from == nil || from.RoleKey != "staff" {
		t.Fatalf("want no limit, from staff; got %v from %+v", limit, from)
	}
}

// The storage kind reads its home folder and limit by role and unit, never by
// name (addon-contract 2.4): a target calling them `folder` and `space` reads
// the same; a limit counted in something other than bytes is no home limit.
func TestTheHomeLimitIsReadByRoleAndUnit(t *testing.T) {
	roles := addons.Roles{"folder": addons.RoleFlag, "space": addons.RoleLimit, "active": addons.RoleAccount}
	space := addons.EntitlementField{Name: "space", Role: addons.RoleLimit, Unit: "bytes", Combine: "max", Unlimited: "0"}
	set := services.EntitlementSet{Roles: roles, Fields: map[string][]string{"folder": {"true"}},
		Given: map[string][]services.RoleSource{"space\x005368709120": {{ProjectID: "p", RoleKey: "member"}}}}

	var row accountHome
	limit, ok := kinds.HomeLimit([]addons.EntitlementField{{Name: "folder", Role: addons.RoleFlag}, space})
	roleLimits(&row, set, limit)
	if !ok || !row.GivesHome || row.RolesLimit == nil || *row.RolesLimit != 5368709120 {
		t.Fatalf("read by role: gives %v, limit %v", row.GivesHome, row.RolesLimit)
	}

	counted := space
	counted.Unit = "count"
	if _, ok := kinds.HomeLimit([]addons.EntitlementField{counted}); ok {
		t.Fatal("a limit not in bytes is not a home folder's limit")
	}
}

// An add-on older than attribution: its homes say whose none is, so none is
// shown as anybody's. Not read, with the same addon_outdated state the
// reconcile pass records, never blank homes read as nobody's.
func TestHomesFromAnOutdatedAddOnAreNotShownAsAnybodys(t *testing.T) {
	bindings, resolve, inForce, all, homes, shape := dbListTargetBindings, svcResolveEntitlementSet, dbAllowancesInForce, dbAllowancesForSubject, addonsReadHomes, addonsAcceptedShape
	t.Cleanup(func() {
		dbListTargetBindings, svcResolveEntitlementSet, dbAllowancesInForce, dbAllowancesForSubject, addonsReadHomes, addonsAcceptedShape = bindings, resolve, inForce, all, homes, shape
	})
	addonsAcceptedShape = func(context.Context, string) (addons.Shape, error) {
		return addons.Shape{Kind: "storage", EntitlementSchema: truenasSchema}, nil
	}
	dbListTargetBindings = func(context.Context, string) ([]db.TargetBinding, error) {
		return []db.TargetBinding{{SubjectID: "u-booker", Username: "booker"}}, nil
	}
	dbAllowancesInForce = func(context.Context, string, string) ([]db.Allowance, error) { return nil, nil }
	dbAllowancesForSubject = func(context.Context, string) ([]db.Allowance, error) { return nil, nil }
	svcResolveEntitlementSet = func(context.Context, string, string) (services.EntitlementSet, error) {
		return services.EntitlementSet{Roles: truenasRoles}, nil
	}
	addonsReadHomes = func(context.Context, string, bool) addons.Homes {
		return addons.Homes{Reason: db.UnreconciledAddonOutdated}
	}

	r := httptest.NewRequest(http.MethodGet, "/api/v1/targets/truenas/homes", nil)
	r.SetPathValue("target", "truenas")
	rr := httptest.NewRecorder()
	handleTargetHomes(rr, r)
	var body struct {
		Read     bool          `json:"read"`
		Reason   string        `json:"reason"`
		Accounts []accountHome `json:"accounts"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Read || body.Reason != db.UnreconciledAddonOutdated || len(body.Accounts) != 1 || body.Accounts[0].Home != "" {
		t.Fatalf("want not read, addon_outdated, no home shown: %s", rr.Body.String())
	}
}
