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
)

// A-09 For code: Syndra settles what a read can tell, and only that.

func TestASettingChangeIsSettledFromTheTargetsOwnRecordOfIt(t *testing.T) {
	sent := time.Date(2026, 11, 3, 9, 41, 0, 0, time.UTC)
	later, earlier := sent.Add(5*time.Second), sent.Add(-time.Hour)
	put := func(id, value string) db.AddonOperation {
		claimed := sent
		return db.AddonOperation{ID: id, Target: "truenas", Operation: "setting.put", ActorID: "op-1",
			SubjectID: "home_root", About: value, CreatedAt: sent, ClaimedAt: &claimed}
	}
	purge := db.AddonOperation{ID: "op_purge", Operation: "account.purge", CreatedAt: sent}

	cases := []struct {
		name    string
		setting addons.TargetSetting
		settled map[string]bool // id -> happened
		open    []string
	}{
		{"holds the value, set by the asker, after the send: it happened",
			addons.TargetSetting{Name: "home_root", Value: "/mnt/main/homes", SetBy: "op-1", SetAt: &later},
			map[string]bool{"op_set": true}, []string{"op_purge"}},
		{"last written before the send: it didn't",
			addons.TargetSetting{Name: "home_root", Value: "/mnt/old", SetBy: "op-2", SetAt: &earlier},
			map[string]bool{"op_set": false}, []string{"op_purge"}},
		{"changed again since, by someone else: a person decides",
			addons.TargetSetting{Name: "home_root", Value: "/mnt/other", SetBy: "op-2", SetAt: &later},
			map[string]bool{}, []string{"op_purge", "op_set"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			read, resolve := addonsReadSettings, dbResolveAddonOperation
			t.Cleanup(func() { addonsReadSettings, dbResolveAddonOperation = read, resolve })
			addonsReadSettings = func(context.Context, string) ([]addons.TargetSetting, bool) {
				return []addons.TargetSetting{c.setting}, true
			}
			got := map[string]bool{}
			dbResolveAddonOperation = func(_ context.Context, _, id string, happened bool, by string, _ time.Duration) (db.AddonOperation, error) {
				if by != db.SyndraReader {
					t.Errorf("a read settles in Syndra's name, got %q", by)
				}
				got[id] = happened
				return db.AddonOperation{}, nil
			}
			open := settleFromRead(context.Background(), "truenas", []db.AddonOperation{purge, put("op_set", "/mnt/main/homes")})
			if len(got) != len(c.settled) {
				t.Fatalf("settled %v, want %v", got, c.settled)
			}
			for id, happened := range c.settled {
				if v, ok := got[id]; !ok || v != happened {
					t.Errorf("%s: settled %v (ok %v), want %v", id, v, ok, happened)
				}
			}
			var ids []string
			for _, o := range open {
				ids = append(ids, o.ID)
			}
			if len(ids) != len(c.open) {
				t.Fatalf("open %v, want %v", ids, c.open)
			}
		})
	}
}

// A target that can't be read settles nothing: every row stays for a person.
func TestAnUnreadableTargetSettlesNothing(t *testing.T) {
	read, resolve := addonsReadSettings, dbResolveAddonOperation
	t.Cleanup(func() { addonsReadSettings, dbResolveAddonOperation = read, resolve })
	addonsReadSettings = func(context.Context, string) ([]addons.TargetSetting, bool) { return nil, false }
	dbResolveAddonOperation = func(context.Context, string, string, bool, string, time.Duration) (db.AddonOperation, error) {
		t.Error("nothing may be settled from a read that did not happen")
		return db.AddonOperation{}, nil
	}
	ops := []db.AddonOperation{{ID: "a", Operation: "setting.put"}, {ID: "b", Operation: "account.purge"}}
	if open := settleFromRead(context.Background(), "truenas", ops); len(open) != 2 {
		t.Fatalf("both rows stay open, got %d", len(open))
	}
}

// The list settles unsent records first, and carries no secret-shaped field.
func TestTheListSettlesUnsentRecordsAndNamesTheChange(t *testing.T) {
	unclaimed, list := dbSettleUnclaimedAddonOperations, dbListUnresolvedAddonOperations
	t.Cleanup(func() { dbSettleUnclaimedAddonOperations, dbListUnresolvedAddonOperations = unclaimed, list })
	var order []string
	dbSettleUnclaimedAddonOperations = func(context.Context, string, time.Duration) (int64, error) {
		order = append(order, "unclaimed")
		return 1, nil
	}
	sent := time.Date(2026, 11, 2, 18, 5, 0, 0, time.UTC)
	dbListUnresolvedAddonOperations = func(_ context.Context, target string, _ time.Duration, _ int) ([]db.AddonOperation, error) {
		order = append(order, "list")
		return []db.AddonOperation{{ID: "op_p", Target: target, Operation: "account.purge", ActorID: "op-1",
			SubjectID: "u-grace", About: "grace.hopper", CreatedAt: sent.Add(-time.Second), ClaimedAt: &sent}}, nil
	}

	r := httptest.NewRequest(http.MethodGet, "/api/v1/targets/truenas/unconfirmed", nil)
	r.SetPathValue("target", "truenas")
	rr := httptest.NewRecorder()
	handleListUnconfirmed(rr, r)
	if rr.Code != http.StatusOK || len(order) != 2 || order[0] != "unclaimed" {
		t.Fatalf("unsent records settle before the read: %d %v", rr.Code, order)
	}
	var out struct {
		Operations []map[string]any `json:"operations"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &out)
	if len(out.Operations) != 1 {
		t.Fatalf("one row, got %s", rr.Body.String())
	}
	row := out.Operations[0]
	if row["about"] != "grace.hopper" || row["sent_at"] != sent.Format(time.RFC3339) {
		t.Errorf("the row names the account and dates the send from the claim: %v", row)
	}
	for _, key := range []string{"status", "params", "password"} {
		if _, present := row[key]; present {
			t.Errorf("the row must not carry %q", key)
		}
	}
}
