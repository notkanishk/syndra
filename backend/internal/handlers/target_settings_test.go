package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"syndra/internal/addons"
)

func putTargetSetting(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodPut, "/api/v1/targets/truenas/settings/home_root", strings.NewReader(body))
	r.SetPathValue("target", "truenas")
	r.SetPathValue("name", "home_root")
	rr := httptest.NewRecorder()
	handlePutTargetSetting(rr, r)
	return rr
}

// The add-on judges the value; Syndra relays its answer in its own words and
// says who changed it. A system that didn't answer changed nothing.
func TestATargetSettingIsRelayedAndJudgedByTheTarget(t *testing.T) {
	put := addonsPutSetting
	t.Cleanup(func() { addonsPutSetting = put })

	var sent [3]string
	answer := addons.CallResponse{Status: http.StatusOK, Outcome: addons.OutcomeSucceeded,
		Body: []byte(`{"name":"home_root","label":"Where home folders go","value":"/mnt/p/main/home_dir","values":[],"values_read":true}`)}
	reached := true
	addonsPutSetting = func(_ context.Context, target, name, value, _ string) (addons.CallResponse, bool) {
		sent = [3]string{target, name, value}
		return answer, reached
	}

	rr := putTargetSetting(t, `{"value":"/mnt/p/main/home_dir"}`)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"value":"/mnt/p/main/home_dir"`) {
		t.Fatalf("set: %d %s", rr.Code, rr.Body.String())
	}
	if sent != [3]string{"truenas", "home_root", "/mnt/p/main/home_dir"} {
		t.Fatalf("sent %v", sent)
	}

	answer = addons.CallResponse{Status: http.StatusUnprocessableEntity, Body: []byte(`{"error":"NOT_A_HOME_PLACE"}`)}
	if rr := putTargetSetting(t, `{"value":"/etc"}`); rr.Code != http.StatusBadRequest {
		t.Fatalf("refused value: %d %s", rr.Code, rr.Body.String())
	}

	reached = false
	if rr := putTargetSetting(t, `{"value":"/mnt/p/main/home_dir"}`); rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("unreached: %d", rr.Code)
	}

	if rr := putTargetSetting(t, `{"value":"x","extra":1}`); rr.Code != http.StatusBadRequest {
		t.Fatalf("strict decoding: %d", rr.Code)
	}
}

func TestTargetSettingsSayWhenTheyCouldntBeRead(t *testing.T) {
	read := addonsReadSettings
	t.Cleanup(func() { addonsReadSettings = read })
	addonsReadSettings = func(context.Context, string) ([]addons.TargetSetting, bool) { return nil, false }

	r := httptest.NewRequest(http.MethodGet, "/api/v1/targets/truenas/settings", nil)
	r.SetPathValue("target", "truenas")
	rr := httptest.NewRecorder()
	handleTargetSettings(rr, r)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"read":false`) {
		t.Fatalf("got %d %s", rr.Code, rr.Body.String())
	}
}
