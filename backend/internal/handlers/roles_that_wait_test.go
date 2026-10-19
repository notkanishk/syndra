package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"syndra/internal/db"
)

func stubRolesThatWait(t *testing.T, model string) (marked *[]string, audits *[]string) {
	t.Helper()
	oList, oMark, oUnmark, oSugg, oMeta, oSet, oAudit :=
		dbListRoleWaits, dbMarkRoleWaits, dbUnmarkRoleWaits, dbListWaitSuggestions, dbGetConfigSettingMeta, dbSetConfigSetting, dbInsertAuditLog
	t.Cleanup(func() {
		dbListRoleWaits, dbMarkRoleWaits, dbUnmarkRoleWaits, dbListWaitSuggestions, dbGetConfigSettingMeta, dbSetConfigSetting, dbInsertAuditLog =
			oList, oMark, oUnmark, oSugg, oMeta, oSet, oAudit
	})
	var m, a []string
	dbListRoleWaits = func(context.Context) ([]db.RoleWait, error) { return nil, nil }
	dbMarkRoleWaits = func(_ context.Context, p, k, _ string) (bool, error) {
		m = append(m, p+":"+k)
		return true, nil
	}
	dbUnmarkRoleWaits = func(context.Context, string, string) (bool, error) { return true, nil }
	dbListWaitSuggestions = func(context.Context) ([]db.WaitSuggestion, error) {
		return []db.WaitSuggestion{{ProjectID: "p1", RoleKey: "operator",
			Via: []db.WaitSource{{Kind: "bundle", ID: "b1", Name: "Lab Tech"}}}}, nil
	}
	dbGetConfigSettingMeta = func(context.Context, string) (db.ConfigSetting, bool, error) {
		if model == "" {
			return db.ConfigSetting{}, false, nil
		}
		return db.ConfigSetting{Value: model, UpdatedBy: "op", UpdatedAt: time.Now()}, true, nil
	}
	dbSetConfigSetting = func(context.Context, string, string, string) error { return nil }
	dbInsertAuditLog = func(_ context.Context, _, _, action, _ string) error {
		a = append(a, action)
		return nil
	}
	return &m, &a
}

// Before the switch over the page offers the one-time move; after it, not.
func TestRolesThatWait_SuggestsTheMoveOnlyBeforeTheSwitch(t *testing.T) {
	for _, tc := range []struct {
		model    string
		switched bool
		suggest  int
	}{{"", false, 1}, {db.ConfirmationModelRoles, true, 0}} {
		stubRolesThatWait(t, tc.model)
		rr := httptest.NewRecorder()
		handleListRolesThatWait(rr, httptest.NewRequest(http.MethodGet, "/api/v1/roles-that-wait", nil))
		var got rolesThatWaitResponse
		if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got.SwitchedOver != tc.switched || len(got.Suggestions) != tc.suggest || got.Roles == nil {
			t.Fatalf("model %q: %+v", tc.model, got)
		}
	}
}

// Marking is a setting: recorded in the audit log, and refused for a role
// Zitadel doesn't offer.
func TestMarkRoleWaits_RecordsAndRefusesUnofferedRoles(t *testing.T) {
	marked, audits := stubRolesThatWait(t, "")

	req := httptest.NewRequest(http.MethodPut, "/api/v1/roles-that-wait/p1/laser.operator", nil)
	req.SetPathValue("project_id", "p1")
	req.SetPathValue("role_key", "laser.operator")
	rr := httptest.NewRecorder()
	handleMarkRoleWaits(rr, req)
	if rr.Code != http.StatusOK || len(*marked) != 1 || (*audits)[0] != "role.waits_marked" {
		t.Fatalf("mark: code=%d marked=%v audits=%v", rr.Code, *marked, *audits)
	}

	orig := checkRoleOffered
	t.Cleanup(func() { checkRoleOffered = orig })
	checkRoleOffered = func(context.Context, string, string) error { return errRoleNotOffered }
	rr = httptest.NewRecorder()
	handleMarkRoleWaits(rr, req)
	if rr.Code == http.StatusOK || len(*marked) != 1 {
		t.Fatalf("a role Zitadel doesn't offer must not be marked: %d %v", rr.Code, *marked)
	}

	bad := httptest.NewRequest(http.MethodPut, "/api/v1/roles-that-wait/p1/x", nil)
	bad.SetPathValue("project_id", "p1")
	bad.SetPathValue("role_key", "a b")
	rr = httptest.NewRecorder()
	handleMarkRoleWaits(rr, bad)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("malformed role key: want 400, got %d", rr.Code)
	}
}

func TestSwitchToRolesThatWait_OnlyToRoles(t *testing.T) {
	_, audits := stubRolesThatWait(t, "")
	rr := httptest.NewRecorder()
	handleSwitchToRolesThatWait(rr, httptest.NewRequest(http.MethodPut, "/api/v1/config/confirmation-model", strings.NewReader(`{"model":"bundles"}`)))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("only the switch to roles exists: got %d", rr.Code)
	}
	rr = httptest.NewRecorder()
	handleSwitchToRolesThatWait(rr, httptest.NewRequest(http.MethodPut, "/api/v1/config/confirmation-model", strings.NewReader(`{"model":"roles"}`)))
	if rr.Code != http.StatusOK || len(*audits) != 1 || (*audits)[0] != "confirmation.switched_to_roles" {
		t.Fatalf("switch: %d %v", rr.Code, *audits)
	}
}
