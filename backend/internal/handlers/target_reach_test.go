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

// The mappings page renders "not read from TrueNAS yet" as an answer, so the
// endpoint answers 200 either way, and says which.
func TestTargetReachIsAnAnswerEvenWhenNothingWasRead(t *testing.T) {
	reach := addonsReadReach
	t.Cleanup(func() { addonsReadReach = reach })

	read := time.Date(2026, 10, 14, 9, 2, 0, 0, time.UTC)
	addonsReadReach = func(context.Context, string, bool) addons.Reach {
		return addons.Reach{Read: addons.ReadOf(&addons.Observation{TakenAt: read, Stale: &addons.Stale{Reason: "unreachable"}}, addons.Why{}), Groups: map[string][]addons.Place{
			"community": {{Share: "main", Folder: "waddle_share", Access: "change"}},
			"interns":   {},
		}}
	}
	got := getTargetReach(t)
	if got.Read.State != addons.ReadKept || got.Read.Reason != addons.ReasonAppUnreachable || !got.Read.TakenAt.Equal(read) || len(got.Groups["community"]) != 1 {
		t.Fatalf("got %+v", got)
	}
	if opens, ok := got.Groups["interns"]; !ok || len(opens) != 0 {
		t.Fatalf("a group that opens nothing must arrive as an empty list: %v %v", opens, ok)
	}

	addonsReadReach = func(context.Context, string, bool) addons.Reach {
		return addons.Reach{Read: addons.ReadOf(nil, addons.Why{Reason: addons.ReasonAddonUnreachable})}
	}
	r := httptest.NewRequest(http.MethodGet, "/api/v1/targets/truenas/reach", nil)
	r.SetPathValue("target", "truenas")
	rr := httptest.NewRecorder()
	handleTargetReach(rr, r)
	if rr.Code != http.StatusServiceUnavailable || strings.Contains(rr.Body.String(), "groups") {
		t.Fatalf("nothing read: want 503 and no groups, got %d %s", rr.Code, rr.Body)
	}
}

func getTargetReach(t *testing.T) addons.Reach {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/targets/truenas/reach", nil)
	r.SetPathValue("target", "truenas")
	rr := httptest.NewRecorder()
	handleTargetReach(rr, r)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
	var out addons.Reach
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}
