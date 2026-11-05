package handlers

import (
	"net/http"
	"strings"

	"syndra/internal/access"
	"syndra/internal/services"
)

// handleRulePreview names what the automatic rules would also give a person
// with roles in one project: GET /users/{id}/rule-preview?project_id=&role_key=
// (role_key repeats for an adoption that carries several).
//
// Every consent surface that gives a role reads it before the operator
// confirms (coherence audit C1, C2): giving one role, approving a request,
// adopting drift. The answer is services.RulesFollowing, the same delta the
// write queues through db.FollowRules, never a closure computed in the UI.
func handleRulePreview(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	projectID := strings.TrimSpace(q.Get("project_id"))
	var roles []access.Role
	for _, rk := range q["role_key"] {
		if rk = strings.TrimSpace(rk); rk != "" {
			roles = append(roles, access.Role{ProjectID: projectID, RoleKey: rk})
		}
	}
	if projectID == "" || len(roles) == 0 {
		jsonValidationErrorResponse(w, "project_id and role_key are required",
			map[string]string{"project_id": "required", "role_key": "required"})
		return
	}
	rows, err := svcRulesFollowing(r.Context(), r.PathValue("id"), roles)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	if rows == nil {
		rows = []services.RuleRow{}
	}
	jsonResponse(w, http.StatusOK, map[string]any{"rows": rows})
}
