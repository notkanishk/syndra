package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"syndra/internal/addons"
)

func stubValuesSchema(t *testing.T) {
	t.Helper()
	schema := addonsEntitlementSchema
	t.Cleanup(func() { addonsEntitlementSchema = schema })
	addonsEntitlementSchema = func(string) ([]addons.EntitlementField, error) {
		return []addons.EntitlementField{
			{Name: "group", Type: "string[]", Label: "Group on TrueNAS"},
			{Name: "home", Type: "bool", Label: "Gives a home folder", Single: true},
			{Name: "enabled", Type: "bool", Lifecycle: true},
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

// Fresh, then the target goes quiet: the last answer is served and says so.
// A target never read is said to be never read, not answered with nothing.
func TestMappableValuesAreFreshThenLastKnownThenNeverRead(t *testing.T) {
	stubValuesSchema(t)
	read := addonsReadValues
	t.Cleanup(func() { addonsReadValues = read })

	answering := true
	addonsReadValues = func(_ context.Context, _, field string) addons.Values {
		if !answering {
			return addons.Values{Outcome: addons.OutcomeUnreached}
		}
		return addons.Values{Outcome: addons.OutcomeSucceeded, Enumerable: true,
			Values: []string{"true"}, Labels: map[string]string{"true": "Gives a home folder"}}
	}

	rr, fresh := getMappableValues(t, "nas-a", "home")
	if rr.Code != http.StatusOK || !fresh.Current || fresh.ReadAt.IsZero() || !fresh.Single || fresh.Label != "Gives a home folder" {
		t.Fatalf("fresh: %d %+v", rr.Code, fresh)
	}
	if body := rr.Body.String(); !strings.Contains(body, `"values":["true"]`) || !strings.Contains(body, `"read_at":`) {
		t.Errorf("the answer is one flat object: %s", body)
	}
	if fresh.Labels["true"] != "Gives a home folder" {
		t.Errorf("labels didn't cross: %v", fresh.Labels)
	}

	answering = false
	rr, stale := getMappableValues(t, "nas-a", "home")
	if rr.Code != http.StatusOK || stale.Current || !stale.ReadAt.Equal(fresh.ReadAt) || len(stale.Values.Values) != 1 {
		t.Fatalf("stale: %d %+v", rr.Code, stale)
	}

	rr, _ = getMappableValues(t, "nas-b", "home")
	if rr.Code != http.StatusConflict || !strings.Contains(rr.Body.String(), "VALUES_NEVER_READ") {
		t.Fatalf("never read: %d %s", rr.Code, rr.Body.String())
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
