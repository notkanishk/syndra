package handlers

import (
	"net/http"
	"regexp"
	"time"

	"syndra/internal/db"
)

// Roles that wait for you (reconcile N14 option B, N16: its one write place).
// A marked role waits in Pending changes whenever anyone would gain or lose
// it, however it happens; every other role goes straight to Zitadel. Marking
// is a setting, not a version: no plan, recorded in the audit log, and
// changes already waiting stay waiting.

var (
	dbListRoleWaits        = db.ListRoleWaits
	dbMarkRoleWaits        = db.MarkRoleWaits
	dbUnmarkRoleWaits      = db.UnmarkRoleWaits
	dbListWaitSuggestions  = db.ListWaitSuggestions
	dbGetConfigSettingMeta = db.GetConfigSettingMeta
)

// A role is named the way it was created: letters, numbers, dot, dash,
// underscore. Project ids are Zitadel's.
var roleWaitPart = regexp.MustCompile(`^[A-Za-z0-9._-]{1,255}$`)

type switchOver struct {
	SetBy string    `json:"set_by"`
	SetAt time.Time `json:"set_at"`
}

type rolesThatWaitResponse struct {
	Roles []db.RoleWait `json:"roles"`
	// SwitchedOver: only these roles decide whether a change waits. Until
	// then, bundles and rules set to wait still hold their changes too, and
	// Suggestions is the one-time move.
	SwitchedOver bool                `json:"switched_over"`
	Switch       *switchOver         `json:"switch,omitempty"`
	Suggestions  []db.WaitSuggestion `json:"suggestions"`
}

// GET /api/v1/roles-that-wait
func handleListRolesThatWait(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	roles, err := dbListRoleWaits(ctx)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	resp := rolesThatWaitResponse{Roles: roles, Suggestions: []db.WaitSuggestion{}}
	if resp.Roles == nil {
		resp.Roles = []db.RoleWait{}
	}
	model, set, err := dbGetConfigSettingMeta(ctx, db.ConfigKeyConfirmationModel)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	if set && model.Value == db.ConfirmationModelRoles {
		resp.SwitchedOver = true
		resp.Switch = &switchOver{SetBy: model.UpdatedBy, SetAt: model.UpdatedAt}
	} else {
		sugg, err := dbListWaitSuggestions(ctx)
		if err != nil {
			jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
			return
		}
		if sugg != nil {
			resp.Suggestions = sugg
		}
	}
	jsonResponse(w, http.StatusOK, resp)
}

func roleWaitTarget(w http.ResponseWriter, r *http.Request) (string, string, bool) {
	project, role := r.PathValue("project_id"), r.PathValue("role_key")
	if !roleWaitPart.MatchString(project) || !roleWaitPart.MatchString(role) {
		jsonValidationErrorResponse(w, "project_id and role_key are required",
			map[string]string{"project_id": "format", "role_key": "format"})
		return "", "", false
	}
	return project, role, true
}

// PUT /api/v1/roles-that-wait/{project_id}/{role_key}
func handleMarkRoleWaits(w http.ResponseWriter, r *http.Request) {
	project, role, ok := roleWaitTarget(w, r)
	if !ok {
		return
	}
	// A role Zitadel doesn't offer can't be given, so it has nothing to wait
	// for; refusing it keeps the list to roles that exist.
	if err := checkRoleOffered(r.Context(), project, role); err != nil {
		writeRoleOfferedError(w, err)
		return
	}
	actor := resolveActor(r, "")
	created, err := dbMarkRoleWaits(r.Context(), project, role, actor)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	if created {
		_ = dbInsertAuditLog(r.Context(), actor, "", "role.waits_marked", project+":"+role)
	}
	jsonResponse(w, http.StatusOK, map[string]bool{"marked": true})
}

// DELETE /api/v1/roles-that-wait/{project_id}/{role_key}
func handleUnmarkRoleWaits(w http.ResponseWriter, r *http.Request) {
	project, role, ok := roleWaitTarget(w, r)
	if !ok {
		return
	}
	removed, err := dbUnmarkRoleWaits(r.Context(), project, role)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	if removed {
		_ = dbInsertAuditLog(r.Context(), resolveActor(r, ""), "", "role.waits_unmarked", project+":"+role)
	}
	jsonResponse(w, http.StatusOK, map[string]bool{"marked": false})
}

type switchOverRequest struct {
	Model string `json:"model"`
}

// PUT /api/v1/config/confirmation-model {"model":"roles"}
//
// The switch over, after the one-time move: from now on only roles that wait
// hold a change, and bundles' and rules' own modes stop counting. One way:
// there is nothing to switch back to once those modes are gone.
func handleSwitchToRolesThatWait(w http.ResponseWriter, r *http.Request) {
	var req switchOverRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}
	if req.Model != db.ConfirmationModelRoles {
		jsonValidationErrorResponse(w, `model must be "roles"`, map[string]string{"model": "enum"})
		return
	}
	actor := resolveActor(r, "")
	if err := dbSetConfigSetting(r.Context(), db.ConfigKeyConfirmationModel, req.Model, actor); err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	_ = dbInsertAuditLog(r.Context(), actor, "", "confirmation.switched_to_roles", "")
	jsonResponse(w, http.StatusOK, map[string]string{"model": req.Model})
}
