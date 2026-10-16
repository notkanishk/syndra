package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"syndra/internal/services"
)

func stubTerms(t *testing.T) (saved *string, audits *[]string) {
	t.Helper()
	oLoad, oSet, oAudit := svcLoadTerms, dbSetConfigSetting, dbInsertAuditLog
	t.Cleanup(func() { svcLoadTerms, dbSetConfigSetting, dbInsertAuditLog = oLoad, oSet, oAudit })
	var s string
	var a []string
	svcLoadTerms = func(context.Context, time.Time) (services.TermsView, error) {
		return services.TermsView{Terms: services.DefaultTerms(2026)}, nil
	}
	dbSetConfigSetting = func(_ context.Context, key, value, _ string) error {
		if key != services.ConfigKeyTerms {
			t.Fatalf("key %q", key)
		}
		s = value
		return nil
	}
	dbInsertAuditLog = func(_ context.Context, _, _, action, _ string) error { a = append(a, action); return nil }
	return &s, &a
}

func TestGetTerms_DefaultsHaveNoSetter(t *testing.T) {
	stubTerms(t)
	rr := httptest.NewRecorder()
	handleGetTerms(rr, httptest.NewRequest(http.MethodGet, "/api/v1/terms", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("%d %s", rr.Code, rr.Body)
	}
	var got map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &got)
	if v, has := got["set_by"]; !has || v != nil {
		t.Fatalf("set_by must be present and null: %v", got)
	}
	if v, has := got["set_at"]; !has || v != nil {
		t.Fatalf("set_at must be present and null: %v", got)
	}
	if terms := got["terms"].([]any); len(terms) != 3 || terms[0].(map[string]any)["name"] != "Spring" {
		t.Fatalf("terms: %v", got["terms"])
	}
}

func TestPutTerms_SavesSortedAndAudits(t *testing.T) {
	saved, audits := stubTerms(t)
	body := `{"terms":[{"name":" Spring ","start":"2027-01-10","end":"2027-05-20"},{"name":"Monsoon","start":"2026-08-01","end":"2026-12-15"}]}`
	rr := httptest.NewRecorder()
	handlePutTerms(rr, httptest.NewRequest(http.MethodPut, "/api/v1/terms", strings.NewReader(body)))
	if rr.Code != http.StatusOK {
		t.Fatalf("%d %s", rr.Code, rr.Body)
	}
	want := `[{"name":"Monsoon","start":"2026-08-01","end":"2026-12-15"},{"name":"Spring","start":"2027-01-10","end":"2027-05-20"}]`
	if *saved != want {
		t.Fatalf("saved %s", *saved)
	}
	if len(*audits) != 1 || (*audits)[0] != "terms.updated" {
		t.Fatalf("audits %v", *audits)
	}
	var got services.TermsView
	_ = json.Unmarshal(rr.Body.Bytes(), &got)
	if got.Terms[0].Name != "Monsoon" || got.SetBy == nil || got.SetAt == nil {
		t.Fatalf("response %+v", got)
	}
}

func TestPutTerms_RefusesBadInputAndSavesNothing(t *testing.T) {
	saved, audits := stubTerms(t)
	for name, body := range map[string]string{
		"unknown field": `{"terms":[{"name":"A","start":"2026-01-01","end":"2026-02-01"}],"x":1}`,
		"unknown inner": `{"terms":[{"name":"A","start":"2026-01-01","end":"2026-02-01","z":1}]}`,
		"empty":         `{"terms":[]}`,
		"bad date":      `{"terms":[{"name":"A","start":"2026-13-01","end":"2026-02-01"}]}`,
		"overlap":       `{"terms":[{"name":"A","start":"2026-01-01","end":"2026-03-01"},{"name":"B","start":"2026-02-01","end":"2026-04-01"}]}`,
	} {
		rr := httptest.NewRecorder()
		handlePutTerms(rr, httptest.NewRequest(http.MethodPut, "/api/v1/terms", strings.NewReader(body)))
		if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "VALIDATION_FAILED") {
			t.Errorf("%s: %d %s", name, rr.Code, rr.Body)
		}
	}
	if *saved != "" || len(*audits) != 0 {
		t.Fatalf("a refused write must leave no trace: %q %v", *saved, *audits)
	}
	// The field map names the field.
	rr := httptest.NewRecorder()
	handlePutTerms(rr, httptest.NewRequest(http.MethodPut, "/api/v1/terms",
		strings.NewReader(`{"terms":[{"name":"","start":"2026-01-01","end":"2026-02-01"}]}`)))
	if !strings.Contains(rr.Body.String(), "terms[0].name") {
		t.Fatalf("%s", rr.Body)
	}
}
