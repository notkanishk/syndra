package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func getSettings(t *testing.T, s *server) SettingView {
	t.Helper()
	rr := httptest.NewRecorder()
	s.handleSettings(rr, httptest.NewRequest(http.MethodGet, "/settings", nil), nil)
	var out struct {
		Settings []SettingView `json:"settings"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil || len(out.Settings) != 1 {
		t.Fatalf("settings: %d %s", rr.Code, rr.Body.String())
	}
	return out.Settings[0]
}

func putHomeRoot(t *testing.T, s *server, value string) *httptest.ResponseRecorder {
	t.Helper()
	body := []byte(`{"value":"` + value + `","actor":"op-1"}`)
	r := httptest.NewRequest(http.MethodPut, "/settings/home_root", nil)
	r.SetPathValue("name", "home_root")
	rr := httptest.NewRecorder()
	s.handlePutSetting(rr, r, body)
	return rr
}

// Where home folders go is chosen once, from the dataset folders at the top of
// each share, offered the way people say them. The share root, a plain
// directory, a hidden folder and a disabled share are not offered.
func TestTheHomeLocationIsChosenFromTheShareFolders(t *testing.T) {
	s := testServer(t, reachFake())
	view := getSettings(t, s)
	if !view.ValuesRead || view.Value != "" || view.SetAt != nil {
		t.Fatalf("unset: %+v", view)
	}
	if got := strings.Join(view.Values, ","); got != "/mnt/tank/main/home_dir,/mnt/tank/main/quack,/mnt/tank/main/waddle" {
		t.Fatalf("values = %s", got)
	}
	if view.Labels["/mnt/tank/main/home_dir"] != "main › home_dir" {
		t.Errorf("labels = %v", view.Labels)
	}

	if rr := putHomeRoot(t, s, "/mnt/tank/main/home_dir"); rr.Code != http.StatusOK {
		t.Fatalf("set: %d %s", rr.Code, rr.Body.String())
	}
	view = getSettings(t, s)
	if view.Value != "/mnt/tank/main/home_dir" || view.SetBy != "op-1" || view.SetAt == nil {
		t.Fatalf("after set: %+v", view)
	}
	if root, _ := s.homeRoot(); root != "/mnt/tank/main/home_dir" {
		t.Fatalf("converge reads %q", root)
	}
}

func TestOnlyAFolderTheNASOffersCanBeTheHomeLocation(t *testing.T) {
	s := testServer(t, reachFake())
	for _, bad := range []string{"/mnt/tank/main", "/mnt/tank/main/notes", "/mnt/tank/main/.ssh", "/etc"} {
		if rr := putHomeRoot(t, s, bad); rr.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s: want 422, got %d", bad, rr.Code)
		}
	}
	if root, _ := s.homeRoot(); root != "" {
		t.Fatalf("a refused value was kept: %q", root)
	}
	// Empty stops making new homes; nothing is moved or removed.
	putHomeRoot(t, s, "/mnt/tank/main/home_dir")
	if rr := putHomeRoot(t, s, ""); rr.Code != http.StatusOK {
		t.Fatalf("clear: %d", rr.Code)
	}
	if root, _ := s.homeRoot(); root != "" {
		t.Fatalf("cleared, still %q", root)
	}
}

// A NAS that can't be read refuses the write rather than taking a place
// nobody checked.
func TestTheHomeLocationIsNotSetBlind(t *testing.T) {
	fake := reachFake()
	fake.refuse = map[string]string{"filesystem.getacl": "[EACCES] no"}
	s := testServer(t, fake)
	if rr := putHomeRoot(t, s, "/mnt/tank/main/home_dir"); rr.Code == http.StatusOK {
		t.Fatal("set without reading what the NAS offers")
	}
}

// Nor against the last read when this one failed: a folder destroyed since
// would pass, and homes would be made as plain directories in its parent.
func TestTheHomeLocationIsNotSetAgainstAStaleRead(t *testing.T) {
	fake := reachFake()
	s := testServer(t, fake)
	getSettings(t, s) // a good read, now cached
	s.reach.readAt = time.Now().Add(-2 * reachMaxAge)
	fake.refuse = map[string]string{"filesystem.getacl": "[EACCES] no"}
	if rr := putHomeRoot(t, s, "/mnt/tank/main/home_dir"); rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503 against a stale read, got %d", rr.Code)
	}
}

func TestAnUnknownSettingIsNotFound(t *testing.T) {
	s := testServer(t, reachFake())
	r := httptest.NewRequest(http.MethodPut, "/settings/colour", nil)
	r.SetPathValue("name", "colour")
	rr := httptest.NewRecorder()
	s.handlePutSetting(rr, r, []byte(`{"value":"x"}`))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("unknown setting: %d", rr.Code)
	}
}

// Every home folder in a share Syndra considers, with what is saved in it and
// its limit, for the system page.
func TestHomesListsEachHomeWithItsUseAndLimit(t *testing.T) {
	fake := reachFake()
	fake.users = `[
		{"username":"charlotte","id":11,"uid":200001,"locked":false,"smb":true,"groups":[50],"home":"/mnt/tank/main/home_dir/charlotte"},
		{"username":"booker","id":12,"uid":200002,"locked":false,"smb":true,"groups":[50],"home":"/var/empty"},
		{"username":"cora","id":13,"uid":200003,"locked":false,"smb":true,"groups":[50],"home":"/mnt/tank/old/cora"}]`
	fake.answers["pool.dataset.get_quota tank/main/home_dir"] = `[{"id":200001,"used_bytes":19756849561,"quota":21474836480,"quota_type":"USER"}]`
	s := testServer(t, fake)

	rr := httptest.NewRecorder()
	s.handleHomes(rr, httptest.NewRequest(http.MethodGet, "/homes", nil), nil)
	var out HomesResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
	// bo has no home; cy's is in a disabled share, outside what Syndra reads.
	if len(out.Homes) != 1 || out.Homes[0].Username != "charlotte" || !out.UsageRead {
		t.Fatalf("homes = %+v", out)
	}
	if h := out.Homes[0]; h.UsedBytes != 19756849561 || h.QuotaBytes != 21474836480 {
		t.Fatalf("charlotte = %+v", h)
	}
}

func putHomeRootKeyed(t *testing.T, s *server, value, key string) *httptest.ResponseRecorder {
	t.Helper()
	body := []byte(`{"value":"` + value + `","actor":"op-1"}`)
	r := httptest.NewRequest(http.MethodPut, "/settings/home_root", nil)
	r.SetPathValue("name", "home_root")
	r.Header.Set("Idempotency-Key", key)
	rr := httptest.NewRecorder()
	s.handlePutSetting(rr, r, body)
	return rr
}

// The backend sends its operation id as Idempotency-Key. A replay of the same
// key answers what the first call answered and writes nothing, whatever it
// carries: it is the same call, retried after a lost answer. A call without a
// key (an older backend) is written as before.
func TestASettingReplayIsAnsweredFromTheFirstCall(t *testing.T) {
	s := testServer(t, reachFake())
	first := putHomeRootKeyed(t, s, "/mnt/tank/main/home_dir", "op-row-1")
	if first.Code != http.StatusOK {
		t.Fatalf("first: %d %s", first.Code, first.Body.String())
	}
	replay := putHomeRootKeyed(t, s, "/mnt/tank/main/quack", "op-row-1")
	if replay.Code != http.StatusOK || replay.Body.String() != first.Body.String() {
		t.Fatalf("replay must answer as the first call did:\n first  %s\n replay %d %s", first.Body.String(), replay.Code, replay.Body.String())
	}
	if root, _ := s.homeRoot(); root != "/mnt/tank/main/home_dir" {
		t.Fatalf("a replay must write nothing; home root is now %q", root)
	}

	if rr := putHomeRootKeyed(t, s, "/mnt/tank/main/quack", "op-row-2"); rr.Code != http.StatusOK {
		t.Fatalf("a new key is a new call: %d", rr.Code)
	}
	if root, _ := s.homeRoot(); root != "/mnt/tank/main/quack" {
		t.Fatalf("a new key must write; home root is %q", root)
	}
	if rr := putHomeRoot(t, s, "/mnt/tank/main/waddle"); rr.Code != http.StatusOK {
		t.Fatalf("no key: %d", rr.Code)
	}
	if root, _ := s.homeRoot(); root != "/mnt/tank/main/waddle" {
		t.Fatalf("a call without a key writes as before; home root is %q", root)
	}
}

// A refused call is not remembered: nothing was written, so a retry under the
// same key is judged afresh rather than answered with the refusal.
func TestARefusedSettingIsNotRemembered(t *testing.T) {
	s := testServer(t, reachFake())
	if rr := putHomeRootKeyed(t, s, "/mnt/tank/main/not_a_place", "op-row-9"); rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422 for a place the NAS does not offer, got %d", rr.Code)
	}
	if rr := putHomeRootKeyed(t, s, "/mnt/tank/main/home_dir", "op-row-9"); rr.Code != http.StatusOK {
		t.Fatalf("retry: %d %s", rr.Code, rr.Body.String())
	}
	if root, _ := s.homeRoot(); root != "/mnt/tank/main/home_dir" {
		t.Fatalf("a retry after a refusal must write; home root is %q", root)
	}
}
