package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"syndra/internal/addons"
	"syndra/internal/db"
	"syndra/internal/services"
)

// Role-to-target mapping CRUD (change `addon-platform` group 7).
//
// Validation is split, and the split is the design. Syndra checks STRUCTURE:
// the field is one the add-on's manifest declares, it is not a lifecycle field,
// and no binding already exists for that (target, project, role, field). The
// add-on checks REFERENCE: that `lab_makers` actually resolves on its target.
// Syndra cannot do the second — it does not know what the value means — so a
// mapping write is validated through the add-on and refused if it cannot
// confirm the referent.
//
// The structural half runs first, and runs even when the add-on is unreachable.
// A field the schema does not declare is wrong whatever the target says, and
// spending a network call to be told so would make an outage look like a
// validation failure.

type mappingRequest struct {
	Target    string `json:"target"`
	ProjectID string `json:"project_id"`
	RoleKey   string `json:"role_key"`
	Field     string `json:"field"`
	Value     string `json:"value"`
}

func handleListRoleMappings(w http.ResponseWriter, r *http.Request) {
	rows, err := dbListRoleMappings(r.Context(), r.URL.Query().Get("target"))
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	if rows == nil {
		rows = []db.RoleMapping{}
	}
	jsonResponse(w, http.StatusOK, map[string]any{"mappings": rows})
}

func handleCreateRoleMapping(w http.ResponseWriter, r *http.Request) {
	var req mappingRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}
	if err := validateMappingAgainstTarget(r.Context(), req.Target, req.Field, req.Value); err != nil {
		writeMappingError(w, err)
		return
	}

	created, err := dbCreateRoleMapping(r.Context(), db.RoleMapping{
		Target: req.Target, ProjectID: req.ProjectID, RoleKey: req.RoleKey,
		Field: req.Field, Value: req.Value, CreatedBy: resolveActor(r, ""),
	})
	if err != nil {
		writeMappingError(w, err)
		return
	}
	jsonResponse(w, http.StatusCreated, created)
}

type mappingValueRequest struct {
	Value string `json:"value"`
	// PlanID cites the rehearsal that showed who this moves. Required whenever
	// the mapping has holders — an edit that reaches nobody is a change to a
	// definition and there is nothing to review.
	PlanID string `json:"plan_id,omitempty"`
}

func handleUpdateRoleMapping(w http.ResponseWriter, r *http.Request) {
	var req mappingValueRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}
	// Read first, so the value is validated against the target this mapping is
	// actually on rather than one the caller names. Only the value moves:
	// re-pointing a mapping at a different role is deleting one binding and
	// creating another, with a different cohort on each side.
	existing, err := dbGetRoleMapping(r.Context(), r.PathValue("id"))
	if err != nil {
		writeMappingError(w, err)
		return
	}
	if err := validateMappingAgainstTarget(r.Context(), existing.Target, existing.Field, req.Value); err != nil {
		writeMappingError(w, err)
		return
	}
	applyMappingRequest(w, r, existing, planSurfaceMappingEdit, req.PlanID, req.Value, "updated")
}

type mappingDeleteRequest struct {
	PlanID string `json:"plan_id,omitempty"`
}

func handleDeleteRoleMapping(w http.ResponseWriter, r *http.Request) {
	// A DELETE with a body, because the citation has to travel somewhere and a
	// query parameter is a place approvals end up in browser history and access
	// logs. An empty body is tolerated: a mapping nobody holds needs no citation.
	var req mappingDeleteRequest
	if r.Body != nil {
		_ = decodeJSONStrict(r.Body, &req)
	}
	existing, err := dbGetRoleMapping(r.Context(), r.PathValue("id"))
	if err != nil {
		writeMappingError(w, err)
		return
	}
	applyMappingRequest(w, r, existing, planSurfaceMappingDelete, req.PlanID, "", "deleted")
}

// applyMappingRequest is the shared half: count the cohort, spend the approval
// if there is one to spend, make the change, and queue the convergences.
func applyMappingRequest(w http.ResponseWriter, r *http.Request, m db.RoleMapping, surface, planID, newValue, verb string) {
	holders, err := dbMappingHolders(r.Context(), m.ProjectID, m.RoleKey)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	converged, err := applyMappingChange(r.Context(), m, surface, resolveActor(r, ""), planID, newValue, len(holders))
	if err != nil {
		writeMappingPlanError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{
		"status": verb,
		// What was queued, not what was applied. The convergences wait for the
		// drain like every other add-on row, and reporting them as done is the
		// one thing a surface must not say.
		"queued_convergences": converged,
	})
}

// handleMappingHolders reports the cohort a mapping edit or delete would move,
// so the surface can show the blast radius before anything lands.
func handleMappingHolders(w http.ResponseWriter, r *http.Request) {
	m, err := dbGetRoleMapping(r.Context(), r.PathValue("id"))
	if err != nil {
		writeMappingError(w, err)
		return
	}
	holders, err := dbMappingHolders(r.Context(), m.ProjectID, m.RoleKey)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	if holders == nil {
		holders = []string{}
	}
	jsonResponse(w, http.StatusOK, map[string]any{"mapping": m, "holders": holders, "count": len(holders)})
}

