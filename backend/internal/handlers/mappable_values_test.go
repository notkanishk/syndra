package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"syndra/internal/addons"
)

func stubValuesSchema(t *testing.T) {
	t.Helper()
	schema := addonsEntitlementSchema
	t.Cleanup(func() { addonsEntitlementSchema = schema })
	addonsEntitlementSchema = func(string) ([]addons.EntitlementField, error) {
		return []addons.EntitlementField{
			{Name: "group", Role: "membership", Label: "Group on TrueNAS"},
			{Name: "home", Role: "flag", Label: "Gives a home folder"},
			{Name: "enabled", Role: "account"},
		}, nil
	}
}

func getMappableValues(t *testing.T, target, field string) (*httptest.ResponseRecorder, mappableValuesAnswer) {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/targets/"+target+"/mappable-values?field="+field, nil)
	r.SetPathValue("target", target)
	rr := httptest.NewRecorder()
	handleMappableValues(rr, r)
	var out mappableValuesAnswer
	_ = json.Unmarshal(rr.Body.Bytes(), &out)
	return rr, out
}

// Answered, then the target goes quiet: Syndra keeps no copy (design §13.3),
// so the second answer is 503 TARGET_NOT_READ, never the last list again.
func TestMappableValuesAreReadNowOrNotAtAll(t *testing.T) {
	stubValuesSchema(t)
	read := addonsReadValues
	t.Cleanup(func() { addonsReadValues = read })

	answering := true
	at := time.Date(2026, 11, 21, 9, 0, 0, 0, time.UTC)
	addonsReadValues = func(_ context.Context, _, field string) addons.Values {
		if !answering {
			return addons.Values{Read: addons.ReadOf(nil, addons.Why{Reason: addons.ReasonAddonUnreachable, Cause: addons.CauseNoAnswer})}
		}
		v := addons.Values{Read: addons.ReadOf(&addons.Observation{TakenAt: at, Current: true}, addons.Why{})}
		v.Enumerable, v.Values, v.Labels = true, []string{"true"}, map[string]string{"true": "Gives a home folder"}
		return v
	}

	rr, fresh := getMappableValues(t, "nas-a", "home")
	if rr.Code != http.StatusOK || fresh.Read.State != addons.ReadNow || !fresh.Read.TakenAt.Equal(at) || !fresh.Single || fresh.Label != "Gives a home folder" {
		t.Fatalf("fresh: %d %+v", rr.Code, fresh)
	}
	if body := rr.Body.String(); !strings.Contains(body, `"values":["true"]`) || !strings.Contains(body, `"read":{"state":"now"`) {
		t.Errorf("the answer is one flat object: %s", body)
	}
	if fresh.Labels["true"] != "Gives a home folder" {
		t.Errorf("labels didn't cross: %v", fresh.Labels)
	}

	answering = false
	rr, _ = getMappableValues(t, "nas-a", "home")
	if rr.Code != http.StatusServiceUnavailable || !strings.Contains(rr.Body.String(), codeTargetNotRead) || strings.Contains(rr.Body.String(), `"values"`) {
		t.Fatalf("quiet: want 503 TARGET_NOT_READ and no values, got %d %s", rr.Code, rr.Body.String())
	}
}

// Only what a mapping can set has values to pick: a lifecycle field is the
// resolver's own, and a field the system doesn't have is nothing.
func TestOnlyMappableFieldsHaveValues(t *testing.T) {
	stubValuesSchema(t)
	for _, field := range []string{"enabled", "colour"} {
		if rr, _ := getMappableValues(t, "nas-a", field); rr.Code != http.StatusNotFound {
			t.Errorf("%s: want 404, got %d", field, rr.Code)
		}
	}
}
