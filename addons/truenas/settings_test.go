package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
