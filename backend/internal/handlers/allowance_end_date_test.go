package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"syndra/internal/addons"
	"syndra/internal/clock"
	"syndra/internal/db"
)

// The wire carries the date a person means. A hold that ends "2026-11-30" is
// stored as the last second of 30 Nov in the deployment's zone (the column stays
// timestamptz), whatever zone the client believed in; an instant a client worked
// out for itself is refused, never read as a day.
func TestAHoldEndsOnTheDateSentInTheDeploymentZone(t *testing.T) {
	stubMappingDeps(t, []addons.EntitlementField{{Name: "group", Role: "membership"}})
	create := dbCreateAllowance
	t.Cleanup(func() { dbCreateAllowance = create })
	var seen db.Allowance
	dbCreateAllowance = func(_ context.Context, a db.Allowance) (db.Allowance, error) { seen = a; a.ID = "a1"; return a, nil }
	old := clock.Loc
	t.Cleanup(func() { clock.Loc = old })

	post := func(end string) *httptest.ResponseRecorder {
		rr := httptest.NewRecorder()
		handleCreateAllowance(rr, httptest.NewRequest(http.MethodPost, "/api/v1/allowances", strings.NewReader(
			`{"subject_id":"u1","target":"truenas","field":"group","value":"lab_makers","direction":"deny","reason":"r",`+end+`}`)))
		return rr
	}
	for zone, want := range map[string]string{
		"Asia/Kolkata":     "2026-11-30T23:59:59+05:30",
		"America/Santiago": "2026-11-30T23:59:59-03:00",
	} {
		_, clock.Loc, _ = clockLoad(zone)
		seen = db.Allowance{}
		if rr := post(`"expires_at":"2026-11-30"`); rr.Code != http.StatusCreated {
			t.Fatalf("%s: %d %s", zone, rr.Code, rr.Body.String())
		}
		if seen.ExpiresAt == nil || seen.ExpiresAt.Format(time.RFC3339) != want {
			t.Errorf("%s: stored %v, want %s", zone, seen.ExpiresAt, want)
		}
	}
	seen = db.Allowance{}
	if rr := post(`"expires_at":"2026-11-30T23:59:59Z"`); rr.Code == http.StatusCreated || seen.ExpiresAt != nil {
		t.Errorf("an instant must be refused, not read as a day: %d %s", rr.Code, rr.Body.String())
	}
}

func clockLoad(zone string) (string, *time.Location, error) {
	loc, err := time.LoadLocation(zone)
	return zone, loc, err
}
