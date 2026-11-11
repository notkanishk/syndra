package main

import (
	"encoding/json"
	"fmt"
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

// putHomeRoot writes the setting under a fresh call_id, as the backend does
// for each operator write.
func putHomeRoot(t *testing.T, s *server, value string) *httptest.ResponseRecorder {
	t.Helper()
	putSeq++
	return putHomeRootKeyed(t, s, value, fmt.Sprintf("op-row-fresh-%d", putSeq))
}

var putSeq int

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

// The write reads the NAS now, never the cache: a place deleted a minute after
// a good read must not pass while that read is still fresh.
func TestTheHomeLocationIsNotSetAgainstACachedRead(t *testing.T) {
	fake := reachFake()
	s := testServer(t, fake)
	getSettings(t, s) // a good read, cached and fresh
	fake.answers["filesystem.listdir /mnt/tank/main"] = `[{"name":"waddle","path":"/mnt/tank/main/waddle"}]`
	rr := putHomeRoot(t, s, "/mnt/tank/main/home_dir")
	if rr.Code != http.StatusUnprocessableEntity || problemCode(t, rr) != CodeNotAHomePlace {
		t.Fatalf("a place gone since the cached read must be refused, got %d %s", rr.Code, rr.Body)
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
	if len(out.Homes) != 1 || out.Homes[0].Username != "charlotte" || !out.QuotaRead {
		t.Fatalf("homes = %+v", out)
	}
	if h := out.Homes[0]; h.UsedBytes != 19756849561 || h.QuotaBytes != 21474836480 {
		t.Fatalf("charlotte = %+v", h)
	}
}

func putHomeRootKeyed(t *testing.T, s *server, value, callID string) *httptest.ResponseRecorder {
	t.Helper()
	body := []byte(fmt.Sprintf(`{"contract_version":%d,"call_id":%q,"value":%q,"actor":"op-1"}`, ContractVersion, callID, value))
	r := httptest.NewRequest(http.MethodPut, "/settings/home_root", nil)
	r.SetPathValue("name", "home_root")
	rr := httptest.NewRecorder()
	s.handlePutSetting(rr, r, body)
	return rr
}

// The backend sends its operation id as the body's call_id (contract 1 sent it
// as the Idempotency-Key header). A replay of the same call answers what the
// first call answered and writes nothing; the same call_id for another value
// is a different request, 422 CALL_ID_REUSED; a call without one is refused.
func TestASettingReplayIsAnsweredFromTheFirstCall(t *testing.T) {
	s := testServer(t, reachFake())
	first := putHomeRootKeyed(t, s, "/mnt/tank/main/home_dir", "op-row-1")
	if first.Code != http.StatusOK {
		t.Fatalf("first: %d %s", first.Code, first.Body.String())
	}
	if err := s.store.PutSetting(settingHomeRoot, Setting{Value: "/mnt/tank/main/moved"}); err != nil {
		t.Fatal(err)
	}
	replay := putHomeRootKeyed(t, s, "/mnt/tank/main/home_dir", "op-row-1")
	if replay.Code != http.StatusOK || replay.Body.String() != first.Body.String() {
		t.Fatalf("replay must answer as the first call did:\n first  %s\n replay %d %s", first.Body.String(), replay.Code, replay.Body.String())
	}
	if root, _ := s.homeRoot(); root != "/mnt/tank/main/moved" {
		t.Fatalf("a replay must write nothing; home root is now %q", root)
	}
	reused := putHomeRootKeyed(t, s, "/mnt/tank/main/quack", "op-row-1")
	if reused.Code != http.StatusUnprocessableEntity || problemCode(t, reused) != CodeCallIDReused {
		t.Fatalf("the same call_id for another value: want 422 CALL_ID_REUSED, got %d %s", reused.Code, reused.Body)
	}

	if rr := putHomeRootKeyed(t, s, "/mnt/tank/main/quack", "op-row-2"); rr.Code != http.StatusOK {
		t.Fatalf("a new call_id is a new call: %d", rr.Code)
	}
	if root, _ := s.homeRoot(); root != "/mnt/tank/main/quack" {
		t.Fatalf("a new call_id must write; home root is %q", root)
	}
	if rr := putHomeRootKeyed(t, s, "/mnt/tank/main/waddle", " "); rr.Code != http.StatusBadRequest || problemCode(t, rr) != CodeBadRequest {
		t.Fatalf("no call_id: want 400 BAD_REQUEST, got %d %s", rr.Code, rr.Body)
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

// A NAS that cannot be reached leaves the setting as it was, and the reply says
// nothing was changed (TARGET_UNREACHABLE), not "may have applied".
func TestASettingWriteThatReachedNothingSaysSo(t *testing.T) {
	fake := reachFake()
	s := testServer(t, fake)
	sessionGone(s.nas)()
	rr := putHomeRoot(t, s, "/mnt/tank/main/home_dir")
	if rr.Code != http.StatusServiceUnavailable || problemCode(t, rr) != CodeTargetUnreachable || rr.Header().Get("Retry-After") == "" {
		t.Fatalf("want 503 TARGET_UNREACHABLE with Retry-After, got %d %s", rr.Code, problemCode(t, rr))
	}
	if v, found, _ := s.store.GetSetting(settingHomeRoot); found && v.Value != "" {
		t.Fatalf("the setting changed to %q", v.Value)
	}
}
