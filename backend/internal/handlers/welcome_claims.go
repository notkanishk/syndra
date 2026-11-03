package handlers

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"syndra/internal/claimserve"
	"syndra/internal/services"
)

// Welcome roles on a new member's first sign-in (assessment SC4 option b,
// owner 3 Nov 2026): one makerspace-wide switch, off by default, whose one
// write place is Makerspace settings. The data plane reads it from the
// snapshot; this side computes what it sends (services.WelcomeClaims) and
// writes it through.

var svcWelcomeClaims = services.WelcomeClaims

// welcomeClaimsView is the switch as Makerspace settings reads it.
type welcomeClaimsView struct {
	On    bool       `json:"on"`
	SetBy *string    `json:"set_by"`
	SetAt *time.Time `json:"set_at"`
}

// GET /api/v1/config/welcome-claims
func handleGetWelcomeClaims(w http.ResponseWriter, r *http.Request) {
	c, set, err := dbGetConfigSettingMeta(r.Context(), services.ConfigKeyWelcomeClaims)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	view := welcomeClaimsView{On: c.Value == "true"}
	if set {
		view.SetBy, view.SetAt = &c.UpdatedBy, &c.UpdatedAt
	}
	jsonResponse(w, http.StatusOK, view)
}

type putWelcomeClaimsRequest struct {
	On *bool `json:"on"`
}

// PUT /api/v1/config/welcome-claims {"on": true}
//
// Saved, recorded in the change history when it changed, then published to
// the data plane straight away (the minute pass retries a failed publish).
func handlePutWelcomeClaims(w http.ResponseWriter, r *http.Request) {
	var req putWelcomeClaimsRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}
	if req.On == nil {
		jsonValidationErrorResponse(w, "on is required", map[string]string{"on": "required"})
		return
	}
	ctx := r.Context()
	cur, _, err := dbGetConfigSettingMeta(ctx, services.ConfigKeyWelcomeClaims)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	value := "false"
	if *req.On {
		value = "true"
	}
	actor := resolveActor(r, "")
	if err := dbSetConfigSetting(ctx, services.ConfigKeyWelcomeClaims, value, actor); err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	if (cur.Value == "true") != *req.On {
		audit := map[bool]string{true: "welcome_claims.turned_on", false: "welcome_claims.turned_off"}
		_ = dbInsertAuditLog(ctx, actor, "", audit[*req.On], "")
	}
	publishWelcomeAfterSave(ctx)
	now := time.Now().UTC()
	jsonResponse(w, http.StatusOK, welcomeClaimsView{On: *req.On, SetBy: &actor, SetAt: &now})
}

// publishWelcome computes the welcome switch and writes it through. A failed
// read leaves the last publish answering.
func publishWelcome(ctx context.Context) error {
	wc, err := svcWelcomeClaims(ctx)
	if err != nil {
		return err
	}
	rdb := claimConfigRedis()
	if rdb == nil {
		return errors.New("redis is not connected")
	}
	return claimserve.PublishWelcome(ctx, rdb, wc)
}

// publishWelcomeAfterSave publishes after a save already committed: the
// switch itself, and a role marked to wait, which must stop being sent early
// within a poll rather than a minute.
func publishWelcomeAfterSave(ctx context.Context) {
	if err := publishWelcome(ctx); err != nil {
		log.Printf("[WELCOME-CLAIMS] ERROR the data plane has not got this change yet (the next minute's publish retries): %v", err)
	}
}
