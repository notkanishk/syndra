package main

import (
	"fmt"
	"net/http"
	"slices"
	"time"
)

// System-wide choices an operator makes through Syndra.
//
// One today: where home folders go. It is one place per system, not one per
// role (owner, 15 Oct): a role says only whether it gives a home folder and its
// limit, so two roles can never disagree about where a person's home is.
// Changing it later moves nothing. Syndra never moves a home folder, so homes
// already made stay where they are and new ones are made in the new place.
//
// Kept here, beside the bindings, because this add-on is what acts on it and
// the backend reaches it only through this add-on. The backend proxies the
// read and the write, so the operator still sets it in Syndra.

const settingHomeRoot = "home_root"

// SettingView is one setting as the operator sees it: its value, who set it,
// and what it may be.
type SettingView struct {
	Name  string     `json:"name"`
	Label string     `json:"label"`
	Value string     `json:"value"`
	SetBy string     `json:"set_by,omitempty"`
	SetAt *time.Time `json:"set_at,omitempty"`
	// Values is what it may be, with labels the way people say them. Read
	// from the NAS; ValuesRead false is "couldn't read", never "nothing".
	Values     []string          `json:"values"`
	Labels     map[string]string `json:"labels,omitempty"`
	ValuesRead bool              `json:"values_read"`
}

// homeRoot is where new home folders go, empty when none is chosen.
func (s *server) homeRoot() (string, error) {
	v, _, err := s.store.GetSetting(settingHomeRoot)
	if err != nil {
		return "", fmt.Errorf("read where home folders go: %w", err)
	}
	return v.Value, nil
}

func (s *server) homeRootView() (SettingView, error) {
	v, found, err := s.store.GetSetting(settingHomeRoot)
	if err != nil {
		return SettingView{}, err
	}
	view := SettingView{Name: settingHomeRoot, Label: "Where home folders go", Value: v.Value, SetBy: v.SetBy, Values: []string{}}
	if found {
		at := v.SetAt
		view.SetAt = &at
	}
	if places, labels, err := s.homePlaces(); err == nil {
		view.Values, view.Labels, view.ValuesRead = places, labels, true
	}
	return view, nil
}

func (s *server) handleSettings(w http.ResponseWriter, _ *http.Request, _ []byte) {
	view, err := s.homeRootView()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "SETTINGS_UNREADABLE"})
		return
	}
	writeJSON(w, http.StatusOK, map[string][]SettingView{"settings": {view}})
}

type putSettingRequest struct {
	Value string `json:"value"`
	Actor string `json:"actor"`
}

// handlePutSetting sets one setting. The value must be one the NAS offers
// right now, or empty to stop making new home folders. A NAS that can't be
// read refuses the write rather than accepting a place nobody checked.
func (s *server) handlePutSetting(w http.ResponseWriter, r *http.Request, body []byte) {
	if r.PathValue("name") != settingHomeRoot {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "NO_SUCH_SETTING"})
		return
	}
	var req putSettingRequest
	if err := decodeStrict(body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "INVALID_BODY"})
		return
	}
	if req.Value != "" {
		places, _, err := s.homePlaces()
		if err != nil {
			writeJSON(w, statusFor(err), map[string]string{"error": "TARGET_UNREADABLE"})
			return
		}
		if !slices.Contains(places, req.Value) {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "NOT_A_HOME_PLACE",
				"detail": "That isn't a folder at the top of a file share that is a dataset of its own."})
			return
		}
	}
	if err := s.store.PutSetting(settingHomeRoot, Setting{Value: req.Value, SetBy: req.Actor, SetAt: time.Now().UTC()}); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "SETTING_NOT_SAVED"})
		return
	}
	s.record("settings."+settingHomeRoot, "", req.Actor, "", "succeeded")
	view, err := s.homeRootView()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "SETTINGS_UNREADABLE"})
		return
	}
	writeJSON(w, http.StatusOK, view)
}
