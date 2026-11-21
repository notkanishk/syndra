package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

// A pass that halted, as the API sends it: the inventory is a 503 and the
// check a 200, and neither body carries a count, a list or a date it never
// read (holding is observed). The card read "0 people with an account missing"
// and "1 Jan 1" off exactly these bodies.
func TestAHaltedPassSendsNoCountListOrDate(t *testing.T) {
	origInv, origGet, origRec := svcTargetInventory, addonsGet, svcReconcileAddon
	t.Cleanup(func() { svcTargetInventory, addonsGet, svcReconcileAddon = origInv, origGet, origRec })
	// What Inventory and halt() return: the target, the flag and the reason, nothing counted.
	halted := drift.AddonReconcileResult{Target: "truenas", Halted: true, Reason: db.UnreconciledUnreachable}
	svcTargetInventory = func(context.Context, string, bool) (drift.AddonReconcileResult, error) { return halted, nil }
	addonsGet = func(string) (*addons.Addon, error) { return &addons.Addon{}, nil }
	svcReconcileAddon = func(context.Context, string) (drift.AddonReconcileResult, error) { return halted, nil }

	for _, c := range []struct {
		method, path string
		handle       http.HandlerFunc
		status       int
	}{
		{http.MethodGet, "/api/v1/targets/truenas/inventory", handleTargetInventory, http.StatusServiceUnavailable},
		{http.MethodPost, "/api/v1/targets/truenas/reconcile", handleReconcileTarget, http.StatusOK},
	} {
		r := httptest.NewRequest(c.method, c.path, nil)
		r.SetPathValue("target", "truenas")
		rr := httptest.NewRecorder()
		c.handle(rr, r)
		if rr.Code != c.status {
			t.Fatalf("%s: status %d, want %d: %s", c.path, rr.Code, c.status, rr.Body)
		}
		m := keysOf(t, rr.Body.Bytes())
		if string(m["halted"]) != "true" || string(m["reason"]) != `"`+db.UnreconciledUnreachable+`"` {
			t.Errorf("%s: the body must say it halted and why: %s", c.path, rr.Body)
		}
		for _, k := range []string{"bound", "accounts", "unmanaged", "stale", "read_at"} {
			if v, ok := m[k]; ok {
				t.Errorf("%s: a halted body carries %s=%s; nothing was read", c.path, k, v)
			}
		}
	}
}

// A read time nobody took is absent, never Go's zero time: the page would date
// it "1 Jan 1". The last storage read and a mapping's values, as sent.
func TestAZeroReadTimeIsNeverSent(t *testing.T) {
	for name, v := range map[string]any{
		"last storage":   lastKnownStorage{},
		"mapping values": mappableValuesAnswer{},
	} {
		raw, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		if got, ok := keysOf(t, raw)["read_at"]; ok {
			t.Errorf("%s: read_at=%s for a read never taken", name, got)
		}
	}
}

// The other side of the halted body: a whole read that found nothing sends
// its zeros, because they were counted. Absent means unread, never empty.
func TestAWholeEmptyReadSendsItsZeros(t *testing.T) {
	read := drift.AddonReconcileResult{Target: "truenas", Bound: new(0), Unmanaged: []drift.UnmanagedAccount{},
		Stale: []drift.StaleBinding{}, ReadAt: time.Now(), Current: true}
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
	if _, ok := m["read_at"]; !ok {
		t.Error("a whole read is dated")
	}
}
