package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"syndra/internal/addons"
	"syndra/internal/db"
	"syndra/internal/services"
	"syndra/internal/services/drift"
)

// "Read again" is the card's own GET with ?fresh=true, and nothing else is.
func TestWantsFreshOnlyOnTheLiteralTrue(t *testing.T) {
	for query, want := range map[string]bool{"": false, "?fresh=true": true, "?fresh=1": false, "?fresh=TRUE": false, "?fresh=": false, "?fresh=false": false} {
		if got := wantsFresh(httptest.NewRequest(http.MethodGet, "/x"+query, nil)); got != want {
			t.Errorf("%q: wantsFresh %t, want %t", query, got, want)
		}
	}
}

func TestTheInventoryPassesReadAgainThroughAndCountsDown(t *testing.T) {
	orig := svcTargetInventory
	t.Cleanup(func() { svcTargetInventory = orig })
	var asked []bool
	svcTargetInventory = func(_ context.Context, _ string, fresh bool) (drift.AddonReconcileResult, error) {
		asked = append(asked, fresh)
		if fresh {
			return drift.AddonReconcileResult{Halted: true, RetryAfter: 9}, nil
		}
		return drift.AddonReconcileResult{}, nil
	}
	get := func(target string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodGet, target, nil)
		r.SetPathValue("target", "truenas")
		rr := httptest.NewRecorder()
		handleTargetInventory(rr, r)
		return rr
	}
	if rr := get("/api/v1/targets/truenas/inventory"); rr.Code != http.StatusOK {
		t.Errorf("a page load: %d", rr.Code)
	}
	rr := get("/api/v1/targets/truenas/inventory?fresh=true")
	var body ErrorResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if rr.Code != http.StatusTooManyRequests || rr.Header().Get("Retry-After") != "9" || body.RetryAfter != 9 || body.Error != "FRESH_READ_COOLDOWN" {
		t.Errorf("read again inside the cooldown: %d Retry-After %q %s, want 429 FRESH_READ_COOLDOWN, retry_after 9", rr.Code, rr.Header().Get("Retry-After"), rr.Body)
	}
	if len(asked) != 2 || asked[0] || !asked[1] {
		t.Errorf("asked fresh %v, want [false true]", asked)
	}
}

// Owner Q1: a removal inside the cooldown is refused with the countdown, and
// nothing is removed.
func TestADormantRemovalInsideTheCooldownCountsDown(t *testing.T) {
	h := &sweepHarness{reportErr: services.ReadCooledDown{RetryAfter: 12}}
	stubSweep(t, h)
	rr := sweep(t, `{"accounts":["gone"],"elevated_key":"k","confirmed":true}`)
	var body ErrorResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if rr.Code != http.StatusTooManyRequests || rr.Header().Get("Retry-After") != "12" || body.RetryAfter != 12 || body.Error != "FRESH_READ_COOLDOWN" {
		t.Errorf("want 429, Retry-After 12, retry_after 12: %d %q %s", rr.Code, rr.Header().Get("Retry-After"), rr.Body)
	}
	if len(h.purged) != 0 {
		t.Error("something was removed on the strength of a copy")
	}

	r := httptest.NewRequest(http.MethodGet, "/api/v1/targets/truenas/accounts/dormant", nil)
	r.SetPathValue("target", "truenas")
	list := httptest.NewRecorder()
	handleDormantAccounts(list, r)
	if list.Code != http.StatusTooManyRequests || list.Header().Get("Retry-After") != "12" {
		t.Errorf("the list inside the cooldown: %d %q", list.Code, list.Header().Get("Retry-After"))
	}
}

