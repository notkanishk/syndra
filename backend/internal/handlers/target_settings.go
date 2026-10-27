package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"syndra/internal/addons"
)

// A connected system's own settings, such as where its home folders go.
//
// The add-on keeps them and is the judge of what each may be: only it can read
// the NAS's shares. Syndra is where the operator changes them, so this relays
// the read and the write, and says who changed what.

func handleTargetSettings(w http.ResponseWriter, r *http.Request) {
	settings, ok := addonsReadSettings(r.Context(), r.PathValue("target"))
	if !ok {
		// 200: "couldn't read the settings right now" is something the page
		// says, not an error it fails on.
		jsonResponse(w, http.StatusOK, map[string]any{"read": false})
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{"read": true, "settings": settings})
}

type putTargetSettingRequest struct {
	Value string `json:"value"`
}

func handlePutTargetSetting(w http.ResponseWriter, r *http.Request) {
	target, name := r.PathValue("target"), r.PathValue("name")
	var req putTargetSettingRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}
	actor := resolveActor(r, "")
	resp, ok := addonsPutSetting(r.Context(), target, name, req.Value, actor)
	// First, because a timeout or a 5xx is not "nothing was changed": the
	// request was sent and the answer was lost, so the write may have landed.
	// Said the way every other add-on operation says it.
	if resp.Outcome == addons.OutcomeIndeterminate {
		jsonErrorResponse(w, http.StatusServiceUnavailable, "TARGET_UNCONFIRMED",
			"The system didn't confirm, so the change may have applied. Check the setting before trying again.")
		return
	}
	if !ok {
		jsonErrorResponse(w, http.StatusServiceUnavailable, "TARGET_UNREACHABLE",
			"The system didn't answer, so nothing was changed.")
		return
	}
	switch {
	case resp.Status == http.StatusOK:
		var view addons.TargetSetting
		if err := json.Unmarshal(resp.Body, &view); err != nil {
			jsonErrorResponse(w, http.StatusBadGateway, "TARGET_ANSWER_UNREADABLE", "The system changed it, and its answer couldn't be read.")
			return
		}
		log.Printf("[TARGETS] %s set %s on %s to %q", actor, name, target, req.Value)
		jsonResponse(w, http.StatusOK, view)
	case resp.Status == http.StatusNotFound:
		jsonErrorResponse(w, http.StatusNotFound, "NO_SUCH_SETTING", "This system has no such setting.")
	case resp.Status == http.StatusUnprocessableEntity:
		jsonValidationErrorResponse(w, "That isn't a folder this system offers for home folders.",
			map[string]string{"value": "not_offered"})
	default:
		jsonErrorResponse(w, http.StatusServiceUnavailable, "TARGET_UNREADABLE",
			"The system couldn't check the folder just now, so nothing was changed.")
	}
}