type publishMappingRequest struct {
	Target string `json:"target"`
	Note   string `json:"note"`
}

func handlePublishMappingVersion(w http.ResponseWriter, r *http.Request) {
	var req publishMappingRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}
	if strings.TrimSpace(req.Target) == "" {
		jsonValidationErrorResponse(w, "target is required", map[string]string{"target": "required"})
		return
	}
	// The same guard its sibling runs, and skipped here it let an unregistered
	// target reach the foreign key — which answers 500 with raw constraint text,
	// the failure validateMappingAgainstTarget exists to prevent.
	if _, err := addonsEntitlementSchema(req.Target); errors.Is(err, addons.ErrNotRegistered) {
		jsonValidationErrorResponse(w,
			fmt.Sprintf("%s is not a registered add-on target", req.Target),
			map[string]string{"target": "unregistered"})
		return
	}
	version, err := dbPublishMappingVersion(r.Context(), req.Target, req.Note, resolveActor(r, ""))
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusCreated, map[string]any{"target": req.Target, "version": version})
}

func handleRollbackMappingVersion(w http.ResponseWriter, r *http.Request) {
	target := r.PathValue("target")
	version, err := strconv.Atoi(r.PathValue("version"))
	if err != nil || version < 1 {
		jsonValidationErrorResponse(w, "version must be a positive integer", map[string]string{"version": "invalid"})
		return
	}
	actor := resolveActor(r, "")
	converged, err := rollbackAndConverge(r.Context(), target, version, actor)
	if err != nil {
		writeMappingError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{
		"status": "rolled_back", "target": target, "version": version,
		// Restoring the bindings is only half of a rollback. The people who
		// hold the affected roles are still converged to what the reverted set
		// said, and nothing else would ever notice: a rollback that changed the
		// definition and left the target alone is the definition and the world
		// disagreeing, silently, which is exactly what a rollback is for undoing.
		"queued_convergences": converged,
	})
}

// validateMappingAgainstTarget runs both halves of the split, structure first.
func validateMappingAgainstTarget(ctx context.Context, target, field, value string) error {
	schema, err := addonsEntitlementSchema(target)
	switch {
	case errors.Is(err, addons.ErrNotRegistered):
		// Nothing about a mapping to a target the deployment does not run can
		// be validated, and the foreign key would refuse the row anyway — with
		// a message about a constraint rather than about the deployment.
		return fmt.Errorf("%w: %s is not a registered add-on target", db.ErrMappingInvalid, target)
	case err != nil:
		// Registered and never answered. Structure could still be checked
		// against a manifest we do not have, which is to say it could not.
		return fmt.Errorf("%w: %s has not published a capability manifest yet, so its entitlement schema is unknown", errMappingTargetSilent, target)
	}

	declared := make([]string, 0, len(schema))
	for _, f := range schema {
		// A lifecycle field the add-on declares is still not bindable. Both
		// sides say so — the manifest flag and the backend's own list — because
		// the manifest is the least trusted input in the system and this is a
		// rule about Syndra's resolver, not about the target.
		if f.Lifecycle || services.IsLifecycleField(f.Name) {
			continue
		}
		declared = append(declared, f.Name)
	}
	if err := services.ValidateMappingField(declared, field); err != nil {
		return err
	}

	// Reference, last, and only once structure holds. This is the network call,
	// and spending it to be told a field is misspelled would make an outage
	// look like a validation failure.
	if err := addonsResolvesValue(ctx, target, field, value); err != nil {
		return err
	}
	return nil
}

// errMappingTargetSilent separates "the add-on is wrong" from "the add-on has
// not spoken". An operator whose mapping is refused needs to know which.
var errMappingTargetSilent = errors.New("the target has not published its capabilities")

func writeMappingError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, db.ErrMappingExists):
		jsonErrorResponse(w, http.StatusConflict, "MAPPING_EXISTS", err.Error())
	case errors.Is(err, db.ErrMappingNotFound):
		jsonErrorResponse(w, http.StatusNotFound, "MAPPING_NOT_FOUND", err.Error())
	case errors.Is(err, db.ErrMappingInvalid):
		jsonValidationErrorResponse(w, err.Error(), map[string]string{"mapping": "invalid"})
	case errors.Is(err, errMappingTargetSilent):
		// 503, not 400: the request may be perfectly correct and the backend
		// cannot tell yet. Retrying once the add-on answers is the right move.
		jsonErrorResponse(w, http.StatusServiceUnavailable, "TARGET_CAPABILITIES_UNKNOWN", err.Error())
	case errors.Is(err, addons.ErrValueNotResolvable):
		// The add-on's OWN sentinel, not a local restatement of it. This branch
		// used to test a sentinel declared in this file that nothing ever
		// wrapped, so a mapping naming a group the NAS does not have — an
		// operator typo, the single likeliest mistake on this form — fell
		// through to the default and came back 500 DB_ERROR. The target had
		// answered clearly and the operator was shown an internal error.
		jsonValidationErrorResponse(w, err.Error(), map[string]string{"value": "unresolvable"})
	default:
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
	}
}
