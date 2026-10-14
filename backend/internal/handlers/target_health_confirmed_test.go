package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"syndra/internal/addons"
	"syndra/internal/db"
)

// The operator's health read carries the confirmed answer beside the live one,
// on both paths: one failed read is not an outage, and the page must be able to
// say which it is looking at.
func TestTargetHealthCarriesTheConfirmedState(t *testing.T) {
	health, healthOf, anchor, conflicts := addonsHealth, addonsHealthOf, dbGetLogAnchor, dbStandingBindingConflicts
	t.Cleanup(func() {
		addonsHealth, addonsHealthOf, dbGetLogAnchor, dbStandingBindingConflicts = health, healthOf, anchor, conflicts
	})
	dbStandingBindingConflicts = func(context.Context, string) ([]db.BindingConflict, error) { return nil, nil }
	dbGetLogAnchor = func(context.Context, string) (db.LogAnchor, bool, error) { return db.LogAnchor{}, false, nil }
	since := time.Date(2026, 10, 14, 9, 40, 0, 0, time.UTC)

	for _, tc := range []struct {
		name  string
		live  addons.TargetHealth
		state addons.HealthState
	}{
		{"answering", addons.TargetHealth{Reachable: true, Outcome: addons.OutcomeSucceeded},
			addons.HealthState{Status: addons.HealthOnline, CheckedAt: since}},
		{"down", addons.TargetHealth{Outcome: addons.OutcomeUnreached, Err: errors.New("no route")},
			addons.HealthState{Status: addons.HealthOffline, CheckedAt: since.Add(time.Minute), ReachableSince: &since}},
	} {
		addonsHealth = func(context.Context, string) addons.TargetHealth { return tc.live }
		addonsHealthOf = func(string) addons.HealthState { return tc.state }

		r := httptest.NewRequest(http.MethodGet, "/api/v1/targets/truenas/health", nil)
		r.SetPathValue("target", "truenas")
		rr := httptest.NewRecorder()
		handleTargetHealth(rr, r)

		var body struct {
			Health addons.HealthState `json:"health"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if body.Health.Status != tc.state.Status {
			t.Errorf("%s: want health %s, got %s (%s)", tc.name, tc.state.Status, body.Health.Status, rr.Body.String())
		}
		if tc.state.ReachableSince != nil && (body.Health.ReachableSince == nil || !body.Health.ReachableSince.Equal(since)) {
			t.Errorf("%s: an outage carries when it began: %s", tc.name, rr.Body.String())
		}
	}
}
