package handlers

import (
	"net/http"
	"strings"

	"mkauth/internal/db"
)

type CreateMappingRuleRequest struct {
	SourceProject string `json:"source_project"`
	SourceRole    string `json:"source_role"`
	TargetProject string `json:"target_project"`
	TargetRole    string `json:"target_role"`
}

type CreateMappingRuleResponse struct {
	ID      string `json:"id"`
	Message string `json:"message"`
}

func handleGetMappingRules(w http.ResponseWriter, r *http.Request) {
	rules, err := db.GetActiveMappingRules(r.Context())
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	// Send [] instead of null
	if rules == nil {
		jsonResponse(w, http.StatusOK, []interface{}{})
		return
	}

	jsonResponse(w, http.StatusOK, rules)
}

func handleCreateMappingRule(w http.ResponseWriter, r *http.Request) {
	var req CreateMappingRuleRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}

	req.SourceProject = strings.TrimSpace(req.SourceProject)
	req.SourceRole = strings.TrimSpace(req.SourceRole)
	req.TargetProject = strings.TrimSpace(req.TargetProject)
	req.TargetRole = strings.TrimSpace(req.TargetRole)

	if req.SourceProject == "" || req.TargetProject == "" || req.SourceRole == "" || req.TargetRole == "" {
		jsonValidationErrorResponse(w, "source_project, source_role, target_project, and target_role are required", map[string]string{
			"source_project": "required",
			"source_role":    "required",
			"target_project": "required",
			"target_role":    "required",
		})
		return
	}

	if req.SourceProject == req.TargetProject && req.SourceRole == req.TargetRole {
		jsonValidationErrorResponse(w, "mapping rule cannot point to itself", map[string]string{
			"source_project": "must_not_equal_target",
			"source_role":    "must_not_equal_target",
		})
		return
	}

	// Circular dependency guard
	if err := db.DetectCycleOnInsert(r.Context(), req.SourceProject, req.SourceRole, req.TargetProject, req.TargetRole); err != nil {
		jsonErrorResponse(w, http.StatusConflict, "CYCLE_DETECTED", err.Error())
		return
	}

	id, err := db.CreateMappingRule(r.Context(), req.SourceProject, req.SourceRole, req.TargetProject, req.TargetRole)
	if err != nil {
		jsonErrorResponse(w, http.StatusConflict, "CONFLICT_ERROR", "Likely duplication: "+err.Error())
		return
	}

	// Audit log
	_ = db.InsertAuditLog(r.Context(), "system", "-", "mapping_rule.created", id)

	jsonResponse(w, http.StatusCreated, CreateMappingRuleResponse{
		ID:      id,
		Message: "Mapping Rule integrated seamlessly.",
	})
}

func handleUpdateMappingRule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		jsonErrorResponse(w, http.StatusBadRequest, "BAD_REQUEST", "Missing rule ID")
		return
	}

	if err := db.UpdateMappingRule(r.Context(), id); err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "UPDATE_FAILED", err.Error())
		return
	}

	// Audit log
	_ = db.InsertAuditLog(r.Context(), "system", "-", "mapping_rule.version_bumped", id)

	jsonResponse(w, http.StatusOK, map[string]string{"message": "Version incremented successfully"})
}