// The member's and the person's pages pass "Read again" to every read they
// make, and a page load passes none.
func TestTheStorageCardsPassReadAgainThrough(t *testing.T) {
	origReach, origHomes, origAct := addonsReadReach, addonsReadHomes, addonsActivity
	t.Cleanup(func() { addonsReadReach, addonsReadHomes, addonsActivity = origReach, origHomes, origAct })
	var asked []bool
	addonsReadReach = func(_ context.Context, _ string, fresh bool) addons.Reach {
		asked = append(asked, fresh)
		return addons.Reach{RetryAfter: 4}
	}
	addonsActivity = func(_ context.Context, _, _, _, _ string, fresh bool) addons.ActivityReport {
		asked = append(asked, fresh)
		return addons.ActivityReport{RetryAfter: 4}
	}
	for _, q := range []string{"", "&fresh=true"} {
		r := httptest.NewRequest(http.MethodGet, "/api/v1/targets/truenas/reach?x=1"+q, nil)
		r.SetPathValue("target", "truenas")
		rr := httptest.NewRecorder()
		handleTargetReach(rr, r)
		var reach addons.Reach
		_ = json.Unmarshal(rr.Body.Bytes(), &reach)
		if reach.RetryAfter != 4 {
			t.Errorf("the reach countdown did not reach the page: %s", rr.Body)
		}

		r = httptest.NewRequest(http.MethodGet, "/api/v1/targets/truenas/activity?subject=s"+q, nil)
		r.SetPathValue("target", "truenas")
		rr = httptest.NewRecorder()
		handleTargetActivity(rr, r)
		var act map[string]any
		_ = json.Unmarshal(rr.Body.Bytes(), &act)
		if act["reason"] != "cooldown" || act["retry_after"] != float64(4) || act["readable"] != false {
			t.Errorf("the activity countdown did not reach the page: %s", rr.Body)
		}
	}
	if want := []bool{false, false, true, true}; len(asked) != 4 || asked[0] != want[0] || asked[1] != want[1] || asked[2] != want[2] || asked[3] != want[3] {
		t.Errorf("asked fresh %v, want %v", asked, want)
	}
}

func TestTheMembersReadsPassReadAgainThrough(t *testing.T) {
	stubMyStorage(t, &storageHarness{entitled: true, bound: true})
	read, reads := addonsRead, addonsMemberReads
	t.Cleanup(func() { addonsRead, addonsMemberReads = read, reads })
	addonsMemberReads = func(string) []string { return []string{"storage.status"} }
	var asked []bool
	addonsRead = func(_ context.Context, _, _, _, _ string, _ map[string]any, fresh bool) addons.ReadReply {
		asked = append(asked, fresh)
		return storageReply(addons.StorageStatus{Outcome: addons.OutcomeSucceeded})
	}
	for _, q := range []string{"", "?fresh=true"} {
		if _, _, err := describeTargetFor(httptest.NewRequest(http.MethodGet, "/api/v1/me/targets"+q, nil), "truenas", "u1"); err != nil {
			t.Fatal(err)
		}
	}
	if len(asked) != 2 || asked[0] || !asked[1] {
		t.Errorf("the member's reads asked fresh %v, want [false true]", asked)
	}
}

func TestTheHomesSayHowFreshTheirReadIs(t *testing.T) {
	bindings, homes := dbListTargetBindings, addonsReadHomes
	t.Cleanup(func() { dbListTargetBindings, addonsReadHomes = bindings, homes })
	dbListTargetBindings = func(context.Context, string) ([]db.TargetBinding, error) { return nil, nil }
	retry := 20
	at := time.Date(2026, 11, 10, 9, 30, 0, 0, time.UTC)
	var asked []bool
	addonsReadHomes = func(_ context.Context, _ string, fresh bool) addons.Homes {
		asked = append(asked, fresh)
		if fresh {
			return addons.Homes{RetryAfter: 6}
		}
		return addons.Homes{Read: true, Observation: &addons.Observation{TakenAt: at, Stale: &addons.Stale{Reason: "cached", RetryAfter: &retry}}}
	}
	for _, q := range []string{"", "?fresh=true"} {
		r := httptest.NewRequest(http.MethodGet, "/api/v1/targets/truenas/homes"+q, nil)
		r.SetPathValue("target", "truenas")
		rr := httptest.NewRecorder()
		handleTargetHomes(rr, r)
		var body map[string]any
		_ = json.Unmarshal(rr.Body.Bytes(), &body)
		if q == "" && (body["taken_at"] != "2026-11-10T09:30:00Z" || body["current"] != false || body["stale"].(map[string]any)["reason"] != "cached") {
			t.Errorf("a copy is dated and labelled: %s", rr.Body)
		}
		if q != "" && (body["retry_after"] != float64(6) || body["read"] != false) {
			t.Errorf("a cooldown with nothing to serve counts down: %s", rr.Body)
		}
	}
	if len(asked) != 2 || asked[0] || !asked[1] {
		t.Errorf("asked fresh %v", asked)
	}
}

// "Check now" runs the pass inside the person's request, so it is the pass
// that never waits out a cooldown (drift.ReconcileAddonNow), not the
// scheduled sweep's, which may wait up to a minute.
func TestCheckNowIsThePassThatNeverWaits(t *testing.T) {
	if reflect.ValueOf(svcReconcileAddon).Pointer() != reflect.ValueOf(drift.ReconcileAddonNow).Pointer() {
		t.Error("handleReconcileTarget runs a pass that may wait out a cooldown inside the request")
	}
}
