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
	"syndra/internal/services"
)

// Each managed account's home folder limit: the largest its roles give and
// which role gives it, its own limit when one is in force, what is saved there,
// and an own limit that ended with the folder still over the roles' limit.
func TestTargetHomesSaysEachLimitAndWhereItComesFrom(t *testing.T) {
	bindings, resolve, inForce, all, homes := dbListTargetBindings, svcResolveEntitlementSet, dbAllowancesInForce, dbAllowancesForSubject, addonsReadHomes
	t.Cleanup(func() {
		dbListTargetBindings, svcResolveEntitlementSet, dbAllowancesInForce, dbAllowancesForSubject, addonsReadHomes = bindings, resolve, inForce, all, homes
	})
	const gb = int64(1 << 30)
	dbListTargetBindings = func(context.Context, string) ([]db.TargetBinding, error) {
		return []db.TargetBinding{{SubjectID: "u-charlotte", Username: "charlotte"}, {SubjectID: "u-booker", Username: "booker"}}, nil
	}
	svcResolveEntitlementSet = func(_ context.Context, subject, _ string) (services.EntitlementSet, error) {
		return services.EntitlementSet{
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
	addonsReadHomes = func(context.Context, string) addons.Homes {
		return addons.Homes{Read: true, UsageRead: true, Homes: []addons.HomeUsage{
			{Username: "charlotte", Home: "/mnt/p/main/home_dir/charlotte", UsedBytes: 18 * gb},
			{Username: "booker", Home: "/mnt/p/main/home_dir/booker", UsedBytes: 34 * gb},
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
	if booker.UsedBytes == nil || *booker.UsedBytes != 34*gb {
		t.Errorf("booker's use: %v", booker.UsedBytes)
	}
}

// "No limit" is the largest limit, as the add-on applies it: beside a 20 GB
// role it wins, and it never reads as a 0-byte limit the folder is "over".
func TestNoLimitIsTheLargestRolesLimit(t *testing.T) {
	set := services.EntitlementSet{Given: map[string][]services.RoleSource{
		"home_quota\x0021474836480": {{ProjectID: "p", RoleKey: "member"}},
		"home_quota\x000":           {{ProjectID: "p", RoleKey: "staff"}},
	}}
	limit, from := largestGiven(set, fieldHomeQuota)
	if limit != nil || from == nil || from.RoleKey != "staff" {
		t.Fatalf("want no limit, from staff; got %v from %+v", limit, from)
	}
}
