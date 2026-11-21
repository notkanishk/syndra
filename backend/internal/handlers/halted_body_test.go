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
	"syndra/internal/db"
	"syndra/internal/services/drift"
)

// keysOf is a response body by key: what is absent is absent.
func keysOf(t *testing.T, body []byte) map[string]json.RawMessage {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("%v: %s", err, body)
	}
	return m
}

// A pass with no copy, as the API sends it: the inventory and the check are
// both 503 TARGET_NOT_READ with no data member (design §13.3). The card read
// "0 people with an account missing" and "1 Jan 1" off the old halted bodies.
func TestAHaltedPassSendsNoCountListOrDate(t *testing.T) {
	origInv, origGet, origRec := svcTargetInventory, addonsGet, svcReconcileAddon
	t.Cleanup(func() { svcTargetInventory, addonsGet, svcReconcileAddon = origInv, origGet, origRec })
	why := addons.Why{Reason: addons.ReasonAddonUnreachable, Cause: addons.CauseNoAnswer}
	svcTargetInventory = func(context.Context, string, bool) (drift.AddonReconcileResult, error) {
		return drift.AddonReconcileResult{}, addons.NotRead{Why: why}
	}
	addonsGet = func(string) (*addons.Addon, error) { return &addons.Addon{}, nil }
	svcReconcileAddon = func(context.Context, string) (drift.AddonReconcileResult, error) {
		return drift.AddonReconcileResult{Target: "truenas", Halted: true, Reason: db.UnreconciledAddonUnreachable,
			Read: addons.ReadOf(nil, why)}, nil
	}

	for _, c := range []struct {
		method, path string
		handle       http.HandlerFunc
	}{
		{http.MethodGet, "/api/v1/targets/truenas/inventory", handleTargetInventory},
		{http.MethodPost, "/api/v1/targets/truenas/reconcile", handleReconcileTarget},
	} {
		r := httptest.NewRequest(c.method, c.path, nil)
		r.SetPathValue("target", "truenas")
		rr := httptest.NewRecorder()
		c.handle(rr, r)
		if rr.Code != http.StatusServiceUnavailable || rr.Header().Get("Retry-After") == "" {
			t.Fatalf("%s: status %d, want 503 with Retry-After: %s", c.path, rr.Code, rr.Body)
		}
		m := keysOf(t, rr.Body.Bytes())
		if string(m["error"]) != `"`+codeTargetNotRead+`"` || string(m["details"]) != `{"cause":"no_answer","reason":"addon_unreachable"}` {
			t.Errorf("%s: the body must say not read, and why: %s", c.path, rr.Body)
		}
		for k := range m {
			if k != "error" && k != "message" && k != "details" && k != "retry_after" {
				t.Errorf("%s: no copy carries %s; nothing was read", c.path, k)
			}
		}
	}
}

// The other side of no copy: a whole read that found nothing sends its zeros,
// because they were counted. Absent means unread, never empty.
func TestAWholeEmptyReadSendsItsZeros(t *testing.T) {
	read := drift.AddonReconcileResult{Target: "truenas", Bound: new(0), Unmanaged: []drift.UnmanagedAccount{},
		Stale: []drift.StaleBinding{}, Read: addons.ReadOf(&addons.Observation{TakenAt: time.Now(), Current: true}, addons.Why{})}
	raw, err := json.Marshal(read)
	if err != nil {
		t.Fatal(err)
	}
	m := keysOf(t, raw)
	for k, want := range map[string]string{"bound": "0", "unmanaged": "[]", "stale": "[]"} {
		if string(m[k]) != want {
			t.Errorf("%s=%s, want %s: a count taken is sent, 0 included", k, m[k], want)
		}
	}
	if !json.Valid(m["read"]) || !strings.Contains(string(m["read"]), `"taken_at"`) {
		t.Error("a whole read is dated")
	}
}
