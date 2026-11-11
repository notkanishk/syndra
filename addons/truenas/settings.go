package main

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
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
		writeError(w, http.StatusInternalServerError, CodeSettingsUnreadable, "The settings could not be read.", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string][]SettingView{"settings": {view}})
}

type putSettingRequest struct {
	ContractVersion int    `json:"contract_version"`
	CallID          string `json:"call_id"`
	Value           string `json:"value"`
	Actor           string `json:"actor"`
}

// handlePutSetting sets one setting. The value must be one the NAS offers
// right now, or empty to stop making new home folders. A NAS that can't be
// read refuses the write rather than accepting a place nobody checked.
func (s *server) handlePutSetting(w http.ResponseWriter, r *http.Request, body []byte) {
	if r.PathValue("name") != settingHomeRoot {
		writeError(w, http.StatusNotFound, CodeNoSuchSetting, "This add-on has no such setting.", nil)
		return
	}
	var req putSettingRequest
	if !decodeRequest(w, body, &req, CodeInvalidBody) || !writeContractRefusal(w, req.ContractVersion) {
		return
	}
	if strings.TrimSpace(req.CallID) == "" {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "A setting write needs a call_id.", nil)
		return
	}
	// The same dedup the apply and the operations use, keyed by the call_id in
	// the body (contract 1 carried it as the Idempotency-Key header, because
	// this body had no field for it). A replay answers what the first call
	// answered and writes nothing.
	identity := map[string]any{"name": settingHomeRoot, "value": req.Value}
	s.once(w, req.CallID, kindSetting+settingHomeRoot, "", identity, func(w http.ResponseWriter) { s.putHomeRoot(w, req) })
}

func (s *server) putHomeRoot(w http.ResponseWriter, req putSettingRequest) {
	if req.Value != "" {
		// A read of the NAS now, never the copy and never the last read: a
		// place deleted a minute ago must not pass. Not kept: the write that
		// follows forgets every copy anyway (server.mutate).
		read, err := s.readSharesACLs()
		if errors.Is(err, ErrNotSent) {
			// Nothing was sent, and nothing is stored until the place is
			// checked: the one code Syndra reads as "nothing happened".
			writeError(w, http.StatusServiceUnavailable, CodeTargetUnreachable,
				"The NAS could not be reached, so the place could not be checked and nothing was changed.", nil)
			return
		}
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, CodeTargetUnreadable,
				"The NAS's folders could not be read just now, so the place could not be checked.", nil)
			return
		}
		if places, _ := homePlacesIn(read); !slices.Contains(places, req.Value) {
			writeError(w, http.StatusUnprocessableEntity, CodeNotAHomePlace,
				"That isn't a folder at the top of a file share that is a dataset of its own.", nil)
			return
		}
	}
	err := s.mutate(caller{"settings." + settingHomeRoot, "", req.Actor, req.CallID}, "store.setting", func() error {
		return s.store.PutSetting(settingHomeRoot, Setting{Value: req.Value, SetBy: req.Actor, SetAt: time.Now().UTC()})
	})
	switch {
	case errors.Is(err, errNotLogged):
		writeError(w, http.StatusInternalServerError, CodeSettingNotSaved, notLoggedDetail, nil)
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, CodeSettingNotSaved, "The setting could not be stored.", nil)
		return
	}
	view, err := s.homeRootView()
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeSettingsUnreadable, "The settings could not be read back.", nil)
		return
	}
	writeJSON(w, http.StatusOK, view)
}
