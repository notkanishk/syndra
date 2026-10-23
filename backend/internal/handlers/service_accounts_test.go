package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"syndra/internal/models"
	"syndra/internal/services"
)

type saStub struct {
	saved   []services.ServiceAccountInput
	savedID string
	actor   string
	audits  []string
	unmark  []string
}

func stubServiceAccounts(t *testing.T) *saStub {
	t.Helper()
	oList, oGet, oSave, oUnmark, oAudit := svcListServiceAccounts, svcGetServiceAccount, svcSaveServiceAccount, svcUnmarkServiceAccount, dbInsertAuditLog
	t.Cleanup(func() {
		svcListServiceAccounts, svcGetServiceAccount, svcSaveServiceAccount, svcUnmarkServiceAccount, dbInsertAuditLog = oList, oGet, oSave, oUnmark, oAudit
	})
	s := &saStub{}
	netbird := models.ServiceAccount{ID: "u-netbird", Name: "netbird", Kind: "integration", ZitadelType: "machine", RoleCount: 2, RoleCountObserved: true}
	svcListServiceAccounts = func(context.Context) ([]models.ServiceAccount, error) {
		return []models.ServiceAccount{netbird}, nil
	}
	svcGetServiceAccount = func(_ context.Context, id string) (models.ServiceAccount, error) {
		if id != "u-netbird" {
			return models.ServiceAccount{}, services.ErrServiceAccountNotFound
		}
		return netbird, nil
	}
	svcSaveServiceAccount = func(_ context.Context, id string, in services.ServiceAccountInput, actor string) (models.ServiceAccount, error) {
		s.saved, s.savedID, s.actor = append(s.saved, in), id, actor
		return models.ServiceAccount{ID: id, Name: "Kiosk", Kind: in.Kind, ZitadelType: "human", Purpose: in.Purpose, OwnerID: in.OwnerID}, nil
	}
	svcUnmarkServiceAccount = func(_ context.Context, id string) error {
		s.unmark = append(s.unmark, id)
		switch id {
		case "u-machine":
			return services.ErrMachineAccount
		case "u-kiosk":
			return nil
		}
		return services.ErrServiceAccountNotFound
	}
	dbInsertAuditLog = func(_ context.Context, _, target, action, _ string) error {
		s.audits = append(s.audits, action+":"+target)
		return nil
	}
	return s
}

func saRequest(h http.HandlerFunc, method, id, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/api/v1/service-accounts/"+id, strings.NewReader(body))
	req.SetPathValue("id", id)
	rr := httptest.NewRecorder()
	h(rr, req)
	return rr
}

func errCode(t *testing.T, rr *httptest.ResponseRecorder) string {
	t.Helper()
	var e ErrorResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &e)
	return e.Error
}

