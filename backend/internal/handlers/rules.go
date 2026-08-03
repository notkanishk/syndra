package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"

	"mkauth/internal/db"
)

type CreateMappingRuleRequest struct {
	SourceProject    string `json:"source_project"`
	SourceRole       string `json:"source_role"`
	TargetProject    string `json:"target_project"`
	TargetRole       string `json:"target_role"`
	ConfirmationMode string `json:"confirmation_mode,omitempty"`
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

	// How many people each rule applies to — the count of holders of its input
	// role. Editing a rule changes access for all of them at once, so the
	// number belongs in the list rather than behind the editor. Best-effort:
	// a failed count renders zeros, never a failed page.
	if counts, err := dbGetAssignedUserCounts(r.Context()); err == nil {
		for i := range rules {
			rules[i].HolderCount = counts[rules[i].SourceProject+":"+rules[i].SourceRole]
		}
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

	actor := getAdminUserID(r.Context())
	if actor == "" {
		actor = "system"
	}
	// The cascade owns the mutation now (create rule + per-holder outbox rows commit in one
	// tx via db.CreateMappingRuleAndEnqueue), then (auto mode) drains those rows. Enqueue
	// failure rolls back the mutation → 500; a drain failure rides in cascade.drain.
	mode, err := resolveConfirmationMode(r.Context(), req.ConfirmationMode)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	ruleID, cascade, err := svcCascadeRuleCreated(r.Context(), actor, req.SourceProject, req.SourceRole, req.TargetProject, req.TargetRole, mode)
	if err != nil {
		if db.IsUniqueViolation(err) {
			jsonErrorResponse(w, http.StatusConflict, "CONFLICT", "A mapping rule with this source/target combination already exists")
			return
		}
		jsonErrorResponse(w, http.StatusInternalServerError, "CASCADE_ERROR", err.Error())
		return
	}

	jsonResponse(w, http.StatusCreated, map[string]any{
		"id":      ruleID,
		"message": "Mapping Rule integrated seamlessly.",
		"cascade": cascade,
	})
}

// handleUpdateMappingRule is the 6th cascade trigger: a matcher/target change on an existing
// rule. It reads the PRE-update rule first — the updated fields alone don't tell us the OLD
// target, and the composition (add new-source holders, revoke old-source holders off the old
// target unless kept/covered) needs both. Validation mirrors handleCreateMappingRule (self-ref,
// cycle) but the cycle check must exclude this rule's own old edge (db.DetectCycleOnUpdate), or a
// valid retarget that only cycles WITH that old edge would be falsely rejected.
// PUT /api/v1/rules/mapping/{id}
func handleUpdateMappingRule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !trimmedNonEmpty(id) {
		jsonValidationErrorResponse(w, "id path parameter is required", map[string]string{"id": "required"})
		return
	}

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

	old, err := dbGetMappingRuleByID(r.Context(), id)
	if err != nil {
		jsonErrorResponse(w, http.StatusNotFound, "NOT_FOUND", "mapping rule not found")
		return
	}

	// Circular dependency guard — excludes this rule's own old edge from the graph first.
	if err := dbDetectCycleOnUpdate(r.Context(), id, req.SourceProject, req.SourceRole, req.TargetProject, req.TargetRole); err != nil {
		jsonErrorResponse(w, http.StatusConflict, "CYCLE_DETECTED", err.Error())
		return
	}

	actor := getAdminUserID(r.Context())
	if actor == "" {
		actor = "system"
	}
	// The cascade owns the mutation (update rule + add/revoke outbox rows commit in one tx via
	// db.UpdateMappingRuleAndEnqueue), then (auto mode) drains those rows. Enqueue failure rolls
	// back the mutation → 500; a drain failure rides in cascade.drain and is not fatal.
	cascade, err := svcCascadeRuleUpdated(r.Context(), actor, old, req.SourceProject, req.SourceRole, req.TargetProject, req.TargetRole)
	if err != nil {
		if db.IsUniqueViolation(err) {
			jsonErrorResponse(w, http.StatusConflict, "CONFLICT", "A mapping rule with this source/target combination already exists")
			return
		}
		jsonErrorResponse(w, http.StatusInternalServerError, "CASCADE_ERROR", err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"message": "Mapping rule updated",
		"cascade": cascade,
	})
}

// handleDeleteMappingRule retires a rule and takes back the access only it was granting.
//
// It reads the rule first for the same reason the update path does: the id alone does not say
// what the rule reached, and the revoke set is computed from the closure the rule was part of.
// There is no cycle check — removing an edge cannot create one — and no confirmation-mode
// argument: a deletion inherits the mode the rule itself carried, so a rule that queued its
// writes queues the writes that undo it.
// DELETE /api/v1/rules/mapping/{id}
func handleDeleteMappingRule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !trimmedNonEmpty(id) {
		jsonValidationErrorResponse(w, "id path parameter is required", map[string]string{"id": "required"})
		return
	}

	old, err := dbGetMappingRuleByID(r.Context(), id)
	if err != nil {
		jsonErrorResponse(w, http.StatusNotFound, "NOT_FOUND", "mapping rule not found")
		return
	}

	actor := getAdminUserID(r.Context())
	if actor == "" {
		actor = "system"
	}
	cascade, err := svcCascadeRuleDeleted(r.Context(), actor, old)
	if err != nil {
		// A concurrent delete lands here: the row was gone by the time the tx ran, and its
		// revokes were enqueued by whoever won. Reporting 500 would invite a retry of writes
		// that already exist.
		if errors.Is(err, pgx.ErrNoRows) {
			jsonErrorResponse(w, http.StatusNotFound, "NOT_FOUND", "mapping rule not found")
			return
		}
		jsonErrorResponse(w, http.StatusInternalServerError, "CASCADE_ERROR", err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"message": "Mapping rule deleted",
		"cascade": cascade,
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
