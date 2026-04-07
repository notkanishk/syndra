package handlers

import (
	"encoding/json"
	"net/http"

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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErrorResponse(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid JSON body")
		return
	}

	if req.SourceProject == "" || req.TargetProject == "" {
		jsonErrorResponse(w, http.StatusBadRequest, "VALIDATION_FAILED", "Source and Target projects are strictly required")
		return
	}

	id, err := db.CreateMappingRule(r.Context(), req.SourceProject, req.SourceRole, req.TargetProject, req.TargetRole)
	if err != nil {
		jsonErrorResponse(w, http.StatusConflict, "CONFLICT_ERROR", "Likely duplication: "+err.Error())
		return
	}

	// Future hook: Trigger Redis/Zitadel cache invalidation here!

	jsonResponse(w, http.StatusCreated, CreateMappingRuleResponse{
		ID:      id,
		Message: "Mapping Rule integrated seamlessly.",
	})
}
