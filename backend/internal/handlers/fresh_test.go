package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
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
			return drift.AddonReconcileResult{}, addons.NotRead{Why: addons.Why{Reason: addons.ReasonCooldown, RetryAfter: 9}}
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
	if rr.Code != http.StatusServiceUnavailable || rr.Header().Get("Retry-After") != "9" || body.RetryAfter != 9 ||
		body.Error != codeTargetNotRead || body.Details["reason"] != addons.ReasonCooldown {
		t.Errorf("read again inside the cooldown, nothing kept: %d Retry-After %q %s, want 503 TARGET_NOT_READ cooldown, retry_after 9", rr.Code, rr.Header().Get("Retry-After"), rr.Body)
	}
	if len(asked) != 2 || asked[0] || !asked[1] {
		t.Errorf("asked fresh %v, want [false true]", asked)
	}
}

// Owner Q1: a removal inside the cooldown is refused with the countdown, and
// nothing is removed: the one 429 left. The list is a read, so inside the
// cooldown it is 503 TARGET_NOT_READ cooldown like every read (design §13.3).
func TestADormantRemovalInsideTheCooldownCountsDown(t *testing.T) {
	h := &sweepHarness{reportErr: fmt.Errorf("dormant: %w", addons.NotRead{Why: addons.Why{Reason: addons.ReasonCooldown, RetryAfter: 12}})}
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
	var listed ErrorResponse
	_ = json.Unmarshal(list.Body.Bytes(), &listed)
	if list.Code != http.StatusServiceUnavailable || list.Header().Get("Retry-After") != "12" ||
		listed.Error != codeTargetNotRead || listed.Details["reason"] != addons.ReasonCooldown {
		t.Errorf("the list inside the cooldown: want 503 TARGET_NOT_READ cooldown, got %d %q %s", list.Code, list.Header().Get("Retry-After"), list.Body)
	}
}

// A dormant read cut at the add-on's cap was answered, not refused: its own
// sentence, never "didn't serve this read".
func TestATruncatedDormantReadSaysItWasCut(t *testing.T) {
	stubSweep(t, &sweepHarness{reportErr: fmt.Errorf("dormant: %w: %w", services.ErrReadTruncated,
		addons.NotRead{Why: addons.Why{Reason: addons.ReasonReadRefused}})})
	r := httptest.NewRequest(http.MethodGet, "/api/v1/targets/truenas/accounts/dormant", nil)
	r.SetPathValue("target", "truenas")
	rr := httptest.NewRecorder()
	handleDormantAccounts(rr, r)
	var body ErrorResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if rr.Code != http.StatusServiceUnavailable || body.Error != codeTargetNotRead || !strings.Contains(body.Message, "only part of its accounts") {
		t.Errorf("a truncated dormant read: %d %s", rr.Code, rr.Body)
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
		return addons.Reach{Read: cooledDown(4)}
	}
	addonsActivity = func(_ context.Context, _, _, _, _ string, fresh bool) addons.ActivityReport {
		asked = append(asked, fresh)
		return addons.ActivityReport{Read: cooledDown(4)}
	}
	for _, q := range []string{"", "&fresh=true"} {
		r := httptest.NewRequest(http.MethodGet, "/api/v1/targets/truenas/reach?x=1"+q, nil)
		r.SetPathValue("target", "truenas")
		rr := httptest.NewRecorder()
		handleTargetReach(rr, r)
		if rr.Code != http.StatusServiceUnavailable || rr.Header().Get("Retry-After") != "4" {
			t.Errorf("the reach countdown did not reach the page: %d %s", rr.Code, rr.Body)
		}

		r = httptest.NewRequest(http.MethodGet, "/api/v1/targets/truenas/activity?subject=s"+q, nil)
		r.SetPathValue("target", "truenas")
		rr = httptest.NewRecorder()
		handleTargetActivity(rr, r)
		var act ErrorResponse
		_ = json.Unmarshal(rr.Body.Bytes(), &act)
		if rr.Code != http.StatusServiceUnavailable || act.Details["reason"] != addons.ReasonCooldown || act.RetryAfter != 4 {
			t.Errorf("the activity countdown did not reach the page: %d %s", rr.Code, rr.Body)
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
			return addons.Homes{Read: cooledDown(6)}
		}
		return addons.Homes{Read: addons.ReadOf(&addons.Observation{TakenAt: at, Stale: &addons.Stale{Reason: "cached", RetryAfter: &retry}}, addons.Why{})}
	}
	for _, q := range []string{"", "?fresh=true"} {
		r := httptest.NewRequest(http.MethodGet, "/api/v1/targets/truenas/homes"+q, nil)
		r.SetPathValue("target", "truenas")
		rr := httptest.NewRecorder()
		handleTargetHomes(rr, r)
		var body struct {
			Read    addons.ReadState `json:"read"`
			Details map[string]string
		}
		_ = json.Unmarshal(rr.Body.Bytes(), &body)
		if q == "" && (body.Read.State != addons.ReadKept || !body.Read.TakenAt.Equal(at) || body.Read.Reason != addons.ReasonCached || body.Read.RetryAfter != 20) {
			t.Errorf("a copy is dated and said kept: %s", rr.Body)
		}
		if q != "" && (rr.Code != http.StatusServiceUnavailable || rr.Header().Get("Retry-After") != "6" || body.Details["reason"] != addons.ReasonCooldown) {
			t.Errorf("a cooldown with nothing to serve counts down: %d %s", rr.Code, rr.Body)
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

// cooledDown is a fresh read refused inside the add-on's cooldown with
// nothing kept: no copy, with its countdown.
func cooledDown(retryAfter int) addons.ReadState {
	return addons.ReadOf(nil, addons.Why{Reason: addons.ReasonCooldown, RetryAfter: retryAfter})
}

// readNow is a read answered now.
func readNow() addons.ReadState {
	return addons.ReadOf(&addons.Observation{TakenAt: time.Now().UTC(), Current: true}, addons.Why{})
}
