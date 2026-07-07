package handlers

import (
	"context"
	"net/http"
	"strings"

	"mkauth/internal/db"
	"mkauth/internal/models"
)

// resolveConfirmationMode returns reqMode (normalized) when the caller supplied one, else the
// configured global default (db.ConfigKeyDefaultConfirmationMode), falling back to "auto" when
// no default has been set either. Shared by handleCreateBundle and handleCreateMappingRule (Task
// 22 Step 1) so new bundles/rules inherit the operator's global policy unless overridden per-call.
func resolveConfirmationMode(ctx context.Context, reqMode string) (string, error) {
	if trimmedNonEmpty(reqMode) {
		return db.NormalizeConfirmationMode(strings.TrimSpace(reqMode)), nil
	}
	def, err := dbGetConfigSetting(ctx, db.ConfigKeyDefaultConfirmationMode)
	if err != nil {
		return "", err
	}
	return db.NormalizeConfirmationMode(def), nil // "" normalizes to "auto"
}

// handleGetGlobalConfirmationDefault reports the current global default confirmation mode.
// User-gated (not operator-only): the create-bundle/create-rule forms need this to prefill their
// mode selector for any authenticated user, not just operators.
// GET /api/v1/config/confirmation-mode-default
func handleGetGlobalConfirmationDefault(w http.ResponseWriter, r *http.Request) {
	def, err := dbGetConfigSetting(r.Context(), db.ConfigKeyDefaultConfirmationMode)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, map[string]string{"mode": db.NormalizeConfirmationMode(def)})
}

type setGlobalConfirmationDefaultRequest struct {
	Mode string `json:"mode"`
}

// handleSetGlobalConfirmationDefault changes the global default confirmation mode. Operator-gated
// (see router.go) — this is global policy, same posture as the welcome-bundle toggle.
// PUT /api/v1/config/confirmation-mode-default
func handleSetGlobalConfirmationDefault(w http.ResponseWriter, r *http.Request) {
	var req setGlobalConfirmationDefaultRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}
	if req.Mode != "auto" && req.Mode != "manual" {
		jsonValidationErrorResponse(w, "mode must be auto or manual", map[string]string{"mode": "must_be_auto_or_manual"})
		return
	}
	actor := getAdminUserID(r.Context())
	if actor == "" {
		actor = "system"
	}
	if err := dbSetConfigSetting(r.Context(), db.ConfigKeyDefaultConfirmationMode, req.Mode, actor); err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, map[string]string{"mode": req.Mode})
}

type bulkConfirmationModeRequest struct {
	Kind string   `json:"kind"` // rule | bundle
	IDs  []string `json:"ids"`
	Mode string   `json:"mode"`
}

// handleBulkSetConfirmationMode bulk-toggles confirmation_mode on the selected rules or bundles
// in one statement (db.SetRuleConfirmationMode / db.SetBundleConfirmationMode). Operator-gated —
// same posture as other cross-cutting policy mutations.
// POST /api/v1/policies/confirmation-mode
func handleBulkSetConfirmationMode(w http.ResponseWriter, r *http.Request) {
	var req bulkConfirmationModeRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}
	if req.Kind != "rule" && req.Kind != "bundle" {
		jsonValidationErrorResponse(w, "kind must be rule or bundle", map[string]string{"kind": "must_be_rule_or_bundle"})
		return
	}
	if len(req.IDs) == 0 {
		jsonValidationErrorResponse(w, "ids must not be empty", map[string]string{"ids": "required"})
		return
	}
	if req.Mode != "auto" && req.Mode != "manual" {
		jsonValidationErrorResponse(w, "mode must be auto or manual", map[string]string{"mode": "must_be_auto_or_manual"})
		return
	}

	var err error
	if req.Kind == "rule" {
		err = dbSetRuleConfirmationMode(r.Context(), req.IDs, req.Mode)
	} else {
		err = dbSetBundleConfirmationMode(r.Context(), req.IDs, req.Mode)
	}
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{"updated": len(req.IDs), "kind": req.Kind, "mode": req.Mode})
}

// recentCascadesLimit caps the "Recent cascades" feed — an operator-facing glance list, not a
// paginated worklist.
const recentCascadesLimit = 50

// handleGetRecentCascades lists the most recently applied cascade-originated outbox rows, so
// automated bundle/rule/lifecycle projections are never invisible to the operator.
// GET /api/v1/propagations/cascades
func handleGetRecentCascades(w http.ResponseWriter, r *http.Request) {
	rows, err := dbGetRecentCascades(r.Context(), recentCascadesLimit)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	if rows == nil {
		rows = []models.CascadeSummary{}
	}
	jsonResponse(w, http.StatusOK, map[string]any{"cascades": rows})
}

// handleRemoveBundleFromUser is the revoke-side counterpart to handleAssignBundleToUser. The
// cascade OWNS the mutation (delete assignment + per-role revoke outbox rows commit in one tx via
// db.RemoveBundleFromUserAndEnqueue), then (auto mode) drains those rows. The handler only
// validates path params, resolves the actor, and reports the cascade outcome.
// DELETE /api/v1/users/{id}/bundles/{bundleId}
func handleRemoveBundleFromUser(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	bundleID := r.PathValue("bundleId")
	if !trimmedNonEmpty(userID) || !trimmedNonEmpty(bundleID) {
		jsonValidationErrorResponse(w, "id and bundleId are required", map[string]string{
			"id":       "required",
			"bundleId": "required",
		})
		return
	}

	actor := getAdminUserID(r.Context())
	if actor == "" {
		actor = "system"
	}
	cascade, err := svcCascadeBundleRemoved(r.Context(), actor, userID, bundleID)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "CASCADE_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{
		"message": "Bundle removed from user",
		"cascade": cascade,
	})
}

// handleRemoveRoleFromBundle is the revoke-side counterpart to handleAddRoleToBundle. The cascade
// OWNS the mutation (delete bundle_role + per-member revoke outbox rows commit in one tx via
// db.RemoveRoleFromBundleAndEnqueue), then (auto mode) drains those rows.
// DELETE /api/v1/bundles/{id}/roles/{projectId}/{roleKey}
func handleRemoveRoleFromBundle(w http.ResponseWriter, r *http.Request) {
	bundleID := r.PathValue("id")
	projectID := r.PathValue("projectId")
	roleKey := r.PathValue("roleKey")
	if !trimmedNonEmpty(bundleID) || !trimmedNonEmpty(projectID) || !trimmedNonEmpty(roleKey) {
		jsonValidationErrorResponse(w, "id, projectId, and roleKey are required", map[string]string{
			"id":        "required",
			"projectId": "required",
			"roleKey":   "required",
		})
		return
	}

	actor := getAdminUserID(r.Context())
	if actor == "" {
		actor = "system"
	}
	cascade, err := svcCascadeRoleRemoved(r.Context(), actor, bundleID, projectID, roleKey)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "CASCADE_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{
		"message": "Role removed from bundle",
		"cascade": cascade,
	})
}
