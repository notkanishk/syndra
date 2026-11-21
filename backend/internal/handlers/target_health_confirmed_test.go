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
	health, healthOf, anchor := addonsHealth, addonsHealthOf, dbGetLogAnchor
	t.Cleanup(func() { addonsHealth, addonsHealthOf, dbGetLogAnchor = health, healthOf, anchor })
	dbGetLogAnchor = func(context.Context, string) (db.LogAnchor, bool, error) { return db.LogAnchor{}, false, nil }
	since := time.Date(2026, 10, 14, 9, 40, 0, 0, time.UTC)

	creds := addonsTransportCredentials
	t.Cleanup(func() { addonsTransportCredentials = creds })
	addonsTransportCredentials = func() []addons.TransportCredential { return nil }

	for _, tc := range []struct {
		name   string
		live   addons.TargetHealth
		state  addons.HealthState
		status string
	}{
		{"answering", addons.TargetHealth{Reachable: true, Outcome: addons.OutcomeSucceeded},
			addons.HealthState{State: addons.StateAnswering, CheckedAt: since}, "answering"},
		{"down", addons.TargetHealth{Outcome: addons.OutcomeUnreached, Err: errors.New("no route")},
			addons.HealthState{State: addons.StateAddonUnreachable, Cause: addons.CauseNoAnswer, CheckedAt: since.Add(time.Minute), Since: &since}, "addon_unreachable"},
	} {
		addonsHealth = func(context.Context, string) addons.TargetHealth { return tc.live }
		addonsHealthOf = func(string) addons.HealthState { return tc.state }

		r := httptest.NewRequest(http.MethodGet, "/api/v1/targets/truenas/health", nil)
		r.SetPathValue("target", "truenas")
		rr := httptest.NewRecorder()
		handleTargetHealth(rr, r)

		var body struct {
			Health addons.HealthState `json:"health"`
			Status string             `json:"status"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if body.Health.State != tc.state.State || body.Health.Cause != tc.state.Cause {
			t.Errorf("%s: want health %s, got %s (%s)", tc.name, tc.state.State, body.Health.State, rr.Body.String())
		}
		if body.Status != tc.status {
			t.Errorf("%s: the system page reads the same ranked status as the roster, want %s, got %q", tc.name, tc.status, body.Status)
		}
		if tc.state.Since != nil && (body.Health.Since == nil || !body.Health.Since.Equal(since)) {
			t.Errorf("%s: an outage carries when it began: %s", tc.name, rr.Body.String())
		}
	}
}

// The roster's status is the monitor's state and nothing ranks over it
// (design §13.2): a key that does not load is the monitor's wrong_host, an
// open breaker is the cause that opened it.
func TestTheRosterStatusIsTheMonitorsState(t *testing.T) {
	for _, st := range []addons.ReachState{addons.StateAnswering, addons.StateAddonUnreachable, addons.StateAppUnreachable, addons.StateChecking} {
		s := targetSummary{TransportStatus: "error", CircuitOpen: true, Health: addons.HealthState{State: st}}
		if got := targetStatus(s); got != string(st) {
			t.Errorf("want %s, got %s", st, got)
		}
	}
}
