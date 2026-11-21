package handlers

import (
	"errors"
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
	target := r.PathValue("target")
	settings, read := addonsReadSettings(r.Context(), target)
	if read.None() {
		// Not "it has none": 503 TARGET_NOT_READ (design §13.3).
		writeUnread(w, r, target, read.Why())
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{"read": read, "settings": settings})
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
	resp, _ := addonsPutSetting(r.Context(), target, name, req.Value, actor)
	if errors.Is(resp.Err, addons.ErrOwnFault) {
		// Syndra's own failure before sending: not the add-on's to be blamed for.
		jsonErrorResponse(w, http.StatusInternalServerError, "SETTING_NOT_SET", resp.Err.Error())
		return
	}
	// Unreached or indeterminate, said apart (design §13.4): a timeout or a
	// 5xx is not "nothing was changed", the request went out and the answer
	// was lost. An add-on answering that its app is down is unreached too.
	if resp.Outcome == addons.OutcomeUnreached || resp.Outcome == addons.OutcomeIndeterminate {
		writeOutcome(w, notLandedOf(r.Context(), target, resp.Outcome, addons.Unread(resp)), nil)
		return
	}
	switch {
	case resp.Status == http.StatusOK:
		var view addons.TargetSetting
		if err := addons.DecodeReply(resp.Body, &view); err != nil {
			jsonErrorResponse(w, http.StatusBadGateway, "TARGET_ANSWER_UNREADABLE", "The system changed it, and its answer couldn't be read.")
			return
		}
		log.Printf("[TARGETS] %s set %s on %s to %q", actor, name, target, req.Value)
		jsonResponse(w, http.StatusOK, view)
	// By the add-on's code, not its status alone: a 404 is also a route its
	// manifest no longer declares, and a 422 is also a contract it does not
	// speak, and neither is about the value.
	case resp.Code == "NO_SUCH_SETTING":
		jsonErrorResponse(w, http.StatusNotFound, "NO_SUCH_SETTING", "This system has no such setting.")
	case resp.Code == "NOT_A_HOME_PLACE":
		jsonValidationErrorResponse(w, "That isn't a folder this system offers for home folders.",
			map[string]string{"value": "not_offered"})
	default:
		jsonErrorResponse(w, http.StatusServiceUnavailable, "TARGET_UNREADABLE",
			"The system couldn't check the folder just now, so nothing was changed.")
	}
}