func TestServiceAccounts_ListAndGetShape(t *testing.T) {
	stubServiceAccounts(t)
	rr := httptest.NewRecorder()
	handleListServiceAccounts(rr, httptest.NewRequest(http.MethodGet, "/api/v1/service-accounts", nil))
	var got struct {
		Accounts []map[string]any `json:"accounts"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil || rr.Code != 200 || len(got.Accounts) != 1 {
		t.Fatalf("%d %s", rr.Code, rr.Body)
	}
	for _, f := range []string{"id", "name", "kind", "zitadel_type", "purpose", "owner_id", "role_count", "marked_by", "marked_at"} {
		if _, ok := got.Accounts[0][f]; !ok {
			t.Errorf("field %q missing from %v", f, got.Accounts[0])
		}
	}
	if got.Accounts[0]["marked_at"] != nil {
		t.Errorf("a machine with no mark has a null marked_at: %v", got.Accounts[0])
	}

	if rr := saRequest(handleGetServiceAccount, http.MethodGet, "u-netbird", ""); rr.Code != 200 {
		t.Errorf("get one: %d %s", rr.Code, rr.Body)
	}
	if rr := saRequest(handleGetServiceAccount, http.MethodGet, "u-person", ""); rr.Code != 404 || errCode(t, rr) != "NOT_FOUND" {
		t.Errorf("a person is not a service account: %d %s", rr.Code, rr.Body)
	}
}

func TestServiceAccounts_PutMarksAndAudits(t *testing.T) {
	s := stubServiceAccounts(t)
	rr := saRequest(handlePutServiceAccount, http.MethodPut, "u-kiosk", `{"kind":"device","purpose":"Front panel","owner_id":"u-owner"}`)
	if rr.Code != 200 {
		t.Fatalf("%d %s", rr.Code, rr.Body)
	}
	if len(s.saved) != 1 || s.savedID != "u-kiosk" || s.saved[0] != (services.ServiceAccountInput{Kind: "device", Purpose: "Front panel", OwnerID: "u-owner"}) {
		t.Fatalf("saved: %+v for %s", s.saved, s.savedID)
	}
	if len(s.audits) != 1 || s.audits[0] != "service_account.saved:u-kiosk" {
		t.Fatalf("audits: %v", s.audits)
	}
	var acc models.ServiceAccount
	_ = json.Unmarshal(rr.Body.Bytes(), &acc)
	if acc.ID != "u-kiosk" || acc.Kind != "device" {
		t.Fatalf("returns the account: %+v", acc)
	}
}

func TestServiceAccounts_PutRefusals(t *testing.T) {
	s := stubServiceAccounts(t)
	// The unknown field is refused before anything is read or written.
	if rr := saRequest(handlePutServiceAccount, http.MethodPut, "u-kiosk", `{"kind":"device","purpose":"","owner_id":"","zitadel_type":"machine"}`); rr.Code != 400 || errCode(t, rr) != "VALIDATION_FAILED" {
		t.Errorf("strict decode: %d %s", rr.Code, rr.Body)
	}
	svcSaveServiceAccount = func(context.Context, string, services.ServiceAccountInput, string) (models.ServiceAccount, error) {
		return models.ServiceAccount{}, &services.ServiceAccountInvalid{Field: "kind", Message: "Choose what it is."}
	}
	rr := saRequest(handlePutServiceAccount, http.MethodPut, "u-kiosk", `{"kind":"robot","purpose":"","owner_id":""}`)
	if rr.Code != 400 || errCode(t, rr) != "VALIDATION_FAILED" || !strings.Contains(rr.Body.String(), `"kind"`) {
		t.Errorf("bad kind: %d %s", rr.Code, rr.Body)
	}
	svcSaveServiceAccount = func(context.Context, string, services.ServiceAccountInput, string) (models.ServiceAccount, error) {
		return models.ServiceAccount{}, &services.ServiceAccountInvalid{Field: "owner_id", Message: "Choose a person."}
	}
	rr = saRequest(handlePutServiceAccount, http.MethodPut, "u-kiosk", `{"kind":"device","purpose":"","owner_id":"u-other-sa"}`)
	if rr.Code != 400 || !strings.Contains(rr.Body.String(), `"owner_id"`) {
		t.Errorf("owner that is a service account: %d %s", rr.Code, rr.Body)
	}
	svcSaveServiceAccount = func(context.Context, string, services.ServiceAccountInput, string) (models.ServiceAccount, error) {
		return models.ServiceAccount{}, services.ErrServiceAccountNotFound
	}
	if rr := saRequest(handlePutServiceAccount, http.MethodPut, "u-nobody", `{"kind":"device","purpose":"","owner_id":""}`); rr.Code != 404 {
		t.Errorf("unknown id: %d", rr.Code)
	}
	if len(s.audits) != 0 {
		t.Errorf("a refused request is not audited: %v", s.audits)
	}
}

func TestServiceAccounts_DeleteMarksAsPerson(t *testing.T) {
	s := stubServiceAccounts(t)
	if rr := saRequest(handleDeleteServiceAccount, http.MethodDelete, "u-kiosk", ""); rr.Code != 204 {
		t.Fatalf("unmark: %d %s", rr.Code, rr.Body)
	}
	if len(s.audits) != 1 || s.audits[0] != "service_account.unmarked:u-kiosk" {
		t.Fatalf("audits: %v", s.audits)
	}
	if rr := saRequest(handleDeleteServiceAccount, http.MethodDelete, "u-machine", ""); rr.Code != 409 || errCode(t, rr) != "MACHINE_ACCOUNT" {
		t.Errorf("a machine cannot be marked as a person: %d %s", rr.Code, rr.Body)
	}
	if rr := saRequest(handleDeleteServiceAccount, http.MethodDelete, "u-plain", ""); rr.Code != 404 {
		t.Errorf("an unmarked person: %d", rr.Code)
	}
	if len(s.audits) != 1 {
		t.Errorf("only the real unmark is audited: %v", s.audits)
	}
}
