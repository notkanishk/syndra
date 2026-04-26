package handlers

import (
	"net/http"
	"strings"
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
	rules, err := dbGetActiveMappingRules(r.Context())
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	// Send [] instead of null
	if rules == nil {
		jsonResponse(w, http.StatusOK, []any{})
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
	if err := dbDetectCycleOnInsert(r.Context(), req.SourceProject, req.SourceRole, req.TargetProject, req.TargetRole); err != nil {
		jsonErrorResponse(w, http.StatusConflict, "CYCLE_DETECTED", err.Error())
		return
	}

	id, err := dbCreateMappingRule(r.Context(), req.SourceProject, req.SourceRole, req.TargetProject, req.TargetRole)
	if err != nil {
		jsonErrorResponse(w, http.StatusConflict, "CONFLICT_ERROR", "Likely duplication: "+err.Error())
		return
	}

	// Audit log
	_ = dbInsertAuditLog(r.Context(), "system", "-", "mapping_rule.created", id)

	jsonResponse(w, http.StatusCreated, CreateMappingRuleResponse{
		ID:      id,
		Message: "Mapping Rule integrated seamlessly.",
	})
}

// handleValidateMappingRule reports whether a candidate rule would create a
// cycle, without persisting anything. The UI calls this on every form-field
// change to surface the warning inline rather than waiting for the create POST
// to fail. Same shape as create plus a `would_cycle` and `self_reference`
// boolean in the response.
func handleValidateMappingRule(w http.ResponseWriter, r *http.Request) {
	var req CreateMappingRuleRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}

	req.SourceProject = strings.TrimSpace(req.SourceProject)
	req.SourceRole = strings.TrimSpace(req.SourceRole)
	req.TargetProject = strings.TrimSpace(req.TargetProject)
	req.TargetRole = strings.TrimSpace(req.TargetRole)

	type response struct {
		WouldCycle    bool   `json:"would_cycle"`
		SelfReference bool   `json:"self_reference"`
		Reason        string `json:"reason,omitempty"`
	}

	if req.SourceProject == "" || req.SourceRole == "" || req.TargetProject == "" || req.TargetRole == "" {
		// Tolerate partial input — the form calls this on every change.
		jsonResponse(w, http.StatusOK, response{})
		return
	}

	if req.SourceProject == req.TargetProject && req.SourceRole == req.TargetRole {
		jsonResponse(w, http.StatusOK, response{SelfReference: true, Reason: "Source and target are the same role"})
		return
	}

	if err := dbDetectCycleOnInsert(r.Context(), req.SourceProject, req.SourceRole, req.TargetProject, req.TargetRole); err != nil {
		jsonResponse(w, http.StatusOK, response{WouldCycle: true, Reason: err.Error()})
		return
	}
	jsonResponse(w, http.StatusOK, response{})
}

func handleUpdateMappingRule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		jsonErrorResponse(w, http.StatusBadRequest, "BAD_REQUEST", "Missing rule ID")
		return
	}

	if err := dbUpdateMappingRule(r.Context(), id); err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "UPDATE_FAILED", err.Error())
		return
	}

	// Audit log
	_ = dbInsertAuditLog(r.Context(), "system", "-", "mapping_rule.version_bumped", id)

	jsonResponse(w, http.StatusOK, map[string]string{"message": "Version incremented successfully"})
}
