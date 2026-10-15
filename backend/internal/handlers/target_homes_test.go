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
		return []db.TargetBinding{{SubjectID: "u-ada", Username: "ada"}, {SubjectID: "u-bo", Username: "bo"}}, nil
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
		if subject != "u-ada" {
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
			{Username: "ada", Home: "/mnt/p/main/home_dir/ada", UsedBytes: 18 * gb},
			{Username: "bo", Home: "/mnt/p/main/home_dir/bo", UsedBytes: 34 * gb},
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
	ada, bo := body.Accounts[0], body.Accounts[1]
	if !ada.GivesHome || ada.RolesLimit == nil || *ada.RolesLimit != 20*gb || ada.RolesLimitFrom.RoleKey != "fab_lead" {
		t.Errorf("ada's roles' limit: %+v from %+v", ada.RolesLimit, ada.RolesLimitFrom)
	}
	if ada.Own == nil || ada.Own.Bytes != 50*gb || !ada.Own.ReviewDue || ada.OwnEnded != nil {
		t.Errorf("ada's own limit: %+v ended %+v", ada.Own, ada.OwnEnded)
	}
	if bo.Own != nil || bo.OwnEnded == nil || bo.OwnEnded.Bytes != 50*gb {
		t.Errorf("bo's own limit ended at 50 GB with 34 GB in a 20 GB home: %+v", bo.OwnEnded)
	}
	if bo.UsedBytes == nil || *bo.UsedBytes != 34*gb {
		t.Errorf("bo's use: %v", bo.UsedBytes)
	}
}
